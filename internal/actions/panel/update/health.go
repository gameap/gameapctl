package update

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	// healthWaitTimeout bounds how long an updated panel may take to answer. The
	// panel answers only after its migrations ran and every plugin loaded, and the
	// first start of a new release may compile all plugins again.
	healthWaitTimeout  = 5 * time.Minute
	healthWaitInterval = 2 * time.Second
	// healthWaitLogEvery keeps a slow start from writing a log line per attempt.
	healthWaitLogEvery = 10 * time.Second
)

const (
	systemdLoadStateLoaded     = "loaded"
	systemdActiveStateFailed   = "failed"
	systemdActiveStateInactive = "inactive"
)

var (
	errServiceRestarted       = errors.New("the GameAP service restarted since it was started")
	errServiceStopped         = errors.New("the GameAP service is not running")
	errUnexpectedHealthStatus = errors.New("unexpected health check status")
)

// crashDetector reports an error once the panel process is known to be gone and
// nil while it runs or when nothing can be told.
type crashDetector func(ctx context.Context) error

func noCrashDetection(context.Context) error {
	return nil
}

// waitForHealth polls probe until the panel answers. It gives up when the timeout
// passes, the context ends or crashed reports that the panel process is gone, so
// a binary that exits instead of starting is rolled back without waiting for the
// whole timeout. An answer is checked against crashed too: systemd starts a panel
// that exits again, so the process answering may be the restart of one that
// crashed. The timeout bounds the probes and the crash checks as well, so neither
// a panel that accepts the connection and never answers nor a crash check that
// hangs can hold the wait past it.
func waitForHealth(
	ctx context.Context,
	probe func(ctx context.Context) error,
	crashed crashDetector,
	timeout, interval time.Duration,
) error {
	started := time.Now()

	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var lastLogged time.Time

	for {
		err := probe(probeCtx)
		if err == nil {
			if crashErr := crashed(probeCtx); crashErr != nil {
				return errors.WithMessage(crashErr, "GameAP stopped while starting (last health check passed)")
			}

			log.Println("Health check passed!")

			return nil
		}

		if ctx.Err() != nil {
			return errors.Wrap(ctx.Err(), "waiting for GameAP to start")
		}

		if crashErr := crashed(probeCtx); crashErr != nil {
			return errors.WithMessagef(crashErr, "GameAP stopped while starting (last health check: %v)", err)
		}

		elapsed := time.Since(started)
		if elapsed >= timeout {
			return errors.WithMessagef(err, "GameAP did not answer the health check within %s", timeout)
		}

		if lastLogged.IsZero() || time.Since(lastLogged) >= healthWaitLogEvery {
			log.Printf("Waiting for GameAP to start (%s): %v\n", elapsed.Round(time.Second), err)

			lastLogged = time.Now()
		}

		select {
		case <-probeCtx.Done():
			if ctx.Err() != nil {
				return errors.Wrap(ctx.Err(), "waiting for GameAP to start")
			}

			// A probe started past the timeout would report the deadline rather than
			// why the panel does not answer.
			return errors.WithMessagef(err, "GameAP did not answer the health check within %s", timeout)
		case <-time.After(interval):
		}
	}
}

// systemdServiceState is what systemctl show reports about the panel unit.
type systemdServiceState struct {
	LoadState   string
	ActiveState string
	NRestarts   int
}

// parseSystemdServiceState reads the Key=Value lines of systemctl show. A
// property systemd does not report (NRestarts before systemd 235) keeps its zero
// value.
func parseSystemdServiceState(output string) systemdServiceState {
	var state systemdServiceState

	for line := range strings.SplitSeq(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}

		switch key {
		case "LoadState":
			state.LoadState = value
		case "ActiveState":
			state.ActiveState = value
		case "NRestarts":
			if restarts, err := strconv.Atoi(value); err == nil {
				state.NRestarts = restarts
			}
		}
	}

	return state
}

// serviceCrashed compares the unit with its state right after the start. With
// Restart=always a panel that exits is started again, so a restart counts as a
// crash as much as a unit that failed or stopped. A unit systemd does not know
// tells nothing.
func serviceCrashed(baseline, current systemdServiceState) error {
	if current.LoadState != systemdLoadStateLoaded {
		return nil
	}

	if restarts := current.NRestarts - baseline.NRestarts; restarts > 0 {
		return errors.WithMessagef(errServiceRestarted, "%d time(s)", restarts)
	}

	switch current.ActiveState {
	case systemdActiveStateFailed, systemdActiveStateInactive:
		return errors.WithMessagef(errServiceStopped, "state %s", current.ActiveState)
	}

	return nil
}
