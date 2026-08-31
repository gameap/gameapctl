package panel

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/gameap/gameapctl/pkg/releasefinder"
)

const (
	versionProbeTimeout = 10 * time.Second
	versionOutputPrefix = "GameAP "
)

// InstalledVersion reports the version the installed panel binary prints, e.g.
// "v4.5.0". Panels older than v4.5 do not know the flag and yield an empty
// string, as does a binary built from a branch, which reports "development".
//
// The probe deliberately passes --version rather than the bare "version"
// command that v4.5 also accepts: an older panel parses its arguments with the
// flag package, which rejects an unknown flag and exits, while a positional
// argument is simply ignored and the panel starts serving.
func InstalledVersion(ctx context.Context, binaryPath string) string {
	ctx, cancel := context.WithTimeout(ctx, versionProbeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "--version")

	out := &bytes.Buffer{}
	cmd.Stdout = out
	// An older panel prints its usage here before exiting; that is an expected
	// answer, not something to put in the log.
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return ""
	}

	return parseVersionOutput(out.String())
}

func parseVersionOutput(out string) string {
	line, _, _ := strings.Cut(out, "\n")

	version := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), versionOutputPrefix))
	if version == "" {
		return ""
	}

	if !releasefinder.HasMajorMinor(version) {
		return ""
	}

	return version
}
