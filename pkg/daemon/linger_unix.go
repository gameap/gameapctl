//go:build linux || darwin

package daemon

import (
	"context"
	"log"
	"os/user"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/pkg/errors"
)

func enableLinger(ctx context.Context) {
	u, err := user.Current()
	if err != nil {
		log.Println(errors.Wrap(err, "failed to determine current user for loginctl enable-linger"))

		return
	}

	if err := oscore.ExecCommand(ctx, "loginctl", "enable-linger", u.Username); err != nil {
		log.Printf(
			"Warning: failed to enable linger for %s (gameap-daemon will stop on logout): %v.\n"+
				"To fix manually: sudo loginctl enable-linger %s\n",
			u.Username, err, u.Username,
		)
	}
}
