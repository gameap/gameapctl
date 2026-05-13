//go:build linux || darwin

package gameap

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemonPathsForScope_System(t *testing.T) {
	paths, err := DaemonPathsForScope(ScopeSystem)
	require.NoError(t, err)

	assert.Equal(t, ScopeSystem, paths.Scope)
	assert.Equal(t, DefaultWorkPath, paths.WorkPath)
	assert.Equal(t, DefaultSteamCMDPath, paths.SteamCMDPath)
	assert.Equal(t, DefaultDaemonFilePath, paths.DaemonFilePath)
	assert.Equal(t, DefaultDaemonConfigFilePath, paths.DaemonConfigFilePath)
	assert.Equal(t, DefaultDaemonCertPath, paths.CertsPath)
	assert.Equal(t, DefaultOutputLogPath, paths.OutputLogPath)
	assert.Equal(t, "/etc/systemd/system/gameap-daemon.service", paths.SystemdUnitPath)
	assert.Equal(t, "/etc/systemd/system", paths.SystemdUnitDir)
}

func TestDaemonPathsForScope_EmptyDefaultsToSystem(t *testing.T) {
	paths, err := DaemonPathsForScope("")
	require.NoError(t, err)

	assert.Equal(t, ScopeSystem, paths.Scope)
}

func TestDaemonPathsForScope_Unknown(t *testing.T) {
	_, err := DaemonPathsForScope("nonsense")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown daemon scope")
}

func TestUserDaemonPathsFromHome(t *testing.T) {
	const home = "/home/tester"

	paths := userDaemonPathsFromHome(home)

	assert.Equal(t, ScopeUser, paths.Scope)
	assert.Equal(t, filepath.Join(home, "gameap"), paths.WorkPath)
	assert.Equal(t, filepath.Join(home, "gameap"), paths.ToolsPath)
	assert.Equal(t, filepath.Join(home, "gameap", "steamcmd"), paths.SteamCMDPath)
	assert.Equal(t, filepath.Join(home, ".local", "bin", "gameap-daemon"), paths.DaemonFilePath)
	assert.Equal(t, filepath.Join(home, ".config", "gameap-daemon"), paths.DaemonConfigDir)
	assert.Equal(t, filepath.Join(home, ".config", "gameap-daemon", "gameap-daemon.yaml"), paths.DaemonConfigFilePath)
	assert.Equal(t, filepath.Join(home, ".config", "gameap-daemon", "certs"), paths.CertsPath)
	assert.Equal(t, filepath.Join(home, ".local", "state", "gameap-daemon", "output.log"), paths.OutputLogPath)
	assert.Equal(t, filepath.Join(home, ".config", "systemd", "user", "gameap-daemon.service"), paths.SystemdUnitPath)
	assert.Equal(t, filepath.Join(home, ".config", "systemd", "user"), paths.SystemdUnitDir)
}
