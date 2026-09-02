package install

import (
	"context"
	"net"
	"testing"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_checkHost(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		expectedHost  string
		expectedError string
		expectedPort  string
	}{
		{
			name:         "with_http",
			host:         "http://gameap.ru",
			expectedHost: "gameap.ru",
			expectedPort: "80",
		},
		{
			name:         "with_https",
			host:         "https://gameap.ru",
			expectedHost: "gameap.ru",
			expectedPort: "80",
		},
		{
			name:         "without_http",
			host:         "gameap.ru",
			expectedHost: "gameap.ru",
			expectedPort: "80",
		},
		{
			name:         "other_port",
			host:         "https://gameap.ru:9000",
			expectedHost: "gameap.ru",
			expectedPort: "9000",
		},
		{
			name:         "with_slash",
			host:         "https://www.gameap.ru/",
			expectedHost: "www.gameap.ru",
			expectedPort: "80",
		},
		{
			name:         "ip",
			host:         "127.0.0.1",
			expectedHost: "127.0.0.1",
			expectedPort: "80",
		},
		{
			name:         "unknown_host",
			host:         "unknown_host",
			expectedHost: "unknown_host",
			expectedPort: "80",
		},
		{
			name:          "url_address",
			host:          "http://gameap.ru/en",
			expectedError: "invalid host",
		},
		{
			name:         "whitespace",
			host:         "  example.com  ",
			expectedHost: "example.com",
			expectedPort: "80",
		},
		{
			name:         "whitespace_with_http",
			host:         "  http://gameap.ru  ",
			expectedHost: "gameap.ru",
			expectedPort: "80",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			initState := panelInstallStateV3{
				Host: test.host,
			}

			resultState, err := filterAndCheckHost(initState)

			if test.expectedError == "" {
				require.NoError(t, err)
				assert.Equal(t, test.expectedHost, resultState.Host)
				assert.Equal(t, test.expectedPort, resultState.Port)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			}
		})
	}
}

func Test_filterAndCheckHostV4(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		expectedHost  string
		expectedError string
		expectedPort  string
	}{
		{
			name:         "with_http",
			host:         "http://gameap.ru",
			expectedHost: "gameap.ru",
			expectedPort: "80",
		},
		{
			name:         "with_https",
			host:         "https://gameap.ru",
			expectedHost: "gameap.ru",
			expectedPort: "80",
		},
		{
			name:         "without_http",
			host:         "gameap.ru",
			expectedHost: "gameap.ru",
			expectedPort: "80",
		},
		{
			name:         "other_port",
			host:         "https://gameap.ru:9000",
			expectedHost: "gameap.ru",
			expectedPort: "9000",
		},
		{
			name:         "with_slash",
			host:         "https://www.gameap.ru/",
			expectedHost: "www.gameap.ru",
			expectedPort: "80",
		},
		{
			name:         "ip",
			host:         "127.0.0.1",
			expectedHost: "127.0.0.1",
			expectedPort: "80",
		},
		{
			name:         "unknown_host",
			host:         "unknown_host",
			expectedHost: "unknown_host",
			expectedPort: "80",
		},
		{
			name:          "url_address",
			host:          "http://gameap.ru/en",
			expectedError: "invalid host",
		},
		{
			name:         "whitespace",
			host:         "  example.com  ",
			expectedHost: "example.com",
			expectedPort: "80",
		},
		{
			name:         "whitespace_with_http",
			host:         "  http://gameap.ru  ",
			expectedHost: "gameap.ru",
			expectedPort: "80",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			initState := panelInstallStateV4{
				Host: test.host,
			}

			resultState, err := filterAndCheckHostV4(initState)

			if test.expectedError == "" {
				require.NoError(t, err)
				assert.Equal(t, test.expectedHost, resultState.Host)
				assert.Equal(t, test.expectedPort, resultState.Port)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			}
		})
	}
}

func Test_existingPanelDetected_RequiresPreviousInstallationOnTheSamePort(t *testing.T) {
	port := servePanelHealth(t, panelHealthHandler)

	useTemporaryStateDirectory(t)
	ctx := context.Background()

	// A service answering /api/health without a recorded installation is not our panel.
	assert.False(t, existingPanelDetected(ctx, panelInstallStateV4{Port: port}))

	require.NoError(t, gameapctl.SavePanelInstallState(ctx, gameapctl.PanelInstallState{
		Version: "v4",
		Port:    port,
	}))

	assert.True(t, existingPanelDetected(ctx, panelInstallStateV4{Port: port}))
	assert.False(t, existingPanelDetected(ctx, panelInstallStateV4{Port: "1"}))
}

func Test_existingPanelDetected_RejectsSPAFallback(t *testing.T) {
	port := servePanelHealth(t, spaFallbackHandler)

	useTemporaryStateDirectory(t)
	ctx := context.Background()

	require.NoError(t, gameapctl.SavePanelInstallState(ctx, gameapctl.PanelInstallState{
		Version: "v4",
		Port:    port,
	}))

	// index.html with 200 on every path is not a ready panel.
	assert.False(t, existingPanelDetected(ctx, panelInstallStateV4{Port: port}))
}

func Test_existingPanelDetected_RejectsOtherScope(t *testing.T) {
	port := servePanelHealth(t, panelHealthHandler)

	useTemporaryStateDirectory(t)
	ctx := context.Background()

	require.NoError(t, gameapctl.SavePanelInstallState(ctx, gameapctl.PanelInstallState{
		Version: "v4",
		Scope:   gameap.ScopeUser,
		Port:    port,
	}))

	assert.False(t, existingPanelDetected(ctx, panelInstallStateV4{Scope: gameap.ScopeSystem, Port: port}))
	assert.True(t, existingPanelDetected(ctx, panelInstallStateV4{Scope: gameap.ScopeUser, Port: port}))
}

func Test_existingPanelDetected_ProbesPreviousHost(t *testing.T) {
	port := servePanelHealth(t, panelHealthHandler)

	useTemporaryStateDirectory(t)
	ctx := context.Background()

	require.NoError(t, gameapctl.SavePanelInstallState(ctx, gameapctl.PanelInstallState{
		Version: "v4",
		Host:    "127.0.0.1",
		HostIP:  "127.0.0.1",
		Port:    port,
	}))

	assert.True(t, existingPanelDetected(ctx, panelInstallStateV4{Port: port}))
}

func Test_panelProbeHosts(t *testing.T) {
	tests := []struct {
		name     string
		state    gameapctl.PanelInstallState
		expected []string
	}{
		{
			name:     "empty_state_falls_back_to_loopback",
			state:    gameapctl.PanelInstallState{},
			expected: []string{"127.0.0.1"},
		},
		{
			name:     "ip_host_is_not_duplicated",
			state:    gameapctl.PanelInstallState{Host: "2.29.29.94", HostIP: "2.29.29.94"},
			expected: []string{"2.29.29.94", "127.0.0.1"},
		},
		{
			name:     "domain_host_then_its_ip",
			state:    gameapctl.PanelInstallState{Host: "panel.example.com", HostIP: "10.0.0.5"},
			expected: []string{"panel.example.com", "10.0.0.5", "127.0.0.1"},
		},
		{
			name:     "loopback_host_once",
			state:    gameapctl.PanelInstallState{Host: "127.0.0.1"},
			expected: []string{"127.0.0.1"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, panelProbeHosts(test.state))
		})
	}
}

func Test_checkPortAvailabilityV4_ReplacesOccupiedDefaultPort(t *testing.T) {
	useTemporaryStateDirectory(t)

	busyPort := occupyFreePort(t)

	state, err := checkPortAvailabilityV4(context.Background(), panelInstallStateV4{
		NonInteractive: true,
		Host:           "127.0.0.1",
		Port:           busyPort,
	})

	require.NoError(t, err)
	assert.NotEqual(t, busyPort, state.Port)
	assert.NoError(t, utils.CheckPortAvailability("127.0.0.1", state.Port))
}

func Test_checkPortAvailabilityV4_KeepsExplicitlyChosenPort(t *testing.T) {
	useTemporaryStateDirectory(t)

	busyPort := occupyFreePort(t)

	state, err := checkPortAvailabilityV4(context.Background(), panelInstallStateV4{
		NonInteractive: true,
		Host:           "127.0.0.1",
		Port:           busyPort,
		PortInput:      busyPort,
	})

	require.Error(t, err)
	assert.Equal(t, busyPort, state.Port)
}

// occupyFreePort holds an arbitrary port until the test ends and returns it.
func occupyFreePort(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = listener.Close()
	})

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	return port
}

// useTemporaryStateDirectory keeps the install state of the machine running the tests
// out of the way, so that the checkers see no previous installation.
func useTemporaryStateDirectory(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}
