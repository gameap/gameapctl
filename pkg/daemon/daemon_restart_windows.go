//go:build windows

package daemon

import (
	"context"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func Restart(_ context.Context) error {
	err := utils.ExecCommand("winsw", "restart", defaultDaemonConfigPath)
	if err != nil {
		return errors.WithMessage(err, "failed to get daemon status")
	}

	return nil
}
