//go:build windows
// +build windows

package panel

import (
	"context"
)

func SetPrivileges(_ context.Context, path string) error {
	return nil
}
