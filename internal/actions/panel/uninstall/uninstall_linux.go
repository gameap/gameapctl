//go:build linux

package uninstall

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	systemdServicePath       = "/etc/systemd/system/gameap.service"
	daemonSystemdServicePath = "/etc/systemd/system/gameap-daemon.service"
)

func uninstallGameAP(ctx context.Context, removeData bool) error {
	fmt.Println("Disabling GameAP systemd service...")
	if err := oscore.ExecCommand(ctx, "systemctl", "disable", "gameap"); err != nil {
		log.Println(errors.WithMessage(err, "failed to disable gameap service"))
	}

	if utils.IsFileExists(systemdServicePath) {
		fmt.Println("Removing GameAP systemd service file...")
		if err := os.Remove(systemdServicePath); err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove %s", systemdServicePath))
		}
	}

	fmt.Println("Reloading systemd daemon...")
	if err := oscore.ExecCommand(ctx, "systemctl", "daemon-reload"); err != nil {
		log.Println(errors.WithMessage(err, "failed to reload systemd daemon"))
	}

	if removeData {
		fmt.Println("Removing GameAP binary...")
		binaryPath := gameap.DefaultBinaryPath
		if utils.IsFileExists(binaryPath) {
			if err := os.Remove(binaryPath); err != nil {
				log.Println(errors.WithMessagef(err, "failed to remove %s", binaryPath))
			}
		}
	}

	return nil
}

//nolint:nestif
func uninstallDaemon(ctx context.Context, removeData bool) error {
	_, err := exec.LookPath("gameap-daemon")
	if err != nil {
		return errors.WithMessage(err, "failed to find gameap-daemon binary in PATH")
	}

	fmt.Println("Disabling GameAP Daemon systemd service...")
	if err := oscore.ExecCommand(ctx, "systemctl", "disable", "gameap-daemon"); err != nil {
		log.Println(errors.WithMessage(err, "failed to disable gameap-daemon service"))
	}

	if utils.IsFileExists(daemonSystemdServicePath) {
		fmt.Println("Removing GameAP Daemon systemd service file...")
		if err := os.Remove(daemonSystemdServicePath); err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove %s", daemonSystemdServicePath))
		}
	}

	fmt.Println("Reloading systemd daemon...")
	if err := oscore.ExecCommand(ctx, "systemctl", "daemon-reload"); err != nil {
		log.Println(errors.WithMessage(err, "failed to reload systemd daemon"))
	}

	if removeData {
		fmt.Println("Removing GameAP Daemon binary...")
		daemonPath := gameap.DefaultDaemonFilePath
		if utils.IsFileExists(daemonPath) {
			if err := os.Remove(daemonPath); err != nil {
				log.Println(errors.WithMessagef(err, "failed to remove %s", daemonPath))
			}
		}

		fmt.Println("Removing GameAP Daemon configuration...")
		daemonConfigPath := gameap.DefaultDaemonConfigFilePath
		if utils.IsFileExists(daemonConfigPath) {
			if err := os.Remove(daemonConfigPath); err != nil {
				log.Println(errors.WithMessagef(err, "failed to remove %s", daemonConfigPath))
			}
		}

		fmt.Println("Removing GameAP Daemon certificates...")
		daemonCertPath := gameap.DefaultDaemonCertPath
		if utils.IsFileExists(daemonCertPath) {
			if err := os.RemoveAll(daemonCertPath); err != nil {
				log.Println(errors.WithMessagef(err, "failed to remove %s", daemonCertPath))
			}
		}
	}

	return nil
}

func removeData(_ context.Context) error {
	state, err := gameapctl.LoadPanelInstallState(context.Background())
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to load panel install state"))
	}

	configDir := gameap.DefaultConfigFilePath
	if state.ConfigDirectory != "" {
		configDir = state.ConfigDirectory
	}

	dataDir := gameap.DefaultDataPath
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
