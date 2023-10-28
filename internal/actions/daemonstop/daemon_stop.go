package daemonstop

import (
	"fmt"

	"github.com/gameap/gameapctl/pkg/daemon"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

func Handle(cliCtx *cli.Context) error {
	fmt.Println("Stop daemon")

	err := daemon.Stop(cliCtx.Context)
	if err != nil {
		return errors.WithMessage(err, "failed to stop daemon")
	}

	return nil
}
