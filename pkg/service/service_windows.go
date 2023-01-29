//go:build windows
// +build windows

package service

import (
	"context"
	"log"
	"os/exec"

	"github.com/pkg/errors"
)

type Windows struct{}

func NewWindows() *Windows {
	return &Windows{}
}

var aliases = map[string][]string{
	"mysql": {"mariadb"},
}

func (s *Windows) Start(ctx context.Context, serviceName string) error {
	err := s.start(ctx, serviceName)
	a, exists := aliases[serviceName]
	if err != nil && !exists {
		return err
	}

	for _, alias := range a {
		err = s.start(ctx, alias)
		if err == nil {
			return nil
		}
	}

	return err
}

func (s *Windows) Stop(ctx context.Context, serviceName string) error {
	err := s.stop(ctx, serviceName)
	a, exists := aliases[serviceName]
	if err != nil && !exists {
		return err
	}

	for _, alias := range a {
		err = s.stop(ctx, alias)
		if err == nil {
			return nil
		}
	}

	return err
}

func (s *Windows) Restart(_ context.Context, _ string) error {
	return errors.New("use stop and start ins")
}

func (s *Windows) start(_ context.Context, serviceName string) error {
	cmd := exec.Command("sc", "start", serviceName)
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	log.Println('\n', cmd.String())
	return cmd.Run()
}

func (s *Windows) stop(_ context.Context, serviceName string) error {
	cmd := exec.Command("sc", "start", serviceName)
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	log.Println('\n', cmd.String())
	return cmd.Run()
}
