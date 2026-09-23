//go:build windows

package https

import (
	"context"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/pkg/errors"
)

// applyCertPermissions grants the service account read access to the certificate
// directory, the same account the installer grants the config directory to.
//
// C:\gameap is protected first: in an installation made by an older gameapctl every local
// user can modify the certificate and its key.
func applyCertPermissions(ctx context.Context, _ gameap.PanelPaths, dir string) error {
	if err := packagemanager.ProtectWorkPath(ctx); err != nil {
		return errors.WithMessage(err, "failed to protect the GameAP directory")
	}

	if err := oscore.GrantReadExecute(ctx, dir, oscore.WindowsNetworkServiceAccount); err != nil {
		return errors.WithMessage(err, "failed to grant read access to the certificate directory")
	}

	return nil
}
