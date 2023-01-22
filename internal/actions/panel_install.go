package actions

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/sethvargo/go-password/password"
	"github.com/urfave/cli/v2"
)

var errEmptyPath = errors.New("empty path")
var errEmptyHost = errors.New("empty host")
var errEmptyDatabase = errors.New("empty database")
var errEmptyWebServer = errors.New("empty web server")

type panelInstallState struct {
	NonInteractive bool
	Host           string
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

//nolint:funlen,gocognit
func PanelInstall(cliCtx *cli.Context) error {
	state := panelInstallState{}

	state.NonInteractive = cliCtx.Bool("non-interactive")
	state.Host = cliCtx.String("host")
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

	if state.Database == "mysql" {
		state, err = installMySQL(cliCtx.Context, pm, state)
		if err != nil {
			return err
		}

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
	}

	err = generateEncryptionKey(state.Path)
	if err != nil {
		return errors.WithMessage(err, "failed to generate encryption key")
	}

	err = runMigration(state)
	if err != nil {
		return errors.WithMessage(err, "failed to run migration")
	}

	fmt.Println("---------------------------------")
	fmt.Println("DONE!")
	fmt.Println()
	fmt.Println("GameAP file path:", state.Path)
	fmt.Println()
	fmt.Println("Database name:", state.DBCreds.DatabaseName)
	if state.DBCreds.RootPassword != "" {
		fmt.Println("Database root password:", state.DBCreds.RootPassword)
	}
	fmt.Println("Database user name:", state.DBCreds.Username)
	fmt.Println("Database user password:", state.DBCreds.Password)
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
				result.database = "mysql"
				fmt.Println("Okay! Will try install MySQL...")
			case "2":
				result.database = "sqlite"
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

//nolint:funlen
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

	if needToCreateDababaseAndUser {
		err = configureMysql(ctx, state.DBCreds)
		if err != nil {
			return state, err
		}
	}

	db, err := sql.Open(
		"mysql",
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
		return state, errors.WithMessage(err, "failed to connect to MySQL")
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

	return state, err
}

func preconfigureMysql(ctx context.Context, dbCreds databaseCredentials) (databaseCredentials, error) {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	var err error

	if dbCreds.Username == "" {
		dbCreds.Username = "gameap"
	}

	if dbCreds.DatabaseName == "" {
		dbCreds.DatabaseName = "gameap"
	}

	if dbCreds.Host == "" {
		dbCreds.Host = "localhost"
	}

	if dbCreds.Password == "" {
		dbCreds.Password, err = password.Generate(16, 8, 8, false, false)
		if err != nil {
			return dbCreds, errors.WithMessage(err, "failed to generate password")
		}
	}

	if dbCreds.RootPassword == "" {
		dbCreds.RootPassword, err = password.Generate(16, 8, 8, false, false)
		if err != nil {
			return dbCreds, errors.WithMessage(err, "failed to generate password")
		}
	}

	if osInfo.Distribution == "debian" || osInfo.Distribution == "ubuntu" {
		err := utils.ExecCommand(
			"sh",
			"-c",
			fmt.Sprintf(`echo debconf mysql-server/root_password password %s | debconf-set-selections`, dbCreds.RootPassword),
		)
		if err != nil {
			return dbCreds, errors.WithMessage(err, "failed to set debconf")
		}

		err = utils.ExecCommand(
			"sh",
			"-c",
			fmt.Sprintf(
				`echo debconf mysql-server/root_password_again password %s" | debconf-set-selections`,
				dbCreds.RootPassword,
			),
		)
		if err != nil {
			return dbCreds, errors.WithMessage(err, "failed to set debconf")
		}
	}

	return dbCreds, nil
}

func configureMysql(_ context.Context, dbCreds databaseCredentials) error {
	db, err := sql.Open(
		"mysql",
		fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s",
			"root",
			dbCreds.RootPassword,
			dbCreds.Host,
			dbCreds.Port,
			"mysql",
		),
	)
	if err != nil {
		return errors.WithMessage(err, "failed to connect to MySQL")
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Println(err)
		}
	}(db)

	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS ?", dbCreds.DatabaseName)
	if err != nil {
		return err
	}

	_, err = db.Exec(
		"CREATE USER IF NOT EXISTS ?@'%' IDENTIFIED BY ?",
		dbCreds.Username,
		dbCreds.Password,
	)
	if err != nil {
		return err
	}
	_, err = db.Exec("GRANT SELECT ON *.* TO ?@'%'", dbCreds.Username)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		"GRANT ALL PRIVILEGES ON ?.* TO ?@'%'",
		dbCreds.DatabaseName,
		dbCreds.Username,
	)
	if err != nil {
		return err
	}
	_, err = db.Exec("FLUSH PRIVILEGES")
	if err != nil {
		return err
	}

	return nil
}

func installGameAP(ctx context.Context, path string) error {
	tempDir, err := os.MkdirTemp("", "gameap")
	if err != nil {
		return errors.WithMessage(err, "failed to create temp dir")
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return errors.WithMessage(err, "failed to execute key generate command")
	}

	return nil
}

func runMigration(state panelInstallState) error {
	fmt.Println("Generating encryption key...")
	var cmd *exec.Cmd
	if state.DatabaseWasInstalled {
		cmd = exec.Command("php", "artisan", "migrate", "--seed")
	} else {
		cmd = exec.Command("php", "artisan", "migrate")
	}

	cmd.Dir = state.Path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return errors.WithMessage(err, "failed to execute key generate command")
	}

	return nil
}
