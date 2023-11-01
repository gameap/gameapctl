package daemonupdate

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/gameap/gameapctl/pkg/daemon"
	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/minio/selfupdate"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

//nolint:funlen
func Handle(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	fmt.Println("Daemon update")

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

	err = utils.Download(
		ctx,
		release.URL,
		tmpDir,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to download")
	}
	f, err := os.Open(filepath.Join(tmpDir, "gameap-daemon"))
	if err != nil {
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			fmt.Println("Failed to close temp file")

			return
		}
		err = os.RemoveAll(tmpDir)
		if err != nil {
			fmt.Printf("Failed to remove temp dir %s", tmpDir)
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

	return err
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
