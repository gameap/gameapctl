//go:build linux

package utils_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/stretchr/testify/require"
)

// A binary that is being executed cannot be opened for writing (ETXTBSY), which is how
// a re-installation over a running panel used to fail. Renaming a staged file over it works.
func Test_ReplaceFile_ReplacesRunningExecutable(t *testing.T) {
	sleepBinary, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep binary is not available")
	}

	dir := t.TempDir()
	running := filepath.Join(dir, "running")

	original, err := os.ReadFile(sleepBinary)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(running, original, 0755))

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, running, "60")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
	})

	_, err = os.OpenFile(running, os.O_WRONLY, 0)
	require.ErrorIs(t, err, syscall.ETXTBSY)

	src := filepath.Join(dir, "src")
	require.NoError(t, os.WriteFile(src, []byte("replacement"), 0600))

	require.NoError(t, utils.ReplaceFile(src, running, 0755))

	content, err := os.ReadFile(running)
	require.NoError(t, err)
	require.Equal(t, "replacement", string(content))
}
