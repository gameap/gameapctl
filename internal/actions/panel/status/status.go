package status

import (
	"log"

	panelpkg "github.com/gameap/gameapctl/internal/pkg/panel"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

func Handle(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	if paths, err := panelpkg.ResolveScope(ctx, cliCtx.String("scope")); err == nil {
		log.Println("Installation scope:", paths.Scope)
	}

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
