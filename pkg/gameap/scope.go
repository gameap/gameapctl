package gameap

import (
	"runtime"

	"github.com/pkg/errors"
)

const (
	ScopeSystem = "system"
	ScopeUser   = "user"
)

const linuxOS = "linux"

// ScopeOrDefault returns the system scope for an empty value.
func ScopeOrDefault(scope string) string {
	if scope == "" {
		return ScopeSystem
	}

	return scope
}

// ResolveScope validates a user supplied scope value against the current platform.
func ResolveScope(scope string) (string, error) {
	return resolveScopeForOS(scope, runtime.GOOS)
}

func resolveScopeForOS(scope, goos string) (string, error) {
	switch ScopeOrDefault(scope) {
	case ScopeSystem:
		return ScopeSystem, nil
	case ScopeUser:
		if goos != linuxOS {
			return "", errors.Errorf(
				"--scope=user requires Linux with systemd (current OS: %s)",
				goos,
			)
		}

		return ScopeUser, nil
	default:
		return "", errors.Errorf(
			"unknown --scope value %q (expected %q or %q)",
			scope, ScopeSystem, ScopeUser,
		)
	}
}
