package actions

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func DaemonInstall(_ *cli.Context) error {
	fmt.Println("Install daemon")
	return nil
}
