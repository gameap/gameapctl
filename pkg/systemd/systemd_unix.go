//go:build linux || darwin

// Package systemd wraps systemctl invocations that must work both against the
// system manager and against a per-user manager (systemctl --user).
package systemd

import (
	"context"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
)

const (
	unitFileMode = 0644
	unitDirMode  = 0755
)

const userManagerHint = "Log in as the target user directly (ssh user@host or machinectl shell user@), " +
	"not via su or sudo"

// Args prepends --user to systemctl arguments in user scope.
func Args(scope string, args ...string) []string {
	if scope == gameap.ScopeUser {
		out := make([]string, 0, len(args)+1)
		out = append(out, "--user")
		out = append(out, args...)

		return out
	}

	return args
}

func Run(ctx context.Context, scope string, args ...string) error {
	if err := ensureUserManager(scope); err != nil {
		return err
	}

	return oscore.ExecCommand(ctx, "systemctl", Args(scope, args...)...)
}

func Start(ctx context.Context, scope, unitName string) error {
	if scope == gameap.ScopeUser {
		resetFailed(ctx, scope, unitName)

		return Run(ctx, scope, "start", unitName)
	}

	return service.Start(ctx, unitName)
}

func Stop(ctx context.Context, scope, unitName string) error {
	if scope == gameap.ScopeUser {
		return Run(ctx, scope, "stop", unitName)
	}

	return service.Stop(ctx, unitName)
}

func Restart(ctx context.Context, scope, unitName string) error {
	if scope == gameap.ScopeUser {
		resetFailed(ctx, scope, unitName)

		return Run(ctx, scope, "restart", unitName)
	}

	return service.Restart(ctx, unitName)
}

// Clears the failed state and the start rate-limit counter, so an explicit
// start is not rejected with "start-limit-hit" after previous crash-loop
// restarts. Best effort: an error for a not-loaded unit is expected.
func resetFailed(ctx context.Context, scope, unitName string) {
	if err := Run(ctx, scope, "reset-failed", unitName); err != nil {
		log.Printf("Failed to reset %s failed state: %v", unitName, err)
	}
}

// InstallUnit writes the unit file, reloads the manager and enables the unit.
// In user scope it also tries to enable lingering, without which the unit is
// stopped on logout and never started at boot.
func InstallUnit(ctx context.Context, scope, unitPath string, content []byte) error {
	if err := ensureUserManager(scope); err != nil {
		return err
	}

	log.Println("Writing systemd service configuration to", unitPath)

	if err := os.MkdirAll(filepath.Dir(unitPath), unitDirMode); err != nil {
		return errors.Wrap(err, "failed to create systemd unit directory")
	}

	if err := os.WriteFile(unitPath, content, unitFileMode); err != nil {
		return errors.Wrap(err, "failed to write service configuration")
	}

	if err := Run(ctx, scope, "daemon-reload"); err != nil {
		return errors.WithMessage(err, "failed to reload systemctl")
	}

	unitName := strings.TrimSuffix(filepath.Base(unitPath), ".service")
	if err := Run(ctx, scope, "enable", unitName); err != nil {
		return errors.WithMessagef(err, "failed to enable %s service", unitName)
	}

	if scope == gameap.ScopeUser {
		EnableLinger(ctx)
	}

	return nil
}

func EnableLinger(ctx context.Context) {
	u, err := user.Current()
	if err != nil {
		log.Println(errors.Wrap(err, "failed to determine current user for loginctl enable-linger"))

		return
	}

	if err := oscore.ExecCommand(ctx, "loginctl", "enable-linger", u.Username); err != nil {
		log.Printf(
			"Warning: failed to enable linger for %s (user services will stop on logout): %v.\n"+
				"To fix manually: sudo loginctl enable-linger %s\n",
			u.Username, err, u.Username,
		)
	}
}

// CheckUserManager reports whether a systemd user manager is reachable. Without
// this check systemctl --user fails with an opaque "Failed to connect to bus"
// under su/sudo, where XDG_RUNTIME_DIR is not set for the target user.
func CheckUserManager() error {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return errors.New(
			"XDG_RUNTIME_DIR is not set, the systemd user manager is not reachable. " + userManagerHint,
		)
	}

	privatePath := filepath.Clean(filepath.Join(runtimeDir, "systemd", "private"))
	if _, err := os.Stat(privatePath); err != nil {
		return errors.Wrapf(
			err,
			"the systemd user manager is not running for the current user (%s is not available). %s",
			privatePath, userManagerHint,
		)
	}

	return nil
}

func ensureUserManager(scope string) error {
	if scope != gameap.ScopeUser {
		return nil
	}

	return CheckUserManager()
}
