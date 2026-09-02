package install

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	healthOKBody = `{"status":"ok"}`

	testRetryDelay     = time.Millisecond
	testLongRetryDelay = 10 * time.Second
	testCancelAfter    = 50 * time.Millisecond
	testPromptReturn   = time.Second
)

// servePanelHealth runs a fake panel and returns the port it listens on (127.0.0.1).
func servePanelHealth(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	_, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	require.NoError(t, err)

	return port
}

// panelHealthHandler answers like GameAP v4: JSON on /api/health, 404 elsewhere.
func panelHealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/health" {
		http.NotFound(w, r)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(healthOKBody))
}

// spaFallbackHandler answers every path with index.html, as the panel does for unknown routes.
func spaFallbackHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<!DOCTYPE html><html><body>GameAP</body></html>"))
}

func unavailableHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusServiceUnavailable)
}

func localPanelState(port string) panelInstallStateV4 {
	return panelInstallStateV4{Host: "127.0.0.1", Port: port}
}

func Test_waitForPanelHealthCheck_Succeeds(t *testing.T) {
	port := servePanelHealth(t, panelHealthHandler)

	err := waitForPanelHealthCheck(context.Background(), localPanelState(port), 1, testRetryDelay)

	require.NoError(t, err)
}

func Test_waitForPanelHealthCheck_RetriesUntilReady(t *testing.T) {
	var mu sync.Mutex
	calls := 0

	port := servePanelHealth(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		attempt := calls
		mu.Unlock()

		if attempt == 1 {
			unavailableHandler(w, r)

			return
		}

		panelHealthHandler(w, r)
	})

	err := waitForPanelHealthCheck(context.Background(), localPanelState(port), healthCheckRetries, testRetryDelay)

	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 2, calls)
}

func Test_waitForPanelHealthCheck_RejectsSPAFallback(t *testing.T) {
	port := servePanelHealth(t, spaFallbackHandler)

	err := waitForPanelHealthCheck(context.Background(), localPanelState(port), 1, testRetryDelay)

	require.ErrorContains(t, err, errPanelNotReady.Error())
}

func Test_waitForPanelHealthCheck_ReturnsPromptlyWithCancelledContext(t *testing.T) {
	port := servePanelHealth(t, unavailableHandler)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	started := time.Now()
	err := waitForPanelHealthCheck(ctx, localPanelState(port), healthCheckRetries, healthCheckInterval)

	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, time.Since(started), testPromptReturn)
}

func Test_waitForPanelHealthCheck_StopsWaitingWhenContextIsCancelled(t *testing.T) {
	port := servePanelHealth(t, unavailableHandler)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	time.AfterFunc(testCancelAfter, cancel)

	started := time.Now()
	err := waitForPanelHealthCheck(ctx, localPanelState(port), healthCheckRetries, testLongRetryDelay)

	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, time.Since(started), testPromptReturn)
}
