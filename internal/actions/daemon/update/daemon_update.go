package update

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	daemonpkg "github.com/gameap/gameapctl/internal/pkg/daemon"
	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/daemon"
	"github.com/gameap/gameapctl/pkg/gameap"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/minio/selfupdate"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

//nolint:funlen,gocognit
func Handle(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	fmt.Println("Daemon update")

	fromGithub := cliCtx.Bool("github")
	branch := cliCtx.String("branch")

	daemonState, stateErr := gameapctl.LoadDaemonInstallState(ctx)
	if stateErr == nil {
		if !fromGithub && daemonState.FromGithub {
			fromGithub = true
		}
		if branch == "" && daemonState.Branch != "" {
			branch = daemonState.Branch
		}
	}

	if branch == "" {
		branch = "master"
	}

	if fromGithub {
		return handleFromGithub(ctx, branch)
	}

	gameapDaemonPath, err := exec.LookPath("gameap-daemon")
	if err != nil {
		fmt.Println("Daemon not found")

		return errors.WithMessage(err, "failed to find gameap-daemon")
	}

	fmt.Println("Checking new versions...")
	release, err := findRelease(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to find release")
	}

	fmt.Println("Last version is", release.Tag)

	tmpDir, err := os.MkdirTemp("", "gameap-daemon")
	if err != nil {
		return errors.WithMessage(err, "failed to create temp file")
	}
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			log.Printf("Failed to remove temp dir %s: %v\n", tmpDir, removeErr)
		}
	}()

	err = utils.Download(
		ctx,
		release.URL,
		tmpDir,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to download")
	}

	filename := filepath.Join(tmpDir, "gameap-daemon")
	if runtime.GOOS == "windows" {
		filename += ".exe"
	}

	f, err := os.Open(filename)
	if err != nil {
		return errors.WithMessage(err, "failed to open downloaded file")
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			log.Printf("Failed to close temp file: %v\n", closeErr)
		}
	}()

	fmt.Println("Stopping daemon...")
	err = stopDaemon(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to stop daemon")
	}

	backupPath := filepath.Join(os.TempDir(), "gameap-daemon-backup")
	err = utils.Copy(gameapDaemonPath, backupPath)
	if err != nil {
		return errors.WithMessage(err, "failed to make backup file")
	}
	defer func() {
		err := os.Remove(backupPath)
		if err != nil {
			fmt.Println("Failed to remove backup file")
		}
	}()

	fmt.Println("Updating...")
	err = selfupdate.Apply(f, selfupdate.Options{
		TargetPath: gameapDaemonPath,
	})
	if err != nil {
		fmt.Println("Update failed, reverting...")
		if revertErr := revert(ctx, gameapDaemonPath, backupPath); revertErr != nil {
			return errors.WithMessage(revertErr, "failed to revert after update failure")
		}

		return errors.WithMessage(err, "failed to update")
	}

	fmt.Println("Starting daemon...")
	err = startDaemon(ctx)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Failed to start daemon. Reverting...")
		err = revert(ctx, gameapDaemonPath, backupPath)
		if err != nil {
			return errors.WithMessage(err, "failed to revert")
		}

		fmt.Println("Starting daemon...")
		err = startDaemon(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to start daemon")
		}
	}

	fmt.Println("Updated successfully")

	return nil
}

func stopDaemon(ctx context.Context) error {
	err := daemon.Stop(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to stop daemon")
	}

	fmt.Println("Checking process status...")
	daemonProcess, err := daemon.FindProcess(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to find daemon process")
	}
	if daemonProcess != nil {
		return errors.New("daemon process already running")
	}

	return nil
}

func startDaemon(ctx context.Context) error {
	err := daemon.Start(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to start daemon")
	}

	log.Println("Checking process status...")
	daemonProcess, err := daemon.FindProcess(ctx)
	if err != nil || daemonProcess == nil {
		return errors.WithMessage(err, "failed to find daemon process")
	}

	return nil
}

func revert(_ context.Context, path, backupPath string) error {
	f, err := os.Open(backupPath)
	if err != nil {
		return errors.WithMessage(err, "failed to open backup file")
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			fmt.Println("Failed to close temp file")
		}
	}(f)

	err = selfupdate.Apply(f, selfupdate.Options{
		TargetPath: path,
	})

	return errors.WithMessage(err, "failed to revert")
}

func findRelease(ctx context.Context) (*releasefinder.Release, error) {
	release, err := releasefinder.Find(
		ctx,
		"https://api.github.com/repos/gameap/daemon/releases",
		runtime.GOOS,
		runtime.GOARCH,
	)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to find release")
	}

	return release, nil
}

func handleFromGithub(ctx context.Context, branch string) error {
	log.Printf("Upgrading daemon from GitHub (branch: %s)...\n", branch)

	gameapDaemonPath, err := exec.LookPath("gameap-daemon")
	if err != nil {
		gameapDaemonPath = gameap.DefaultDaemonFilePath
	}

	backupPath := filepath.Join(os.TempDir(), "gameap-daemon-backup")

	if _, statErr := os.Stat(gameapDaemonPath); statErr == nil {
		log.Println("Backing up current binary...")
		if err := utils.Copy(gameapDaemonPath, backupPath); err != nil {
			return errors.WithMessage(err, "failed to create backup")
		}
		defer func() {
			if removeErr := os.Remove(backupPath); removeErr != nil {
				log.Printf("Failed to remove backup file: %v\n", removeErr)
			}
		}()
	}

	fmt.Println("Stopping daemon...")
	if err := stopDaemon(ctx); err != nil {
		return errors.WithMessage(err, "failed to stop daemon")
	}

	pm, err := packagemanager.Load(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to load package manager")
	}

	fmt.Println("Building daemon from GitHub source...")
	if err := daemonpkg.SetupDaemonFromGithub(ctx, pm, branch); err != nil {
		log.Printf("Build failed: %v\n", err)

		if _, statErr := os.Stat(backupPath); statErr == nil {
			fmt.Println("Reverting...")
			if revertErr := revert(ctx, gameapDaemonPath, backupPath); revertErr != nil {
				log.Printf("Failed to revert: %v\n", revertErr)
			}
		}

		if startErr := startDaemon(ctx); startErr != nil {
			log.Printf("Failed to start daemon after build failure: %v\n", startErr)
		}

		return errors.WithMessage(err, "failed to build daemon from github")
	}

	fmt.Println("Starting daemon...")
	if err := startDaemon(ctx); err != nil {
		fmt.Println("Failed to start daemon. Reverting...")

		if revertErr := revertFromBackup(ctx, gameapDaemonPath, backupPath); revertErr != nil {
			return errors.WithMessage(revertErr, "failed to revert and restart after build failure")
		}

		return errors.WithMessage(err, "daemon failed to start with new build")
	}

	fmt.Println("Updated successfully from GitHub")

	return nil
}

func revertFromBackup(ctx context.Context, binaryPath, backupPath string) error {
	if _, err := os.Stat(backupPath); err != nil {
		return errors.WithMessage(err, "backup file not found")
	}

	if err := revert(ctx, binaryPath, backupPath); err != nil {
		return errors.WithMessage(err, "failed to revert")
	}

	fmt.Println("Starting daemon with old version...")

	return startDaemon(ctx)
}
