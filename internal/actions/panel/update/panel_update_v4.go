package update

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	installpkg "github.com/gameap/gameapctl/internal/pkg/panel"
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/panel"
	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

const (
	backupSuffix       = ".backup"
	healthCheckRetries = 5
	healthCheckDelay   = 2 * time.Second
)

func handleV4(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	log.Println("Downloading latest GameAP release...")
	tmpDir, downloadedBinary, err := downloadLatestRelease(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to download latest release")
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("Failed to remove temporary directory: %v\n", err)
		}
	}()

	log.Println("Stopping GameAP...")
	if err := panel.Stop(ctx); err != nil {
		return errors.WithMessage(err, "failed to stop GameAP")
	}

	log.Println("Backing up and replacing binary...")
	backupPath := gameap.DefaultBinaryPath + backupSuffix
	if err := backupAndReplace(downloadedBinary, gameap.DefaultBinaryPath, backupPath); err != nil {
		return errors.WithMessage(err, "failed to backup and replace binary")
	}

	log.Println("Starting GameAP...")
	if err := panel.Start(ctx); err != nil {
		return errors.WithMessage(err, "failed to start GameAP")
	}

	log.Println("Checking if new version is working...")
	httpHost, httpPort, httpsEnabled, err := readConfigEnv()
	if err != nil {
		log.Printf("Warning: failed to read config.env: %v\n", err)

		httpHost = "127.0.0.1"
		httpPort = "8025"
		httpsEnabled = false
	}

	if err := checkHealth(ctx, httpHost, httpPort, httpsEnabled); err != nil {
		log.Printf("Health check failed: %v\n", err)
		log.Println("Rolling back to previous version...")

		if stopErr := panel.Stop(ctx); stopErr != nil {
			log.Printf("Failed to stop GameAP during rollback: %v\n", stopErr)
		}

		if err := restoreBackupV4(backupPath, gameap.DefaultBinaryPath); err != nil {
			return errors.WithMessage(err, "failed to restore backup")
		}

		if startErr := panel.Start(ctx); startErr != nil {
			return errors.WithMessage(startErr, "failed to start GameAP after rollback")
		}

		return errors.New("update failed, rolled back to previous version")
	}

	log.Println("Update successful! Removing backup...")
	if err := os.Remove(backupPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Printf("Warning: failed to remove backup file: %v\n", err)
	}

	fmt.Println("GameAP has been successfully updated!")

	return nil
}

// downloadLatestRelease downloads the latest GameAP release to a temporary directory
// and returns the temporary directory path and the path to the downloaded binary.
func downloadLatestRelease(ctx context.Context) (string, string, error) {
	tmpDir, err := os.MkdirTemp("", "gameap-update-*")
	if err != nil {
		return "", "", errors.WithMessage(err, "failed to create temporary directory")
	}

	release, err := releasefinder.Find(
		ctx,
		"https://api.github.com/repos/gameap/gameap/releases",
		runtime.GOOS,
		runtime.GOARCH,
	)
	if err != nil {
		return tmpDir, "", errors.WithMessage(err, "failed to find release")
	}

	log.Printf("Found release: %s\n", release.Tag)
	log.Printf("Downloading from: %s\n", release.URL)

	if err := utils.Download(ctx, release.URL, tmpDir); err != nil {
		return tmpDir, "", errors.WithMessage(err, "failed to download release")
	}

	binaryNames := []string{"gameap", "gameap.exe"}
	for _, name := range binaryNames {
		binaryPath := filepath.Join(tmpDir, name)
		if _, err := os.Stat(binaryPath); err == nil {
			return tmpDir, binaryPath, nil
		}
	}

	return tmpDir, "", errors.New("downloaded binary not found in archive")
}

// backupAndReplace creates a backup of the current binary and replaces it with the new one.
func backupAndReplace(newBinary, currentBinary, backupPath string) error {
	if err := utils.Copy(currentBinary, backupPath); err != nil {
		return errors.WithMessage(err, "failed to create backup")
	}

	if err := os.Remove(currentBinary); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errors.WithMessage(err, "failed to remove old binary")
	}

	if err := utils.Copy(newBinary, currentBinary); err != nil {
		if restoreErr := restoreBackupV4(backupPath, currentBinary); restoreErr != nil {
			log.Printf("Failed to restore backup after copy failure: %v\n", restoreErr)
		}

		return errors.WithMessage(err, "failed to copy new binary")
	}

	if err := os.Chmod(currentBinary, 0755); err != nil {
		return errors.WithMessage(err, "failed to set executable permissions")
	}

	return nil
}

// restoreBackupV4 restores the backup to the current binary path.
func restoreBackupV4(backupPath, currentBinary string) error {
	if _, err := os.Stat(backupPath); err != nil {
		return errors.WithMessage(err, "backup file not found")
	}

	if err := os.Remove(currentBinary); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errors.WithMessage(err, "failed to remove current binary")
	}

	if err := utils.Copy(backupPath, currentBinary); err != nil {
		return errors.WithMessage(err, "failed to restore backup")
	}

	// Ensure the restored binary is executable
	if err := os.Chmod(currentBinary, 0755); err != nil {
		return errors.WithMessage(err, "failed to set executable permissions on restored binary")
	}

	return nil
}

// readConfigEnv reads HTTP_HOST and HTTP_PORT from config.env.
func readConfigEnv() (host, port string, httpsEnabled bool, err error) {
	configPath := gameap.DefaultConfigFilePath

	file, err := os.Open(configPath)
	if err != nil {
		return "", "", false, errors.WithMessage(err, "failed to open config file")
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("Failed to close config file: %v\n", err)
		}
	}(file)

	scanner := bufio.NewScanner(file)
	host = "127.0.0.1"
	port = "8025"
	httpsEnabled = false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE format
		//nolint:nestif
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch key {
				case "HTTP_HOST":
					if value != "" {
						host = value
					}
				case "HTTP_PORT":
					if value != "" {
						port = value
					}
				case "HTTPS_ENABLED":
					httpsEnabled = value == "true" || value == "1"
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", false, errors.WithMessage(err, "failed to read config file")
	}

	return host, port, httpsEnabled, nil
}

// checkHealth performs health checks on the GameAP instance.
func checkHealth(ctx context.Context, host, port string, httpsEnabled bool) error {
	for i := 0; i < healthCheckRetries; i++ {
		if i > 0 {
			log.Printf("Retry %d/%d...\n", i+1, healthCheckRetries)
			time.Sleep(healthCheckDelay)
		}

		if err := installpkg.CheckInstallationV4(ctx, host, port, httpsEnabled); err == nil {
			log.Println("Health check passed!")

			return nil
		} else {
			log.Printf("Health check attempt %d failed: %v\n", i+1, err)
		}
	}

	return errors.New("health check failed after multiple retries")
}
