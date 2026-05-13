//go:build linux || darwin

package daemon

import (
	"context"
	"io/fs"
	"log"
	"os"
	"time"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/runhelper"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
)

const (
	defaultTerminateWaitTimeout = 30 * time.Second
)

func Stop(ctx context.Context, opts ...Options) error {
	o := firstOptions(opts)

	if o.scope() == gameap.ScopeUser {
		return stopDaemonSystemdScope(ctx, gameap.ScopeUser)
	}

	init, err := runhelper.DetectInit(ctx)
	if err != nil {
		log.Println("Failed to detect init:", err)
	}

	switch init {
	case runhelper.InitSystemd:
		err = stopDaemonSystemdScope(ctx, gameap.ScopeSystem)
	case runhelper.InitUnknown:
		err = stopDaemonProcess(ctx)
	}

	if err != nil {
		return errors.WithMessage(err, "failed to stop daemon")
	}

	return nil
}

func stopDaemonSystemdScope(ctx context.Context, scope string) error {
	paths, err := gameap.DaemonPathsForScope(scope)
	if err != nil {
		return errors.WithMessage(err, "failed to resolve daemon paths")
	}

	_, statErr := os.Stat(paths.SystemdUnitPath)
	if statErr != nil && errors.Is(statErr, fs.ErrNotExist) {
		log.Printf(
			"gameap-daemon systemd configuration file %s not found, nothing to stop\n",
			paths.SystemdUnitPath,
		)

		return nil
	}
	if statErr != nil {
		return errors.WithMessage(statErr, "failed to stat gameap-daemon service configuration")
	}

	if err := stopSystemdService(ctx, paths.Scope); err != nil {
		return errors.WithMessage(err, "failed to stop gameap-daemon")
	}

	return nil
}

func stopSystemdService(ctx context.Context, scope string) error {
	if scope == gameap.ScopeUser {
		return oscore.ExecCommand(ctx, "systemctl", "--user", "stop", daemonServiceName)
	}

	return service.Stop(ctx, daemonServiceName)
}

func stopDaemonProcess(ctx context.Context) error {
	p, err := FindProcess(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to find daemon process")
	}
	if p == nil {
		log.Println("gameap-daemon process is not running, nothing to stop")

		return nil
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
