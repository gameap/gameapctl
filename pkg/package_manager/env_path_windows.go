//go:build windows

package packagemanager

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	internalcontext "github.com/gameap/gameapctl/internal/context"
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/package_manager/windows"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

//nolint:gocognit
func UpdateEnvPath(ctx context.Context) {
	packages, err := windows.LoadPackages(internalcontext.OSInfoFromContext(ctx))
	if err != nil {
		panic(errors.WithMessage(err, "failed to load packages"))
	}

	currentPath := strings.Split(os.Getenv("PATH"), string(filepath.ListSeparator))
	appendPath := make([]string, 0, len(packages)*2)

	for _, p := range packages {
		if p.InstallPath == "" {
			continue
		}

		if !utils.IsFileExists(p.InstallPath) {
			continue
		}

		if utils.Contains(currentPath, p.InstallPath) {
			continue
		}

		if utils.Contains(appendPath, p.InstallPath) {
			continue
		}

		appendPath = append(appendPath, p.InstallPath)
	}

	//nolint:nestif
	if utils.IsFileExists(gameap.DefaultToolsPath) {
		entries, err := os.ReadDir(gameap.DefaultToolsPath)
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to read dir"))
		} else {
			if !utils.Contains(currentPath, gameap.DefaultToolsPath) &&
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
		}
	}

	daemonDir := filepath.Dir(gameap.DefaultDaemonFilePath)
	if utils.IsFileExists(daemonDir) {
		if !utils.Contains(currentPath, daemonDir) &&
			!utils.Contains(appendPath, daemonDir) {
			appendPath = append(appendPath, daemonDir)
		}
	}

	newPath := os.Getenv("PATH") +
		string(filepath.ListSeparator) +
		strings.Join(appendPath, string(filepath.ListSeparator))

	log.Println("New PATH:", newPath)
	err = os.Setenv("PATH", newPath)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to set PATH"))
	}
}
