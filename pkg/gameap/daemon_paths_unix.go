//go:build linux || darwin

package gameap

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const systemDaemonSystemdUnitPath = "/etc/systemd/system/gameap-daemon.service"

func SystemDaemonPaths() DaemonPaths {
	return DaemonPaths{
		Scope:                ScopeSystem,
		WorkPath:             DefaultWorkPath,
		SteamCMDPath:         DefaultSteamCMDPath,
		ToolsPath:            DefaultToolsPath,
		CertsPath:            DefaultDaemonCertPath,
		DaemonFilePath:       DefaultDaemonFilePath,
		DaemonConfigFilePath: DefaultDaemonConfigFilePath,
		DaemonConfigDir:      filepath.Dir(DefaultDaemonConfigFilePath),
		OutputLogPath:        DefaultOutputLogPath,
		SystemdUnitPath:      systemDaemonSystemdUnitPath,
		SystemdUnitDir:       filepath.Dir(systemDaemonSystemdUnitPath),
	}
}

func UserDaemonPaths() (DaemonPaths, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return DaemonPaths{}, errors.Wrap(err, "failed to detect user home directory")
	}
	if homeDir == "" {
		return DaemonPaths{}, errors.New("empty user home directory")
	}

	return userDaemonPathsFromHome(homeDir), nil
}

func userDaemonPathsFromHome(homeDir string) DaemonPaths {
	workPath := filepath.Join(homeDir, "gameap")
	configDir := filepath.Join(homeDir, ".config", "gameap-daemon")
	systemdUnitDir := filepath.Join(homeDir, ".config", "systemd", "user")
	stateDir := filepath.Join(homeDir, ".local", "state", "gameap-daemon")

	return DaemonPaths{
		Scope:                ScopeUser,
		WorkPath:             workPath,
		SteamCMDPath:         filepath.Join(workPath, "steamcmd"),
		ToolsPath:            workPath,
		CertsPath:            filepath.Join(configDir, "certs"),
		DaemonFilePath:       filepath.Join(homeDir, ".local", "bin", "gameap-daemon"),
		DaemonConfigFilePath: filepath.Join(configDir, "gameap-daemon.yaml"),
		DaemonConfigDir:      configDir,
		OutputLogPath:        filepath.Join(stateDir, "output.log"),
		SystemdUnitPath:      filepath.Join(systemdUnitDir, "gameap-daemon.service"),
		SystemdUnitDir:       systemdUnitDir,
	}
}

func DaemonPathsForScope(scope string) (DaemonPaths, error) {
	switch scope {
	case "", ScopeSystem:
		return SystemDaemonPaths(), nil
	case ScopeUser:
		return UserDaemonPaths()
	default:
		return DaemonPaths{}, errors.Errorf("unknown daemon scope %q (expected %q or %q)", scope, ScopeSystem, ScopeUser)
	}
}
