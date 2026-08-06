package panel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPanelReadWritePaths(t *testing.T) {
	config := InstallConfig{
		DataDirectory:      "/var/lib/gameap",
		FilesLocalBasePath: "/srv/gameap",
	}

	assert.Equal(t, "/var/lib/gameap /srv/gameap", panelReadWritePaths(config))
}

func TestPanelReadWritePaths_LegacyPath(t *testing.T) {
	config := InstallConfig{
		DataDirectory:      "/var/lib/gameap",
		FilesLocalBasePath: "/srv/gameap",
		LegacyPath:         "/var/www/gameap",
	}

	assert.Equal(t, "/var/lib/gameap /srv/gameap /var/www/gameap", panelReadWritePaths(config))
}
