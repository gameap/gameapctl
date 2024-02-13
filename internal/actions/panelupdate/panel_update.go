package panelupdate

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/internal/pkg/panel"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

//nolint:funlen,gocyclo
func Handle(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	fmt.Println("GameAP update")

	pm, err := packagemanager.Load(cliCtx.Context)
	if err != nil {
		return errors.WithMessage(err, "failed to load package manager")
	}

	state, err := gameapctl.LoadPanelInstallState(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to load panel install state")
	}

	tmpDir, err := os.MkdirTemp("", "gameapctl-update-panel")
	if err != nil {
		return errors.WithMessage(err, "failed to create temp file")
	}
	defer func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove temporary directory"))
		}
	}()

	tmpPanelDir := filepath.Join(tmpDir, "gameap")

	if state.FromGithub {
		fmt.Println("Setup GameAP from github ...")
		err = panel.SetupGameAPFromGithub(ctx, pm, tmpPanelDir, state.Branch)
	} else {
		fmt.Println("Setup GameAP ...")
		err = panel.SetupGameAPFromRepo(cliCtx.Context, tmpPanelDir)
	}
	if err != nil {
		return errors.WithMessage(err, "failed to download gameap")
	}

	backupDir, err := os.MkdirTemp("", "gameapctl-update-panel-backup")
	if err != nil {
		return errors.WithMessage(err, "failed to create temp file")
	}

	fmt.Println("Backup GameAP ...")
	err = utils.Move(state.Path, backupDir)
	if err != nil {
		return errors.WithMessage(err, "failed to backup")
	}

	fmt.Println("Upgrading GameAP ...")
	err = utils.Move(tmpPanelDir, state.Path)
	if err != nil {
		fmt.Println("Failed to upgrade GameAP: ", err)
		fmt.Println("Restoring backup ...")

		backupErr := restoreBackup(ctx, backupDir, state.Path)
		if backupErr != nil {
			fmt.Println("Failed to restore backup: ", backupErr)
			log.Println(errors.WithMessagef(err, "failed to restore backup directory"))
		}

		return errors.WithMessage(err, "failed to upgrade")
	}

	err = utils.Copy(filepath.Join(backupDir, ".env"), filepath.Join(state.Path, ".env"))
	if err != nil {
		backupErr := restoreBackup(ctx, backupDir, state.Path)
		if backupErr != nil {
			fmt.Println("Failed to restore backup: ", backupErr)
			log.Println(errors.WithMessagef(err, "failed to restore backup directory"))
		}

		return errors.WithMessage(err, "failed to upgrade")
	}

	err = utils.Copy(filepath.Join(backupDir, ".env"), filepath.Join(state.Path, ".env"))
	if err != nil {
		backupErr := restoreBackup(ctx, backupDir, state.Path)
		if backupErr != nil {
			fmt.Println("Failed to restore backup: ", backupErr)
			log.Println(errors.WithMessagef(err, "failed to restore backup directory"))
		}

		return errors.WithMessage(err, "failed to upgrade")
	}

	err = utils.Copy(filepath.Join(backupDir, "storage", "app"), filepath.Join(state.Path, "storage", "app"))
	if err != nil {
		backupErr := restoreBackup(ctx, backupDir, state.Path)
		if backupErr != nil {
			fmt.Println("Failed to restore backup: ", backupErr)
			log.Println(errors.WithMessagef(err, "failed to restore backup directory"))
		}

		return errors.WithMessage(err, "failed to upgrade")
	}

	defer func() {
		err := os.RemoveAll(backupDir)
		if err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove backup directory"))
		}
	}()

	fmt.Println("Upgrading games ...")
	err = panel.UpgradeGames(cliCtx.Context, state.Path)
	if err != nil {
		// Don't return error here
		log.Println("Failed to upgrade games: ", err)
	}

	return nil
}

func restoreBackup(_ context.Context, backupDir, path string) error {
	if utils.IsFileExists(path) {
		fmt.Println("Removing GameAP ...")
		err := os.RemoveAll(path)
		if err != nil {
			return errors.WithMessage(err, "failed to remove current gameap dir before backup restore")
		}
	}

	fmt.Println("Restoring backup ...")
	err := utils.Move(backupDir, path)
	if err != nil {
		fmt.Println()
		fmt.Println("Backup directory: ", backupDir)
		fmt.Println()

		return errors.WithMessage(err, "failed to restore backup")
	}

	return nil
}
