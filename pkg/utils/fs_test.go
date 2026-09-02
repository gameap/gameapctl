package utils_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/stretchr/testify/require"
)

func Test_TailFile(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		maxLines int
		maxBytes int64
		expected string
	}{
		{
			name:     "empty file",
			contents: "",
			maxLines: 10,
			maxBytes: 1024,
			expected: "",
		},
		{
			name:     "file is smaller than limits",
			contents: "first\nsecond\nthird\n",
			maxLines: 10,
			maxBytes: 1024,
			expected: "first\nsecond\nthird",
		},
		{
			name:     "more lines than limit",
			contents: "first\nsecond\nthird\nfourth\n",
			maxLines: 2,
			maxBytes: 1024,
			expected: "third\nfourth",
		},
		{
			name:     "windows line endings",
			contents: "first\r\nsecond\r\n",
			maxLines: 10,
			maxBytes: 1024,
			expected: "first\nsecond",
		},
		{
			name:     "incomplete first line is dropped",
			contents: "first\nsecond\nthird\n",
			maxLines: 10,
			maxBytes: 12,
			expected: "third",
		},
		{
			name:     "complete first line after offset is kept",
			contents: "aa\nbb\ncc\n",
			maxLines: 10,
			maxBytes: 6,
			expected: "bb\ncc",
		},
		{
			name:     "complete single line after offset is kept",
			contents: "aaaa\nbb\n",
			maxLines: 10,
			maxBytes: 3,
			expected: "bb",
		},
		{
			name:     "offset between CR and LF keeps the first line",
			contents: "aa\r\nbb\r\ncc\r\n",
			maxLines: 10,
			maxBytes: 9,
			expected: "bb\ncc",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.log")
			require.NoError(t, os.WriteFile(path, []byte(test.contents), 0600))

			result, err := utils.TailFile(path, test.maxLines, test.maxBytes)

			require.NoError(t, err)
			require.Equal(t, test.expected, result)
		})
	}
}

func Test_TailFile_LargeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.log")

	builder := strings.Builder{}
	for i := range 10000 {
		builder.WriteString("line " + strconv.Itoa(i) + "\n")
	}
	require.NoError(t, os.WriteFile(path, []byte(builder.String()), 0600))

	result, err := utils.TailFile(path, 50, 16*1024)

	require.NoError(t, err)
	lines := strings.Split(result, "\n")
	require.Len(t, lines, 50)
	require.Equal(t, "line 9999", lines[49])
}

func Test_TailFile_NotExistingFile(t *testing.T) {
	_, err := utils.TailFile(filepath.Join(t.TempDir(), "missing.log"), 10, 1024)

	require.Error(t, err)
}

func Test_ReplaceFile_ReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	require.NoError(t, os.WriteFile(src, []byte("new"), 0600))
	require.NoError(t, os.WriteFile(dst, []byte("old"), 0600))

	require.NoError(t, utils.ReplaceFile(src, dst, 0755))

	content, err := os.ReadFile(dst)
	require.NoError(t, err)
	require.Equal(t, "new", string(content))

	if runtime.GOOS != "windows" {
		info, err := os.Stat(dst)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0755), info.Mode().Perm())
	}

	require.NoFileExists(t, src)
	requireNoStagingFiles(t, dir)
}

func Test_ReplaceFile_CreatesMissingDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "bin", "dst")
	require.NoError(t, os.WriteFile(src, []byte("new"), 0600))

	require.NoError(t, utils.ReplaceFile(src, dst, 0755))

	content, err := os.ReadFile(dst)
	require.NoError(t, err)
	require.Equal(t, "new", string(content))
	requireNoStagingFiles(t, filepath.Dir(dst))
}

func Test_ReplaceFile_ConcurrentReplacements(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "dst")
	require.NoError(t, os.WriteFile(dst, []byte("old"), 0600))

	const writers = 8

	contents := make(map[string]struct{}, writers)
	for i := 0; i < writers; i++ {
		contents[strconv.Itoa(i)] = struct{}{}
	}

	errs := make(chan error, writers)

	var wg sync.WaitGroup
	for content := range contents {
		src := filepath.Join(dir, "src-"+content)
		require.NoError(t, os.WriteFile(src, []byte(content), 0600))

		wg.Add(1)
		go func() {
			defer wg.Done()

			errs <- utils.ReplaceFile(src, dst, 0755)
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	require.Contains(t, contents, string(got))
	requireNoStagingFiles(t, dir)
}

func Test_ReplaceFile_MissingSource(t *testing.T) {
	dir := t.TempDir()

	err := utils.ReplaceFile(filepath.Join(dir, "missing"), filepath.Join(dir, "dst"), 0755)

	require.Error(t, err)
	require.NoFileExists(t, filepath.Join(dir, "dst"))
	requireNoStagingFiles(t, dir)
}

func requireNoStagingFiles(t *testing.T, dir string) {
	t.Helper()

	leftovers, err := filepath.Glob(filepath.Join(dir, "*.new"))
	require.NoError(t, err)
	require.Empty(t, leftovers)
}
