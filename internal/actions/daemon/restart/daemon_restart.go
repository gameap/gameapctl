package restart

import (
	"fmt"
	"log"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/daemon"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

func Handle(cliCtx *cli.Context) error {
	fmt.Println("Restart daemon")

	opts := daemon.Options{}
	if st, err := gameapctl.LoadDaemonInstallState(cliCtx.Context); err == nil {
		opts.Scope = st.Scope
	}

	err := daemon.Restart(cliCtx.Context, opts)
	if err != nil {
		return errors.WithMessage(err, "failed to restart daemon")
	}

	log.Println("Checking process status")
	daemonProcess, err := daemon.WaitForProcess(cliCtx.Context)
	if err != nil {
		return errors.WithMessage(err, "failed to find daemon process")
	}
	if daemonProcess == nil {
		return errors.New("daemon process not found")
	}

	log.Println("Success! Daemon process found with pid", daemonProcess.Pid)

	return nil
}
