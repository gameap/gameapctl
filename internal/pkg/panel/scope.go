package panel

import (
	"context"
	"log"
	"os"
	"runtime"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/gameap"
	panelsvc "github.com/gameap/gameapctl/pkg/panel"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

// ResolveScope determines which installation the panel commands should act on.
// An explicit flag wins, then the scope recorded at install time, then a probe of
// the file system for installations whose state file was lost.
func ResolveScope(ctx context.Context, flagScope string) (gameap.PanelPaths, error) {
	if flagScope != "" {
		scope, err := gameap.ResolveScope(flagScope)
		if err != nil {
			return gameap.PanelPaths{}, err
		}

		return gameap.PanelPathsForScope(scope)
	}

	if state, err := gameapctl.LoadPanelInstallState(ctx); err == nil && state.Scope != "" {
		return gameap.PanelPathsForScope(state.Scope)
	}

	system, err := gameap.PanelPathsForScope(gameap.ScopeSystem)
	if err != nil {
		return gameap.PanelPaths{}, errors.WithMessage(err, "failed to resolve panel paths")
	}

	// User paths are unavailable on Windows and without a home directory; there is
	// nothing to probe then, so the system scope stands.
	user, userErr := gameap.PanelPathsForScope(gameap.ScopeUser)
	if userErr == nil && detectScopeFromPaths(system, user) == gameap.ScopeUser {
		return user, nil
	}

	warnNonRootSystemScope()

	return system, nil
}

// CheckBinaryInstalled verifies the binary the service unit actually executes.
// Looking it up in PATH is unreliable: ~/.local/bin is frequently missing from it.
func CheckBinaryInstalled(paths gameap.PanelPaths) error {
	if utils.IsFileExists(paths.BinaryPath) {
		return nil
	}

	return errors.WithMessagef(panelsvc.ErrGameAPNotInstalled, "gameap binary not found at %s", paths.BinaryPath)
}

func detectScopeFromPaths(system, user gameap.PanelPaths) string {
	switch {
	case utils.IsFileExists(user.SystemdUnitPath):
		return gameap.ScopeUser
	case utils.IsFileExists(system.SystemdUnitPath):
		return gameap.ScopeSystem
	case utils.IsFileExists(user.BinaryPath) && !utils.IsFileExists(system.BinaryPath):
		return gameap.ScopeUser
	default:
		return gameap.ScopeSystem
	}
}

func warnNonRootSystemScope() {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		return
	}

	log.Println(
		"Warning: acting on a system scope installation as a non-root user, " +
			"service operations will most likely fail. Pass --scope=user for a rootless installation.",
	)
}
