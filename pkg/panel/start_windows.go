package panel

import (
	"context"

	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
)

func Start(ctx context.Context, opts ...Options) error {
	if err := checkScopeSupported(opts); err != nil {
		return err
	}

	err := service.Start(ctx, serviceName)
	if err != nil {
		return errors.WithMessage(err, "failed to execute start gameap command")
	}

	return nil
}
