package daemoninstall

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
