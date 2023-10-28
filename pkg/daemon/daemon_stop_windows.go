//go:build windows
// +build windows

package daemon

import (
	"context"
)

func Stop(_ context.Context) error {
	return nil
}
