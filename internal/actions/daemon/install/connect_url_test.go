package install

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ParseConnectURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		want    ConnectInfo
		wantErr bool
	}{
		{
			name:   "valid url",
			rawURL: "grpc://example.com:9090/secretkey",
			want:   ConnectInfo{Host: "example.com", Port: 9090, SetupKey: "secretkey"},
		},
		{
			name:   "valid url with ip",
			rawURL: "grpc://192.168.1.1:31717/abc123",
			want:   ConnectInfo{Host: "192.168.1.1", Port: 31717, SetupKey: "abc123"},
		},
		{
			name:   "key with slashes",
			rawURL: "grpc://example.com:9090/some/deep/key",
			want:   ConnectInfo{Host: "example.com", Port: 9090, SetupKey: "some/deep/key"},
		},
		{
			name:    "wrong scheme http",
			rawURL:  "http://example.com:9090/key",
			wantErr: true,
		},
		{
			name:    "wrong scheme https",
			rawURL:  "https://example.com:9090/key",
			wantErr: true,
		},
		{
			name:    "missing port",
			rawURL:  "grpc://example.com/key",
			wantErr: true,
		},
		{
			name:    "missing host",
			rawURL:  "grpc://:9090/key",
			wantErr: true,
		},
		{
			name:    "missing key no slash",
			rawURL:  "grpc://example.com:9090",
			wantErr: true,
		},
		{
			name:    "missing key empty path",
			rawURL:  "grpc://example.com:9090/",
			wantErr: true,
		},
		{
			name:    "port zero",
			rawURL:  "grpc://example.com:0/key",
			wantErr: true,
		},
		{
			name:    "port too large",
			rawURL:  "grpc://example.com:70000/key",
			wantErr: true,
		},
		{
			name:    "non-numeric port",
			rawURL:  "grpc://example.com:abc/key",
			wantErr: true,
		},
		{
			name:    "empty string",
			rawURL:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConnectURL(tt.rawURL)
			if tt.wantErr {
				require.Error(t, err)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
