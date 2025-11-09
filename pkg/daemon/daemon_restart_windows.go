//go:build windows

package daemon

import (
	"context"

	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
)

func Restart(ctx context.Context) error {
	err := service.Restart(ctx, serviceName)
	if err != nil {
		return errors.WithMessage(err, "failed to get daemon status")
	}

	return nil
}
