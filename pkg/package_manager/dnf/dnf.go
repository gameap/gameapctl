package dnf

import (
	"embed"
	"fmt"
	"strings"
	"sync"

	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/goccy/go-yaml"
)

//go:embed *.yaml
var fs embed.FS

type packagesConfig struct {
	Packages []PackageConfig `yaml:"packages"`
}

type Condition struct {
	FileNotExists string `yaml:"file-not-exists"`
}

type PreInstallStep struct {
	Conditions  []Condition `yaml:"conditions,omitempty"`
	RunCommands []string    `yaml:"run-commands,omitempty"`
}

type InstallStep struct {
	RunCommands []string `yaml:"run-commands,omitempty"`
}

type PackageConfig struct {
	Name        string           `yaml:"name"`
	ReplaceWith []string         `yaml:"replace-with"`
	Virtual     bool             `yaml:"virtual"`
	LookupPaths []string         `yaml:"lookup-paths"`
	PreInstall  []PreInstallStep `yaml:"pre-install"`
	Install     []InstallStep    `yaml:"install"`
	PostInstall []string         `yaml:"post-install"`
}

var (
	packageCache      = make(map[string]map[string]PackageConfig)
	packageCacheMutex sync.RWMutex
)

func buildCacheKey(osinf osinfo.Info) string {
	return fmt.Sprintf(
		"%s_%s_%s",
		osinf.Distribution.String(),
		osinf.DistributionVersion,
		osinf.Platform.String(),
	)
}

func LoadPackages(osinf osinfo.Info) (map[string]PackageConfig, error) {
	cacheKey := buildCacheKey(osinf)

	packageCacheMutex.RLock()
	if cached, exists := packageCache[cacheKey]; exists {
		packageCacheMutex.RUnlock()

		return cached, nil
	}
	packageCacheMutex.RUnlock()

	packageCacheMutex.Lock()
	defer packageCacheMutex.Unlock()

	if cached, exists := packageCache[cacheKey]; exists {
		return cached, nil
	}
	packages := make(map[string]PackageConfig)

	distribution := osinf.Distribution.String()
	version := osinf.DistributionVersion
	arch := osinf.Platform.String()

	filesToLoad := []string{
		"default.yaml",
	}

	if arch != "" {
		filesToLoad = append(filesToLoad, fmt.Sprintf("default_%s.yaml", arch))
	}

	if distribution != "" {
		filesToLoad = append(filesToLoad, fmt.Sprintf("%s.yaml", strings.ToLower(distribution)))
	}

	if distribution != "" && version != "" {
		filesToLoad = append(filesToLoad, fmt.Sprintf("%s_%s.yaml", strings.ToLower(distribution), version))
	}

	if distribution != "" && version != "" && arch != "" {
		filesToLoad = append(filesToLoad, fmt.Sprintf("%s_%s_%s.yaml", strings.ToLower(distribution), version, arch))
	}

	for _, filename := range filesToLoad {
		data, err := fs.ReadFile(filename)
		if err != nil {
			continue
		}

		var config packagesConfig
		err = yaml.Unmarshal(data, &config)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s: %w", filename, err)
		}

		for _, pkg := range config.Packages {
			for i := range pkg.PreInstall {
				pkg.PreInstall[i].RunCommands = replaceDistributionVariablesSlice(pkg.PreInstall[i].RunCommands, osinf)
			}
			for i := range pkg.Install {
				pkg.Install[i].RunCommands = replaceDistributionVariablesSlice(pkg.Install[i].RunCommands, osinf)
			}
			pkg.PostInstall = replaceDistributionVariablesSlice(pkg.PostInstall, osinf)

			packages[pkg.Name] = pkg
		}
	}

	packageCache[cacheKey] = packages

	return packages, nil
}

func replaceDistributionVariablesSlice(inputs []string, osinf osinfo.Info) []string {
	results := make([]string, len(inputs))
	for i, input := range inputs {
		results[i] = replaceDistributionVariables(input, osinf)
	}

	return results
}

func replaceDistributionVariables(input string, osinf osinfo.Info) string {
	result := strings.ReplaceAll(input, "{{distname}}", osinf.Distribution.String())
	result = strings.ReplaceAll(result, "{{distversion}}", osinf.DistributionVersion)
	result = strings.ReplaceAll(result, "{{codename}}", osinf.DistributionCodename)
	result = strings.ReplaceAll(result, "{{architecture}}", osinf.Platform.String())

	return result
}
