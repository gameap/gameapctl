package actions

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"

	"github.com/urfave/cli/v2"
)

var errEmptyPath = errors.New("empty path")
var errEmptyHost = errors.New("empty host")
var errEmptyDatabase = errors.New("empty database")
var errEmptyWebServer = errors.New("empty web server")

//nolint:funlen,gocognit
func PanelInstall(cliCtx *cli.Context) error {
	nonInteractive := cliCtx.Bool("non-interactive")

	host := cliCtx.String("host")
	path := cliCtx.String("path")
	database := cliCtx.String("database")
	webServer := cliCtx.String("web-server")

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
	_, err = pm.CheckForUpdates(cliCtx.Context)
	if err != nil {
		return errors.WithMessage(err, "failed to check for updates")
	}

	fmt.Println("Checking for curl...")
	isAvailable := utils.IsCommandAvailable("curl")
	if !isAvailable {
		fmt.Println("Installing curl...")
		_, err = pm.Install(cliCtx.Context, packagemanager.CurlPackage)
		if err != nil {
			return errors.WithMessage(err, "failed to install curl")
		}
	}

	isAvailable = utils.IsCommandAvailable("gpg")
	if !isAvailable {
		fmt.Println("Installing gpg...")
		_, err = pm.Install(cliCtx.Context, packagemanager.GnuPGPackage)
		if err != nil {
			return errors.WithMessage(err, "failed to install gpg")
		}
	}

	isAvailable = utils.IsCommandAvailable("php")
	if !isAvailable {
		fmt.Println("Installing php...")
		_, err = pm.Install(cliCtx.Context, packagemanager.PHPPackage)
		if err != nil {
			return errors.WithMessage(err, "failed to install php")
		}
	}

	tempDir, err := os.MkdirTemp("", "gameap")
	if err != nil {
		return errors.WithMessage(err, "failed to create temp dir")
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tempDir)

	err = utils.Download(cliCtx.Context, "http://packages.gameap.ru/gameap/latest", tempDir)
	if err != nil {
		return errors.WithMessage(err, "failed to download gameap")
	}

	err = utils.Move(tempDir+string(os.PathSeparator)+"gameap", path)
	if err != nil {
		return errors.WithMessage(err, "failed to move gameap")
	}

	err = utils.Copy(path+string(os.PathSeparator)+".env.example", path+string(os.PathSeparator)+".env")
	if err != nil {
		return errors.WithMessage(err, "failed to copy .env.example")
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

	reader := bufio.NewReader(os.Stdin)

	if _, ok := needToAsk["path"]; ok {
		if result.path == "" {
			fmt.Println("")
			fmt.Print("Enter gameap installation path (Example: /var/www/gameap): ")
			result.path, err = reader.ReadString('\n')
			if err != nil {
				return result, errors.WithMessage(err, "failed to read path")
			}
			result.path = strings.TrimSpace(result.path)
		}

		if result.path == "" {
			result.path = "/var/www/gameap"
		}
	}

	if _, ok := needToAsk["host"]; ok {
		for result.host == "" {
			fmt.Println("")
			fmt.Print("Enter gameap host (example.com): ")
			result.host, err = reader.ReadString('\n')
			if err != nil {
				return result, err
			}
			result.host = strings.TrimSpace(result.host)
		}
	}

	if _, ok := needToAsk["database"]; ok {
		fmt.Println("Select database to install and configure")
		fmt.Println("")
		fmt.Println("1) MySQL")
		fmt.Println("2) SQLite")
		fmt.Println("3) None. Do not install a database")
		fmt.Println("")

		for {
			num := ""
			fmt.Print("Enter number: ")
			num, err = reader.ReadString('\n')
			if err != nil {
				fmt.Println()
				fmt.Println("Please answer 1-3.")
				continue
			}

			num = strings.TrimSpace(num)

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

	if _, ok := needToAsk["webServer"]; ok {
		if result.webServer == "" {
			num := ""

			fmt.Println()
			fmt.Println("Select Web-server to install and configure")
			fmt.Println()
			fmt.Println("1) Nginx (Recommended)")
			fmt.Println("2) Apache")
			fmt.Println("3) None. Do not install a Web Server")
			fmt.Println()
			for {
				fmt.Print("Enter number: ")
				num, err = reader.ReadString('\n')
				if err != nil {
					fmt.Println()
					fmt.Println("Please answer 1-3.")
					continue
				}

				num = strings.TrimSpace(num)

				switch num {
				case "1":
					result.webServer = "nginx"
					fmt.Println("Okay! Will try to install Nginx...")
				case "2":
					result.webServer = "apache"
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
