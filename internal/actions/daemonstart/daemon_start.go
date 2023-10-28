package daemonstart

import (
	"fmt"

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

	return nil
}
