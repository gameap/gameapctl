package letsencrypt

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

const (
	defaultConfigDirUnix    = "/etc/gameap"
	defaultConfigDirWindows = "C:\\gameap\\web"
	configFileName          = "config.env"
)

// envKeysOwned lists every config.env key that the letsencrypt subcommand
// rewrites. Anything not in this list is preserved verbatim. DNS-provider-
// specific credentials that the operator supplies live alongside these and
// are merged in at write time.
var envKeysOwned = []string{
	"ACME_ENABLED",
	"ACME_CHALLENGE_TYPE",
	"ACME_EMAIL",
	"ACME_DOMAINS",
	"ACME_DIRECTORY_URL",
	"ACME_DNS_PROVIDER",
	"ACME_RENEWAL_THRESHOLD",
	"ACME_RENEWAL_CHECK_INTERVAL",
	"ACME_PROPAGATION_TIMEOUT",
	"ACME_STORAGE_PATH",
}

const (
	ChallengeHTTP01 = "http-01"
	ChallengeDNS01  = "dns-01"
)

func ConfigPath() string {
	var dir string
	if runtime.GOOS == "windows" {
		dir = defaultConfigDirWindows
	} else {
		dir = defaultConfigDirUnix
	}

	return filepath.Join(dir, configFileName)
}

// readEnv parses config.env preserving line order, comments, and blank lines.
// The returned slice is the source-of-truth representation; the map is a quick
// lookup of current values.
func readEnv(path string) (lines []string, values map[string]string, err error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, errors.Errorf("config file not found at %s", path)
		}

		return nil, nil, errors.WithMessage(err, "failed to open config file")
	}

	defer func() { _ = file.Close() }()

	values = make(map[string]string)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		raw := scanner.Text()
		lines = append(lines, raw)

		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if !strings.Contains(trimmed, "=") {
			continue
		}

		parts := strings.SplitN(trimmed, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		values[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, errors.WithMessage(err, "failed to read config file")
	}

	return lines, values, nil
}

// writeEnv replaces lines for keys present in updates while preserving every
// other line verbatim. New keys are appended at the end (sorted) for
// deterministic diffs. Removal is signalled by an entry whose value is the
// sentinel removeMarker.
const removeMarker = "\x00__REMOVE__\x00"

func writeEnv(path string, lines []string, updates map[string]string) error {
	seen := make(map[string]bool, len(updates))

	for i, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "=") {
			continue
		}

		parts := strings.SplitN(trimmed, "=", 2)
		key := strings.TrimSpace(parts[0])

		newValue, ok := updates[key]
		if !ok {
			continue
		}

		seen[key] = true

		if newValue == removeMarker {
			lines[i] = ""
		} else {
			lines[i] = key + "=" + newValue
		}
	}

	missing := make([]string, 0)
	for k, v := range updates {
		if seen[k] || v == removeMarker {
			continue
		}

		missing = append(missing, k)
	}

	sort.Strings(missing)

	for _, k := range missing {
		lines = append(lines, k+"="+updates[k])
	}

	tmp := path + ".tmp"

	const ownerOnly = 0o600

	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, ownerOnly)
	if err != nil {
		return errors.WithMessage(err, "failed to open temp config")
	}

	w := bufio.NewWriter(out)

	for i, raw := range lines {
		if _, err := w.WriteString(raw); err != nil {
			_ = out.Close()
			_ = os.Remove(tmp)

			return errors.WithMessage(err, "failed to write config")
		}

		if i < len(lines)-1 {
			if _, err := w.WriteString("\n"); err != nil {
				_ = out.Close()
				_ = os.Remove(tmp)

				return errors.WithMessage(err, "failed to write config newline")
			}
		}
	}

	if err := w.Flush(); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)

		return errors.WithMessage(err, "failed to flush config")
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)

		return errors.WithMessage(err, "failed to close config")
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)

		return errors.WithMessage(err, "failed to rename config")
	}

	return nil
}
