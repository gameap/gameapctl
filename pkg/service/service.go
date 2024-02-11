package service

import (
	"context"
	"log"
	"os/exec"
	"sync"

	contextInternal "github.com/gameap/gameapctl/internal/context"
)

var service Service

type Service interface {
	Start(ctx context.Context, serviceName string) error
	Stop(ctx context.Context, serviceName string) error
	Restart(ctx context.Context, serviceName string) error
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
		log.Println(err)
		err = s.Stop(ctx, serviceName)
		if err != nil {
			log.Println(err)
		}

		return s.Start(ctx, serviceName)
	}

	return nil
}

//nolint:ireturn,nolintlint
func Load(ctx context.Context) (srv Service, err error) {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	once := sync.Once{}
	once.Do(func() {
		switch osInfo.Distribution {
		case "debian", "ubuntu", "centos", "almalinux", "rocky", "fedora", "rhel", "opensuse", "sles", "amzn":
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
		case "windows":
			service = NewWindows()
		default:
			err = NewErrUnsupportedDistribution(osInfo.Distribution)
		}
	})

	srv = service

	return
}

type Systemd struct{}

func NewSystemd() *Systemd {
	return &Systemd{}
}

func (s *Systemd) Start(_ context.Context, serviceName string) error {
	cmd := exec.Command("systemctl", "start", serviceName)
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	log.Println('\n', cmd.String())

	return cmd.Run()
}

func (s *Systemd) Stop(_ context.Context, serviceName string) error {
	cmd := exec.Command("systemctl", "stop", serviceName)
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	log.Println('\n', cmd.String())

	return cmd.Run()
}

func (s *Systemd) Restart(_ context.Context, serviceName string) error {
	cmd := exec.Command("systemctl", "restart", serviceName)
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	log.Println('\n', cmd.String())

	return cmd.Run()
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
