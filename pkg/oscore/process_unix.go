//go:build linux || darwin

package oscore

import (
	"context"
	"os"

	"github.com/shirou/gopsutil/v3/process"
)

// processBelongsToCurrentUser reports whether the process is owned by the
// current effective user. Root manages processes of all users.
func processBelongsToCurrentUser(ctx context.Context, p *process.Process) bool {
	euid := os.Geteuid()
	if euid == 0 {
		return true
	}

	uids, err := p.UidsWithContext(ctx)
	if err != nil {
		return false
	}

	for _, uid := range uids {
		if int(uid) == euid {
			return true
		}
	}

	return false
}
