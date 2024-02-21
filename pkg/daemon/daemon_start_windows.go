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

func Start(_ context.Context) error {
	result, err := utils.ExecCommandWithOutput("winsw", "start", defaultDaemonConfigPath)
	if err != nil {
		return errors.WithMessage(err, "failed to execute start gameap-daemon command")
	}

	switch {
	case strings.Contains(result, "started successfully"):
		fmt.Println("Daemon process started")
	case strings.Contains(result, "has already started"):
		fmt.Println("Daemon process already started. If you want to restart it, use 'gameapctl daemon restart'")
		return nil
	}

	return errors.New("failed to start daemon")
}
