//go:build windows

package sendlogs

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	servicesDestinationDir = "services"
	scmEventsCount         = "200"
)

var collectedServices = []string{
	"GameAP", "GameAP Daemon", "MariaDB", "mysql", "PostgreSQL", "nginx", "php-fpm",
}

func collectServiceLogs(_ context.Context, destinationDir string) error {
	if !utils.IsFileExists(servicesPath) {
		// skip
		return nil
	}

	destinationDir, err := servicesDestination(destinationDir)
	if err != nil {
		return err
	}

	if utils.IsFileExists(servicesLogsPath) {
		err = copyLogDir(servicesLogsPath, filepath.Join(destinationDir, "logs"), time.Now().Add(-serviceLogMaxAge))
		if err != nil {
			return errors.WithMessage(err, "failed to copy service logs")
		}
	}

	entries, err := os.ReadDir(servicesPath)
	if err != nil {
		return errors.Wrap(err, "failed to read services directory")
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".yaml") {
			continue
		}

		err = utils.Copy(filepath.Join(servicesPath, entry.Name()), filepath.Join(destinationDir, entry.Name()))
		if err != nil {
			log.Println(errors.WithMessagef(err, "failed to copy service definition %s", entry.Name()))
		}
	}

	return nil
}

func collectServiceStatus(ctx context.Context, destinationDir string) error {
	destinationDir, err := servicesDestination(destinationDir)
	if err != nil {
		return err
	}

	builder := &strings.Builder{}

	for _, serviceName := range collectedServices {
		status, err := service.QueryStatus(ctx, serviceName)
		if err != nil {
			fmt.Fprintf(builder, "%s: %s\n", serviceName, err)

			continue
		}

		fmt.Fprintf(
			builder,
			"%s: state=%s pid=%d win32ExitCode=%d serviceSpecificExitCode=%d\n",
			status.Name, status.State, status.PID, status.Win32ExitCode, status.ServiceSpecificExitCode,
		)
	}

	err = os.WriteFile(filepath.Join(destinationDir, "status.txt"), []byte(builder.String()), 0600)
	if err != nil {
		return errors.Wrap(err, "failed to write service status")
	}

	collectSCMEvents(ctx, destinationDir)
	collectListeningPorts(ctx, destinationDir)

	return nil
}

func collectSCMEvents(ctx context.Context, destinationDir string) {
	output, err := oscore.ExecCommandWithOutput(
		ctx,
		"wevtutil", "qe", "System",
		"/q:*[System[Provider[@Name='Service Control Manager']]]",
		"/c:"+scmEventsCount, "/rd:true", "/f:text",
	)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to read service control manager events"))

		return
	}

	err = os.WriteFile(filepath.Join(destinationDir, "scm_events.txt"), []byte(output), 0600)
	if err != nil {
		log.Println(errors.Wrap(err, "failed to write service control manager events"))
	}
}

func collectListeningPorts(ctx context.Context, destinationDir string) {
	output, err := oscore.ExecCommandWithOutput(ctx, "netstat", "-ano")
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to read listening ports"))

		return
	}

	contents := output

	state, err := gameapctl.LoadPanelInstallState(ctx)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to load panel install state"))
	} else if state.Port != "" {
		contents = fmt.Sprintf(
			"netstat -ano, filtered by panel port %s\n\n%s\n",
			state.Port, filterPortLines(output, state.Port),
		)
	}

	err = os.WriteFile(filepath.Join(destinationDir, "ports.txt"), []byte(contents), 0600)
	if err != nil {
		log.Println(errors.Wrap(err, "failed to write listening ports"))
	}
}

func servicesDestination(destinationDir string) (string, error) {
	destinationDir = filepath.Join(destinationDir, servicesDestinationDir)

	err := os.MkdirAll(destinationDir, 0755)
	if err != nil {
		return "", errors.Wrap(err, "failed to create services directory")
	}

	return destinationDir, nil
}
