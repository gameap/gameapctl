package panel

import (
	"context"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/pkg/errors"
)

func Start(ctx context.Context) error {
	err := oscore.ExecCommand(ctx, "winsw", "start", defaultServiceConfigPath)
	if err != nil {
		return errors.WithMessage(err, "failed to execute start gameap command")
	}

	return nil
}
