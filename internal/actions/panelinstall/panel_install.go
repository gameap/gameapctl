package panelinstall

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/internal/pkg/panel"
	"github.com/gameap/gameapctl/pkg/gameap"
	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/sethvargo/go-password/password"
	"github.com/urfave/cli/v2"
)

const (
	mysqlDatabase  = "mysql"
	sqliteDatabase = "sqlite"
	noneDatabase   = "none"
)

const (
	nginxWebServer  = "nginx"
	apacheWebServer = "apache"
	noneWebServer   = "none"
)

const (
	defaultMysqlUsername = "gameap"
	defaultMysqlHost     = "localhost"
	defaultMysqlDatabase = "gameap"
)

const (
	defaultPasswordLen        = 16
	defaultPasswordNumDigits  = 6
	defaultPasswordNumSymbols = 2
)

var errEmptyPath = errors.New("empty path")
var errEmptyHost = errors.New("empty host")
var errEmptyDatabase = errors.New("empty database")
var errEmptyWebServer = errors.New("empty web server")
var errApacheWindowsIsNotSupported = errors.New("apache is not supported yet, sorry")

type panelInstallState struct {
	NonInteractive bool
	SkipWarnings   bool

	HTTPS         bool
	Host          string
	HostIP        string
	Port          string
	Path          string
	AdminPassword string
	WebServer     string
	FromGithub    bool
	Branch        string
	Database      string
	DBCreds       databaseCredentials
	OSInfo        osinfo.Info

	// Installation variables
	DatabaseWasInstalled bool
	DatabaseIsNotEmpty   bool
}

type databaseCredentials struct {
	Host         string
	Port         string
	DatabaseName string
	Username     string
	Password     string
	RootPassword string
}

//nolint:funlen,gocognit,gocyclo
func Handle(cliCtx *cli.Context) error {
	var err error
	state := panelInstallState{}

	state.NonInteractive = cliCtx.Bool("non-interactive")
	state.Host = cliCtx.String("host")
	state.Port = cliCtx.String("port")
	state.Path = cliCtx.String("path")
	state.WebServer = cliCtx.String("web-server")
	state.Database = cliCtx.String("database")
	state.DBCreds = databaseCredentials{
		Host:         cliCtx.String("database-host"),
		Port:         cliCtx.String("database-port"),
		DatabaseName: cliCtx.String("database-name"),
		Username:     cliCtx.String("database-username"),
		Password:     cliCtx.String("database-password"),
	}
	state.OSInfo = contextInternal.OSInfoFromContext(cliCtx.Context)

	state.FromGithub = cliCtx.Bool("github")
	developBranch := cliCtx.Bool("develop")
	if developBranch {
		state.Branch = "develop"
	} else {
		state.Branch = cliCtx.String("branch")
	}

	if state.Branch == "" {
		state.Branch = "master"
	}

	fmt.Printf(
		"Detected operating system as %s/%s (%s).\n",
		state.OSInfo.Distribution,
		state.OSInfo.DistributionCodename,
		state.OSInfo.Platform,
	)

	//nolint:nestif
	if !state.NonInteractive {
		needToAsk := make(map[string]struct{}, 4) //nolint:gomnd
		if state.Host == "" {
			needToAsk["host"] = struct{}{}
		}
		if state.Path == "" {
			needToAsk["path"] = struct{}{}
		}
		if state.Database == "" {
			needToAsk["database"] = struct{}{}
		}
		if state.WebServer == "" {
			needToAsk["webServer"] = struct{}{}
		}
		answers, err := askUser(cliCtx.Context, state, needToAsk)
		if err != nil {
			return err
		}

		if _, ok := needToAsk["path"]; ok {
			state.Path = answers.path
		}

		if _, ok := needToAsk["host"]; ok {
			state.Host = answers.host
		}

		if _, ok := needToAsk["database"]; ok {
			state.Database = answers.database
		}

		if _, ok := needToAsk["webServer"]; ok {
			state.WebServer = answers.webServer
		}
	}

	if state.WebServer == apacheWebServer && state.OSInfo.Distribution == packagemanager.DistributionWindows {
		return errApacheWindowsIsNotSupported
	}

	if state.Path == "" {
		return errEmptyPath
	}

	if state.Host == "" {
		return errEmptyHost
	}

	if state.Database == "" {
		return errEmptyDatabase
	}

	if state.WebServer == "" {
		return errEmptyWebServer
	}

	if state.Port == "" {
		state.Port = "80"
	}

	state, err = filterAndCheckHost(state)
	if err != nil {
		return errors.WithMessage(err, "failed to check host")
	}

	state, err = checkWebServers(cliCtx.Context, state)
	if err != nil {
		return errors.WithMessage(err, "failed to check web servers")
	}

	state, err = checkHTTPHostAvailability(cliCtx.Context, state)
	if err != nil {
		return errors.WithMessage(err, "failed to check http host availability")
	}

	fmt.Println()
	fmt.Println("Path:", state.Path)
	fmt.Println("Host:", state.Host)
	fmt.Println("Port:", state.Port)
	fmt.Println("Database:", state.Database)
	fmt.Println("Web server:", state.WebServer)
	fmt.Println("Develop:", cliCtx.Bool("develop"))
	if state.FromGithub {
		fmt.Println("Installation from GitHub: yes")
		fmt.Println("Branch:", state.Branch)
	}
	fmt.Println()

	pm, err := packagemanager.Load(cliCtx.Context)
	if err != nil {
		return errors.WithMessage(err, "failed to load package manager")
	}

	fmt.Println("Checking for updates ...")
	if err = pm.CheckForUpdates(cliCtx.Context); err != nil {
		return errors.WithMessage(err, "failed to check for updates")
	}

	fmt.Println("Checking for curl ...")
	if !utils.IsCommandAvailable("curl") {
		fmt.Println("Installing curl ...")
		if err = pm.Install(cliCtx.Context, packagemanager.CurlPackage); err != nil {
			return errors.WithMessage(err, "failed to install curl")
		}
	}

	fmt.Println("Checking for gpg ...")
	if !utils.IsCommandAvailable("gpg") {
		fmt.Println("Installing gpg ...")
		if err = pm.Install(cliCtx.Context, packagemanager.GnuPGPackage); err != nil {
			return errors.WithMessage(err, "failed to install gpg")
		}
	}

	fmt.Println("Checking for php ...")
	state, err = checkAndInstallPHP(cliCtx.Context, pm, state)
	if err != nil {
		return errors.WithMessage(err, "failed to check and install php")
	}

	state, err = checkPHPExtensions(cliCtx.Context, state)
	if err != nil {
		log.Println(err)

		fmt.Println("Installing needed php extensions ...")
		if err = pm.Install(cliCtx.Context, packagemanager.PHPExtensionsPackage); err != nil {
			return errors.WithMessage(err, "failed to install php extensions")
		}

		state, err = checkPHPExtensions(cliCtx.Context, state)
		if err != nil {
			return errors.WithMessage(err, "failed to check php extensions")
		}
	}

	if state.FromGithub {
		fmt.Println("Installing GameAP from github ...")
		state, err = installGameAPFromGithub(cliCtx.Context, pm, state)
	} else {
		fmt.Println("Installing GameAP ...")
		state, err = installGameAP(cliCtx.Context, state)
	}
	if err != nil {
		return err
	}

	err = packagemanager.TryBindPHPDirectories(cliCtx.Context, state.Path)
	if err != nil {
		return errors.WithMessage(err, "failed to bind php directories")
	}

	switch state.Database {
	case mysqlDatabase:
		state, err = installMySQL(cliCtx.Context, pm, state)
		if err != nil {
			return err
		}
	case sqliteDatabase:
		state, err = installSqlite(cliCtx.Context, state)
		if err != nil {
			return err
		}
	}

	state, err = updateDotEnv(cliCtx.Context, state)
	if err != nil {
		return errors.WithMessage(err, "failed to update .env")
	}

	err = panel.GenerateEncryptionKey(cliCtx.Context, state.Path)
	if err != nil {
		return errors.WithMessage(err, "failed to generate encryption key")
	}

	state, err = runMigrationWithRetry(cliCtx.Context, state)
	if err != nil {
		return errors.WithMessage(err, "failed to run migration")
	}

	switch state.WebServer {
	case nginxWebServer:
		fmt.Println("Installing nginx ...")
		state, err = installNginx(cliCtx.Context, pm, state)
		if err != nil {
			return errors.WithMessage(err, "failed to install nginx")
		}
	case apacheWebServer:
		fmt.Println("Installing apache ...")
		state, err = installApache(cliCtx.Context, pm, state)
		if err != nil {
			return errors.WithMessage(err, "failed to install apache")
		}
	}

	if state.OSInfo.Distribution != packagemanager.DistributionWindows {
		err = configureCron(cliCtx.Context, state)
		if err != nil {
			log.Println("Failed to configure cron: ", err)
			fmt.Println("Failed to configure cron: ", err.Error())
		}
	}

	state, err = updateAdminPassword(cliCtx.Context, state)
	if err != nil {
		return errors.WithMessage(err, "failed to update admin password")
	}

	state, err = clearGameAPCache(cliCtx.Context, state)
	if err != nil {
		return errors.WithMessage(err, "failed to clear panel cache")
	}

	if state.OSInfo.Distribution != packagemanager.DistributionWindows {
		fmt.Println("Updating files permissions ...")
		err = utils.ExecCommand("chown", "-R", "www-data:www-data", state.Path)
		if err != nil {
			return errors.WithMessage(err, "failed to change owner")
		}
	}

	if state.WebServer != noneWebServer {
		fmt.Println("Checking panel installation ...")
		if state, err = checkInstallation(cliCtx.Context, state); err != nil {
			fmt.Println("Installation checking failed")
			log.Println("Installation checking failed")
			log.Println(err)
			if state, err = tryToFixPanelInstallation(cliCtx.Context, state); err != nil {
				return errors.WithMessage(err, "failed to check and fixpanel installation")
			}
		}
	}

	if err = savePanelInstallationDetails(cliCtx.Context, state); err != nil {
		fmt.Println("Failed to save installation details: ", err.Error())
		log.Println("Failed to save installation details: ", err)
	}

	log.Println("GameAP successfully installed")

	fmt.Println("---------------------------------")
	fmt.Println("DONE!")
	fmt.Println()
	fmt.Println("GameAP file path:", state.Path)
	fmt.Println()

	if state.Database == mysqlDatabase {
		fmt.Println("Database name:", state.DBCreds.DatabaseName)
		if state.DBCreds.RootPassword != "" {
			fmt.Println("Database root password:", state.DBCreds.RootPassword)
		}
		fmt.Println("Database user name:", state.DBCreds.Username)
		fmt.Println("Database user password:", state.DBCreds.Password)
	}

	if state.Database == sqliteDatabase {
		fmt.Println("Database file path:", state.Path+"/database.sqlite")
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

func isMySQLInstalled(_ context.Context) bool {
	return utils.IsCommandAvailable("mysqld")
}

//nolint:funlen,gocognit
func installMySQL(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state panelInstallState,
) (panelInstallState, error) {
	fmt.Println("Installing MySQL ...")

	var err error

	if state.DBCreds.Port == "" && state.OSInfo.Distribution != packagemanager.DistributionWindows {
		state.DBCreds.Port = "3306"
	} else if state.DBCreds.Port == "" {
		// Default port for windows
		state.DBCreds.Port = "9306"
	}

	//nolint:nestif
	if state.DBCreds.Host == "" {
		if !isMySQLInstalled(ctx) {
			state.DBCreds, err = preconfigureMysql(ctx, state.DBCreds)
			if err != nil {
				return state, err
			}

			isDataDirExistsBefore := true
			if state.OSInfo.OS == "GNU/Linux" {
				_, err := os.Stat("/var/lib/mysql")
				if err != nil && os.IsNotExist(err) {
					isDataDirExistsBefore = false
				}
			}

			if err := pm.Install(ctx, packagemanager.MySQLServerPackage); err != nil {
				if state.OSInfo.Distribution == packagemanager.DistributionWindows {
					return state, errors.WithMessage(err, "failed to install mysql")
				}

				fmt.Println("Failed to install MySQL server. Trying to replace by MariaDB ...")
				log.Println(err)

				fmt.Println("Removing MySQL server ...")
				err = pm.Purge(ctx, packagemanager.MySQLServerPackage)
				if err != nil {
					return state, errors.WithMessage(err, "failed to remove MySQL server")
				}

				if state.OSInfo.OS == "GNU/Linux" && !isDataDirExistsBefore {
					err := os.RemoveAll("/var/lib/mysql")
					if err != nil {
						return state, errors.WithMessage(err, "failed to remove MySQL data directory")
					}
				}

				fmt.Println("Installing MariaDB server ...")
				err = pm.Install(ctx, packagemanager.MariaDBServerPackage)
				if err != nil {
					return state, errors.WithMessage(err, "failed to install MariaDB server")
				}
			}

			state.DatabaseWasInstalled = true
		} else {
			fmt.Println("MySQL already installed")
		}
	}

	fmt.Println("Starting MySQL server ...")
	if err = service.Start(ctx, "mysql"); err != nil {
		if err = service.Start(ctx, "mysqld"); err != nil {
			if err = service.Start(ctx, "mariadb"); err != nil {
				return state, errors.WithMessage(err, "failed to start MySQL server")
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

	return checkMySQLConnection(ctx, state)
}

func checkMySQLConnection(
	ctx context.Context,
	state panelInstallState,
) (panelInstallState, error) {
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

	if !isDatabaseEmpty {
		state.DatabaseIsNotEmpty = true
	}

	return state, nil
}

func preconfigureMysql(_ context.Context, dbCreds databaseCredentials) (databaseCredentials, error) {
	if dbCreds.Username == "" {
		dbCreds.Username = defaultMysqlUsername
	}

	if dbCreds.DatabaseName == "" {
		dbCreds.DatabaseName = defaultMysqlDatabase
	}

	if dbCreds.Host == "" {
		dbCreds.Host = defaultMysqlHost
	}

	passwordGenerator, err := password.NewGenerator(&password.GeneratorInput{
		Symbols: "_-+=",
	})
	if err != nil {
		return dbCreds, errors.WithMessage(err, "failed to create password generator")
	}

	if dbCreds.Password == "" {
		dbCreds.Password, err = passwordGenerator.Generate(
			defaultPasswordLen, defaultPasswordNumDigits, defaultPasswordNumSymbols, false, false,
		)
		if err != nil {
			return dbCreds, errors.WithMessage(err, "failed to generate password")
		}
	}

	if dbCreds.RootPassword == "" {
		dbCreds.RootPassword, err = passwordGenerator.Generate(
			defaultPasswordLen, defaultPasswordNumDigits, defaultPasswordNumSymbols, false, false,
		)
		if err != nil {
			return dbCreds, errors.WithMessage(err, "failed to generate password")
		}
	}

	return dbCreds, nil
}

func runMigrationWithRetry(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	withSeed := state.DatabaseWasInstalled && !state.DatabaseIsNotEmpty

	err := panel.RunMigration(ctx, state.Path, withSeed)
	if err != nil && state.DBCreds.Host == "localhost" {
		state.DBCreds.Host = "127.0.0.1"
		state, err = updateDotEnv(ctx, state)
		if err != nil {
			return state, err
		}
		err = panel.RunMigration(ctx, state.Path, withSeed)
	}

	return state, err
}

func configureMysql(ctx context.Context, dbCreds databaseCredentials) error {
	db, err := mysqlMakeAdminConnection(ctx, dbCreds)
	if err != nil {
		return errors.WithMessage(err, "failed to make admin connection")
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

	var databaseExists bool
	databaseExists, err = mysqlIsDatabaseExists(ctx, db, dbCreds.DatabaseName)
	if err != nil {
		return errors.WithMessage(err, "failed to check database")
	}

	if !databaseExists {
		fmt.Println("Creating database ...")
		err = mysqlCreateDatabase(ctx, db, dbCreds.DatabaseName)
		if err != nil {
			return errors.WithMessage(err, "failed to create database")
		}
	}

	var userExists bool
	userExists, err = mysqlIsUsereExists(ctx, db, dbCreds.Username, dbCreds.Host)
	if err != nil {
		return errors.WithMessage(err, "failed to create database")
	}

	if !userExists {
		fmt.Println("Creating user ...")
		err = mysqlCreateUser(ctx, db, dbCreds.Username, dbCreds.Password)
		if err != nil {
			return errors.WithMessage(err, "failed to create user")
		}
	} else {
		fmt.Printf("User '%s' already exists\n", dbCreds.Username)
	}

	if userExists && dbCreds.Username == "gameap" {
		fmt.Println("Changing mysql gameap user password ...")
		err = mysqlChangeUserPassword(ctx, db, dbCreds.Username, dbCreds.Password)
		if err != nil {
			return errors.WithMessage(err, "failed to change user password")
		}
	}

	fmt.Println("Granting privileges ...")
	err = mysqlGrantPrivileges(ctx, db, dbCreds.Username, dbCreds.DatabaseName)
	if err != nil {
		return errors.WithMessage(err, "failed to grant privileges")
	}

	return nil
}

func installSqlite(_ context.Context, state panelInstallState) (panelInstallState, error) {
	dbPath := filepath.Join(state.Path, "database.sqlite")
	f, err := os.Create(dbPath)
	if err != nil {
		return state, errors.WithMessage(err, "failed to database.sqlite")
	}
	err = f.Close()
	if err != nil {
		return state, errors.WithMessage(err, "failed to close database.sqlite")
	}

	state.DBCreds.DatabaseName = dbPath
	state.DatabaseWasInstalled = true

	return state, nil
}

func checkAndInstallPHP(
	ctx context.Context, pm packagemanager.PackageManager, state panelInstallState,
) (panelInstallState, error) {
	if !packagemanager.IsPHPCommandAvailable(ctx) || state.OSInfo.Distribution == packagemanager.DistributionWindows {
		fmt.Println("Installing php ...")
		if err := pm.Install(ctx, packagemanager.PHPPackage); err != nil {
			return state, errors.WithMessage(err, "failed to install php")
		}
	}

	return state, nil
}

func checkPHPExtensions(_ context.Context, state panelInstallState) (panelInstallState, error) {
	extensions, err := packagemanager.DefinePHPExtensions()
	if err != nil {
		return state, errors.WithMessage(err, "failed to define php extensions")
	}

	log.Println("Found PHP extensions:", extensions)

	for _, extension := range []string{
		"bcmath", "bz2", "curl", "dom",
		"fileinfo", "gd", "gmp", "intl",
		"json", "mbstring", "openssl", "pdo",
		"tokenizer", "readline", "sockets",
		"session", "xml", "zip",
	} {
		if !utils.Contains(extensions, extension) {
			return state, errors.Errorf("PHP extension %s not found", extension)
		}
	}

	if state.Database == mysqlDatabase && !utils.Contains(extensions, "pdo_mysql") {
		return state, errors.New("pdo_mysql extension not found")
	}

	if state.Database == sqliteDatabase && !utils.Contains(extensions, "pdo_sqlite") {
		return state, errors.New("pdo_sqlite extension not found")
	}

	return state, nil
}

func installGameAP(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	path := state.Path

	tempDir, err := os.MkdirTemp("", "gameap")
	if err != nil {
		return state, errors.WithMessage(err, "failed to create temp dir")
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			log.Println(err)
		}
	}(tempDir)

	fmt.Println("Downloading GameAP ...")
	downloadPath, err := url.JoinPath(gameap.Repository(), "gameap/latest.tar.gz")
	if err != nil {
		return state, errors.WithMessage(err, "failed to join url")
	}

	err = utils.Download(ctx, downloadPath, tempDir)
	if err != nil {
		return state, errors.WithMessagef(err, "failed to download gameap from '%s'", downloadPath)
	}

	err = utils.Move(tempDir+string(os.PathSeparator)+"gameap", path)
	if err != nil {
		return state, errors.WithMessage(err, "failed to move gameap")
	}

	fmt.Println("Installing GameAP ...")
	err = utils.Copy(path+string(os.PathSeparator)+".env.example", path+string(os.PathSeparator)+".env")
	if err != nil {
		return state, errors.WithMessage(err, "failed to copy .env.example")
	}

	return state, nil
}

func installGameAPFromGithub(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state panelInstallState,
) (panelInstallState, error) {
	var err error

	fmt.Println("Installing git ...")
	if err = pm.Install(ctx, packagemanager.GitPackage); err != nil {
		return state, errors.WithMessage(err, "failed to install git")
	}

	fmt.Println("Installing composer ...")
	if err = pm.Install(ctx, packagemanager.ComposerPackage); err != nil {
		return state, errors.WithMessage(err, "failed to install composer")
	}

	fmt.Println("Installing nodejs ...")
	if err = pm.Install(ctx, packagemanager.NodeJSPackage); err != nil {
		return state, errors.WithMessage(err, "failed to install nodejs")
	}

	fmt.Println("Cloning gameap ...")
	err = utils.ExecCommand(
		"git", "clone", "-b", state.Branch, "https://github.com/et-nik/gameap.git", state.Path,
	)
	if err != nil {
		return state, errors.WithMessage(err, "failed to clone gameap from github")
	}

	fmt.Println("Installing composer dependencies ...")

	cmdName, args, err := packagemanager.DefinePHPComposerCommandAndArgs(
		"update", "--no-dev", "--optimize-autoloader", "--no-interaction", "--working-dir", state.Path,
	)
	if err != nil {
		return state, errors.WithMessage(err, "failed to define php composer command and args")
	}

	err = utils.ExecCommand(cmdName, args...)
	if err != nil {
		return state, errors.WithMessage(err, "failed to run composer update")
	}

	fmt.Println("Building styles ...")
	err = panel.BuildStyles(ctx, state.Path)
	if err != nil {
		return state, errors.WithMessage(err, "failed to build styles")
	}

	return state, nil
}

func updateDotEnv(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	var err error

	fmt.Println("Updating .env ...")

	envPath := filepath.Join(state.Path, ".env")

	u := "http://" + state.Host
	if state.HTTPS {
		u = "https://" + state.Host
	}

	if !utils.IsFileExists(envPath) {
		envExamplePath := filepath.Join(state.Path, ".env.example")
		err = utils.Copy(envExamplePath, envPath)
		if err != nil {
			return state, err
		}
	}

	err = utils.FindLineAndReplace(ctx, envPath, map[string]string{
		"APP_URL=":       "APP_URL=" + u,
		"DB_CONNECTION=": "DB_CONNECTION=" + state.Database,
		"DB_HOST=":       "DB_HOST=" + state.DBCreds.Host,
		"DB_PORT=":       "DB_PORT=" + state.DBCreds.Port,
		"DB_DATABASE=":   "DB_DATABASE=" + state.DBCreds.DatabaseName,
		"DB_USERNAME=":   "DB_USERNAME=" + state.DBCreds.Username,
		"DB_PASSWORD=":   "DB_PASSWORD=" + state.DBCreds.Password,
	})
	if err != nil {
		return state, errors.WithMessage(err, "failed to update .env file")
	}

	return state, nil
}

//nolint:funlen
func installNginx(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state panelInstallState,
) (panelInstallState, error) {
	err := pm.Install(ctx, packagemanager.NginxPackage)
	if err != nil {
		return state, errors.WithMessage(err, "failed to install package")
	}

	gameapHostConfPath, err := packagemanager.ConfigForDistro(
		ctx,
		packagemanager.NginxPackage,
		"gameap_host_conf",
	)
	if err != nil {
		return state, err
	}

	if utils.IsFileExists(gameapHostConfPath) {
		err = os.Remove(gameapHostConfPath)
		if err != nil {
			return state, errors.WithMessage(err, "failed to remove gameap nginx config")
		}
	}

	if !utils.IsFileExists(filepath.Dir(gameapHostConfPath)) {
		err = os.MkdirAll(filepath.Dir(gameapHostConfPath), os.ModePerm)
		if err != nil {
			return state, errors.WithMessage(err, "failed to create directory")
		}
	}

	err = utils.DownloadFile(
		ctx,
		"https://raw.githubusercontent.com/gameap/auto-install-scripts/master/web-server-configs/nginx-no-ssl.conf",
		gameapHostConfPath,
	)
	if err != nil {
		return state, errors.WithMessage(err, "failed to download nginx config")
	}

	socketPath, err := packagemanager.ConfigForDistro(
		ctx,
		packagemanager.PHPPackage,
		"fpm_sock",
	)
	if err != nil {
		return state, errors.WithMessage(err, "failed to get fpm_sock config for distro")
	}

	if socketPath != "" {
		err = utils.FindLineAndReplace(ctx, gameapHostConfPath, map[string]string{
			"fastcgi_pass    unix": fmt.Sprintf("fastcgi_pass %s;", socketPath),
		})
		if err != nil {
			return state, errors.WithMessage(err, "failed to update fastcgi_pass in nginx host config")
		}
	} else {
		err = utils.FindLineAndReplace(ctx, gameapHostConfPath, map[string]string{
			"#fastcgi_pass    localhost": "fastcgi_pass localhost:9000;",
		})
		if err != nil {
			return state, errors.WithMessage(err, "failed to update fastcgi_pass in nginx host config")
		}
	}

	err = utils.FindLineAndReplace(ctx, gameapHostConfPath, map[string]string{
		"server_name":                  fmt.Sprintf("server_name %s;", state.Host),
		"listen":                       fmt.Sprintf("listen %s;", state.Port),
		"root /var/www/gameap/public;": fmt.Sprintf("root %s%c%s;", state.Path, os.PathSeparator, "public"),
	})
	if err != nil {
		return state, errors.WithMessage(err, "failed to update nginx host config")
	}

	nginxMainConf, err := packagemanager.ConfigForDistro(
		ctx,
		packagemanager.NginxPackage,
		"nginx_conf",
	)
	if err != nil {
		return state, errors.WithMessage(err, "failed to get nginx_conf")
	}

	switch {
	case state.OSInfo.Distribution == packagemanager.DistributionWindows:

		err = os.Rename(nginxMainConf, nginxMainConf+".old")
		if err != nil {
			return state, errors.WithMessage(err, "failed to rename config")
		}

		err = utils.DownloadFile(
			ctx,
			"https://raw.githubusercontent.com/gameap/auto-install-scripts/master/web-server-configs/nginx-windows.conf",
			nginxMainConf,
		)
		if err != nil {
			return state, errors.WithMessage(err, "failed to download nginx config")
		}

	case state.OSInfo.Distribution == packagemanager.DistributionUbuntu,
		state.OSInfo.Distribution == packagemanager.DistributionDebian:

		err = utils.FindLineAndReplace(ctx, nginxMainConf, map[string]string{
			"user": "user www-data;",
		})
		if err != nil {
			return state, errors.WithMessage(err, "failed to update nginx config")
		}
	}

	phpServiceName := "php-fpm"
	if state.OSInfo.Distribution != packagemanager.DistributionWindows {
		phpVersion, err := packagemanager.DefinePHPVersion()
		if err != nil {
			return state, errors.WithMessage(err, "failed to define php version")
		}
		phpServiceName = "php" + phpVersion + "-fpm"
	}

	err = service.Start(ctx, phpServiceName)
	if err != nil {
		return state, errors.WithMessage(err, "failed to start php-fpm")
	}

	err = service.Start(ctx, "nginx")
	if err != nil {
		return state, errors.WithMessage(err, "failed to start nginx")
	}

	return state, nil
}

//nolint:funlen
func installApache(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state panelInstallState,
) (panelInstallState, error) {
	err := pm.Install(ctx, packagemanager.ApachePackage)
	if err != nil {
		return state, errors.WithMessage(err, "failed to install apache")
	}

	gameapHostConf, err := packagemanager.ConfigForDistro(
		ctx,
		packagemanager.ApachePackage,
		"gameap_host_conf",
	)
	if err != nil {
		return state, errors.WithMessage(err, "failed to get gameap_host_conf")
	}

	err = utils.DownloadFile(
		ctx,
		"https://raw.githubusercontent.com/gameap/auto-install-scripts/master/web-server-configs/apache-no-ssl.conf",
		gameapHostConf,
	)
	if err != nil {
		return state, errors.WithMessage(err, "failed to download apache config")
	}

	err = utils.FindLineAndReplace(ctx, gameapHostConf, map[string]string{
		"ServerName":                         fmt.Sprintf("ServerName %s", state.Host),
		"DocumentRoot":                       fmt.Sprintf("DocumentRoot %s/public", state.Path),
		"<VirtualHost":                       fmt.Sprintf("<VirtualHost *:%s>", state.Port),
		"<Directory /var/www/gameap/public>": fmt.Sprintf("<Directory %s/public>", state.Path),
	})
	if err != nil {
		return state, errors.WithMessage(err, "failed to update apache config")
	}

	if state.Port != "80" {
		err = utils.FindLineAndReplace(ctx, gameapHostConf, map[string]string{
			"# Listen 80": fmt.Sprintf("Listen %s", state.Port),
		})
		if err != nil {
			return state, errors.WithMessage(err, "failed to update apache ports config")
		}
	}

	err = utils.ExecCommand("a2enmod", "rewrite")
	if err != nil {
		return state, errors.WithMessage(err, "failed to enable apache rewrite module")
	}

	err = utils.ExecCommand("a2ensite", "gameap")
	if err != nil {
		return state, errors.WithMessage(err, "failed to enable site")
	}

	err = service.Start(ctx, "apache2")
	if err != nil {
		return state, errors.WithMessage(err, "failed to start apache")
	}

	return state, nil
}

//nolint:funlen
func configureCron(_ context.Context, state panelInstallState) error {
	fmt.Println("Configuring cron ...")

	if utils.IsCommandAvailable("crontab") {
		fmt.Println("Crontab is not available. Skip cron configuration")

		return nil
	}

	cmd := exec.Command("crontab", "-l")
	log.Println('\n', cmd.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.WithMessage(err, "failed to get crontab")
	}

	cmdName, args, err := packagemanager.DefinePHPCommandAndArgs(
		filepath.Join(state.Path, "artisan"), "schedule:run",
	)
	if err != nil {
		return errors.WithMessage(err, "failed to define php command and args")
	}

	cronCMDBuilder := strings.Builder{}
	cronCMDBuilder.WriteString("* * * * * ")
	cronCMDBuilder.WriteString(cmdName)
	for _, arg := range args {
		cronCMDBuilder.WriteString(" ")
		cronCMDBuilder.WriteString(arg)
	}
	cronCMDBuilder.WriteString(" >> /dev/null 2>&1\n")

	buf := bytes.NewBuffer(out)
	buf.WriteString(cronCMDBuilder.String())

	tmpDir, err := os.MkdirTemp("", "gameap_cron")
	if err != nil {
		return errors.WithMessage(err, "failed to create temp dir")
	}
	defer func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			log.Println(err)
		}
	}()

	err = os.WriteFile(tmpDir+"/crontab", buf.Bytes(), 0600)
	if err != nil {
		return errors.WithMessage(err, "failed to write crontab")
	}

	cmd = exec.Command("crontab", "crontab")
	cmd.Dir = tmpDir
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	log.Println('\n', cmd.String())
	err = cmd.Run()
	if err != nil {
		return errors.WithMessage(err, "failed to update crontab")
	}

	return nil
}

func updateAdminPassword(ctx context.Context, state panelInstallState) (panelInstallState, error) {
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

	err = panel.ChangePassword(ctx, state.Path, "admin", state.AdminPassword)
	if err != nil {
		return state, errors.WithMessage(err, "failed to change admin password")
	}

	return state, nil
}

func clearGameAPCache(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	err := panel.ClearCache(ctx, state.Path)
	if err != nil {
		return state, errors.WithMessage(err, "failed to clear cache")
	}

	return state, nil
}

func checkInstallation(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	var err error
	state, err = clearGameAPCache(ctx, state)
	if err != nil {
		return state, errors.WithMessage(err, "failed to clear panel cache")
	}

	hostPort := state.Host
	if state.Port != "80" && state.Port != "433" {
		hostPort = state.Host + ":" + state.Port
	}

	u := "http://" + hostPort + "/api/healthz"
	if state.HTTPS {
		u = "https://" + hostPort + "/api/healthz"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return state, err
	}
	//nolint:bodyclose
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return state, err
	}
	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to close response body"))
		}
	}(response.Body)

	if response.StatusCode != http.StatusOK {
		return state, errors.New("unsuccessful response from panel")
	}

	r := struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{}

	err = json.NewDecoder(response.Body).Decode(&r)
	if err != nil {
		return state, errors.WithMessage(err, "failed to decode response")
	}

	if r.Status != "ok" {
		return state, errors.New("unsuccessful response from panel")
	}

	return state, nil
}

//nolint:funlen
func tryToFixPanelInstallation(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	fmt.Println("Trying to fix panel installation ...")

	tried := map[int]struct{}{}
	isTried := func(step int) bool {
		_, ok := tried[step]

		return ok
	}

	var err error

	for {
		switch {
		case !isTried(0) &&
			state.WebServer == "nginx" && utils.IsFileExists("/etc/nginx/conf.d/default.conf"):
			tried[0] = struct{}{}

			log.Println("Disabling nginx default.conf config")

			err = os.Rename("/etc/nginx/conf.d/default.conf", "/etc/nginx/conf.d/default.conf.disabled")
			if err != nil {
				return state, errors.WithMessage(err, "failed to rename default nginx config")
			}
			err = service.Restart(ctx, "nginx")
			if err != nil {
				return state, errors.WithMessage(err, "failed to restart nginx")
			}
		case !isTried(1) &&
			state.WebServer == apacheWebServer:
			tried[1] = struct{}{}

			log.Println("Disabling apache 000-default site")

			err = utils.ExecCommand("a2dissite", "000-default")
			if err != nil {
				return state, errors.WithMessage(err, "failed to disable 000-default")
			}

			err = service.Restart(ctx, "apache2")
			if err != nil {
				return state, errors.WithMessage(err, "failed to restart apache")
			}
		case !isTried(2) && utils.IsFileExists(state.Path+"/.env") && state.DBCreds.Host == "localhost":
			tried[2] = struct{}{}

			log.Print("Replacing localhost to 127.0.0.1 in .env")

			state.DBCreds.Host = "127.0.0.1"
			state, err = updateDotEnv(ctx, state)
			if err != nil {
				return state, errors.WithMessage(err, "failed to update .env")
			}
		default:
			return state, errors.New("failed to fix panel installation")
		}

		state, err = checkInstallation(ctx, state)
		if err != nil {
			log.Println(err)
		} else {
			break
		}
	}

	return state, nil
}

func savePanelInstallationDetails(ctx context.Context, state panelInstallState) error {
	return gameapctl.SavePanelInstallState(ctx, gameapctl.PanelInstallState{
		Host:                 state.Host,
		HostIP:               state.HostIP,
		Port:                 state.Port,
		Path:                 state.Path,
		WebServer:            state.WebServer,
		Database:             state.Database,
		DatabaseWasInstalled: state.DatabaseWasInstalled,
		FromGithub:           state.FromGithub,
		Branch:               state.Branch,
	})
}
