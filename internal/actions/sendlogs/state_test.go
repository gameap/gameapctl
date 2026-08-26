package sendlogs

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/stretchr/testify/require"
)

func Test_collectInstallState(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	ctx := context.Background()

	require.NoError(t, gameapctl.SavePanelInstallState(ctx, gameapctl.PanelInstallState{
		Host:           "127.0.0.1",
		Port:           "80",
		DBUsername:     "gameap",
		DBPassword:     "db-secret",
		DBRootPassword: "root-secret",
		AdminPassword:  "admin-secret",
	}))

	destinationDir := t.TempDir()

	require.NoError(t, collectInstallState(ctx, destinationDir))

	contents, err := os.ReadFile(filepath.Join(destinationDir, "state", "panel_install_state.json"))
	require.NoError(t, err)

	saved := string(contents)
	require.NotContains(t, saved, "db-secret")
	require.NotContains(t, saved, "root-secret")
	require.NotContains(t, saved, "admin-secret")
	require.Contains(t, saved, `"dbUsername": "gameap"`)
	require.Contains(t, saved, maskedValue)
}

func Test_maskPanelInstallState(t *testing.T) {
	state := gameapctl.PanelInstallState{
		Version:        "4",
		Host:           "127.0.0.1",
		Port:           "80",
		Database:       "mysql",
		DBHost:         "127.0.0.1",
		DBPort:         "9306",
		DBName:         "gameap",
		DBUsername:     "gameap",
		DBPassword:     "db-secret",
		DBRootPassword: "root-secret",
		AdminPassword:  "admin-secret",
	}

	masked := maskPanelInstallState(state)

	require.Equal(t, maskedValue, masked.DBPassword)
	require.Equal(t, maskedValue, masked.DBRootPassword)
	require.Equal(t, maskedValue, masked.AdminPassword)
	require.Equal(t, "gameap", masked.DBUsername)
	require.Equal(t, "9306", masked.DBPort)
}

func Test_maskPanelInstallState_EmptyValuesAreNotMasked(t *testing.T) {
	masked := maskPanelInstallState(gameapctl.PanelInstallState{Host: "127.0.0.1"})

	require.Empty(t, masked.DBPassword)
	require.Empty(t, masked.DBRootPassword)
	require.Empty(t, masked.AdminPassword)
}

// Fails when a new secret field is added to the state, but not to the mask.
func Test_maskPanelInstallState_AllSecretFieldsAreMasked(t *testing.T) {
	state := gameapctl.PanelInstallState{}
	value := reflect.ValueOf(&state).Elem()

	for i := range value.NumField() {
		if value.Field(i).Kind() == reflect.String {
			value.Field(i).SetString("secret-value")
		}
	}

	masked := reflect.ValueOf(maskPanelInstallState(state))

	for i := range masked.NumField() {
		name := masked.Type().Field(i).Name
		if !isSecretFieldName(name) {
			continue
		}

		require.Equalf(t, maskedValue, masked.Field(i).String(), "field %s is not masked", name)
	}
}

func Test_maskDaemonInstallState(t *testing.T) {
	tests := []struct {
		name       string
		connectURL string
		expected   string
	}{
		{
			name:       "setup key is hidden",
			connectURL: "grpc://gameap.local:8081/qwerty-setup-key",
			expected:   "grpc://gameap.local:8081/***",
		},
		{
			name:       "empty url",
			connectURL: "",
			expected:   "",
		},
		{
			name:       "unparsable url",
			connectURL: "::not-an-url::",
			expected:   maskedValue,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			masked := maskDaemonInstallState(gameapctl.DaemonInstallState{
				Host:       "gameap.local",
				ConnectURL: test.connectURL,
			})

			require.Equal(t, test.expected, masked.ConnectURL)
			require.Equal(t, "gameap.local", masked.Host)
		})
	}
}

func isSecretFieldName(name string) bool {
	for _, part := range []string{"Password", "Secret", "Token", "Key"} {
		if strings.Contains(name, part) {
			return true
		}
	}

	return false
}
