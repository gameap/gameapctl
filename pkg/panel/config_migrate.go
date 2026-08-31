package panel

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gameap/gameapctl/pkg/configenv"
	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

// configEnvRename is a config.env key the panel renamed.
//
// Convert adapts the value on the way from Old to New, Revert on the way back.
// Both are nil when only the name changed.
type configEnvRename struct {
	Old     string
	New     string
	Convert configenv.Converter
	Revert  configenv.Converter
}

// configEnvDrop is a config.env key the panel stopped reading.
//
// RestoreValue is written back when config.env is migrated down to a release
// that still read the key. An empty RestoreValue means the key is not restored:
// it carried no meaning for the older panel either.
type configEnvDrop struct {
	Key          string
	RestoreValue string
}

// configEnvMigration is one panel release that changed the config.env contract.
//
// Entries are applied in declaration order (reversed when migrating down) and
// must be idempotent: the same config.env is migrated on every upgrade, and the
// gate looks at the target release alone, so a config that was already migrated
// simply reports no changes.
type configEnvMigration struct {
	MinVersion string
	Renames    []configEnvRename
	Drops      []configEnvDrop
}

// configEnvMigrations is the full history of config.env renames, oldest first.
//
// The panel keeps its own compatibility shim (internal/config/deprecated.go) for
// exactly one release, so an operator who skips a release loses the setting.
// gameapctl is the only place that remembers the whole chain, which is why
// entries are never dropped from this table.
var configEnvMigrations = []configEnvMigration{
	{
		// GameAP v4.3.0 stopped reading these. GRPC_ENABLED is restored as
		// "true" on the way down because that is what the installer writes for
		// a panel that speaks gRPC (see applyGRPCDecision); LEGACY_PATH and
		// LEGACY_ENV_PATH were always rendered empty, so there is nothing to
		// put back. GRPC_PORT is still read and is deliberately absent here.
		MinVersion: "v4.3",
		Drops: []configEnvDrop{
			{Key: "GRPC_ENABLED", RestoreValue: "true"},
			{Key: "LEGACY_PATH"},
			{Key: "LEGACY_ENV_PATH"},
		},
	},
	{
		// GameAP v4.5.0 moved the unit out of the name and into the value.
		MinVersion: "v4.5",
		Renames: []configEnvRename{
			{
				Old:     "PLUGIN_HTTP_MAX_TIMEOUT_SECONDS",
				New:     "PLUGIN_HTTP_MAX_TIMEOUT",
				Convert: appendUnit("s"),
				Revert:  durationToSeconds,
			},
			{
				Old:     "PLUGIN_NET_MAX_TIMEOUT_SECONDS",
				New:     "PLUGIN_NET_MAX_TIMEOUT",
				Convert: appendUnit("s"),
				Revert:  durationToSeconds,
			},
			{
				// A plain byte count is already a valid ByteSize, so only the
				// way back has to expand a suffix the old int field cannot read.
				Old:    "PLUGIN_NET_READ_BUFFER_BYTES",
				New:    "PLUGIN_NET_READ_BUFFER",
				Revert: byteSizeToBytes,
			},
		},
	},
	{
		// GameAP v4.5.0 put the whole plugin block under one prefix: PLUGINS_
		// follows the Plugins struct that holds it. Mirrors renamedVars in the
		// panel's internal/config/deprecated.go one for one, so the two tables
		// can be compared with a diff.
		MinVersion: "v4.5",
		Renames: []configEnvRename{
			{Old: "PLUGIN_STORE_URL", New: "PLUGINS_STORE_URL"},
			{Old: "PLUGIN_STORE_LICENSE_KEY", New: "PLUGINS_STORE_LICENSE_KEY"},
			{Old: "PLUGIN_HTTP_BLOCK_PRIVATE_IPS", New: "PLUGINS_HTTP_BLOCK_PRIVATE_IPS"},
			{Old: "PLUGIN_HTTP_ALLOWED_SCHEMES", New: "PLUGINS_HTTP_ALLOWED_SCHEMES"},
			{Old: "PLUGIN_HTTP_ALLOWED_HOSTS", New: "PLUGINS_HTTP_ALLOWED_HOSTS"},
			{Old: "PLUGIN_HTTP_MAX_TIMEOUT", New: "PLUGINS_HTTP_MAX_TIMEOUT"},
			{Old: "PLUGIN_HTTP_MAX_REDIRECTS", New: "PLUGINS_HTTP_MAX_REDIRECTS"},
			{Old: "PLUGIN_HTTP_RESPONSE_HEADER_ALLOWLIST", New: "PLUGINS_HTTP_RESPONSE_HEADER_ALLOWLIST"},
			{Old: "PLUGIN_SCHEDULER_MIN_INTERVAL", New: "PLUGINS_SCHEDULER_MIN_INTERVAL"},
			{Old: "PLUGIN_SCHEDULER_MAX_TASKS_PER_PLUGIN", New: "PLUGINS_SCHEDULER_MAX_TASKS_PER_PLUGIN"},
			{Old: "PLUGIN_SCHEDULER_CALL_TIMEOUT", New: "PLUGINS_SCHEDULER_CALL_TIMEOUT"},
			{Old: "PLUGIN_SCHEDULER_MAX_CALL_TIMEOUT", New: "PLUGINS_SCHEDULER_MAX_CALL_TIMEOUT"},
			{Old: "PLUGIN_SCHEDULER_MAX_RETRIES", New: "PLUGINS_SCHEDULER_MAX_RETRIES"},
			{Old: "PLUGIN_SCHEDULER_MAX_RETRY_DELAY", New: "PLUGINS_SCHEDULER_MAX_RETRY_DELAY"},
			{Old: "PLUGIN_SCHEDULER_MAX_JITTER", New: "PLUGINS_SCHEDULER_MAX_JITTER"},
			{Old: "PLUGIN_SCHEDULER_REFRESH_INTERVAL", New: "PLUGINS_SCHEDULER_REFRESH_INTERVAL"},
			{Old: "PLUGIN_SECRETS_MAX_KEYS_PER_PLUGIN", New: "PLUGINS_SECRETS_MAX_KEYS_PER_PLUGIN"},
			{Old: "PLUGIN_SECRETS_MAX_VALUE", New: "PLUGINS_SECRETS_MAX_VALUE"},
			{Old: "PLUGIN_SECRETS_REQUIRE_ENCRYPTION", New: "PLUGINS_SECRETS_REQUIRE_ENCRYPTION"},
			{Old: "PLUGIN_SSH_ENABLED", New: "PLUGINS_SSH_ENABLED"},
			{Old: "PLUGIN_SSH_BLOCK_PRIVATE_IPS", New: "PLUGINS_SSH_BLOCK_PRIVATE_IPS"},
			{Old: "PLUGIN_SSH_ALLOWED_HOSTS", New: "PLUGINS_SSH_ALLOWED_HOSTS"},
			{Old: "PLUGIN_SSH_ALLOW_ACCEPT_ANY_HOST_KEY", New: "PLUGINS_SSH_ALLOW_ACCEPT_ANY_HOST_KEY"},
			{Old: "PLUGIN_SSH_MAX_CONNECTIONS", New: "PLUGINS_SSH_MAX_CONNECTIONS"},
			{Old: "PLUGIN_SSH_MAX_OPERATIONS", New: "PLUGINS_SSH_MAX_OPERATIONS"},
			{Old: "PLUGIN_SSH_CONNECT_TIMEOUT", New: "PLUGINS_SSH_CONNECT_TIMEOUT"},
			{Old: "PLUGIN_SSH_MAX_EXEC_TIMEOUT", New: "PLUGINS_SSH_MAX_EXEC_TIMEOUT"},
			{Old: "PLUGIN_SSH_IDLE_TIMEOUT", New: "PLUGINS_SSH_IDLE_TIMEOUT"},
			{Old: "PLUGIN_SSH_MAX_OUTPUT_BYTES", New: "PLUGINS_SSH_MAX_OUTPUT_BYTES"},
			{Old: "PLUGIN_SSH_MAX_STDIN_BYTES", New: "PLUGINS_SSH_MAX_STDIN_BYTES"},
			{Old: "PLUGIN_SSH_OPERATION_RETENTION", New: "PLUGINS_SSH_OPERATION_RETENTION"},
			{Old: "PLUGIN_SSH_MAX_RETAINED_OPERATIONS", New: "PLUGINS_SSH_MAX_RETAINED_OPERATIONS"},
			{Old: "PLUGIN_SSH_KEEPALIVE_INTERVAL", New: "PLUGINS_SSH_KEEPALIVE_INTERVAL"},
			{Old: "PLUGIN_SSH_COMPLETION_CALL_TIMEOUT", New: "PLUGINS_SSH_COMPLETION_CALL_TIMEOUT"},
			{Old: "PLUGIN_SSH_BUSY_RETRY_DELAY", New: "PLUGINS_SSH_BUSY_RETRY_DELAY"},
			{Old: "PLUGIN_SSH_BUSY_RETRIES", New: "PLUGINS_SSH_BUSY_RETRIES"},
			{Old: "PLUGIN_NET_ENABLED", New: "PLUGINS_NET_ENABLED"},
			{Old: "PLUGIN_NET_BLOCK_PRIVATE_IPS", New: "PLUGINS_NET_BLOCK_PRIVATE_IPS"},
			{Old: "PLUGIN_NET_ALLOWED_HOSTS", New: "PLUGINS_NET_ALLOWED_HOSTS"},
			{Old: "PLUGIN_NET_MAX_TIMEOUT", New: "PLUGINS_NET_MAX_TIMEOUT"},
			{Old: "PLUGIN_NET_READ_BUFFER", New: "PLUGINS_NET_READ_BUFFER"},
			{Old: "PLUGIN_NET_MAX_CONNECTIONS", New: "PLUGINS_NET_MAX_CONNECTIONS"},
			{Old: "PLUGIN_RUNTIME_MAX_MEMORY", New: "PLUGINS_RUNTIME_MAX_MEMORY"},
			{Old: "PLUGIN_RUNTIME_MAX_MODULE_SIZE", New: "PLUGINS_RUNTIME_MAX_MODULE_SIZE"},
			{Old: "PLUGINS_CACHE_ENABLED", New: "PLUGINS_RUNTIME_CACHE_ENABLED"},
			{Old: "PLUGINS_CACHE_DIR", New: "PLUGINS_RUNTIME_CACHE_DIR"},
			{Old: "PLUGIN_PERMISSIONS_ENFORCE", New: "PLUGINS_PERMISSIONS_ENFORCE"},
			{Old: "PLUGIN_PERMISSIONS_CACHE_TTL", New: "PLUGINS_PERMISSIONS_CACHE_TTL"},
			{Old: "PLUGIN_RECOVERY_ENABLED", New: "PLUGINS_RECOVERY_ENABLED"},
			{Old: "PLUGIN_RECOVERY_INITIAL_DELAY", New: "PLUGINS_RECOVERY_INITIAL_DELAY"},
			{Old: "PLUGIN_RECOVERY_MAX_DELAY", New: "PLUGINS_RECOVERY_MAX_DELAY"},
			{Old: "PLUGIN_RECOVERY_MAX_ATTEMPTS", New: "PLUGINS_RECOVERY_MAX_ATTEMPTS"},
			{Old: "PLUGIN_SYNC_DISABLED", New: "PLUGINS_SYNC_DISABLED"},
			{Old: "PLUGIN_SYNC_REFRESH_INTERVAL", New: "PLUGINS_SYNC_REFRESH_INTERVAL"},
			{Old: "PLUGIN_SYNC_MIN_BACKOFF", New: "PLUGINS_SYNC_MIN_BACKOFF"},
			{Old: "PLUGIN_SYNC_MAX_BACKOFF", New: "PLUGINS_SYNC_MAX_BACKOFF"},
			{Old: "PLUGIN_NODEFS_MAX_INLINE", New: "PLUGINS_NODEFS_MAX_INLINE"},
			{Old: "PLUGIN_NODEFS_PATH_POLICY", New: "PLUGINS_NODEFS_PATH_POLICY"},
			{Old: "PLUGIN_NODEFS_ALLOWED_PATHS", New: "PLUGINS_NODEFS_ALLOWED_PATHS"},
			{Old: "PLUGIN_STORAGE_MAX_KEYS_PER_PLUGIN", New: "PLUGINS_STORAGE_MAX_KEYS_PER_PLUGIN"},
			{Old: "PLUGIN_STORAGE_MAX_VALUE", New: "PLUGINS_STORAGE_MAX_VALUE"},
			{Old: "PLUGIN_STORAGE_MAX_TOTAL", New: "PLUGINS_STORAGE_MAX_TOTAL"},
			{Old: "PLUGIN_CACHE_MAX_VALUE", New: "PLUGINS_CACHE_MAX_VALUE"},
			{Old: "PLUGIN_RATELIMIT_NODECMD_RPS", New: "PLUGINS_RATELIMIT_NODECMD_RPS"},
			{Old: "PLUGIN_RATELIMIT_NODECMD_BURST", New: "PLUGINS_RATELIMIT_NODECMD_BURST"},
			{Old: "PLUGIN_RATELIMIT_SERVERCONTROL_RPS", New: "PLUGINS_RATELIMIT_SERVERCONTROL_RPS"},
			{Old: "PLUGIN_RATELIMIT_SERVERCONTROL_BURST", New: "PLUGINS_RATELIMIT_SERVERCONTROL_BURST"},
			{Old: "PLUGIN_RATELIMIT_NODEFS_RPS", New: "PLUGINS_RATELIMIT_NODEFS_RPS"},
			{Old: "PLUGIN_RATELIMIT_NODEFS_BURST", New: "PLUGINS_RATELIMIT_NODEFS_BURST"},
			{Old: "PLUGIN_RATELIMIT_HTTP_RPS", New: "PLUGINS_RATELIMIT_HTTP_RPS"},
			{Old: "PLUGIN_RATELIMIT_HTTP_BURST", New: "PLUGINS_RATELIMIT_HTTP_BURST"},
			{Old: "PLUGIN_RATELIMIT_RBAC_RPS", New: "PLUGINS_RATELIMIT_RBAC_RPS"},
			{Old: "PLUGIN_RATELIMIT_RBAC_BURST", New: "PLUGINS_RATELIMIT_RBAC_BURST"},
			{Old: "PLUGIN_RATELIMIT_SSH_RPS", New: "PLUGINS_RATELIMIT_SSH_RPS"},
			{Old: "PLUGIN_RATELIMIT_SSH_BURST", New: "PLUGINS_RATELIMIT_SSH_BURST"},
		},
	},
}

// appendUnit turns a bare number into a value carrying its unit ("512" ->
// "512M"). A value that already has a non-numeric tail is passed through: an
// operator who set the old variable to "512M" gets what they meant, not
// "512MM". Mirrors the helper the panel deleted along with the shim, so this is
// now the only copy.
func appendUnit(unit string) configenv.Converter {
	return func(value string) string {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return value
		}

		if !isDigits(trimmed) {
			return trimmed
		}

		return trimmed + unit
	}
}

// durationToSeconds undoes appendUnit("s") for a field that used to hold a
// whole number of seconds. A duration finer than a second is truncated, and
// anything that is not a duration at all is passed through for the operator to
// fix.
func durationToSeconds(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || isDigits(trimmed) {
		return value
	}

	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return trimmed
	}

	return strconv.FormatInt(int64(parsed.Seconds()), 10)
}

// byteSizeToBytes expands a ByteSize back into the plain byte count the older
// int field could read ("64K" -> "65536"). Suffixes are powers of 1024, as in
// the panel's ByteSize. Anything unrecognized is passed through.
func byteSizeToBytes(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || isDigits(trimmed) {
		return value
	}

	matches := byteSizePattern.FindStringSubmatch(strings.ToUpper(trimmed))
	if matches == nil {
		return trimmed
	}

	multiplier, ok := byteSizeSuffixes[matches[2]]
	if !ok {
		return trimmed
	}

	number, err := strconv.ParseFloat(matches[1], 64)
	if err != nil || number < 0 {
		return trimmed
	}

	return strconv.FormatUint(uint64(number*float64(multiplier)), 10)
}

var (
	byteSizePattern = regexp.MustCompile(`^\s*(\d+(?:\.\d+)?)\s*([KMGTP]?I?B?)\s*$`)

	byteSizeSuffixes = map[string]uint64{
		"": 1, "B": 1,
		"K": kibibyte, "KB": kibibyte, "KIB": kibibyte,
		"M": mebibyte, "MB": mebibyte, "MIB": mebibyte,
		"G": gibibyte, "GB": gibibyte, "GIB": gibibyte,
		"T": tebibyte, "TB": tebibyte, "TIB": tebibyte,
		"P": pebibyte, "PB": pebibyte, "PIB": pebibyte,
	}
)

// Every ByteSize suffix is a power of 1024, matching the panel's own parser.
const (
	kibibyte uint64 = 1 << 10
	mebibyte        = kibibyte << 10
	gibibyte        = mebibyte << 10
	tebibyte        = gibibyte << 10
	pebibyte        = tebibyte << 10
)

func isDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// ConfigEnvMigration is the outcome of MigrateConfigEnv. Its zero value
// describes a config.env that was not touched and whose Restore is a no-op.
type ConfigEnvMigration struct {
	Changes []string

	path   string
	before []string
}

// Restore puts config.env back the way it was before the migration.
func (m ConfigEnvMigration) Restore() error {
	if len(m.Changes) == 0 {
		return nil
	}

	return configenv.Write(m.path, m.before)
}

// MigrateConfigEnv brings config.env in line with what the panel release
// targetVersion reads.
//
// installedVersion only tells the two directions apart: when it is newer than
// the target the renames are undone, so that a pinned older panel still finds
// its settings. An empty or unparseable installedVersion means "unknown" and
// selects the ordinary forward migration, which is safe because every forward
// migration is idempotent and gated on the target alone.
func MigrateConfigEnv(path, installedVersion, targetVersion string) (ConfigEnvMigration, error) {
	if cmp, ok := releasefinder.CompareMajorMinor(installedVersion, targetVersion); ok && cmp > 0 {
		return migrateConfigEnv(path, downgradePlan(installedVersion, targetVersion))
	}

	return migrateConfigEnv(path, upgradePlan(targetVersion))
}

// MigrateConfigEnvToLatest applies every forward migration. It is for builds
// from a GitHub branch, which carry no release tag: the branch is newer than
// every release, so there is no downgrade to consider.
func MigrateConfigEnvToLatest(path string) (ConfigEnvMigration, error) {
	return migrateConfigEnv(path, forwardPlan(func(string) bool { return true }))
}

// configEnvPlan is the migration table flattened into the changes one run has
// to make. Renames stay ordered so that a key renamed twice across releases
// (PLUGIN_HTTP_MAX_TIMEOUT_SECONDS -> PLUGIN_HTTP_MAX_TIMEOUT ->
// PLUGINS_HTTP_MAX_TIMEOUT) travels the whole chain in one pass. Removals and
// restores never share a key with a rename, so their order does not matter.
type configEnvPlan struct {
	renames  []plannedRename
	removes  []plannedDrop
	restores []plannedRestore
}

type plannedRename struct {
	from    string
	to      string
	convert configenv.Converter
}

type plannedDrop struct {
	key   string
	since string
}

type plannedRestore struct {
	key    string
	value  string
	before string
}

// upgradePlan collects every migration the target release expects to be
// applied. An unknown or unparseable target selects none of them, which leaves
// config.env exactly as it was.
func upgradePlan(targetVersion string) configEnvPlan {
	return forwardPlan(func(minVersion string) bool {
		return releasefinder.IsAtLeast(targetVersion, minVersion)
	})
}

func forwardPlan(applies func(minVersion string) bool) configEnvPlan {
	var plan configEnvPlan

	for _, migration := range configEnvMigrations {
		if !applies(migration.MinVersion) {
			continue
		}

		for _, rename := range migration.Renames {
			plan.renames = append(plan.renames, plannedRename{
				from:    rename.Old,
				to:      rename.New,
				convert: rename.Convert,
			})
		}

		for _, drop := range migration.Drops {
			plan.removes = append(plan.removes, plannedDrop{key: drop.Key, since: migration.MinVersion})
		}
	}

	return plan
}

// downgradePlan undoes the migrations that lie between the target and the
// installed release, newest first.
func downgradePlan(installedVersion, targetVersion string) configEnvPlan {
	var plan configEnvPlan

	for i := len(configEnvMigrations) - 1; i >= 0; i-- {
		migration := configEnvMigrations[i]

		if !releasefinder.IsAtLeast(installedVersion, migration.MinVersion) {
			continue
		}

		if releasefinder.IsAtLeast(targetVersion, migration.MinVersion) {
			continue
		}

		for _, rename := range migration.Renames {
			plan.renames = append(plan.renames, plannedRename{
				from:    rename.New,
				to:      rename.Old,
				convert: rename.Revert,
			})
		}

		for _, drop := range migration.Drops {
			if drop.RestoreValue == "" {
				continue
			}

			plan.restores = append(plan.restores, plannedRestore{
				key:    drop.Key,
				value:  drop.RestoreValue,
				before: migration.MinVersion,
			})
		}
	}

	return plan
}

func migrateConfigEnv(path string, plan configEnvPlan) (ConfigEnvMigration, error) {
	if !utils.IsFileExists(path) {
		return ConfigEnvMigration{}, nil
	}

	lines, values, err := configenv.Read(path)
	if err != nil {
		return ConfigEnvMigration{}, errors.WithMessage(err, "failed to read config.env")
	}

	before := slices.Clone(lines)

	lines, changes := applyRenames(lines, values, plan.renames)

	lines, removed := applyDrops(lines, values, plan.removes)
	changes = append(changes, removed...)

	lines, restored := applyRestores(lines, values, plan.restores)
	changes = append(changes, restored...)

	if len(changes) == 0 {
		return ConfigEnvMigration{}, nil
	}

	if err := configenv.Write(path, lines); err != nil {
		return ConfigEnvMigration{}, errors.WithMessage(err, "failed to write config.env")
	}

	return ConfigEnvMigration{Changes: changes, path: path, before: before}, nil
}

// applyRenames rewrites every key the plan renames, keeping values in step so
// that a chained rename finds the key its predecessor produced. When both names
// are assigned, the one the target release reads wins and the other is dropped,
// the same rule the panel's own shim follows.
func applyRenames(lines []string, values map[string]string, renames []plannedRename) ([]string, []string) {
	changes := make([]string, 0, len(renames))

	for _, rename := range renames {
		oldValue, present := values[rename.from]
		if !present {
			continue
		}

		if _, taken := values[rename.to]; taken {
			var removed bool

			lines, removed = configenv.Remove(lines, rename.from)
			if !removed {
				continue
			}

			delete(values, rename.from)
			changes = append(changes, fmt.Sprintf("%s removed, superseded by %s", rename.from, rename.to))

			continue
		}

		newValue, renamed := configenv.Rename(lines, rename.from, rename.to, rename.convert)
		if !renamed {
			continue
		}

		delete(values, rename.from)
		values[rename.to] = newValue

		if newValue == oldValue {
			changes = append(changes, fmt.Sprintf("%s renamed to %s", rename.from, rename.to))

			continue
		}

		changes = append(changes,
			fmt.Sprintf("%s renamed to %s (%s -> %s)", rename.from, rename.to, oldValue, newValue))
	}

	return lines, changes
}

func applyDrops(lines []string, values map[string]string, drops []plannedDrop) ([]string, []string) {
	changes := make([]string, 0, len(drops))

	for _, drop := range drops {
		if _, present := values[drop.key]; !present {
			continue
		}

		var removed bool

		lines, removed = configenv.Remove(lines, drop.key)
		if !removed {
			continue
		}

		delete(values, drop.key)
		changes = append(changes, fmt.Sprintf("%s removed, unused since GameAP %s", drop.key, drop.since))
	}

	return lines, changes
}

func applyRestores(lines []string, values map[string]string, restores []plannedRestore) ([]string, []string) {
	changes := make([]string, 0, len(restores))

	for _, restore := range restores {
		if _, present := values[restore.key]; present {
			continue
		}

		lines = configenv.Append(lines, restore.key, restore.value)
		values[restore.key] = restore.value
		changes = append(changes,
			fmt.Sprintf("%s restored, read by GameAP before %s", restore.key, restore.before))
	}

	return lines, changes
}
