//go:build linux || darwin

package panel

import (
	"context"
	"fmt"
	"os/user"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func SetPrivileges(_ context.Context, path string) error {
	users := []string{"www-data", "apache", "nginx"}

	for _, u := range users {
		if uinfo, err := user.Lookup(u); err == nil {
			err = utils.ExecCommand(
				"chown", "-R",
				fmt.Sprintf("%s:%s", uinfo.Uid, uinfo.Gid), path,
			)
			if err != nil {
				return errors.WithMessage(err, "failed to change owner")
			}

			break
		}
	}

	return nil
}
