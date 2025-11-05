package install

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

type askedParamsV4 struct {
	host     string
	database string
}

//nolint:funlen
func askUserV4(ctx context.Context, needToAsk map[string]struct{}) (askedParamsV4, error) {
	var err error
	result := askedParamsV4{}

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
		fmt.Println("1) PostgreSQL (Recommended)")
		fmt.Println("2) MySQL")
		fmt.Println("3) SQLite")
		fmt.Println("4) None. Do not install a database")

		for {
			num := ""
			num, err = utils.Ask(
				ctx,
				"Enter number: ",
				true,
				func(s string) (bool, string, error) {
					if s != "1" && s != "2" && s != "3" && s != "4" {
						return false, "Please answer 1-4.", nil
					}

					return true, "", nil
				},
			)
			if err != nil {
				return result, err
			}

			switch num {
			case "1":
				result.database = postgresDatabase
				fmt.Println("Okay! Will try install PostgreSQL ...")
			case "2":
				result.database = sqliteDatabase
				fmt.Println("Okay! Will try install MySQL ...")
			case "3":
				result.database = sqliteDatabase
				fmt.Println("Okay! Will try install SQLite ...")
			case "4":
				result.database = noneDatabase
				fmt.Println("Okay!  ...")
			default:
				fmt.Println("Please answer 1-3.")

				continue
			}

			break
		}
	}

	return result, nil
}

func warningAskForActionV4(
	ctx context.Context,
	state panelInstallStateV4,
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

func warningV4(ctx context.Context, state panelInstallStateV4, text string) error {
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

		return false, "Please answer y or n.", nil
	})

	return err
}
