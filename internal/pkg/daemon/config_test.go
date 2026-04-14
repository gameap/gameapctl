package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "gameap-daemon.yaml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))

	return p
}

func TestConfigFile_ReadString(t *testing.T) {
	p := writeTempConfig(t, `api_host: "https://panel.example.com"
api_key: "secret"
ds_id: 42
`)
	cfg, err := LoadConfig(p)
	require.NoError(t, err)

	host, ok, err := cfg.ReadString("$.api_host")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "https://panel.example.com", host)

	key, ok, err := cfg.ReadString("$.api_key")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "secret", key)

	missing, ok, err := cfg.ReadString("$.not_here")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, missing)
}

func TestConfigFile_ReadUint(t *testing.T) {
	p := writeTempConfig(t, "ds_id: 42\n")
	cfg, err := LoadConfig(p)
	require.NoError(t, err)

	id, ok, err := cfg.ReadUint("$.ds_id")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, uint(42), id)
}

func TestConfigFile_EnsureGRPCEnabled_Appends(t *testing.T) {
	original := `# main daemon config
api_host: "https://panel.example.com"
api_key: "secret"
`
	p := writeTempConfig(t, original)
	cfg, err := LoadConfig(p)
	require.NoError(t, err)

	require.NoError(t, cfg.EnsureGRPCEnabled(""))
	require.NoError(t, cfg.Save())

	out, err := os.ReadFile(p)
	require.NoError(t, err)
	result := string(out)
	assert.Contains(t, result, "# main daemon config")
	assert.Contains(t, result, "api_host: \"https://panel.example.com\"")
	assert.Contains(t, result, "grpc:")
	assert.Contains(t, result, "enabled: true")
}

func TestConfigFile_EnsureGRPCEnabled_WithAddress(t *testing.T) {
	p := writeTempConfig(t, "api_host: \"https://panel.example.com\"\n")
	cfg, err := LoadConfig(p)
	require.NoError(t, err)

	require.NoError(t, cfg.EnsureGRPCEnabled("panel.example.com:31718"))
	require.NoError(t, cfg.Save())

	reloaded, err := LoadConfig(p)
	require.NoError(t, err)

	enabled, ok, err := reloaded.ReadString("$.grpc.enabled")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "true", enabled)

	addr, ok, err := reloaded.ReadString("$.grpc.address")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "panel.example.com:31718", addr)
}

func TestConfigFile_EnsureGRPCEnabled_ExistingBlockReplacesEnabled(t *testing.T) {
	p := writeTempConfig(t, `api_host: "x"
grpc:
  enabled: false
  address: "old:31718"
`)
	cfg, err := LoadConfig(p)
	require.NoError(t, err)

	require.NoError(t, cfg.EnsureGRPCEnabled(""))
	require.NoError(t, cfg.Save())

	reloaded, err := LoadConfig(p)
	require.NoError(t, err)

	enabled, ok, err := reloaded.ReadString("$.grpc.enabled")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "true", enabled)
}

func TestConfigFile_DeleteKey_RemovesTopLevelKey(t *testing.T) {
	p := writeTempConfig(t, `api_host: "https://panel.example.com"
api_key: "secret"
ds_id: 42
`)
	cfg, err := LoadConfig(p)
	require.NoError(t, err)

	require.NoError(t, cfg.DeleteKey("$.api_host"))
	require.NoError(t, cfg.Save())

	reloaded, err := LoadConfig(p)
	require.NoError(t, err)

	_, ok, err := reloaded.ReadString("$.api_host")
	require.NoError(t, err)
	assert.False(t, ok)

	apiKey, ok, err := reloaded.ReadString("$.api_key")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "secret", apiKey)

	dsID, ok, err := reloaded.ReadUint("$.ds_id")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, uint(42), dsID)
}

func TestConfigFile_DeleteKey_MissingKey_NoError(t *testing.T) {
	original := `api_key: "secret"
ds_id: 42
`
	p := writeTempConfig(t, original)
	cfg, err := LoadConfig(p)
	require.NoError(t, err)

	require.NoError(t, cfg.DeleteKey("$.api_host"))
	require.NoError(t, cfg.Save())

	out, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Contains(t, string(out), "api_key: \"secret\"")
	assert.Contains(t, string(out), "ds_id: 42")
}

func TestConfigFile_DeleteKey_RejectsNestedPath(t *testing.T) {
	p := writeTempConfig(t, `grpc:
  enabled: true
`)
	cfg, err := LoadConfig(p)
	require.NoError(t, err)

	err = cfg.DeleteKey("$.grpc.enabled")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "top-level")
}

func TestConfigFile_DeleteKey_CombinedWithEnsureGRPCEnabled(t *testing.T) {
	p := writeTempConfig(t, `api_host: "https://panel.example.com"
api_key: "secret"
ds_id: 7
ca_certificate_file: "certs/ca.crt"
`)
	cfg, err := LoadConfig(p)
	require.NoError(t, err)

	require.NoError(t, cfg.EnsureGRPCEnabled("panel.example.com:31718"))
	require.NoError(t, cfg.DeleteKey("$.api_host"))
	require.NoError(t, cfg.DeleteKey("$.api_key"))
	require.NoError(t, cfg.Save())

	out, err := os.ReadFile(p)
	require.NoError(t, err)
	result := string(out)
	assert.NotContains(t, result, "api_host")
	assert.NotContains(t, result, "api_key")
	assert.Contains(t, result, "ca_certificate_file")

	reloaded, err := LoadConfig(p)
	require.NoError(t, err)

	enabled, ok, err := reloaded.ReadString("$.grpc.enabled")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "true", enabled)

	addr, ok, err := reloaded.ReadString("$.grpc.address")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "panel.example.com:31718", addr)
}

func TestConfigFile_EnsureGRPCEnabled_PreservesComments(t *testing.T) {
	p := writeTempConfig(t, `# top comment
api_host: "https://panel.example.com" # inline comment
api_key: "secret"
# trailing comment
`)
	cfg, err := LoadConfig(p)
	require.NoError(t, err)

	require.NoError(t, cfg.EnsureGRPCEnabled(""))
	require.NoError(t, cfg.Save())

	out, err := os.ReadFile(p)
	require.NoError(t, err)
	result := string(out)
	assert.Contains(t, result, "# top comment")
	assert.Contains(t, result, "# inline comment")
}

func TestBackupAndRestore_RoundTrip(t *testing.T) {
	p := writeTempConfig(t, "api_host: \"v1\"\n")

	backup, err := Backup(p)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(p, []byte("api_host: \"v2\"\n"), 0o600))

	require.NoError(t, Restore(backup, p))

	out, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "api_host: \"v1\"\n", string(out))
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	p := writeTempConfig(t, "foo: [unterminated\n")
	_, err := LoadConfig(p)
	require.Error(t, err)
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "nope.yaml"))
	require.Error(t, err)
}
