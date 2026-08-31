//go:build linux || darwin

package configenv

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrite_PreservesMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.env")

	require.NoError(t, os.WriteFile(path, []byte("A=1\n"), 0o640))

	lines, _, err := Read(path)
	require.NoError(t, err)
	require.NoError(t, Write(path, lines))

	info, err := os.Stat(path)
	require.NoError(t, err)

	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
}

func TestWrite_PreservesOwner(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("changing file ownership requires root")
	}

	path := filepath.Join(t.TempDir(), "config.env")

	require.NoError(t, os.WriteFile(path, []byte("A=1\n"), 0o600))

	const nobody = 65534

	require.NoError(t, os.Chown(path, nobody, nobody))

	lines, _, err := Read(path)
	require.NoError(t, err)
	require.NoError(t, Write(path, lines))

	info, err := os.Stat(path)
	require.NoError(t, err)

	stat, ok := info.Sys().(*syscall.Stat_t)
	require.True(t, ok)

	assert.Equal(t, uint32(nobody), stat.Uid)
	assert.Equal(t, uint32(nobody), stat.Gid)
}

func TestWrite_UnwritableDirectoryLeavesTheOriginalIntact(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.env")

	require.NoError(t, os.WriteFile(path, []byte("A=1\n"), 0o600))
	require.NoError(t, os.Chmod(dir, 0o500))

	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	require.Error(t, Write(path, []string{"A=2"}))

	body, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "A=1\n", string(body))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "config.env", entries[0].Name())
}
