//go:build linux
// +build linux

package daemoninstall

import (
	"context"
	"log"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/process"
)

const (
	defaultProcessManager = "tmux"
	systemDProcessManager = "systemd"
)

func configureProcessManager(ctx context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	state.ProcessManager = defaultProcessManager
	// check that pid=1 is systemd
	p, err := process.NewProcess(1)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to get process 1"))
		return state, nil
	}

	name, err := p.NameWithContext(ctx)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to get process name"))

		return state, nil
	}

	if name == "systemd" {
		state.ProcessManager = systemDProcessManager

		return state, nil
	}

	return state, nil
}
