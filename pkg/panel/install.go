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
	"text/template"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	randomKeyLength     = 32
	randomAuthKeyLength = 32
)

// InstallConfig represents the configuration for GameAP v4 installation.
type InstallConfig struct {
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
}

// ConfigEnvData represents the data for config.env template.
type ConfigEnvData struct {
	HTTPHost           string
	HTTPPort           string
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

// Install installs GameAP v4.
//
//nolint:gocognit,funlen
func Install(ctx context.Context, config InstallConfig) error {
	// Set defaults if not provided
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
		switch config.DatabaseDriver {
		case "sqlite", "sqlite3":
			config.CacheDriver = "inmemory"
		case "mysql":
			config.CacheDriver = "mysql"
		default:
			config.CacheDriver = "inmemory"
		}
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

	// Generate encryption key and auth secret if not provided
	if config.EncryptionKey == "" {
		key, err := generateRandomKey(randomKeyLength)
		if err != nil {
			return errors.WithMessage(err, "failed to generate encryption key")
		}
		config.EncryptionKey = key
	}
	if config.AuthSecret == "" {
		secret, err := generateRandomKey(randomAuthKeyLength)
		if err != nil {
			return errors.WithMessage(err, "failed to generate auth secret")
		}
		config.AuthSecret = secret
	}

	// Create user and group
	//nolint:nestif
	if runtime.GOOS != "windows" {
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
	}

	// Create directories
	fmt.Println("Creating directories ...")
	if err := createDirectories(ctx, config); err != nil {
		return errors.WithMessage(err, "failed to create directories")
	}

	// Create config.env file
	fmt.Println("Creating configuration file ...")
	if err := createConfigEnv(ctx, config); err != nil {
		return errors.WithMessage(err, "failed to create config.env")
	}

	// Download binaries
	if err := downloadBinaries(ctx, config); err != nil {
		return errors.WithMessage(err, "failed to download binaries")
	}

	// Perform platform-specific installation steps
	return install(ctx, config)
}

// createConfigEnv creates the config.env file.
func createConfigEnv(ctx context.Context, config InstallConfig) error {
	tmpl, err := template.New("config.env").Parse(configEnvTemplate)
	if err != nil {
		return errors.WithMessage(err, "failed to parse config.env template")
	}

	data := ConfigEnvData{
		HTTPHost:           config.HTTPHost,
		HTTPPort:           config.HTTPPort,
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
		return errors.WithMessage(err, "failed to execute config.env template")
	}

	configPath := filepath.Join(config.ConfigDirectory, "config.env")
	if err := os.WriteFile(configPath, buf.Bytes(), 0600); err != nil {
		return errors.WithMessage(err, "failed to write config.env file")
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
	if n != randomKeyLength {
		return "", errors.New("failed to generate required number of random bytes")
	}

	encoded := base64.RawURLEncoding.EncodeToString(random)

	return fmt.Sprintf("base64:%s", encoded), nil
}

func downloadBinaries(ctx context.Context, _ InstallConfig) error {
	tmpDir, err := os.MkdirTemp("", "gameap")
	if err != nil {
		return errors.WithMessage(err, "failed to make temp dir")
	}

	release, err := releasefinder.Find(
		ctx,
		"https://api.github.com/repos/gameap/gameap/releases",
		runtime.GOOS,
		runtime.GOARCH,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to find release")
	}

	fmt.Println("Downloading binaries ...")
	fmt.Println("Downloading from Git repository...")
	fmt.Println("Release Tag:", release.Tag)

	fmt.Println("Downloading from URL:", release.URL)

	err = utils.Download(
		ctx,
		release.URL,
		tmpDir,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to download gameap binaries")
	}

	var binariesInstalled bool

	for _, p := range []string{"gameap", "gameap.exe"} {
		fp := filepath.Join(tmpDir, p)
		if _, err = os.Stat(fp); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return errors.WithMessage(err, "failed to stat file")
		}

		err = utils.Move(fp, gameap.DefaultBinaryPath)
		if err != nil {
			return errors.WithMessage(err, "failed to move gameap binaries")
		}

		binariesInstalled = true

		break
	}

	if !binariesInstalled {
		return errors.New("gameap binaries wasn't installed, invalid archive contents")
	}

	return nil
}
