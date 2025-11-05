package install

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/gameap/gameapctl/pkg/gameap"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

type askedParamsV3 struct {
	path      string
	host      string
	database  string
	webServer string
}

//nolint:gocognit,gocyclo,funlen
func askUserV3(ctx context.Context, state panelInstallStateV3, needToAsk map[string]struct{}) (askedParamsV3, error) {
	var err error
	result := askedParamsV3{}

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
				false,
				func(s string) (bool, string, error) {
					if s == "" {
						s = gameap.DefaultWebInstallationPath
					}
					if utils.IsFileExists(s) {
						return false,
							fmt.Sprintf("Directory '%s' already exists. Please provide another path", s),
							nil
					}

					return true, "", nil
				},
			)

			if result.path == "" {
				result.path = gameap.DefaultWebInstallationPath
			}

			if err != nil {
				return result, err
			}
		}
	}

	//nolint:nestif
	if _, ok := needToAsk["host"]; ok {
		exampleHost, err := os.Hostname()
		if err != nil {
			return result, err
		}

		if !strings.Contains(exampleHost, ".") {
			exampleHost, err = chooseIPFromHost(exampleHost)
			if err != nil {
				return result, err
			}
		}

		result.host, err = utils.Ask(
			ctx,
			fmt.Sprintf("Enter gameap host (Examples: %s, example.com): ", exampleHost),
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

func warning(ctx context.Context, state panelInstallStateV3, text string) error {
	fmt.Println()
	fmt.Println(text)

	if state.SkipWarnings {
		return nil
	}

	if state.NonInteractive {
		log.Println(text)

		return errors.New("The installation cannot be continued. Please fix it or set the --skip-warnings flag")
	}

	_, err := utils.Ask(ctx, "Are you want to continue? (Y/n): ", false, func(s string) (bool, string, error) {
		if s == "y" || s == "Y" {
			return true, "", nil
		}
		if s == "n" || s == "N" {
			return true, "", errors.New("installation aborted by user")
		}

		//nolint:goconst
		return false, "Please answer y or n.", nil
	})

	return err
}

func warningAskForAction(
	ctx context.Context,
	state panelInstallStateV3,
	text string,
	actionText string,
	action func(context.Context) error,
) error {
	fmt.Println()
	fmt.Println(text)

	if state.SkipWarnings || state.NonInteractive {
		return nil
	}

	_, err := utils.Ask(ctx, actionText, false, func(s string) (bool, string, error) {
		if s == "y" || s == "Y" {
			err := action(ctx)
			if err != nil {
				return false, err.Error(), err
			}

			return true, "", nil
		}
		if s == "n" || s == "N" {
			return true, "", nil
		}

		return false, "Please answer y or n.", nil
	})

	return err
}
