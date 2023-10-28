package panelinstall

import (
	"testing"

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
			name:          "unknown_host",
			host:          "unknown.host.unknown",
			expectedError: "failed to lookup ip: lookup unknown.host.unknown: no such host",
		},
		{
			name:          "url_address",
			host:          "http://gameap.ru/en",
			expectedError: "invalid host",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			initState := panelInstallState{
				Host: test.host,
			}

			resultState, err := filterAndCheckHost(initState)

			if test.expectedError == "" {
				require.NoError(t, err)
				assert.Equal(t, test.expectedHost, resultState.Host)
				assert.Equal(t, test.expectedPort, resultState.Port)
			} else {
				assert.Equal(t, test.expectedError, err.Error())
			}
		})
	}
}
