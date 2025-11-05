package install

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	daemoninstall "github.com/gameap/gameapctl/internal/actions/daemon/install"
	"github.com/gameap/gameapctl/internal/actions/panel/changepassword"
	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/daemon"
	"github.com/gameap/gameapctl/pkg/gameap"
	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/gameap/gameapctl/pkg/oscore"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/panel"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/sethvargo/go-password/password"
	"github.com/urfave/cli/v2"
)

type panelInstallStateV4 struct {
	NonInteractive bool
	SkipWarnings   bool

	HTTPS           bool
	Host            string
	HostIP          string
	Port            string
	ConfigDirectory string
	DataDirectory   string
	AdminPassword   string
	Database        string
	DBCreds         databaseCredentials
	WithDaemon      bool
	OSInfo          osinfo.Info

	// Installation variables
	DatabaseWasInstalled     bool
	DatabaseDirExistedBefore bool
	DatabaseIsNotEmpty       bool
}

//nolint:unparam
func loadPanelInstallStateV4(cliCtx *cli.Context) (panelInstallStateV4, error) {
	state := panelInstallStateV4{}

	state.NonInteractive = cliCtx.Bool("non-interactive")
	state.SkipWarnings = cliCtx.Bool("skip-warnings")

	state.Host = cliCtx.String("host")
	state.Port = cliCtx.String("port")
	state.Database = cliCtx.String("database")
	state.DBCreds = databaseCredentials{
		Host:         cliCtx.String("database-host"),
		Port:         cliCtx.String("database-port"),
		DatabaseName: cliCtx.String("database-name"),
		Username:     cliCtx.String("database-username"),
		Password:     cliCtx.String("database-password"),
	}
	state.WithDaemon = cliCtx.Bool("with-daemon")
	state.OSInfo = contextInternal.OSInfoFromContext(cliCtx.Context)

	// Set default directories
	state.ConfigDirectory = filepath.Dir(gameap.DefaultConfigFilePath)
	state.DataDirectory = gameap.DefaultDataPath

	return state, nil
}

//nolint:gocognit,gocyclo,funlen
func HandleV4(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	state, err := loadPanelInstallStateV4(cliCtx)
	if err != nil {
		return errors.WithMessage(err, "failed to load panel install state")
	}

	fmt.Printf(
		"Detected operating system as %s/%s (%s).\n",
		state.OSInfo.Distribution,
		state.OSInfo.DistributionCodename,
		state.OSInfo.Platform,
	)

	log.Println(state.OSInfo.String())

	//nolint:nestif
	if !state.NonInteractive {
		needToAsk := make(map[string]struct{}, 4) //nolint:mnd
		if state.Host == "" {
			needToAsk["host"] = struct{}{}
		}
		if state.Database == "" {
			needToAsk["database"] = struct{}{}
		}
		answers, err := askUserV4(ctx, needToAsk)
		if err != nil {
			return err
		}

		if _, ok := needToAsk["host"]; ok {
			state.Host = answers.host
		}

		if _, ok := needToAsk["database"]; ok {
			state.Database = answers.database
		}
	}

	if state.Host == "" {
		return errEmptyHost
	}

	if state.Database == "" {
		return errEmptyDatabase
	}

	if state.Port == "" {
		state.Port = "80"
	}

	state, err = checkPortAvailabilityV4(ctx, state)
	if err != nil {
		return errors.WithMessage(err, "failed to check port availability")
	}

	state, err = filterAndCheckHostV4(state)
	if err != nil {
		return errors.WithMessage(err, "failed to check host")
	}

	state, err = checkHTTPHostAvailabilityV4(ctx, state)
	if err != nil {
		return errors.WithMessage(err, "failed to check http host availability")
	}

	fmt.Println()
	fmt.Println("Host:", state.Host)
	fmt.Println("Port:", state.Port)
	fmt.Println("Database:", state.Database)
	fmt.Println()

	pm, err := packagemanager.Load(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to load package manager")
	}

	state, err = checkSELinuxV4(ctx, state)
	if err != nil {
		return errors.WithMessage(err, "failed to check selinux")
	}

	fmt.Println("Checking for updates ...")
	if err = pm.CheckForUpdates(ctx); err != nil {
		return errors.WithMessage(err, "failed to check for updates")
	}

	//nolint:nestif
	if state.OSInfo.IsLinux() {
		fmt.Println("Checking for ca-certificates ...")
		if !utils.IsCommandAvailable("update-ca-certificates") {
			fmt.Println("Installing ca-certificates ...")
			if err = pm.Install(ctx, packagemanager.CACertificatesPackage); err != nil {
				return errors.WithMessage(err, "failed to install curl")
			}

			err = oscore.ExecCommand(ctx, "update-ca-certificates")
			if err != nil {
				return errors.WithMessage(err, "failed to update ca-certificates")
			}

			// Without reloading all certificates may not be applied.
			// I had an error: `failed to install gameap: failed to install GameAP v4:
			//   failed to download binaries: failed to find release:
			//   failed to get releases: Get "https://api.github.com/repos/gameap/gameap/releases":
			//   tls: failed to verify certificate: x509: certificate signed by unknown authority`
			// I think the problem is that the current process still uses old certificates.
			// I tried to time.Sleep(10 * time.Second) but it did not help.
			// I didn't delve too deeply into the problem, so perhaps it's possible to avoid reloading.

			// So now better to re-run the installer.

			fmt.Println()
			fmt.Println("Re-run the installer again to apply updated certificates:")

			fmt.Println(cmdLineFromPanelInstallStateV4(state))

			return nil
		}
	}

	fmt.Println("Checking for curl ...")
	if !utils.IsCommandAvailable("curl") {
		fmt.Println("Installing curl ...")
		if err = pm.Install(ctx, packagemanager.CurlPackage); err != nil {
			return errors.WithMessage(err, "failed to install curl")
		}
	}

	fmt.Println("Checking for gpg ...")
	if !utils.IsCommandAvailable("gpg") {
		fmt.Println("Installing gpg ...")
		if err = pm.Install(ctx, packagemanager.GnuPGPackage); err != nil {
			return errors.WithMessage(err, "failed to install gpg")
		}
	}

	fmt.Println("Checking for tar ...")
	if !utils.IsCommandAvailable("tar") {
		fmt.Println("Installing tar ...")
		if err = pm.Install(ctx, packagemanager.TarPackage); err != nil {
			return errors.WithMessage(err, "failed to install tar")
		}
	}

	switch state.Database {
	case postgresDatabase:
		state, err = installPostgreSQL(ctx, pm, state)
		if err != nil {
			return err
		}
	case mysqlDatabase:
		state, err = installMySQLOrMariaDBV4(ctx, pm, state)
		if err != nil {
			return err
		}
	case sqliteDatabase:
		state, err = installSqliteV4(ctx, state)
		if err != nil {
			return err
		}
	}

	fmt.Println("Installing GameAP ...")
	state, err = installGameAPV4(ctx, state)
	if err != nil {
		return errors.WithMessage(err, "failed to install gameap")
	}

	var daemonInstalled bool

	if state.WithDaemon {
		state, err = daemonInstallV4(ctx, state)
		if err != nil {
			fmt.Println("Failed to install daemon: ", err.Error())
			log.Println(errors.WithMessage(err, "failed to install daemon, try to install it manually"))
		} else {
			daemonInstalled = true
		}
	}

	if err = savePanelInstallationDetailsV4(cliCtx.Context, state); err != nil {
		fmt.Println("Failed to save installation details: ", err.Error())
		log.Println("Failed to save installation details: ", err)
	}

	fmt.Println("Starting GameAP ...")
	err = panel.Start(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to start GameAP")
	}

	err = waitForPanelHealthCheck(
		ctx,
		fmt.Sprintf("http://%s:%s", state.Host, state.Port),
		30, //nolint:mnd
		2*time.Second,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to wait for GameAP health check")
	}

	state, err = updateAdminPasswordv4(ctx, state)
	if err != nil {
		return errors.WithMessage(err, "failed to update admin password")
	}

	if daemonInstalled {
		err = daemon.Start(ctx)
		if err != nil {
			fmt.Println("Failed to start daemon: ", err.Error())
			log.Println(errors.WithMessage(err, "failed to start GameAP daemon"))
		}
	}

	fmt.Println()
	log.Println("GameAP successfully installed")

	fmt.Println("---------------------------------")
	fmt.Println("DONE!")
	fmt.Println()
	fmt.Println("GameAP configuration path:", state.ConfigDirectory)
	fmt.Println("GameAP data path:", state.DataDirectory)
	fmt.Println()

	if state.Database == postgresDatabase {
		fmt.Println("# PostgreSQL database connection details")
		fmt.Println("Database name:", state.DBCreds.DatabaseName)
		if state.DBCreds.RootPassword != "" {
			fmt.Println("Database root password:", state.DBCreds.RootPassword)
		}
		fmt.Println("Database user name:", state.DBCreds.Username)
		fmt.Println("Database user password:", state.DBCreds.Password)
	}

	if state.Database == mysqlDatabase {
		fmt.Println("# MySQL/MariaDB database connection details")
		fmt.Println("Database name:", state.DBCreds.DatabaseName)
		if state.DBCreds.RootPassword != "" {
			fmt.Println("Database root password:", state.DBCreds.RootPassword)
		}
		fmt.Println("Database user name:", state.DBCreds.Username)
		fmt.Println("Database user password:", state.DBCreds.Password)
	}

	if state.Database == sqliteDatabase {
		fmt.Println("# SQLite database details")
		fmt.Println("Database file path:", filepath.Join(state.DataDirectory, "database.sqlite"))
	}

	fmt.Println()
	fmt.Println("Administrator credentials")
	fmt.Println("Login: admin")
	fmt.Println("Password:", state.AdminPassword)
	fmt.Println()
	if state.Port == "80" {
		fmt.Println("Host: http://" + state.Host)
	} else {
		fmt.Println("Host: http://" + state.Host + ":" + state.Port)
	}
	fmt.Println()
	fmt.Println("---------------------------------")

	return nil
}

func installGameAPV4(ctx context.Context, state panelInstallStateV4) (panelInstallStateV4, error) {
	// Build database URL
	var databaseURL string
	var cacheDriver string

	switch state.Database {
	case mysqlDatabase:
		databaseURL = fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?parseTime=true",
			state.DBCreds.Username,
			state.DBCreds.Password,
			state.DBCreds.Host,
			state.DBCreds.Port,
			state.DBCreds.DatabaseName,
		)
		cacheDriver = "mysql"
	case sqliteDatabase:
		databaseURL = state.DBCreds.DatabaseName
		cacheDriver = "inmemory"
	case postgresDatabase:
		databaseURL = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=disable",
			state.DBCreds.Username,
			state.DBCreds.Password,
			state.DBCreds.Host,
			state.DBCreds.Port,
			state.DBCreds.DatabaseName,
		)
		cacheDriver = "redis"
	}

	// Install GameAP v4
	err := panel.Install(ctx, panel.InstallConfig{
		ConfigDirectory: state.ConfigDirectory,
		DataDirectory:   state.DataDirectory,
		HTTPHost:        state.Host,
		HTTPPort:        state.Port,
		DatabaseDriver:  state.Database,
		DatabaseURL:     databaseURL,
		CacheDriver:     cacheDriver,
		FilesDriver:     "local",
	})
	if err != nil {
		return state, errors.WithMessage(err, "failed to install GameAP v4")
	}

	return state, nil
}

func installMySQLOrMariaDBV4(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state panelInstallStateV4,
) (panelInstallStateV4, error) {
	var err error

	if state.DBCreds.Port == "" && state.OSInfo.Distribution != packagemanager.DistributionWindows {
		state.DBCreds.Port = "3306"
	} else if state.DBCreds.Port == "" {
		// Default port for windows
		state.DBCreds.Port = "9306"
	}

	state, err = installMySQLV4(ctx, pm, state)
	//nolint:nestif
	if err != nil {
		fmt.Println("Failed to install MySQL server. Trying to replace by MariaDB ...")
		log.Println(err)

		if state.OSInfo.Distribution != packagemanager.DistributionWindows {
			fmt.Println("Removing MySQL server ...")
			err = pm.Purge(ctx, packagemanager.MySQLServerPackage)
			if err != nil {
				return state, errors.WithMessage(err, "failed to remove MySQL server")
			}
		}

		if state.OSInfo.IsLinux() && !state.DatabaseDirExistedBefore && utils.IsFileExists("/var/lib/mysql") {
			err := os.RemoveAll("/var/lib/mysql")
			if err != nil {
				return state, errors.WithMessage(err, "failed to remove MySQL data directory")
			}
		}

		state, err = installMariaDBV4(ctx, pm, state)
		if err != nil {
			return state, errors.WithMessage(err, "failed to install MariaDB")
		}
	}

	return state, nil
}

//nolint:gocognit
func installMariaDBV4(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state panelInstallStateV4,
) (panelInstallStateV4, error) {
	fmt.Println("Installing MariaDB ...")

	var err error

	//nolint:nestif
	if state.DBCreds.Host == "" ||
		state.DBCreds.Host == "localhost" ||
		strings.HasPrefix(state.DBCreds.Host, "127.") {
		if !isMariaDBInstalled(ctx) {
			state.DBCreds, err = preconfigureMysql(ctx, state.DBCreds)
			if err != nil {
				return state, err
			}

			state.DatabaseDirExistedBefore = true
			if state.OSInfo.IsLinux() {
				_, err := os.Stat("/var/lib/mysql")
				if err != nil && os.IsNotExist(err) {
					state.DatabaseDirExistedBefore = false
				}
			}

			fmt.Println("Installing MariaDB server ...")
			err = pm.Install(ctx, packagemanager.MariaDBServerPackage)
			if err != nil {
				return state, errors.WithMessage(err, "failed to install MariaDB server")
			}

			state.DatabaseWasInstalled = true
		} else {
			fmt.Println("MariaDB already installed")
		}
	}

	fmt.Println("Starting MariaDB server ...")
	//nolint:staticcheck
	switch {
	case state.OSInfo.Distribution == packagemanager.DistributionWindows:
		err = service.Start(ctx, "mariadb")
		if err != nil {
			return state, errors.WithMessage(err, "failed to start MariaDB server")
		}
	default:
		if err = service.Start(ctx, "mysql"); err != nil {
			if err = service.Start(ctx, "mysqld"); err != nil {
				if err = service.Start(ctx, "mariadb"); err != nil {
					return state, errors.WithMessage(err, "failed to start MariaDB server")
				}
			}
		}
	}

	fmt.Println("Configuring MariaDB ...")
	if state.DBCreds.Host == "" || state.DBCreds.Username == "" {
		state.DBCreds, err = preconfigureMysql(ctx, state.DBCreds)
		if err != nil {
			return state, err
		}
	}

	err = configureMysql(ctx, state.DBCreds)
	if err != nil {
		return state, err
	}

	state, err = checkMySQLConnectionV4(ctx, state)
	if err != nil {
		log.Println(err)
		switch state.DBCreds.Host {
		case "localhost":
			state.DBCreds.Host = "127.0.0.1"
		case "127.0.0.1":
			state.DBCreds.Host = "localhost"
		}

		state, err = checkMySQLConnectionV4(ctx, state)
	}

	if err != nil {
		log.Println(err)
		if state.DBCreds.Port != "3306" {
			state.DBCreds.Port = "3306"
			state, err = checkMySQLConnectionV4(ctx, state)
		}
	}

	return state, err
}

//nolint:gocognit,funlen
func installMySQLV4(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state panelInstallStateV4,
) (panelInstallStateV4, error) {
	fmt.Println("Installing MySQL ...")

	var err error

	//nolint:nestif
	if state.DBCreds.Host == "" ||
		state.DBCreds.Host == "localhost" ||
		strings.HasPrefix(state.DBCreds.Host, "127.") {
		if !isMySQLInstalled(ctx) {
			state.DBCreds, err = preconfigureMysql(ctx, state.DBCreds)
			if err != nil {
				return state, err
			}

			state.DatabaseDirExistedBefore = true
			if state.OSInfo.IsLinux() {
				_, err := os.Stat("/var/lib/mysql")
				if err != nil && os.IsNotExist(err) {
					state.DatabaseDirExistedBefore = false
				}
			}

			if err := pm.Install(ctx, packagemanager.MySQLServerPackage); err != nil {
				if state.OSInfo.Distribution == packagemanager.DistributionWindows {
					return state, errors.WithMessage(err, "failed to install mysql")
				}
			}

			state.DatabaseWasInstalled = true
		} else {
			state.DatabaseDirExistedBefore = true

			fmt.Println("MySQL already installed")
		}
	}

	fmt.Println("Starting MySQL server ...")
	switch state.OSInfo.Distribution {
	case packagemanager.DistributionWindows:
		var errNotFound *service.NotFoundError
		err = service.Start(ctx, "mysql")
		if err != nil && errors.As(err, &errNotFound) {
			fmt.Println("MySQL service not found, trying to install ...")
			log.Println(err)
			err = utils.ExecCommand("mysqld", "--install")
			if err != nil {
				return state, errors.WithMessage(err, "failed to exec 'mysqld --install' command")
			}

			err = service.Start(ctx, "mysql")
			if err != nil {
				return state, errors.WithMessage(err, "failed to start MySQL server after 'mysqld --install'")
			}
		}
		if err != nil {
			return state, errors.WithMessage(err, "failed to start MySQL server")
		}
	default:
		if err = service.Start(ctx, "mysql"); err != nil {
			if err = service.Start(ctx, "mysqld"); err != nil {
				if err = service.Start(ctx, "mariadb"); err != nil {
					return state, errors.WithMessage(err, "failed to start MySQL server")
				}
			}
		}
	}

	fmt.Println("Configuring MySQL ...")
	if state.DBCreds.Host == "" || state.DBCreds.Username == "" {
		state.DBCreds, err = preconfigureMysql(ctx, state.DBCreds)
		if err != nil {
			return state, err
		}
	}

	err = configureMysql(ctx, state.DBCreds)
	if err != nil {
		return state, err
	}

	state, err = checkMySQLConnectionV4(ctx, state)
	if err != nil {
		log.Println(err)
		switch state.DBCreds.Host {
		case "localhost":
			state.DBCreds.Host = "127.0.0.1"
		case "127.0.0.1":
			state.DBCreds.Host = "localhost"
		}
		state, err = checkMySQLConnectionV4(ctx, state)
	}

	if err != nil {
		log.Println(err)
		if state.DBCreds.Port != "3306" {
			state.DBCreds.Port = "3306"
			state, err = checkMySQLConnectionV4(ctx, state)
		}
	}

	return state, err
}

func checkMySQLConnectionV4(
	ctx context.Context,
	state panelInstallStateV4,
) (panelInstallStateV4, error) {
	fmt.Println("Checking MySQL connection ...")
	db, err := sql.Open(
		mysqlDatabase,
		fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s",
			state.DBCreds.Username,
			state.DBCreds.Password,
			state.DBCreds.Host,
			state.DBCreds.Port,
			state.DBCreds.DatabaseName,
		),
	)
	if err != nil {
		return state, errors.WithMessage(err, "failed to open MySQL")
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)
	err = db.PingContext(ctx)
	if err != nil {
		return state, errors.WithMessage(err, "failed to connect to MySQL")
	}

	_, err = db.ExecContext(ctx, "SELECT 1")
	if err != nil {
		return state, errors.WithMessage(err, "failed to execute MySQL query")
	}

	isDatabaseEmpty, err := mysqlIsDatabaseEmpty(ctx, db, state.DBCreds.DatabaseName)
	if err != nil {
		return state, errors.WithMessage(err, "failed to check database")
	}

	state.DatabaseIsNotEmpty = !isDatabaseEmpty

	return state, nil
}

func installSqliteV4(_ context.Context, state panelInstallStateV4) (panelInstallStateV4, error) {
	if !utils.IsFileExists(state.DBCreds.DatabaseName) {
		err := os.MkdirAll(state.DataDirectory, 0644)
		if err != nil {
			return state, errors.WithMessage(err, "failed to create data directory for sqlite database")
		}
	}

	dbPath := filepath.Join(state.DataDirectory, "database.sqlite")
	f, err := os.Create(dbPath)
	if err != nil {
		return state, errors.WithMessage(err, "failed to create database.sqlite")
	}
	err = f.Close()
	if err != nil {
		return state, errors.WithMessage(err, "failed to close database.sqlite")
	}

	state.DBCreds.DatabaseName = dbPath
	state.DatabaseWasInstalled = true

	return state, nil
}

func installPostgreSQL(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state panelInstallStateV4,
) (panelInstallStateV4, error) {
	err := pm.Install(ctx, packagemanager.PostgreSQLPackage)
	if err != nil {
		return state, errors.WithMessage(err, "failed to install PostgreSQL")
	}

	return state, nil
}

//nolint:funlen
func daemonInstallV4(ctx context.Context, state panelInstallStateV4) (panelInstallStateV4, error) {
	token := fmt.Sprintf("gameapctl%d", time.Now().UnixMilli())

	configPath := filepath.Join(state.ConfigDirectory, "config.env")

	// Append DAEMON_SETUP_TOKEN to config.env
	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return state, errors.WithMessage(err, "failed to open config.env")
	}

	tokenLine := fmt.Sprintf("\nDAEMON_SETUP_TOKEN=%s\n", token)
	_, err = f.WriteString(tokenLine)
	if err != nil {
		closeErr := f.Close()
		if closeErr != nil {
			log.Println(errors.WithMessage(closeErr, "failed to close config.env after write error"))
		}

		return state, errors.WithMessage(err, "failed to write DAEMON_SETUP_TOKEN to config.env")
	}
	closeErr := f.Close()
	if closeErr != nil {
		log.Println(errors.WithMessage(closeErr, "failed to close config.env after write error"))
	}

	// Remove token from config after daemon installation
	defer func() {
		// Read the config file
		content, err := os.ReadFile(configPath)
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to read config.env for cleanup"))

			return
		}

		// Remove the DAEMON_SETUP_TOKEN line
		lines := strings.Split(string(content), "\n")
		var filteredLines []string
		for _, line := range lines {
			if !strings.HasPrefix(strings.TrimSpace(line), "DAEMON_SETUP_TOKEN=") {
				filteredLines = append(filteredLines, line)
			}
		}

		// Write back without the token line
		err = os.WriteFile(configPath, []byte(strings.Join(filteredLines, "\n")), 0600)
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to remove DAEMON_SETUP_TOKEN from config.env"))
		}
	}()

	// Temporary start gameap with DAEMON_SETUP_TOKEN in config.env
	err = panel.Start(ctx)
	if err != nil {
		return state, errors.WithMessage(err, "failed to start GameAP")
	}

	defer func() {
		err = panel.Stop(ctx)
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to stop GameAP"))
		}
	}()

	host := "http://" + state.Host + ":" + state.Port

	// Wait for GameAP panel to be ready using health check
	maxRetries := 30
	retryDelay := 2 * time.Second

	err = waitForPanelHealthCheck(ctx, host, maxRetries, retryDelay)
	if err != nil {
		return state, err
	}

	// Make HTTP GET request to retrieve createToken
	client := &http.Client{
		Timeout: 5 * time.Second, //nolint:mnd
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/gdaemon/setup/%s", host, token),
		nil,
	)
	if err != nil {
		return state, errors.WithMessage(err, "failed to create HTTP request for daemon setup")
	}

	resp, err := client.Do(req)
	if err != nil {
		return state, errors.WithMessage(err, "failed to get daemon setup token")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return state, errors.New("failed to get daemon setup: non-200 status code")
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return state, errors.WithMessage(err, "failed to read daemon setup response")
	}

	// Parse createToken from response
	createToken := ""
	split := strings.Split(string(body), ";")
	for _, s := range split {
		switch {
		case strings.HasPrefix(s, "export createToken="):
			createToken = strings.TrimPrefix(s, "export createToken=")
		case strings.HasPrefix(s, "export CREATE_TOKEN="):
			createToken = strings.TrimPrefix(s, "export CREATE_TOKEN=")
		}
	}

	if createToken == "" {
		return state, errors.New("failed to extract create token from daemon setup response")
	}

	err = daemoninstall.Install(
		ctx,
		host,
		createToken,
	)
	if err != nil {
		return state, errors.WithMessage(err, "failed to install daemon")
	}

	return state, nil
}

func waitForPanelHealthCheck(ctx context.Context, host string, maxRetries int, retryDelay time.Duration) error {
	client := &http.Client{
		Timeout: 5 * time.Second, //nolint:mnd
	}

	healthCheckURL := fmt.Sprintf("%s/health", host)

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthCheckURL, nil)
		if err != nil {
			return errors.WithMessage(err, "failed to create health check request")
		}

		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()

			return nil
		}

		if resp != nil {
			resp.Body.Close()
		}

		if i == maxRetries-1 {
			return errors.New("GameAP panel failed to become ready in time")
		}

		time.Sleep(retryDelay)
	}

	return nil
}

func updateAdminPasswordv4(ctx context.Context, state panelInstallStateV4) (panelInstallStateV4, error) {
	var err error
	if state.AdminPassword == "" {
		fmt.Println("Generating admin password ...")

		state.AdminPassword, err = password.Generate(
			defaultPasswordLen, defaultPasswordNumDigits, 0, false, false,
		)
		if err != nil {
			return state, errors.WithMessage(err, "failed to generate password")
		}
	}

	err = changepassword.ChangePassword(ctx, "admin", state.AdminPassword)
	if err != nil {
		return state, errors.WithMessage(err, "failed to change admin password")
	}

	return state, nil
}

func cmdLineFromPanelInstallStateV4(state panelInstallStateV4) string {
	sb := strings.Builder{}
	sb.Grow(128) //nolint:mnd

	if state.OSInfo.IsWindows() {
		sb.WriteString("gameapctl.exe ")
	} else {
		sb.WriteString("gameapctl ")
	}

	sb.WriteString("panel install --version=4 --host=")
	sb.WriteString(state.Host)
	sb.WriteString(" --port=")
	sb.WriteString(state.Port)
	sb.WriteString(" --database=")
	sb.WriteString(state.Database)

	if state.WithDaemon {
		sb.WriteString(" --with-daemon")
	}

	return sb.String()
}

func savePanelInstallationDetailsV4(ctx context.Context, state panelInstallStateV4) error {
	return gameapctl.SavePanelInstallState(ctx, gameapctl.PanelInstallState{
		Version:              "4",
		Host:                 state.Host,
		HostIP:               state.HostIP,
		Port:                 state.Port,
		ConfigDirectory:      state.ConfigDirectory,
		DataDirectory:        state.DataDirectory,
		Database:             state.Database,
		DatabaseWasInstalled: state.DatabaseWasInstalled,
	})
}
