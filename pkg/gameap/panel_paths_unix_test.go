//go:build linux || darwin

package gameap

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPanelPathsForScope_System(t *testing.T) {
	paths, err := PanelPathsForScope(ScopeSystem)
	require.NoError(t, err)

	assert.Equal(t, ScopeSystem, paths.Scope)
	assert.Equal(t, "/etc/gameap", paths.ConfigDir)
	assert.Equal(t, DefaultConfigFilePath, paths.ConfigFilePath)
	assert.Equal(t, DefaultDataPath, paths.DataDir)
	assert.Equal(t, "/var/lib/gameap/files", paths.FilesBasePath)
	assert.Equal(t, DefaultBinaryPath, paths.BinaryPath)
	assert.Equal(t, "/etc/systemd/system/gameap.service", paths.SystemdUnitPath)
	assert.Equal(t, "/etc/systemd/system", paths.SystemdUnitDir)
	assert.Equal(t, "gameap", paths.User)
	assert.Equal(t, "gameap", paths.Group)
}

func TestPanelPathsForScope_EmptyDefaultsToSystem(t *testing.T) {
	paths, err := PanelPathsForScope("")
	require.NoError(t, err)

	assert.Equal(t, ScopeSystem, paths.Scope)
}

func TestPanelPathsForScope_Unknown(t *testing.T) {
	_, err := PanelPathsForScope("nonsense")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown panel scope")
}

func TestUserPanelPathsFromHome(t *testing.T) {
	const home = "/home/tester"

	paths := userPanelPathsFromHome(home)

	assert.Equal(t, ScopeUser, paths.Scope)
	assert.Equal(t, filepath.Join(home, ".config", "gameap"), paths.ConfigDir)
	assert.Equal(t, filepath.Join(home, ".config", "gameap", "config.env"), paths.ConfigFilePath)
	assert.Equal(t, filepath.Join(home, ".local", "share", "gameap"), paths.DataDir)
	assert.Equal(t, filepath.Join(home, ".local", "share", "gameap", "files"), paths.FilesBasePath)
	assert.Equal(t, filepath.Join(home, ".local", "bin", "gameap"), paths.BinaryPath)
	assert.Equal(t, filepath.Join(home, ".config", "systemd", "user", "gameap.service"), paths.SystemdUnitPath)
	assert.Equal(t, filepath.Join(home, ".config", "systemd", "user"), paths.SystemdUnitDir)
	assert.Empty(t, paths.User, "user scope must not create or chown to a dedicated account")
	assert.Empty(t, paths.Group)
}
