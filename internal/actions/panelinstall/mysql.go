package panelinstall

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
)

// Some mysql helpers

func mysqlMakeAdminConnection(ctx context.Context, dbCreds databaseCredentials) (*sql.DB, error) {
	mysqlCfgs := []mysql.Config{
		{
			User:                 "root",
			Passwd:               dbCreds.RootPassword,
			Net:                  "unix",
			Addr:                 "/var/run/mysqld/mysqld.sock",
			DBName:               "mysql",
			AllowNativePasswords: true,
		},
		{
			User:                 "root",
			Net:                  "tcp",
			Addr:                 fmt.Sprintf("%s:%s", dbCreds.Host, dbCreds.Port),
			DBName:               "mysql",
			AllowNativePasswords: true,
		},
		{
			User:                 "root",
			Passwd:               dbCreds.RootPassword,
			Net:                  "tcp",
			Addr:                 fmt.Sprintf("%s:%s", dbCreds.Host, dbCreds.Port),
			DBName:               "mysql",
			AllowNativePasswords: true,
		},
	}

	var err error
	var db *sql.DB
	for _, cfg := range mysqlCfgs {
		db, err = sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			continue
		}

		log.Printf("Cheking database %s\n", cfg.FormatDSN())

		err = db.Ping()
		if err != nil {
			log.Println(err)

			continue
		}

		version, err := mysqlVersion(ctx, db)
		if err != nil {
			log.Println(err)

			continue
		}

		log.Printf("MySQL version: %s\n", version)

		break
	}

	if err != nil {
		return nil, errors.WithMessage(err, "failed to get MySQL connection")
	}

	return db, err
}

func mysqlVersion(ctx context.Context, db *sql.DB) (string, error) {
	var version string

	err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version)
	if err != nil {
		return "", err
	}

	return version, nil
}

func mysqlIsDatabaseExists(ctx context.Context, db *sql.DB, database string) (bool, error) {
	var exists bool

	err := db.QueryRowContext(
		ctx,
		"SELECT EXISTS(SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?)",
		database,
	).Scan(&exists)
	if err != nil {
		return false, errors.WithMessage(err, "failed to execute query")
	}

	return exists, nil
}

func mysqlIsDatabaseEmpty(ctx context.Context, db *sql.DB, database string) (bool, error) {
	var exists bool

	err := db.QueryRowContext(
		ctx,
		"EXISTS(SELECT 1 FROM "+database+".games LIMIT 1)",
	).Scan(&exists)
	if err != nil {
		return false, errors.WithMessage(err, "failed to execute query")
	}

	return !exists, nil
}

func mysqlCreateDatabase(ctx context.Context, db *sql.DB, database string) error {
	_, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS "+database)
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}

	return nil
}

func mysqlIsUsereExists(ctx context.Context, db *sql.DB, username, host string) (bool, error) {
	var exists bool

	err := db.QueryRowContext(
		ctx,
		"SELECT EXISTS(SELECT 1 FROM mysql.user WHERE user = ? AND (host = '%' OR host = ?))",
		username, host,
	).Scan(&exists)
	if err != nil {
		return false, errors.WithMessage(err, "failed to execute query")
	}

	return exists, nil
}

func mysqlCreateUser(ctx context.Context, db *sql.DB, username, password string) error {
	var err error

	version, err := mysqlVersion(ctx, db)
	if err != nil {
		return errors.WithMessage(err, "failed to get mysql version")
	}

	majorVersion := strings.Split(version, ".")[0]

	if majorVersion == "8" {
		_, err = db.ExecContext(
			ctx,
			"CREATE USER IF NOT EXISTS "+
				username+
				"@'%' IDENTIFIED WITH mysql_native_password BY '"+
				password+"'",
		)
	} else {
		_, err = db.ExecContext(
			ctx,
			"CREATE USER IF NOT EXISTS "+
				username+
				"@'%' IDENTIFIED BY '"+
				password+"'",
		)
	}
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}

	return nil
}

func mysqlChangeUserPassword(ctx context.Context, db *sql.DB, username, password string) error {
	_, err := db.ExecContext(
		ctx,
		"ALTER USER '"+username+"'@'%' IDENTIFIED BY '"+password+"'",
	)
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}

	return nil
}

func mysqlGrantPrivileges(ctx context.Context, db *sql.DB, username, databaseName string) error {
	//nolint:gosec
	_, err := db.ExecContext(ctx, "GRANT SELECT ON *.* TO '"+username+"'@'%'")
	if err != nil {
		return errors.WithMessage(err, "failed to grant select privileges")
	}
	_, err = db.ExecContext(ctx, "GRANT ALL PRIVILEGES ON "+databaseName+".* TO '"+username+"'@'%'")
	if err != nil {
		return errors.WithMessage(err, "failed to grant all privileges")
	}
	_, err = db.ExecContext(ctx, "FLUSH PRIVILEGES")
	if err != nil {
		return errors.WithMessage(err, "failed to flush privileges")
	}

	return nil
}
