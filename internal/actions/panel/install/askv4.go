package install

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"golang.org/x/term"
)

type askedParamsV4 struct {
	host             string
	database         string
	existingDatabase bool
	dbCreds          databaseCredentials
}

//nolint:gocognit,cyclop
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

		ips := utils.RemoveLocalIPs(utils.DetectIPs())
		if len(ips) > 0 {
			fmt.Println("Available addresses:")
			for _, ip := range ips {
				if utils.IsIPv4(ip) {
					fmt.Printf("  * %s\n", ip)
				}
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

	if _, ok := needToAsk["database"]; ok { //nolint:nestif
		result.database, err = askDatabaseType(ctx)
		if err != nil {
			return result, err
		}

		if result.database == postgresDatabase || result.database == mysqlDatabase {
			result.existingDatabase, err = askDatabaseInstallationType(ctx, result.database)
			if err != nil {
				return result, err
			}
		}

		if result.existingDatabase {
			result.dbCreds, err = askDatabaseCredentials(ctx, result.database)
			if err != nil {
				return result, err
			}
		}
	}

	return result, nil
}

func askDatabaseType(ctx context.Context) (string, error) {
	fmt.Println("Select database to install and configure")
	fmt.Println("")
	fmt.Println("1) PostgreSQL (Recommended)")
	fmt.Println("2) MySQL")
	fmt.Println("3) SQLite")
	fmt.Println("4) None. Do not install a database")

	num, err := utils.Ask(
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
		return "", err
	}

	switch num {
	case "1":
		return postgresDatabase, nil
	case "2":
		return mysqlDatabase, nil
	case "3":
		fmt.Println("Okay! Will try install SQLite ...")

		return sqliteDatabase, nil
	case "4":
		fmt.Println("Okay! ...")

		return noneDatabase, nil
	default:
		return "", errors.New("unexpected database choice")
	}
}

func askDatabaseInstallationType(ctx context.Context, database string) (bool, error) {
	dbName := "PostgreSQL"
	if database == mysqlDatabase {
		dbName = "MySQL"
	}

	fmt.Println("")
	fmt.Printf("%s installation type:\n", dbName)
	fmt.Printf("1) Install new %s server\n", dbName)
	fmt.Printf("2) Use existing %s server\n", dbName)

	num, err := utils.Ask(
		ctx,
		"Enter number: ",
		true,
		func(s string) (bool, string, error) {
			if s != "1" && s != "2" {
				return false, "Please answer 1-2.", nil
			}

			return true, "", nil
		},
	)
	if err != nil {
		return false, err
	}

	switch num {
	case "1":
		fmt.Printf("Okay! Will try install %s ...\n", dbName)

		return false, nil
	case "2":
		fmt.Printf("Okay! Will use existing %s server ...\n", dbName)

		return true, nil
	default:
		return false, errors.New("unexpected installation type choice")
	}
}

//nolint:cyclop
func askDatabaseCredentials(ctx context.Context, database string) (databaseCredentials, error) {
	for {
		creds, err := promptDatabaseCredentials(ctx, database)
		if err != nil {
			return creds, err
		}

		tempState := panelInstallStateV4{
			Database: database,
			DBCreds:  creds,
		}

		switch database {
		case postgresDatabase:
			_, err = checkPostgreSQLConnectionV4(ctx, tempState)
		case mysqlDatabase:
			_, err = checkMySQLConnectionV4(ctx, tempState)
		}

		if err == nil {
			fmt.Println("Database connection successful!")

			return creds, nil
		}

		fmt.Printf("\nFailed to connect to database: %s\n", err)
		fmt.Println("Please re-enter the connection details.")
	}
}

func promptDatabaseCredentials(ctx context.Context, database string) (databaseCredentials, error) {
	creds := databaseCredentials{}

	defaultPort := "5432"
	if database == mysqlDatabase {
		defaultPort = "3306"
	}

	var err error

	creds.Host, err = utils.Ask(ctx, fmt.Sprintf("Enter database host (default: %s): ", defaultDBHost), true, nil)
	if err != nil {
		return creds, err
	}
	if creds.Host == "" {
		creds.Host = defaultDBHost
	}

	creds.Port, err = utils.Ask(ctx, fmt.Sprintf("Enter database port (default: %s): ", defaultPort), true, nil)
	if err != nil {
		return creds, err
	}
	if creds.Port == "" {
		creds.Port = defaultPort
	}

	creds.DatabaseName, err = utils.Ask(
		ctx,
		fmt.Sprintf("Enter database name (default: %s): ", defaultDatabaseName),
		true,
		nil,
	)
	if err != nil {
		return creds, err
	}
	if creds.DatabaseName == "" {
		creds.DatabaseName = defaultDatabaseName
	}

	creds.Username, err = utils.Ask(
		ctx,
		fmt.Sprintf("Enter database username (default: %s): ", defaultDBUsername),
		true,
		nil,
	)
	if err != nil {
		return creds, err
	}
	if creds.Username == "" {
		creds.Username = defaultDBUsername
	}

	creds.Password, err = promptDatabasePassword()
	if err != nil {
		return creds, err
	}

	return creds, nil
}

func promptDatabasePassword() (string, error) {
	fmt.Print("\nEnter database password: ")

	stdinFd := int(os.Stdin.Fd()) //nolint:gosec
	if !term.IsTerminal(stdinFd) {
		reader := bufio.NewReader(os.Stdin)

		pw, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		return strings.TrimSpace(pw), nil
	}

	passwordBytes, err := term.ReadPassword(stdinFd)
	if err != nil {
		return "", err
	}

	fmt.Println()

	return string(passwordBytes), nil
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

		return false, "Please answer y or n.", nil //nolint:goconst,nolintlint
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
