package panel

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testInstallConfig() InstallConfig {
	return InstallConfig{
		ConfigDirectory:    "/etc/gameap",
		DataDirectory:      "/var/lib/gameap",
		BinaryPath:         "/usr/local/bin/gameap",
		User:               "gameap",
		Group:              "gameap",
		FilesLocalBasePath: "/srv/gameap",
	}
}

func TestRenderSystemdUnit(t *testing.T) {
	unitBytes, err := renderSystemdUnit(testInstallConfig())
	require.NoError(t, err)
	unit := string(unitBytes)

	assert.Contains(t, unit, "User=gameap")
	assert.Contains(t, unit, "Group=gameap")
	assert.Contains(t, unit, "WorkingDirectory=/var/lib/gameap")
	assert.Contains(t, unit, "ExecStart=/usr/local/bin/gameap")
	assert.Contains(t, unit, "EnvironmentFile=/etc/gameap/config.env")
	assert.Contains(t, unit, "ReadWritePaths=/var/lib/gameap /srv/gameap\n")
}

func TestRenderSystemdUnit_StartLimitInUnitSection(t *testing.T) {
	unitBytes, err := renderSystemdUnit(testInstallConfig())
	require.NoError(t, err)
	unit := string(unitBytes)

	assert.Contains(t, unit, "StartLimitIntervalSec=60")
	assert.Contains(t, unit, "StartLimitBurst=5")
	assert.NotContains(t, unit, "StartLimitInterval=60\n")
	assert.NotContains(t, unit, "StartLimitBurst=3")

	serviceSectionIndex := strings.Index(unit, "[Service]")
	require.Positive(t, serviceSectionIndex)
	assert.Less(t, strings.Index(unit, "StartLimitIntervalSec=60"), serviceSectionIndex)
	assert.Less(t, strings.Index(unit, "StartLimitBurst=5"), serviceSectionIndex)
}

func TestRenderSystemdUnit_DatabaseOrdering(t *testing.T) {
	unitBytes, err := renderSystemdUnit(testInstallConfig())
	require.NoError(t, err)
	unit := string(unitBytes)

	assert.Contains(
		t,
		unit,
		"After=network.target network-online.target postgresql.service mysql.service mariadb.service",
	)
}

func TestRenderSystemdUnit_LegacyPath(t *testing.T) {
	config := testInstallConfig()
	config.LegacyPath = "/var/www/gameap"

	unitBytes, err := renderSystemdUnit(config)
	require.NoError(t, err)

	assert.Contains(t, string(unitBytes), "ReadWritePaths=/var/lib/gameap /srv/gameap /var/www/gameap\n")
}
