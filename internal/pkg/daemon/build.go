package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const daemonBuildOutputDirMode = 0755

func SetupDaemonFromGithub(
	ctx context.Context,
	pm packagemanager.PackageManager,
	branch string,
	outputPath string,
	userScope bool,
) error {
	if outputPath == "" {
		outputPath = gameap.DefaultDaemonFilePath
	}

	if err := ensureGithubBuildTools(ctx, pm, userScope); err != nil {
		return err
	}

	path, err := os.MkdirTemp("", "gameapctl-daemon")
	if err != nil {
		return errors.WithMessage(err, "failed to create temp dir")
	}
	defer func() {
		if removeErr := os.RemoveAll(path); removeErr != nil {
			log.Printf("Failed to remove temp dir %s: %v\n", path, removeErr)
		}
	}()

	fmt.Println("Cloning gameap-daemon ...")
	err = oscore.ExecCommand(
		ctx, "git", "clone", "-b", branch, gameap.GithubRepositoryDaemon, path,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to clone daemon from github")
	}

	fmt.Println("Building gameap-daemon ...")
	err = BuildGoDaemon(ctx, path, outputPath)
	if err != nil {
		return errors.WithMessage(err, "failed to build daemon")
	}

	return nil
}

func ensureGithubBuildTools(
	ctx context.Context,
	pm packagemanager.PackageManager,
	userScope bool,
) error {
	if userScope {
		if !utils.IsCommandAvailable("git") {
			return errors.New(
				"git is required to build gameap-daemon from GitHub in user scope; " +
					"please install it manually (e.g. via sudo) and retry",
			)
		}
		if !utils.IsCommandAvailable("go") {
			return errors.New(
				"go is required to build gameap-daemon from GitHub in user scope; " +
					"please install it manually (e.g. via sudo) and retry",
			)
		}

		return nil
	}

	fmt.Println("Installing git ...")
	if err := pm.Install(ctx, packagemanager.GitPackage); err != nil {
		return errors.WithMessage(err, "failed to install git")
	}

	fmt.Println("Installing golang ...")
	if err := pm.Install(ctx, packagemanager.GOPackage); err != nil {
		return errors.WithMessage(err, "failed to install golang")
	}
	packagemanager.UpdateEnvPath(ctx)

	return nil
}

func BuildGoDaemon(_ context.Context, repoPath, outputPath string) error {
	log.Println("Building go daemon...")

	if outputPath == "" {
		outputPath = gameap.DefaultDaemonFilePath
	}

	if dir := filepath.Dir(outputPath); dir != "" {
		if err := os.MkdirAll(dir, daemonBuildOutputDirMode); err != nil {
			return errors.Wrapf(err, "failed to create output directory %s", dir)
		}
	}

	cmdPath := filepath.Join(repoPath, "cmd", "gameap-daemon")

	cmd := exec.Command("go", "build", "-o", outputPath, ".")
	log.Println('\n', cmd.String())
	cmd.Dir = cmdPath
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()

	err := cmd.Run()
	if err != nil {
		return errors.WithMessage(err, "failed to build go daemon")
	}

	return nil
}
