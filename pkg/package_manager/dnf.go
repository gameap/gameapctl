package packagemanager

import (
	"bufio"
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

	return parseDnfInfoOutput(out)
}

func parseDnfInfoOutput(out []byte) ([]PackageInfo, error) {
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
		case "Version":
			currentPackage.Version = value
		case "Architecture":
			currentPackage.Architecture = value
		case "Size":
			currentPackage.Size = value
		case "Description":
			currentPackage.Description = value
		case "":
			if value != "" && currentPackage != nil {
				currentPackage.Description += " " + value
			}
		}
	}

	if currentPackage != nil {
		packages = append(packages, *currentPackage)
	}

	return packages, nil
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
	*dnf
}

func newExtendedDNF(d *dnf) *extendedDNF {
	return &extendedDNF{d}
}

func (d *extendedDNF) Install(ctx context.Context, packs ...string) error {
	var err error
	packs = d.replaceAliases(ctx, packs)

	packs, err = d.preInstallationSteps(ctx, packs...)
	if err != nil {
		return errors.WithMessage(err, "failed to run pre-installation steps")
	}

	return d.dnf.Install(ctx, packs...)
}

func (d *extendedDNF) CheckForUpdates(ctx context.Context) error {
	return d.dnf.CheckForUpdates(ctx)
}

func (d *extendedDNF) Remove(ctx context.Context, packs ...string) error {
	packs = d.replaceAliases(ctx, packs)

	return d.dnf.Remove(ctx, packs...)
}

func (d *extendedDNF) Purge(ctx context.Context, packs ...string) error {
	packs = d.replaceAliases(ctx, packs)

	return d.dnf.Purge(ctx, packs...)
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
		//nolint:gocritic
		switch pack {
		case PHPPackage:
			err := d.addPHPRepository(ctx)
			if err != nil {
				return nil, errors.WithMessage(err, "failed to add PHP repository")
			}

			updatedPacks = append(updatedPacks, pack)
		}
	}

	return updatedPacks, nil
}

func (d *extendedDNF) addPHPRepository(ctx context.Context) error {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	//nolint:gocritic
	switch {
	case osInfo.Distribution == DistributionCentOS && osInfo.DistributionCodename == "8":
		err := utils.ExecCommand("dnf", "-y", "install", "https://rpms.remirepo.net/enterprise/remi-release-8.rpm")
		if err != nil {
			return errors.WithMessage(err, "failed to install remirepo")
		}

		err = utils.ExecCommand("dnf", "-y", "switch-to", "php:remi-8.2")
		if err != nil {
			return errors.WithMessage(err, "failed to switch to remirepo")
		}
	}

	return nil
}
