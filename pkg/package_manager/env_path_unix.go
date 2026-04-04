//go:build linux || darwin

package packagemanager

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	internalcontext "github.com/gameap/gameapctl/internal/context"
	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	pmapt "github.com/gameap/gameapctl/pkg/package_manager/apt"
	pmdnf "github.com/gameap/gameapctl/pkg/package_manager/dnf"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func UpdateEnvPath(ctx context.Context) {
	osInfo := internalcontext.OSInfoFromContext(ctx)

	currentPath := strings.Split(os.Getenv("PATH"), string(filepath.ListSeparator))
	appendPath := make([]string, 0)

	appendPath = collectAPTPathEnv(osInfo, currentPath, appendPath)
	appendPath = collectDNFPathEnv(osInfo, currentPath, appendPath)

	if len(appendPath) == 0 {
		return
	}

	newPath := os.Getenv("PATH") +
		string(filepath.ListSeparator) +
		strings.Join(appendPath, string(filepath.ListSeparator))

	log.Println("New PATH:", newPath)
	err := os.Setenv("PATH", newPath)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to set PATH"))
	}
}

func collectAPTPathEnv(osinf osinfo.Info, currentPath, appendPath []string) []string {
	packages, err := pmapt.LoadPackages(osinf)
	if err != nil {
		return appendPath
	}

	for _, p := range packages {
		appendPath = collectPathEnvEntries(p.PathEnv, currentPath, appendPath)
	}

	return appendPath
}

func collectDNFPathEnv(osinf osinfo.Info, currentPath, appendPath []string) []string {
	packages, err := pmdnf.LoadPackages(osinf)
	if err != nil {
		return appendPath
	}

	for _, p := range packages {
		appendPath = collectPathEnvEntries(p.PathEnv, currentPath, appendPath)
	}

	return appendPath
}

func collectPathEnvEntries(pathEnvs, currentPath, appendPath []string) []string {
	for _, pathEnv := range pathEnvs {
		if pathEnv == "" || !utils.IsFileExists(pathEnv) {
			continue
		}
		if utils.Contains(currentPath, pathEnv) || utils.Contains(appendPath, pathEnv) {
			continue
		}
		appendPath = append(appendPath, pathEnv)
	}

	return appendPath
}
