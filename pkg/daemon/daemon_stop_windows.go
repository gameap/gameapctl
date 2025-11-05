//go:build windows

package daemon

import (
	"context"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/pkg/errors"
)

func Stop(ctx context.Context) error {
	err := oscore.ExecCommand(ctx, "winsw", "stop", defaultDaemonConfigPath)
	if err != nil {
		return errors.WithMessage(err, "failed to execute stop gameap-daemon command")
	}

	return nil
}
