package panel

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/gameap/gameapctl/pkg/gameap"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func SetupGameAPFromGithub(
	ctx context.Context,
	pm packagemanager.PackageManager,
	path string,
	branch string,
) error {
	var err error

	fmt.Println("Installing git ...")
	if err = pm.Install(ctx, packagemanager.GitPackage); err != nil {
		return errors.WithMessage(err, "failed to install git")
	}

	fmt.Println("Installing composer ...")
	if err = pm.Install(ctx, packagemanager.ComposerPackage); err != nil {
		return errors.WithMessage(err, "failed to install composer")
	}

	fmt.Println("Installing nodejs ...")
	if err = pm.Install(ctx, packagemanager.NodeJSPackage); err != nil {
		return errors.WithMessage(err, "failed to install nodejs")
	}

	fmt.Println("Cloning gameap ...")
	err = utils.ExecCommand(
		"git", "clone", "-b", branch, "https://github.com/et-nik/gameap.git", path,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to clone gameap from github")
	}

	fmt.Println("Installing composer dependencies ...")

	cmdName, args, err := packagemanager.DefinePHPComposerCommandAndArgs(
		"update", "--no-dev", "--optimize-autoloader", "--no-interaction", "--working-dir", path,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to define php composer command and args")
	}

	err = utils.ExecCommand(cmdName, args...)
	if err != nil {
		return errors.WithMessage(err, "failed to run composer update")
	}

	fmt.Println("Building styles ...")
	err = BuildStyles(ctx, path)
	if err != nil {
		return errors.WithMessage(err, "failed to build styles")
	}

	return nil
}

func SetupGameAPFromRepo(ctx context.Context, path string) error {
	tempDir, err := os.MkdirTemp("", "gameap")
	if err != nil {
		return errors.WithMessage(err, "failed to create temp dir")
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			log.Println(err)
		}
	}(tempDir)

	fmt.Println("Downloading GameAP ...")
	downloadPath, err := url.JoinPath(gameap.Repository(), "gameap/latest.tar.gz")
	if err != nil {
		return errors.WithMessage(err, "failed to join url")
	}

	err = utils.Download(ctx, downloadPath, tempDir)
	if err != nil {
		return errors.WithMessagef(err, "failed to download gameap from '%s'", downloadPath)
	}

	err = utils.Move(tempDir+string(os.PathSeparator)+"gameap", path)
	if err != nil {
		return errors.WithMessage(err, "failed to move gameap")
	}

	return nil
}
