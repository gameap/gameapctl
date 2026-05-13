//go:build linux || darwin

package daemon

import (
	"context"
	"io/fs"
	"log"
	"os"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/runhelper"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
)

func Restart(ctx context.Context, opts ...Options) error {
	o := firstOptions(opts)

	if o.scope() == gameap.ScopeUser {
		return restartDaemonSystemdScope(ctx, gameap.ScopeUser)
	}

	init, err := runhelper.DetectInit(ctx)
	if err != nil {
		log.Println("Failed to detect init:", err)
	}

	switch init {
	case runhelper.InitSystemd:
		err = restartDaemonSystemdScope(ctx, gameap.ScopeSystem)
	case runhelper.InitUnknown:
		err = restartDaemonProcess(ctx)
	}

	return err
}

func restartDaemonSystemdScope(ctx context.Context, scope string) error {
	paths, err := gameap.DaemonPathsForScope(scope)
	if err != nil {
		return errors.WithMessage(err, "failed to resolve daemon paths")
	}

	_, statErr := os.Stat(paths.SystemdUnitPath)
	if statErr != nil && errors.Is(statErr, fs.ErrNotExist) {
		return errors.WithMessagef(
			statErr,
			"daemon service configuration file %s not found",
			paths.SystemdUnitPath,
		)
	}
	if statErr != nil {
		return errors.WithMessage(statErr, "failed to stat gameap-daemon service configuration")
	}

	if err := restartSystemdService(ctx, paths.Scope); err != nil {
		return errors.WithMessage(err, "failed to restart gameap-daemon")
	}

	return nil
}

func restartSystemdService(ctx context.Context, scope string) error {
	if scope == gameap.ScopeUser {
		return oscore.ExecCommand(ctx, "systemctl", "--user", "restart", daemonServiceName)
	}

	return service.Restart(ctx, daemonServiceName)
}

func restartDaemonProcess(ctx context.Context) error {
	p, err := FindProcess(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to find daemon process")
	}
	if p != nil {
		err := oscore.TerminateAndKillProcess(ctx, p)
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
