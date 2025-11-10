package packagemanager

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"strings"

	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/gameap/gameapctl/pkg/oscore"
	pmdnf "github.com/gameap/gameapctl/pkg/package_manager/dnf"
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
	packages   map[string]pmdnf.PackageConfig
	underlined PackageManager
}

func newExtendedDNF(osinfo osinfo.Info, underlined PackageManager) (*extendedDNF, error) {
	packages, err := pmdnf.LoadPackages(osinfo)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to load dnf packages configuration")
	}

	return &extendedDNF{
		packages:   packages,
		underlined: underlined,
	}, nil
}

func (d *extendedDNF) Install(ctx context.Context, packs ...string) error {
	var err error

	packs, err = d.excludeByLookupPathFound(ctx, packs...)
	if err != nil {
		return errors.WithMessage(err, "failed to check lookup paths")
	}

	if len(packs) == 0 {
		return nil
	}

	packs, err = d.preInstallationSteps(ctx, packs...)
	if err != nil {
		return errors.WithMessage(err, "failed to run pre-installation steps")
	}

	packs = d.replaceAliases(ctx, packs)

	err = d.underlined.Install(ctx, packs...)
	if err != nil {
		return errors.WithMessage(err, "failed to install packages")
	}

	err = d.postInstallationSteps(ctx, packs...)
	if err != nil {
		return errors.WithMessage(err, "failed to run post-installation steps")
	}

	return nil
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

func (d *extendedDNF) replaceAliases(_ context.Context, packs []string) []string {
	updatedPacks := make([]string, 0, len(packs))

	for _, packName := range packs {
		if config, exists := d.packages[packName]; exists && config.ReplaceWith != nil {
			updatedPacks = append(updatedPacks, config.ReplaceWith...)
		} else {
			updatedPacks = append(updatedPacks, packName)
		}
	}

	return updatedPacks
}

func (d *extendedDNF) excludeByLookupPathFound(_ context.Context, packs ...string) ([]string, error) {
	filteredPacks := make([]string, 0, len(packs))

	for _, packName := range packs {
		config, exists := d.packages[packName]
		if !exists || len(config.LookupPaths) == 0 {
			filteredPacks = append(filteredPacks, packName)

			continue
		}

		found := false
		for _, lookupPath := range config.LookupPaths {
			if _, err := exec.LookPath(lookupPath); err == nil {
				found = true

				break
			}
		}

		if !found {
			filteredPacks = append(filteredPacks, packName)
		}
	}

	return filteredPacks, nil
}

func (d *extendedDNF) executePreInstallationSteps(ctx context.Context, packs []string) error {
	executedPackages := make(map[string]bool)

	for _, packName := range packs {
		config, exists := d.packages[packName]
		if !exists {
			continue
		}

		if len(config.PreInstall) == 0 {
			continue
		}

		if executedPackages[packName] {
			continue
		}

		for _, step := range config.PreInstall {
			if !d.checkConditions(step.Conditions) {
				continue
			}

			for _, cmd := range step.RunCommands {
				if err := d.executeCommand(ctx, cmd); err != nil {
					return errors.WithMessagef(
						err,
						"failed to execute pre-install command for %s: %s", packName, cmd,
					)
				}
			}
		}

		executedPackages[packName] = true
	}

	return nil
}

func (d *extendedDNF) checkConditions(conditions []pmdnf.Condition) bool {
	if len(conditions) == 0 {
		return true
	}

	for _, condition := range conditions {
		if condition.FileNotExists != "" {
			if _, err := os.Stat(condition.FileNotExists); err == nil {
				return false
			}
		}
	}

	return true
}

func (d *extendedDNF) executeInstallationSteps(
	ctx context.Context,
	packs []string,
	getCommands func(pmdnf.PackageConfig) []string,
) error {
	executedPackages := make(map[string]bool)

	for _, packName := range packs {
		config, exists := d.packages[packName]
		if !exists {
			continue
		}

		commands := getCommands(config)
		if len(commands) == 0 {
			continue
		}

		if executedPackages[packName] {
			continue
		}

		for _, cmd := range commands {
			if err := d.executeCommand(ctx, cmd); err != nil {
				return errors.WithMessagef(
					err,
					"failed to execute command for %s: %s", packName, cmd,
				)
			}
		}

		executedPackages[packName] = true
	}

	return nil
}

func (d *extendedDNF) preInstallationSteps(ctx context.Context, packs ...string) ([]string, error) {
	err := d.executePreInstallationSteps(ctx, packs)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to run pre-installation steps")
	}

	return packs, nil
}

func (d *extendedDNF) postInstallationSteps(ctx context.Context, packs ...string) error {
	err := d.executeInstallationSteps(
		ctx,
		packs,
		func(config pmdnf.PackageConfig) []string { return config.PostInstall },
	)
	if err != nil {
		return errors.WithMessage(err, "failed to run post-installation steps")
	}

	return nil
}

func (d *extendedDNF) executeCommand(ctx context.Context, cmdStr string) error {
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return nil
	}

	command := "bash"

	if _, err := exec.LookPath(command); err != nil {
		command = "sh"
	}

	args := []string{"-c", cmdStr}

	return oscore.ExecCommand(ctx, command, args...)
}
