package apt

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

type PackageConfig struct {
	Name         string   `yaml:"name"`
	Dependencies []string `yaml:"dependencies"`
	ReplaceWith  []string `yaml:"replace-with"`
	Virtual      bool     `yaml:"virtual"`
	PreInstall   []string `yaml:"pre-install"`
	Install      []string `yaml:"install"`
	PostInstall  []string `yaml:"post-install"`
}

var (
	packageCache      = make(map[string]map[string]PackageConfig)
	packageCacheMutex sync.RWMutex
)

func buildCacheKey(osinf osinfo.Info) string {
	return fmt.Sprintf(
		"%s_%s_%s_%s",
		osinf.Distribution.String(),
		osinf.DistributionVersion,
		osinf.DistributionCodename,
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

	if distribution != "" && osinf.DistributionVersion != "" {
		filesToLoad = append(
			filesToLoad,
			fmt.Sprintf("%s_%s.yaml", strings.ToLower(distribution), osinf.DistributionVersion),
		)
	}

	if distribution != "" && osinf.DistributionCodename != "" {
		filesToLoad = append(
			filesToLoad,
			fmt.Sprintf("%s_%s.yaml", strings.ToLower(distribution), osinf.DistributionCodename),
		)
	}

	if distribution != "" && osinf.DistributionVersion != "" && arch != "" {
		filesToLoad = append(
			filesToLoad,
			fmt.Sprintf("%s_%s_%s.yaml", strings.ToLower(distribution), osinf.DistributionVersion, arch),
		)
	}

	if distribution != "" && osinf.DistributionCodename != "" && arch != "" {
		filesToLoad = append(
			filesToLoad,
			fmt.Sprintf("%s_%s_%s.yaml", strings.ToLower(distribution), osinf.DistributionCodename, arch),
		)
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
			pkg.PreInstall = replaceDistributionVariablesSlice(pkg.PreInstall, osinf)
			pkg.Install = replaceDistributionVariablesSlice(pkg.Install, osinf)
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
