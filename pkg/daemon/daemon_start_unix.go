//go:build linux || darwin
// +build linux darwin

package daemon

import (
	"context"
	"io/fs"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/process"
)

const (
	initUnknown = "unknown"
	initSystemd = "systemd"
)

func Start(ctx context.Context) error {
	init, err := detectInit(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to detect init")
	}

	switch init {
	case initSystemd:
		err = startDaemonSystemd(ctx)
	case initUnknown:
		err = startDaemonFork(ctx)
	}

	if err != nil {
		return errors.WithMessage(err, "failed to start daemon")
	}

	return nil
}

func detectInit(ctx context.Context) (string, error) {
	result := initUnknown

	p, err := process.NewProcessWithContext(ctx, 1)
	if err != nil {
		return "", errors.WithMessage(err, "failed to load process with pid 1")
	}

	exe, err := p.Exe()
	if err != nil {
		return "", errors.WithMessage(err, "failed to get executable path of the process")
	}

	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", errors.WithMessage(err, "failed to evaluate symlink")
	}

	_, filename := filepath.Split(exe)

	switch filename {
	case "systemd":
		log.Println("Detected systemd init")
		result = initSystemd
	default:
		log.Println("Unsupported init:", filename)
	}

	return result, nil
}

func startDaemonSystemd(ctx context.Context) error {
	_, err := os.Stat("/etc/systemd/system/gameap-daemon.service")
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

func startDaemonFork(_ context.Context) error {
	log.Println("Starting daemon (fork)")

	exePath, err := exec.LookPath("gameap-daemon")
	if err != nil {
		return errors.WithMessage(err, "failed to lookup gameap-daemon path")
	}
	log.Println("Found", exePath)

	attr := os.ProcAttr{
		Dir:   "/srv/gameap",
		Env:   os.Environ(),
		Sys:   &syscall.SysProcAttr{Noctty: true},
		Files: []*os.File{os.Stdin, nil, nil},
	}
	p, err := os.StartProcess(exePath, []string{}, &attr)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to start process"))

		attr = os.ProcAttr{
			Dir:   "/srv/gameap",
			Env:   os.Environ(),
			Files: []*os.File{os.Stdin, nil, nil},
		}
		p, err = os.StartProcess(exePath, []string{}, &attr)
		if err != nil {
			return errors.WithMessage(err, "failed to start process")
		}
	}

	log.Println("Process started with pid", p.Pid)
	log.Println("Releasing process")
	err = p.Release()
	if err != nil {
		return errors.WithMessage(err, "failed to release process")
	}

	return nil
}
