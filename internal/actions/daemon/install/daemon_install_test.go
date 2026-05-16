package install

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_chooseBestIP(t *testing.T) {
	tests := []struct {
		name string
		ips  []string
		want string
	}{
		{
			name: "success",
			ips:  []string{"127.0.0.1", "8.8.8.8"},
			want: "8.8.8.8",
		},
		{
			name: "success_reverse",
			ips:  []string{"8.8.8.8", "127.0.0.1"},
			want: "8.8.8.8",
		},
		{
			name: "without_public",
			ips:  []string{"172.0.0.1", "192.168.0.1", "10.0.0.0", "127.0.0.1"},
			want: "192.168.0.1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, _ := chooseBestIP(test.ips)
			assert.Equal(t, test.want, result)
		})
	}
}

func Test_removeLocalIPs(t *testing.T) {
	tests := []struct {
		name string
		ips  []string
		want []string
	}{
		{
			name: "ipv4 only",
			ips:  []string{"127.0.0.1", "8.8.8.8"},
			want: []string{"8.8.8.8"},
		},
		{
			name: "with ipv6",
			ips:  []string{"127.0.0.1", "8.8.8.8", "::1", "fe80::a00:27ff:fe8e:8aa8", "2001:4860:4860::8844"},
			want: []string{"8.8.8.8", "2001:4860:4860::8844"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := utils.RemoveLocalIPs(test.ips)
			assert.Equal(t, test.want, result)
		})
	}
}

func Test_parseConfigOverrides(t *testing.T) {
	tests := []struct {
		name      string
		configEnv string
		want      map[string]string
	}{
		{
			name:      "empty config",
			configEnv: "",
			want:      map[string]string{},
		},
		{
			name:      "single value",
			configEnv: base64.StdEncoding.EncodeToString([]byte("process_manager.name=systemd")),
			want:      map[string]string{"process_manager.name": "systemd"},
		},
		{
			name:      "multiple values",
			configEnv: base64.StdEncoding.EncodeToString([]byte("process_manager.name=podman;process_manager.config.image=debian:bookworm-slim")),
			want: map[string]string{
				"process_manager.name":         "podman",
				"process_manager.config.image": "debian:bookworm-slim",
			},
		},
		{
			name:      "with spaces",
			configEnv: base64.StdEncoding.EncodeToString([]byte("  process_manager.name = systemd  ;  log_level = info  ")),
			want: map[string]string{
				"process_manager.name": "systemd",
				"log_level":            "info",
			},
		},
		{
			name:      "empty pairs skipped",
			configEnv: base64.StdEncoding.EncodeToString([]byte("process_manager.name=systemd;;log_level=debug")),
			want: map[string]string{
				"process_manager.name": "systemd",
				"log_level":            "debug",
			},
		},
		{
			name:      "plain text single value",
			configEnv: "process_manager.name=systemd",
			want:      map[string]string{"process_manager.name": "systemd"},
		},
		{
			name:      "plain text multiple values",
			configEnv: "process_manager.name=podman;listen_ip=0.0.0.0",
			want: map[string]string{
				"process_manager.name": "podman",
				"listen_ip":            "0.0.0.0",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := parseConfigOverrides(test.configEnv)
			assert.Equal(t, test.want, result)
		})
	}
}

func Test_applyConfigOverrides(t *testing.T) {
	t.Run("process_manager.name", func(t *testing.T) {
		cfg := &DaemonConfig{
			ProcessManager: ProcessManagerConfig{Name: "tmux"},
		}
		applyConfigOverrides(cfg, map[string]string{"process_manager.name": "systemd"})
		assert.Equal(t, "systemd", cfg.ProcessManager.Name)
	})

	t.Run("process_manager.config", func(t *testing.T) {
		cfg := &DaemonConfig{}
		applyConfigOverrides(cfg, map[string]string{
			"process_manager.config.image":   "debian:bookworm-slim",
			"process_manager.config.network": "host",
		})
		require.Len(t, cfg.ProcessManager.Config, 2)
		assert.Equal(t, "debian:bookworm-slim", cfg.ProcessManager.Config["image"])
		assert.Equal(t, "host", cfg.ProcessManager.Config["network"])
	})

	t.Run("listen_ip", func(t *testing.T) {
		cfg := &DaemonConfig{ListenIP: "0.0.0.0"}
		applyConfigOverrides(cfg, map[string]string{"listen_ip": "192.168.1.1"})
		assert.Equal(t, "192.168.1.1", cfg.ListenIP)
	})

	t.Run("listen_port", func(t *testing.T) {
		cfg := &DaemonConfig{ListenPort: 31717}
		applyConfigOverrides(cfg, map[string]string{"listen_port": "8080"})
		assert.Equal(t, 8080, cfg.ListenPort)
	})

	t.Run("listen_port invalid", func(t *testing.T) {
		cfg := &DaemonConfig{ListenPort: 31717}
		applyConfigOverrides(cfg, map[string]string{"listen_port": "invalid"})
		assert.Equal(t, 31717, cfg.ListenPort)
	})

	t.Run("log_level", func(t *testing.T) {
		cfg := &DaemonConfig{LogLevel: "debug"}
		applyConfigOverrides(cfg, map[string]string{"log_level": "info"})
		assert.Equal(t, "info", cfg.LogLevel)
	})

	t.Run("work_path", func(t *testing.T) {
		cfg := &DaemonConfig{WorkPath: "/srv/gameap"}
		applyConfigOverrides(cfg, map[string]string{"work_path": "/var/gameap"})
		assert.Equal(t, "/var/gameap", cfg.WorkPath)
	})

	t.Run("steamcmd_path", func(t *testing.T) {
		cfg := &DaemonConfig{SteamCMDPath: "/srv/steamcmd"}
		applyConfigOverrides(cfg, map[string]string{"steamcmd_path": "/opt/steamcmd"})
		assert.Equal(t, "/opt/steamcmd", cfg.SteamCMDPath)
	})

	t.Run("multiple overrides", func(t *testing.T) {
		cfg := &DaemonConfig{
			ListenIP:   "0.0.0.0",
			ListenPort: 31717,
			LogLevel:   "debug",
			ProcessManager: ProcessManagerConfig{
				Name: "tmux",
			},
		}
		applyConfigOverrides(cfg, map[string]string{
			"process_manager.name":         "podman",
			"process_manager.config.image": "debian:bookworm-slim",
			"listen_ip":                    "192.168.1.100",
			"log_level":                    "error",
		})
		assert.Equal(t, "podman", cfg.ProcessManager.Name)
		assert.Equal(t, "debian:bookworm-slim", cfg.ProcessManager.Config["image"])
		assert.Equal(t, "192.168.1.100", cfg.ListenIP)
		assert.Equal(t, "error", cfg.LogLevel)
		assert.Equal(t, 31717, cfg.ListenPort)
	})
}

func Test_applyWindowsNetworkServiceUser_addsKeyWhenMissing(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "gameap-daemon.yaml")
	require.NoError(t, os.WriteFile(p, []byte("api_host: \"https://panel.example.com\"\napi_key: \"secret\"\n"), 0600))

	require.NoError(t, applyWindowsNetworkServiceUser(p))

	out, err := os.ReadFile(p)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &raw))
	assert.Equal(t, true, raw["use_network_service_user"])
	assert.Equal(t, "https://panel.example.com", raw["api_host"])
	assert.Equal(t, "secret", raw["api_key"])
}

func Test_applyWindowsNetworkServiceUser_preservesExistingFalse(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "gameap-daemon.yaml")
	require.NoError(t, os.WriteFile(p, []byte("api_host: \"x\"\nuse_network_service_user: false\n"), 0600))

	require.NoError(t, applyWindowsNetworkServiceUser(p))

	out, err := os.ReadFile(p)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &raw))
	assert.Equal(t, false, raw["use_network_service_user"])
}

func Test_applyWindowsNetworkServiceUser_preservesOtherFields(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "gameap-daemon.yaml")
	original := `ds_id: 42
api_host: "https://panel.example.com"
api_key: "secret"
listen_ip: "0.0.0.0"
listen_port: 31717
work_path: "C:\\gameap\\daemon"
process_manager:
  name: shawl
  config:
    foo: bar
`
	require.NoError(t, os.WriteFile(p, []byte(original), 0600))

	require.NoError(t, applyWindowsNetworkServiceUser(p))

	out, err := os.ReadFile(p)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &raw))

	assert.Equal(t, uint64(42), raw["ds_id"])
	assert.Equal(t, "https://panel.example.com", raw["api_host"])
	assert.Equal(t, "secret", raw["api_key"])
	assert.Equal(t, "0.0.0.0", raw["listen_ip"])
	assert.Equal(t, uint64(31717), raw["listen_port"])
	assert.Equal(t, "C:\\gameap\\daemon", raw["work_path"])
	assert.Equal(t, true, raw["use_network_service_user"])

	pm, ok := raw["process_manager"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "shawl", pm["name"])
	pmCfg, ok := pm["config"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "bar", pmCfg["foo"])
}

func Test_applyWindowsNetworkServiceUser_errorOnMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	err := applyWindowsNetworkServiceUser(missing)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read daemon config")
}

func Test_applySteamCMDPath_addsKeyWhenMissing(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "gameap-daemon.yaml")
	require.NoError(t, os.WriteFile(p, []byte("api_host: \"https://panel.example.com\"\napi_key: \"secret\"\n"), 0600))

	require.NoError(t, applySteamCMDPath(p, "/srv/gameap/steamcmd"))

	out, err := os.ReadFile(p)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &raw))
	assert.Equal(t, "/srv/gameap/steamcmd", raw["steamcmd_path"])
	assert.Equal(t, "https://panel.example.com", raw["api_host"])
	assert.Equal(t, "secret", raw["api_key"])
}

func Test_applySteamCMDPath_preservesExistingNonEmpty(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "gameap-daemon.yaml")
	require.NoError(t, os.WriteFile(p, []byte("steamcmd_path: \"/opt/steamcmd\"\n"), 0600))

	require.NoError(t, applySteamCMDPath(p, "/srv/gameap/steamcmd"))

	out, err := os.ReadFile(p)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &raw))
	assert.Equal(t, "/opt/steamcmd", raw["steamcmd_path"])
}

func Test_applySteamCMDPath_replacesEmptyString(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "gameap-daemon.yaml")
	require.NoError(t, os.WriteFile(p, []byte("steamcmd_path: \"\"\napi_key: \"secret\"\n"), 0600))

	require.NoError(t, applySteamCMDPath(p, "/srv/gameap/steamcmd"))

	out, err := os.ReadFile(p)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &raw))
	assert.Equal(t, "/srv/gameap/steamcmd", raw["steamcmd_path"])
	assert.Equal(t, "secret", raw["api_key"])
}

func Test_applySteamCMDPath_preservesOtherFields(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "gameap-daemon.yaml")
	original := `ds_id: 42
api_host: "https://panel.example.com"
api_key: "secret"
listen_ip: "0.0.0.0"
listen_port: 31717
work_path: "/srv/gameap"
process_manager:
  name: systemd
  config:
    foo: bar
`
	require.NoError(t, os.WriteFile(p, []byte(original), 0600))

	require.NoError(t, applySteamCMDPath(p, "/srv/gameap/steamcmd"))

	out, err := os.ReadFile(p)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &raw))

	assert.Equal(t, uint64(42), raw["ds_id"])
	assert.Equal(t, "https://panel.example.com", raw["api_host"])
	assert.Equal(t, "secret", raw["api_key"])
	assert.Equal(t, "0.0.0.0", raw["listen_ip"])
	assert.Equal(t, uint64(31717), raw["listen_port"])
	assert.Equal(t, "/srv/gameap", raw["work_path"])
	assert.Equal(t, "/srv/gameap/steamcmd", raw["steamcmd_path"])

	pm, ok := raw["process_manager"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "systemd", pm["name"])
	pmCfg, ok := pm["config"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "bar", pmCfg["foo"])
}

func Test_applySteamCMDPath_errorOnMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	err := applySteamCMDPath(missing, "/srv/gameap/steamcmd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read daemon config")
}

func Test_applySteamCMDPath_noopOnEmptyArgument(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	require.NoError(t, applySteamCMDPath(missing, ""))
	require.NoError(t, applySteamCMDPath(missing, "   "))
}
