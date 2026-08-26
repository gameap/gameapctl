package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_maskedArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "flag with a value in one argument",
			args:     []string{"gameapctl", "panel", "install", "--database-password=qwerty"},
			expected: []string{"gameapctl", "panel", "install", "--database-password=***"},
		},
		{
			name:     "flag with a value in the next argument",
			args:     []string{"gameapctl", "panel", "install", "--database-password", "qwerty"},
			expected: []string{"gameapctl", "panel", "install", "--database-password", "***"},
		},
		{
			name:     "regular flags are kept",
			args:     []string{"gameapctl", "panel", "install", "--host=127.0.0.1", "--port", "80"},
			expected: []string{"gameapctl", "panel", "install", "--host=127.0.0.1", "--port", "80"},
		},
		{
			name:     "value that looks like a flag name is kept",
			args:     []string{"gameapctl", "panel", "install", "--host", "password.example.com"},
			expected: []string{"gameapctl", "panel", "install", "--host", "password.example.com"},
		},
		{
			name:     "setup key is masked",
			args:     []string{"gameapctl", "daemon", "install", "--setup-key", "abcdef"},
			expected: []string{"gameapctl", "daemon", "install", "--setup-key", "***"},
		},
		{
			name:     "connect URL in one argument is masked",
			args:     []string{"gameapctl", "daemon", "install", "--connect=grpc://panel.example.com:31717/abcdef"},
			expected: []string{"gameapctl", "daemon", "install", "--connect=***"},
		},
		{
			name:     "connect URL in the next argument is masked",
			args:     []string{"gameapctl", "daemon", "install", "--connect", "grpc://panel.example.com:31717/abcdef"},
			expected: []string{"gameapctl", "daemon", "install", "--connect", "***"},
		},
		{
			name:     "env entry in one argument is masked",
			args:     []string{"gameapctl", "panel", "letsencrypt", "setup", "--env=CLOUDFLARE_EMAIL=admin@example.com"},
			expected: []string{"gameapctl", "panel", "letsencrypt", "setup", "--env=***"},
		},
		{
			name:     "env entry in the next argument is masked",
			args:     []string{"gameapctl", "panel", "letsencrypt", "setup", "--env", "CLOUDFLARE_EMAIL=admin@example.com"},
			expected: []string{"gameapctl", "panel", "letsencrypt", "setup", "--env", "***"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, maskedArgs(test.args))
		})
	}
}
