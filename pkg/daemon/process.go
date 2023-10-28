package daemon

import (
	"context"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/process"
)

const (
	daemonProcessName = "gameap-daemon"
)

func FindProcess(ctx context.Context) (*process.Process, error) {
	processes, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to load all processes")
	}

	for _, p := range processes {
		name, err := p.NameWithContext(ctx)
		if err != nil {
			continue
		}

		if name == daemonProcessName {
			return p, nil
		}
	}

	return nil, nil //nolint:nilnil
}
