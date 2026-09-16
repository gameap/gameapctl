//go:build !windows

package update

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/pkg/errors"
)

const panelUnitName = "gameap.service"

// newCrashDetector watches the systemd unit of the panel from its state right
// after the start. Without systemctl, or for a panel systemd does not run, nothing
// can be told and the wait relies on its timeout.
func newCrashDetector(ctx context.Context, scope string) crashDetector {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return noCrashDetection
	}

	baseline, err := querySystemdService(ctx, scope)
	if err != nil || baseline.LoadState != systemdLoadStateLoaded {
		return noCrashDetection
	}

	return func(ctx context.Context) error {
		current, err := querySystemdService(ctx, scope)
		if err != nil {
			return nil //nolint:nilerr // a failed query tells nothing about the panel
		}

		return serviceCrashed(baseline, current)
	}
}

// querySystemdService runs systemctl show without logging the command: it is
// polled every few seconds while the panel starts.
func querySystemdService(ctx context.Context, scope string) (systemdServiceState, error) {
	args := []string{"show", panelUnitName, "-p", "LoadState", "-p", "ActiveState", "-p", "NRestarts"}
	if scope == gameap.ScopeUser {
		args = append([]string{"--user"}, args...)
	}

	var stdout bytes.Buffer

	cmd := exec.CommandContext(ctx, "systemctl", args...)
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return systemdServiceState{}, errors.Wrap(err, "failed to query the GameAP service")
	}

	return parseSystemdServiceState(stdout.String()), nil
}
