package update

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	installpkg "github.com/gameap/gameapctl/internal/pkg/panel"
	"github.com/gameap/gameapctl/pkg/configenv"
	"github.com/gameap/gameapctl/pkg/gameap"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/panel"
	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/gameap/gameapctl/pkg/releasesource"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

const (
	backupSuffix       = ".backup"
	healthCheckRetries = 5
	healthCheckDelay   = 2 * time.Second

	defaultHealthCheckHost = "127.0.0.1"
	defaultHealthCheckPort = "8025"
)

func handleV4(cliCtx *cli.Context, paths gameap.PanelPaths, tag, tagPrefix string) error {
	ctx := cliCtx.Context

	fromGithub := cliCtx.Bool("github")
	branch := cliCtx.String("branch")

	if (tag != "" || tagPrefix != "") && (fromGithub || branch != "") {
		return errors.New("--version is mutually exclusive with --github and --branch")
	}

	state, stateErr := gameapctl.LoadPanelInstallState(ctx)
	if stateErr == nil {
		if !fromGithub && state.FromGithub && tag == "" && tagPrefix == "" {
			fromGithub = true
		}
		if branch == "" && state.Branch != "" && tag == "" && tagPrefix == "" {
			branch = state.Branch
		}
	}

	if branch == "" {
		branch = "main"
	}

	if fromGithub {
		return handleV4FromGithub(ctx, paths, branch)
	}

	installedVersion := detectInstalledVersion(ctx, paths, state)
	if installedVersion != "" {
		log.Printf("Installed GameAP version: %s\n", installedVersion)
	}

	log.Println("Downloading GameAP release...")
	tmpDir, downloadedBinary, resolvedTag, err := downloadRelease(ctx, tag, tagPrefix)
	if err != nil {
		return errors.WithMessage(err, "failed to download release")
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("Failed to remove temporary directory: %v\n", err)
		}
	}()

	log.Println("Stopping GameAP...")
	if err := panel.Stop(ctx, panel.Options{Scope: paths.Scope}); err != nil {
		return errors.WithMessage(err, "failed to stop GameAP")
	}

	log.Println("Backing up and replacing binary...")
	backupPath := paths.BinaryPath + backupSuffix
	if err := backupAndReplace(downloadedBinary, paths.BinaryPath, backupPath); err != nil {
		return errors.WithMessage(err, "failed to backup and replace binary")
	}

	migration := reportConfigEnvMigration(
		panel.MigrateConfigEnv(paths.ConfigFilePath, installedVersion, resolvedTag),
	)

	if err := startAndVerifyV4(ctx, paths, backupPath, migration); err != nil {
		return err
	}

	log.Println("Update successful! Removing backup...")
	if err := os.Remove(backupPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Printf("Warning: failed to remove backup file: %v\n", err)
	}

	if resolvedTag != "" {
		updatePanelStateVersion(ctx, resolvedTag)
	}

	fmt.Println("GameAP has been successfully updated!")

	return nil
}

func updatePanelStateVersion(ctx context.Context, resolvedTag string) {
	state, err := gameapctl.LoadPanelInstallState(ctx)
	if err != nil {
		log.Printf("Warning: failed to load panel state to record version: %v\n", err)

		return
	}
	if state.Version == resolvedTag {
		return
	}
	state.Version = resolvedTag
	if err := gameapctl.SavePanelInstallState(ctx, state); err != nil {
		log.Printf("Warning: failed to save panel state with new version: %v\n", err)
	}
}

// downloadRelease downloads the GameAP release matching tag/prefix to a temporary
// directory and returns the temporary directory path, the path to the downloaded
// binary, and the resolved release tag.
func downloadRelease(ctx context.Context, tag, tagPrefix string) (string, string, string, error) {
	tmpDir, err := os.MkdirTemp("", "gameap-update-*")
	if err != nil {
		return "", "", "", errors.WithMessage(err, "failed to create temporary directory")
	}

	opts := releasefinder.FindOptions{
		Tag:       tag,
		TagPrefix: tagPrefix,
	}
	if tag != "" {
		if norm, normErr := releasefinder.NormalizeTag(tag); normErr == nil && norm.HasPrereleaseSuffix() {
			opts.AllowPrerelease = true
		}
	}

	release, err := releasesource.FindRelease(
		ctx,
		releasesource.ComponentPanel,
		runtime.GOOS,
		runtime.GOARCH,
		opts,
	)
	if err != nil {
		return tmpDir, "", "", errors.WithMessage(err, "failed to find release")
	}

	log.Printf("Found release: %s\n", release.Tag)
	log.Printf("Downloading from: %s\n", release.PrimaryURL())

	if err := releasesource.Download(ctx, release, tmpDir); err != nil {
		return tmpDir, "", "", errors.WithMessage(err, "failed to download release")
	}

	binaryNames := []string{"gameap", "gameap.exe"}
	for _, name := range binaryNames {
		binaryPath := filepath.Join(tmpDir, name)
		if _, err := os.Stat(binaryPath); err == nil {
			return tmpDir, binaryPath, release.Tag, nil
		}
	}

	return tmpDir, "", "", errors.New("downloaded binary not found in archive")
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

// readConfigEnv reads the address the panel answers on. HTTPS is derived from
// the certificate source rather than from a flag: the panel has no key that
// switches TLS on, it serves it as soon as a certificate is configured, and with
// TLS_FORCE_HTTPS the plain HTTP endpoint only answers with a redirect.
func readConfigEnv(configPath string) (host, port string, httpsEnabled bool, err error) {
	_, values, err := configenv.Read(configPath)
	if err != nil {
		return "", "", false, err
	}

	host = panel.ConfigValue(values, panel.HTTPHostKey)
	if host == "" {
		host = defaultHealthCheckHost
	}

	port = panel.ConfigValue(values, panel.HTTPPortKey)
	if port == "" {
		port = defaultHealthCheckPort
	}

	httpsEnabled = panel.TLSEnabled(values)
	if httpsEnabled {
		port = panel.HTTPSPort(values)
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

func handleV4FromGithub(ctx context.Context, paths gameap.PanelPaths, branch string) error {
	log.Printf("Upgrading GameAP from GitHub (branch: %s)...\n", branch)

	pm, err := packagemanager.Load(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to load package manager")
	}

	// Build into a temporary file while GameAP is still running to minimize downtime.
	tmpBuildDir, err := os.MkdirTemp("", "gameap-build-*")
	if err != nil {
		return errors.WithMessage(err, "failed to create temporary build directory")
	}
	defer func() {
		if removeErr := os.RemoveAll(tmpBuildDir); removeErr != nil {
			log.Printf("Failed to remove temporary build directory: %v\n", removeErr)
		}
	}()

	builtBinary := filepath.Join(tmpBuildDir, "gameap")
	if runtime.GOOS == "windows" {
		builtBinary += ".exe"
	}

	log.Println("Building GameAP from source...")
	if err := installpkg.SetupGameAPFromGithubV4(
		ctx, pm, branch, builtBinary, paths.Scope == gameap.ScopeUser,
	); err != nil {
		return errors.WithMessage(err, "failed to build GameAP from github")
	}

	log.Println("Stopping GameAP...")
	if err := panel.Stop(ctx, panel.Options{Scope: paths.Scope}); err != nil {
		return errors.WithMessage(err, "failed to stop GameAP")
	}

	log.Println("Backing up and replacing binary...")
	backupPath := paths.BinaryPath + backupSuffix
	if err := backupAndReplace(builtBinary, paths.BinaryPath, backupPath); err != nil {
		// The previous binary is still in place: either the backup could not be
		// created and the binary was never touched, or the failed replacement was
		// already rolled back from the backup. Restart it directly.
		log.Printf("Failed to replace binary: %v\n", err)
		log.Println("Restarting GameAP with the previous binary...")

		if startErr := panel.Start(ctx, panel.Options{Scope: paths.Scope}); startErr != nil {
			return errors.WithMessage(startErr, "failed to restart GameAP after replace failure")
		}

		return errors.WithMessage(err, "failed to backup and replace binary")
	}

	// A branch build is newer than every release, so every migration applies.
	migration := reportConfigEnvMigration(panel.MigrateConfigEnvToLatest(paths.ConfigFilePath))

	if err := startAndVerifyV4(ctx, paths, backupPath, migration); err != nil {
		return err
	}

	log.Println("Update successful! Removing backup...")
	if err := os.Remove(backupPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Printf("Warning: failed to remove backup file: %v\n", err)
	}

	fmt.Println("GameAP has been successfully updated from GitHub!")

	return nil
}

// detectInstalledVersion reports the panel release currently in place, which is
// what tells a config.env upgrade from a downgrade. The binary knows its own
// version from GameAP v4.5 on; before that the only record is the tag gameapctl
// wrote when it installed or last upgraded the panel. An empty result means
// unknown, and the migration falls back to its ordinary forward direction.
func detectInstalledVersion(
	ctx context.Context, paths gameap.PanelPaths, state gameapctl.PanelInstallState,
) string {
	if version := panel.InstalledVersion(ctx, paths.BinaryPath); version != "" {
		return version
	}

	if releasefinder.HasMajorMinor(state.Version) {
		return state.Version
	}

	return ""
}

// reportConfigEnvMigration logs what the migration changed. A migration that
// fails never fails the upgrade: the panel ignores env vars it does not know,
// and its own compatibility shim keeps the previous names working for a
// release, so a stale config.env is a warning rather than a broken install.
func reportConfigEnvMigration(migration panel.ConfigEnvMigration, err error) panel.ConfigEnvMigration {
	if err != nil {
		log.Printf("Warning: failed to migrate config.env: %v\n", err)

		return panel.ConfigEnvMigration{}
	}

	for _, change := range migration.Changes {
		log.Printf("config.env: %s\n", change)
	}

	return migration
}

// rollbackV4 puts the previous binary and its config.env back and starts the
// panel again.
//
// The config is restored before the binary: it is best-effort and log-only, so
// going first means it still happens when the binary restore fails and returns
// early. The service is stopped throughout, so the order has no other effect.
func rollbackV4(
	ctx context.Context, paths gameap.PanelPaths, backupPath string, migration panel.ConfigEnvMigration,
) error {
	if stopErr := panel.Stop(ctx, panel.Options{Scope: paths.Scope}); stopErr != nil {
		log.Printf("Failed to stop GameAP during rollback: %v\n", stopErr)
	}

	if restoreErr := migration.Restore(); restoreErr != nil {
		log.Printf("Failed to restore config.env during rollback: %v\n", restoreErr)
	}

	if err := restoreBackupV4(backupPath, paths.BinaryPath); err != nil {
		return errors.WithMessage(err, "failed to restore backup")
	}

	if err := panel.Start(ctx, panel.Options{Scope: paths.Scope}); err != nil {
		return errors.WithMessage(err, "failed to start GameAP after rollback")
	}

	return nil
}

// startAndVerifyV4 starts the upgraded panel and makes sure it answers. A panel
// that will not start or will not respond is rolled back together with the
// config.env the migration rewrote for it, so that the operator is left with a
// running installation rather than a matched but dead pair.
func startAndVerifyV4(
	ctx context.Context, paths gameap.PanelPaths, backupPath string, migration panel.ConfigEnvMigration,
) error {
	log.Println("Starting GameAP...")
	if err := panel.Start(ctx, panel.Options{Scope: paths.Scope}); err != nil {
		log.Printf("Failed to start GameAP: %v\n", err)
		log.Println("Rolling back to previous version...")

		if rollbackErr := rollbackV4(ctx, paths, backupPath, migration); rollbackErr != nil {
			return rollbackErr
		}

		return errors.New("new binary failed to start, rolled back to previous version")
	}

	log.Println("Checking if new version is working...")

	httpHost, httpPort, httpsEnabled, err := readConfigEnv(paths.ConfigFilePath)
	if err != nil {
		log.Printf("Warning: failed to read config.env: %v\n", err)

		httpHost = "127.0.0.1"
		httpPort = "8025"
		httpsEnabled = false
	}

	if err := checkHealth(ctx, httpHost, httpPort, httpsEnabled); err != nil {
		log.Printf("Health check failed: %v\n", err)
		log.Println("Rolling back to previous version...")

		if rollbackErr := rollbackV4(ctx, paths, backupPath, migration); rollbackErr != nil {
			return rollbackErr
		}

		return errors.New("update failed, rolled back to previous version")
	}

	return nil
}
