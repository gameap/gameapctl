//go:build linux || darwin

package packagemanager

import "context"

// ProtectWorkPath is a no-op on Linux and macOS: a new directory gets its permissions from
// the mode it is created with, not from its parent directory.
func ProtectWorkPath(_ context.Context) error {
	return nil
}
