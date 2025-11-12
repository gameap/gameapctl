package packagemanager

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/template"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/gameap/gameapctl/pkg/oscore"
	pmapt "github.com/gameap/gameapctl/pkg/package_manager/apt"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/samber/lo"
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
	search, err := pmapt.Search(packName)
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
		case "Architecture":
			info.Architecture = value
		case "Version":
			info.Version = value
		case "Size":
			info.Size = value
		case "Description":
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

// Remove removes a set of packages.
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

type extendedAPT struct {
	packages map[string]pmapt.PackageConfig
	apt      *apt
}

func newExtendedAPT(osinfo osinfo.Info, apt *apt) (*extendedAPT, error) {
	packages, err := pmapt.LoadPackages(osinfo)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to load package configurations")
	}

	return &extendedAPT{
		packages: packages,
		apt:      apt,
	}, nil
}

func (e *extendedAPT) Search(ctx context.Context, name string) ([]PackageInfo, error) {
	return e.apt.Search(ctx, name)
}

func (e *extendedAPT) Install(ctx context.Context, pack string, opts ...InstallOptions) error {
	var err error

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

	err = e.installDependencies(ctx, packs...)
	if err != nil {
		return errors.WithMessage(err, "failed to install dependencies")
	}

	packs, err = e.preInstallationSteps(ctx, lo.Uniq(append(packs, pack)), options)
	if err != nil {
		return errors.WithMessage(err, "failed to run pre installation steps")
	}

	packs = e.replaceAliases(ctx, packs)

	for _, p := range packs {
		err = e.apt.Install(ctx, p, opts...)
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

func (e *extendedAPT) CheckForUpdates(ctx context.Context) error {
	return e.apt.CheckForUpdates(ctx)
}

func (e *extendedAPT) Remove(ctx context.Context, packs ...string) error {
	err := e.preRemovingSteps(ctx, packs...)
	if err != nil {
		return errors.WithMessage(err, "failed preRemovingSteps")
	}
	packs = e.replaceAliases(ctx, packs)

	return e.apt.Remove(ctx, packs...)
}

func (e *extendedAPT) Purge(ctx context.Context, packs ...string) error {
	err := e.preRemovingSteps(ctx, packs...)
	if err != nil {
		return errors.WithMessage(err, "failed preRemovingSteps")
	}
	packs = e.replaceAliases(ctx, packs)

	return e.apt.Purge(ctx, packs...)
}

func (e *extendedAPT) replaceAliases(_ context.Context, packs []string) []string {
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
func (e *extendedAPT) excludeByLookupPathFound(_ context.Context, packs ...string) ([]string, error) {
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

func (e *extendedAPT) installDependencies(ctx context.Context, packs ...string) error {
	dependencies := make([]string, 0)

	for _, packName := range packs {
		config, exists := e.packages[packName]
		if !exists {
			continue
		}

		dependencies = append(dependencies, config.Dependencies...)
	}

	if len(dependencies) == 0 {
		return nil
	}

	for _, dep := range dependencies {
		err := e.apt.Install(ctx, dep)
		if err != nil {
			return errors.WithMessage(err, "failed to install dependencies")
		}
	}

	return nil
}

func (e *extendedAPT) preInstallationSteps(
	ctx context.Context, packs []string, options *installOptions,
) ([]string, error) {
	err := e.executePreInstallationSteps(ctx, packs, options)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to run pre-installation steps")
	}

	updatedPacks := make([]string, 0, len(packs))

	// Hardcode. To be refactored later.
	for _, pack := range packs {
		switch pack {
		case PHPPackage, PHPExtensionsPackage:
			err := e.installAPTRepositoriesDependencies(ctx)
			if err != nil {
				return nil, err
			}

			packages, err := e.findPHPPackages(ctx)
			if err != nil {
				return nil, err
			}

			updatedPacks = append(updatedPacks, packages...)
		case ApachePackage:
			err := e.apachePackageProcess(ctx)
			if err != nil {
				return nil, errors.WithMessage(err, "failed to process apache packages")
			}
		default:
			updatedPacks = append(updatedPacks, pack)
		}
	}

	return updatedPacks, nil
}

func (e *extendedAPT) postInstallationSteps(ctx context.Context, packs []string, options *installOptions) error {
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

		runtimeVars := aptRuntimeTemplateVariables{
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

func (e *extendedAPT) executePreInstallationSteps(ctx context.Context, packs []string, options *installOptions) error {
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

		runtimeVars := aptRuntimeTemplateVariables{
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

func (e *extendedAPT) checkConditions(conditions []pmapt.Condition) bool {
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

func (e *extendedAPT) executeCommand(ctx context.Context, cmdStr string) error {
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

func (e *extendedAPT) preRemovingSteps(ctx context.Context, packs ...string) error {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	for _, pack := range packs {
		if pack == MySQLServerPackage &&
			(osInfo.Distribution == DistributionUbuntu || osInfo.Distribution == DistributionDebian) {
			err := e.apt.Purge(ctx, "mysql*")
			if err != nil {
				return errors.WithMessage(err, "failed to purge mysql packages")
			}
		}
	}

	return nil
}

func (e *extendedAPT) installAPTRepositoriesDependencies(ctx context.Context) error {
	installPackages := make([]string, 0, 2)

	pk, err := e.apt.Search(ctx, "software-properties-common")
	if err != nil {
		return err
	}
	if len(pk) > 0 {
		installPackages = append(installPackages, "software-properties-common")
	}

	pk, err = e.apt.Search(ctx, "apt-transport-https")
	if err != nil {
		return err
	}
	if len(pk) > 0 {
		installPackages = append(installPackages, "apt-transport-https")
	}

	for _, pkg := range installPackages {
		err = e.apt.Install(ctx, pkg)
		if err != nil {
			return err
		}
	}

	return nil
}

//nolint:gocognit,funlen
func (e *extendedAPT) findPHPPackages(ctx context.Context) ([]string, error) {
	var versionAvailable string
	log.Println("Searching for PHP packages...")

	var repoAdded bool
	for {
		log.Println("Checking for PHP 8.4 version availability...")
		pk, err := e.apt.Search(ctx, "php8.4")
		if err != nil {
			return nil, err
		}
		if len(pk) > 0 {
			versionAvailable = "8.4"
			log.Println("PHP 8.4 version found")

			break
		}

		log.Println("PHP 8.4 version not found")

		log.Println("Checking for PHP 8.3 version availability...")
		pk, err = e.apt.Search(ctx, "php8.3")
		if err != nil {
			return nil, err
		}
		if len(pk) > 0 {
			versionAvailable = "8.3"
			log.Println("PHP 8.3 version found")

			break
		}

		log.Println("PHP 8.3 version not found")

		log.Println("Checking for PHP 8.2 version availability...")
		pk, err = e.apt.Search(ctx, "php8.2")
		if err != nil {
			return nil, err
		}
		if len(pk) > 0 {
			versionAvailable = "8.2"
			log.Println("PHP 8.2 version found")

			break
		}

		log.Println("PHP 8.2 version not found")

		log.Println("Checking for PHP 8.1 version availability...")
		pk, err = e.apt.Search(ctx, "php8.1")
		if err != nil {
			return nil, err
		}
		if len(pk) > 0 {
			versionAvailable = "8.1"
			log.Println("PHP 8.1 version found")

			break
		}

		log.Println("PHP 8.1 version not found")

		pk, err = e.apt.Search(ctx, "php8.0")
		if err != nil {
			return nil, err
		}
		if len(pk) > 0 {
			versionAvailable = "8.0"
			log.Println("PHP 8.0 version found")

			break
		}

		log.Println("PHP 8.0 version not found")

		pk, err = e.apt.Search(ctx, "php7.4")
		if err != nil {
			return nil, err
		}
		if len(pk) > 0 {
			versionAvailable = "7.4"
			log.Println("PHP 7.4 version found")

			break
		}

		log.Println("PHP 7.4 version not found")

		pk, err = e.apt.Search(ctx, "php7.3")
		if err != nil {
			return nil, err
		}
		if len(pk) > 0 {
			versionAvailable = "7.3"
			log.Println("PHP 7.3 version found")

			break
		}

		log.Println("PHP 7.3 version not found")

		if repoAdded {
			return nil, errors.New("php version not found")
		}

		repoAdded, err = e.addPHPRepositories(ctx)
		if err != nil {
			return nil, err
		}
	}

	packages := []string{
		"php" + versionAvailable + "-bcmath",
		"php" + versionAvailable + "-bz2",
		"php" + versionAvailable + "-cli",
		"php" + versionAvailable + "-curl",
		"php" + versionAvailable + "-fpm",
		"php" + versionAvailable + "-gd",
		"php" + versionAvailable + "-gmp",
		"php" + versionAvailable + "-intl",
		"php" + versionAvailable + "-mbstring",
		"php" + versionAvailable + "-mysql",
		"php" + versionAvailable + "-opcache",
		"php" + versionAvailable + "-pgsql",
		"php" + versionAvailable + "-sqlite",
		"php" + versionAvailable + "-readline",
		"php" + versionAvailable + "-xml",
		"php" + versionAvailable + "-zip",
	}

	return packages, nil
}

func (e *extendedAPT) addPHPRepositories(ctx context.Context) (bool, error) {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	if osInfo.Distribution == DistributionUbuntu {
		cmd := exec.Command("add-apt-repository", "ppa:ondrej/php")
		cmd.Env = append(cmd.Env, "LC_ALL=C.UTF-8")
		cmd.Stderr = log.Writer()
		cmd.Stdout = log.Writer()
		err := cmd.Run()
		if err != nil {
			return false, err
		}

		return true, nil
	}

	if osInfo.Distribution == DistributionDebian {
		if !utils.IsFileExists(sourcesListPHP) {
			err := utils.WriteContentsToFile(
				[]byte(fmt.Sprintf("deb https://packages.sury.org/php/ %s main", osInfo.DistributionCodename)),
				sourcesListPHP,
			)
			if err != nil {
				return false, err
			}
		}

		err := utils.DownloadFile(ctx, "https://packages.sury.org/php/apt.gpg", "/etc/apt/trusted.gpg.d/php.gpg")
		if err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}

func (e *extendedAPT) apachePackageProcess(ctx context.Context) error {
	phpVersion, err := DefinePHPVersion()
	if err != nil {
		return errors.WithMessage(err, "failed to define php version")
	}

	err = e.apt.Install(ctx, ApachePackage)
	if err != nil {
		return err
	}
	err = e.apt.Install(ctx, "libapache2-mod-php"+phpVersion)
	if err != nil {
		return err
	}

	return nil
}

type aptRuntimeTemplateVariables struct {
	LookupPaths map[string]string
	Options     *installOptions
}

func (e *extendedAPT) replaceRuntimeVariablesString(
	_ context.Context, v string, vars aptRuntimeTemplateVariables,
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
