package utils_test

import (
	"testing"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/stretchr/testify/assert"
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
