package packagemanager

import (
	"testing"

	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parsePacmanInfoOutput(t *testing.T) {
	out := []byte(`
Repository      : multilib
Name            : lib32-gcc-libs
Version         : 14.2.1+r134+gab884fffe3fc-1
Description     : 32-bit runtime libraries shipped by GCC
Architecture    : x86_64
URL             : https://gcc.gnu.org
Licenses        : GPL-3.0-or-later LGPL-3.0-or-later
Download Size   : 1.20 MiB
Installed Size  : 3.49 MiB

Repository      : multilib
Name            : lib32-zlib
Version         : 1.3.1-1
Description     : Compression library implementing the deflate compression method
Architecture    : x86_64
URL             : https://www.zlib.net/
Download Size   : 60.99 KiB
Installed Size  : 168.00 KiB

	`)

	parsed := parsePacmanInfoOutput(out)

	require.Equal(t, []PackageInfo{
		{
			Name:         "lib32-gcc-libs",
			Version:      "14.2.1+r134+gab884fffe3fc-1",
			Architecture: "x86_64",
			Size:         "1.20 MiB",
			Description:  "32-bit runtime libraries shipped by GCC",
		},
		{
			Name:         "lib32-zlib",
			Version:      "1.3.1-1",
			Architecture: "x86_64",
			Size:         "60.99 KiB",
			Description:  "Compression library implementing the deflate compression method",
		},
	}, parsed)
}

func Test_newExtendedPacman(t *testing.T) {
	t.Run("creates extended pacman with packages", func(t *testing.T) {
		osInfo := osinfo.Info{
			Distribution: osinfo.DistributionArch,
			Platform:     osinfo.PlatformAmd64,
		}

		mockPacman := &pacman{}
		ext, err := newExtendedPacman(osInfo, mockPacman)

		require.NoError(t, err)
		require.NotNil(t, ext)
		assert.NotNil(t, ext.packages)
		assert.NotNil(t, ext.underlined)

		_, ok := ext.strategy.(noopStrategy)
		assert.True(t, ok, "pacman must use noopStrategy")
	})
}
