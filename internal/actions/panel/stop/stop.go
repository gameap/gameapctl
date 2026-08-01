package stop

import (
	"fmt"

	panelpkg "github.com/gameap/gameapctl/internal/pkg/panel"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/panel"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

func Handle(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	paths, err := panelpkg.ResolveScope(ctx, cliCtx.String("scope"))
	if err != nil {
		return err
	}

	fmt.Println("Stopping GameAP ...")

	err = panel.Stop(ctx, panel.Options{Scope: paths.Scope})
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
