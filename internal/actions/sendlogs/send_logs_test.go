package sendlogs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectSystemInfo_ContainsVersion(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	err := collectSystemInfo(ctx, tmpDir)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(tmpDir, "system_info.txt"))
	require.NoError(t, err)

	text := string(content)
	assert.Contains(t, text, "GameAPCtl Version: "+gameap.Version)
	assert.Contains(t, text, "GameAPCtl Build Date: "+gameap.BuildDate)
	assert.Contains(t, text, "Kernel:")
	assert.Contains(t, text, "OS:")
	assert.Contains(t, text, "Hostname:")
}

func TestCollectAdditionalLogs_CopiesExistingFiles(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	logFile := filepath.Join(srcDir, "test.log")
	require.NoError(t, os.WriteFile(logFile, []byte("log content"), 0644))

	err := collectAdditionalLogs(context.Background(), []string{logFile}, dstDir)
	require.NoError(t, err)

	copied, err := os.ReadFile(filepath.Join(dstDir, "additional", "test.log"))
	require.NoError(t, err)
	assert.Equal(t, "log content", string(copied))
}

func TestCollectAdditionalLogs_SkipsNonExistentFiles(t *testing.T) {
	dstDir := t.TempDir()

	err := collectAdditionalLogs(context.Background(), []string{"/nonexistent/path.log"}, dstDir)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dstDir, "additional"))
	assert.True(t, os.IsNotExist(err))
}
