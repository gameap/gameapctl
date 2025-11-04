package panel

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/runhelper"
	"github.com/pkg/errors"
)

// Install installs GameAP v4
func install(ctx context.Context, config InstallConfig) error {
	init, err := runhelper.DetectInit(ctx)
	if err != nil {
		log.Println("Failed to detect init:", err)
	}

	if init == runhelper.InitSystemd {
		fmt.Println("Creating systemd unit file ...")

		if err := createSystemdUnit(ctx, config); err != nil {
			return errors.WithMessage(err, "failed to create systemd unit file")
		}
	} else {
		fmt.Println("Init system not detected or unsupported, skipping systemd unit creation")
	}

	fmt.Println("GameAP v4 installation completed successfully")

	return nil
}

func createSystemdUnit(ctx context.Context, config InstallConfig) error {
	tmpl, err := template.New("systemd.unit").Parse(systemdUnitTemplate)
	if err != nil {
		return errors.WithMessage(err, "failed to parse systemd unit template")
	}

	readWritePaths := fmt.Sprintf("%s %s", config.DataDirectory, config.FilesLocalBasePath)
	if config.LegacyPath != "" {
		readWritePaths = fmt.Sprintf("%s %s", readWritePaths, config.LegacyPath)
	}

	data := SystemdUnitConfig{
		User:             config.User,
		Group:            config.Group,
		WorkingDirectory: config.DataDirectory,
		ExecStart:        config.BinaryPath,
		EnvironmentFile:  filepath.Join(config.ConfigDirectory, "config.env"),
		ReadWritePaths:   readWritePaths,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return errors.WithMessage(err, "failed to execute systemd unit template")
	}

	unitPath := filepath.Join(defaultSystemdUnitDir, "gameap.service")
	if err := os.WriteFile(unitPath, buf.Bytes(), 0644); err != nil {
		return errors.WithMessage(err, "failed to write systemd unit file")
	}

	// Reload systemd daemon
	if err = oscore.ExecCommand(ctx, "systemctl", "daemon-reload"); err != nil {
		return errors.WithMessage(err, "failed to reload systemd daemon")
	}

	return nil
}
