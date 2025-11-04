package panel

import (
	"context"
	"io/fs"
	"log"
	"os"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/runhelper"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
)

func Restart(ctx context.Context) error {
	init, err := runhelper.DetectInit(ctx)
	if err != nil {
		log.Println("Failed to detect init:", err)
	}

	switch init {
	case runhelper.InitSystemd:
		err = restartSystemd(ctx)
	case runhelper.InitUnknown:
		err = restartProcess(ctx)
	}

	return err
}

func restartSystemd(ctx context.Context) error {
	_, err := os.Stat(systemdConfigPath)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		return errors.WithMessagef(
			err,
			"gameap service configuration file %s not found",
			systemdConfigPath,
		)
	}
	if err != nil {
		return errors.WithMessage(err, "failed to stat gameap service configuration")
	}
	err = service.Restart(ctx, "gameap")
	if err != nil {
		return errors.WithMessage(err, "failed to restart gameap")
	}

	return nil
}

func restartProcess(ctx context.Context) error {
	p, err := oscore.FindProcessByName(ctx, processName)
	if err != nil {
		return errors.WithMessage(err, "failed to find gameap process")
	}
	if p != nil {
		err := oscore.TerminateAndKillProcess(ctx, p)
		if err != nil {
			return errors.WithMessage(err, "failed to terminate/kill gameap process")
		}
	}

	err = startFork(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to start gameap")
	}

	return nil
}
