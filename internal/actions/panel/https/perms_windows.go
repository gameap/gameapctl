//go:build windows

package https

import (
	"context"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/pkg/errors"
)

// applyCertPermissions grants the service account read access to the certificate
// directory, the same account the installer grants the config directory to.
func applyCertPermissions(ctx context.Context, _ gameap.PanelPaths, dir string) error {
	if err := oscore.GrantReadExecute(ctx, dir, oscore.WindowsNetworkServiceAccount); err != nil {
		return errors.WithMessage(err, "failed to grant read access to the certificate directory")
	}

	return nil
}
