package app

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/gameap/gameapctl/internal/actions/daemoninstall"
	"github.com/gameap/gameapctl/internal/actions/daemonrestart"
	"github.com/gameap/gameapctl/internal/actions/daemonstart"
	"github.com/gameap/gameapctl/internal/actions/daemonstatus"
	"github.com/gameap/gameapctl/internal/actions/daemonstop"
	"github.com/gameap/gameapctl/internal/actions/daemonupdate"
	"github.com/gameap/gameapctl/internal/actions/panelinstall"
	"github.com/gameap/gameapctl/internal/actions/panelupdate"
	"github.com/gameap/gameapctl/internal/actions/selfupdate"
	"github.com/gameap/gameapctl/internal/actions/sendlogs"
	"github.com/gameap/gameapctl/internal/actions/ui"
	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/gameap/gameapctl/pkg/gameap"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

//nolint:funlen
func Run(args []string) {
	logfilepath := ""

	if len(args) == 1 && runtime.GOOS == "windows" {
		args = []string{args[0], "ui"}
	}

	app := &cli.App{
		Name:      "gameapctl",
		Usage:     "GameAP Control",
		UsageText: "Find more information at: https://docs.gameap.ru/",
		Before: func(ctx *cli.Context) error {
			var err error
			ctx.Context, err = contextInternal.SetOSContext(ctx.Context)
			if err != nil {
				return err
			}

			if ctx.Bool("debug") {
				osInfo := contextInternal.OSInfoFromContext(ctx.Context)

				fmt.Println("---------------")
				fmt.Println("Information")
				fmt.Println()
				fmt.Println("Kernel:", osInfo.Kernel)
				fmt.Println("Core:", osInfo.Core)
				fmt.Println("Distribution:", osInfo.Distribution)
				fmt.Println("DistributionVersion:", osInfo.DistributionVersion)
				fmt.Println("DistributionCodename:", osInfo.DistributionCodename)
				fmt.Println("Platform:", osInfo.Platform)
				fmt.Println("OS:", osInfo.OS)
				fmt.Println("Hostname:", osInfo.Hostname)
				fmt.Println("CPUs:", osInfo.CPUs)
				fmt.Println("---------------")
			}

			return nil
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "non-interactive",
				Value: false,
			},
			&cli.BoolFlag{
				Name:  "skip-warnings",
				Value: false,
			},
			&cli.BoolFlag{
				Name:    "debug",
				Value:   false,
				EnvVars: []string{"DEBUG"},
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
						Before: func(_ *cli.Context) error {
							logfilepath = initLogFile("daemon_install")

							packagemanager.UpdateEnvPath()

							return nil
						},
						Action: daemoninstall.Handle,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "token",
								EnvVars: []string{"CREATE_TOKEN"},
							},
							&cli.StringFlag{
								Name:    "host",
								EnvVars: []string{"PANEL_HOST"},
							},
							&cli.StringFlag{
								Name: "work-dir",
							},
						},
					},
					{
						Name:        "upgrade",
						Aliases:     []string{"update", "u"},
						Description: "Update daemon to a new version",
						Usage:       "Update daemon to a new version",
						Before: func(_ *cli.Context) error {
							packagemanager.UpdateEnvPath()

							return nil
						},
						Action: daemonupdate.Handle,
					},
					{
						Name:        "start",
						Aliases:     []string{"s"},
						Description: "Start daemon",
						Usage:       "Start daemon",
						Action:      daemonstart.Handle,
					},
					{
						Name:        "stop",
						Description: "Stop daemon",
						Usage:       "Stop daemon",
						Action:      daemonstop.Handle,
					},
					{
						Name:        "status",
						Description: "Daemon status",
						Usage:       "Daemon status",
						Action:      daemonstatus.Handle,
					},
					{
						Name:        "restart",
						Aliases:     []string{"r"},
						Description: "Restart daemon",
						Usage:       "Restart daemon",
						Action:      daemonrestart.Handle,
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
						Before: func(_ *cli.Context) error {
							logfilepath = initLogFile("panel_install")

							packagemanager.UpdateEnvPath()

							return nil
						},
						Action: panelinstall.Handle,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "path",
								Usage: "Path to GameAP root directory",
							},
							&cli.StringFlag{
								Name: "host",
							},
							&cli.StringFlag{
								Name: "port",
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
								Name:   "branch",
								Usage:  "Set specific GitHub branch.",
								Hidden: true,
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
							&cli.BoolFlag{
								Name:  "with-daemon",
								Usage: "Daemon will be also installed with panel. ",
							},
						},
					},
					{
						Name:    "upgrade",
						Aliases: []string{"update", "u"},
						Usage:   "Update panel to a new version",
						Before: func(_ *cli.Context) error {
							packagemanager.UpdateEnvPath()

							return nil
						},
						Action: panelupdate.Handle,
					},
				},
			},
			{
				Name:        "ui",
				Description: "Web interface in default browser. ",
				Usage:       "Web interface in default browser. ",
				Before: func(_ *cli.Context) error {
					packagemanager.UpdateEnvPath()

					return nil
				},
				Action: ui.Handle,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "host",
						DefaultText: "localhost",
					},
					&cli.StringFlag{
						Name:        "port",
						DefaultText: "17080",
					},
					&cli.BoolFlag{
						Name:  "no-browser",
						Usage: "Do not open browser",
					},
				},
			},
			{
				Name:        "self-update",
				Description: "Update the gameapctl binary to the latest version",
				Usage:       "Update the gameapctl binary to the latest version",
				Action:      selfupdate.Handle,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "force",
						Usage: "Update even if dev version is used. ",
					},
				},
			},
			{
				Name:        "send-logs",
				Description: "Send logs to GameAP support. You can specify log which you want to send.",
				Usage:       "Send logs to GameAP support",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:  "include-logs",
						Usage: "Send additional logs, for example: --include-logs=/var/log/nginx",
					},
				},
				Action: sendlogs.Handle,
			},
			{
				Name:    "version",
				Usage:   "Print version information",
				Aliases: []string{"v"},
				Action: func(_ *cli.Context) error {
					fmt.Println("Version:", gameap.Version)
					fmt.Println("Build Date:", gameap.BuildDate)

					return nil
				},
			},
		},
	}

	ctx := shutdownContext(context.Background())

	err := app.RunContext(ctx, args)
	if err != nil && errors.Is(err, context.Canceled) {
		fmt.Println("Terminated")
		os.Exit(130) //nolint:gomnd
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

	logpath := ""

	if runtime.GOOS == "windows" {
		logpath = "C:\\gameap\\logs\\"
	} else {
		logpath = "/var/log/gameapctl/"
	}

	if _, err := os.Stat(logpath); errors.Is(err, fs.ErrNotExist) {
		err = os.Mkdir(logpath, 0755)
		if err != nil {
			logpath, err = os.MkdirTemp("", "gameapctl-log")
			if err != nil {
				log.Fatalf("Failed to init log: %s", err)
			}
		}
	}

	f, err := os.OpenFile(
		filepath.Clean(logpath+string(os.PathSeparator)+logname),
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0666,
	)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	log.SetOutput(f)

	return filepath.Clean(logpath + string(os.PathSeparator) + logname)
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
