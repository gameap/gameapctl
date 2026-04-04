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
	"github.com/pkg/errors"
)

func SetupDaemonFromGithub(
	ctx context.Context,
	pm packagemanager.PackageManager,
	branch string,
) error {
	fmt.Println("Installing git ...")
	if err := pm.Install(ctx, packagemanager.GitPackage); err != nil {
		return errors.WithMessage(err, "failed to install git")
	}

	fmt.Println("Installing golang ...")
	if err := pm.Install(ctx, packagemanager.GOPackage); err != nil {
		return errors.WithMessage(err, "failed to install golang")
	}
	packagemanager.UpdateEnvPath(ctx)

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
	err = BuildGoDaemon(ctx, path)
	if err != nil {
		return errors.WithMessage(err, "failed to build daemon")
	}

	return nil
}

func BuildGoDaemon(_ context.Context, repoPath string) error {
	log.Println("Building go daemon...")

	cmdPath := filepath.Join(repoPath, "cmd", "gameap-daemon")

	//nolint:gosec
	cmd := exec.Command("go", "build", "-o", gameap.DefaultDaemonFilePath, ".")
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
