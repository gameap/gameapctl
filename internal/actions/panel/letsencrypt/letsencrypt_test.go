package letsencrypt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadEnv(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.env")

	body := `# GameAP config
HTTP_HOST=panel.example.com
HTTP_PORT=8025

# ACME settings
ACME_ENABLED=false
ACME_EMAIL=

DATABASE_URL=mysql://user:pass@host/db
`

	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	lines, values, err := readEnv(path)
	require.NoError(t, err)

	assert.Len(t, lines, 9)
	assert.Equal(t, "panel.example.com", values["HTTP_HOST"])
	assert.Equal(t, "8025", values["HTTP_PORT"])
	assert.Equal(t, "false", values["ACME_ENABLED"])
	assert.Equal(t, "", values["ACME_EMAIL"])
	assert.Equal(t, "mysql://user:pass@host/db", values["DATABASE_URL"])
}

func TestWriteEnv_UpdatesExistingKeys(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.env")

	original := `HTTP_HOST=panel.example.com
ACME_ENABLED=false
ACME_EMAIL=
# trailing comment
`

	require.NoError(t, os.WriteFile(path, []byte(original), 0o600))

	lines, _, err := readEnv(path)
	require.NoError(t, err)

	updates := map[string]string{
		"ACME_ENABLED": "true",
		"ACME_EMAIL":   "ops@example.com",
		"ACME_DOMAINS": "*.example.com,example.com",
	}

	require.NoError(t, writeEnv(path, lines, updates))

	rewritten, err := os.ReadFile(path)
	require.NoError(t, err)

	content := string(rewritten)

	assert.Contains(t, content, "HTTP_HOST=panel.example.com")
	assert.Contains(t, content, "ACME_ENABLED=true")
	assert.Contains(t, content, "ACME_EMAIL=ops@example.com")
	assert.Contains(t, content, "ACME_DOMAINS=*.example.com,example.com")
	assert.Contains(t, content, "# trailing comment")
	assert.NotContains(t, content, "ACME_ENABLED=false")
}

func TestWriteEnv_RemovesKeys(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.env")

	original := `HTTP_HOST=panel.example.com
ACME_ENABLED=true
ACME_EMAIL=ops@example.com
ACME_DOMAINS=example.com
`

	require.NoError(t, os.WriteFile(path, []byte(original), 0o600))

	lines, _, err := readEnv(path)
	require.NoError(t, err)

	updates := map[string]string{
		"ACME_ENABLED": "false",
		"ACME_EMAIL":   removeMarker,
		"ACME_DOMAINS": removeMarker,
	}

	require.NoError(t, writeEnv(path, lines, updates))

	rewritten, err := os.ReadFile(path)
	require.NoError(t, err)

	content := string(rewritten)

	assert.Contains(t, content, "HTTP_HOST=panel.example.com")
	assert.Contains(t, content, "ACME_ENABLED=false")
	assert.NotContains(t, content, "ACME_EMAIL=ops@example.com")
	assert.NotContains(t, content, "ACME_DOMAINS=example.com")
}

func TestWriteEnv_PreservesUnrelatedLines(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.env")

	original := `# header

HTTP_HOST=panel.example.com
HTTP_PORT=8025
DATABASE_DRIVER=mysql

# section
ACME_ENABLED=false
`

	require.NoError(t, os.WriteFile(path, []byte(original), 0o600))

	lines, _, err := readEnv(path)
	require.NoError(t, err)

	require.NoError(t, writeEnv(path, lines, map[string]string{"ACME_ENABLED": "true"}))

	rewritten, err := os.ReadFile(path)
	require.NoError(t, err)

	content := string(rewritten)

	assert.Contains(t, content, "# header")
	assert.Contains(t, content, "HTTP_HOST=panel.example.com")
	assert.Contains(t, content, "HTTP_PORT=8025")
	assert.Contains(t, content, "DATABASE_DRIVER=mysql")
	assert.Contains(t, content, "# section")
	assert.Contains(t, content, "ACME_ENABLED=true")
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"example.com", []string{"example.com"}},
		{"a, b ,c", []string{"a", "b", "c"}},
		{"  *.example.com  , example.com ", []string{"*.example.com", "example.com"}},
		{",,empty,,", []string{"empty"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitAndTrim(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
