//go:build darwin

package uninstall

import (
	"context"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
)

func uninstallGameAP(_ context.Context, _ bool) error {
	return nil
}

func uninstallDaemon(_ context.Context, _ bool) error {
	return nil
}

func removeData(_ context.Context) error {
	return nil
}

func removePlatformServices(_ context.Context, _ packagemanager.PackageManager) error {
	return nil
}

func removePlatformDatabase(
	_ context.Context,
	_ packagemanager.PackageManager,
	_ gameapctl.PanelInstallState,
) error {
	return nil
}
