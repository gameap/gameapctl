package update

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/panel"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

const (
	dbDriverMySQL             = "mysql"
	webServerNginx            = "nginx"
	webServerApache           = "apache"
	webServerApache2          = "apache2"
	nginxConfigPath           = "/etc/nginx/sites-available/gameap.conf"
	apacheConfigPath          = "/etc/apache2/sites-available/gameap.conf"
	encryptionKeySize         = 32
	healthCheckTimeout        = 5
	defaultHTTPHost           = "0.0.0.0"
	defaultHTTPPort           = "8025"
	defaultDirPermission      = 0o755
	webServerConfigPermission = 0o644
)

//nolint:funlen,gocognit
func handleV3toV4(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	log.Println("Starting GameAP v3 to v4 migration...")

	state, err := gameapctl.LoadPanelInstallState(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to load panel install state")
	}

	v3Path := state.Path
	if v3Path == "" {
		v3Path = gameap.DefaultWebInstallationPath
	}

	envPath := filepath.Join(v3Path, ".env")
	if !utils.IsFileExists(envPath) {
		return errors.Errorf(".env file not found at %s", envPath)
	}

	log.Println("Parsing v3 configuration...")
	v3Config, err := parseV3Env(envPath)
	if err != nil {
		return errors.WithMessage(err, "failed to parse v3 .env file")
	}

	log.Println("Creating backup of v3 installation...")
	backupDir, err := os.MkdirTemp("", "gameapctl-v3-backup-")
	if err != nil {
		return errors.WithMessage(err, "failed to create backup directory")
	}
	log.Printf("Backup directory: %s\n", backupDir)

	backupV3Dir := filepath.Join(backupDir, "gameap-v3")
	if err := utils.Copy(v3Path, backupV3Dir); err != nil {
		return errors.WithMessage(err, "failed to backup v3 installation")
	}

	var webServerBackup string
	if state.WebServer != "" {
		log.Printf("Backing up %s configuration...\n", state.WebServer)
		webServerBackup, err = backupWebServerConfig(state.WebServer)
		if err != nil {
			log.Printf("Warning: failed to backup web server config: %v\n", err)
		} else {
			log.Printf("Web server config backed up to: %s\n", webServerBackup)
		}
	}

	log.Println("Building v4 installation configuration...")
	installConfig, err := buildInstallConfig(v3Config, state)
	if err != nil {
		return errors.WithMessage(err, "failed to build v4 install config")
	}

	installConfig.LegacyPath = v3Path

	log.Println("Installing GameAP v4...")
	if err := panel.Install(ctx, installConfig); err != nil {
		log.Printf("Failed to install v4: %v\n", err)
		log.Println("Rolling back...")
		rollbackErr := rollbackV3toV4(ctx, backupV3Dir, v3Path, webServerBackup, state.WebServer)
		if rollbackErr != nil {
			fmt.Println("Failed to rollback v3: ", rollbackErr.Error())
			log.Printf("Failed to rollback v3: %v\n", rollbackErr)
		}

		return errors.WithMessage(err, "failed to install v4")
	}

	log.Println("Migrating storage files...")
	storageAppDir := filepath.Join(v3Path, "storage", "app")
	if utils.IsFileExists(storageAppDir) {
		v4FilesDir := filepath.Join(installConfig.DataDirectory, "files")
		if err := utils.Copy(storageAppDir, v4FilesDir); err != nil {
			log.Printf("Warning: failed to copy storage files: %v\n", err)
		} else {
			log.Println("Storage files migrated successfully")
		}
	}

	log.Println("Starting GameAP v4 service...")
	if err := panel.Start(ctx); err != nil {
		log.Printf("Failed to start v4: %v\n", err)
		log.Println("Rolling back...")

		rollbackErr := rollbackV3toV4(ctx, backupV3Dir, v3Path, webServerBackup, state.WebServer)
		if rollbackErr != nil {
			fmt.Println("Failed to rollback v3: ", rollbackErr.Error())
			log.Printf("Failed to rollback v3: %v\n", rollbackErr)
		}

		return errors.WithMessage(err, "failed to start v4")
	}

	time.Sleep(2 * time.Second)

	//nolint:nestif
	if state.WebServer != "" {
		log.Printf("Updating %s configuration to proxy to v4...\n", state.WebServer)

		if err := updateWebServerConfigForV4(state.WebServer, installConfig.HTTPPort); err != nil {
			log.Printf("Failed to update web server config: %v\n", err)
			log.Println("Rolling back...")

			stopErr := panel.Stop(ctx)
			if stopErr != nil {
				fmt.Println("Failed to run panel.Stop: ", stopErr.Error())
				log.Printf("Failed to run panel.Stop: %v\n", stopErr)
			}

			rollbackErr := rollbackV3toV4(ctx, backupV3Dir, v3Path, webServerBackup, state.WebServer)
			if rollbackErr != nil {
				fmt.Println("Failed to rollback v3: ", rollbackErr.Error())
				log.Printf("Failed to rollback v3: %v\n", rollbackErr)
			}

			return errors.WithMessage(err, "failed to update web server config")
		}

		log.Printf("Restarting %s...\n", state.WebServer)
		if err := restartWebServer(ctx, state.WebServer); err != nil {
			log.Printf("Warning: failed to restart web server: %v\n", err)
		}
	}

	log.Println("Running health check...")
	if err := checkHealthV4(ctx, installConfig.HTTPHost, installConfig.HTTPPort); err != nil {
		log.Printf("Health check failed: %v\n", err)
		log.Println("Rolling back...")

		stopErr := panel.Stop(ctx)
		if stopErr != nil {
			fmt.Println("Failed to run panel.Stop: ", stopErr.Error())
			log.Printf("Failed to run panel.Stop: %v\n", stopErr)
		}

		rollbackErr := rollbackV3toV4(ctx, backupV3Dir, v3Path, webServerBackup, state.WebServer)
		if rollbackErr != nil {
			fmt.Println("Failed to rollback v3: ", rollbackErr.Error())
			log.Printf("Failed to rollback v3: %v\n", rollbackErr)
		}

		return errors.New("health check failed, rolled back to v3")
	}

	log.Println("Health check passed!")

	log.Println("Moving v3 installation to backup location...")
	timestamp := time.Now().Format("20060102-150405")
	finalBackupPath := filepath.Join("/var/backups", fmt.Sprintf("gameap-v3-%s", timestamp))
	if err := os.MkdirAll(filepath.Dir(finalBackupPath), defaultDirPermission); err != nil {
		log.Printf("Warning: failed to create backup directory: %v\n", err)
	} else {
		if err := utils.Move(v3Path, finalBackupPath); err != nil {
			log.Printf("Warning: failed to move v3 to backup location: %v\n", err)
		} else {
			log.Printf("V3 installation backed up to: %s\n", finalBackupPath)
		}
	}

	newState := gameapctl.PanelInstallState{
		Version:              "4",
		Host:                 state.Host,
		HostIP:               state.HostIP,
		Port:                 installConfig.HTTPPort,
		ConfigDirectory:      installConfig.ConfigDirectory,
		DataDirectory:        installConfig.DataDirectory,
		Database:             state.Database,
		DatabaseWasInstalled: state.DatabaseWasInstalled,
		Develop:              state.Develop,
		FromGithub:           state.FromGithub,
		Branch:               state.Branch,
	}

	if err := gameapctl.SavePanelInstallState(ctx, newState); err != nil {
		log.Printf("Warning: failed to save new panel state: %v\n", err)
	}

	if err := os.RemoveAll(backupDir); err != nil {
		log.Printf("Warning: failed to remove temporary backup: %v\n", err)
	}

	log.Println("Migration completed successfully!")
	log.Printf("GameAP v4 is now running on %s:%s\n", installConfig.HTTPHost, installConfig.HTTPPort)

	return nil
}

func parseV3Env(envPath string) (map[string]string, error) {
	config := make(map[string]string)

	file, err := os.Open(envPath)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to open .env file")
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to close .env file"))
		}
	}(file)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				value = strings.Trim(value, "\"'")

				config[key] = value
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.WithMessage(err, "failed to scan .env file")
	}

	return config, nil
}

func buildDatabaseURL(v3Config map[string]string) (string, string, error) {
	dbConnection := v3Config["DB_CONNECTION"]
	if dbConnection == "" {
		dbConnection = dbDriverMySQL
	}

	switch dbConnection {
	case dbDriverMySQL:
		host := v3Config["DB_HOST"]
		if host == "" {
			host = "localhost"
		}
		port := v3Config["DB_PORT"]
		if port == "" {
			port = "3306"
		}
		database := v3Config["DB_DATABASE"]
		if database == "" {
			database = "gameap"
		}
		username := v3Config["DB_USERNAME"]
		password := v3Config["DB_PASSWORD"]

		return dbDriverMySQL, fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?parseTime=true",
			username, password, host, port, database,
		), nil

	case "pgsql", "postgres", "postgresql":
		host := v3Config["DB_HOST"]
		if host == "" {
			host = "localhost"
		}
		port := v3Config["DB_PORT"]
		if port == "" {
			port = "5432"
		}
		database := v3Config["DB_DATABASE"]
		if database == "" {
			database = "gameap"
		}
		username := v3Config["DB_USERNAME"]
		password := v3Config["DB_PASSWORD"]

		return "postgres", fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=disable",
			username, password, host, port, database,
		), nil

	case "sqlite":
		dbPath := v3Config["DB_DATABASE"]
		if dbPath == "" {
			return "", "", errors.New("sqlite database path not specified")
		}

		return "sqlite", dbPath, nil

	default:
		return "", "", errors.Errorf("unsupported database connection: %s", dbConnection)
	}
}

func buildInstallConfig(v3Config map[string]string, _ gameapctl.PanelInstallState) (panel.InstallConfig, error) {
	dbDriver, dbURL, err := buildDatabaseURL(v3Config)
	if err != nil {
		return panel.InstallConfig{}, errors.WithMessage(err, "failed to build database URL")
	}

	encryptionKey := v3Config["APP_KEY"]
	if encryptionKey == "" || !strings.HasPrefix(encryptionKey, "base64:") {
		log.Println("Generating new encryption key...")
		key := make([]byte, encryptionKeySize)
		if _, err := rand.Read(key); err != nil {
			return panel.InstallConfig{}, errors.WithMessage(err, "failed to generate encryption key")
		}
		encryptionKey = "base64:" + base64.StdEncoding.EncodeToString(key)
	}

	authSecret := make([]byte, encryptionKeySize)
	if _, err := rand.Read(authSecret); err != nil {
		return panel.InstallConfig{}, errors.WithMessage(err, "failed to generate auth secret")
	}
	authSecretB64 := "base64:" + base64.StdEncoding.EncodeToString(authSecret)

	httpHost := defaultHTTPHost
	httpPort := defaultHTTPPort

	configDir := filepath.Dir(gameap.DefaultConfigFilePath)
	dataDir := gameap.DefaultDataPath
	binaryPath := gameap.DefaultBinaryPath

	return panel.InstallConfig{
		ConfigDirectory:    configDir,
		DataDirectory:      dataDir,
		BinaryPath:         binaryPath,
		User:               "gameap",
		Group:              "gameap",
		HTTPHost:           httpHost,
		HTTPPort:           httpPort,
		DatabaseDriver:     dbDriver,
		DatabaseURL:        dbURL,
		EncryptionKey:      encryptionKey,
		AuthSecret:         authSecretB64,
		AuthService:        "paseto",
		CacheDriver:        dbDriver,
		FilesDriver:        "local",
		FilesLocalBasePath: filepath.Join(dataDir, "files"),
		GlobalAPIURL:       "https://api.gameap.com",
	}, nil
}

func backupWebServerConfig(webServer string) (string, error) {
	var configPath string

	switch webServer {
	case webServerNginx:
		configPath = nginxConfigPath
	case webServerApache, webServerApache2:
		configPath = apacheConfigPath
	default:
		return "", errors.Errorf("unsupported web server: %s", webServer)
	}

	if !utils.IsFileExists(configPath) {
		return "", errors.Errorf("web server config not found: %s", configPath)
	}

	backupPath := configPath + ".v3.backup"
	if err := utils.Copy(configPath, backupPath); err != nil {
		return "", errors.WithMessage(err, "failed to copy config file")
	}

	return backupPath, nil
}

func updateWebServerConfigForV4(webServer, targetPort string) error {
	switch webServer {
	case webServerNginx:
		return updateNginxConfigForV4(targetPort)
	case webServerApache, webServerApache2:
		return updateApacheConfigForV4(targetPort)
	default:
		return errors.Errorf("unsupported web server: %s", webServer)
	}
}

func updateNginxConfigForV4(targetPort string) error {
	proxyTarget := fmt.Sprintf("http://127.0.0.1:%s", targetPort)

	newConfig := `server {
    listen 80;
    listen [::]:80;

    server_name _;

    location / {
        proxy_pass ` + proxyTarget + `;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }
}
`

	if err := os.WriteFile(nginxConfigPath, []byte(newConfig), webServerConfigPermission); err != nil {
		return errors.WithMessage(err, "failed to write nginx config")
	}

	return nil
}

func updateApacheConfigForV4(targetPort string) error {
	proxyTarget := fmt.Sprintf("http://127.0.0.1:%s/", targetPort)

	newConfig := `<VirtualHost *:80>
    ServerAdmin webmaster@localhost
    DocumentRoot /var/www/html

    ProxyPreserveHost On
    ProxyPass / ` + proxyTarget + `
    ProxyPassReverse / ` + proxyTarget + `

    ErrorLog ${APACHE_LOG_DIR}/gameap_error.log
    CustomLog ${APACHE_LOG_DIR}/gameap_access.log combined
</VirtualHost>
`

	if err := os.WriteFile(apacheConfigPath, []byte(newConfig), webServerConfigPermission); err != nil {
		return errors.WithMessage(err, "failed to write apache config")
	}

	return nil
}

func restartWebServer(ctx context.Context, webServer string) error {
	var serviceName string

	switch webServer {
	case webServerNginx:
		serviceName = webServerNginx
	case webServerApache, webServerApache2:
		serviceName = webServerApache2
	default:
		return errors.Errorf("unsupported web server: %s", webServer)
	}

	err := service.Restart(ctx, serviceName)
	if err != nil {
		return errors.WithMessagef(err, "failed to restart web server service (%s)", serviceName)
	}

	return nil
}

func restoreWebServerConfig(backupPath, originalPath string) error {
	if backupPath == "" || !utils.IsFileExists(backupPath) {
		return nil
	}

	if err := utils.Copy(backupPath, originalPath); err != nil {
		return errors.WithMessage(err, "failed to restore web server config")
	}

	return nil
}

func rollbackV3toV4(
	ctx context.Context,
	backupV3Dir,
	v3Path,
	webServerBackup,
	webServer string,
) error {
	log.Println("Restoring v3 installation...")

	if utils.IsFileExists(v3Path) {
		if err := os.RemoveAll(v3Path); err != nil {
			log.Printf("Warning: failed to remove v4 installation: %v\n", err)
		}
	}

	if err := utils.Move(backupV3Dir, v3Path); err != nil {
		return errors.WithMessage(err, "failed to restore v3 backup")
	}

	if webServerBackup != "" && webServer != "" {
		restoreWebServerFromBackup(ctx, webServerBackup, webServer)
	}

	log.Println("Rollback completed")

	return nil
}

func restoreWebServerFromBackup(ctx context.Context, webServerBackup, webServer string) {
	log.Println("Restoring web server configuration...")

	var configPath string
	switch webServer {
	case webServerNginx:
		configPath = nginxConfigPath
	case webServerApache, webServerApache2:
		configPath = apacheConfigPath
	}

	if configPath == "" {
		return
	}

	if err := restoreWebServerConfig(webServerBackup, configPath); err != nil {
		log.Printf("Warning: failed to restore web server config: %v\n", err)

		return
	}

	log.Printf("Restarting %s...\n", webServer)
	if err := restartWebServer(ctx, webServer); err != nil {
		log.Printf("Warning: failed to restart web server: %v\n", err)
	}
}

func checkHealthV4(ctx context.Context, host, port string) error {
	for i := 0; i < healthCheckRetries; i++ {
		if i > 0 {
			log.Printf("Retry %d/%d...\n", i+1, healthCheckRetries)
			time.Sleep(healthCheckDelay)
		}

		healthURL := fmt.Sprintf("http://%s:%s/health", host, port)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
		if err != nil {
			continue
		}

		client := &http.Client{Timeout: healthCheckTimeout * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Health check attempt %d failed: %v\n", i+1, err)

			continue
		}
		bodyCloseErr := resp.Body.Close()
		if bodyCloseErr != nil {
			log.Println(errors.WithMessage(err, "failed to close response body"))
		}

		if resp.StatusCode == http.StatusOK {
			return nil
		}

		log.Printf("Health check attempt %d returned status %d\n", i+1, resp.StatusCode)
	}

	return errors.New("health check failed after multiple retries")
}
