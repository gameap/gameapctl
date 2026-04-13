package gameapctl

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const (
	daemonInstallStateFile = "daemon_install_state.json"
)

type DaemonInstallState struct {
	Host           string `json:"host"`
	ConnectURL     string `json:"connectUrl,omitempty"`
	WorkPath       string `json:"workPath"`
	SteamCMDPath   string `json:"steamCmdPath"`
	CertsPath      string `json:"certsPath"`
	FromGithub     bool   `json:"fromGithub"`
	Branch         string `json:"branch"`
	ProcessManager string `json:"processManager"`
	GRPCEnabled    bool   `json:"grpcEnabled,omitempty"`
}

func SaveDaemonInstallState(_ context.Context, state DaemonInstallState) error {
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return errors.WithMessage(err, "failed to marshal json")
	}

	dir, err := stateDirectory()
	if err != nil {
		return errors.WithMessage(err, "failed to get state directory")
	}

	finalPath := filepath.Join(dir, daemonInstallStateFile)
	tmpPath := finalPath + ".tmp"

	err = os.WriteFile(tmpPath, b, 0600)
	if err != nil {
		return errors.WithMessage(err, "failed to write temp file")
	}

	if err = os.Rename(tmpPath, finalPath); err != nil {
		if writeErr := os.WriteFile(finalPath, b, 0600); writeErr != nil {
			return errors.WithMessage(writeErr, "failed to write file")
		}

		_ = os.Remove(tmpPath)
	}

	return nil
}

func LoadDaemonInstallState(_ context.Context) (DaemonInstallState, error) {
	var state DaemonInstallState

	dir, err := stateDirectory()
	if err != nil {
		return state, errors.WithMessage(err, "failed to get state directory")
	}

	b, err := os.ReadFile(filepath.Join(dir, daemonInstallStateFile))
	if err != nil {
		return state, errors.WithMessage(err, "failed to read file")
	}

	err = json.Unmarshal(b, &state)
	if err != nil {
		return state, errors.WithMessage(err, "failed to unmarshal json")
	}

	return state, nil
}
