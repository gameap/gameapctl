package gameap

import (
	"path/filepath"
	"strings"
)

// DaemonPathsForScopeWithWorkPath resolves daemon paths for the scope and
// overrides the work path when workPath is not empty. An empty workPath
// returns the same result as DaemonPathsForScope.
func DaemonPathsForScopeWithWorkPath(scope, workPath string) (DaemonPaths, error) {
	paths, err := DaemonPathsForScope(scope)
	if err != nil {
		return DaemonPaths{}, err
	}

	workPath = strings.TrimSpace(workPath)
	if workPath == "" {
		return paths, nil
	}

	return applyWorkPath(paths, filepath.Clean(workPath)), nil
}

type DaemonPaths struct {
	Scope string

	WorkPath     string
	SteamCMDPath string
	ToolsPath    string

	CertsPath            string
	DaemonFilePath       string
	DaemonConfigFilePath string
	DaemonConfigDir      string
	OutputLogPath        string

	SystemdUnitPath string
	SystemdUnitDir  string
}
