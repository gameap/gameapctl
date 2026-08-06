package panel

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/runhelper"
	"github.com/gameap/gameapctl/pkg/systemd"
	"github.com/pkg/errors"
)

// Install installs GameAP v4.
func install(ctx context.Context, config InstallConfig) error {
	paths, err := gameap.PanelPathsForScope(config.scope())
	if err != nil {
		return errors.WithMessage(err, "failed to resolve panel paths")
	}

	// In user scope the init system is never probed: DetectInit reads /proc/1/exe,
	// which is not readable by an unprivileged user. systemd.InstallUnit reports a
	// missing user manager on its own.
	if paths.Scope != gameap.ScopeUser {
		init, err := runhelper.DetectInit(ctx)
		if err != nil {
			log.Println("Failed to detect init:", err)
		}

		if init != runhelper.InitSystemd {
			fmt.Println("Init system not detected or unsupported, skipping systemd unit creation")
			fmt.Println("GameAP v4 installation completed successfully")

			return nil
		}
	}

	fmt.Println("Creating systemd unit file ...")

	if err := createSystemdUnit(ctx, config, paths); err != nil {
		return errors.WithMessage(err, "failed to create systemd unit file")
	}

	fmt.Println("GameAP v4 installation completed successfully")

	return nil
}

func panelReadWritePaths(config InstallConfig) string {
	readWritePaths := fmt.Sprintf("%s %s", config.DataDirectory, config.FilesLocalBasePath)
	if config.LegacyPath != "" {
		readWritePaths = fmt.Sprintf("%s %s", readWritePaths, config.LegacyPath)
	}

	return readWritePaths
}

func createSystemdUnit(ctx context.Context, config InstallConfig, paths gameap.PanelPaths) error {
	unit, err := renderSystemdUnit(SystemdUnitConfig{
		Scope:            paths.Scope,
		User:             config.User,
		Group:            config.Group,
		WorkingDirectory: config.DataDirectory,
		ExecStart:        config.BinaryPath,
		EnvironmentFile:  filepath.Join(config.ConfigDirectory, "config.env"),
		ReadWritePaths:   panelReadWritePaths(config),
	})
	if err != nil {
		return err
	}

	return systemd.InstallUnit(ctx, paths.Scope, paths.SystemdUnitPath, unit)
}
