package install

import (
	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/sethvargo/go-password/password"
	"github.com/urfave/cli/v2"
)

var errV3InstallNotSupported = errors.New(
	"GameAP v3 installation via --version is not supported; only v4 (4.x.x) tags are accepted",
)

var DatabasePasswordGenerator = lo.Must(password.NewGenerator(&password.GeneratorInput{
	Symbols: "_-+=",
}))

func Handle(cliCtx *cli.Context) error {
	raw := cliCtx.String("version")
	if raw == "" {
		return HandleV4(cliCtx)
	}

	norm, err := releasefinder.NormalizeTag(raw)
	if err != nil {
		return err
	}

	if releasefinder.IsMajorV3(norm) {
		return errV3InstallNotSupported
	}

	return HandleV4(cliCtx)
}
