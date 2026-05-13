//go:build linux || darwin

package daemon

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/runhelper"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
)

const daemonServiceName = "gameap-daemon"

func Start(ctx context.Context, opts ...Options) error {
	o := firstOptions(opts)

	if o.scope() == gameap.ScopeUser {
		return startDaemonSystemdScope(ctx, gameap.ScopeUser)
	}

	init, err := runhelper.DetectInit(ctx)
	if err != nil {
		log.Println("Failed to detect init:", err)
	}

	switch init {
	case runhelper.InitSystemd:
		err = startDaemonSystemdScope(ctx, gameap.ScopeSystem)
	case runhelper.InitUnknown:
		err = startDaemonFork(ctx)
	}

	if err != nil {
		return errors.WithMessage(err, "failed to start daemon")
	}

	return nil
}

func startDaemonSystemdScope(ctx context.Context, scope string) error {
	paths, err := gameap.DaemonPathsForScope(scope)
	if err != nil {
		return errors.WithMessage(err, "failed to resolve daemon paths")
	}

	_, statErr := os.Stat(paths.SystemdUnitPath)
	if statErr != nil && errors.Is(statErr, fs.ErrNotExist) {
		if cfgErr := daemonConfigureSystemd(ctx, paths); cfgErr != nil {
			return cfgErr
		}
	} else if statErr != nil {
		return errors.WithMessage(statErr, "failed to stat gameap-daemon service configuration")
	}

	if err := startSystemdService(ctx, paths.Scope); err != nil {
		return errors.WithMessage(err, "failed to start gameap-daemon")
	}

	return nil
}

func startSystemdService(ctx context.Context, scope string) error {
	if scope == gameap.ScopeUser {
		return oscore.ExecCommand(ctx, "systemctl", "--user", "start", daemonServiceName)
	}

	return service.Start(ctx, daemonServiceName)
}

func daemonConfigureSystemd(ctx context.Context, paths gameap.DaemonPaths) error {
	log.Println("Writing systemd service configuration to", paths.SystemdUnitPath)

	if err := os.MkdirAll(paths.SystemdUnitDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create systemd unit directory")
	}

	//nolint:gosec // systemd unit files must be world-readable
	if err := os.WriteFile(paths.SystemdUnitPath, []byte(renderDaemonUnit(paths)), 0644); err != nil {
		return errors.WithMessage(err, "failed to write service configuration")
	}

	if err := runSystemctl(ctx, paths.Scope, "daemon-reload"); err != nil {
		return errors.WithMessage(err, "failed to reload systemctl")
	}

	if err := runSystemctl(ctx, paths.Scope, "enable", daemonServiceName); err != nil {
		return errors.WithMessage(err, "failed to enable gameap-daemon service")
	}

	if paths.Scope == gameap.ScopeUser {
		enableLinger(ctx)
	}

	return nil
}

func renderDaemonUnit(paths gameap.DaemonPaths) string {
	var b strings.Builder

	b.WriteString("[Unit]\n")
	b.WriteString("Description=GameAP Daemon\n\n")
	b.WriteString("Wants=network-online.target\n")
	b.WriteString("After=network.target network-online.target\n\n")

	b.WriteString("[Service]\n")
	if paths.Scope == gameap.ScopeSystem {
		b.WriteString("User=root\n")
	}
	fmt.Fprintf(&b, "WorkingDirectory=%s\n", paths.WorkPath)
	fmt.Fprintf(&b, "ExecStart=/bin/bash -c '%s -c %s'\n", paths.DaemonFilePath, paths.DaemonConfigFilePath)
	b.WriteString("Restart=always\n\n")

	b.WriteString("[Install]\n")
	if paths.Scope == gameap.ScopeUser {
		b.WriteString("WantedBy=default.target\n")
	} else {
		b.WriteString("WantedBy=multi-user.target\n")
	}

	return b.String()
}

func runSystemctl(ctx context.Context, scope string, args ...string) error {
	return oscore.ExecCommand(ctx, "systemctl", buildSystemctlArgs(scope, args...)...)
}

func buildSystemctlArgs(scope string, args ...string) []string {
	if scope == gameap.ScopeUser {
		out := make([]string, 0, len(args)+1)
		out = append(out, "--user")
		out = append(out, args...)

		return out
	}

	return args
}

type daemonAlreadyRunningError int

func (e daemonAlreadyRunningError) Error() string {
	return fmt.Sprintf("daemon is already running with pid %d", e)
}

func startDaemonFork(ctx context.Context) error {
	log.Println("Starting daemon (fork)")

	daemonProcess, err := FindProcess(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to find daemon process")
	}

	if daemonProcess != nil && daemonProcess.Pid != 0 {
		return daemonAlreadyRunningError(daemonProcess.Pid)
	}

	exePath, err := exec.LookPath("gameap-daemon")
	if err != nil {
		return errors.WithMessage(err, "failed to lookup gameap-daemon path")
	}
	log.Println("Found", exePath)

	if _, err := os.Stat(gameap.DefaultWorkPath); errors.Is(err, fs.ErrNotExist) {
		err := os.Mkdir(gameap.DefaultWorkPath, 0755)
		if err != nil {
			return errors.WithMessage(err, "failed to create work path")
		}
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return errors.WithMessage(err, "failed to open /dev/null")
	}
	defer func(devNull *os.File) {
		err := devNull.Close()
		if err != nil {
			log.Println("Failed to close /dev/null:", err)
		}
	}(devNull)

	attr := os.ProcAttr{
		Dir: gameap.DefaultWorkPath,
		Env: os.Environ(),
		Sys: &syscall.SysProcAttr{
			Setsid: true,
		},
		Files: []*os.File{devNull, devNull, devNull},
	}
	p, err := os.StartProcess(exePath, []string{}, &attr)
	if err != nil {
		return errors.WithMessage(err, "failed to start process")
	}

	log.Println("Process started with pid", p.Pid)

	go func() {
		state, waitErr := p.Wait()
		if waitErr != nil {
			log.Printf("Error waiting for process (pid %d): %v\n", p.Pid, waitErr)

			return
		}
		log.Printf("Process (pid %d) exited with status: %s\n", p.Pid, state.String())
	}()

	return nil
}
