package packagemanager

import (
	"bufio"
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/gameap/gameapctl/pkg/package_manager/pkgconfig"
	"github.com/pkg/errors"
)

const (
	sourcesListNginx = "/etc/apt/sources.list.d/nginx.list"
	sourcesListPHP   = "/etc/apt/sources.list.d/php.list"
	sourcesListNode  = "/etc/apt/sources.list.d/nodesource.list"
)

type apt struct{}

// Search list packages available in the system that match the search
// pattern.
func (apt *apt) Search(_ context.Context, packName string) ([]PackageInfo, error) {
	search, err := aptListSearch(packName)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to search package")
	}

	if len(search) == 0 {
		// Fall back to apt-cache search
		log.Println("Package not found using inner apt package. Running apt-cache search")

		return apt.searchAptCache(context.Background(), packName)
	}

	result := make([]PackageInfo, 0, len(search))

	for _, p := range search {
		installedSize, err := strconv.Atoi(p.InstalledSize)
		if err != nil {
			// Ignore error
			installedSize = 0
		}

		result = append(result, PackageInfo{
			Name:            p.PackageName,
			Architecture:    p.Architecture,
			Version:         p.Version,
			Size:            p.Size,
			Description:     p.Description,
			InstalledSizeKB: installedSize,
		})
	}

	return result, nil
}

func (apt *apt) searchAptCache(_ context.Context, packName string) ([]PackageInfo, error) {
	cmd := exec.Command(
		"apt-cache",
		"show",
		packName,
	)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "DEBIAN_FRONTEND=noninteractive")

	out, err := cmd.CombinedOutput()
	log.Print(string(out))
	if err != nil {
		// Avoid returning an error if the list is empty
		if bytes.Contains(out, []byte("E: No packages found")) {
			return []PackageInfo{}, nil
		}

		return nil, errors.WithMessage(err, "failed to run apt-cache")
	}

	return parseAPTCacheShowOutput(out), nil
}

func parseAPTCacheShowOutput(out []byte) []PackageInfo {
	scanner := bufio.NewScanner(bytes.NewReader(out))

	var packageInfos []PackageInfo

	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 2)
		if len(parts) < 2 {
			continue
		}

		info := PackageInfo{}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "PackageInfo":
			info.Name = value
		case fieldArchitecture:
			info.Architecture = value
		case fieldVersion:
			info.Version = value
		case fieldSize:
			info.Size = value
		case fieldDescription:
			info.Description = value
		case "Installed-Size":
			size, err := strconv.Atoi(value)
			if err != nil {
				// Ignore error
				size = 0
			}
			info.InstalledSizeKB = size
		}

		packageInfos = append(packageInfos, info)
	}

	return packageInfos
}

// CheckForUpdates runs an apt update to retrieve new packages available
// from the repositories.
func (apt *apt) CheckForUpdates(_ context.Context) error {
	cmd := exec.Command("apt-get", "update", "-q")

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "DEBIAN_FRONTEND=noninteractive")

	log.Println('\n', cmd.String())
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()

	return cmd.Run()
}

func (apt *apt) Install(_ context.Context, pack string, _ ...InstallOptions) error {
	if pack == "" || pack == " " {
		return nil
	}

	args := []string{"install", "-y", pack}
	cmd := exec.Command("apt-get", args...)

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "DEBIAN_FRONTEND=noninteractive")

	log.Println('\n', cmd.String())
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()

	return cmd.Run()
}

// Remove removes a set of packages.
func (apt *apt) Remove(_ context.Context, packs ...string) error {
	args := []string{"remove", "-y"}
	for _, pack := range packs {
		if pack == "" || pack == " " {
			continue
		}
		args = append(args, pack)
	}
	cmd := exec.Command("apt-get", args...)

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "DEBIAN_FRONTEND=noninteractive")

	log.Println('\n', cmd.String())
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()

	return cmd.Run()
}

// Purge removes a set of packages and their configuration.
func (apt *apt) Purge(_ context.Context, packs ...string) error {
	args := []string{"purge", "-y"}
	for _, pack := range packs {
		if pack == "" || pack == " " {
			continue
		}
		args = append(args, pack)
	}
	cmd := exec.Command("apt-get", args...)

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "DEBIAN_FRONTEND=noninteractive")

	log.Println('\n', cmd.String())
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()

	return cmd.Run()
}

func newExtendedAPT(osInfo osinfo.Info, apt *apt) (*extended, error) {
	packages, err := pkgconfig.LoadPackages("apt", osInfo)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to load package configurations")
	}

	return &extended{
		packages:   packages,
		underlined: apt,
		strategy: &aptStrategy{
			packages: packages,
			apt:      apt,
		},
	}, nil
}
