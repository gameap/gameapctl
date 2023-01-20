package app

import (
	"log"

	"github.com/gameap/gameapctl/internal/actions"
	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/urfave/cli/v2"
)

func Run(args []string) {
	app := &cli.App{
		Name:      "gameapctl",
		Usage:     "GameAP Control",
		UsageText: "Find more information at: https://docs.gameap.ru/",
		Before: func(context *cli.Context) error {
			var err error
			context.Context, err = contextInternal.SetOSContext(context.Context)
			if err != nil {
				return err
			}

			return nil
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "non-interactive",
				Value: false,
			},
		},
		Commands: []*cli.Command{
			{
				Name:        "daemon",
				Aliases:     []string{"d"},
				Description: "Daemon actions",
				Usage:       "Daemon actions",
				Subcommands: []*cli.Command{
					{
						Name:        "install",
						Aliases:     []string{"i"},
						Description: "Install daemon",
						Usage:       "Install daemon",
						Action:      actions.DaemonInstall,
					},
					{
						Name:        "upgrade",
						Aliases:     []string{"update", "u"},
						Description: "Upgrade daemon to new version",
						Usage:       "Upgrade daemon to new version",
					},
					{
						Name:        "uninstall",
						Description: "Uninstall daemon",
						Usage:       "Uninstall daemon",
					},
					{
						Name:        "start",
						Aliases:     []string{"s"},
						Description: "Start daemon",
						Usage:       "Start daemon",
					},
					{
						Name:        "stop",
						Description: "Stop daemon",
						Usage:       "Stop daemon",
					},
					{
						Name:        "restart",
						Aliases:     []string{"r"},
						Description: "Restart daemon",
						Usage:       "Restart daemon",
					},
				},
			},
			{
				Name:        "panel",
				Aliases:     []string{"p"},
				Description: "GameAP web part actions",
				Usage:       "GameAP web part actions",
				Subcommands: []*cli.Command{
					{
						Name:        "install",
						Aliases:     []string{"i"},
						Description: "Install panel",
						Usage:       "Install panel",
						Action:      actions.PanelInstall,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name: "path",
							},
							&cli.StringFlag{
								Name: "host",
							},
							&cli.StringFlag{
								Name: "web-server",
							},
							&cli.StringFlag{
								Name: "database",
							},
							&cli.BoolFlag{
								Name: "develop",
							},
							&cli.BoolFlag{
								Name: "github",
							},
						},
					},
				},
			},
		},
	}

	err := app.Run(args)
	if err != nil {
		log.Fatal(err)
	}
}
