package restart

import (
	"fmt"
	"log"

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

	if err := panelpkg.CheckBinaryInstalled(paths); err != nil {
		return err
	}

	fmt.Println("Restarting GameAP ...")

	err = panel.Restart(ctx, panel.Options{Scope: paths.Scope})
	if err != nil {
		return errors.WithMessage(err, "failed to restart gameap")
	}

	log.Println("Checking process status")

	pr, err := oscore.WaitForProcessByName(ctx, "gameap")
	if err != nil {
		return errors.WithMessage(err, "failed to find started gameap process")
	}
	if pr == nil {
		return errors.New("started gameap process not found")
	}

	log.Println("Success! GameAP process found with pid", pr.Pid)

	return nil
}
