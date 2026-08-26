package install

import (
	"context"
	"sync"
)

const (
	diagnosticsLogLines = 50
	diagnosticsLogBytes = 16 * 1024
)

var panelDiagnosticsOnce sync.Once

// logPanelStartDiagnosticsOnce writes into the install log why the panel did not
// answer the health check. During one installation the health check runs up to three
// times, the diagnostics are needed only once.
func logPanelStartDiagnosticsOnce(ctx context.Context, state panelInstallStateV4) {
	panelDiagnosticsOnce.Do(func() {
		logPanelStartDiagnostics(ctx, state)
	})
}
