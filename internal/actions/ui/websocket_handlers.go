package ui

import (
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/daemon"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
)

const (
	serviceStatusActive   = "active"
	serviceStatusInactive = "inactive"
	serviceStatusNotFound = "not found"
)

func cmdHandle(ctx context.Context, w io.Writer, m message) error {
	cmd := strings.Split(m.Value, " ")
	if len(cmd) == 0 {
		return errors.New("empty command")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var args []string
	if len(cmd) > 1 {
		args = cmd[1:]
	}

	switch cmd[0] {
	case "node-info":
		return nodeInfo(ctx, w, args)
	case "service-status":
		return serviceStatus(ctx, w, args)
	case "gameap-install":
		duplicateLogWriter(ctx, w)

		return gameapInstall(ctx, w, args)
	case "gameap-upgrade":
		duplicateLogWriter(ctx, w)

		return gameapUpgrade(ctx, w, args)
	case "daemon-install":
		duplicateLogWriter(ctx, w)

		return daemonInstall(ctx, w, args)
	case "daemon-upgrade":
		duplicateLogWriter(ctx, w)

		return daemonUpgrade(ctx, w, args)
	case "service-command":
		duplicateLogWriter(ctx, w)

		return serviceCommand(ctx, w, args)
	}

	return errors.New("unknown command")
}

func duplicateLogWriter(ctx context.Context, w io.Writer) {
	oldLogWriter := log.Writer()
	mw := io.MultiWriter(w, oldLogWriter)
	log.SetOutput(mw)

	go func() {
		<-ctx.Done()
		log.SetOutput(oldLogWriter)
	}()
}

func nodeInfo(ctx context.Context, w io.Writer, _ []string) error {
	info := contextInternal.OSInfoFromContext(ctx)
	_, _ = w.Write([]byte(info.String()))

	return nil
}

var replaceMap = map[string]string{
	"daemon":       "gameap-daemon",
	"gameapdaemon": "gameap-daemon",
	"mysql":        "mariadb",
	"php":          "php-fpm",
}

func serviceStatus(ctx context.Context, w io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("no service name provided")
	}

	serviceName := strings.ToLower(args[0])

	if replace, ok := replaceMap[serviceName]; ok {
		serviceName = replace
	}

	if serviceName == "gameap" {
		return gameapStatus(ctx, w, args)
	}

	if serviceName == "gameap-daemon" {
		return daemonStatus(ctx, w, args)
	}

	var errNotFound *service.NotFoundError
	err := service.Status(ctx, serviceName)
	if err != nil && errors.Is(err, service.ErrInactiveService) {
		_, _ = w.Write([]byte(serviceStatusInactive))

		return nil
	}
	if err != nil && !errors.As(err, &errNotFound) {
		return errors.WithMessage(err, "failed to get service status")
	}
	if errNotFound != nil {
		_, _ = w.Write([]byte(serviceStatusNotFound))

		//nolint:nilerr
		return nil
	}

	_, _ = w.Write([]byte(serviceStatusActive))

	return nil
}

func gameapStatus(ctx context.Context, w io.Writer, _ []string) error {
	_, err := gameapctl.LoadPanelInstallState(ctx)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to get panel install state"))
		_, _ = w.Write([]byte(serviceStatusNotFound))
	}

	_, _ = w.Write([]byte(serviceStatusActive))

	return nil
}

func daemonStatus(ctx context.Context, w io.Writer, _ []string) error {
	_, err := exec.LookPath("gameap-daemon")
	if err != nil {
		log.Println(errors.WithMessage(err, "gameap-daemon not found"))
		_, _ = w.Write([]byte(serviceStatusNotFound))

		return nil
	}

	err = daemon.Status(ctx)
	if err != nil {
		log.Println(errors.WithMessage(err, "daemon status failed"))
		_, _ = w.Write([]byte(serviceStatusInactive))

		return nil
	}

	_, _ = w.Write([]byte(serviceStatusActive))

	return nil
}

func gameapInstall(_ context.Context, w io.Writer, args []string) error {
	// Testing
	if len(args) == 0 {
		return errors.New("no args")
	}

	ex, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "failed to get executable path")
	}

	exPath := filepath.Dir(ex)

	exArgs := append([]string{"--non-interactive", "panel", "install"}, args...)

	cmd := exec.Command(ex, exArgs...)
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Dir = exPath

	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to execute command")
	}

	return nil
}

func serviceCommand(ctx context.Context, _ io.Writer, args []string) error {
	if len(args) < 2 {
		return errors.New("not enough arguments, should be at least 2: command and service name")
	}

	command := args[0]
	serviceName := strings.ToLower(args[1])

	if replace, ok := replaceMap[serviceName]; ok {
		serviceName = replace
	}

	log.Printf("Service command: %s %s", command, serviceName)

	var err error
	switch command {
	case "start":
		err = service.Start(ctx, serviceName)
	case "stop":
		err = service.Stop(ctx, serviceName)
	case "restart":
		err = service.Restart(ctx, serviceName)
	default:
		return errors.New("unknown command")
	}

	if err != nil {
		return errors.WithMessage(err, "failed to start service")
	}

	return nil
}

func gameapUpgrade(_ context.Context, w io.Writer, _ []string) error {
	ex, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "failed to get executable path")
	}

	exPath := filepath.Dir(ex)

	cmd := exec.Command(ex, "--non-interactive", "panel", "upgrade")
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Dir = exPath

	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to execute command")
	}

	return nil
}

func daemonInstall(_ context.Context, w io.Writer, _ []string) error {
	ex, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "failed to get executable path")
	}

	exPath := filepath.Dir(ex)

	cmd := exec.Command(ex, "--non-interactive", "daemon", "install")
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Dir = exPath

	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to execute command")
	}

	return nil
}

func daemonUpgrade(_ context.Context, w io.Writer, _ []string) error {
	ex, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "failed to get executable path")
	}

	exPath := filepath.Dir(ex)

	cmd := exec.Command(ex, "--non-interactive", "daemon", "upgrade")
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Dir = exPath

	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to execute command")
	}

	return nil
}
