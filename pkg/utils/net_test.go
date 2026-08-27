package utils_test

import (
	"net"
	"strconv"
	"testing"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_IsIPv4(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{
			name: "valid",
			ip:   "127.0.0.1",
			want: true,
		},
		{
			name: "invalid",
			ip:   "127.0.0.256",
			want: false,
		},
		{
			name: "ipv6",
			ip:   "2001:4860:4860::8888",
			want: false,
		},
		{
			name: "domain",
			ip:   "gameap.ru",
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := utils.IsIPv4(test.ip)

			assert.Equal(t, test.want, result)
		})
	}
}

func Test_IsIPv6(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{
			name: "valid",
			ip:   "2001:4860:4860::8888",
			want: true,
		},
		{
			name: "invalid",
			ip:   "2001:4860:4860::888g",
			want: false,
		},
		{
			name: "ipv4",
			ip:   "127.0.0.1",
			want: false,
		},
		{
			name: "domain",
			ip:   "gameap.ru",
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := utils.IsIPv6(test.ip)

			assert.Equal(t, test.want, result)
		})
	}
}

func Test_CheckPortAvailability(t *testing.T) {
	busyPort, _ := listenOnFreePort(t)

	assert.Error(t, utils.CheckPortAvailability("127.0.0.1", busyPort))
	assert.NoError(t, utils.CheckPortAvailability("127.0.0.1", freePort(t)))
}

func Test_CheckPortAvailability_WildcardSeesIPv4OnlyListener(t *testing.T) {
	listener, err := net.Listen("tcp4", "0.0.0.0:0")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = listener.Close()
	})

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	assert.Error(t, utils.CheckPortAvailability("", port))
	assert.Error(t, utils.CheckPortAvailability("0.0.0.0", port))
}

func Test_FindAvailablePort_PreferredIsFree(t *testing.T) {
	preferred := freePort(t)

	result, found := utils.FindAvailablePort("127.0.0.1", preferred)

	assert.True(t, found)
	assert.Equal(t, preferred, result)
}

func Test_FindAvailablePort_PreferredIsBusy(t *testing.T) {
	preferred, _ := listenOnFreePort(t)

	result, found := utils.FindAvailablePort("127.0.0.1", preferred)

	require.True(t, found)
	assert.NotEqual(t, preferred, result)

	resultPort, err := strconv.Atoi(result)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resultPort, 8025)
	assert.LessOrEqual(t, resultPort, 8034)
}

func Test_FindAvailablePort_NoFreePortWithinLimit(t *testing.T) {
	for port := 8025; port <= 8034; port++ {
		listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err != nil {
			t.Skipf("port %d is occupied by something else, cannot run the test: %s", port, err)
		}

		t.Cleanup(func() {
			_ = listener.Close()
		})
	}

	preferred, _ := listenOnFreePort(t)

	result, found := utils.FindAvailablePort("127.0.0.1", preferred)

	assert.False(t, found)
	assert.Empty(t, result)
}

// listenOnFreePort occupies an arbitrary free port until the test ends and returns it.
func listenOnFreePort(t *testing.T) (string, net.Listener) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = listener.Close()
	})

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	return port, listener
}

// freePort returns a port that was free a moment ago.
func freePort(t *testing.T) string {
	t.Helper()

	port, listener := listenOnFreePort(t)
	require.NoError(t, listener.Close())

	return port
}
