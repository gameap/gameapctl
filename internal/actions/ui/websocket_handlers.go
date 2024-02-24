package ui

import (
	"context"
	"io"
	"os/exec"
	"strings"

	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
)

func cmdHandle(ctx context.Context, w io.Writer, m message) error {
	cmd := strings.Split(m.Value, " ")
	if len(cmd) == 0 {
		return errors.New("empty command")
	}

	switch cmd[0] {
	case "os-info":
	// handle some command
	case "service-status":
		return serviceStatus(ctx, w, cmd[1:])
	case "gameap-install":
		return gameapInstall(ctx, w, cmd[1:])
	case "gameap-upgrade":
		return gameapInstall(ctx, w, cmd[1:])
	case "daemon-install":
		return gameapInstall(ctx, w, cmd[1:])
	case "daemon-upgrade":
		return gameapInstall(ctx, w, cmd[1:])
	case "service-command":
		return serviceCommand(ctx, w, cmd[1:])
	}

	return errors.New("unknown command")
}

func serviceStatus(ctx context.Context, w io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("no service name provided")
	}

	_, _ = w.Write([]byte("active"))
	return nil

	serviceName := args[0]

	var errNotFound *service.NotFoundError
	err := service.Status(ctx, serviceName)
	if err != nil && !errors.As(err, &errNotFound) {
		return errors.WithMessage(err, "failed to get service status")
	}
	if errNotFound != nil {
		_, _ = w.Write([]byte("not found"))

		//nolint:nilerr
		return nil
	}
	if err != nil && errors.Is(err, service.ErrInactiveService) {
		_, _ = w.Write([]byte("inactive"))

		return nil
	}

	_, _ = w.Write([]byte("active"))

	return nil
}

func gameapInstall(_ context.Context, w io.Writer, args []string) error {
	// Testing
	if len(args) == 0 {
		return errors.New("no args")
	}

	_, _ = w.Write([]byte(strings.Join(args, " ") + "\n"))

	cmd := exec.Command("ping", "google.com")
	cmd.Stdout = w
	cmd.Stderr = w

	err := cmd.Run()
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
	serviceName := args[1]

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
