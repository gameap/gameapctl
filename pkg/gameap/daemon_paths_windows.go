//go:build windows

package gameap

import (
	"path/filepath"

	"github.com/pkg/errors"
)

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
	}
}

func UserDaemonPaths() (DaemonPaths, error) {
	return DaemonPaths{}, errors.New("user scope is not supported on Windows")
}

func DaemonPathsForScope(scope string) (DaemonPaths, error) {
	switch scope {
	case "", ScopeSystem:
		return SystemDaemonPaths(), nil
	case ScopeUser:
		return DaemonPaths{}, errors.New("user scope is not supported on Windows")
	default:
		return DaemonPaths{}, errors.Errorf("unknown daemon scope %q (expected %q or %q)", scope, ScopeSystem, ScopeUser)
	}
}

// applyWorkPath rewrites the fields derived from the work path, keeping the
// rest of the paths (binary, config, certs, logs) untouched.
func applyWorkPath(paths DaemonPaths, dir string) DaemonPaths {
	paths.WorkPath = dir
	paths.SteamCMDPath = filepath.Join(dir, "steamcmd")
	paths.ToolsPath = filepath.Join(dir, "tools")

	return paths
}
