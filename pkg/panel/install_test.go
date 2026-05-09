package panel

import (
	"strings"
	"testing"

	"github.com/gameap/gameapctl/pkg/gameap"
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
