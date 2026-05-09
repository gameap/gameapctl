package letsencrypt

import (
	"log"

	"github.com/gameap/gameapctl/pkg/panel"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

func Disable(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	configPath := ConfigPath()
	log.Printf("Reading config from: %s\n", configPath)

	lines, _, err := readEnv(configPath)
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

		updates[k] = removeMarker
	}

	if err := writeEnv(configPath, lines, updates); err != nil {
		return errors.WithMessage(err, "failed to write config")
	}

	if !utils.IsCommandAvailable("gameap") {
		return errors.WithMessage(panel.ErrGameAPNotInstalled, "gameap binary not found in PATH")
	}

	log.Println("ACME disabled in config.env. Restarting gameap ...")

	if err := panel.Restart(ctx); err != nil {
		return errors.WithMessage(err, "failed to restart gameap")
	}

	if cliCtx.Bool("purge-certs") {
		log.Println("--purge-certs requested but cert deletion via API is not yet implemented; " +
			"remove files manually from the configured ACME storage if needed.")
	}

	return nil
}
