package panel

import (
	"context"

	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/pkg/errors"
)

func install(ctx context.Context, _ InstallConfig) error {
	pm, err := packagemanager.Load(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to load package manager")
	}

	err = pm.Install(ctx, packagemanager.GameAP)
	if err != nil {
		return errors.WithMessage(err, "failed to install gameap")
	}

	return nil
}
