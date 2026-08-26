package app

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/stretchr/testify/require"
)

func Test_setLogFile(t *testing.T) {
	defer log.SetOutput(os.Stderr)

	dir := t.TempDir()

	logfile, ok := setLogFile(dir, "panel_install.log")

	require.True(t, ok)
	require.Equal(t, filepath.Join(dir, "panel_install.log"), logfile)

	contents, err := os.ReadFile(logfile)
	require.NoError(t, err)

	require.Contains(t, string(contents), gameap.Version)
	require.Contains(t, string(contents), "Command:")
}
