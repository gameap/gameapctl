package packagemanager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_windowsCommandExecutable(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "path without arguments",
			input: `C:\nginx\nginx.exe`,
			want:  `C:\nginx\nginx.exe`,
		},
		{
			name:  "path with arguments",
			input: `C:\gameap\tools\shawl\shawl.exe run --name nginx -- nginx.exe`,
			want:  `C:\gameap\tools\shawl\shawl.exe`,
		},
		{
			name:  "quoted path with spaces",
			input: `"C:\Program Files\nginx\nginx.exe" -c conf\nginx.conf`,
			want:  `C:\Program Files\nginx\nginx.exe`,
		},
		{
			name:  "quoted path without arguments",
			input: `"C:\Program Files\nginx\nginx.exe"`,
			want:  `C:\Program Files\nginx\nginx.exe`,
		},
		{
			name:  "surrounded by spaces",
			input: `   C:\nginx\nginx.exe   `,
			want:  `C:\nginx\nginx.exe`,
		},
		{
			name:  "unterminated quote",
			input: `"C:\nginx\nginx.exe`,
			want:  `C:\nginx\nginx.exe`,
		},
		{
			name:  "empty command line",
			input: "",
			want:  "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, windowsCommandExecutable(test.input))
		})
	}
}
