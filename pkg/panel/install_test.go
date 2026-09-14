package panel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/releasesource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderConfigEnv_GRPCDisabled(t *testing.T) {
	out, err := renderConfigEnv(InstallConfig{
		HTTPHost: "0.0.0.0",
		HTTPPort: "8025",
	})
	require.NoError(t, err)

	rendered := string(out)
	assert.NotContains(t, rendered, "GRPC_ENABLED")
	assert.NotContains(t, rendered, "GRPC_PORT")
	assert.Contains(t, rendered, "HTTP_HOST=0.0.0.0")
	assert.Contains(t, rendered, "HTTP_PORT=8025")
}

func TestRenderConfigEnv_GRPCEnabled_DefaultPort(t *testing.T) {
	out, err := renderConfigEnv(applyConfigDefaults(InstallConfig{
		HTTPHost:    "0.0.0.0",
		HTTPPort:    "8025",
		GRPCEnabled: true,
	}))
	require.NoError(t, err)

	rendered := string(out)
	assert.Contains(t, rendered, "GRPC_ENABLED=true")
	assert.Contains(t, rendered, "GRPC_PORT="+gameap.DefaultGRPCPort)
}

func TestRenderConfigEnv_GRPCEnabled_CustomPort(t *testing.T) {
	out, err := renderConfigEnv(InstallConfig{
		HTTPHost:    "0.0.0.0",
		HTTPPort:    "8025",
		GRPCEnabled: true,
		GRPCPort:    "41718",
	})
	require.NoError(t, err)

	rendered := string(out)
	assert.Contains(t, rendered, "GRPC_ENABLED=true")
	assert.Contains(t, rendered, "GRPC_PORT=41718")
}

func TestApplyConfigDefaults_GRPCPortFallback(t *testing.T) {
	cfg := applyConfigDefaults(InstallConfig{GRPCEnabled: true})
	assert.Equal(t, gameap.DefaultGRPCPort, cfg.GRPCPort)

	cfg = applyConfigDefaults(InstallConfig{GRPCEnabled: false})
	assert.Empty(t, cfg.GRPCPort, "GRPCPort must remain empty when GRPC is disabled")

	cfg = applyConfigDefaults(InstallConfig{GRPCEnabled: true, GRPCPort: "41718"})
	assert.Equal(t, "41718", cfg.GRPCPort)
}

// Sanity check that renderConfigEnv output is a valid env-file shape — no
// trailing spaces in keys, every non-blank/non-comment line has '='.
func TestRenderConfigEnv_ValidShape(t *testing.T) {
	out, err := renderConfigEnv(applyConfigDefaults(InstallConfig{GRPCEnabled: true}))
	require.NoError(t, err)

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		assert.Contains(t, line, "=", "expected key=value, got %q", line)
	}
}

func TestRenderConfigEnv_PluginsStoreURL(t *testing.T) {
	tests := []struct {
		name   string
		config InstallConfig
		want   string
	}{
		{
			name:   "release_before_the_rename",
			config: InstallConfig{PluginsStoreURL: PluginsStoreMirrorURL, Tag: "v4.3.0"},
			want:   "\n" + pluginsStoreLegacyKey + "=" + PluginsStoreMirrorURL + "\n",
		},
		{
			name: "resolved_release_wins_over_the_requested_tag",
			config: InstallConfig{
				PluginsStoreURL:    PluginsStoreMirrorURL,
				Tag:                "v4.3.0",
				PreResolvedRelease: &releasesource.Release{Tag: "v4.5.2"},
			},
			want: "\n" + pluginsStoreKey + "=" + PluginsStoreMirrorURL + "\n",
		},
		{
			name:   "github_build",
			config: InstallConfig{PluginsStoreURL: PluginsStoreURL},
			want:   "\n" + pluginsStoreKey + "=" + PluginsStoreURL + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := renderConfigEnv(tt.config)
			require.NoError(t, err)

			assert.Contains(t, string(out), tt.want)
		})
	}
}

func TestRenderConfigEnv_WithoutPluginsStoreURL(t *testing.T) {
	out, err := renderConfigEnv(InstallConfig{})
	require.NoError(t, err)

	assert.NotContains(t, string(out), "STORE_URL")
}

func TestExistingPluginsStoreURL(t *testing.T) {
	dir := t.TempDir()
	config := InstallConfig{ConfigDirectory: dir}

	assert.Empty(t, existingPluginsStoreURL(config))

	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "config.env"),
		[]byte("HTTP_PORT=8025\nPLUGIN_STORE_URL=\"https://plugins.example.com/api\"\n"),
		0o600,
	))

	assert.Equal(t, "https://plugins.example.com/api", existingPluginsStoreURL(config))
}
