package status

import (
	"log"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

func Handle(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

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
