package changepassword

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

const (
	defaultConfigDirUnix    = "/etc/gameap"
	defaultConfigDirWindows = "C:\\gameap\\web"
	configFileName          = "config.env"
)

func Handle(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	// Get username from first argument
	username := cliCtx.Args().First()
	if username == "" {
		return errors.New("username is required as first argument")
	}

	// Get password from second argument or prompt
	password := cliCtx.Args().Get(1)
	if password == "" {
		var err error
		password, err = promptPassword()
		if err != nil {
			return errors.WithMessage(err, "failed to read password")
		}
	}

	if password == "" {
		return errors.New("password cannot be empty")
	}

	return ChangePassword(ctx, username, password)
}

// ChangePassword changes the password for a user in the GameAP v4 database.
// It reads the database configuration from config.env, connects to the database,
// verifies the user exists, hashes the password, and updates it in the database.
func ChangePassword(ctx context.Context, username, password string) error {
	if username == "" {
		return errors.New("username cannot be empty")
	}

	if password == "" {
		return errors.New("password cannot be empty")
	}

	// Get config file path
	configPath := getConfigPath()
	log.Printf("Reading config from: %s\n", configPath)

	// Parse config.env file
	config, err := parseConfig(configPath)
	if err != nil {
		return errors.WithMessage(err, "failed to parse config file")
	}

	databaseDriver := config["DATABASE_DRIVER"]
	databaseURL := config["DATABASE_URL"]

	if databaseDriver == "" {
		return errors.New("DATABASE_DRIVER not found in config.env")
	}
	if databaseURL == "" {
		return errors.New("DATABASE_URL not found in config.env")
	}

	log.Printf("Connecting to database (driver: %s)...\n", databaseDriver)

	// Connect to database
	db, err := connectDatabase(databaseDriver, databaseURL)
	if err != nil {
		return errors.WithMessage(err, "failed to connect to database")
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Printf("Failed to close database connection: %v\n", err)
		}
	}(db)

	// Determine placeholder syntax based on driver
	isPostgres := databaseDriver == "postgres" || databaseDriver == "postgresql" || databaseDriver == "pgsql"

	// Verify user exists
	var userID int
	selectQuery := "SELECT id FROM users WHERE login = ?"
	if isPostgres {
		selectQuery = "SELECT id FROM users WHERE login = $1"
	}

	err = db.QueryRowContext(ctx, selectQuery, username).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.Errorf("user '%s' not found in database", username)
		}

		return errors.WithMessage(err, "failed to query user")
	}

	log.Printf("Found user '%s' (id: %d)\n", username, userID)

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return errors.WithMessage(err, "failed to hash password")
	}

	now := time.Now().UTC()
	updateQuery := "UPDATE users SET password = ?, updated_at = ? WHERE login = ?"
	if isPostgres {
		updateQuery = "UPDATE users SET password = $1, updated_at = $2 WHERE login = $3"
	}

	result, err := db.ExecContext(
		ctx,
		updateQuery,
		string(hashedPassword),
		now,
		username,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to update password")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.WithMessage(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.New("no rows were updated")
	}

	log.Printf("Successfully changed password for user '%s'\n", username)

	return nil
}

func getConfigPath() string {
	var configDir string
	if runtime.GOOS == "windows" {
		configDir = defaultConfigDirWindows
	} else {
		configDir = defaultConfigDirUnix
	}

	return filepath.Join(configDir, configFileName)
}

func parseConfig(configPath string) (map[string]string, error) {
	config := make(map[string]string)

	file, err := os.Open(configPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, errors.Errorf("config file not found at %s", configPath)
		}

		return nil, errors.WithMessage(err, "failed to open config file")
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("Failed to close config file: %v\n", err)
		}
	}(file)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE format
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				config[key] = value
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.WithMessage(err, "failed to read config file")
	}

	return config, nil
}

func connectDatabase(driver, dsn string) (*sql.DB, error) {
	var driverName string

	switch driver {
	case "mysql":
		driverName = "mysql"
		if !strings.Contains(dsn, "parseTime=") {
			if strings.Contains(dsn, "?") {
				dsn += "&parseTime=true"
			} else {
				dsn += "?parseTime=true"
			}
		}
	case "postgres", "postgresql", "pgsql":
		driverName = "pgx"
	case "sqlite":
		driverName = "sqlite"
	default:
		return nil, errors.Errorf("unsupported database driver: %s", driver)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to open database connection")
	}

	// Test the connection
	err = db.Ping()
	if err != nil {
		closeErr := db.Close()
		if closeErr != nil {
			log.Printf("Failed to close database connection: %v\n", closeErr)
		}

		return nil, errors.WithMessage(err, "failed to ping database")
	}

	return db, nil
}

func promptPassword() (string, error) {
	fmt.Print("Enter new password: ")

	// Check if stdin is a terminal
	stdinFd := int(os.Stdin.Fd())
	if !term.IsTerminal(stdinFd) {
		// Not a terminal, read from stdin directly
		reader := bufio.NewReader(os.Stdin)
		password, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		return strings.TrimSpace(password), nil
	}

	// Terminal - read password without echo
	passwordBytes, err := term.ReadPassword(stdinFd)
	if err != nil {
		return "", err
	}
	fmt.Println() // Print newline after password input

	return string(passwordBytes), nil
}
