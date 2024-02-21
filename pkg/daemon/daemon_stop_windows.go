//go:build windows
// +build windows

package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func Stop(_ context.Context) error {
	result, err := utils.ExecCommandWithOutput("winsw", "stop", defaultDaemonConfigPath)
	if err != nil {
		return errors.WithMessage(err, "failed to execute stop gameap-daemon command")
	}

	switch {
	case strings.Contains(result, "stopped successfully"):
		fmt.Println("Daemon process stopped")
		return nil
	case strings.Contains(result, "has already stopped"):
		fmt.Println("Daemon process already stopped")
		return nil
	}

	return errors.New("failed to stop daemon")
}
