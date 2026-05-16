package packagemanager

import (
	"bufio"
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"strings"

	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/gameap/gameapctl/pkg/package_manager/pkgconfig"
	"github.com/pkg/errors"
)

type pacman struct{}

func (p *pacman) Search(_ context.Context, name string) ([]PackageInfo, error) {
	cmd := exec.Command("pacman", "-Si", name)
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	log.Print(string(out))
	if err != nil {
		if bytes.Contains(out, []byte("was not found")) {
			return []PackageInfo{}, nil
		}

		return nil, err
	}

	return parsePacmanInfoOutput(out), nil
}

func parsePacmanInfoOutput(out []byte) []PackageInfo {
	scanner := bufio.NewScanner(bytes.NewReader(out))

	var packages []PackageInfo
	var currentPackage *PackageInfo

	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 2)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Name":
			if currentPackage != nil {
				packages = append(packages, *currentPackage)
			}
			currentPackage = &PackageInfo{}

			currentPackage.Name = value
		case fieldVersion:
			if currentPackage != nil {
				currentPackage.Version = value
			}
		case fieldArchitecture:
			if currentPackage != nil {
				currentPackage.Architecture = value
			}
		case fieldDescription:
			if currentPackage != nil {
				currentPackage.Description = value
			}
		case "Download Size":
			if currentPackage != nil {
				currentPackage.Size = value
			}
		}
	}

	if currentPackage != nil {
		packages = append(packages, *currentPackage)
	}

	return packages
}

func (p *pacman) Install(_ context.Context, pack string, _ ...InstallOptions) error {
	if pack == "" || pack == " " {
		return nil
	}

	args := []string{"-S", "--noconfirm", "--needed", pack}
	cmd := exec.Command("pacman", args...)

	cmd.Env = os.Environ()

	log.Println("\n", cmd.String())
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()

	return cmd.Run()
}

func (p *pacman) CheckForUpdates(_ context.Context) error {
	cmd := exec.Command("pacman", "-Sy", "--noconfirm")

	cmd.Env = os.Environ()

	log.Println("\n", cmd.String())
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()

	return cmd.Run()
}

func (p *pacman) Remove(_ context.Context, packs ...string) error {
	args := []string{"-R", "--noconfirm"}
	for _, pack := range packs {
		if pack == "" || pack == " " {
			continue
		}
		args = append(args, pack)
	}
	cmd := exec.Command("pacman", args...)

	cmd.Env = os.Environ()

	log.Println("\n", cmd.String())
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()

	return cmd.Run()
}

func (p *pacman) Purge(ctx context.Context, packs ...string) error {
	return p.Remove(ctx, packs...)
}

func newExtendedPacman(osInfo osinfo.Info, underlined PackageManager) (*extended, error) {
	packages, err := pkgconfig.LoadPackages("pacman", osInfo)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to load pacman packages configuration")
	}

	return &extended{
		packages:   packages,
		underlined: underlined,
		strategy:   noopStrategy{},
	}, nil
}
