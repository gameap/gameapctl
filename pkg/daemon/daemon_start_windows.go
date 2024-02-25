//go:build windows
// +build windows

package daemon

import (
	"context"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func Start(_ context.Context) error {
	err := utils.ExecCommand("winsw", "start", defaultDaemonConfigPath)
	if err != nil {
		return errors.WithMessage(err, "failed to execute start gameap-daemon command")
	}

	return nil
}
