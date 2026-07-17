package oscore

import (
	"context"
	"os"
	"testing"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/stretchr/testify/require"
)

func TestFindProcessByName_FindsOwnProcess(t *testing.T) {
	ctx := context.Background()
	self, err := process.NewProcessWithContext(ctx, int32(os.Getpid()))
	require.NoError(t, err)
	name, err := self.NameWithContext(ctx)
	require.NoError(t, err)

	found, err := FindProcessByName(ctx, name)

	require.NoError(t, err)
	require.NotNil(t, found)
}
