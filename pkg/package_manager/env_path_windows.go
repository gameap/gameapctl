//go:build windows
// +build windows

package packagemanager

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func UpdateEnvPath() {
	currentPath := strings.Split(os.Getenv("PATH"), string(filepath.ListSeparator))
	appendPath := make([]string, 0, len(repository)*2)

	for _, p := range repository {
		if p.DefaultInstallPath == "" {
			continue
		}

		if !utils.IsFileExists(p.DefaultInstallPath) {
			continue
		}

		if utils.Contains(currentPath, p.DefaultInstallPath) {
			continue
		}

		if utils.Contains(appendPath, p.DefaultInstallPath) {
			continue
		}

		appendPath = append(appendPath, p.DefaultInstallPath)
	}

	entries, err := os.ReadDir(gameap.DefaultToolsPath)
	if err != nil {
		log.Fatal(err)
	}

	if utils.IsFileExists(gameap.DefaultToolsPath) &&
		!utils.Contains(currentPath, gameap.DefaultToolsPath) &&
		!utils.Contains(appendPath, gameap.DefaultToolsPath) {
		appendPath = append(appendPath, gameap.DefaultToolsPath)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		path := filepath.Join(gameap.DefaultToolsPath, e.Name())

		if !utils.IsFileExists(path) {
			continue
		}

		if utils.Contains(appendPath, path) {
			continue
		}

		if utils.Contains(currentPath, path) {
			continue
		}

		appendPath = append(appendPath, path)
	}

	newPath := os.Getenv("PATH") +
		string(filepath.ListSeparator) +
		strings.Join(appendPath, string(filepath.ListSeparator))

	log.Println("New PATH:", newPath)
	err = os.Setenv(
		"PATH",
		newPath,
	)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to set PATH"))
	}
}
