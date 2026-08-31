package configenv

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRead(t *testing.T) {
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

	lines, values, err := Read(path)
	require.NoError(t, err)

	require.Len(t, lines, 9)
	assert.Equal(t, "panel.example.com", values["HTTP_HOST"])
	assert.Equal(t, "8025", values["HTTP_PORT"])
	assert.Equal(t, "false", values["ACME_ENABLED"])
	assert.Equal(t, "", values["ACME_EMAIL"])
	assert.Equal(t, "mysql://user:pass@host/db", values["DATABASE_URL"])
}

func TestUpdate_UpdatesExistingKeys(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.env")

	original := `HTTP_HOST=panel.example.com
ACME_ENABLED=false
ACME_EMAIL=
# trailing comment
`

	require.NoError(t, os.WriteFile(path, []byte(original), 0o600))

	lines, _, err := Read(path)
	require.NoError(t, err)

	updates := map[string]string{
		"ACME_ENABLED": "true",
		"ACME_EMAIL":   "ops@example.com",
		"ACME_DOMAINS": "*.example.com,example.com",
	}

	require.NoError(t, Update(path, lines, updates))

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

func TestUpdate_RemovesKeys(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.env")

	original := `HTTP_HOST=panel.example.com
ACME_ENABLED=true
ACME_EMAIL=ops@example.com
ACME_DOMAINS=example.com
`

	require.NoError(t, os.WriteFile(path, []byte(original), 0o600))

	lines, _, err := Read(path)
	require.NoError(t, err)

	updates := map[string]string{
		"ACME_ENABLED": "false",
		"ACME_EMAIL":   RemoveMarker,
		"ACME_DOMAINS": RemoveMarker,
	}

	require.NoError(t, Update(path, lines, updates))

	rewritten, err := os.ReadFile(path)
	require.NoError(t, err)

	content := string(rewritten)

	assert.Contains(t, content, "HTTP_HOST=panel.example.com")
	assert.Contains(t, content, "ACME_ENABLED=false")
	assert.NotContains(t, content, "ACME_EMAIL=ops@example.com")
	assert.NotContains(t, content, "ACME_DOMAINS=example.com")
}

func TestUpdate_PreservesUnrelatedLines(t *testing.T) {
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

	lines, _, err := Read(path)
	require.NoError(t, err)

	require.NoError(t, Update(path, lines, map[string]string{"ACME_ENABLED": "true"}))

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

func TestRead_MissingFile(t *testing.T) {
	_, _, err := Read(filepath.Join(t.TempDir(), "absent.env"))

	require.Error(t, err)
	assert.True(t, errors.Is(err, fs.ErrNotExist))
}

func TestWrite_RoundTripIsByteStable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.env")

	body := `# header

HTTP_HOST=panel.example.com
HTTP_PORT=8025

# section
ACME_ENABLED=false
`

	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	lines, _, err := Read(path)
	require.NoError(t, err)
	require.NoError(t, Write(path, lines))

	rewritten, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Equal(t, body, string(rewritten))
}

func TestRename(t *testing.T) {
	appendS := func(v string) string {
		if v == "" {
			return v
		}

		for _, r := range v {
			if r < '0' || r > '9' {
				return v
			}
		}

		return v + "s"
	}

	tests := []struct {
		name      string
		lines     []string
		convert   Converter
		wantLines []string
		wantValue string
		wantOK    bool
	}{
		{
			name:      "bare_number_gains_its_unit",
			lines:     []string{"OLD=10"},
			convert:   appendS,
			wantLines: []string{"NEW=10s"},
			wantValue: "10s",
			wantOK:    true,
		},
		{
			name:      "value_that_already_has_a_unit_is_left_alone",
			lines:     []string{"OLD=30s"},
			convert:   appendS,
			wantLines: []string{"NEW=30s"},
			wantValue: "30s",
			wantOK:    true,
		},
		{
			name:      "double_quoted_value_keeps_its_quotes",
			lines:     []string{`OLD="10"`},
			convert:   appendS,
			wantLines: []string{`NEW="10s"`},
			wantValue: `"10s"`,
			wantOK:    true,
		},
		{
			name:      "single_quoted_value_keeps_its_quotes",
			lines:     []string{"OLD='10'"},
			convert:   appendS,
			wantLines: []string{"NEW='10s'"},
			wantValue: "'10s'",
			wantOK:    true,
		},
		{
			name:      "empty_value_stays_empty",
			lines:     []string{"OLD="},
			convert:   appendS,
			wantLines: []string{"NEW="},
			wantValue: "",
			wantOK:    true,
		},
		{
			name:      "nil_converter_carries_the_value_over",
			lines:     []string{"OLD=65536"},
			convert:   nil,
			wantLines: []string{"NEW=65536"},
			wantValue: "65536",
			wantOK:    true,
		},
		{
			name:      "absent_key_leaves_the_file_alone",
			lines:     []string{"OTHER=1"},
			convert:   appendS,
			wantLines: []string{"OTHER=1"},
			wantValue: "",
			wantOK:    false,
		},
		{
			name:      "key_keeps_its_position_and_its_comment",
			lines:     []string{"# what OLD does", "OLD=10", "AFTER=2"},
			convert:   appendS,
			wantLines: []string{"# what OLD does", "NEW=10s", "AFTER=2"},
			wantValue: "10s",
			wantOK:    true,
		},
		{
			name:      "hash_and_spaces_in_the_value_are_preserved",
			lines:     []string{"OLD=a b # c"},
			convert:   nil,
			wantLines: []string{"NEW=a b # c"},
			wantValue: "a b # c",
			wantOK:    true,
		},
		{
			name:      "commented_out_key_is_not_renamed",
			lines:     []string{"# OLD=10"},
			convert:   appendS,
			wantLines: []string{"# OLD=10"},
			wantValue: "",
			wantOK:    false,
		},
		{
			name:      "exported_key_is_not_renamed",
			lines:     []string{"export OLD=10"},
			convert:   appendS,
			wantLines: []string{"export OLD=10"},
			wantValue: "",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok := Rename(tt.lines, "OLD", "NEW", tt.convert)

			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantValue, value)
			assert.Equal(t, tt.wantLines, tt.lines)
		})
	}
}

func TestRemove(t *testing.T) {
	lines := []string{"# comment", "A=1", "B=2"}

	lines, ok := Remove(lines, "A")
	require.True(t, ok)
	require.Len(t, lines, 2)
	assert.Equal(t, []string{"# comment", "B=2"}, lines)

	lines, ok = Remove(lines, "MISSING")
	assert.False(t, ok)
	assert.Equal(t, []string{"# comment", "B=2"}, lines)
}

func TestAppend(t *testing.T) {
	lines := Append([]string{"A=1"}, "B", "2")

	require.Len(t, lines, 2)
	assert.Equal(t, "B=2", lines[1])
}
