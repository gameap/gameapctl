//go:build linux || darwin

package panel

import (
	"path/filepath"
	"testing"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderSystemdUnit_System(t *testing.T) {
	paths := gameap.SystemPanelPaths()

	unit, err := renderSystemdUnit(SystemdUnitConfig{
		Scope:            paths.Scope,
		User:             paths.User,
		Group:            paths.Group,
		WorkingDirectory: paths.DataDir,
		ExecStart:        paths.BinaryPath,
		EnvironmentFile:  paths.ConfigFilePath,
		ReadWritePaths:   paths.DataDir + " " + paths.FilesBasePath,
	})
	require.NoError(t, err)

	rendered := string(unit)
	assert.Contains(t, rendered, "User=gameap")
	assert.Contains(t, rendered, "Group=gameap")
	assert.Contains(t, rendered, "AmbientCapabilities=CAP_NET_BIND_SERVICE")
	assert.Contains(t, rendered, "ProtectHome=true")
	assert.Contains(t, rendered, "Requires=network.target")
	assert.Contains(t, rendered, "ReadWritePaths=/var/lib/gameap /var/lib/gameap/files")
	assert.Contains(t, rendered, "WantedBy=multi-user.target")
}

func TestRenderSystemdUnit_User(t *testing.T) {
	const home = "/home/tester"
	paths := userPanelPathsForTest(home)

	unit, err := renderSystemdUnit(SystemdUnitConfig{
		Scope:            paths.Scope,
		WorkingDirectory: paths.DataDir,
		ExecStart:        paths.BinaryPath,
		EnvironmentFile:  paths.ConfigFilePath,
		ReadWritePaths:   paths.DataDir + " " + paths.FilesBasePath,
	})
	require.NoError(t, err)

	rendered := string(unit)

	for _, forbidden := range []string{
		"User=",
		"Group=",
		"AmbientCapabilities",
		"ProtectHome",
		"ProtectSystem",
		"PrivateTmp",
		"ReadWritePaths",
		"PIDFile",
		"Requires=",
		"multi-user.target",
	} {
		assert.NotContains(t, rendered, forbidden, "directive %q breaks a systemd --user unit", forbidden)
	}

	assert.Contains(t, rendered, "WorkingDirectory="+filepath.Join(home, ".local", "share", "gameap"))
	assert.Contains(t, rendered, "ExecStart="+filepath.Join(home, ".local", "bin", "gameap"))
	assert.Contains(t, rendered, "EnvironmentFile="+filepath.Join(home, ".config", "gameap", "config.env"))
	assert.Contains(t, rendered, "WantedBy=default.target")
}

func userPanelPathsForTest(home string) gameap.PanelPaths {
	configDir := filepath.Join(home, ".config", "gameap")
	dataDir := filepath.Join(home, ".local", "share", "gameap")

	return gameap.PanelPaths{
		Scope:          gameap.ScopeUser,
		ConfigDir:      configDir,
		ConfigFilePath: filepath.Join(configDir, "config.env"),
		DataDir:        dataDir,
		FilesBasePath:  filepath.Join(dataDir, "files"),
		BinaryPath:     filepath.Join(home, ".local", "bin", "gameap"),
	}
}
