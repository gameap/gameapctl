//go:build linux || darwin

package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderDaemonUnit_System(t *testing.T) {
	paths := gameap.SystemDaemonPaths()

	unit := renderDaemonUnit(paths)

	assert.Contains(t, unit, "User=root")
	assert.Contains(t, unit, "WorkingDirectory="+paths.WorkPath)
	assert.Contains(t, unit, "ExecStart=/bin/bash -c '"+paths.DaemonFilePath+" -c "+paths.DaemonConfigFilePath+"'")
	assert.Contains(t, unit, "WantedBy=multi-user.target")
	assert.NotContains(t, unit, "WantedBy=default.target")
}

func TestRenderDaemonUnit_User(t *testing.T) {
	const home = "/home/tester"
	paths := gameap.DaemonPaths{
		Scope:                gameap.ScopeUser,
		WorkPath:             filepath.Join(home, "gameap"),
		DaemonFilePath:       filepath.Join(home, ".local", "bin", "gameap-daemon"),
		DaemonConfigFilePath: filepath.Join(home, ".config", "gameap-daemon", "gameap-daemon.yaml"),
	}

	unit := renderDaemonUnit(paths)

	assert.NotContains(t, unit, "User=root")
	assert.NotContains(t, unit, "User=")
	assert.Contains(t, unit, "WorkingDirectory="+paths.WorkPath)
	assert.Contains(t, unit, "ExecStart=/bin/bash -c '"+paths.DaemonFilePath+" -c "+paths.DaemonConfigFilePath+"'")
	assert.Contains(t, unit, "WantedBy=default.target")
	assert.NotContains(t, unit, "WantedBy=multi-user.target")
	assert.NotContains(t, unit, "Wants=network-online.target")
	assert.NotContains(t, unit, "After=network.target network-online.target")
}

func TestRenderDaemonUnit_CustomWorkPath(t *testing.T) {
	const workPath = "/opt/gameap"

	paths, err := gameap.DaemonPathsForScopeWithWorkPath(gameap.ScopeSystem, workPath)
	require.NoError(t, err)

	unit := renderDaemonUnit(paths)

	assert.Contains(t, unit, "WorkingDirectory="+workPath)
}

func TestRenderDaemonUnit_WorkPathWithPercent(t *testing.T) {
	const workPath = "/opt/game%p"

	paths, err := gameap.DaemonPathsForScopeWithWorkPath(gameap.ScopeSystem, workPath)
	require.NoError(t, err)

	unit := renderDaemonUnit(paths)

	assert.Contains(t, unit, "WorkingDirectory=/opt/game%%p\n")
	assert.NotContains(t, unit, "WorkingDirectory="+workPath+"\n")
}

func TestDaemonUnitOutdated_UnitMissing(t *testing.T) {
	paths := gameap.SystemDaemonPaths()
	paths.SystemdUnitPath = filepath.Join(t.TempDir(), "gameap-daemon.service")

	outdated, err := daemonUnitOutdated(paths)

	require.NoError(t, err)
	assert.True(t, outdated)
}

func TestDaemonUnitOutdated_UnitUpToDate(t *testing.T) {
	paths := gameap.SystemDaemonPaths()
	paths.SystemdUnitPath = filepath.Join(t.TempDir(), "gameap-daemon.service")
	require.NoError(t, os.WriteFile(paths.SystemdUnitPath, []byte(renderDaemonUnit(paths)), 0600))

	outdated, err := daemonUnitOutdated(paths)

	require.NoError(t, err)
	assert.False(t, outdated)
}

func TestDaemonUnitOutdated_WorkPathChanged(t *testing.T) {
	const workPath = "/opt/gameap-custom"

	installed := gameap.SystemDaemonPaths()
	installed.SystemdUnitPath = filepath.Join(t.TempDir(), "gameap-daemon.service")
	require.NoError(t, os.WriteFile(installed.SystemdUnitPath, []byte(renderDaemonUnit(installed)), 0600))

	requested, err := gameap.DaemonPathsForScopeWithWorkPath(gameap.ScopeSystem, workPath)
	require.NoError(t, err)
	requested.SystemdUnitPath = installed.SystemdUnitPath

	outdated, err := daemonUnitOutdated(requested)
	require.NoError(t, err)
	assert.True(t, outdated)

	assert.Contains(t, renderDaemonUnit(requested), "WorkingDirectory="+workPath+"\n")
}
