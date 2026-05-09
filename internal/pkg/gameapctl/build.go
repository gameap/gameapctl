package gameapctl

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/pkg/errors"
)

// BuildGameapctlFromGithub clones the gameapctl repository at the given branch,
// builds the embedded UI bundle and the Go binary, and returns the path to the
// freshly built executable along with the temporary clone directory. The caller
// is responsible for removing tmpDir.
func BuildGameapctlFromGithub(
	ctx context.Context,
	pm packagemanager.PackageManager,
	branch string,
) (binaryPath, tmpDir string, err error) {
	if err := installBuildDependencies(ctx, pm); err != nil {
		return "", "", err
	}

	repoDir, err := os.MkdirTemp("", "gameapctl-src")
	if err != nil {
		return "", "", errors.WithMessage(err, "failed to create temp dir")
	}

	cleanup := func() { _ = os.RemoveAll(repoDir) }

	fmt.Println("Cloning gameapctl ...")
	if err := oscore.ExecCommand(
		ctx, "git", "clone", "-b", branch, gameap.GithubRepositoryGameapctl, repoDir,
	); err != nil {
		cleanup()

		return "", "", errors.WithMessage(err, "failed to clone gameapctl from github")
	}

	if err := buildUI(repoDir); err != nil {
		cleanup()

		return "", "", errors.WithMessage(err, "failed to build ui")
	}

	if runtime.GOOS == "windows" {
		if err := generateWindowsResources(ctx, repoDir); err != nil {
			cleanup()

			return "", "", errors.WithMessage(err, "failed to generate windows resources")
		}
	}

	binPath := filepath.Join(repoDir, "gameapctl-build")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	fmt.Println("Building gameapctl ...")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = filepath.Join(repoDir, "cmd", "gameapctl")
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	log.Println('\n', cmd.String())

	if err := cmd.Run(); err != nil {
		cleanup()

		return "", "", errors.WithMessage(err, "failed to build gameapctl")
	}

	return binPath, repoDir, nil
}

func installBuildDependencies(ctx context.Context, pm packagemanager.PackageManager) error {
	fmt.Println("Installing git ...")
	if err := pm.Install(ctx, packagemanager.GitPackage); err != nil {
		return errors.WithMessage(err, "failed to install git")
	}

	fmt.Println("Installing nodejs ...")
	if err := pm.Install(ctx, packagemanager.NodeJSPackage); err != nil {
		return errors.WithMessage(err, "failed to install nodejs")
	}

	fmt.Println("Installing golang ...")
	if err := pm.Install(ctx, packagemanager.GOPackage); err != nil {
		return errors.WithMessage(err, "failed to install golang")
	}
	packagemanager.UpdateEnvPath(ctx)

	return nil
}

func buildUI(repoDir string) error {
	uiPath := filepath.Join(repoDir, "ui")

	log.Println("Installing ui dependencies ...")
	cmd := exec.Command("npm", "ci")
	cmd.Dir = uiPath
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	log.Println('\n', cmd.String())
	if err := cmd.Run(); err != nil {
		return errors.WithMessage(err, "failed to install ui dependencies")
	}

	log.Println("Building ui bundle ...")
	cmd = exec.Command("npm", "run", "build", "--if-present")
	cmd.Dir = uiPath
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	log.Println('\n', cmd.String())
	if err := cmd.Run(); err != nil {
		return errors.WithMessage(err, "failed to build ui bundle")
	}

	return nil
}

func generateWindowsResources(ctx context.Context, repoDir string) error {
	log.Println("Generating Windows resources ...")
	cmd := exec.CommandContext(ctx, "go", "generate", "./cmd/gameapctl/...")
	cmd.Dir = repoDir
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	log.Println('\n', cmd.String())

	if err := cmd.Run(); err != nil {
		return errors.WithMessage(err, "failed to run go generate")
	}

	return nil
}
