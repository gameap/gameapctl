package panel

import (
	"context"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	windowUsername = "NT AUTHORITY\\NETWORK SERVICE"
)

func install(ctx context.Context, cfg InstallConfig) error {
	pm, err := packagemanager.Load(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to load package manager")
	}

	if utils.IsFileExists(gameap.DefaultWorkPath) {
		if err = oscore.GrantReadExecute(
			ctx,
			gameap.DefaultWorkPath,
			windowUsername,
		); err != nil {
			return errors.WithMessage(err, "failed to grant read execute permissions for working directory")
		}
	}

	if err = oscore.GrantReadExecute(
		ctx,
		cfg.ConfigDirectory,
		windowUsername,
	); err != nil {
		return errors.WithMessage(err, "failed to set permissions for config directory")
	}

	if err = oscore.GrantFullControl(
		ctx,
		cfg.DataDirectory,
		windowUsername,
	); err != nil {
		return errors.WithMessage(err, "failed to set permissions for config directory")
	}

	if err = oscore.GrantReadExecute(
		ctx,
		cfg.BinaryPath,
		windowUsername,
	); err != nil {
		return errors.WithMessage(err, "failed to set permissions for config directory")
	}

	err = pm.Install(ctx, packagemanager.GameAP)
	if err != nil {
		return errors.WithMessage(err, "failed to install gameap")
	}

	return nil
}
