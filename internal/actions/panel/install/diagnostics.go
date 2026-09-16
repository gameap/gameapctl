package install

import (
	"context"
	"sync"

	panelpkg "github.com/gameap/gameapctl/internal/pkg/panel"
)

var panelDiagnosticsOnce sync.Once

// logPanelStartDiagnosticsOnce writes into the install log why the panel did not
// answer the health check. During one installation the health check runs up to three
// times, the diagnostics are needed only once.
func logPanelStartDiagnosticsOnce(ctx context.Context, state panelInstallStateV4) {
	panelDiagnosticsOnce.Do(func() {
		panelpkg.LogStartDiagnostics(ctx, state.Scope)
	})
}
