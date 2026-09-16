//go:build !windows

package panel

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strconv"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/pkg/errors"
)

const panelServiceName = "gameap"

// LogStartDiagnostics writes into the log why the panel did not answer its health
// check: the service status and the last lines of its journal.
func LogStartDiagnostics(ctx context.Context, scope string) {
	log.Println("Collecting GameAP service diagnostics ...")

	if _, err := exec.LookPath("systemctl"); err != nil {
		log.Println(errors.WithMessage(err, "systemctl is not available, skipping service diagnostics"))

		return
	}

	scopeArgs := []string{}
	if scope == gameap.ScopeUser {
		scopeArgs = append(scopeArgs, "--user")
	}

	output, err := oscore.ExecCommandWithOutput(
		ctx, "systemctl", append(append([]string{}, scopeArgs...), "status", panelServiceName, "--no-pager")...,
	)
	if err != nil {
		log.Println(errors.WithMessagef(err, "failed to get '%s' service status", panelServiceName))
	} else {
		log.Printf("Status of '%s' service:\n%s\n", panelServiceName, output)
	}

	output, err = oscore.ExecCommandWithOutput(
		ctx,
		"journalctl",
		append(append([]string{}, scopeArgs...),
			"-u", panelServiceName, "--no-pager", "-n", strconv.Itoa(diagnosticsLogLines))...,
	)
	if err != nil {
		log.Println(errors.WithMessagef(err, "failed to read journal for '%s'", panelServiceName))

		return
	}

	log.Printf("Last lines of the '%s' journal:\n%s\n", panelServiceName, output)

	fmt.Println("Run 'gameapctl send-logs' to send the logs to GameAP support")
}
