//go:build linux || darwin

package https

import (
	"context"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/pkg/errors"
)

// applyCertPermissions hands the certificate directory to the account the panel
// runs as. A rootless installation runs as the invoking user and owns it already.
func applyCertPermissions(ctx context.Context, paths gameap.PanelPaths, dir string) error {
	if paths.Scope != gameap.ScopeSystem || paths.User == "" || paths.Group == "" {
		return nil
	}

	if err := oscore.ChownRecursive(ctx, dir, paths.User, paths.Group); err != nil {
		return errors.WithMessage(err, "failed to set the certificate directory ownership")
	}

	return nil
}
