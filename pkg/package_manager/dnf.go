package packagemanager

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"strings"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/gameap/gameapctl/pkg/utils"
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

func (d *dnf) Install(_ context.Context, packs ...string) error {
	args := []string{"install", "-y"}
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

type extendedDNF struct {
	underlined PackageManager
}

func newExtendedDNF(underlined PackageManager) *extendedDNF {
	return &extendedDNF{underlined: underlined}
}

func (d *extendedDNF) Install(ctx context.Context, packs ...string) error {
	var err error

	packs, err = d.preInstallationSteps(ctx, packs...)
	if err != nil {
		return errors.WithMessage(err, "failed to run pre-installation steps")
	}

	packs = d.replaceAliases(ctx, packs)

	return d.underlined.Install(ctx, packs...)
}

func (d *extendedDNF) CheckForUpdates(ctx context.Context) error {
	return d.underlined.CheckForUpdates(ctx)
}

func (d *extendedDNF) Remove(ctx context.Context, packs ...string) error {
	packs = d.replaceAliases(ctx, packs)

	return d.underlined.Remove(ctx, packs...)
}

func (d *extendedDNF) Purge(ctx context.Context, packs ...string) error {
	packs = d.replaceAliases(ctx, packs)

	return d.underlined.Purge(ctx, packs...)
}

func (d *extendedDNF) Search(ctx context.Context, name string) ([]PackageInfo, error) {
	return d.underlined.Search(ctx, name)
}

func (d *extendedDNF) replaceAliases(ctx context.Context, packs []string) []string {
	return replaceAliases(ctx, dnfPackageAliases, packs)
}

func replaceAliases(ctx context.Context, aliasesMap distVersionPackagesMap, packs []string) []string {
	replacedPacks := make([]string, 0, len(packs))

	osInfo := contextInternal.OSInfoFromContext(ctx)

	for _, pack := range packs {
		if aliases, exists :=
			aliasesMap[osInfo.Distribution][osInfo.DistributionCodename][osInfo.Platform][pack]; exists {
			replacedPacks = append(replacedPacks, aliases...)
		} else if aliases, exists =
			aliasesMap[osInfo.Distribution][osInfo.DistributionCodename][Default][pack]; exists {
			replacedPacks = append(replacedPacks, aliases...)
		} else if aliases, exists =
			aliasesMap[osInfo.Distribution][Default][Default][pack]; exists {
			replacedPacks = append(replacedPacks, aliases...)
		} else if aliases, exists =
			aliasesMap[Default][Default][Default][pack]; exists {
			replacedPacks = append(replacedPacks, aliases...)
		} else {
			replacedPacks = append(replacedPacks, pack)
		}
	}

	return replacedPacks
}

func (d *extendedDNF) preInstallationSteps(ctx context.Context, packs ...string) ([]string, error) {
	updatedPacks := make([]string, 0, len(packs))

	for _, pack := range packs {
		switch pack {
		case PHPPackage:
			err := d.addPHPRepository(ctx)
			if err != nil {
				return nil, errors.WithMessage(err, "failed to add PHP repository")
			}

			updatedPacks = append(updatedPacks, pack)
		default:
			updatedPacks = append(updatedPacks, pack)
		}
	}

	return updatedPacks, nil
}

func (d *extendedDNF) addPHPRepository(ctx context.Context) error {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	switch {
	case osInfo.Distribution == DistributionCentOS && osInfo.DistributionCodename == "8":
		err := utils.ExecCommand("dnf", "-y", "install", "https://rpms.remirepo.net/enterprise/remi-release-8.rpm")
		if err != nil {
			return errors.WithMessage(err, "failed to install remirepo")
		}

		err = utils.ExecCommand("dnf", "-y", "module", "switch-to", "php:remi-8.2")
		if err != nil {
			return errors.WithMessage(err, "failed to switch to remirepo")
		}
	case osInfo.Distribution == DistributionCentOS && osInfo.DistributionCodename == "7":
		repolistOut, err := utils.ExecCommandWithOutput("yum", "repolist")
		if err != nil {
			return errors.WithMessage(err, "failed to get repolist")
		}

		if strings.Contains(repolistOut, "remi-php82") {
			return nil
		}

		err = utils.ExecCommand("yum", "-y", "install", "https://rpms.remirepo.net/enterprise/remi-release-7.rpm")
		if err != nil {
			return errors.WithMessage(err, "failed to install remirepo")
		}

		err = utils.ExecCommand("yum", "-y", "install", "yum-utils")
		if err != nil {
			return errors.WithMessage(err, "failed to install yum-utils")
		}

		err = utils.ExecCommand("yum-config-manager", "--enable", "remi-php82")
		if err != nil {
			return errors.WithMessage(err, "failed to enable remi-php82 repository")
		}
	}

	return nil
}
