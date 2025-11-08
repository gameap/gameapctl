//go:build windows

package packagemanager

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/package_manager/windows"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_convertAccessToOSCoreFlag(t *testing.T) {
	tests := []struct {
		name   string
		access string
		want   oscore.GrantFlag
	}{
		{
			name:   "read lowercase",
			access: "r",
			want:   oscore.GrantFlagRead,
		},
		{
			name:   "read full",
			access: "read",
			want:   oscore.GrantFlagRead,
		},
		{
			name:   "read-execute lowercase",
			access: "rx",
			want:   oscore.GrantFlagReadExecute,
		},
		{
			name:   "read-execute full",
			access: "read-execute",
			want:   oscore.GrantFlagReadExecute,
		},
		{
			name:   "read-execute no hyphen",
			access: "readexecute",
			want:   oscore.GrantFlagReadExecute,
		},
		{
			name:   "write lowercase",
			access: "w",
			want:   oscore.GrantFlagWrite,
		},
		{
			name:   "write full",
			access: "write",
			want:   oscore.GrantFlagWrite,
		},
		{
			name:   "modify lowercase",
			access: "m",
			want:   oscore.GrantFlagModify,
		},
		{
			name:   "modify full",
			access: "modify",
			want:   oscore.GrantFlagModify,
		},
		{
			name:   "full-control lowercase",
			access: "f",
			want:   oscore.GrantFlagFullControl,
		},
		{
			name:   "full-control with hyphen",
			access: "full-control",
			want:   oscore.GrantFlagFullControl,
		},
		{
			name:   "full-control no hyphen",
			access: "fullcontrol",
			want:   oscore.GrantFlagFullControl,
		},
		{
			name:   "uppercase read",
			access: "READ",
			want:   oscore.GrantFlagRead,
		},
		{
			name:   "mixed case",
			access: "Full-Control",
			want:   oscore.GrantFlagFullControl,
		},
		{
			name:   "unknown defaults to read",
			access: "unknown",
			want:   oscore.GrantFlagRead,
		},
		{
			name:   "empty defaults to read",
			access: "",
			want:   oscore.GrantFlagRead,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertAccessToOSCoreFlag(tt.access)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_waitUntil(t *testing.T) {
	t.Run("success on first try", func(t *testing.T) {
		ctx := context.Background()

		callCount := 0
		f := func(ctx context.Context) (bool, error) {
			callCount++

			return true, nil
		}

		err := waitUntil(ctx, f)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("success after multiple tries", func(t *testing.T) {
		ctx := context.Background()

		callCount := 0
		f := func(ctx context.Context) (bool, error) {
			callCount++
			if callCount >= 3 {
				return true, nil
			}

			return false, nil
		}

		err := waitUntil(ctx, f)
		assert.NoError(t, err)
		assert.Equal(t, 3, callCount)
	})

	t.Run("timeout after max retries", func(t *testing.T) {
		ctx := context.Background()

		callCount := 0
		f := func(ctx context.Context) (bool, error) {
			callCount++

			return false, nil
		}

		err := waitUntil(ctx, f)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout waiting")
		assert.Equal(t, waitTriesMax, callCount)
	})

	t.Run("nil function returns nil", func(t *testing.T) {
		ctx := context.Background()

		err := waitUntil(ctx, nil)
		assert.NoError(t, err)
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		callCount := 0
		f := func(ctx context.Context) (bool, error) {
			callCount++
			if callCount == 2 {
				cancel()
			}

			return false, nil
		}

		err := waitUntil(ctx, f)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout waiting")
	})

	t.Run("function returns error", func(t *testing.T) {
		ctx := context.Background()

		expectedErr := assert.AnError
		f := func(ctx context.Context) (bool, error) {
			return false, expectedErr
		}

		err := waitUntil(ctx, f)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to execute wait after func")
	})
}

func Test_appendPathEnvVariable(t *testing.T) {
	t.Run("add new paths", func(t *testing.T) {
		originalPath := os.Getenv("PATH")
		defer func() {
			_ = os.Setenv("PATH", originalPath)
		}()

		tempDir := t.TempDir()
		newPath1 := filepath.Join(tempDir, "bin1")
		newPath2 := filepath.Join(tempDir, "bin2")

		require.NoError(t, os.MkdirAll(newPath1, 0755))
		require.NoError(t, os.MkdirAll(newPath2, 0755))

		err := os.Setenv("PATH", "/usr/bin"+string(filepath.ListSeparator)+"/usr/local/bin")
		require.NoError(t, err)

		err = appendPathEnvVariable([]string{newPath1, newPath2})
		assert.NoError(t, err)

		currentPath := os.Getenv("PATH")
		assert.Contains(t, currentPath, newPath1)
		assert.Contains(t, currentPath, newPath2)
	})

	t.Run("skip non-existent paths", func(t *testing.T) {
		originalPath := os.Getenv("PATH")
		defer func() {
			_ = os.Setenv("PATH", originalPath)
		}()

		err := os.Setenv("PATH", "/usr/bin"+string(filepath.ListSeparator)+"/usr/local/bin")
		require.NoError(t, err)

		nonExistentPath := filepath.Join(t.TempDir(), "nonexistent")

		err = appendPathEnvVariable([]string{nonExistentPath})
		assert.NoError(t, err)

		currentPath := os.Getenv("PATH")
		assert.NotContains(t, currentPath, nonExistentPath)
	})

	t.Run("skip duplicate paths", func(t *testing.T) {
		originalPath := os.Getenv("PATH")
		defer func() {
			_ = os.Setenv("PATH", originalPath)
		}()

		tempDir := t.TempDir()
		newPath := filepath.Join(tempDir, "bin")
		require.NoError(t, os.MkdirAll(newPath, 0755))

		err := os.Setenv("PATH", "/usr/bin"+string(filepath.ListSeparator)+newPath)
		require.NoError(t, err)

		err = appendPathEnvVariable([]string{newPath})
		assert.NoError(t, err)

		currentPath := os.Getenv("PATH")
		pathParts := strings.Split(currentPath, string(filepath.ListSeparator))
		count := 0
		for _, p := range pathParts {
			if p == newPath {
				count++
			}
		}
		assert.Equal(t, 1, count)
	})

	t.Run("skip duplicate in new paths", func(t *testing.T) {
		originalPath := os.Getenv("PATH")
		defer func() {
			_ = os.Setenv("PATH", originalPath)
		}()

		tempDir := t.TempDir()
		newPath := filepath.Join(tempDir, "bin")
		require.NoError(t, os.MkdirAll(newPath, 0755))

		err := os.Setenv("PATH", "/usr/bin")
		require.NoError(t, err)

		err = appendPathEnvVariable([]string{newPath, newPath})
		assert.NoError(t, err)

		currentPath := os.Getenv("PATH")
		pathParts := strings.Split(currentPath, string(filepath.ListSeparator))
		count := 0
		for _, p := range pathParts {
			if p == newPath {
				count++
			}
		}
		assert.Equal(t, 1, count)
	})

	t.Run("empty paths list does nothing", func(t *testing.T) {
		originalPath := os.Getenv("PATH")
		defer func() {
			_ = os.Setenv("PATH", originalPath)
		}()

		expectedPath := "/usr/bin" + string(filepath.ListSeparator) + "/usr/local/bin"
		err := os.Setenv("PATH", expectedPath)
		require.NoError(t, err)

		err = appendPathEnvVariable([]string{})
		assert.NoError(t, err)

		currentPath := os.Getenv("PATH")
		assert.Equal(t, expectedPath, currentPath)
	})
}

func Test_WindowsPackageManager_installDependencies(t *testing.T) {
	t.Run("no dependencies", func(t *testing.T) {
		ctx := context.Background()
		pm := &WindowsPackageManager{
			packages: map[string]windows.Package{
				"test-package": {
					Name:         "test-package",
					Dependencies: []string{},
				},
			},
		}

		err := pm.installDependencies(ctx, "test-package")
		assert.NoError(t, err)
	})

	t.Run("package does not exist", func(t *testing.T) {
		ctx := context.Background()
		pm := &WindowsPackageManager{
			packages: map[string]windows.Package{},
		}

		err := pm.installDependencies(ctx, "non-existent-package")
		assert.NoError(t, err)
	})

	t.Run("self dependency error", func(t *testing.T) {
		ctx := context.Background()
		pm := &WindowsPackageManager{
			packages: map[string]windows.Package{
				"test-package": {
					Name:         "test-package",
					Dependencies: []string{"test-package"},
				},
			},
		}

		err := pm.installDependencies(ctx, "test-package")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrCannotDependOnSelf)
	})
}

func Test_WindowsPackageManager_executeCommand(t *testing.T) {
	t.Run("empty command", func(t *testing.T) {
		ctx := context.Background()
		pm := &WindowsPackageManager{}

		err := pm.executeCommand(ctx, "")
		assert.NoError(t, err)
	})

	t.Run("whitespace only command", func(t *testing.T) {
		ctx := context.Background()
		pm := &WindowsPackageManager{}

		err := pm.executeCommand(ctx, "   \t\n   ")
		assert.NoError(t, err)
	})
}
