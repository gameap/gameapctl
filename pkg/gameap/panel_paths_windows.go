//go:build windows

package gameap

import (
	"path/filepath"

	"github.com/pkg/errors"
)

const (
	defaultPanelUser  = "gameap"
	defaultPanelGroup = "gameap"
)

func SystemPanelPaths() PanelPaths {
	return PanelPaths{
		Scope:          ScopeSystem,
		ConfigDir:      DefaultWebInstallationPath,
		ConfigFilePath: DefaultConfigFilePath,
		DataDir:        DefaultDataPath,
		FilesBasePath:  filepath.Join(DefaultDataPath, "files"),
		BinaryPath:     DefaultBinaryPath,
		User:           defaultPanelUser,
		Group:          defaultPanelGroup,
	}
}

func UserPanelPaths() (PanelPaths, error) {
	return PanelPaths{}, errors.New("user scope is not supported on Windows")
}

func PanelPathsForScope(scope string) (PanelPaths, error) {
	switch scope {
	case "", ScopeSystem:
		return SystemPanelPaths(), nil
	case ScopeUser:
		return PanelPaths{}, errors.New("user scope is not supported on Windows")
	default:
		return PanelPaths{}, errors.Errorf("unknown panel scope %q (expected %q or %q)", scope, ScopeSystem, ScopeUser)
	}
}
