//go:build windows
// +build windows

package daemon

import (
	"context"
	"log"
	"strings"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	defaultDaemonConfigPath = "C:\\gameap\\daemon\\gameap-daemon.xml"
)

func Status(_ context.Context) error {
	result, err := utils.ExecCommandWithOutput("winsw", "status", defaultDaemonConfigPath)
	if err != nil {
		return errors.Wrap(err, "failed to get daemon status")
	}

	if !strings.Contains(result, "Active (running)") {
		return errors.New("daemon process not found")
	}

	log.Println("Daemon process found")

	return nil
}
