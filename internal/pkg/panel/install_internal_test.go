package panel

import (
	"testing"

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
