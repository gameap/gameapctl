//go:build !linux

package panel

import (
	"runtime"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/pkg/errors"
)

// checkScopeSupported rejects user scope outside Linux: it relies on systemd user units.
func checkScopeSupported(opts []Options) error {
	if firstOptions(opts).scope() == gameap.ScopeUser {
		return errors.Errorf("user scope is not supported on %s", runtime.GOOS)
	}

	return nil
}
