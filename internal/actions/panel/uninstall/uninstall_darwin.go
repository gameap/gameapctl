//go:build darwin

package uninstall

import (
	"context"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/gameap"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
)

func uninstallGameAP(_ context.Context, _ gameap.PanelPaths, _ bool) error {
	return nil
}

func uninstallDaemon(_ context.Context, _ gameap.DaemonPaths, _ bool) error {
	return nil
}

func removeData(_ context.Context, _ gameap.PanelPaths) error {
	return nil
}

func removePlatformDatabase(
	_ context.Context,
	_ packagemanager.PackageManager,
	_ gameapctl.PanelInstallState,
) error {
	return nil
}
