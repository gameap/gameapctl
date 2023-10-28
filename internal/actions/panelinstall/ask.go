package panelinstall

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"runtime"

	"github.com/gameap/gameapctl/pkg/gameap"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

type askedParams struct {
	path      string
	host      string
	database  string
	webServer string
}

//nolint:funlen,gocognit
func askUser(ctx context.Context, state panelInstallState, needToAsk map[string]struct{}) (askedParams, error) {
	var err error
	result := askedParams{}

	//nolint:nestif
	if _, ok := needToAsk["path"]; ok {
		pathText := "Enter gameap installation path (Example: /var/www/gameap): "

		if runtime.GOOS == "windows" {
			pathText = "Enter gameap installation path (Example: C:\\gameap\\web): "
		}

		if result.path == "" {
			result.path, err = utils.Ask(
				ctx,
				pathText,
				true,
				func(s string) (bool, string, error) {
					if _, err := os.Stat(s); errors.Is(err, fs.ErrNotExist) {
						return true, "", nil
					}

					return false,
						fmt.Sprintf("Directory '%s' already exists. Please provide another path", s),
						nil
				},
			)
			if err != nil {
				return result, err
			}
		}

		if result.path == "" {
			result.path = gameap.DefaultWebInstallationPath
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
				func(s string) (bool, string, error) {
					if s != "1" && s != "2" && s != "3" {
						return false, "Please answer 1-3.", nil
					}

					return true, "", nil
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

			if state.OSInfo.Distribution != packagemanager.DistributionWindows {
				fmt.Println("2) Apache")
			}

			fmt.Println("3) None. Do not install a Web Server")

			for {
				num, err = utils.Ask(
					ctx,
					"Enter number: ",
					true,
					func(s string) (bool, string, error) {
						if s != "1" && s != "2" && s != "3" {
							return false, "Please answer 1-3.", nil
						}

						return true, "", nil
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

func warning(ctx context.Context, state panelInstallState, text string) error {
	fmt.Println()
	fmt.Println(text)

	if state.SkipWarnings {
		return nil
	}

	if state.NonInteractive {
		return errors.New("The installation cannot be continued. Please fix it or set the --skip-warnings flag")
	}

	_, err := utils.Ask(ctx, "Are you want to continue? (Y/n): ", false, func(s string) (bool, string, error) {
		if s == "y" || s == "Y" {
			return true, "", nil
		}
		if s == "n" || s == "N" {
			return true, "", errors.New("installation aborted by user")
		}

		return false, "Please answer y or n.", nil
	})

	return err
}
