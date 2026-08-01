//go:build linux || darwin

package gameap

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const systemPanelSystemdUnitPath = "/etc/systemd/system/gameap.service"

const (
	defaultPanelUser  = "gameap"
	defaultPanelGroup = "gameap"
)

func SystemPanelPaths() PanelPaths {
	return PanelPaths{
		Scope:           ScopeSystem,
		ConfigDir:       filepath.Dir(DefaultConfigFilePath),
		ConfigFilePath:  DefaultConfigFilePath,
		DataDir:         DefaultDataPath,
		FilesBasePath:   filepath.Join(DefaultDataPath, "files"),
		BinaryPath:      DefaultBinaryPath,
		SystemdUnitPath: systemPanelSystemdUnitPath,
		SystemdUnitDir:  filepath.Dir(systemPanelSystemdUnitPath),
		User:            defaultPanelUser,
		Group:           defaultPanelGroup,
	}
}

func UserPanelPaths() (PanelPaths, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return PanelPaths{}, errors.Wrap(err, "failed to detect user home directory")
	}
	if homeDir == "" {
		return PanelPaths{}, errors.New("empty user home directory")
	}

	return userPanelPathsFromHome(homeDir), nil
}

func userPanelPathsFromHome(homeDir string) PanelPaths {
	configDir := filepath.Join(homeDir, ".config", "gameap")
	dataDir := filepath.Join(homeDir, ".local", "share", "gameap")
	systemdUnitDir := filepath.Join(homeDir, ".config", "systemd", "user")

	return PanelPaths{
		Scope:           ScopeUser,
		ConfigDir:       configDir,
		ConfigFilePath:  filepath.Join(configDir, "config.env"),
		DataDir:         dataDir,
		FilesBasePath:   filepath.Join(dataDir, "files"),
		BinaryPath:      filepath.Join(homeDir, ".local", "bin", "gameap"),
		SystemdUnitPath: filepath.Join(systemdUnitDir, "gameap.service"),
		SystemdUnitDir:  systemdUnitDir,
	}
}

func PanelPathsForScope(scope string) (PanelPaths, error) {
	switch scope {
	case "", ScopeSystem:
		return SystemPanelPaths(), nil
	case ScopeUser:
		return UserPanelPaths()
	default:
		return PanelPaths{}, errors.Errorf("unknown panel scope %q (expected %q or %q)", scope, ScopeSystem, ScopeUser)
	}
}
