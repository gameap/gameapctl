package update

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testActiveState     = "active"
	testActivatingState = "activating"
	testNotFoundState   = "not-found"

	testWaitInterval = time.Millisecond
	testWaitTimeout  = 50 * time.Millisecond
	testLongTimeout  = 10 * time.Second
	testCancelAfter  = 50 * time.Millisecond
	testPromptReturn = time.Second
)

var (
	errTestRefused = errors.New("connection refused")
	errTestCrashed = errors.New("service exited")
)

func Test_waitForHealth(t *testing.T) {
	tests := []struct {
		name string
		// readyOn is the attempt the probe first succeeds on; 0 never.
		readyOn int
		// crashOn is the check crashed first reports on; 0 never.
		crashOn      int
		timeout      time.Duration
		wantError    string
		wantAttempts int
	}{
		{
			name:         "ready_at_once",
			readyOn:      1,
			timeout:      testLongTimeout,
			wantAttempts: 1,
		},
		{
			name:         "ready_after_retries",
			readyOn:      3,
			timeout:      testLongTimeout,
			wantAttempts: 3,
		},
		{
			name:      "timeout_returns_the_last_probe_error",
			timeout:   testWaitTimeout,
			wantError: "GameAP did not answer the health check within 50ms: connection refused",
		},
		{
			name:         "crash_stops_waiting",
			crashOn:      2,
			timeout:      testLongTimeout,
			wantError:    "GameAP stopped while starting (last health check: connection refused): service exited",
			wantAttempts: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempts := 0
			probe := func(context.Context) error {
				attempts++
				if tt.readyOn != 0 && attempts >= tt.readyOn {
					return nil
				}

				return errTestRefused
			}

			checks := 0
			crashed := func(context.Context) error {
				checks++
				if tt.crashOn != 0 && checks >= tt.crashOn {
					return errTestCrashed
				}

				return nil
			}

			started := time.Now()

			err := waitForHealth(context.Background(), probe, crashed, tt.timeout, testWaitInterval)

			assert.Less(t, time.Since(started), testPromptReturn)

			if tt.wantError == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
			}

			if tt.wantAttempts != 0 {
				assert.Equal(t, tt.wantAttempts, attempts)
			}
		})
	}
}

func Test_waitForHealth_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testCancelAfter)
	defer cancel()

	probe := func(context.Context) error {
		return errTestRefused
	}

	started := time.Now()

	err := waitForHealth(ctx, probe, noCrashDetection, testLongTimeout, testWaitInterval)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Contains(t, err.Error(), "waiting for GameAP to start")
	assert.Less(t, time.Since(started), testPromptReturn)
}

func Test_parseSystemdServiceState(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   systemdServiceState
	}{
		{
			name:   "all_properties",
			output: "LoadState=loaded\nActiveState=activating\nNRestarts=2\n",
			want:   systemdServiceState{LoadState: systemdLoadStateLoaded, ActiveState: testActivatingState, NRestarts: 2},
		},
		{
			name:   "systemd_without_restart_counter",
			output: "LoadState=loaded\nActiveState=active\n",
			want:   systemdServiceState{LoadState: systemdLoadStateLoaded, ActiveState: testActiveState},
		},
		{
			name:   "unknown_unit",
			output: "LoadState=not-found\nActiveState=inactive\nNRestarts=0\n",
			want:   systemdServiceState{LoadState: testNotFoundState, ActiveState: systemdActiveStateInactive},
		},
		{
			name:   "noise_is_ignored",
			output: "garbage\nNRestarts=many\nActiveState=active\r\n",
			want:   systemdServiceState{ActiveState: testActiveState},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseSystemdServiceState(tt.output))
		})
	}
}

func Test_serviceCrashed(t *testing.T) {
	baseline := systemdServiceState{LoadState: systemdLoadStateLoaded, ActiveState: testActiveState, NRestarts: 1}

	tests := []struct {
		name      string
		current   systemdServiceState
		wantError string
	}{
		{
			name:    "running",
			current: systemdServiceState{LoadState: systemdLoadStateLoaded, ActiveState: testActiveState, NRestarts: 1},
		},
		{
			name:      "restarted_after_the_start",
			current:   systemdServiceState{LoadState: systemdLoadStateLoaded, ActiveState: testActivatingState, NRestarts: 2},
			wantError: "1 time(s): the GameAP service restarted since it was started",
		},
		{
			name:      "failed",
			current:   systemdServiceState{LoadState: systemdLoadStateLoaded, ActiveState: systemdActiveStateFailed, NRestarts: 1},
			wantError: "state failed: the GameAP service is not running",
		},
		{
			name:      "stopped",
			current:   systemdServiceState{LoadState: systemdLoadStateLoaded, ActiveState: systemdActiveStateInactive, NRestarts: 1},
			wantError: "state inactive: the GameAP service is not running",
		},
		{
			name:    "unit_systemd_no_longer_knows",
			current: systemdServiceState{LoadState: testNotFoundState, ActiveState: systemdActiveStateInactive},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := serviceCrashed(baseline, tt.current)

			if tt.wantError == "" {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
			assert.Equal(t, tt.wantError, err.Error())
		})
	}
}
