//go:build linux || darwin

package oscore

import (
	"context"
	"os"
	"testing"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/stretchr/testify/require"
)

func TestProcessBelongsToCurrentUser_OwnProcess(t *testing.T) {
	ctx := context.Background()
	self, err := process.NewProcessWithContext(ctx, int32(os.Getpid()))
	require.NoError(t, err)

	require.True(t, processBelongsToCurrentUser(ctx, self))
}

func TestProcessBelongsToCurrentUser_InitProcess(t *testing.T) {
	ctx := context.Background()
	initProcess, err := process.NewProcessWithContext(ctx, 1)
	if err != nil {
		t.Skipf("cannot inspect pid 1: %v", err)
	}

	if os.Geteuid() == 0 {
		require.True(t, processBelongsToCurrentUser(ctx, initProcess))

		return
	}

	uids, err := initProcess.UidsWithContext(ctx)
	if err != nil {
		t.Skipf("cannot read pid 1 uids: %v", err)
	}
	for _, uid := range uids {
		if int(uid) == os.Geteuid() {
			t.Skip("pid 1 belongs to current user, cannot test foreign-process filtering")
		}
	}

	require.False(t, processBelongsToCurrentUser(ctx, initProcess))
}
