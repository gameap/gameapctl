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

func collectWebServerLogs(ctx context.Context, destinationDir string) error {
	state, err := gameapctl.LoadPanelInstallState(ctx)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to load panel install state"))

		return nil
	}

	if state.WebServer == "" || state.WebServer == "none" {
		return nil
	}

	destinationDir = filepath.Join(destinationDir, "webserver")

	var candidates []string

	switch state.WebServer {
	case "nginx":
		candidates = append(candidates, "/var/log/nginx/error.log")
		matches, _ := filepath.Glob("/var/log/nginx/gameap*")
		candidates = append(candidates, matches...)
	case "apache":
		candidates = append(candidates,
			"/var/log/apache2/error.log",
			"/var/log/apache2/gameap_error.log",
			"/var/log/apache2/gameap_access.log",
			"/var/log/httpd/error_log",
		)
		matches, _ := filepath.Glob("/var/log/httpd/gameap*")
		candidates = append(candidates, matches...)
	}

	copied := false
	for _, logPath := range candidates {
		if !utils.IsFileExists(logPath) {
			continue
		}

		if !copied {
			if err := os.MkdirAll(destinationDir, 0755); err != nil {
				return errors.WithMessage(err, "failed to create webserver logs directory")
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
