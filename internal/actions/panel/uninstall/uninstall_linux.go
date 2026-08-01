//go:build linux

package uninstall

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/gameap"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/systemd"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func uninstallGameAP(ctx context.Context, paths gameap.PanelPaths, removeData bool) error {
	fmt.Println("Disabling GameAP systemd service...")
	if err := systemd.Run(ctx, paths.Scope, "disable", "gameap"); err != nil {
		log.Println(errors.WithMessage(err, "failed to disable gameap service"))
	}

	if utils.IsFileExists(paths.SystemdUnitPath) {
		fmt.Println("Removing GameAP systemd service file...")
		if err := os.Remove(paths.SystemdUnitPath); err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove %s", paths.SystemdUnitPath))
		}
	}

	fmt.Println("Reloading systemd daemon...")
	if err := systemd.Run(ctx, paths.Scope, "daemon-reload"); err != nil {
		log.Println(errors.WithMessage(err, "failed to reload systemd daemon"))
	}

	if removeData {
		fmt.Println("Removing GameAP binary...")
		if utils.IsFileExists(paths.BinaryPath) {
			if err := os.Remove(paths.BinaryPath); err != nil {
				log.Println(errors.WithMessagef(err, "failed to remove %s", paths.BinaryPath))
			}
		}
	}

	if paths.Scope == gameap.ScopeUser {
		fmt.Println("Lingering is left enabled: other user services may depend on it. " +
			"To disable it: loginctl disable-linger $USER")
	}

	return nil
}

//nolint:nestif
func uninstallDaemon(ctx context.Context, paths gameap.DaemonPaths, removeData bool) error {
	if !utils.IsFileExists(paths.DaemonFilePath) && !utils.IsCommandAvailable("gameap-daemon") {
		return errors.Errorf("gameap-daemon binary not found at %s", paths.DaemonFilePath)
	}

	fmt.Println("Disabling GameAP Daemon systemd service...")
	if err := systemd.Run(ctx, paths.Scope, "disable", "gameap-daemon"); err != nil {
		log.Println(errors.WithMessage(err, "failed to disable gameap-daemon service"))
	}

	if utils.IsFileExists(paths.SystemdUnitPath) {
		fmt.Println("Removing GameAP Daemon systemd service file...")
		if err := os.Remove(paths.SystemdUnitPath); err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove %s", paths.SystemdUnitPath))
		}
	}

	fmt.Println("Reloading systemd daemon...")
	if err := systemd.Run(ctx, paths.Scope, "daemon-reload"); err != nil {
		log.Println(errors.WithMessage(err, "failed to reload systemd daemon"))
	}

	if removeData {
		fmt.Println("Removing GameAP Daemon binary...")
		if utils.IsFileExists(paths.DaemonFilePath) {
			if err := os.Remove(paths.DaemonFilePath); err != nil {
				log.Println(errors.WithMessagef(err, "failed to remove %s", paths.DaemonFilePath))
			}
		}

		fmt.Println("Removing GameAP Daemon configuration...")
		if utils.IsFileExists(paths.DaemonConfigFilePath) {
			if err := os.Remove(paths.DaemonConfigFilePath); err != nil {
				log.Println(errors.WithMessagef(err, "failed to remove %s", paths.DaemonConfigFilePath))
			}
		}

		fmt.Println("Removing GameAP Daemon certificates...")
		if utils.IsFileExists(paths.CertsPath) {
			if err := os.RemoveAll(paths.CertsPath); err != nil {
				log.Println(errors.WithMessagef(err, "failed to remove %s", paths.CertsPath))
			}
		}
	}

	return nil
}

func removeData(ctx context.Context, paths gameap.PanelPaths) error {
	state, err := gameapctl.LoadPanelInstallState(ctx)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to load panel install state"))
	}

	configDir := paths.ConfigDir
	if state.ConfigDirectory != "" {
		configDir = state.ConfigDirectory
	}

	dataDir := paths.DataDir
	if state.DataDirectory != "" {
		dataDir = state.DataDirectory
	}

	if utils.IsFileExists(configDir) {
		fmt.Printf("Removing GameAP configuration directory: %s\n", configDir)
		if err := os.RemoveAll(configDir); err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove %s", configDir))
		}
	}

	if utils.IsFileExists(dataDir) {
		fmt.Printf("Removing GameAP data directory: %s\n", dataDir)
		if err := os.RemoveAll(dataDir); err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove %s", dataDir))
		}
	}

	return nil
}

func removePlatformDatabase(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state gameapctl.PanelInstallState,
) error {
	if !state.DatabaseWasInstalled {
		return nil
	}

	if state.Scope == gameap.ScopeUser {
		fmt.Println("No database server was installed by gameapctl in user scope, nothing to remove")

		return nil
	}

	if state.Database == "mysql" || state.Database == "mariadb" {
		err := pm.Remove(ctx, packagemanager.MySQLServerPackage)
		if err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove %s", packagemanager.MySQLServerPackage))
		}
	}

	if state.Database == "postgresql" {
		err := pm.Remove(ctx, packagemanager.PostgreSQLPackage)
		if err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove %s", packagemanager.PostgreSQLPackage))
		}
	}

	return nil
}
