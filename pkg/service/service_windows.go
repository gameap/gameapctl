//go:build windows

package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/shellquote"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type Windows struct{}

func NewWindows() *Windows {
	return &Windows{}
}

var aliases = map[string][]string{
	"mysql": {"mariadb", "mysql57", "mysql80"},
}

var commands = map[string]struct {
	Start string
	Stop  string
}{}

func (s *Windows) Start(ctx context.Context, serviceName string) error {
	err := s.start(ctx, serviceName)
	if err != nil {
		if strings.Contains(err.Error(), "already running") {
			return nil
		}

		log.Println(errors.WithMessage(err, "failed to start service"))
	} else {
		return nil
	}

	c, commandExists := commands[serviceName]
	a, aliasesExists := aliases[serviceName]
	if err != nil && !aliasesExists && !commandExists {
		log.Println(err)

		return err
	}

	for _, alias := range a {
		err = s.start(ctx, alias)
		if err == nil {
			return nil
		}
		log.Println(errors.WithMessagef(err, "failed to start alias %s for service %s", alias, serviceName))
	}

	if err != nil {
		log.Println(err)
	}
	if err == nil {
		return nil
	}

	//nolint:nestif
	if commandExists {
		var cmd []string
		cmd, err = shellquote.Split(c.Start)
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to split command"))
		} else {
			err = oscore.ExecCommand(ctx, cmd[0], cmd[1:]...)
			if err != nil {
				log.Println(errors.WithMessage(err, "failed to exec command"))
			} else {
				return nil
			}
		}
	}

	for _, alias := range a {
		ac, aliasCommandExists := commands[alias]
		if !aliasCommandExists {
			continue
		}

		var aliasCmd []string
		aliasCmd, err = shellquote.Split(ac.Start)
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to split alias command"))
		} else {
			err = oscore.ExecCommand(ctx, aliasCmd[0], aliasCmd[1:]...)
			if err != nil {
				log.Println(errors.WithMessage(err, "failed to exec command"))
			} else {
				return nil
			}
		}
	}

	return err
}

func (s *Windows) Stop(ctx context.Context, serviceName string) error {
	err := s.stop(ctx, serviceName)
	if err != nil {
		if strings.Contains(err.Error(), "already running") {
			return nil
		}
		log.Println(errors.WithMessage(err, "failed to stop service"))
	} else {
		return nil
	}

	c, commandExists := commands[serviceName]
	a, aliasesExists := aliases[serviceName]
	if err != nil && !aliasesExists && !commandExists {
		return err
	}

	for _, alias := range a {
		err = s.stop(ctx, alias)
		if err != nil {
			log.Println(errors.WithMessagef(err, "failed to stop alias %s for service %s", alias, serviceName))
		} else {
			return nil
		}
	}

	if err == nil {
		return nil
	}

	if commandExists {
		var cmd []string
		cmd, err = shellquote.Split(c.Stop)

		if err == nil {
			err = oscore.ExecCommand(ctx, cmd[0], cmd[1:]...)
			if err == nil {
				return nil
			}
		}
	}

	if err != nil {
		log.Println(errors.WithMessage(err, "failed to stop service"))
	}

	for _, alias := range a {
		ac, aliasCommandExists := commands[alias]
		if !aliasCommandExists {
			continue
		}

		var aliasCmd []string
		aliasCmd, err = shellquote.Split(ac.Stop)
		if err != nil {
			err = oscore.ExecCommand(ctx, aliasCmd[0], aliasCmd[1:]...)
			if err == nil {
				return nil
			}
		}
	}

	return err
}

func (s *Windows) Restart(_ context.Context, _ string) error {
	return errors.New("use stop and start instead of restart")
}

func (s *Windows) Status(_ context.Context, serviceName string) error {
	state, err := findService(serviceName)
	if err != nil {
		fmt.Println(errors.WithMessage(err, "failed to find service"))

		return NewNotFoundError(serviceName)
	}
	if state == 0 {
		return NewNotFoundError(serviceName)
	}

	if state != svc.Running && state != svc.StartPending {
		return ErrInactiveService
	}

	return nil
}

func (s *Windows) start(ctx context.Context, serviceName string) error {
	state, err := findService(serviceName)
	if err != nil {
		fmt.Println(errors.WithMessage(err, "failed to find service"))

		return NewNotFoundError(serviceName)
	}
	if state == 0 {
		return NewNotFoundError(serviceName)
	}

	switch state {
	case svc.Running:
		log.Printf("Service '%s' is already running\n", serviceName)
	case svc.StartPending:
		log.Printf("Service '%s' is starting\n", serviceName)

		return s.waitStatus(ctx, serviceName, svc.Running)
	default:
		err = oscore.ExecCommand(ctx, "sc", "start", serviceName)
	}
	if err != nil {
		return err
	}

	state, err = findService(serviceName)
	if err != nil {
		return err
	}

	if state != svc.StartPending {
		log.Printf("Service '%s' status is starting\n", serviceName)

		return s.waitStatus(ctx, serviceName, svc.Running)
	}

	return nil
}

func (s *Windows) stop(ctx context.Context, serviceName string) error {
	state, err := findService(serviceName)
	if err != nil || state == 0 {
		return NewNotFoundError(serviceName)
	}

	switch state {
	case svc.Running, svc.StartPending:
		err = oscore.ExecCommand(ctx, "sc", "stop", serviceName)
	case svc.Stopped:
		log.Printf("Service '%s' is already stopped\n", serviceName)
	case svc.StopPending:
		log.Printf("Service '%s' is stopping\n", serviceName)

		return s.waitStatus(ctx, serviceName, svc.Stopped)
	}
	if err != nil {
		return err
	}

	state, err = findService(serviceName)
	if err != nil {
		return err
	}

	if state != svc.StartPending {
		log.Printf("Service status '%s' is starting\n", serviceName)

		return s.waitStatus(ctx, serviceName, svc.Stopped)
	}

	return nil
}

func (s *Windows) waitStatus(ctx context.Context, serviceName string, status svc.State) error {
	log.Println("Waiting for service status")

	t := time.NewTicker(5 * time.Second) //nolint:mnd
	defer func() {
		t.Stop()
	}()

	checksAvailable := 15

	for checksAvailable > 0 {
		select {
		case <-t.C:
		case <-ctx.Done():
			return ctx.Err()
		}

		state, err := findService(serviceName)
		if err != nil || state == 0 {
			return NewNotFoundError(serviceName)
		}

		if state == status {
			return nil
		}

		if state != svc.StartPending &&
			state != svc.StopPending &&
			state != svc.ContinuePending &&
			state != svc.PausePending {
			return errors.WithMessagef(
				errors.New("failed to wait service status, service state is not pending"),
				"current service state: %d", state,
			)
		}

		checksAvailable--
	}

	return errors.New("failed to wait service status")
}

func IsExists(_ context.Context, serviceName string) bool {
	m, err := mgr.Connect()
	if err != nil {
		return false
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return false
	}
	s.Close()

	return true
}

func findService(serviceName string) (svc.State, error) {
	m, err := mgr.Connect()
	if err != nil {
		return 0, errors.WithMessage(err, "failed to connect to service manager")
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return 0, nil
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return 0, errors.WithMessage(err, "failed to query service status")
	}

	return status.State, nil
}
