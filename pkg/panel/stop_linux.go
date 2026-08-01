package panel

import (
	"context"
	"io/fs"
	"log"
	"os"
	"time"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/runhelper"
	"github.com/gameap/gameapctl/pkg/systemd"
	"github.com/pkg/errors"
)

const (
	defaultTerminateWaitTimeout = 30 * time.Second
)

func Stop(ctx context.Context, opts ...Options) error {
	o := firstOptions(opts)

	if o.scope() == gameap.ScopeUser {
		return stopPanelSystemdScope(ctx, gameap.ScopeUser)
	}

	init, err := runhelper.DetectInit(ctx)
	if err != nil {
		log.Println("Failed to detect init:", err)
	}

	switch init {
	case runhelper.InitSystemd:
		err = stopPanelSystemdScope(ctx, gameap.ScopeSystem)
	case runhelper.InitUnknown:
		err = stopProcess(ctx)
	}

	if err != nil {
		return errors.WithMessage(err, "failed to stop gameap")
	}

	return nil
}

func stopPanelSystemdScope(ctx context.Context, scope string) error {
	paths, err := gameap.PanelPathsForScope(scope)
	if err != nil {
		return errors.WithMessage(err, "failed to resolve panel paths")
	}

	_, statErr := os.Stat(paths.SystemdUnitPath)
	if statErr != nil && errors.Is(statErr, fs.ErrNotExist) {
		log.Printf("gameap systemd configuration file %s not found, nothing to stop\n", paths.SystemdUnitPath)

		return nil
	}
	if statErr != nil {
		return errors.WithMessage(statErr, "failed to stat gameap service configuration")
	}

	err = systemd.Stop(ctx, paths.Scope, panelServiceName)
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
		log.Println("gameap process is not running, nothing to stop")

		return nil
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
