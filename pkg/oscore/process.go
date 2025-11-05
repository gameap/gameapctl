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
	defaultKillWaitTimeout      = 10 * time.Second
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
	defer ticker.Stop()

	for stop := false; !stop; {
		if isRunning, _ := p.IsRunningWithContext(ctx); !isRunning {
			return nil
		}

		select {
		case <-ctxWithTimeout.Done():
			stop = true
		case <-ticker.C:
			log.Printf("Process %s still running\n", processName)
		}
	}

	log.Printf("Killing %s process\n", processName)

	err = p.KillWithContext(ctx)
	if err != nil {
		return errors.WithMessagef(err, "failed to kill %s process", processName)
	}

	// Check process status to detect zombie processes
	s, err := p.StatusWithContext(ctx)
	if err != nil {
		return errors.WithMessagef(err, "failed to get %s process status", processName)
	}

	if len(s) > 0 {
		log.Printf("Process status: %s\n", s[0])
		// Zombie processes cannot be killed, they need to be reaped by parent
		if s[0] == "Z" || s[0] == "zombie" {
			return errors.Errorf(
				"process %s is in zombie state and cannot be killed (needs to be reaped by parent process)",
				processName,
			)
		}
	}

	isRunning, err := p.IsRunningWithContext(ctx)
	if err != nil {
		return errors.WithMessagef(err, "failed to check if %s process is running", processName)
	}

	if !isRunning {
		log.Printf("Process %s is not running\n", processName)

		return nil
	}

	// Wait for process to die after SIGKILL
	ctxWithTimeout2, cancel2 := context.WithTimeout(ctx, defaultKillWaitTimeout)
	defer cancel2()
	ticker2 := time.NewTicker(1 * time.Second)
	defer ticker2.Stop()

	for stop := false; !stop; {
		if isRunning, _ := p.IsRunningWithContext(ctx); !isRunning {
			return nil
		}

		select {
		case <-ctxWithTimeout2.Done():
			stop = true
		case <-ticker2.C:
			log.Printf("Process %s still running\n", processName)
		}
	}

	// If we reach here, process is still running after kill timeout
	return errors.Errorf(
		"failed to kill %s process: still running after %s timeout",
		processName,
		defaultKillWaitTimeout,
	)
}
