//go:build windows

package service

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gopherclass/go-shellquote"
	"github.com/pkg/errors"
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

func (s *Windows) Status(ctx context.Context, serviceName string) error {
	svc, err := findService(ctx, serviceName)
	if err != nil {
		fmt.Println(errors.WithMessage(err, "failed to find service"))

		return NewNotFoundError(serviceName)
	}
	if svc == nil {
		return NewNotFoundError(serviceName)
	}

	if svc.State != windowsServiceStateRunning && svc.State != windowsServiceStateStartPending {
		return ErrInactiveService
	}

	return nil
}

func (s *Windows) start(ctx context.Context, serviceName string) error {
	svc, err := findService(ctx, serviceName)
	if err != nil {
		fmt.Println(errors.WithMessage(err, "failed to find service"))

		return NewNotFoundError(serviceName)
	}
	if svc == nil {
		return NewNotFoundError(serviceName)
	}

	switch svc.State {
	case windowsServiceStateRunning:
		log.Printf("Service '%s' is already running\n", serviceName)
	case windowsServiceStateStartPending:
		log.Printf("Service '%s' is starting\n", serviceName)

		return s.waitStatus(ctx, serviceName, windowsServiceStateRunning)
	default:
		err = oscore.ExecCommand(ctx, "sc", "start", serviceName)
	}
	if err != nil {
		return err
	}

	svc, err = findService(ctx, serviceName)
	if err != nil {
		return err
	}

	if svc.State != windowsServiceStateStartPending {
		log.Printf("Service '%s' status is starting\n", serviceName)

		return s.waitStatus(ctx, serviceName, windowsServiceStateRunning)
	}

	return nil
}

func (s *Windows) stop(ctx context.Context, serviceName string) error {
	svc, err := findService(ctx, serviceName)
	if err != nil || svc == nil {
		return NewNotFoundError(serviceName)
	}

	switch svc.State {
	case windowsServiceStateRunning, windowsServiceStateStartPending:
		err = oscore.ExecCommand(ctx, "sc", "stop", serviceName)
	case windowsServiceStateStopped:
		log.Printf("Service '%s' is already stopped\n", serviceName)
	case windowsServiceStateStopPending:
		log.Printf("Service '%s' is stopping\n", serviceName)

		return s.waitStatus(ctx, serviceName, windowsServiceStateStopped)
	}
	if err != nil {
		return err
	}

	svc, err = findService(ctx, serviceName)
	if err != nil {
		return err
	}

	if svc.State != windowsServiceStateStartPending {
		log.Printf("Service status '%s' is starting\n", serviceName)

		return s.waitStatus(ctx, serviceName, windowsServiceStateStopped)
	}

	return nil
}

func (s *Windows) waitStatus(ctx context.Context, serviceName string, status windowsServiceState) error {
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

		svc, err := findService(ctx, serviceName)
		if err != nil || svc == nil {
			return NewNotFoundError(serviceName)
		}

		if svc.State == status {
			return nil
		}

		if svc.State != windowsServiceStateStartPending &&
			svc.State != windowsServiceStateStopPending &&
			svc.State != windowsServiceStateContinuePending &&
			svc.State != windowsServiceStatePausePending {
			return errors.WithMessagef(
				errors.New("failed to wait service status, service state is not pending"),
				"current service state: %d", svc.State,
			)
		}

		checksAvailable--
	}

	return errors.New("failed to wait service status")
}

func IsExists(ctx context.Context, serviceName string) bool {
	s, err := findService(ctx, serviceName)
	if err != nil {
		return false
	}

	return s != nil
}

func findService(_ context.Context, serviceName string) (*windowsService, error) {
	cmd := exec.Command("sc", "queryex", "type=service", "state=all")
	buf := &bytes.Buffer{}
	buf.Grow(10240) //nolint:mnd
	cmd.Stdout = buf
	cmd.Stderr = log.Writer()

	err := cmd.Run()
	if err != nil {
		return nil, errors.WithMessage(err, "service query command failed")
	}

	log.Println("\n", cmd.String())

	services, err := parseScQueryex(buf.Bytes())
	if err != nil {
		log.Println(buf.String())

		return nil, err
	}

	serviceNames := make([]string, 0, len(services))
	for _, winservice := range services {
		serviceNames = append(serviceNames, winservice.ServiceName)
	}
	log.Println("Services: ", strings.Join(serviceNames, ", "))

	for _, winservice := range services {
		if strings.EqualFold(winservice.ServiceName, serviceName) {
			return &winservice, nil
		}
	}

	return nil, NewNotFoundError(serviceName)
}

type windowsServiceState int

const (
	windowsServiceStateUnknown         windowsServiceState = 0
	windowsServiceStateStopped         windowsServiceState = 1
	windowsServiceStateStartPending    windowsServiceState = 2
	windowsServiceStateStopPending     windowsServiceState = 3
	windowsServiceStateRunning         windowsServiceState = 4
	windowsServiceStateContinuePending windowsServiceState = 5
	windowsServiceStatePausePending    windowsServiceState = 6
	windowsServiceStatePause           windowsServiceState = 7
)

func parseWindowsServiceState(s string) windowsServiceState {
	statusString := string(s[0])
	result, err := strconv.Atoi(statusString)
	if err != nil {
		log.Println("Failed to parse windows service state")
		log.Println(s)
		log.Println(err)
	}

	return windowsServiceState(result)
}

type windowsService struct {
	ServiceName   string
	DisplayName   string
	Type          string
	State         windowsServiceState
	Win32ExitCode string
	ExitCode      string
	Checkpoint    string
	WaitHint      string
	PID           string
	Flags         string
}

//nolint:unparam
func parseScQueryex(buf []byte) ([]windowsService, error) {
	scanner := bufio.NewScanner(bytes.NewReader(buf))
	services := make([]windowsService, 0)
	var s windowsService

	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 2)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "SERVICE_NAME":
			s.ServiceName = value
		case "DISPLAY_NAME":
			s.DisplayName = value
		case "TYPE":
			s.Type = value
		case "STATE":
			s.State = parseWindowsServiceState(value)
		case "WIN32_EXIT_CODE":
			s.Win32ExitCode = value
		case "SERVICE_EXIT_CODE":
			s.ExitCode = value
		case "CHECKPOINT":
			s.Checkpoint = value
		case "WAIT_HINT":
			s.WaitHint = value
		case "PID":
			s.PID = value
		case "FLAGS":
			s.Flags = value

			services = append(services, s)
			s = windowsService{}
		}
	}

	return services, nil
}
