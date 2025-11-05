package panel

import (
	"context"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/pkg/errors"
)

func Restart(ctx context.Context) error {
	err := oscore.ExecCommand(ctx, "winsw", "restart", defaultServiceConfigPath)
	if err != nil {
		return errors.WithMessage(err, "failed to get daemon status")
	}

	return nil
}
