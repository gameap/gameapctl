package actions

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"

	contextInternal "github.com/gameap/gameapctl/internal/context"
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

var errEmptyPath = errors.New("empty path")
var errEmptyHost = errors.New("empty host")
var errEmptyDatabase = errors.New("empty database")
var errEmptyWebServer = errors.New("empty web server")

type panelInstallState struct {
	NonInteractive bool
	HTTPS          bool
	Host           string
	HostIP         string
	Port           string
	Path           string
	AdminPassword  string
	WebServer      string
	Database       string
	DBCreds        databaseCredentials
	OSInfo         osinfo.Info

	// Installation variables
	DatabaseWasInstalled bool
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
func PanelInstall(cliCtx *cli.Context) error {
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

	fmt.Printf(
		"Detected operating system as %s/%s.\n",
		state.OSInfo.Distribution,
		state.OSInfo.DistributionCodename,
	)

	// nolint:nestif
	if !state.NonInteractive {
		needToAsk := make(map[string]struct{}, 4)
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
		answers, err := askUser(cliCtx.Context, needToAsk)
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

	fmt.Println()
	fmt.Println("Path:", state.Path)
	fmt.Println("Host:", state.Host)
	fmt.Println("Database:", state.Database)
	fmt.Println("Web server:", state.WebServer)
	fmt.Println("Develop:", cliCtx.Bool("develop"))
	fmt.Println()

	pm, err := packagemanager.Load(cliCtx.Context)
	if err != nil {
		return errors.WithMessage(err, "failed to load package manager")
	}

	fmt.Println("Checking for updates ...")
	if pm.CheckForUpdates(cliCtx.Context) != nil {
		return errors.WithMessage(err, "failed to check for updates")
	}

	fmt.Println("Checking for curl ...")
	isAvailable := utils.IsCommandAvailable("curl")
	if !isAvailable {
		fmt.Println("Installing curl ...")
		if pm.Install(cliCtx.Context, packagemanager.CurlPackage) != nil {
			return errors.WithMessage(err, "failed to install curl")
		}
	}

	isAvailable = utils.IsCommandAvailable("gpg")
	if !isAvailable {
		fmt.Println("Installing gpg ...")
		if pm.Install(cliCtx.Context, packagemanager.GnuPGPackage) != nil {
			return errors.WithMessage(err, "failed to install gpg")
		}
	}

	isAvailable = utils.IsCommandAvailable("php")
	if !isAvailable {
		fmt.Println("Installing php ...")
		if pm.Install(cliCtx.Context, packagemanager.PHPPackage) != nil {
			return errors.WithMessage(err, "failed to install php")
		}
	}

	err = installGameAP(cliCtx.Context, state.Path)
	if err != nil {
		return err
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

	err = generateEncryptionKey(state.Path)
	if err != nil {
		return errors.WithMessage(err, "failed to generate encryption key")
	}

	err = runMigration(state)
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

	fmt.Println("Updating files permissions ...")
	err = utils.ExecCommand("chown", "-R", "www-data:www-data", state.Path)
	if err != nil {
		return errors.WithMessage(err, "failed to change owner")
	}

	err = configureCron(cliCtx.Context, state)
	if err != nil {
		log.Println("Failed to configure cron: ", err)
		fmt.Println("Failed to configure cron: ", err.Error())
	}

	state, err = updateAdminPassword(state)
	if err != nil {
		return errors.WithMessage(err, "failed to update admin password")
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

	if err = savePanelInstallationDetails(state); err != nil {
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
	fmt.Println("Host: http://" + state.Host)
	fmt.Println()
	fmt.Println("---------------------------------")

	return nil
}

type askedParams struct {
	path      string
	host      string
	database  string
	webServer string
}

//nolint:funlen,gocognit
func askUser(ctx context.Context, needToAsk map[string]struct{}) (askedParams, error) {
	var err error
	result := askedParams{}

	//nolint:nestif
	if _, ok := needToAsk["path"]; ok {
		if result.path == "" {
			result.path, err = utils.Ask(
				ctx,
				"Enter gameap installation path (Example: /var/www/gameap): ",
				true,
				func(s string) (bool, string) {
					if _, err := os.Stat(s); errors.Is(err, fs.ErrNotExist) {
						return true, ""
					}

					return false, fmt.Sprintf("Directory '%s' already exists. Please provide another path", s)
				},
			)
			if err != nil {
				return result, err
			}
		}

		if result.path == "" {
			result.path = "/var/www/gameap"
		}
	}

	if _, ok := needToAsk["host"]; ok {
		result.host, err = utils.Ask(
			ctx,
			"Enter gameap host (Example: example.com): ",
			false,
			nil,
		)
		if err != nil {
			return result, err
		}
	}

	if _, ok := needToAsk["database"]; ok {
		fmt.Println("Select database to install and configure")
		fmt.Println("")
		fmt.Println("1) MySQL")
		fmt.Println("2) SQLite")
		fmt.Println("3) None. Do not install a database")

		for {
			num := ""
			num, err = utils.Ask(
				ctx,
				"Enter number: ",
				true,
				func(s string) (bool, string) {
					if s != "1" && s != "2" && s != "3" {
						return false, "Please answer 1-3."
					}

					return true, ""
				},
			)
			if err != nil {
				return result, err
			}

			switch num {
			case "1":
				result.database = mysqlDatabase
				fmt.Println("Okay! Will try install MySQL ...")
			case "2":
				result.database = sqliteDatabase
				fmt.Println("Okay! Will try install SQLite ...")
			case "3":
				result.database = noneDatabase
				fmt.Println("Okay!  ...")
			default:
				fmt.Println("Please answer 1-3.")
				continue
			}
			break
		}
	}

	//nolint:nestif
	if _, ok := needToAsk["webServer"]; ok {
		if result.webServer == "" {
			num := ""

			fmt.Println()
			fmt.Println("Select Web-server to install and configure")
			fmt.Println()
			fmt.Println("1) Nginx (Recommended)")
			fmt.Println("2) Apache")
			fmt.Println("3) None. Do not install a Web Server")

			for {
				num, err = utils.Ask(
					ctx,
					"Enter number: ",
					true,
					func(s string) (bool, string) {
						if s != "1" && s != "2" && s != "3" {
							return false, "Please answer 1-3."
						}

						return true, ""
					},
				)
				if err != nil {
					return result, err
				}

				switch num {
				case "1":
					result.webServer = nginxWebServer
					fmt.Println("Okay! Will try to install Nginx ...")
				case "2":
					result.webServer = apacheWebServer
					fmt.Println("Okay! Will try to install Apache ...")
				case "3":
					result.webServer = noneWebServer
					fmt.Println("Okay!  ...")
				default:
					fmt.Println("Please answer 1-3.")
					continue
				}
				break
			}
		}
	}

	return result, nil
}

func filterAndCheckHost(state panelInstallState) (panelInstallState, error) {
	if idx := strings.Index(state.Host, "http://"); idx >= 0 {
		state.Host = state.Host[7:]
	} else if idx = strings.Index(state.Host, "https://"); idx >= 0 {
		state.Host = state.Host[8:]
	}

	state.Host = strings.TrimRight(state.Host, "/?&")

	var invalidChars = []int32{'/', '?', '&'}
	for _, s := range state.Host {
		if utils.Contains(invalidChars, s) {
			return state, errors.New("invalid host")
		}
	}

	//nolint:nestif
	if utils.IsIPv4(state.Host) || utils.IsIPv6(state.Host) {
		state.HostIP = state.Host
	} else {
		ips, err := net.LookupIP(state.Host)
		if err != nil {
			return state, errors.WithMessage(err, "failed to lookup ip")
		}

		if len(ips) == 0 {
			return state, errors.New("no ip for chosen host")
		}

		for i := range ips {
			if utils.IsIPv4(ips[i].String()) {
				state.HostIP = ips[i].String()
			}
		}

		if state.HostIP == "" {
			state.HostIP = ips[0].String()
		}
	}

	return state, nil
}

//nolint:funlen,gocognit
func installMySQL(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state panelInstallState,
) (panelInstallState, error) {
	fmt.Println("Installing MySQL ...")

	var err error

	if state.DBCreds.Port == "" {
		state.DBCreds.Port = "3306"
	}

	var needToCreateDababaseAndUser bool

	//nolint:nestif
	if state.DBCreds.Host == "" {
		if isAvailable := utils.IsCommandAvailable("mysqld"); !isAvailable {
			needToCreateDababaseAndUser = true
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
				fmt.Println("Failed to install MySQL server. Trying to replace by MariaDB ...")
				log.Println(err)
				log.Println("Failed to install MySQL server. Trying to replace by MariaDB")

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

	if needToCreateDababaseAndUser {
		fmt.Println("Configuring MySQL ...")
		err = configureMysql(ctx, state.DBCreds)
		if err != nil {
			return state, err
		}
	}

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
	err = db.Ping()
	if err != nil {
		return state, errors.WithMessage(err, "failed to connect to MySQL")
	}

	_, err = db.Exec("SELECT 1")
	if err != nil {
		return state, errors.WithMessage(err, "failed to execute MySQL query")
	}

	return state, err
}

//nolint:funlen
func preconfigureMysql(_ context.Context, dbCreds databaseCredentials) (databaseCredentials, error) {
	if dbCreds.Username == "" {
		dbCreds.Username = "gameap"
	}

	if dbCreds.DatabaseName == "" {
		dbCreds.DatabaseName = "gameap"
	}

	if dbCreds.Host == "" {
		dbCreds.Host = "localhost"
	}

	passwordGenerator, err := password.NewGenerator(&password.GeneratorInput{
		Symbols: "_-+=",
	})
	if err != nil {
		return dbCreds, errors.WithMessage(err, "failed to create password generator")
	}

	if dbCreds.Password == "" {
		dbCreds.Password, err = passwordGenerator.Generate(16, 6, 2, false, false)
		if err != nil {
			return dbCreds, errors.WithMessage(err, "failed to generate password")
		}
	}

	if dbCreds.RootPassword == "" {
		dbCreds.RootPassword, err = passwordGenerator.Generate(16, 6, 2, false, false)
		if err != nil {
			return dbCreds, errors.WithMessage(err, "failed to generate password")
		}
	}

	return dbCreds, nil
}

//nolint:funlen
func configureMysql(_ context.Context, dbCreds databaseCredentials) error {
	dsns := []string{
		"root@unix(/var/run/mysqld/mysqld.sock)/mysql",
		fmt.Sprintf(
			"root:%s@tcp(%s:%s)/%s",
			dbCreds.Password,
			dbCreds.Host,
			dbCreds.Port,
			dbCreds.DatabaseName,
		),
	}

	var err error
	var db *sql.DB
	for _, dsn := range dsns {
		db, err = sql.Open("mysql", dsn)
		if err != nil {
			continue
		}
		defer func(db *sql.DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)
		err = db.Ping()
		if err == nil {
			break
		}
	}

	if err != nil {
		return errors.WithMessage(err, "failed to connect to MySQL")
	}

	fmt.Println("Creating database ...")
	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS " + dbCreds.DatabaseName)
	if err != nil {
		return errors.WithMessage(err, "failed to create database")
	}

	fmt.Println("Creating user ...")
	_, err = db.Exec("CREATE USER IF NOT EXISTS " + dbCreds.Username + "@'%' IDENTIFIED BY '" + dbCreds.Password + "'")
	if err != nil {
		return errors.WithMessage(err, "failed to create user")
	}

	fmt.Println("Granting privileges ...")
	//nolint:gosec
	_, err = db.Exec("GRANT SELECT ON *.* TO '" + dbCreds.Username + "'@'%'")
	if err != nil {
		return errors.WithMessage(err, "failed to grant select privileges")
	}
	//nolint:gosec
	_, err = db.Exec("GRANT ALL PRIVILEGES ON " + dbCreds.DatabaseName + ".* TO '" + dbCreds.Username + "'@'%'")
	if err != nil {
		return errors.WithMessage(err, "failed to grant all privileges")
	}
	_, err = db.Exec("FLUSH PRIVILEGES")
	if err != nil {
		return errors.WithMessage(err, "failed to flush privileges")
	}

	return nil
}

func installSqlite(_ context.Context, state panelInstallState) (panelInstallState, error) {
	dbPath := state.Path + string(os.PathSeparator) + "database.sqlite"
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

func installGameAP(ctx context.Context, path string) error {
	tempDir, err := os.MkdirTemp("", "gameap")
	if err != nil {
		return errors.WithMessage(err, "failed to create temp dir")
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			log.Println(err)
		}
	}(tempDir)

	fmt.Println("Downloading GameAP ...")
	err = utils.Download(ctx, "http://packages.gameap.ru/gameap/latest.tar.gz", tempDir)
	if err != nil {
		return errors.WithMessage(err, "failed to download gameap")
	}

	err = utils.Move(tempDir+string(os.PathSeparator)+"gameap", path)
	if err != nil {
		return errors.WithMessage(err, "failed to move gameap")
	}

	fmt.Println("Installing GameAP ...")
	err = utils.Copy(path+string(os.PathSeparator)+".env.example", path+string(os.PathSeparator)+".env")
	if err != nil {
		return errors.WithMessage(err, "failed to copy .env.example")
	}

	return nil
}

func generateEncryptionKey(dir string) error {
	fmt.Println("Generating encryption key ...")
	cmd := exec.Command("php", "artisan", "key:generate", "--force")
	cmd.Dir = dir
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	err := cmd.Run()
	log.Println('\n', cmd.String())
	if err != nil {
		return errors.WithMessage(err, "failed to execute key generate command")
	}

	return nil
}

func updateDotEnv(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	var err error

	fmt.Println("Updating .env ...")

	url := "http://" + state.Host
	if state.HTTPS {
		url = "https://" + state.Host
	}

	err = utils.FindLineAndReplace(ctx, state.Path+"/.env", map[string]string{
		"APP_URL=":       "APP_URL=" + url,
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

func runMigration(state panelInstallState) error {
	fmt.Println("Running migration ...")
	var cmd *exec.Cmd
	if state.DatabaseWasInstalled {
		cmd = exec.Command("php", "artisan", "migrate", "--seed")
	} else {
		cmd = exec.Command("php", "artisan", "migrate")
	}

	cmd.Dir = state.Path
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	log.Println('\n', cmd.String())
	err := cmd.Run()
	if err != nil {
		return errors.WithMessage(err, "failed to execute key generate command")
	}

	return nil
}

func installNginx(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state panelInstallState,
) (panelInstallState, error) {
	err := pm.Install(ctx, packagemanager.NginxPackage)
	if err != nil {
		return state, errors.WithMessage(err, "failed to install nginx")
	}

	err = utils.DownloadFile(
		ctx,
		"https://raw.githubusercontent.com/gameap/auto-install-scripts/master/web-server-configs/nginx-no-ssl.conf",
		"/etc/nginx/conf.d/gameap.conf",
	)
	if err != nil {
		return state, errors.WithMessage(err, "failed to download nginx config")
	}

	phpVersion, err := packagemanager.DefinePHPVersion()
	if err != nil {
		return state, errors.WithMessage(err, "failed to define php version")
	}

	fpmUnixSocket := fmt.Sprintf("unix:/var/run/php/php%s-fpm.sock", phpVersion)

	err = utils.FindLineAndReplace(ctx, "/etc/nginx/conf.d/gameap.conf", map[string]string{
		"server_name":                  fmt.Sprintf("server_name       %s;", state.Host),
		"listen":                       fmt.Sprintf("listen       %s;", state.Port),
		"root /var/www/gameap/public;": fmt.Sprintf("root       %s%c%s;", state.Path, os.PathSeparator, "public"),
		"fastcgi_pass    unix":         fmt.Sprintf("fastcgi_pass %s;", fpmUnixSocket),
	})
	if err != nil {
		return state, errors.WithMessage(err, "failed to update nginx host config")
	}

	err = utils.FindLineAndReplace(ctx, "/etc/nginx/nginx.conf", map[string]string{
		"user": "user www-data;",
	})
	if err != nil {
		return state, errors.WithMessage(err, "failed to update nginx config")
	}

	err = service.Start(ctx, "nginx")
	if err != nil {
		return state, errors.WithMessage(err, "failed to start nginx")
	}
	err = service.Start(ctx, "php"+phpVersion+"-fpm")
	if err != nil {
		return state, errors.WithMessage(err, "failed to start php-fpm")
	}

	return state, nil
}

func installApache(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state panelInstallState,
) (panelInstallState, error) {
	err := pm.Install(ctx, packagemanager.ApachePackage)
	if err != nil {
		return state, errors.WithMessage(err, "failed to install apache")
	}

	err = utils.DownloadFile(
		ctx,
		"https://raw.githubusercontent.com/gameap/auto-install-scripts/master/web-server-configs/apache-no-ssl.conf",
		"/etc/apache2/sites-available/gameap.conf",
	)
	if err != nil {
		return state, errors.WithMessage(err, "failed to download apache config")
	}

	err = utils.FindLineAndReplace(ctx, "/etc/apache2/sites-available/gameap.conf", map[string]string{
		"ServerName":                         fmt.Sprintf("ServerName %s", state.Host),
		"DocumentRoot":                       fmt.Sprintf("DocumentRoot %s/public", state.Path),
		"<VirtualHost":                       fmt.Sprintf("<VirtualHost *:%s>", state.Port),
		"<Directory /var/www/gameap/public>": fmt.Sprintf("<Directory %s/public>", state.Path),
	})
	if err != nil {
		return state, errors.WithMessage(err, "failed to update apache config")
	}

	if state.Port != "80" {
		err = utils.FindLineAndReplace(ctx, "/etc/apache2/sites-available/gameap.conf", map[string]string{
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

func configureCron(_ context.Context, state panelInstallState) error {
	fmt.Println("Configuring cron ...")

	if isAvailable := utils.IsCommandAvailable("crontab"); !isAvailable {
		fmt.Println("Crontab is not available. Skip cron configuration")
		return nil
	}

	cmd := exec.Command("crontab", "-l")
	log.Println('\n', cmd.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.WithMessage(err, "failed to get crontab")
	}

	buf := bytes.NewBuffer(out)
	buf.Write([]byte(fmt.Sprintf("* * * * * cd %s && php artisan schedule:run >> /dev/null 2>&1\n", state.Path)))

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

func updateAdminPassword(state panelInstallState) (panelInstallState, error) {
	var err error
	if state.AdminPassword == "" {
		fmt.Println("Generating admin password ...")

		state.AdminPassword, err = password.Generate(18, 6, 0, false, false)
		if err != nil {
			return state, errors.WithMessage(err, "failed to generate password")
		}
	}

	//nolint:gosec
	cmd := exec.Command("php", "artisan", "user:change-password", "admin", state.AdminPassword)
	log.Println('\n', "php artisan user:change-password admin ********")
	cmd.Dir = state.Path
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()

	err = cmd.Run()
	if err != nil {
		return state, errors.WithMessage(err, "failed to execute artisan command")
	}

	return state, nil
}

func checkInstallation(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	url := "http://" + state.Host + "/api/healthz"
	if state.HTTPS {
		url = "https://" + state.Host + "/api/healthz"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
			continue
		} else {
			break
		}
	}

	return state, nil
}

func savePanelInstallationDetails(state panelInstallState) error {
	saveStruct := struct {
		Host                 string `json:"host"`
		HostIP               string `json:"hostIP"`
		Port                 string `json:"port"`
		Path                 string `json:"path"`
		WebServer            string `json:"webServer"`
		Database             string `json:"database"`
		DatabaseWasInstalled bool   `json:"databaseWasInstalled"`
	}{
		Host:                 state.Host,
		HostIP:               state.HostIP,
		Port:                 state.Port,
		Path:                 state.Path,
		WebServer:            state.WebServer,
		Database:             state.Database,
		DatabaseWasInstalled: state.DatabaseWasInstalled,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.WithMessage(err, "failed to get user home dir")
	}

	saveFilePath := homeDir + string(os.PathSeparator) + ".gameapctl"
	if _, err := os.Stat(saveFilePath); errors.Is(err, fs.ErrNotExist) {
		err = os.Mkdir(saveFilePath, 0600)
		if err != nil {
			return err
		}
	}

	b, err := json.Marshal(saveStruct)
	if err != nil {
		return errors.WithMessage(err, "failed to marshal json")
	}

	err = os.WriteFile(
		homeDir+string(os.PathSeparator)+".gameapctl"+string(os.PathSeparator)+"panel_install.json",
		b,
		0600,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to write file")
	}

	return nil
}
