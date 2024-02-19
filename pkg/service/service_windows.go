//go:build windows
// +build windows

package service

import (
	"bufio"
	"bytes"
	"context"
	"log"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gameap/gameapctl/pkg/utils"
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
		log.Println(err)
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
		log.Println(err)
	}

	if err == nil {
		return nil
	}
	log.Println(err)

	if commandExists {
		var cmd []string
		cmd, err = shellquote.Split(c.Start)

		if err == nil {
			err = utils.ExecCommand(cmd[0], cmd[1:]...)
			if err == nil {
				return nil
			}
		}
	}

	if err != nil {
		log.Println(err)
	}

	for _, alias := range a {
		ac, aliasCommandExists := commands[alias]
		if !aliasCommandExists {
			continue
		}

		var aliasCmd []string
		aliasCmd, err = shellquote.Split(ac.Start)
		if err != nil {
			err = utils.ExecCommand(aliasCmd[0], aliasCmd[1:]...)
			if err == nil {
				return nil
			}
		}
	}

	return err
}

func (s *Windows) Stop(ctx context.Context, serviceName string) error {
	err := s.stop(ctx, serviceName)
	c, commandExists := commands[serviceName]
	a, aliasesExists := aliases[serviceName]
	if err != nil && !aliasesExists && !commandExists {
		return err
	}

	for _, alias := range a {
		err = s.stop(ctx, alias)
		if err == nil {
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
			err = utils.ExecCommand(cmd[0], cmd[1:]...)
			if err == nil {
				return nil
			}
		}
	}

	if err != nil {
		log.Println(err)
	}

	for _, alias := range a {
		ac, aliasCommandExists := commands[alias]
		if !aliasCommandExists {
			continue
		}

		var aliasCmd []string
		aliasCmd, err = shellquote.Split(ac.Stop)
		if err != nil {
			err = utils.ExecCommand(aliasCmd[0], aliasCmd[1:]...)
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

func (s *Windows) start(ctx context.Context, serviceName string) error {
	svc, err := findService(ctx, serviceName)
	if err != nil || svc == nil {
		return NewNotFoundError(serviceName)
	}

	if svc.State == windowsServiceStateStopped {
		return utils.ExecCommand("sc", "start", serviceName)
	} else {
		log.Printf("Service '%s' is already running\n", serviceName)
	}

	return nil
}

func (s *Windows) stop(ctx context.Context, serviceName string) error {
	svc, err := findService(ctx, serviceName)
	if err != nil || svc == nil {
		return NewNotFoundError(serviceName)
	}

	if svc.State == windowsServiceStateRunning {
		return utils.ExecCommand("sc", "stop", serviceName)
	} else {
		log.Printf("Service '%s' is already stopped\n", serviceName)
	}

	return nil
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
	buf.Grow(10240)
	cmd.Stdout = buf
	cmd.Stderr = log.Writer()

	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	log.Println("\n", cmd.String())
	log.Println(buf.String())

	services, err := parseScQueryex(buf.Bytes())
	if err != nil {
		return nil, err
	}

	for _, winservice := range services {
		if strings.ToLower(winservice.ServiceName) == strings.ToLower(serviceName) {
			return &winservice, nil
		}
	}

	return nil, nil
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
