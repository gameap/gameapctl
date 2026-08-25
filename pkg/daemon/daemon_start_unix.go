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
	"github.com/gameap/gameapctl/pkg/runhelper"
	"github.com/gameap/gameapctl/pkg/systemd"
	"github.com/pkg/errors"
)

const daemonServiceName = "gameap-daemon"

func Start(ctx context.Context, opts ...Options) error {
	o := firstOptions(opts)

	if o.scope() == gameap.ScopeUser {
		return startDaemonSystemdScope(ctx, gameap.ScopeUser, o.WorkPath)
	}

	init, err := runhelper.DetectInit(ctx)
	if err != nil {
		log.Println("Failed to detect init:", err)
	}

	switch init {
	case runhelper.InitSystemd:
		err = startDaemonSystemdScope(ctx, gameap.ScopeSystem, o.WorkPath)
	case runhelper.InitUnknown:
		err = startDaemonFork(ctx, o)
	}

	if err != nil {
		return errors.WithMessage(err, "failed to start daemon")
	}

	return nil
}

func startDaemonSystemdScope(ctx context.Context, scope, workPath string) error {
	paths, err := gameap.DaemonPathsForScopeWithWorkPath(scope, workPath)
	if err != nil {
		return errors.WithMessage(err, "failed to resolve daemon paths")
	}

	outdated, err := daemonUnitOutdated(paths)
	if err != nil {
		return err
	}

	if outdated {
		if cfgErr := daemonConfigureSystemd(ctx, paths); cfgErr != nil {
			return cfgErr
		}
	}

	if err := systemd.Start(ctx, paths.Scope, daemonServiceName); err != nil {
		return errors.WithMessage(err, "failed to start gameap-daemon")
	}

	return nil
}

// daemonUnitOutdated reports whether the unit file is missing or no longer
// matches the resolved paths. Without it a changed work path would be applied
// everywhere but the WorkingDirectory of an already installed unit.
func daemonUnitOutdated(paths gameap.DaemonPaths) (bool, error) {
	current, err := os.ReadFile(paths.SystemdUnitPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return true, nil
		}

		return false, errors.Wrap(err, "failed to read gameap-daemon service configuration")
	}

	return string(current) != renderDaemonUnit(paths), nil
}

func daemonConfigureSystemd(ctx context.Context, paths gameap.DaemonPaths) error {
	return systemd.InstallUnit(ctx, paths.Scope, paths.SystemdUnitPath, []byte(renderDaemonUnit(paths)))
}

func renderDaemonUnit(paths gameap.DaemonPaths) string {
	var b strings.Builder

	b.WriteString("[Unit]\n")
	b.WriteString("Description=GameAP Daemon\n\n")
	// network targets exist only in the system manager; in a user unit they
	// reference nothing and provide no ordering
	if paths.Scope == gameap.ScopeSystem {
		b.WriteString("Wants=network-online.target\n")
		b.WriteString("After=network.target network-online.target\n\n")
	}

	b.WriteString("[Service]\n")
	if paths.Scope == gameap.ScopeSystem {
		b.WriteString("User=root\n")
	}
	fmt.Fprintf(&b, "WorkingDirectory=%s\n", escapeSystemdSpecifiers(paths.WorkPath))
	fmt.Fprintf(&b, "ExecStart=/bin/bash -c '%s -c %s'\n",
		escapeSystemdSpecifiers(paths.DaemonFilePath),
		escapeSystemdSpecifiers(paths.DaemonConfigFilePath))
	b.WriteString("Restart=always\n\n")

	b.WriteString("[Install]\n")
	if paths.Scope == gameap.ScopeUser {
		b.WriteString("WantedBy=default.target\n")
	} else {
		b.WriteString("WantedBy=multi-user.target\n")
	}

	return b.String()
}

// systemd expands % specifiers in unit directives, so a literal percent sign
// in a path has to be doubled.
func escapeSystemdSpecifiers(s string) string {
	return strings.ReplaceAll(s, "%", "%%")
}

type daemonAlreadyRunningError int

func (e daemonAlreadyRunningError) Error() string {
	return fmt.Sprintf("daemon is already running with pid %d", e)
}

func startDaemonFork(ctx context.Context, o Options) error {
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

	workPath := o.workPath()

	if _, err := os.Stat(workPath); errors.Is(err, fs.ErrNotExist) {
		err := os.MkdirAll(workPath, 0755)
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
		Dir: workPath,
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
