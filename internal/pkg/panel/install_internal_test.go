package panel

import (
	"net"
	"testing"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func Test_createHealthURL(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		https    bool
		expected string
	}{
		{name: "default_http_port", host: "127.0.0.1", port: "80", expected: "http://127.0.0.1/api/health"},
		{name: "custom_port", host: "127.0.0.1", port: "8025", expected: "http://127.0.0.1:8025/api/health"},
		{name: "https_default_port", host: "example.com", port: "443", https: true, expected: "https://example.com/api/health"},
		{name: "http_on_443_keeps_port", host: "127.0.0.1", port: "443", expected: "http://127.0.0.1:443/api/health"},
		{name: "https_on_80_keeps_port", host: "example.com", port: "80", https: true, expected: "https://example.com:80/api/health"},
		{name: "ipv6_custom_port", host: "::1", port: "8025", expected: "http://[::1]:8025/api/health"},
		{name: "ipv6_default_port", host: "2a01:4f9:c015:fafb::1", port: "80", expected: "http://[2a01:4f9:c015:fafb::1]/api/health"},
		{name: "bracketed_ipv6", host: "[::1]", port: "8025", expected: "http://[::1]:8025/api/health"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, createHealthURL(test.host, test.port, test.https, "/api/health"))
		})
	}
}

func Test_isLocalHost(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		{name: "localhost", host: "localhost", expected: true},
		{name: "loopback_ipv4", host: "127.0.0.1", expected: true},
		{name: "loopback_ipv6", host: "::1", expected: true},
		{name: "unspecified_ipv4", host: "0.0.0.0", expected: true},
		{name: "unspecified_ipv6", host: "::", expected: true},
		{name: "documentation_address", host: "203.0.113.10", expected: false},
		{name: "domain", host: "panel.example.com", expected: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, isLocalHost(test.host))
		})
	}
}

// Test_isLocalHost_interfaceAddress covers the case the panel is configured
// with on a server that answers on its public address only: HTTP_HOST holds an
// address of a local interface, and a probe of it stays on this machine.
func Test_isLocalHost_interfaceAddress(t *testing.T) {
	var address string

	for _, ip := range utils.DetectIPs() {
		parsed := net.ParseIP(ip)
		if parsed != nil && !parsed.IsLoopback() && !parsed.IsUnspecified() {
			address = ip

			break
		}
	}

	if address == "" {
		t.Skip("no non-loopback interface address on this machine")
	}

	assert.True(t, isLocalHost(address))
}
