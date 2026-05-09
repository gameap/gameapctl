package selfupdate

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/minio/selfupdate"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"golang.org/x/mod/semver"
)

func Handle(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	fmt.Println("Self update")

	fmt.Println("Checking new versions...")
	release, err := findRelease(ctx)
	if err != nil {
		var notFound releasefinder.FailedToFindReleaseError
		if errors.As(err, &notFound) && notFound.LatestTag != "" {
			return handleMissingAsset(cliCtx, notFound)
		}

		return errors.WithMessage(err, "failed to find release")
	}

	fmt.Println("Last version is", release.Tag)
	fmt.Println("You version is", gameap.Version)

	if isDevVersion(cliCtx) {
		printDevVersionMessage()

		return nil
	}

	if !isUpdateAvailable(ctx, release) {
		fmt.Println("No updates available")

		return nil
	}

	fmt.Println("Update available")
	fmt.Printf("Downloading from %s \n", release.URL)

	f, err := os.CreateTemp("", "gameapctl")
	if err != nil {
		return errors.WithMessage(err, "failed to create temp file")
	}
	defer func() {
		err := f.Close()
		if err != nil {
			fmt.Println("Failed to close temp file")

			return
		}
		err = os.Remove(f.Name())
		if err != nil {
			fmt.Println("Failed to remove temp file")
		}
	}()

	err = utils.DownloadFile(
		ctx,
		release.URL,
		f.Name(),
	)
	if err != nil {
		return errors.WithMessage(err, "failed to download")
	}

	_, err = f.Seek(0, 0)
	if err != nil {
		return errors.WithMessage(err, "failed to seek temp file")
	}

	fmt.Println("Applying...")
	err = selfupdate.Apply(f, selfupdate.Options{})
	if err != nil {
		return errors.WithMessage(err, "failed to update")
	}

	fmt.Println("Updated successfully")

	return nil
}

func isUpdateAvailable(_ context.Context, release *releasefinder.Release) bool {
	return semver.Compare(release.Tag, gameap.Version) == +1
}

func handleMissingAsset(cliCtx *cli.Context, notFound releasefinder.FailedToFindReleaseError) error {
	fmt.Println("Last version is", notFound.LatestTag)
	fmt.Println("You version is", gameap.Version)

	if isDevVersion(cliCtx) {
		printDevVersionMessage()

		return nil
	}
	if semver.Compare(notFound.LatestTag, gameap.Version) != +1 {
		fmt.Println("No updates available")

		return nil
	}

	return errors.Errorf(
		"update is available (%s), but no binary for %s/%s",
		notFound.LatestTag, notFound.OS, notFound.Arch,
	)
}

func isDevVersion(cliCtx *cli.Context) bool {
	return len(gameap.Version) >= 3 && gameap.Version[0:3] == "dev" && !cliCtx.Bool("force")
}

func printDevVersionMessage() {
	fmt.Println(
		"You use development version. " +
			"Update is not available. " +
			"Specify the --force flag to update you dev version to the latest version.",
	)
}

func findRelease(ctx context.Context) (*releasefinder.Release, error) {
	release, err := releasefinder.FindWithOptions(
		ctx,
		"https://api.github.com/repos/gameap/gameapctl/releases",
		runtime.GOOS,
		runtime.GOARCH,
		releasefinder.FindOptions{AllowPrerelease: true},
	)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to find release")
	}

	return release, nil
}
