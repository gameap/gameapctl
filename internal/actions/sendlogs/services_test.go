package sendlogs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_copyLogDir(t *testing.T) {
	src := t.TempDir()
	destination := filepath.Join(t.TempDir(), "logs")

	require.NoError(t, os.MkdirAll(filepath.Join(src, "gameap"), 0755))
	writeLogFile(t, filepath.Join(src, "gameap", "GameAP.log"), "panel log", time.Now())
	writeLogFile(t, filepath.Join(src, "gameap", "GameAP.2026-01-01.log"), "old log", time.Now().Add(-30*24*time.Hour))

	require.NoError(t, copyLogDir(src, destination, time.Now().Add(-serviceLogMaxAge)))

	contents, err := os.ReadFile(filepath.Join(destination, "gameap", "GameAP.log"))
	require.NoError(t, err)
	require.Equal(t, "panel log", string(contents))

	require.NoFileExists(t, filepath.Join(destination, "gameap", "GameAP.2026-01-01.log"))
}

func Test_copyLogFile_TruncatesLargeFile(t *testing.T) {
	src := filepath.Join(t.TempDir(), "GameAP.log")
	destination := filepath.Join(t.TempDir(), "GameAP.log")

	contents := strings.Repeat("filler line\n", serviceLogMaxSize/len("filler line\n")+100) + "the last line\n"
	writeLogFile(t, src, contents, time.Now())

	info, err := os.Stat(src)
	require.NoError(t, err)

	require.NoError(t, copyLogFile(src, destination, info.Size()))

	copied, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Less(t, int64(len(copied)), info.Size())
	require.Contains(t, string(copied), "truncated")
	require.True(t, strings.HasSuffix(strings.TrimSpace(string(copied)), "the last line"))
}

func Test_filterPortLines(t *testing.T) {
	output := strings.Join([]string{
		"  TCP    0.0.0.0:80             0.0.0.0:0              LISTENING       4",
		"  TCP    0.0.0.0:8080           0.0.0.0:0              LISTENING       1234",
		"  TCP    127.0.0.1:3306         0.0.0.0:0              LISTENING       777",
		"  TCP    192.168.0.2:52180      93.184.216.34:80       ESTABLISHED     5555",
	}, "\n")

	filtered := filterPortLines(output, "80")

	require.Equal(t, []string{
		"  TCP    0.0.0.0:80             0.0.0.0:0              LISTENING       4",
		"  TCP    192.168.0.2:52180      93.184.216.34:80       ESTABLISHED     5555",
	}, strings.Split(filtered, "\n"))
}

func writeLogFile(t *testing.T, path string, contents string, modTime time.Time) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(contents), 0600))
	require.NoError(t, os.Chtimes(path, modTime, modTime))
}
