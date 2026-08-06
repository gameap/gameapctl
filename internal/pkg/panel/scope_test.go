package panel

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectScopeFromPaths(t *testing.T) {
	const (
		systemUnit   = "system-unit"
		systemBinary = "system-binary"
		userUnit     = "user-unit"
		userBinary   = "user-binary"
	)

	tests := []struct {
		name    string
		present []string
		want    string
	}{
		{name: "nothing installed", present: nil, want: gameap.ScopeSystem},
		{name: "user unit only", present: []string{userUnit}, want: gameap.ScopeUser},
		{name: "system unit only", present: []string{systemUnit}, want: gameap.ScopeSystem},
		{name: "both units", present: []string{systemUnit, userUnit}, want: gameap.ScopeUser},
		{name: "user binary only", present: []string{userBinary}, want: gameap.ScopeUser},
		{name: "both binaries", present: []string{userBinary, systemBinary}, want: gameap.ScopeSystem},
		{name: "system binary only", present: []string{systemBinary}, want: gameap.ScopeSystem},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			system := gameap.PanelPaths{
				Scope:           gameap.ScopeSystem,
				SystemdUnitPath: filepath.Join(dir, systemUnit),
				BinaryPath:      filepath.Join(dir, systemBinary),
			}
			user := gameap.PanelPaths{
				Scope:           gameap.ScopeUser,
				SystemdUnitPath: filepath.Join(dir, userUnit),
				BinaryPath:      filepath.Join(dir, userBinary),
			}

			for _, name := range tt.present {
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0600))
			}

			assert.Equal(t, tt.want, detectScopeFromPaths(system, user))
		})
	}
}

func TestResolveScope_FlagWins(t *testing.T) {
	paths, err := ResolveScope(t.Context(), gameap.ScopeSystem)
	require.NoError(t, err)

	assert.Equal(t, gameap.ScopeSystem, paths.Scope)
}

func TestResolveScope_UserFlagRequiresLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		_, err := ResolveScope(t.Context(), gameap.ScopeUser)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires Linux")

		return
	}

	home := t.TempDir()
	t.Setenv("HOME", home)

	paths, err := ResolveScope(t.Context(), gameap.ScopeUser)
	require.NoError(t, err)

	assert.Equal(t, gameap.ScopeUser, paths.Scope)
	assert.Equal(t, filepath.Join(home, ".config", "gameap", "config.env"), paths.ConfigFilePath)
}

func TestResolveScope_InvalidFlag(t *testing.T) {
	_, err := ResolveScope(t.Context(), "nonsense")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown --scope value")
}

func TestCheckBinaryInstalled(t *testing.T) {
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "gameap")

	err := CheckBinaryInstalled(gameap.PanelPaths{BinaryPath: binaryPath})
	require.Error(t, err)
	assert.Contains(t, err.Error(), binaryPath)

	require.NoError(t, os.WriteFile(binaryPath, []byte("x"), 0700))
	require.NoError(t, CheckBinaryInstalled(gameap.PanelPaths{BinaryPath: binaryPath}))
}
