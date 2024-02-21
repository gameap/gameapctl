//go:build windows
// +build windows

package daemon

import (
	"context"
	"os/exec"
	"strings"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	defaultDaemonConfigPath = "C:\\gameap\\services\\gameap-daemon.xml"

	exitCodeStatusNotActive = 0
	exitCodeStatusActive    = 1
)

func Status(_ context.Context) error {
	var exitErr *exec.ExitError
	result, err := utils.ExecCommandWithOutput("winsw", "status", defaultDaemonConfigPath)
	if err != nil && !errors.As(err, &exitErr) {
		return errors.Wrap(err, "failed to get daemon status")
	}

	if exitErr.ExitCode() == exitCodeStatusNotActive {
		return errors.New("daemon process is not active")
	}

	if !strings.Contains(result, "Active (running)") {
		return errors.New("invalid result of daemon status command")
	}

	return nil
}
