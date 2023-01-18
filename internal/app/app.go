package app

import (
	"log"

	"github.com/urfave/cli/v2"
)

func Run(args []string) {
	app := &cli.App{
		Name:      "gameapctl",
		Usage:     "GameAP Control",
		UsageText: "Find more information at: https://docs.gameap.ru/",
		Commands: []*cli.Command{
			{
				Name:   "install",
				Usage:  "Install GameAP Web or GameAP Daemon",
				Action: installAction,
				Subcommands: []*cli.Command{
					{
						Name:        "gameap",
						Aliases:     []string{"web", "panel"},
						Description: "Install GameAP Web",
						Usage:       "Install GameAP Web",
					},
					{
						Name:        "daemon",
						Aliases:     []string{"gameap-daemon"},
						Description: "Install GameAP Daemon",
						Usage:       "Install GameAP Daemon",
					},
				},
			},
			{
				Name:  "upgrade",
				Usage: "Upgrade GameAP Web or GameAP Daemon",
			},
			{
				Name:  "start",
				Usage: "Start",
			},
			{
				Name:  "stop",
				Usage: "Stop",
			},
			{
				Name:  "restart",
				Usage: "Restart",
			},
			{
				Name:  "convert",
				Usage: "Convert from another Game Panel",
			},
			{
				Name:  "uninstall",
				Usage: "Uninstall GameAP Web or GameAP Daemon",
			},
		},
	}

	err := app.Run(args)
	if err != nil {
		log.Fatal(err)
	}
}

func installAction(c *cli.Context) error {
	return nil
}
