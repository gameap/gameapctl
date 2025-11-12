package panel

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
		err = stopProcess(ctx)
	}

	if err != nil {
		return errors.WithMessage(err, "failed to stop gameap")
	}

	return nil
}

func stopDaemonSystemd(ctx context.Context) error {
	_, err := os.Stat(systemdConfigPath)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		return errors.WithMessagef(
			ErrGameAPNotInstalled,
			"gameap systemd configuration file %s not found",
			systemdConfigPath,
		)
	}
	if err != nil {
		return errors.WithMessage(err, "failed to stat gameap service configuration")
	}

	err = service.Stop(ctx, "gameap")
	if err != nil {
		return errors.WithMessage(err, "failed to stop gameap")
	}

	return nil
}

func stopProcess(ctx context.Context) error {
	p, err := oscore.FindProcessByName(ctx, processName)
	if err != nil {
		return errors.WithMessage(err, "failed to find gameap process")
	}
	if p == nil {
		return errors.New("gameap process not found")
	}

	log.Printf("Found gameap process with pid %d \n", p.Pid)

	ctxWithTimeout, cancel := context.WithTimeout(ctx, defaultTerminateWaitTimeout)
	defer cancel()

	err = oscore.TerminateAndKillProcess(ctxWithTimeout, p)
	if err != nil {
		return errors.WithMessage(err, "failed to terminate/kill gameap process")
	}

	return nil
}
