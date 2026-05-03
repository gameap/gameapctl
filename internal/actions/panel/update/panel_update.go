package update

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

type version string

const (
	versionV3 version = "v3"
	versionV4 version = "v4"
)

var errV3UpgradeNotSupported = errors.New("upgrading to GameAP v3 is not supported")

func Handle(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	fmt.Println("GameAP update")

	rawVersion := cliCtx.String("version")
	if rawVersion == "" {
		if rawTo := cliCtx.String("to"); rawTo != "" {
			log.Println("Warning: --to is deprecated, use --version instead")
			rawVersion = rawTo
		}
	}

	norm, err := releasefinder.NormalizeTag(rawVersion)
	if err != nil {
		return err
	}
	if releasefinder.IsMajorV3(norm) {
		return errV3UpgradeNotSupported
	}

	currentMajorVersion, err := detectMajorVersion(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to detect installed GameAP version")
	}

	state, err := gameapctl.LoadPanelInstallState(ctx)
	if err == nil && state.Version == "" {
		state.Version = string(currentMajorVersion)
		saveStateErr := gameapctl.SavePanelInstallState(ctx, state)
		if saveStateErr != nil {
			log.Println("Warning: failed to update panel installation state with detected version:", saveStateErr)
		}
	}
	if err != nil {
		log.Println("Warning: failed to load panel installation state:", err)
	}

	fmt.Printf("Detected installed GameAP version: %s\n", currentMajorVersion)

	if currentMajorVersion == versionV3 {
		if norm.Full != "" || norm.Prefix != "" {
			log.Println(
				"Warning: --version is ignored during v3→v4 migration; latest stable v4 will be used",
			)
		}

		return handleV3toV4(cliCtx)
	}

	return handleV4(cliCtx, norm.Full, norm.Prefix)
}

// detectMajorVersion detects the major version of the installed panel.
// It checks multiple markers in the following priority order:
// 1. Installation state file (fastest, most reliable if exists)
// 2. v4-specific file system markers (config.env, gameap binary, data directory)
// 3. v3-specific file system markers (artisan, .env files)
// Returns the detected version or an error if version cannot be determined.
func detectMajorVersion(ctx context.Context) (version, error) {
	state, err := gameapctl.LoadPanelInstallState(ctx)
	if err == nil && state.Version != "" {
		switch {
		case isStateVersionV3(state.Version):
			return versionV3, nil
		case isStateVersionV4(state.Version):
			return versionV4, nil
		}
	}

	if err == nil && state.Path != "" {
		artisanPath := filepath.Join(state.Path, "artisan")
		if utils.IsFileExists(artisanPath) {
			return versionV3, nil
		}

		envPath := filepath.Join(state.Path, ".env")
		if utils.IsFileExists(envPath) {
			return versionV3, nil
		}
	}

	if utils.IsFileExists(gameap.DefaultConfigFilePath) ||
		utils.IsFileExists(gameap.DefaultBinaryPath) ||
		utils.IsFileExists(gameap.DefaultDataPath) {
		return versionV4, nil
	}

	return "", errors.New("unable to detect GameAP version: no installation markers found")
}

func isStateVersionV3(v string) bool {
	return v == "3" || v == string(versionV3) || strings.HasPrefix(v, "v3.") || strings.HasPrefix(v, "3.")
}

func isStateVersionV4(v string) bool {
	return v == "4" || v == string(versionV4) || strings.HasPrefix(v, "v4.") || strings.HasPrefix(v, "4.")
}
