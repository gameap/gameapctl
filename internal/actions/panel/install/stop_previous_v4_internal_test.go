package install

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/panel"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

// userScopeState points every previous-installation marker into a temporary home.
func userScopeState(t *testing.T) panelInstallStateV4 {
	t.Helper()

	useTemporaryStateDirectory(t)

	return panelInstallStateV4{
		Scope:      gameap.ScopeUser,
		BinaryPath: filepath.Join(t.TempDir(), "gameap"),
	}
}

func Test_stopPreviousPanelV4_SkipsWhenNothingInstalled(t *testing.T) {
	state := userScopeState(t)

	called := false
	stop := func(context.Context, ...panel.Options) error {
		called = true

		return nil
	}

	require.NoError(t, stopPreviousPanelV4(context.Background(), state, stop))
	require.False(t, called)
}

func Test_stopPreviousPanelV4_StopsWhenBinaryExists(t *testing.T) {
	state := userScopeState(t)
	require.NoError(t, os.WriteFile(state.BinaryPath, []byte("binary"), 0600))

	var scopes []string
	stop := func(_ context.Context, opts ...panel.Options) error {
		require.Len(t, opts, 1)
		scopes = append(scopes, opts[0].Scope)

		return nil
	}

	require.NoError(t, stopPreviousPanelV4(context.Background(), state, stop))
	require.Equal(t, []string{gameap.ScopeUser}, scopes)
}

func Test_stopPreviousPanelV4_ToleratesNotRunningPanel(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "not_installed", err: panel.ErrGameAPNotInstalled},
		{name: "inactive_service", err: service.ErrInactiveService},
		{name: "service_not_found", err: service.NewNotFoundError("gameap")},
		{name: "wrapped_not_found", err: errors.WithMessage(service.NewNotFoundError("gameap"), "stop")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := userScopeState(t)
			require.NoError(t, os.WriteFile(state.BinaryPath, []byte("binary"), 0600))

			stop := func(context.Context, ...panel.Options) error {
				return test.err
			}

			require.NoError(t, stopPreviousPanelV4(context.Background(), state, stop))
		})
	}
}

func Test_stopPreviousPanelV4_ReturnsStopError(t *testing.T) {
	state := userScopeState(t)
	require.NoError(t, os.WriteFile(state.BinaryPath, []byte("binary"), 0600))

	stopErr := errors.New("systemctl failed")
	stop := func(context.Context, ...panel.Options) error {
		return stopErr
	}

	err := stopPreviousPanelV4(context.Background(), state, stop)

	require.ErrorIs(t, err, stopErr)
	require.ErrorContains(t, err, "failed to stop GameAP of the previous installation")
}
