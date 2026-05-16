package packagemanager

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"

	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/gameap/gameapctl/pkg/package_manager/pkgconfig"
	"github.com/pkg/errors"
)

type dnf struct{}

func (d *dnf) Search(_ context.Context, name string) ([]PackageInfo, error) {
	cmd := exec.Command("dnf", "info", name)
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	log.Print(string(out))
	if err != nil {
		if bytes.Contains(out, []byte("Error: No matching Packages to list")) {
			return []PackageInfo{}, nil
		}

		return nil, err
	}

	return parseYumInfoOutput(out)
}

func (d *dnf) Install(_ context.Context, pack string, _ ...InstallOptions) error {
	if pack == "" || pack == " " {
		return nil
	}

	args := []string{"install", "-y", pack}
	cmd := exec.Command("dnf", args...)

	cmd.Env = os.Environ()

	log.Println('\n', cmd.String())
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()

	return cmd.Run()
}

func (d *dnf) CheckForUpdates(_ context.Context) error {
	return nil
}

func (d *dnf) Remove(_ context.Context, packs ...string) error {
	args := []string{"remove", "-y"}
	for _, pack := range packs {
		if pack == "" || pack == " " {
			continue
		}
		args = append(args, pack)
	}
	cmd := exec.Command("dnf", args...)

	cmd.Env = os.Environ()

	log.Println('\n', cmd.String())
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()

	return cmd.Run()
}

func (d *dnf) Purge(ctx context.Context, packs ...string) error {
	return d.Remove(ctx, packs...)
}

func newExtendedDNF(osInfo osinfo.Info, underlined PackageManager) (*extended, error) {
	packages, err := pkgconfig.LoadPackages("dnf", osInfo)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to load dnf packages configuration")
	}

	return &extended{
		packages:   packages,
		underlined: underlined,
		strategy:   noopStrategy{},
	}, nil
}
