package letsencrypt

import (
	"log"

	panelpkg "github.com/gameap/gameapctl/internal/pkg/panel"
	"github.com/gameap/gameapctl/pkg/configenv"
	"github.com/gameap/gameapctl/pkg/panel"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

func Disable(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	paths, err := panelpkg.ResolveScope(ctx, cliCtx.String("scope"))
	if err != nil {
		return err
	}

	configPath := paths.ConfigFilePath
	log.Printf("Reading config from: %s\n", configPath)

	lines, _, err := configenv.Read(configPath)
	if err != nil {
		return err
	}

	updates := map[string]string{
		"ACME_ENABLED": "false",
	}

	for _, k := range envKeysOwned {
		if k == "ACME_ENABLED" {
			continue
		}

		updates[k] = configenv.RemoveMarker
	}

	if err := configenv.Update(configPath, lines, updates); err != nil {
		return errors.WithMessage(err, "failed to write config")
	}

	if err := panelpkg.CheckBinaryInstalled(paths); err != nil {
		return err
	}

	log.Println("ACME disabled in config.env. Restarting gameap ...")

	if err := panel.Restart(ctx, panel.Options{Scope: paths.Scope}); err != nil {
		return errors.WithMessage(err, "failed to restart gameap")
	}

	if cliCtx.Bool("purge-certs") {
		log.Println("--purge-certs requested but cert deletion via API is not yet implemented; " +
			"remove files manually from the configured ACME storage if needed.")
	}

	return nil
}
