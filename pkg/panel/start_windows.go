package panel

import (
	"context"

	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
)

func Start(ctx context.Context) error {
	err := service.Start(ctx, serviceName)
	if err != nil {
		return errors.WithMessage(err, "failed to execute start gameap command")
	}

	return nil
}
