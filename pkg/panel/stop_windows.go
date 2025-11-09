package panel

import (
	"context"

	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
)

func Stop(ctx context.Context) error {
	err := service.Stop(ctx, serviceName)
	if err != nil {
		return errors.WithMessage(err, "failed to execute stop gameap command")
	}

	return nil
}
