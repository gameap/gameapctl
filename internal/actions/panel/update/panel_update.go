package update

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

type version string

const (
	versionV3 version = "v3"
	versionV4 version = "v4"
)

func Handle(cliCtx *cli.Context) error {
	fmt.Println("GameAP update")

	v, err := detectMajorVersion(cliCtx.Context)
	if err != nil {
		return errors.WithMessage(err, "failed to detect installed GameAP version")
	}

	if v == versionV4 {
		return handleV4(cliCtx)
	}

	return handleV3(cliCtx)
}

// detectMajorVersion detects the major version of the installed panel.
// It checks multiple markers in the following priority order:
// 1. Installation state file (fastest, most reliable if exists)
// 2. v4-specific file system markers (config.env, gameap binary, data directory)
// 3. v3-specific file system markers (artisan, .env files)
// Returns the detected version or an error if version cannot be determined.
func detectMajorVersion(ctx context.Context) (version, error) {
	// Priority 1: Check installation state file first
	state, err := gameapctl.LoadPanelInstallState(ctx)
	if err == nil && state.Version != "" {
		// Version field is set in installation state
		if state.Version == "3" || state.Version == string(versionV3) {
			return versionV3, nil
		}
		if state.Version == "4" || state.Version == string(versionV4) {
			return versionV4, nil
		}
	}

	// Priority 2: Check v4-specific markers (using constants from pkg/gameap)
	// These paths are platform-specific via build tags
	if utils.IsFileExists(gameap.DefaultConfigFilePath) ||
		utils.IsFileExists(gameap.DefaultBinaryPath) ||
		utils.IsFileExists(gameap.DefaultDataPath) {
		return versionV4, nil
	}

	// Priority 3: Check v3-specific markers using installation state
	if err == nil && state.Path != "" {
		// Check if artisan file exists (Laravel indicator)
		artisanPath := filepath.Join(state.Path, "artisan")
		if utils.IsFileExists(artisanPath) {
			return versionV3, nil
		}

		// Check if .env file exists (Laravel config)
		envPath := filepath.Join(state.Path, ".env")
		if utils.IsFileExists(envPath) {
			return versionV3, nil
		}
	}

	return "", errors.New("unable to detect GameAP version: no installation markers found")
}
