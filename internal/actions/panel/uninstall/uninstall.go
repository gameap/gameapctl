package uninstall

import (
	"context"
	"fmt"
	"log"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	panelpkg "github.com/gameap/gameapctl/internal/pkg/panel"
	"github.com/gameap/gameapctl/pkg/daemon"
	"github.com/gameap/gameapctl/pkg/gameap"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/panel"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

//nolint:nestif
func Handle(cliCtx *cli.Context) error {
	ctx := cliCtx.Context
	withDaemon := cliCtx.Bool("with-daemon")
	withData := cliCtx.Bool("with-data")
	withServices := cliCtx.Bool("with-services")

	fmt.Println("Uninstalling GameAP...")

	fmt.Println("With Daemon:", withDaemon)
	fmt.Println("With Data:", withData)
	fmt.Println("With Services:", withServices)

	paths, err := panelpkg.ResolveScope(ctx, cliCtx.String("scope"))
	if err != nil {
		return err
	}

	pm, err := packagemanager.Load(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to load package manager")
	}

	if err := stopAndUninstallGameAP(ctx, paths, withData); err != nil {
		return errors.WithMessage(err, "failed to uninstall gameap")
	}

	if withDaemon {
		fmt.Println()
		fmt.Println("Uninstalling GameAP Daemon...")
		if err := stopAndUninstallDaemon(ctx, withData); err != nil {
			return errors.WithMessage(err, "failed to uninstall daemon")
		}
	}

	if withServices {
		if paths.Scope == gameap.ScopeUser {
			fmt.Println()
			fmt.Println("No system services were installed by gameapctl in user scope, nothing to remove")
		} else {
			fmt.Println()
			fmt.Println("Removing services...")
			if err := removeServices(ctx, pm); err != nil {
				return errors.WithMessage(err, "failed to remove services")
			}
		}
	}

	if withData {
		fmt.Println()
		fmt.Println("Removing data...")
		if err := removeData(ctx, paths); err != nil {
			return errors.WithMessage(err, "failed to remove data")
		}

		state, err := gameapctl.LoadPanelInstallState(ctx)
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to load panel install state"))
		} else if state.DatabaseWasInstalled {
			fmt.Println()
			fmt.Println("Removing database...")
			if err := removeDatabase(ctx, pm, state); err != nil {
				return errors.WithMessage(err, "failed to remove database")
			}
		}
	}

	fmt.Println()
	fmt.Println("GameAP has been successfully uninstalled!")

	return nil
}

func stopAndUninstallGameAP(ctx context.Context, paths gameap.PanelPaths, removeData bool) error {
	fmt.Println("Stopping GameAP service...")
	if err := panel.Stop(ctx, panel.Options{Scope: paths.Scope}); err != nil {
		if !errors.Is(err, panel.ErrGameAPNotInstalled) && !errors.Is(err, service.ErrInactiveService) {
			log.Println(errors.WithMessage(err, "failed to stop gameap"))
		}
	}

	return uninstallGameAP(ctx, paths, removeData)
}

func stopAndUninstallDaemon(ctx context.Context, removeData bool) error {
	scope := gameap.ScopeSystem
	if state, err := gameapctl.LoadDaemonInstallState(ctx); err == nil && state.Scope != "" {
		scope = state.Scope
	}

	paths, err := gameap.DaemonPathsForScope(scope)
	if err != nil {
		return errors.WithMessage(err, "failed to resolve daemon paths")
	}

	fmt.Println("Stopping GameAP Daemon service...")
	if err := daemon.Stop(ctx, daemon.Options{Scope: paths.Scope}); err != nil {
		if !errors.Is(err, service.ErrInactiveService) {
			log.Println(errors.WithMessage(err, "failed to stop gameap-daemon"))
		}
	}

	return uninstallDaemon(ctx, paths, removeData)
}

//nolint:unparam
func removeServices(ctx context.Context, pm packagemanager.PackageManager) error {
	services := []string{packagemanager.PHPPackage, packagemanager.PHPExtensionsPackage, packagemanager.NginxPackage}

	for _, svc := range services {
		fmt.Printf("Removing service: %s\n", svc)
		if err := pm.Remove(ctx, svc); err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove %s", svc))
		}
	}

	return nil
}

func removeDatabase(ctx context.Context, pm packagemanager.PackageManager, state gameapctl.PanelInstallState) error {
	return removePlatformDatabase(ctx, pm, state)
}
