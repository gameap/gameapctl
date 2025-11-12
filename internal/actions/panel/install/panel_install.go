package install

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/sethvargo/go-password/password"
	"github.com/urfave/cli/v2"
)

var errInvalidVersion = errors.New("invalid version (should be 3 or 4")

var DatabasePasswordGenerator = lo.Must(password.NewGenerator(&password.GeneratorInput{
	Symbols: "_-+=",
}))

func Handle(cliCtx *cli.Context) error {
	var err error

	version := lo.CoalesceOrEmpty(cliCtx.String("version"), "3")

	switch {
	case version == "":
		// Default to v3
		return HandleV3(cliCtx)

	case strings.HasPrefix(version, "3"),
		strings.HasPrefix(version, "v3"):
		return HandleV3(cliCtx)

	case strings.HasPrefix(version, "4"),
		strings.HasPrefix(version, "v4"):
		return HandleV4(cliCtx)

	default:
		err = errInvalidVersion
	}

	return err
}
