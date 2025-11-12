//go:build linux || darwin

package daemon

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/runhelper"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	daemonSystemdConfigPath = "/etc/systemd/system/gameap-daemon.service"
)

func Start(ctx context.Context) error {
	init, err := runhelper.DetectInit(ctx)
	if err != nil {
		log.Println("Failed to detect init:", err)
	}

	switch init {
	case runhelper.InitSystemd:
		err = startDaemonSystemd(ctx)
	case runhelper.InitUnknown:
		err = startDaemonFork(ctx)
	}

	if err != nil {
		return errors.WithMessage(err, "failed to start daemon")
	}

	return nil
}

func startDaemonSystemd(ctx context.Context) error {
	_, err := os.Stat(daemonSystemdConfigPath)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		err = daemonConfigureSystemd(ctx)
		if err != nil {
			return err
		}
	}
	if err != nil {
		return errors.WithMessage(err, "failed to stat gameap-daemon service configuration")
	}

	err = service.Start(ctx, "gameap-daemon")
	if err != nil {
		return errors.WithMessage(err, "failed to start gameap-daemon")
	}

	return nil
}

func daemonConfigureSystemd(ctx context.Context) error {
	tempDir, err := os.MkdirTemp("", "gameap-daemon-service")
	if err != nil {
		return errors.WithMessage(err, "failed to create temp dir")
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			log.Println(err)
		}
	}(tempDir)

	downloadURL, err := url.JoinPath(gameap.Repository(), "gameap-daemon/systemd-service.tar.gz")
	if err != nil {
		return errors.WithMessage(err, "failed to create download url")
	}

	log.Println("Downloading systemctl service configuration")
	err = utils.Download(
		ctx,
		downloadURL,
		tempDir,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to download service configuration")
	}

	err = utils.Copy(filepath.Join(tempDir, "gameap-daemon.service"), "/etc/systemd/system/gameap-daemon.service")
	if err != nil {
		return errors.WithMessage(err, "failed to copy service configuration")
	}

	err = utils.ExecCommand("systemctl", "daemon-reload")
	if err != nil {
		return errors.WithMessage(err, "failed to reload systemctl")
	}

	return nil
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

	// Open /dev/null for stdin, stdout, stderr
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
			Setsid: true, // Create a new session and detach from terminal
		},
		Files: []*os.File{devNull, devNull, devNull},
	}
	p, err := os.StartProcess(exePath, []string{}, &attr)
	if err != nil {
		return errors.WithMessage(err, "failed to start process")
	}

	log.Println("Process started with pid", p.Pid)

	// Start a goroutine to wait for the process and reap it when it terminates
	// This prevents zombie processes from accumulating
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
