package panel

import (
	"context"
	"io/fs"
	"log"
	"os"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/runhelper"
	"github.com/gameap/gameapctl/pkg/systemd"
	"github.com/pkg/errors"
)

func Restart(ctx context.Context, opts ...Options) error {
	o := firstOptions(opts)

	if o.scope() == gameap.ScopeUser {
		return restartPanelSystemdScope(ctx, gameap.ScopeUser)
	}

	init, err := runhelper.DetectInit(ctx)
	if err != nil {
		log.Println("Failed to detect init:", err)
	}

	switch init {
	case runhelper.InitSystemd:
		err = restartPanelSystemdScope(ctx, gameap.ScopeSystem)
	case runhelper.InitUnknown:
		err = restartProcess(ctx, gameap.ScopeSystem)
	}

	return err
}

func restartPanelSystemdScope(ctx context.Context, scope string) error {
	paths, err := gameap.PanelPathsForScope(scope)
	if err != nil {
		return errors.WithMessage(err, "failed to resolve panel paths")
	}

	_, statErr := os.Stat(paths.SystemdUnitPath)
	if statErr != nil && errors.Is(statErr, fs.ErrNotExist) {
		return errors.WithMessagef(
			statErr,
			"gameap service configuration file %s not found",
			paths.SystemdUnitPath,
		)
	}
	if statErr != nil {
		return errors.WithMessage(statErr, "failed to stat gameap service configuration")
	}

	err = systemd.Restart(ctx, paths.Scope, panelServiceName)
	if err != nil {
		return errors.WithMessage(err, "failed to restart gameap")
	}

	return nil
}

func restartProcess(ctx context.Context, scope string) error {
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

	err = startFork(ctx, scope)
	if err != nil {
		return errors.WithMessage(err, "failed to start gameap")
	}

	return nil
}
