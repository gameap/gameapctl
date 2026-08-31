package panel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfigEnv(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.env")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	return path
}

func readConfigEnvValues(t *testing.T, path string) map[string]string {
	t.Helper()

	body, err := os.ReadFile(path)
	require.NoError(t, err)

	values := make(map[string]string)

	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	return values
}

func TestMigrateConfigEnv_RenamesPluginVarsForTargetsFromV45(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		renamed bool
	}{
		{name: "beta_of_the_release_that_renamed_them", target: "v4.5.0-beta.1", renamed: true},
		{name: "the_release_itself", target: "v4.5.0", renamed: true},
		{name: "tag_without_the_leading_v", target: "4.5.0", renamed: true},
		{name: "later_minor", target: "v4.6.0", renamed: true},
		{name: "later_major", target: "v5.0.0", renamed: true},
		{name: "previous_release", target: "v4.4.2", renamed: false},
		{name: "much_older_release", target: "v4.2.4", renamed: false},
		{name: "unknown_target", target: "", renamed: false},
		{name: "unparseable_target", target: "garbage", renamed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfigEnv(t, "PLUGIN_SSH_ENABLED=true\n")

			migration, err := MigrateConfigEnv(path, "", tt.target)
			require.NoError(t, err)

			values := readConfigEnvValues(t, path)

			if !tt.renamed {
				assert.Empty(t, migration.Changes)
				assert.Equal(t, "true", values["PLUGIN_SSH_ENABLED"])

				return
			}

			require.Len(t, migration.Changes, 1)
			assert.Equal(t, "true", values["PLUGINS_SSH_ENABLED"])
			assert.NotContains(t, values, "PLUGIN_SSH_ENABLED")
		})
	}
}

func TestMigrateConfigEnv_RenamesTheCacheKeysThatKeptTheirPrefix(t *testing.T) {
	path := writeConfigEnv(t, "PLUGINS_CACHE_ENABLED=true\nPLUGINS_CACHE_DIR=/var/lib/gameap/wasm\nPLUGIN_CACHE_MAX_VALUE=1M\n")

	_, err := MigrateConfigEnv(path, "", "v4.6.0")
	require.NoError(t, err)

	values := readConfigEnvValues(t, path)

	assert.Equal(t, "true", values["PLUGINS_RUNTIME_CACHE_ENABLED"])
	assert.Equal(t, "/var/lib/gameap/wasm", values["PLUGINS_RUNTIME_CACHE_DIR"])
	assert.Equal(t, "1M", values["PLUGINS_CACHE_MAX_VALUE"])
	assert.NotContains(t, values, "PLUGINS_CACHE_ENABLED")
	assert.NotContains(t, values, "PLUGINS_CACHE_DIR")
	assert.NotContains(t, values, "PLUGIN_CACHE_MAX_VALUE")
}

func TestMigrateConfigEnv_ChainsRenamesAcrossReleasesInOnePass(t *testing.T) {
	path := writeConfigEnv(t, `PLUGIN_HTTP_MAX_TIMEOUT_SECONDS="45"
PLUGIN_NET_MAX_TIMEOUT_SECONDS=10
PLUGIN_NET_READ_BUFFER_BYTES=65536
`)

	migration, err := MigrateConfigEnv(path, "v4.4.2", "v4.6.0")
	require.NoError(t, err)
	require.Len(t, migration.Changes, 6)

	values := readConfigEnvValues(t, path)

	assert.Equal(t, `"45s"`, values["PLUGINS_HTTP_MAX_TIMEOUT"])
	assert.Equal(t, "10s", values["PLUGINS_NET_MAX_TIMEOUT"])
	assert.Equal(t, "65536", values["PLUGINS_NET_READ_BUFFER"])
	assert.NotContains(t, values, "PLUGIN_HTTP_MAX_TIMEOUT_SECONDS")
	assert.NotContains(t, values, "PLUGIN_HTTP_MAX_TIMEOUT")
}

func TestMigrateConfigEnv_RemovesDroppedKeys(t *testing.T) {
	const body = "GRPC_ENABLED=true\nGRPC_PORT=31718\nLEGACY_PATH=\nLEGACY_ENV_PATH=/var/www/gameap/.env\n"

	t.Run("target_that_stopped_reading_them", func(t *testing.T) {
		path := writeConfigEnv(t, body)

		_, err := MigrateConfigEnv(path, "", "v4.6.0")
		require.NoError(t, err)

		values := readConfigEnvValues(t, path)

		assert.NotContains(t, values, "GRPC_ENABLED")
		assert.NotContains(t, values, "LEGACY_PATH")
		assert.NotContains(t, values, "LEGACY_ENV_PATH")
		assert.Equal(t, "31718", values["GRPC_PORT"])
	})

	t.Run("target_that_still_reads_them", func(t *testing.T) {
		path := writeConfigEnv(t, body)

		migration, err := MigrateConfigEnv(path, "", "v4.2.4")
		require.NoError(t, err)

		assert.Empty(t, migration.Changes)
		assert.Equal(t, "true", readConfigEnvValues(t, path)["GRPC_ENABLED"])
	})
}

func TestMigrateConfigEnv_KeepsTheNameTheTargetReads(t *testing.T) {
	path := writeConfigEnv(t, "PLUGIN_SSH_ENABLED=false\nPLUGINS_SSH_ENABLED=true\n")

	migration, err := MigrateConfigEnv(path, "", "v4.6.0")
	require.NoError(t, err)
	require.Len(t, migration.Changes, 1)
	assert.Contains(t, migration.Changes[0], "superseded by PLUGINS_SSH_ENABLED")

	values := readConfigEnvValues(t, path)

	assert.Equal(t, "true", values["PLUGINS_SSH_ENABLED"])
	assert.NotContains(t, values, "PLUGIN_SSH_ENABLED")
}

func TestMigrateConfigEnv_IsIdempotent(t *testing.T) {
	path := writeConfigEnv(t, "PLUGIN_SSH_ENABLED=true\nGRPC_ENABLED=true\n")

	_, err := MigrateConfigEnv(path, "", "v4.6.0")
	require.NoError(t, err)

	first, err := os.ReadFile(path)
	require.NoError(t, err)

	second, err := MigrateConfigEnv(path, "", "v4.6.0")
	require.NoError(t, err)
	assert.Empty(t, second.Changes)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(first), string(after))
}

func TestMigrateConfigEnv_LeavesEverythingElseAlone(t *testing.T) {
	const body = `# Server
HTTP_HOST=0.0.0.0
HTTP_PORT=8025

# Security
ENCRYPTION_KEY=aG92ZXJjcmFmdC1mdWxsLW9mLWVlbHM=
AUTH_SECRET=c2l4LWJ5LW5pbmUtaXMtZm9ydHktdHdv

# Database
DATABASE_URL=postgres://gameap:p@ss=word@127.0.0.1:5432/gameap?sslmode=disable

PLUGIN_SSH_ENABLED=true
`

	path := writeConfigEnv(t, body)

	_, err := MigrateConfigEnv(path, "", "v4.6.0")
	require.NoError(t, err)

	rewritten, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Equal(t, strings.Replace(body, "PLUGIN_SSH_ENABLED", "PLUGINS_SSH_ENABLED", 1), string(rewritten))
}

func TestMigrateConfigEnv_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.env")

	migration, err := MigrateConfigEnv(path, "", "v4.6.0")
	require.NoError(t, err)

	assert.Empty(t, migration.Changes)
	assert.NoFileExists(t, path)
}

func TestMigrateConfigEnv_Restore(t *testing.T) {
	const body = "# plugins\nPLUGIN_SSH_ENABLED=true\nGRPC_ENABLED=true\n"

	path := writeConfigEnv(t, body)

	migration, err := MigrateConfigEnv(path, "", "v4.6.0")
	require.NoError(t, err)
	require.NotEmpty(t, migration.Changes)

	require.NoError(t, migration.Restore())

	restored, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, body, string(restored))
}

func TestConfigEnvMigration_ZeroValueRestoreDoesNothing(t *testing.T) {
	require.NoError(t, ConfigEnvMigration{}.Restore())
}

func TestMigrateConfigEnvToLatest(t *testing.T) {
	path := writeConfigEnv(t, "PLUGIN_HTTP_MAX_TIMEOUT_SECONDS=45\nGRPC_ENABLED=true\n")

	migration, err := MigrateConfigEnvToLatest(path)
	require.NoError(t, err)
	require.Len(t, migration.Changes, 3)

	values := readConfigEnvValues(t, path)

	assert.Equal(t, "45s", values["PLUGINS_HTTP_MAX_TIMEOUT"])
	assert.NotContains(t, values, "GRPC_ENABLED")
}

func TestMigrateConfigEnv_DowngradeRollsTheNamesBack(t *testing.T) {
	path := writeConfigEnv(t, "PLUGINS_HTTP_MAX_TIMEOUT=45s\nPLUGINS_NET_READ_BUFFER=64K\nPLUGINS_SSH_ENABLED=true\n")

	migration, err := MigrateConfigEnv(path, "v4.6.0", "v4.4.2")
	require.NoError(t, err)
	require.Len(t, migration.Changes, 5)

	values := readConfigEnvValues(t, path)

	assert.Equal(t, "45", values["PLUGIN_HTTP_MAX_TIMEOUT_SECONDS"])
	assert.Equal(t, "65536", values["PLUGIN_NET_READ_BUFFER_BYTES"])
	assert.Equal(t, "true", values["PLUGIN_SSH_ENABLED"])
	assert.NotContains(t, values, "PLUGINS_HTTP_MAX_TIMEOUT")
	assert.NotContains(t, values, "PLUGIN_HTTP_MAX_TIMEOUT")
}

func TestMigrateConfigEnv_DowngradeRestoresDroppedKeys(t *testing.T) {
	path := writeConfigEnv(t, "HTTP_PORT=8025\nGRPC_PORT=31718\n")

	_, err := MigrateConfigEnv(path, "v4.6.0", "v4.2.4")
	require.NoError(t, err)

	values := readConfigEnvValues(t, path)

	assert.Equal(t, "true", values["GRPC_ENABLED"])
	assert.NotContains(t, values, "LEGACY_PATH")
	assert.NotContains(t, values, "LEGACY_ENV_PATH")
}

func TestMigrateConfigEnv_DowngradeToTheSameMinorChangesNothing(t *testing.T) {
	path := writeConfigEnv(t, "PLUGINS_SSH_ENABLED=true\n")

	migration, err := MigrateConfigEnv(path, "v4.6.0", "v4.6.1")
	require.NoError(t, err)

	assert.Empty(t, migration.Changes)
}

func TestMigrateConfigEnv_RoundTrip(t *testing.T) {
	const body = "PLUGIN_HTTP_MAX_TIMEOUT_SECONDS=45\nPLUGIN_NET_READ_BUFFER_BYTES=65536\nPLUGIN_SSH_ENABLED=true\n"

	path := writeConfigEnv(t, body)

	_, err := MigrateConfigEnv(path, "v4.4.2", "v4.6.0")
	require.NoError(t, err)

	_, err = MigrateConfigEnv(path, "v4.6.0", "v4.4.2")
	require.NoError(t, err)

	assert.Equal(t, map[string]string{
		"PLUGIN_HTTP_MAX_TIMEOUT_SECONDS": "45",
		"PLUGIN_NET_READ_BUFFER_BYTES":    "65536",
		"PLUGIN_SSH_ENABLED":              "true",
	}, readConfigEnvValues(t, path))
}
