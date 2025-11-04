package start

import (
	"fmt"
	"log"

	"github.com/gameap/gameapctl/pkg/daemon"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

func Handle(cliCtx *cli.Context) error {
	fmt.Println("Start daemon")

	err := daemon.Start(cliCtx.Context)
	if err != nil {
		return errors.WithMessage(err, "failed to start daemon")
	}

	log.Println("Checking process status...")
	daemonProcess, err := daemon.FindProcess(cliCtx.Context)
	if err != nil {
		return errors.WithMessage(err, "failed to find daemon process")
	}
	if daemonProcess == nil {
		return errors.New("daemon process not found")
	}

	log.Println("Success! Daemon process found with pid", daemonProcess.Pid)

	return nil
}
