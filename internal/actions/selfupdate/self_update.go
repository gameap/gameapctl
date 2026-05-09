package selfupdate

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"

	gameapctlpkg "github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/gameap"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/minio/selfupdate"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"golang.org/x/mod/semver"
)

//nolint:funlen
func Handle(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	fmt.Println("Self update")

	fromGithub := cliCtx.Bool("github")
	branch := cliCtx.String("branch")
	rawVersion := cliCtx.String("version")

	if rawVersion != "" && (fromGithub || branch != "") {
		return errors.New("--version is mutually exclusive with --github and --branch")
	}
	if branch != "" && !fromGithub {
		return errors.New("--branch requires --github")
	}
	if fromGithub {
		if branch == "" {
			branch = "main"
		}

		return handleFromGithub(ctx, branch)
	}

	var tag, tagPrefix string
	if rawVersion != "" {
		norm, err := releasefinder.NormalizeTag(rawVersion)
		if err != nil {
			return err
		}
		tag, tagPrefix = norm.Full, norm.Prefix
	}

	fmt.Println("Checking new versions...")
	release, err := findRelease(ctx, releasefinder.FindOptions{
		Tag:             tag,
		TagPrefix:       tagPrefix,
		AllowPrerelease: true,
	})
	if err != nil {
		var notFound releasefinder.FailedToFindReleaseError
		if errors.As(err, &notFound) && notFound.LatestTag != "" && rawVersion == "" {
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

	if rawVersion == "" && !isUpdateAvailable(ctx, release) {
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

func findRelease(ctx context.Context, opts releasefinder.FindOptions) (*releasefinder.Release, error) {
	release, err := releasefinder.FindWithOptions(
		ctx,
		"https://api.github.com/repos/gameap/gameapctl/releases",
		runtime.GOOS,
		runtime.GOARCH,
		opts,
	)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to find release")
	}

	return release, nil
}

func handleFromGithub(ctx context.Context, branch string) error {
	log.Printf("Building gameapctl from GitHub (branch: %s)...\n", branch)

	pm, err := packagemanager.Load(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to load package manager")
	}

	binPath, tmpDir, err := gameapctlpkg.BuildGameapctlFromGithub(ctx, pm, branch)
	if err != nil {
		return errors.WithMessage(err, "failed to build gameapctl from github")
	}
	defer func() {
		if rmErr := os.RemoveAll(tmpDir); rmErr != nil {
			log.Printf("Failed to remove temp dir %s: %v\n", tmpDir, rmErr)
		}
	}()

	f, err := os.Open(binPath)
	if err != nil {
		return errors.WithMessage(err, "failed to open built binary")
	}
	defer func() {
		if cErr := f.Close(); cErr != nil {
			log.Printf("Failed to close built binary: %v\n", cErr)
		}
	}()

	fmt.Println("Applying...")
	if err := selfupdate.Apply(f, selfupdate.Options{}); err != nil {
		return errors.WithMessage(err, "failed to apply update")
	}

	fmt.Println("Updated successfully from GitHub")

	return nil
}
