// Package configenv reads and rewrites the KEY=VALUE files GameAP is configured
// with (config.env for the panel), preserving line order, comments and blank
// lines so that a rewrite touches only what it was asked to touch.
package configenv

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

// RemoveMarker marks a key for deletion in the updates map passed to Update.
const RemoveMarker = "\x00__REMOVE__\x00"

// Converter adapts a value whose unit used to live in the key name, e.g. the
// "30" of PLUGIN_HTTP_MAX_TIMEOUT_SECONDS becoming the "30s" of
// PLUGIN_HTTP_MAX_TIMEOUT. It receives the value with its surrounding quotes
// already removed.
type Converter func(value string) string

const configMode = 0600

// Read parses an env file preserving line order, comments and blank lines. The
// returned slice is the source-of-truth representation; the map is a quick
// lookup of current values. A missing file yields an error matching
// fs.ErrNotExist.
func Read(path string) (lines []string, values map[string]string, err error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, errors.Wrapf(err, "config file not found at %s", path)
		}

		return nil, nil, errors.Wrap(err, "failed to open config file")
	}

	defer func() { _ = file.Close() }()

	values = make(map[string]string)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		raw := scanner.Text()
		lines = append(lines, raw)

		if key, value, ok := splitAssignment(raw); ok {
			values[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, errors.Wrap(err, "failed to read config file")
	}

	return lines, values, nil
}

// Update applies updates to lines and writes the result. Lines for keys present
// in updates are replaced in place, every other line is preserved verbatim, and
// new keys are appended at the end (sorted) for deterministic diffs. A value
// equal to RemoveMarker deletes the key's line.
func Update(path string, lines []string, updates map[string]string) error {
	seen := make(map[string]bool, len(updates))
	kept := make([]string, 0, len(lines))

	for _, raw := range lines {
		key, _, ok := splitAssignment(raw)
		if !ok {
			kept = append(kept, raw)

			continue
		}

		newValue, updated := updates[key]
		if !updated {
			kept = append(kept, raw)

			continue
		}

		seen[key] = true

		if newValue != RemoveMarker {
			kept = append(kept, key+"="+newValue)
		}
	}

	missing := make([]string, 0, len(updates))

	for k, v := range updates {
		if seen[k] || v == RemoveMarker {
			continue
		}

		missing = append(missing, k)
	}

	sort.Strings(missing)

	for _, k := range missing {
		kept = append(kept, k+"="+updates[k])
	}

	return Write(path, kept)
}

// Write replaces path with lines, atomically, preserving the previous file's
// mode and (on unix) owner. The last line is always terminated, so Read
// followed by Write with no edits reproduces the file byte for byte.
//
// The new content is staged under a unique name next to the file rather than a
// fixed one, so that two writers racing over the same config (an upgrade and a
// letsencrypt run, say) each rename a complete file into place instead of
// interleaving into a shared scratch file.
func Write(path string, lines []string) error {
	mode, uid, gid := currentOwnership(path)

	out, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return errors.Wrap(err, "failed to create temp config")
	}

	tmp := out.Name()

	if err := writeLines(out, lines); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)

		return err
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)

		return errors.Wrap(err, "failed to close config")
	}

	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)

		return errors.Wrap(err, "failed to set config mode")
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)

		return errors.Wrap(err, "failed to rename config")
	}

	// Guarded the same way as pkg/utils/fs.go: a file that reported no owner
	// (an unreadable stat, or any file on Windows) is left to the filesystem.
	if uid != 0 && gid != 0 {
		if err := os.Chown(path, int(uid), int(gid)); err != nil {
			return errors.Wrap(err, "failed to restore config ownership")
		}
	}

	return nil
}

func writeLines(out *os.File, lines []string) error {
	w := bufio.NewWriter(out)

	for _, raw := range lines {
		if _, err := w.WriteString(raw + "\n"); err != nil {
			return errors.Wrap(err, "failed to write config")
		}
	}

	if err := w.Flush(); err != nil {
		return errors.Wrap(err, "failed to flush config")
	}

	return nil
}

// currentOwnership reports the mode and owner a rewrite has to restore. A file
// that does not exist yet is created owner-only and unowned.
func currentOwnership(path string) (os.FileMode, uint32, uint32) {
	info, err := os.Stat(path)
	if err != nil {
		return configMode, 0, 0
	}

	uid, gid := utils.FileOwner(path)

	return info.Mode().Perm(), uid, gid
}

// Rename rewrites the assignment of oldKey so that it assigns newKey, in place,
// so the key keeps its position and the comment documenting it. convert, when
// not nil, adapts the value; it sees the value without its surrounding quotes
// and its result is re-quoted the same way. Reports the value written and
// whether a line was rewritten.
//
// Rename knows nothing about newKey already being assigned elsewhere in the
// file; that policy belongs to the caller.
func Rename(lines []string, oldKey, newKey string, convert Converter) (string, bool) {
	for i, raw := range lines {
		key, value, ok := splitAssignment(raw)
		if !ok || key != oldKey {
			continue
		}

		if convert != nil {
			inner, quote := unquote(value)
			value = quote + convert(inner) + quote
		}

		lines[i] = newKey + "=" + value

		return value, true
	}

	return "", false
}

// Remove deletes the line assigning key.
func Remove(lines []string, key string) ([]string, bool) {
	for i, raw := range lines {
		lineKey, _, ok := splitAssignment(raw)
		if !ok || lineKey != key {
			continue
		}

		return append(lines[:i:i], lines[i+1:]...), true
	}

	return lines, false
}

// Append adds an assignment at the end of the file.
func Append(lines []string, key, value string) []string {
	return append(lines, key+"="+value)
}

// splitAssignment recognizes a KEY=VALUE line. Comments, blank lines and lines
// without "=" are not assignments. Everything after the first "=" is the value:
// "#" only comments out a whole line, so "KEY=v # c" has the value "v # c" for
// the panel too.
func splitAssignment(raw string) (key, value string, ok bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}

	key, value, ok = strings.Cut(trimmed, "=")
	if !ok {
		return "", "", false
	}

	return strings.TrimSpace(key), strings.TrimSpace(value), true
}

// unquote splits a value into its content and the quote character wrapping it.
// The panel's own env file loader trims a leading and trailing quote, so
// KEY="10" is a working configuration whose value is 10; a converter must see
// the 10, not the quotes. Anything but a balanced pair is passed through.
func unquote(value string) (inner, quote string) {
	const minQuoted = 2

	if len(value) < minQuoted {
		return value, ""
	}

	first := value[0]
	if first != '"' && first != '\'' {
		return value, ""
	}

	if value[len(value)-1] != first {
		return value, ""
	}

	return value[1 : len(value)-1], string(first)
}
