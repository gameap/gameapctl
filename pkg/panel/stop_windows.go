package panel

import (
	"context"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/pkg/errors"
)

func Stop(ctx context.Context) error {
	err := oscore.ExecCommand(ctx, "winsw", "stop", defaultServiceConfigPath)
	if err != nil {
		return errors.WithMessage(err, "failed to execute stop gameap command")
	}

	return nil
}
