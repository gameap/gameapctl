package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gameap/gameapctl/pkg/releasesource"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cdnAvailability(com, ru bool) map[string]bool {
	return map[string]bool{
		releasesource.CDNGameAPCom: com,
		releasesource.CDNGameAPRu:  ru,
	}
}

// readReplacements parses the saved config in the same shape the daemon
// accepts for remote_repository_replacements.
func readReplacements(t *testing.T, path string) map[string][]string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var doc struct {
		Replacements map[string][]string `yaml:"remote_repository_replacements"` //nolint:tagliatelle
	}
	require.NoError(t, yaml.Unmarshal(data, &doc))

	return doc.Replacements
}

func Test_planCDNReplacements(t *testing.T) {
	tests := []struct {
		name         string
		availability map[string]bool
		want         []replacementRule
	}{
		{
			name:         "both_up",
			availability: cdnAvailability(true, true),
			want:         []replacementRule{},
		},
		{
			name:         "com_down_ru_up",
			availability: cdnAvailability(false, true),
			want: []replacementRule{
				{origin: releasesource.CDNGameAPCom, mirror: releasesource.CDNGameAPRu},
			},
		},
		{
			name:         "ru_down_com_up",
			availability: cdnAvailability(true, false),
			want: []replacementRule{
				{origin: releasesource.CDNGameAPRu, mirror: releasesource.CDNGameAPCom},
			},
		},
		{
			name:         "both_down",
			availability: cdnAvailability(false, false),
			want:         []replacementRule{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, planCDNReplacements(tt.availability))
		})
	}
}

func TestEnsureRepositoryReplacements_KeyAbsent_AddsEntry(t *testing.T) {
	p := writeTempConfig(t, `api_host: "https://panel.example.com"
api_key: "secret"
`)

	require.NoError(t, EnsureRepositoryReplacements(p, cdnAvailability(false, true)))

	replacements := readReplacements(t, p)
	require.Len(t, replacements, 1)
	assert.Equal(t, []string{releasesource.CDNGameAPRu}, replacements[releasesource.CDNGameAPCom])

	cfg, err := LoadConfig(p)
	require.NoError(t, err)

	host, ok, err := cfg.ReadString("$.api_host")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "https://panel.example.com", host)
}

func TestEnsureRepositoryReplacements_ReverseDirection(t *testing.T) {
	p := writeTempConfig(t, "api_host: \"x\"\n")

	require.NoError(t, EnsureRepositoryReplacements(p, cdnAvailability(true, false)))

	replacements := readReplacements(t, p)
	require.Len(t, replacements, 1)
	assert.Equal(t, []string{releasesource.CDNGameAPCom}, replacements[releasesource.CDNGameAPRu])
}

func TestEnsureRepositoryReplacements_PreservesComments(t *testing.T) {
	p := writeTempConfig(t, `# top comment
api_host: "https://panel.example.com" # inline comment
api_key: "secret"
`)

	require.NoError(t, EnsureRepositoryReplacements(p, cdnAvailability(false, true)))

	out, err := os.ReadFile(p)
	require.NoError(t, err)
	result := string(out)
	assert.Contains(t, result, "# top comment")
	assert.Contains(t, result, "# inline comment")
}

func TestEnsureRepositoryReplacements_ExistingKey_AddsHost(t *testing.T) {
	p := writeTempConfig(t, `api_host: "x"
remote_repository_replacements:
  custom.host:
    - mirror.example.com
`)

	require.NoError(t, EnsureRepositoryReplacements(p, cdnAvailability(false, true)))

	replacements := readReplacements(t, p)
	require.Len(t, replacements, 2)
	assert.Equal(t, []string{"mirror.example.com"}, replacements["custom.host"])
	assert.Equal(t, []string{releasesource.CDNGameAPRu}, replacements[releasesource.CDNGameAPCom])
}

func TestEnsureRepositoryReplacements_OriginPresent_Untouched(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "sequence_form",
			content: `remote_repository_replacements:
  cdn.gameap.com:
    - my.mirror.example.com
`,
		},
		{
			name:    "scalar_form",
			content: "remote_repository_replacements:\n  cdn.gameap.com: my.mirror.example.com\n",
		},
		{
			name: "object_form",
			content: `remote_repository_replacements:
  cdn.gameap.com:
    - replace: my.mirror.example.com
      priority: 10
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := writeTempConfig(t, tt.content)

			require.NoError(t, EnsureRepositoryReplacements(p, cdnAvailability(false, true)))

			out, err := os.ReadFile(p)
			require.NoError(t, err)
			assert.Equal(t, tt.content, string(out))
		})
	}
}

func TestEnsureRepositoryReplacements_NullKey_AddsBlock(t *testing.T) {
	p := writeTempConfig(t, `api_host: "x"
remote_repository_replacements:
`)

	require.NoError(t, EnsureRepositoryReplacements(p, cdnAvailability(false, true)))

	replacements := readReplacements(t, p)
	require.Len(t, replacements, 1)
	assert.Equal(t, []string{releasesource.CDNGameAPRu}, replacements[releasesource.CDNGameAPCom])
}

func TestEnsureRepositoryReplacements_EmptyFlowMap_AddsBlock(t *testing.T) {
	p := writeTempConfig(t, "remote_repository_replacements: {}\n")

	require.NoError(t, EnsureRepositoryReplacements(p, cdnAvailability(false, true)))

	replacements := readReplacements(t, p)
	require.Len(t, replacements, 1)
	assert.Equal(t, []string{releasesource.CDNGameAPRu}, replacements[releasesource.CDNGameAPCom])
}

func TestEnsureRepositoryReplacements_FlowStyleMap_ReturnsError(t *testing.T) {
	content := "remote_repository_replacements: {custom.host: mirror.example.com}\n"
	p := writeTempConfig(t, content)

	err := EnsureRepositoryReplacements(p, cdnAvailability(false, true))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flow style")

	out, readErr := os.ReadFile(p)
	require.NoError(t, readErr)
	assert.Equal(t, content, string(out))
}

func TestEnsureRepositoryReplacements_UnexpectedStructure_ReturnsError(t *testing.T) {
	p := writeTempConfig(t, "remote_repository_replacements: \"broken\"\n")

	err := EnsureRepositoryReplacements(p, cdnAvailability(false, true))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected structure")
}

func TestEnsureRepositoryReplacements_NoPlan_NoChanges(t *testing.T) {
	tests := []struct {
		name         string
		availability map[string]bool
	}{
		{name: "both_up", availability: cdnAvailability(true, true)},
		{name: "both_down", availability: cdnAvailability(false, false)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := "api_host: \"x\"\n"
			p := writeTempConfig(t, content)

			require.NoError(t, EnsureRepositoryReplacements(p, tt.availability))

			out, err := os.ReadFile(p)
			require.NoError(t, err)
			assert.Equal(t, content, string(out))
		})
	}
}

func TestEnsureRepositoryReplacements_NoPlan_MissingFileOK(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nope.yaml")

	require.NoError(t, EnsureRepositoryReplacements(p, cdnAvailability(true, true)))
}

func TestEnsureRepositoryReplacements_Idempotent(t *testing.T) {
	p := writeTempConfig(t, "api_host: \"x\"\n")

	require.NoError(t, EnsureRepositoryReplacements(p, cdnAvailability(false, true)))
	afterFirst, err := os.ReadFile(p)
	require.NoError(t, err)

	require.NoError(t, EnsureRepositoryReplacements(p, cdnAvailability(false, true)))
	afterSecond, err := os.ReadFile(p)
	require.NoError(t, err)

	assert.Equal(t, string(afterFirst), string(afterSecond))
}

func TestEnsureRepositoryReplacements_MissingFile_ReturnsError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nope.yaml")

	err := EnsureRepositoryReplacements(p, cdnAvailability(false, true))
	require.Error(t, err)
}

func TestEnsureRepositoryReplacements_MalformedYAML_ReturnsError(t *testing.T) {
	p := writeTempConfig(t, "foo: [unterminated\n")

	err := EnsureRepositoryReplacements(p, cdnAvailability(false, true))
	require.Error(t, err)
}
