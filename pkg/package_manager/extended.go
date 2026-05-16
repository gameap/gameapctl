package packagemanager

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/package_manager/pkgconfig"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

// packageInstallStrategy encapsulates the package-manager-specific behaviour
// that differs between apt and dnf/yum. apt provides aptStrategy; dnf and yum
// use noopStrategy.
type packageInstallStrategy interface {
	installDependencies(ctx context.Context, packs ...string) error
	transformPreInstallPackages(ctx context.Context, packs []string) ([]string, error)
	preRemove(ctx context.Context, packs ...string) error
}

type noopStrategy struct{}

func (noopStrategy) installDependencies(_ context.Context, _ ...string) error {
	return nil
}

func (noopStrategy) transformPreInstallPackages(_ context.Context, packs []string) ([]string, error) {
	return packs, nil
}

func (noopStrategy) preRemove(_ context.Context, _ ...string) error {
	return nil
}

// extended wraps a base PackageManager with YAML-driven pre/install/post
// steps shared by apt, dnf and yum.
type extended struct {
	packages   map[string]pkgconfig.PackageConfig
	underlined PackageManager
	strategy   packageInstallStrategy
}

func (e *extended) Search(ctx context.Context, name string) ([]PackageInfo, error) {
	return e.underlined.Search(ctx, name)
}

func (e *extended) Install(ctx context.Context, pack string, opts ...InstallOptions) error {
	options := &installOptions{}
	for _, opt := range opts {
		opt(options)
	}

	packs, err := e.excludeByLookupPathFound(ctx, pack)
	if err != nil {
		return errors.WithMessage(err, "failed to check lookup paths")
	}

	if len(packs) == 0 {
		return nil
	}

	err = e.strategy.installDependencies(ctx, packs...)
	if err != nil {
		return errors.WithMessage(err, "failed to install dependencies")
	}

	packs, err = e.preInstallationSteps(ctx, lo.Uniq(append(packs, pack)), options)
	if err != nil {
		return errors.WithMessage(err, "failed to run pre-installation steps")
	}

	packs = e.replaceAliases(ctx, packs)

	packs, err = e.executeInstallSteps(ctx, packs, options)
	if err != nil {
		return errors.WithMessage(err, "failed to run install steps")
	}

	for _, p := range packs {
		err = e.underlined.Install(ctx, p, opts...)
		if err != nil {
			return errors.WithMessage(err, "failed to install packages")
		}
	}

	err = e.postInstallationSteps(ctx, lo.Uniq(append(packs, pack)), options)
	if err != nil {
		return errors.WithMessage(err, "failed to run post-installation steps")
	}

	return nil
}

func (e *extended) CheckForUpdates(ctx context.Context) error {
	return e.underlined.CheckForUpdates(ctx)
}

func (e *extended) Remove(ctx context.Context, packs ...string) error {
	err := e.strategy.preRemove(ctx, packs...)
	if err != nil {
		return errors.WithMessage(err, "failed preRemovingSteps")
	}
	packs = e.replaceAliases(ctx, packs)

	return e.underlined.Remove(ctx, packs...)
}

func (e *extended) Purge(ctx context.Context, packs ...string) error {
	err := e.strategy.preRemove(ctx, packs...)
	if err != nil {
		return errors.WithMessage(err, "failed preRemovingSteps")
	}
	packs = e.replaceAliases(ctx, packs)

	return e.underlined.Purge(ctx, packs...)
}

func (e *extended) replaceAliases(_ context.Context, packs []string) []string {
	replacedPacks := make([]string, 0, len(packs))

	for _, packName := range packs {
		if pkgConfig, exists := e.packages[packName]; exists && pkgConfig.ReplaceWith != nil {
			replacedPacks = append(replacedPacks, pkgConfig.ReplaceWith...)
		} else {
			replacedPacks = append(replacedPacks, packName)
		}
	}

	return replacedPacks
}

//nolint:unparam
func (e *extended) excludeByLookupPathFound(_ context.Context, packs ...string) ([]string, error) {
	filteredPacks := make([]string, 0, len(packs))

	for _, packName := range packs {
		config, exists := e.packages[packName]
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

func (e *extended) preInstallationSteps(
	ctx context.Context, packs []string, options *installOptions,
) ([]string, error) {
	err := e.executePreInstallationSteps(ctx, packs, options)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to run pre-installation steps")
	}

	return e.strategy.transformPreInstallPackages(ctx, packs)
}

func (e *extended) executePreInstallationSteps(ctx context.Context, packs []string, options *installOptions) error {
	executedPackages := make(map[string]bool)

	for _, packName := range packs {
		config, exists := e.packages[packName]
		if !exists {
			continue
		}

		if len(config.PreInstall) == 0 {
			continue
		}

		if executedPackages[packName] {
			continue
		}

		runtimeVars := extendedRuntimeTemplateVariables{
			LookupPaths: make(map[string]string, len(config.LookupPaths)),
			Options:     options,
		}

		for _, lookupPath := range config.LookupPaths {
			if resolvedPath, err := exec.LookPath(lookupPath); err == nil {
				runtimeVars.LookupPaths[lookupPath] = resolvedPath
			}
		}

		for _, step := range config.PreInstall {
			if !e.checkConditions(step.Conditions) {
				continue
			}

			for _, cmd := range step.RunCommands {
				processedCmd, err := e.replaceRuntimeVariablesString(ctx, cmd, runtimeVars)
				if err != nil {
					return errors.WithMessagef(
						err,
						"failed to replace runtime variables in pre-install command for %s: %s", packName, cmd,
					)
				}

				if err := e.executeCommand(ctx, processedCmd); err != nil {
					return errors.WithMessagef(
						err,
						"failed to execute pre-install command for %s: %s", packName, processedCmd,
					)
				}
			}
		}

		executedPackages[packName] = true
	}

	return nil
}

func (e *extended) executeInstallSteps(
	ctx context.Context, packs []string, options *installOptions,
) ([]string, error) {
	executedPackages := make(map[string]bool)
	packsToInstall := make([]string, 0, len(packs))

	for _, packName := range packs {
		config, exists := e.packages[packName]
		if !exists || len(config.Install) == 0 {
			packsToInstall = append(packsToInstall, packName)

			continue
		}

		if executedPackages[packName] {
			continue
		}

		runtimeVars := extendedRuntimeTemplateVariables{
			LookupPaths: make(map[string]string, len(config.LookupPaths)),
			Options:     options,
		}

		for _, lookupPath := range config.LookupPaths {
			if resolvedPath, err := exec.LookPath(lookupPath); err == nil {
				runtimeVars.LookupPaths[lookupPath] = resolvedPath
			}
		}

		for _, step := range config.Install {
			for _, cmd := range step.RunCommands {
				processedCmd, err := e.replaceRuntimeVariablesString(ctx, cmd, runtimeVars)
				if err != nil {
					return nil, errors.WithMessagef(
						err,
						"failed to replace runtime variables in install command for %s: %s", packName, cmd,
					)
				}

				if err := e.executeCommand(ctx, processedCmd); err != nil {
					return nil, errors.WithMessagef(
						err,
						"failed to execute install command for %s: %s", packName, processedCmd,
					)
				}
			}
		}

		executedPackages[packName] = true
	}

	return packsToInstall, nil
}

func (e *extended) postInstallationSteps(ctx context.Context, packs []string, options *installOptions) error {
	executedPackages := make(map[string]bool)

	for _, packName := range packs {
		config, exists := e.packages[packName]
		if !exists {
			continue
		}

		if len(config.PostInstall) == 0 {
			continue
		}

		if executedPackages[packName] {
			continue
		}

		runtimeVars := extendedRuntimeTemplateVariables{
			LookupPaths: make(map[string]string, len(config.LookupPaths)),
			Options:     options,
		}

		for _, lookupPath := range config.LookupPaths {
			if resolvedPath, err := exec.LookPath(lookupPath); err == nil {
				runtimeVars.LookupPaths[lookupPath] = resolvedPath
			}
		}

		for _, step := range config.PostInstall {
			for _, cmd := range step.RunCommands {
				processedCmd, err := e.replaceRuntimeVariablesString(ctx, cmd, runtimeVars)
				if err != nil {
					return errors.WithMessagef(
						err,
						"failed to replace runtime variables in post-install command for %s: %s", packName, cmd,
					)
				}

				log.Println("Running post-install command:", processedCmd)

				if err := e.executeCommand(ctx, processedCmd); err != nil {
					return errors.WithMessagef(
						err,
						"failed to execute post-install command for %s: %s", packName, processedCmd,
					)
				}
			}
		}

		executedPackages[packName] = true
	}

	return nil
}

func (e *extended) checkConditions(conditions []pkgconfig.Condition) bool {
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

func (e *extended) executeCommand(ctx context.Context, cmdStr string) error {
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

type extendedRuntimeTemplateVariables struct {
	LookupPaths map[string]string
	Options     *installOptions
}

func (e *extended) replaceRuntimeVariablesString(
	_ context.Context, v string, vars extendedRuntimeTemplateVariables,
) (string, error) {
	funcMap := template.FuncMap{
		"configValue": func(name string) string {
			if vars.Options == nil {
				return ""
			}

			val, exists := vars.Options.configValues[name]
			if !exists {
				return ""
			}

			return val
		},
	}

	tmpl, err := template.New("package").Funcs(runtimeTemplateFuncMap).Funcs(funcMap).Parse(v)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse template")
	}

	var buf bytes.Buffer
	buf.Grow(len(v) + 100) //nolint:mnd

	err = tmpl.Execute(&buf, vars)
	if err != nil {
		return "", errors.Wrap(err, "failed to execute template")
	}

	return buf.String(), nil
}
