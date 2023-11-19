//go:build linux || darwin
// +build linux darwin

package daemon

import (
	"context"
	"io/fs"
	"log"
	"os"
	"time"

	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/process"
)

const (
	defaultTerminateWaitTimeout = 30 * time.Second
)

func Stop(ctx context.Context) error {
	init, err := detectInit(ctx)
	if err != nil {
		log.Println("Failed to detect init:", err)
	}

	switch init {
	case initSystemd:
		err = stopDaemonSystemd(ctx)
	case initUnknown:
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

	err = terminateAndKillProcess(ctxWithTimeout, p)
	if err != nil {
		return errors.WithMessage(err, "failed to terminate/kill daemon process")
	}

	return nil
}

func terminateAndKillProcess(ctx context.Context, p *process.Process) error {
	err := p.TerminateWithContext(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to terminate daemon process")
	}

	log.Println("Waiting for daemon process to terminate")
	ctxWithTimeout, cancel := context.WithTimeout(ctx, defaultTerminateWaitTimeout)
	defer cancel()
	ticker := time.NewTicker(1 * time.Second)

	for stop := false; !stop; {
		if isRunning, _ := p.IsRunning(); !isRunning {
			return nil
		}

		select {
		case <-ctxWithTimeout.Done():
			stop = true
		case <-ticker.C:
			log.Println("Daemon process still running")
		}
	}

	err = p.KillWithContext(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to kill daemon process")
	}

	return nil
}
