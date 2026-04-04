package gameapctl

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const (
	panelInstallStateFile = "panel_install_state.json"
)

type PanelInstallState struct {
	Version              string `json:"version"`
	Host                 string `json:"host"`
	HostIP               string `json:"hostIp"`
	Port                 string `json:"port"`
	Path                 string `json:"path,omitempty"`
	ConfigDirectory      string `json:"configDirectory,omitempty"`
	DataDirectory        string `json:"dataDirectory,omitempty"`
	WebServer            string `json:"webServer,omitempty"`
	Database             string `json:"database"`
	DatabaseWasInstalled bool   `json:"databaseWasInstalled"`
	Develop              bool   `json:"develop"`
	FromGithub           bool   `json:"fromGithub"`
	Branch               string `json:"branch"`

	DBHost         string `json:"dbHost,omitempty"`
	DBPort         string `json:"dbPort,omitempty"`
	DBName         string `json:"dbName,omitempty"`
	DBUsername     string `json:"dbUsername,omitempty"`
	DBPassword     string `json:"dbPassword,omitempty"`
	DBRootPassword string `json:"dbRootPassword,omitempty"`
	AdminPassword  string `json:"adminPassword,omitempty"`
}

func SavePanelInstallState(_ context.Context, state PanelInstallState) error {
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return errors.WithMessage(err, "failed to marshal json")
	}

	dir, err := stateDirectory()
	if err != nil {
		return errors.WithMessage(err, "failed to get state directory")
	}

	finalPath := filepath.Join(dir, panelInstallStateFile)
	tmpPath := finalPath + ".tmp"

	err = os.WriteFile(tmpPath, b, 0600)
	if err != nil {
		return errors.WithMessage(err, "failed to write temp file")
	}

	if err = os.Rename(tmpPath, finalPath); err != nil {
		// Fallback to direct write if rename fails (e.g. file locked on Windows)
		if writeErr := os.WriteFile(finalPath, b, 0600); writeErr != nil {
			return errors.WithMessage(writeErr, "failed to write file")
		}

		_ = os.Remove(tmpPath)
	}

	return nil
}

func LoadPanelInstallState(_ context.Context) (PanelInstallState, error) {
	var state PanelInstallState

	dir, err := stateDirectory()
	if err != nil {
		return state, errors.WithMessage(err, "failed to get state directory")
	}

	b, err := os.ReadFile(filepath.Join(dir, panelInstallStateFile))
	if err != nil {
		return state, errors.WithMessage(err, "failed to read file")
	}

	err = json.Unmarshal(b, &state)
	if err != nil {
		return state, errors.WithMessage(err, "failed to unmarshal json")
	}

	return state, nil
}
