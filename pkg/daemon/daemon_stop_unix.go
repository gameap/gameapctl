//go:build linux || darwin
// +build linux darwin

package daemon

import (
	"context"
	"io/fs"
	"os"

	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
)

func Stop(ctx context.Context) error {
	init, err := detectInit(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to detect init")
	}

	switch init {
	case initSystemd:
		err = stopDaemonSystemd(ctx)
	case initUnknown:
		err = stopDaemonProcess(ctx)
	}

	if err != nil {
		return errors.WithMessage(err, "failed to start daemon")
	}

	return nil
}

func stopDaemonSystemd(ctx context.Context) error {
	_, err := os.Stat(daemonSystemdConfigPath)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		return errors.WithMessagef(
			err,
			"daemon service configuration file %s not found",
			daemonSystemdConfigPath,
		)
	}
	if err != nil {
		return errors.WithMessage(err, "failed to stat gameap-daemon service configuration")
	}

	err = service.Stop(ctx, "gameap-daemon")
	if err != nil {
		return errors.WithMessage(err, "failed to stop gameap-daemon")
	}

	return nil
}

func stopDaemonProcess(_ context.Context) error {
	return errors.New("stopping daemon process is not implemented")
}
