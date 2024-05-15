package panelinstall

import (
	"context"
	"fmt"
	"log"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/gameap/gameapctl/pkg/fixer"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

//nolint:funlen
func tryToFixPanelInstallation(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	fmt.Println("Trying to fix panel installation ...")

	var err error

	fixers := []fixer.Item{
		{
			Name: "Reload nginx",
			Condition: func(ctx context.Context) (bool, error) {
				if state.WebServer != nginxWebServer {
					return false, nil
				}

				osInfo := contextInternal.OSInfoFromContext(ctx)

				return osInfo.IsLinux(), nil
			},
			FixFunc: func(_ context.Context) error {
				err = utils.ExecCommand("nginx", "-s", "reload")
				if err != nil {
					return errors.WithMessage(err, "failed to reload nginx")
				}

				return nil
			},
		},
		{
			Name: "Chown gameap directory",
			Condition: func(_ context.Context) (bool, error) {
				return true, nil
			},
			FixFunc: func(ctx context.Context) error {
				state, err = chownRGameapDirectory(ctx, state)
				if err != nil {
					return errors.WithMessage(err, "failed to chown gameap directory")
				}

				return nil
			},
		},
		{
			Name: "Remove nginx default.conf",
			Condition: func(_ context.Context) (bool, error) {
				return state.WebServer == nginxWebServer && utils.IsFileExists("/etc/nginx/conf.d/default.conf"), nil
			},
			FixFunc: func(ctx context.Context) error {
				log.Println("Disabling nginx default.conf config")

				err = utils.Move("/etc/nginx/conf.d/default.conf", "/etc/nginx/conf.d/default.conf.disabled")
				if err != nil {
					return errors.WithMessage(err, "failed to rename default nginx config")
				}
				err = service.Restart(ctx, "nginx")
				if err != nil {
					return errors.WithMessage(err, "failed to restart nginx")
				}

				return nil
			},
		},
		{
			Name: "Disable apache 000-default site",
			Condition: func(_ context.Context) (bool, error) {
				return state.WebServer == apacheWebServer, nil
			},
			FixFunc: func(ctx context.Context) error {
				log.Println("Disabling apache 000-default site")

				err = utils.ExecCommand("a2dissite", "000-default")
				if err != nil {
					return errors.WithMessage(err, "failed to disable 000-default")
				}

				err = service.Restart(ctx, "apache2")
				if err != nil {
					return errors.WithMessage(err, "failed to restart apache")
				}

				return nil
			},
		},
		{
			Name: "Replace database host from localhost to 127.0.0.1",
			Condition: func(_ context.Context) (bool, error) {
				return utils.IsFileExists(state.Path+"/.env") && state.DBCreds.Host == "localhost", nil
			},
			FixFunc: func(ctx context.Context) error {
				log.Print("Replacing localhost to 127.0.0.1 in .env")

				state.DBCreds.Host = "127.0.0.1" //nolint:goconst
				state, err = updateDotEnv(ctx, state)
				if err != nil {
					return errors.WithMessage(err, "failed to update .env")
				}

				return nil
			},
		},
	}

	ferr := fixer.RunFixer(ctx, func(ctx context.Context) error {
		state, err = checkInstallation(ctx, state)
		if err != nil {
			return err
		}

		return nil
	}, fixers)
	if ferr != nil {
		return state, errors.WithMessage(err, "failed to fix panel installation")
	}

	return state, nil
}
