//go:build windows
// +build windows

package service

import (
	"context"
	"log"
	"os/exec"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/gopherclass/go-shellquote"
	"github.com/pkg/errors"
)

type Windows struct{}

func NewWindows() *Windows {
	return &Windows{}
}

var aliases = map[string][]string{
	"mysql": {"mariadb"},
}

var commands = map[string]struct {
	Start string
	Stop  string
}{
	"mysql": {
		Start: "mysqld",
		Stop:  "mysqladmin −u root shutdown",
	},
}

func (s *Windows) Start(ctx context.Context, serviceName string) error {
	err := s.start(ctx, serviceName)
	c, commandExists := commands[serviceName]
	a, aliasesExists := aliases[serviceName]
	if err != nil && !aliasesExists && !commandExists {
		return err
	}

	for _, alias := range a {
		err = s.start(ctx, alias)
		if err == nil {
			return nil
		}
	}

	if err == nil {
		return nil
	}

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
