package panel

import (
	"context"

	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
)

func Restart(ctx context.Context) error {
	err := service.Restart(ctx, serviceName)
	if err != nil {
		return errors.WithMessage(err, "failed to restart gameap service")
	}

	return nil
}
