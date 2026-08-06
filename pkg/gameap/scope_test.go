package gameap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScopeOrDefault(t *testing.T) {
	assert.Equal(t, ScopeSystem, ScopeOrDefault(""))
	assert.Equal(t, ScopeSystem, ScopeOrDefault(ScopeSystem))
	assert.Equal(t, ScopeUser, ScopeOrDefault(ScopeUser))
}

func TestResolveScopeForOS(t *testing.T) {
	tests := []struct {
		name        string
		scope       string
		goos        string
		want        string
		errContains string
	}{
		{name: "empty defaults to system", scope: "", goos: "linux", want: ScopeSystem},
		{name: "system on linux", scope: ScopeSystem, goos: "linux", want: ScopeSystem},
		{name: "system on windows", scope: ScopeSystem, goos: "windows", want: ScopeSystem},
		{name: "user on linux", scope: ScopeUser, goos: "linux", want: ScopeUser},
		{name: "user on windows", scope: ScopeUser, goos: "windows", errContains: "requires Linux"},
		{name: "user on darwin", scope: ScopeUser, goos: "darwin", errContains: "requires Linux"},
		{name: "unknown", scope: "nonsense", goos: "linux", errContains: "unknown --scope value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveScopeForOS(tt.scope, tt.goos)

			if tt.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
