package install

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
			result := removeLocalIPs(test.ips)
			assert.Equal(t, test.want, result)
		})
	}
}
