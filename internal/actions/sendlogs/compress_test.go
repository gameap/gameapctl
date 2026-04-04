package sendlogs

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompress_RelativePaths(t *testing.T) {
	srcDir := t.TempDir()

	subDir := filepath.Join(srcDir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root content"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested content"), 0644))

	var buf bytes.Buffer
	require.NoError(t, compress(srcDir, &buf))

	gr, err := gzip.NewReader(&buf)
	require.NoError(t, err)
	defer gr.Close()

	tr := tar.NewReader(gr)
	names := make(map[string]string)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		if hdr.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(tr)
			require.NoError(t, err)
			names[hdr.Name] = string(content)
		}

		assert.False(t, filepath.IsAbs(hdr.Name), "archive path should be relative, got: %s", hdr.Name)
	}

	assert.Contains(t, names, "root.txt")
	assert.Contains(t, names, "subdir/nested.txt")
	assert.Equal(t, "root content", names["root.txt"])
	assert.Equal(t, "nested content", names["subdir/nested.txt"])
}

func TestCompress_EmptyDirectory(t *testing.T) {
	srcDir := t.TempDir()

	var buf bytes.Buffer
	require.NoError(t, compress(srcDir, &buf))

	gr, err := gzip.NewReader(&buf)
	require.NoError(t, err)
	defer gr.Close()

	tr := tar.NewReader(gr)
	count := 0
	for {
		_, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		count++
	}

	assert.Equal(t, 1, count)
}
