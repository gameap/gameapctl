package app

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gameap/gameapctl/internal/actions"
	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

// nolint:funlen
func Run(args []string) {
	logfilepath := ""

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
						Before: func(context *cli.Context) error {
							logfilepath = initLogFile("daemon_install")
							return nil
						},
						Action: actions.DaemonInstall,
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
						Before: func(context *cli.Context) error {
							logfilepath = initLogFile("panel_install")
							return nil
						},
						Action: actions.PanelInstall,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "path",
								Usage: "Path to GameAP root directory",
							},
							&cli.StringFlag{
								Name: "host",
							},
							&cli.StringFlag{
								Name: "web-server",
							},
							&cli.BoolFlag{
								Name:  "develop",
								Usage: "Install develop version of panel.",
							},
							&cli.BoolFlag{
								Name:  "github",
								Usage: "Install gameap from GitHub.",
							},
							&cli.StringFlag{
								Name:  "database",
								Usage: "Database type. Available: mysql, sqlite. ",
							},
							&cli.StringFlag{
								Name: "database-host",
							},
							&cli.StringFlag{
								Name: "database-port",
							},
							&cli.StringFlag{
								Name: "database-name",
							},
							&cli.StringFlag{
								Name: "database-username",
							},
							&cli.StringFlag{
								Name: "database-password",
							},
						},
					},
				},
			},
			{
				Name:    "version",
				Aliases: []string{"v"},
				Action: func(context *cli.Context) error {
					fmt.Println("Version:", Version)
					fmt.Println("Build Date:", BuildDate)
					return nil
				},
			},
		},
	}

	ctx := shutdownContext(context.Background())

	err := app.RunContext(ctx, args)
	if err != nil && errors.Is(err, context.Canceled) {
		fmt.Println("Terminated")
		os.Exit(130)
	}
	if err != nil {
		fmt.Println(err)

		if logfilepath != "" {
			fmt.Println("See details in log file: " + logfilepath)
		}

		log.Fatal(err)
	}
}

func initLogFile(command string) string {
	logname := fmt.Sprintf("%s_%s.log", command, time.Now().Format("2006-01-02_15-04-05.000"))

	logpath := "/var/log/gameapctl/"

	if _, err := os.Stat(logpath); errors.Is(err, fs.ErrNotExist) {
		err = os.Mkdir(logpath, 0755)
		if err != nil {
			logpath, err = os.MkdirTemp("", "gameapctl-log")
			if err != nil {
				log.Fatalf("Failed to init log: %s", err)
			}
		}
	}

	f, err := os.OpenFile(logpath+"/"+logname, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	log.SetOutput(f)

	return logpath + "/" + logname
}

func shutdownContext(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGQUIT, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutdown signal received...")
		cancel()
	}()

	return ctx
}
