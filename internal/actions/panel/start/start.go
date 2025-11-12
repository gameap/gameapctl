package start

import (
	"fmt"
	"log"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/panel"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

func Handle(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	if !utils.IsCommandAvailable("gameap") {
		return errors.WithMessage(panel.ErrGameAPNotInstalled, "gameap binary not found in PATH")
	}

	fmt.Println("Starting GameAP ...")

	err := panel.Start(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to start gameap")
	}

	log.Println("Checking process status...")

	pr, err := oscore.FindProcessByName(ctx, "gameap")
	if err != nil {
		return errors.WithMessage(err, "failed to find started gameap process")
	}
	if pr == nil {
		return errors.New("started gameap process not found")
	}

	log.Println("Success! GameAP process found with pid", pr.Pid)

	return nil
}
