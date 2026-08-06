//go:build linux || darwin

package systemd

import (
	"testing"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/stretchr/testify/assert"
)

func TestArgs_System(t *testing.T) {
	got := Args(gameap.ScopeSystem, "daemon-reload")
	assert.Equal(t, []string{"daemon-reload"}, got)
}

func TestArgs_User(t *testing.T) {
	got := Args(gameap.ScopeUser, "enable", "gameap-daemon")
	assert.Equal(t, []string{"--user", "enable", "gameap-daemon"}, got)
}

func TestCheckUserManager_NoRuntimeDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")

	err := CheckUserManager()
	assert.ErrorContains(t, err, "XDG_RUNTIME_DIR is not set")
}

func TestCheckUserManager_NoUserManager(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	err := CheckUserManager()
	assert.ErrorContains(t, err, "systemd user manager is not running")
}
