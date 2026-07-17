//go:build windows

package oscore

import (
	"context"

	"github.com/shirou/gopsutil/v3/process"
)

func processBelongsToCurrentUser(_ context.Context, _ *process.Process) bool {
	return true
}
