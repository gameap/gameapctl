//go:build linux || darwin

package daemon

import (
	"path/filepath"
	"testing"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/stretchr/testify/assert"
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
}

func TestBuildSystemctlArgs_System(t *testing.T) {
	got := buildSystemctlArgs(gameap.ScopeSystem, "daemon-reload")
	assert.Equal(t, []string{"daemon-reload"}, got)
}

func TestBuildSystemctlArgs_User(t *testing.T) {
	got := buildSystemctlArgs(gameap.ScopeUser, "enable", "gameap-daemon")
	assert.Equal(t, []string{"--user", "enable", "gameap-daemon"}, got)
}
