package sendlogs

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/pkg/errors"
)

const maskedValue = "***"

// collectInstallState saves the panel and daemon install states. They show which
// options the installation was run with, which is not derivable from the logs.
// Secrets are replaced by a mask before saving.
func collectInstallState(ctx context.Context, destinationDir string) error {
	destinationDir = filepath.Join(destinationDir, "state")

	err := os.MkdirAll(destinationDir, 0755)
	if err != nil {
		return errors.Wrap(err, "failed to create state directory")
	}

	panelState, err := gameapctl.LoadPanelInstallState(ctx)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to load panel install state"))
	} else {
		err = writeStateFile(filepath.Join(destinationDir, "panel_install_state.json"), maskPanelInstallState(panelState))
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to save panel install state"))
		}
	}

	daemonState, err := gameapctl.LoadDaemonInstallState(ctx)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to load daemon install state"))

		return nil
	}

	err = writeStateFile(filepath.Join(destinationDir, "daemon_install_state.json"), maskDaemonInstallState(daemonState))
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to save daemon install state"))
	}

	return nil
}

func maskPanelInstallState(state gameapctl.PanelInstallState) gameapctl.PanelInstallState {
	state.DBPassword = mask(state.DBPassword)
	state.DBRootPassword = mask(state.DBRootPassword)
	state.AdminPassword = mask(state.AdminPassword)

	return state
}

func maskDaemonInstallState(state gameapctl.DaemonInstallState) gameapctl.DaemonInstallState {
	state.ConnectURL = maskConnectURL(state.ConnectURL)

	return state
}

// maskConnectURL hides the daemon setup key, keeping the address it points to.
func maskConnectURL(connectURL string) string {
	if connectURL == "" {
		return ""
	}

	parsed, err := url.Parse(connectURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return maskedValue
	}

	return parsed.Scheme + "://" + parsed.Host + "/" + maskedValue
}

func mask(value string) string {
	if value == "" {
		return ""
	}

	return maskedValue
}

func writeStateFile(path string, state any) error {
	contents, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal state")
	}

	err = os.WriteFile(path, contents, 0600)
	if err != nil {
		return errors.Wrap(err, "failed to write state file")
	}

	return nil
}
