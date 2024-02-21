//go:build windows
// +build windows

package daemon

import (
	"context"
	"strings"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func Restart(_ context.Context) error {
	result, err := utils.ExecCommandWithOutput("winsw", "restart", defaultDaemonConfigPath)
	if err != nil {
		return errors.WithMessage(err, "failed to get daemon status")
	}

	if !strings.Contains(result, "restarted successfully") {
		return errors.New("failed to restart daemon")
	}

	return nil
}
