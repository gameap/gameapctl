//go:build !linux

package panel

// CheckScope verifies that the environment can actually manage the panel in the
// given scope. Callers use it to fail before doing any work.
func CheckScope(scope string) error {
	return checkScopeSupported([]Options{{Scope: scope}})
}
