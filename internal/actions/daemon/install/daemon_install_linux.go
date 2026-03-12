//go:build linux

package install

import (
	"context"
	"log"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/process"
)

const (
	processManagerDefault = processManagerTmux
	processManagerSystemD = "systemd"
	processManagerDocker  = "docker"
	processManagerPodman  = "podman"
	processManagerSimple  = "simple"
	processManagerTmux    = "tmux"
)

func defineProcessManager(ctx context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	if state.Config != "" {
		overrides := parseConfigOverrides(state.Config)

		if overrides["process_manager.name"] != "" {
			state.ProcessManager = overrides["process_manager.name"]

			return state, nil
		}
	}

	state.ProcessManager = processManagerDefault
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
		state.ProcessManager = processManagerSystemD

		return state, nil
	}

	return state, nil
}
