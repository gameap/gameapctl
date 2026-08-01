package install

import (
	"testing"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPortForScope(t *testing.T) {
	assert.Equal(t, "80", defaultPortForScope(gameap.ScopeSystem))
	assert.Equal(t, "80", defaultPortForScope(""))
	assert.Equal(t, "8025", defaultPortForScope(gameap.ScopeUser))
}

func TestValidateDatabaseForScope(t *testing.T) {
	tests := []struct {
		name     string
		scope    string
		database string
		existing bool
		wantErr  bool
	}{
		{name: "system scope installs postgres", scope: gameap.ScopeSystem, database: postgresDatabase},
		{name: "system scope installs mysql", scope: gameap.ScopeSystem, database: mysqlDatabase},
		{name: "user scope with sqlite", scope: gameap.ScopeUser, database: sqliteDatabase},
		{name: "user scope without database", scope: gameap.ScopeUser, database: noneDatabase},
		{name: "user scope with existing postgres", scope: gameap.ScopeUser, database: postgresDatabase, existing: true},
		{name: "user scope with existing mysql", scope: gameap.ScopeUser, database: mysqlDatabase, existing: true},
		{name: "user scope installs postgres", scope: gameap.ScopeUser, database: postgresDatabase, wantErr: true},
		{name: "user scope installs mysql", scope: gameap.ScopeUser, database: mysqlDatabase, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDatabaseForScope(panelInstallStateV4{
				Scope:            tt.scope,
				Database:         tt.database,
				ExistingDatabase: tt.existing,
			})

			if !tt.wantErr {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), "not possible with --scope=user")
		})
	}
}

func TestCmdLineFromPanelInstallStateV4_UserScope(t *testing.T) {
	cmdLine := cmdLineFromPanelInstallStateV4(panelInstallStateV4{
		Scope:    gameap.ScopeUser,
		Host:     "127.0.0.1",
		Port:     "8025",
		Database: sqliteDatabase,
	})

	assert.Contains(t, cmdLine, "panel install --scope=user")
	assert.Contains(t, cmdLine, "--port=8025")
}

func TestCmdLineFromPanelInstallStateV4_SystemScope(t *testing.T) {
	cmdLine := cmdLineFromPanelInstallStateV4(panelInstallStateV4{
		Scope:    gameap.ScopeSystem,
		Host:     "127.0.0.1",
		Port:     "80",
		Database: sqliteDatabase,
	})

	assert.NotContains(t, cmdLine, "--scope")
}
