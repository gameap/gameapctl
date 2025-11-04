package oscore

import (
	"context"
	"log"
	"time"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/process"
)

const (
	defaultTerminateWaitTimeout = 30 * time.Second
)

func FindProcessByName(ctx context.Context, processName string) (*process.Process, error) {
	processes, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to load all processes")
	}

	for _, p := range processes {
		name, err := p.NameWithContext(ctx)
		if err != nil {
			continue
		}

		if name == processName {
			return p, nil
		}
	}

	return nil, nil //nolint:nilnil
}

func TerminateAndKillProcess(ctx context.Context, p *process.Process) error {
	processName, err := p.Name()
	if err != nil {
		return errors.WithMessage(err, "failed to get process name")
	}

	err = p.TerminateWithContext(ctx)
	if err != nil {
		return errors.WithMessagef(err, "failed to terminate %s process", processName)
	}

	log.Printf("Waiting for %s process to terminate\n", processName)

	ctxWithTimeout, cancel := context.WithTimeout(ctx, defaultTerminateWaitTimeout)
	defer cancel()
	ticker := time.NewTicker(1 * time.Second)

	for stop := false; !stop; {
		if isRunning, _ := p.IsRunning(); !isRunning {
			return nil
		}

		select {
		case <-ctxWithTimeout.Done():
			stop = true
		case <-ticker.C:
			log.Printf("Process %s still running\n", processName)
		}
	}

	err = p.KillWithContext(ctx)
	if err != nil {
		return errors.WithMessagef(err, "failed to kill %s process", processName)
	}

	return nil
}
