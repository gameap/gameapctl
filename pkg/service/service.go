package service

import (
	"context"
	"log"
	"log/slog"
	"os/exec"
	"sync"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/pkg/errors"
)

var (
	once    = sync.Once{}
	service Service
)

type Service interface {
	Start(ctx context.Context, serviceName string) error
	Stop(ctx context.Context, serviceName string) error
	Restart(ctx context.Context, serviceName string) error
	Status(ctx context.Context, serviceName string) error
}

func Start(ctx context.Context, serviceName string) error {
	s, err := Load(ctx)
	if err != nil {
		return err
	}

	return s.Start(ctx, serviceName)
}

func Stop(ctx context.Context, serviceName string) error {
	s, err := Load(ctx)
	if err != nil {
		return err
	}

	return s.Stop(ctx, serviceName)
}

func Restart(ctx context.Context, serviceName string) error {
	s, err := Load(ctx)
	if err != nil {
		return err
	}
	err = s.Restart(ctx, serviceName)
	if err != nil {
		slog.WarnContext(
			ctx,
			"failed to restart",
			slog.String("err", err.Error()),
		)
		err = s.Stop(ctx, serviceName)
		if err != nil {
			slog.WarnContext(
				ctx,
				"failed to stop",
				slog.String("err", err.Error()),
			)
		}

		return s.Start(ctx, serviceName)
	}

	return nil
}

func Status(ctx context.Context, serviceName string) error {
	s, err := Load(ctx)
	if err != nil {
		return err
	}

	return s.Status(ctx, serviceName)
}

//nolint:ireturn,nolintlint
func Load(ctx context.Context) (srv Service, err error) {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	once.Do(func() {
		if osInfo.Distribution == "windows" {
			service = NewWindows()

			return
		}

		_, err = exec.LookPath("service")
		if err == nil {
			service = NewBasic()

			return
		}

		_, err := exec.LookPath("systemctl")
		if err == nil {
			service = NewSystemd()

			return
		}
	})

	if err != nil {
		return nil, err
	}
	if service == nil {
		err = NewErrUnsupportedDistribution(string(osInfo.Distribution))

		return nil, err
	}

	srv = service

	return srv, nil
}

type Systemd struct{}

func NewSystemd() *Systemd {
	return &Systemd{}
}

func (s *Systemd) Start(_ context.Context, serviceName string) error {
	cmd := exec.Command("systemctl", "start", serviceName)
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	slog.Debug(cmd.String())

	return cmd.Run()
}

func (s *Systemd) Stop(_ context.Context, serviceName string) error {
	cmd := exec.Command("systemctl", "stop", serviceName)
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	slog.Debug(cmd.String())

	return cmd.Run()
}

func (s *Systemd) Restart(_ context.Context, serviceName string) error {
	cmd := exec.Command("systemctl", "restart", serviceName)
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	log.Println('\n', cmd.String())

	return cmd.Run()
}

const (
	systemDStatusInactive = 3
	systemDStatusNotFound = 4
)

func (s *Systemd) Status(_ context.Context, serviceName string) error {
	cmd := exec.Command("systemctl", "--no-pager", "status", serviceName)
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	log.Println('\n', cmd.String())

	var exitErr *exec.ExitError
	err := cmd.Run()
	if err != nil && errors.As(err, &exitErr) {
		return errors.WithMessage(err, "service status command failed")
	}
	if exitErr != nil {
		switch exitErr.ExitCode() {
		case systemDStatusInactive:
			return ErrInactiveService
		case systemDStatusNotFound:
			return NewNotFoundError(serviceName)
		default:
			return errors.Wrapf(err, "service status command failed with exit code %d", exitErr.ExitCode())
		}
	}

	return nil
}

type Basic struct{}

func NewBasic() *Basic {
	return &Basic{}
}

func (s *Basic) Start(_ context.Context, serviceName string) error {
	cmd := exec.Command("service", serviceName, "start")
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	log.Println('\n', cmd.String())

	return cmd.Run()
}

func (s *Basic) Stop(_ context.Context, serviceName string) error {
	cmd := exec.Command("service", serviceName, "stop")
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	log.Println('\n', cmd.String())

	return cmd.Run()
}

func (s *Basic) Restart(_ context.Context, serviceName string) error {
	cmd := exec.Command("service", serviceName, "restart")
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	log.Println('\n', cmd.String())

	return cmd.Run()
}

func (s *Basic) Status(_ context.Context, serviceName string) error {
	cmd := exec.Command("service", serviceName, "status")
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	log.Println('\n', cmd.String())

	return cmd.Run()
}
