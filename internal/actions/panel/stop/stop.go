package stop

import (
	"fmt"

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

	fmt.Println("Stopping GameAP ...")

	err := panel.Stop(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to stop gameap")
	}

	fmt.Println("Checking process status...")

	pr, err := oscore.FindProcessByName(ctx, "gameap")
	if err != nil {
		return errors.WithMessage(err, "failed to find started gameap process")
	}
	if pr != nil {
		return errors.New("GameAP process already running")
	}

	fmt.Println("Success! GameAP process not found, stopped successfully")

	return nil
}
