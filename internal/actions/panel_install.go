package actions

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"

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
)

const (
	nginxWebServer  = "nginx"
	apacheWebServer = "apache"
)

var errEmptyPath = errors.New("empty path")
var errEmptyHost = errors.New("empty host")
var errEmptyDatabase = errors.New("empty database")
var errEmptyWebServer = errors.New("empty web server")

type panelInstallState struct {
	NonInteractive bool
	Host           string
	Port           string
	Path           string
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
		answers, err := askUser(needToAsk)
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

	fmt.Println("Checking for updates...")
	if pm.CheckForUpdates(cliCtx.Context) != nil {
		return errors.WithMessage(err, "failed to check for updates")
	}

	fmt.Println("Checking for curl...")
	isAvailable := utils.IsCommandAvailable("curl")
	if !isAvailable {
		fmt.Println("Installing curl...")
		if pm.Install(cliCtx.Context, packagemanager.CurlPackage) != nil {
			return errors.WithMessage(err, "failed to install curl")
		}
	}

	isAvailable = utils.IsCommandAvailable("gpg")
	if !isAvailable {
		fmt.Println("Installing gpg...")
		if pm.Install(cliCtx.Context, packagemanager.GnuPGPackage) != nil {
			return errors.WithMessage(err, "failed to install gpg")
		}
	}

	isAvailable = utils.IsCommandAvailable("php")
	if !isAvailable {
		fmt.Println("Installing php...")
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

		fmt.Println("Updating .env")
		err = utils.FindLineAndReplace(cliCtx.Context, state.Path+"/.env", map[string]string{
			"DB_CONNECTION=": "DB_CONNECTION=mysql",
			"DB_HOST=":       "DB_HOST=" + state.DBCreds.Host,
			"DB_PORT=":       "DB_PORT=" + state.DBCreds.Port,
			"DB_DATABASE=":   "DB_DATABASE=" + state.DBCreds.DatabaseName,
			"DB_USERNAME=":   "DB_USERNAME=" + state.DBCreds.Username,
			"DB_PASSWORD=":   "DB_PASSWORD=" + state.DBCreds.Password,
		})
		if err != nil {
			return errors.WithMessage(err, "failed to update .env file")
		}
	case sqliteDatabase:
		state, err = installSqlite(cliCtx.Context, state)
		if err != nil {
			return err
		}

		fmt.Println("Updating .env")
		err = utils.FindLineAndReplace(cliCtx.Context, state.Path+"/.env", map[string]string{
			"DB_CONNECTION=": "DB_CONNECTION=sqlite",
			"DB_DATABASE=":   "DB_DATABASE=database.sqlite",
		})
		if err != nil {
			return errors.WithMessage(err, "failed to update .env file")
		}
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
		state, err = installNginx(cliCtx.Context, pm, state)
		if err != nil {
			return errors.WithMessage(err, "failed to install nginx")
		}
	case apacheWebServer:
		state, err = installApache(cliCtx.Context, pm, state)
		if err != nil {
			return errors.WithMessage(err, "failed to install apache")
		}
	}

	fmt.Println("Updating files permissions...")
	err = utils.ExecCommand("chown -R www-data:www-data " + state.Path)
	if err != nil {
		return errors.WithMessage(err, "failed to change owner")
	}

	err = configureCron(cliCtx.Context, state)
	if err != nil {
		log.Println("Failed to configure cron: ", err)
		fmt.Println("Failed to configure cron: ", err.Error())
	}

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
	// fmt.Println("Password: admin")
	fmt.Println()
	fmt.Println("Host: http://", state.Host)
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
func askUser(needToAsk map[string]struct{}) (askedParams, error) {
	var err error
	result := askedParams{}

	if _, ok := needToAsk["path"]; ok {
		if result.path == "" {
			result.path, err = utils.Ask(
				"Enter gameap installation path (Example: /var/www/gameap): ",
				true,
				nil,
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
				fmt.Println("Okay! Will try install MySQL...")
			case "2":
				result.database = sqliteDatabase
				fmt.Println("Okay! Will try install SQLite...")
			case "3":
				result.database = "none"
				fmt.Println("Okay! ...")
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
					result.webServer = "nginx"
					fmt.Println("Okay! Will try to install Nginx...")
				case "2":
					result.webServer = "apache"
					fmt.Println("Okay! Will try to install Apache...")
				case "3":
					result.webServer = "none"
					fmt.Println("Okay! ...")
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

//nolint:funlen,gocognit
func installMySQL(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state panelInstallState,
) (panelInstallState, error) {
	fmt.Println("Installing MySQL...")

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

			if err := pm.Install(ctx, packagemanager.MySQLServerPackage); err != nil {
				fmt.Println("Failed to install MySQL server. Trying to replace by MariaDB...")
				log.Println(err)
				log.Println("Failed to install MySQL server. Trying to replace by MariaDB")

				fmt.Println("Removing MySQL server...")
				err = pm.Purge(ctx, packagemanager.MySQLServerPackage)
				if err != nil {
					return state, errors.WithMessage(err, "failed to remove MySQL server")
				}

				fmt.Println("Installing MariaDB server...")
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

	fmt.Println("Starting MySQL server...")
	if err = service.Start(ctx, "mysql"); err != nil {
		if err = service.Start(ctx, "mysqld"); err != nil {
			if err = service.Start(ctx, "mariadb"); err != nil {
				return state, errors.WithMessage(err, "failed to start MySQL server")
			}
		}
	}

	if needToCreateDababaseAndUser {
		fmt.Println("Configuring MySQL...")
		err = configureMysql(ctx, state.DBCreds)
		if err != nil {
			return state, err
		}
	}

	fmt.Println("Checking MySQL connection...")
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

	fmt.Println("Creating database...")
	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS " + dbCreds.DatabaseName)
	if err != nil {
		return errors.WithMessage(err, "failed to create database")
	}

	fmt.Println("Creating user...")
	_, err = db.Exec("CREATE USER IF NOT EXISTS " + dbCreds.Username + "@'%' IDENTIFIED BY '" + dbCreds.Password + "'")
	if err != nil {
		return errors.WithMessage(err, "failed to create user")
	}

	fmt.Println("Granting privileges...")
	//nolint:gosec
	_, err = db.Exec("GRANT SELECT ON *.* TO '" + dbCreds.Username + "'@'%'")
	if err != nil {
		return errors.WithMessage(err, "failed to grant select privileges")
	}
	//nolint:gosec
	_, err = db.Exec("GRANT ALL PRIVILEGES ON " + dbCreds.DatabaseName + ".* TO " + dbCreds.Username + "@'%'")
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
	f, err := os.Create(state.Path + "/database.sqlite")
	if err != nil {
		return state, errors.WithMessage(err, "failed to database.sqlite")
	}
	err = f.Close()
	if err != nil {
		return state, errors.WithMessage(err, "failed to close database.sqlite")
	}

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

	fmt.Println("Downloading GameAP...")
	err = utils.Download(ctx, "http://packages.gameap.ru/gameap/latest.tar.gz", tempDir)
	if err != nil {
		return errors.WithMessage(err, "failed to download gameap")
	}

	err = utils.Move(tempDir+string(os.PathSeparator)+"gameap", path)
	if err != nil {
		return errors.WithMessage(err, "failed to move gameap")
	}

	fmt.Println("Installing GameAP...")
	err = utils.Copy(path+string(os.PathSeparator)+".env.example", path+string(os.PathSeparator)+".env")
	if err != nil {
		return errors.WithMessage(err, "failed to copy .env.example")
	}

	return nil
}

func generateEncryptionKey(dir string) error {
	fmt.Println("Generating encryption key...")
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

func runMigration(state panelInstallState) error {
	fmt.Println("Running migration...")
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

	err = utils.FindLineAndReplace(ctx, state.Path+"/.env", map[string]string{
		"server_name":                  fmt.Sprintf("server_name       %s;", state.Host),
		"listen":                       fmt.Sprintf("listen       %s;", state.Port),
		"root /var/www/gameap/public;": fmt.Sprintf("root       %s/public;", state.Path),
		"fastcgi_pass    unix":         fmt.Sprintf("fastcgi_pass %s;", fpmUnixSocket),
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
		err = utils.FindLineAndReplace(ctx, "/etc/apache2/ports.conf", map[string]string{
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

	err = service.Start(ctx, "apache2")
	if err != nil {
		return state, errors.WithMessage(err, "failed to start apache")
	}

	return state, nil
}

func configureCron(_ context.Context, state panelInstallState) error {
	fmt.Println("Configuring cron...")

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
