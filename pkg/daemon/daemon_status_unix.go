//go:build linux || darwin

package daemon

import (
	"context"
	"log"

	"github.com/pkg/errors"
)

func Status(ctx context.Context) error {
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
