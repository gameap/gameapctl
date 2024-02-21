//go:build windows
// +build windows

package daemon

import (
	"context"
	"log"
	"os/exec"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	defaultDaemonConfigPath = "C:\\gameap\\services\\gameap-daemon.xml"

	exitCodeStatusNotActive = 0
	exitCodeStatusActive    = 1
)

func Status(ctx context.Context) error {
	var exitErr *exec.ExitError
	_, err := utils.ExecCommandWithOutput("winsw", "status", defaultDaemonConfigPath)
	if err != nil && !errors.As(err, &exitErr) {
		return errors.Wrap(err, "failed to get daemon status")
	}

	if exitErr != nil {
		if exitErr.ExitCode() == exitCodeStatusNotActive {
			return errors.New("daemon process is not active")
		}

		if exitErr.ExitCode() != exitCodeStatusActive {
			return errors.WithMessagef(
				errors.New("unknown response"),
				"invalid exit code %d", exitErr.ExitCode(),
			)
		}
	}

	p, err := FindProcess(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to find daemon process")
	}
	if p == nil {
		return errors.New("daemon process not found")
	}
	log.Println("Daemon process found with pid", p.Pid)

	return nil
}
