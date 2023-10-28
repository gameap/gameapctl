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

func Restart(ctx context.Context) error {
	init, err := detectInit(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to detect init")
	}

	switch init {
	case initSystemd:
		err = restartDaemonSystemd(ctx)
	case initUnknown:
		err = restartDaemonProcess(ctx)
	}

	return err
}

func restartDaemonSystemd(ctx context.Context) error {
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
	err = service.Restart(ctx, "gameap-daemon")
	if err != nil {
		return errors.WithMessage(err, "failed to restart gameap-daemon")
	}

	return nil
}

func restartDaemonProcess(ctx context.Context) error {
	p, err := FindProcess(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to find daemon process")
	}
	if p != nil {
		err := terminateAndKillProcess(ctx, p)
		if err != nil {
			return errors.WithMessage(err, "failed to terminate/kill daemon process")
		}
	}

	err = startDaemonFork(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to start daemon")
	}

	return nil
}
