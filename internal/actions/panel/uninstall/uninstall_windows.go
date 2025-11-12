//go:build windows

package uninstall

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	defaultServicesConfigPath = "C:\\gameap\\services"
)

func uninstallGameAP(ctx context.Context, removeData bool) error {
	serviceName := "GameAP"

	if service.IsExists(ctx, serviceName) {
		fmt.Printf("Deleting service: %s\n", serviceName)
		if err := oscore.ExecCommand(ctx, "sc", "delete", serviceName); err != nil {
			log.Println(errors.WithMessagef(err, "failed to delete service %s", serviceName))
		}
	}

	serviceConfigPath := defaultServicesConfigPath + "\\GameAP.yaml"
	if utils.IsFileExists(serviceConfigPath) {
		fmt.Printf("Removing service config: %s\n", serviceConfigPath)
		if err := os.Remove(serviceConfigPath); err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove %s", serviceConfigPath))
		}
	}

	if removeData {
		binaryPath := gameap.DefaultBinaryPath
		if utils.IsFileExists(binaryPath) {
			fmt.Printf("Removing GameAP binary: %s\n", binaryPath)
			if err := os.Remove(binaryPath); err != nil {
				log.Println(errors.WithMessagef(err, "failed to remove %s", binaryPath))
			}
		}
	}

	return nil
}

//nolint:nestif
func uninstallDaemon(ctx context.Context, removeData bool) error {
	if service.IsExists(ctx, daemonServiceName) {
		fmt.Printf("Deleting service: %s\n", daemonServiceName)
		if err := oscore.ExecCommand(ctx, "sc", "delete", daemonServiceName); err != nil {
			log.Println(errors.WithMessagef(err, "failed to delete service %s", daemonServiceName))
		}
	}

	serviceConfigPath := defaultServicesConfigPath + "\\GameAP Daemon.yaml"
	if utils.IsFileExists(serviceConfigPath) {
		fmt.Printf("Removing service config: %s\n", serviceConfigPath)
		if err := os.Remove(serviceConfigPath); err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove %s", serviceConfigPath))
		}
	}

	if removeData {
		daemonBinaryPath := gameap.DefaultDaemonFilePath
		if utils.IsFileExists(daemonBinaryPath) {
			fmt.Printf("Removing daemon binary: %s\n", daemonBinaryPath)
			if err := os.Remove(daemonBinaryPath); err != nil {
				log.Println(errors.WithMessagef(err, "failed to remove %s", daemonBinaryPath))
			}
		}

		daemonConfigPath := gameap.DefaultDaemonConfigFilePath
		if utils.IsFileExists(daemonConfigPath) {
			fmt.Printf("Removing daemon config: %s\n", daemonConfigPath)
			if err := os.Remove(daemonConfigPath); err != nil {
				log.Println(errors.WithMessagef(err, "failed to remove %s", daemonConfigPath))
			}
		}

		daemonCertPath := gameap.DefaultDaemonCertPath
		if utils.IsFileExists(daemonCertPath) {
			fmt.Printf("Removing daemon certificates: %s\n", daemonCertPath)
			if err := os.RemoveAll(daemonCertPath); err != nil {
				log.Println(errors.WithMessagef(err, "failed to remove %s", daemonCertPath))
			}
		}

		daemonDir := "C:\\gameap\\daemon"
		if utils.IsFileExists(daemonDir) {
			fmt.Printf("Removing daemon directory: %s\n", daemonDir)
			if err := os.RemoveAll(daemonDir); err != nil {
				log.Println(errors.WithMessagef(err, "failed to remove %s", daemonDir))
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

	dataDir := gameap.DefaultDataPath
	if state.DataDirectory != "" {
		dataDir = state.DataDirectory
	}

	if utils.IsFileExists(dataDir) {
		fmt.Printf("Removing GameAP data directory: %s\n", dataDir)
		if err := os.RemoveAll(dataDir); err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove %s", dataDir))
		}
	}

	webDir := "C:\\gameap\\web"
	if utils.IsFileExists(webDir) && webDir != dataDir {
		fmt.Printf("Removing GameAP web directory: %s\n", webDir)

		if err := os.RemoveAll(webDir); err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove %s", webDir))
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
		fmt.Println("Skipping database removal as it was not installed by GameAP")

		return nil
	}

	if state.Database == "mysql" || state.Database == "mariadb" {
		fmt.Println("Removing MySQL database package")

		err := pm.Remove(ctx, packagemanager.MySQLServerPackage)
		if err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove %s", packagemanager.MySQLServerPackage))
		}
	}

	if strings.HasPrefix(state.Database, "postgres") {
		fmt.Println("Removing PostgreSQL database package")

		err := pm.Remove(ctx, packagemanager.PostgreSQLPackage)
		if err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove %s", packagemanager.PostgreSQLPackage))
		}
	}

	return nil
}
