//go:build windows

package daemon

import (
	"context"
	"log"

	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
)

func Status(ctx context.Context) error {
	err := service.Status(ctx, serviceName)
	if err != nil {
		if errors.Is(err, service.ErrInactiveService) {
			return errors.WithMessage(err, "daemon process is not active")
		} else {
			return errors.WithMessage(err, "failed to get daemon status")
		}
	}

	log.Println("Daemon status is active. Checking process...")

	p, err := FindProcess(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to find daemon process")
	}
	if p == nil {
		return errors.New("daemon process not found")
	}
	log.Println("Daemon process found with pid", p.Pid)

	return nil
}
