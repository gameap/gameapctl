package oscore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestrictDirectoryAccessArgs(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "default work path",
			path: `C:\gameap`,
		},
		{
			name: "path with spaces stays a single argument",
			path: `D:\Game Servers\gameap`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := restrictDirectoryAccessArgs(test.path)

			require.Len(t, args, 6)
			assert.Equal(t, test.path, args[0])
			assert.Equal(t, []string{
				"/inheritance:r",
				"/grant:r",
				"*S-1-5-18:(OI)(CI)F",
				"*S-1-5-32-544:(OI)(CI)F",
				"*S-1-5-32-545:(OI)(CI)RX",
			}, args[1:])
		})
	}
}

func TestRestrictDirectoryOwnerArgs(t *testing.T) {
	path := `D:\Game Servers\gameap`

	assert.Equal(t, []string{path, "/setowner", "*S-1-5-32-544"}, restrictDirectoryOwnerArgs(path))
}
