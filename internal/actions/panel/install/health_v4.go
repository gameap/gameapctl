package install

import (
	"context"
	"net"
	"time"

	panelpkg "github.com/gameap/gameapctl/internal/pkg/panel"
	"github.com/pkg/errors"
)

const (
	healthCheckRetries  = 30
	healthCheckInterval = 2 * time.Second
	httpClientTimeout   = 5 * time.Second
	panelProbeTimeout   = 2 * time.Second
)

var errPanelNotReady = errors.New("GameAP panel failed to become ready in time")

// checkPanelHealth asks /api/health of the panel once. Unlike the SPA fallback, which
// answers any path with 200, the endpoint reports the panel and its database as ready.
func checkPanelHealth(ctx context.Context, host, port string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return panelpkg.CheckInstallationV4(ctx, host, port, false)
}

// waitForPanelHealthCheck polls the panel until it answers, the context is cancelled or
// maxRetries attempts are exhausted. Only the last case is a start failure, so the
// service diagnostics are collected only then.
func waitForPanelHealthCheck(
	ctx context.Context,
	state panelInstallStateV4,
	maxRetries int,
	retryDelay time.Duration,
) error {
	address := net.JoinHostPort(state.Host, state.Port)

	var lastErr error

	for i := 0; i < maxRetries; i++ {
		lastErr = checkPanelHealth(ctx, state.Host, state.Port, httpClientTimeout)
		if lastErr == nil {
			return nil
		}

		if ctx.Err() != nil {
			return errors.Wrapf(ctx.Err(), "%s, %s", errPanelNotReady, address)
		}

		if i == maxRetries-1 {
			break
		}

		select {
		case <-ctx.Done():
			return errors.Wrapf(ctx.Err(), "%s, %s", errPanelNotReady, address)
		case <-time.After(retryDelay):
		}
	}

	logPanelStartDiagnosticsOnce(ctx, state)

	return errors.WithMessagef(lastErr, "%s, %s", errPanelNotReady, address)
}
