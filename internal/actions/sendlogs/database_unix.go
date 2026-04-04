//go:build linux || darwin

package sendlogs

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func collectDatabaseLogs(ctx context.Context, destinationDir string) error {
	state, err := gameapctl.LoadPanelInstallState(ctx)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to load panel install state"))

		return nil
	}

	if state.Database == "" || state.Database == "none" || state.Database == "sqlite" {
		return nil
	}

	destinationDir = filepath.Join(destinationDir, "database")

	var candidates []string

	switch state.Database {
	case "mysql", "mariadb":
		candidates = append(candidates,
			"/var/log/mysql/error.log",
			"/var/log/mariadb/mariadb.log",
			"/var/log/mysqld.log",
		)
	case "postgres", "postgresql", "pgsql":
		matches, _ := filepath.Glob("/var/log/postgresql/*.log")
		candidates = append(candidates, matches...)
	}

	copied := false
	for _, logPath := range candidates {
		if !utils.IsFileExists(logPath) {
			continue
		}

		if !copied {
			if err := os.MkdirAll(destinationDir, 0755); err != nil {
				return errors.WithMessage(err, "failed to create database logs directory")
			}
			copied = true
		}

		dest := filepath.Join(destinationDir, filepath.Base(logPath))
		if err := utils.Copy(logPath, dest); err != nil {
			log.Println(errors.WithMessagef(err, "failed to copy %s", logPath))
		}
	}

	if copied {
		err = utils.ChownR(destinationDir, 1000, 1000) //nolint:mnd
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to change owner"))
		}

		err = os.Chmod(destinationDir, 0755)
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to change permissions"))
		}
	}

	return nil
}
