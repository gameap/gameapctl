//go:build windows

package install

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const panelServiceName = "GameAP"

// The panel writes nothing on its own, its stdout and stderr are captured by shawl.
// The path repeats the log-directory of the gameap package.
func panelServiceLogPath() string {
	return filepath.Join(gameap.DefaultWorkPath, "services", "logs", "gameap", panelServiceName+".log")
}

func logPanelStartDiagnostics(ctx context.Context, _ panelInstallStateV4) {
	log.Println("Collecting GameAP service diagnostics ...")

	status, err := service.QueryStatus(ctx, panelServiceName)
	if err != nil {
		log.Println(errors.WithMessagef(err, "failed to query '%s' service status", panelServiceName))
	} else {
		log.Printf(
			"Service '%s': state=%s pid=%d win32ExitCode=%d serviceSpecificExitCode=%d\n",
			status.Name, status.State, status.PID, status.Win32ExitCode, status.ServiceSpecificExitCode,
		)
	}

	logPath := panelServiceLogPath()

	if !utils.IsFileExists(logPath) {
		log.Printf("GameAP service log '%s' not found\n", logPath)

		return
	}

	tail, err := utils.TailFile(logPath, diagnosticsLogLines, diagnosticsLogBytes)
	if err != nil {
		log.Println(errors.WithMessagef(err, "failed to read '%s'", logPath))

		return
	}

	log.Printf("Last lines of '%s':\n%s\n", logPath, tail)

	fmt.Println("GameAP log file:", logPath)
	fmt.Println("Run 'gameapctl send-logs' to send the logs to GameAP support")
}
