//go:build linux || darwin

package panel

import (
	"path/filepath"
	"testing"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyScopeDefaults_System(t *testing.T) {
	cfg, err := applyScopeDefaults(InstallConfig{})
	require.NoError(t, err)

	assert.Equal(t, defaultConfigDir, cfg.ConfigDirectory)
	assert.Equal(t, defaultDataDir, cfg.DataDirectory)
	assert.Equal(t, defaultBinaryPath, cfg.BinaryPath)
	assert.Equal(t, defaultUser, cfg.User)
	assert.Equal(t, defaultGroup, cfg.Group)
}

func TestApplyScopeDefaults_User(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := applyScopeDefaults(InstallConfig{Scope: gameap.ScopeUser})
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(home, ".config", "gameap"), cfg.ConfigDirectory)
	assert.Equal(t, filepath.Join(home, ".local", "share", "gameap"), cfg.DataDirectory)
	assert.Equal(t, filepath.Join(home, ".local", "bin", "gameap"), cfg.BinaryPath)
	assert.Empty(t, cfg.User, "user scope must not create or chown to a dedicated account")
	assert.Empty(t, cfg.Group)
}

func TestApplyScopeDefaults_KeepsExplicitValues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := applyScopeDefaults(InstallConfig{
		Scope:           gameap.ScopeUser,
		ConfigDirectory: "/custom/config",
		DataDirectory:   "/custom/data",
		BinaryPath:      "/custom/bin/gameap",
	})
	require.NoError(t, err)

	assert.Equal(t, "/custom/config", cfg.ConfigDirectory)
	assert.Equal(t, "/custom/data", cfg.DataDirectory)
	assert.Equal(t, "/custom/bin/gameap", cfg.BinaryPath)
}
