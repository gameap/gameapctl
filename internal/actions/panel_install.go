package actions

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	contextInternal "github.com/gameap/gameapctl/internal/context"
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

type databaseCredentials struct {
	Host     string
	Port     string
	Name     string
	Username string
	Password string
}

//nolint:funlen,gocognit
func PanelInstall(cliCtx *cli.Context) error {
	osInfo := contextInternal.OSInfoFromContext(cliCtx.Context)
	fmt.Printf("Detected operating system as %s/%s.\n", osInfo.Distribution, osInfo.DistributionCodename)

	nonInteractive := cliCtx.Bool("non-interactive")

	host := cliCtx.String("host")
	path := cliCtx.String("path")
	webServer := cliCtx.String("web-server")
	database := cliCtx.String("database")
	dbCreds := databaseCredentials{
		Host:     cliCtx.String("database-host"),
		Port:     cliCtx.String("database-port"),
		Name:     cliCtx.String("database-name"),
		Username: cliCtx.String("database-username"),
		Password: cliCtx.String("database-password"),
	}

	// nolint:nestif
	if !nonInteractive {
		needToAsk := make(map[string]struct{}, 4)
		if host == "" {
			needToAsk["host"] = struct{}{}
		}
		if path == "" {
			needToAsk["path"] = struct{}{}
		}
		if database == "" {
			needToAsk["database"] = struct{}{}
		}
		if webServer == "" {
			needToAsk["webServer"] = struct{}{}
		}
		answers, err := askUser(needToAsk)
		if err != nil {
			return err
		}

		if _, ok := needToAsk["path"]; ok {
			path = answers.path
		}

		if _, ok := needToAsk["host"]; ok {
			host = answers.host
		}

		if _, ok := needToAsk["database"]; ok {
			database = answers.database
		}

		if _, ok := needToAsk["webServer"]; ok {
			webServer = answers.webServer
		}
	}

	if path == "" {
		return errEmptyPath
	}

	if host == "" {
		return errEmptyHost
	}

	if database == "" {
		return errEmptyDatabase
	}

	if webServer == "" {
		return errEmptyWebServer
	}

	fmt.Println()
	fmt.Println("Path:", path)
	fmt.Println("Host:", host)
	fmt.Println("Database:", database)
	fmt.Println("Web server:", webServer)
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

	if database == "mysql" {
		err = installMySQL(cliCtx.Context, pm, dbCreds, nonInteractive)
	}
	if err != nil {
		return err
	}

	err = installGameAP(cliCtx.Context, path)
	if err != nil {
		return err
	}

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

func installMySQL(ctx context.Context, pm packagemanager.PackageManager, dbCreds databaseCredentials, _ bool) error {
	fmt.Println("Installing MySQL...")

	var err error

	//nolint:nestif
	if dbCreds.Host == "" {
		if isAvailable := utils.IsCommandAvailable("mysqld"); !isAvailable {
			if err := pm.Install(ctx, packagemanager.MySQLServerPackage); err != nil {
				fmt.Println("Failed to install MySQL server. Trying to replace by MariaDB...")
				log.Println(err)
				log.Println("Failed to install MySQL server. Trying to replace by MariaDB")

				fmt.Println("Removing MySQL server...")
				err = pm.Purge(ctx, packagemanager.MySQLServerPackage)
				if err != nil {
					return errors.WithMessage(err, "failed to remove MySQL server")
				}

				fmt.Println("Installing MariaDB server...")
				err = pm.Install(ctx, packagemanager.MariaDBServerPackage)
				if err != nil {
					return errors.WithMessage(err, "failed to install MariaDB server")
				}
			}

			if dbCreds.Password == "" {
				dbCreds.Password, err = password.Generate(16, 8, 8, false, false)
				if err != nil {
					return errors.WithMessage(err, "failed to generate password")
				}
			}

			if dbCreds.Username == "" {
				dbCreds.Username = "gameap"
			}

			if dbCreds.Name == "" {
				dbCreds.Name = "gameap"
			}
		} else {
			fmt.Println("MySQL already installed")
		}
	}

	_, err = sql.Open(
		"mysql",
		fmt.Sprintf("%s:%s@/%s", dbCreds.Username, dbCreds.Password, dbCreds.Name),
	)

	return err
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
