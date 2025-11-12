//go:build linux || darwin

package daemon

import (
	"context"
	"io/fs"
	"log"
	"os"
	"time"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/runhelper"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
)

const (
	defaultTerminateWaitTimeout = 30 * time.Second
)

func Stop(ctx context.Context) error {
	init, err := runhelper.DetectInit(ctx)
	if err != nil {
		log.Println("Failed to detect init:", err)
	}

	switch init {
	case runhelper.InitSystemd:
		err = stopDaemonSystemd(ctx)
	case runhelper.InitUnknown:
		err = stopDaemonProcess(ctx)
	}

	if err != nil {
		return errors.WithMessage(err, "failed to stop daemon")
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

func stopDaemonProcess(ctx context.Context) error {
	p, err := FindProcess(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to find daemon process")
	}
	if p == nil {
		return errors.New("daemon process not found")
	}

	log.Printf("Found daemon process with pid %d \n", p.Pid)

	ctxWithTimeout, cancel := context.WithTimeout(ctx, defaultTerminateWaitTimeout)
	defer cancel()

	err = oscore.TerminateAndKillProcess(ctxWithTimeout, p)
	if err != nil {
		return errors.WithMessage(err, "failed to terminate/kill daemon process")
	}

	return nil
}
