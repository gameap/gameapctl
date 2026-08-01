package panel

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/gameap/gameapctl/pkg/releasesource"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	randomKeyLength     = 32
	randomAuthKeyLength = 32
)

// InstallConfig represents the configuration for GameAP v4 installation.
type InstallConfig struct {
	// Scope selects between a system-wide installation and a rootless one under $HOME.
	Scope string

	// Config paths
	ConfigDirectory string
	DataDirectory   string
	BinaryPath      string

	// User and group
	User  string
	Group string

	// HTTP configuration
	HTTPHost string
	HTTPPort string

	// Database configuration
	DatabaseDriver string
	DatabaseURL    string

	// Security
	EncryptionKey string
	AuthSecret    string
	AuthService   string

	// Cache
	CacheDriver string

	// File storage
	FilesDriver        string
	FilesLocalBasePath string

	// Legacy path (optional)
	LegacyPath string

	// Global API
	GlobalAPIURL string

	// gRPC bidirectional protocol (panel >= v4.2). When enabled, GRPC_ENABLED=true
	// and GRPC_PORT are written to config.env. GRPCPort defaults to gameap.DefaultGRPCPort.
	GRPCEnabled bool
	GRPCPort    string

	// Tag pins the installation to a specific GitHub release tag (e.g. "v4.1.2").
	// If empty, TagPrefix takes effect (or the latest stable release is used).
	Tag string
	// TagPrefix selects the latest stable release whose tag_name starts with this
	// prefix (e.g. "v4.1." matches v4.1.0, v4.1.5).
	TagPrefix string

	// PreResolvedRelease, when non-nil, is used directly by Install instead of
	// querying the release source again. Lets the caller resolve the release once
	// and decide version-dependent settings (like GRPCEnabled) before installation.
	PreResolvedRelease *releasesource.Release
}

// ConfigEnvData represents the data for config.env template.
type ConfigEnvData struct {
	HTTPHost           string
	HTTPPort           string
	GRPCEnabled        bool
	GRPCPort           string
	DatabaseDriver     string
	DatabaseURL        string
	EncryptionKey      string
	AuthSecret         string
	AuthService        string
	CacheDriver        string
	FilesDriver        string
	FilesLocalBasePath string
	LegacyPath         string
	GlobalAPIURL       string
}

// Configure sets up GameAP v4 configuration: creates user/group, directories, config.env,
// and platform-specific setup (e.g. systemd unit). Does not download binaries.
func Configure(ctx context.Context, config InstallConfig) error {
	config, err := applyScopeDefaults(config)
	if err != nil {
		return err
	}

	config = applyConfigDefaults(config)
	config = preserveExistingSecrets(config)

	if config.EncryptionKey == "" {
		config.EncryptionKey, err = generateRandomKey(randomKeyLength)
		if err != nil {
			return errors.WithMessage(err, "failed to generate encryption key")
		}
	}
	if config.AuthSecret == "" {
		config.AuthSecret, err = generateRandomKey(randomAuthKeyLength)
		if err != nil {
			return errors.WithMessage(err, "failed to generate auth secret")
		}
	}

	//nolint:nestif
	if runtime.GOOS != "windows" && config.scope() != gameap.ScopeUser {
		fmt.Println("Creating GameAP user and group ...")

		if err := oscore.CreateGroup(ctx, config.Group); err != nil {
			var existsErr *oscore.GroupAlreadyExistsError
			if !errors.As(err, &existsErr) {
				return errors.WithMessage(err, "failed to create group")
			}

			fmt.Println("Group already exists")
		}

		if err := oscore.CreateUser(ctx, config.User, oscore.WithWorkDir(config.DataDirectory)); err != nil {
			var existsErr *oscore.UserAlreadyExistsError
			if !errors.As(err, &existsErr) {
				return errors.WithMessage(err, "failed to create user")
			}

			fmt.Println("User already exists")
		}
	} else if config.scope() == gameap.ScopeUser {
		fmt.Println("Skipping GameAP user and group creation in user scope ...")
	}

	fmt.Println("Creating directories ...")
	if err := createDirectories(ctx, config); err != nil {
		return errors.WithMessage(err, "failed to create directories")
	}

	fmt.Println("Creating configuration file ...")
	if err := createConfigEnv(ctx, config); err != nil {
		return errors.WithMessage(err, "failed to create config.env")
	}

	return install(ctx, config)
}

// Install installs GameAP v4: configures and downloads binaries.
// Returns the resolved release tag (e.g. "v4.1.2") that was actually downloaded.
func Install(ctx context.Context, config InstallConfig) (string, error) {
	config, err := applyScopeDefaults(config)
	if err != nil {
		return "", err
	}

	if err := Configure(ctx, config); err != nil {
		return "", err
	}

	resolvedTag, err := downloadBinaries(ctx, config)
	if err != nil {
		return "", errors.WithMessage(err, "failed to download binaries")
	}

	return resolvedTag, nil
}

func (c InstallConfig) scope() string {
	return gameap.ScopeOrDefault(c.Scope)
}

// applyScopeDefaults fills paths and ownership from the installation scope. In user
// scope User and Group stay empty: there is no dedicated account and nothing to chown to.
func applyScopeDefaults(config InstallConfig) (InstallConfig, error) {
	if config.scope() == gameap.ScopeUser {
		paths, err := gameap.UserPanelPaths()
		if err != nil {
			return config, errors.WithMessage(err, "failed to resolve user panel paths")
		}

		if config.ConfigDirectory == "" {
			config.ConfigDirectory = paths.ConfigDir
		}
		if config.DataDirectory == "" {
			config.DataDirectory = paths.DataDir
		}
		if config.BinaryPath == "" {
			config.BinaryPath = paths.BinaryPath
		}

		return config, nil
	}

	if config.ConfigDirectory == "" {
		config.ConfigDirectory = defaultConfigDir
	}
	if config.DataDirectory == "" {
		config.DataDirectory = defaultDataDir
	}
	if config.BinaryPath == "" {
		config.BinaryPath = defaultBinaryPath
	}
	if config.User == "" {
		config.User = defaultUser
	}
	if config.Group == "" {
		config.Group = defaultGroup
	}

	return config, nil
}

func applyConfigDefaults(config InstallConfig) InstallConfig {
	if config.HTTPHost == "" {
		config.HTTPHost = "0.0.0.0"
	}
	if config.HTTPPort == "" {
		config.HTTPPort = "8025"
	}
	if config.AuthService == "" {
		config.AuthService = "paseto"
	}
	if config.CacheDriver == "" {
		config.CacheDriver = "inmemory"
	}
	if config.FilesDriver == "" {
		config.FilesDriver = "local"
	}
	if config.FilesLocalBasePath == "" {
		config.FilesLocalBasePath = filepath.Join(config.DataDirectory, "files")
	}
	if config.GlobalAPIURL == "" {
		config.GlobalAPIURL = "https://api.gameap.com"
	}
	if config.GRPCEnabled && config.GRPCPort == "" {
		config.GRPCPort = gameap.DefaultGRPCPort
	}

	return config
}

// renderConfigEnv renders the config.env template into bytes without touching disk.
func renderConfigEnv(config InstallConfig) ([]byte, error) {
	tmpl, err := template.New("config.env").Parse(configEnvTemplate)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to parse config.env template")
	}

	data := ConfigEnvData{
		HTTPHost:           config.HTTPHost,
		HTTPPort:           config.HTTPPort,
		GRPCEnabled:        config.GRPCEnabled,
		GRPCPort:           config.GRPCPort,
		DatabaseDriver:     config.DatabaseDriver,
		DatabaseURL:        config.DatabaseURL,
		EncryptionKey:      config.EncryptionKey,
		AuthSecret:         config.AuthSecret,
		AuthService:        config.AuthService,
		CacheDriver:        config.CacheDriver,
		FilesDriver:        config.FilesDriver,
		FilesLocalBasePath: config.FilesLocalBasePath,
		LegacyPath:         config.LegacyPath,
		GlobalAPIURL:       config.GlobalAPIURL,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, errors.WithMessage(err, "failed to execute config.env template")
	}

	return buf.Bytes(), nil
}

// createConfigEnv creates the config.env file.
func createConfigEnv(ctx context.Context, config InstallConfig) error {
	rendered, err := renderConfigEnv(config)
	if err != nil {
		return err
	}

	configPath := filepath.Join(config.ConfigDirectory, "config.env")
	if err := os.WriteFile(configPath, rendered, 0600); err != nil {
		return errors.WithMessage(err, "failed to write config.env file")
	}

	if config.scope() == gameap.ScopeUser {
		return nil
	}

	if err := oscore.ChownRecursive(ctx, configPath, config.User, config.Group); err != nil {
		return errors.WithMessage(err, "failed to set ownership for config.env")
	}

	return nil
}

// createDirectories creates all necessary directories for GameAP v4.
func createDirectories(ctx context.Context, config InstallConfig) error {
	directories := []string{
		config.ConfigDirectory,
		config.DataDirectory,
		config.FilesLocalBasePath,
		filepath.Join(config.FilesLocalBasePath, "certs"),
		filepath.Join(config.FilesLocalBasePath, "certs", "client"),
		filepath.Join(config.FilesLocalBasePath, "certs", "server"),
	}

	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.WithMessagef(err, "failed to create directory %s", dir)
		}
	}

	if config.scope() == gameap.ScopeUser {
		return nil
	}

	// Set ownership
	for _, dir := range directories {
		if err := oscore.ChownRecursive(ctx, dir, config.User, config.Group); err != nil {
			return errors.WithMessagef(err, "failed to set ownership for %s", dir)
		}
	}

	return nil
}

func generateRandomKey(length int) (string, error) {
	random := make([]byte, length)
	n, err := rand.Read(random)
	if err != nil {
		return "", errors.WithMessage(err, "failed to generate random bytes")
	}
	if n != length {
		return "", errors.New("failed to generate required number of random bytes")
	}

	encoded := base64.StdEncoding.EncodeToString(random)

	return fmt.Sprintf("base64:%s", encoded), nil
}

// ResolveRelease queries the best available release source for the GameAP
// release matching the given install config (Tag/TagPrefix). Lets callers know
// the resolved tag before running Install — useful for version-dependent
// decisions (e.g. enabling gRPC).
func ResolveRelease(ctx context.Context, config InstallConfig) (*releasesource.Release, error) {
	opts := releasefinder.FindOptions{
		Tag:       config.Tag,
		TagPrefix: config.TagPrefix,
	}
	if config.Tag != "" {
		norm, normErr := releasefinder.NormalizeTag(config.Tag)
		if normErr == nil && norm.HasPrereleaseSuffix() {
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
		return nil, errors.WithMessage(err, "failed to find release")
	}

	return release, nil
}

func downloadBinaries(ctx context.Context, config InstallConfig) (string, error) {
	tmpDir, err := os.MkdirTemp("", "gameap")
	if err != nil {
		return "", errors.WithMessage(err, "failed to make temp dir")
	}

	release := config.PreResolvedRelease
	if release == nil {
		release, err = ResolveRelease(ctx, config)
		if err != nil {
			return "", err
		}
	}

	fmt.Println("Downloading binaries ...")
	fmt.Println("Release Tag:", release.Tag)

	fmt.Println("Downloading from URL:", release.PrimaryURL())

	err = releasesource.Download(
		ctx,
		release,
		tmpDir,
	)
	if err != nil {
		return "", errors.WithMessage(err, "failed to download gameap binaries")
	}

	var binariesInstalled bool

	for _, p := range []string{"gameap", "gameap.exe"} {
		fp := filepath.Join(tmpDir, p)
		if _, err = os.Stat(fp); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return "", errors.WithMessage(err, "failed to stat file")
		}

		err = utils.Move(fp, config.BinaryPath)
		if err != nil {
			return "", errors.WithMessage(err, "failed to move gameap binaries")
		}

		if err = os.Chmod(config.BinaryPath, 0755); err != nil {
			return "", errors.Wrap(err, "failed to set executable permissions")
		}

		binariesInstalled = true

		break
	}

	if !binariesInstalled {
		return "", errors.New("gameap binaries wasn't installed, invalid archive contents")
	}

	return release.Tag, nil
}

// preserveExistingSecrets reads ENCRYPTION_KEY and AUTH_SECRET from an existing
// config.env to avoid breaking already encrypted data in the database on re-installation.
func preserveExistingSecrets(config InstallConfig) InstallConfig {
	if config.EncryptionKey != "" && config.AuthSecret != "" {
		return config
	}

	configPath := filepath.Join(config.ConfigDirectory, "config.env")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return config
	}

	existing := parseConfigEnvValues(data)
	if config.EncryptionKey == "" {
		config.EncryptionKey = existing["ENCRYPTION_KEY"]
	}
	if config.AuthSecret == "" {
		config.AuthSecret = existing["AUTH_SECRET"]
	}

	return config
}

func parseConfigEnvValues(data []byte) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = parts[1]
		}
	}

	return result
}
