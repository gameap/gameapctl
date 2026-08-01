package panel

import (
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/systemd"
)

// CheckScope verifies that the environment can actually manage the panel in the
// given scope. Callers use it to fail before doing any work.
func CheckScope(scope string) error {
	if scope != gameap.ScopeUser {
		return nil
	}

	return systemd.CheckUserManager()
}
