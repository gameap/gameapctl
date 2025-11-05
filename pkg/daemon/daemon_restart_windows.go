//go:build windows

package daemon

import (
	"context"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/pkg/errors"
)

func Restart(ctx context.Context) error {
	err := oscore.ExecCommand(ctx, "winsw", "restart", defaultDaemonConfigPath)
	if err != nil {
		return errors.WithMessage(err, "failed to get daemon status")
	}

	return nil
}
