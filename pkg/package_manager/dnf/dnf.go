package dnf

import (
	"embed"
	"fmt"
	"strings"

	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/goccy/go-yaml"
)

//go:embed *.yaml
var fs embed.FS

type packagesConfig struct {
	Packages []PackageConfig `yaml:"packages"`
}

//nolint:tagliatelle
type PackageConfig struct {
	Name        string   `yaml:"name"`
	ReplaceWith []string `yaml:"replace-with"`
	Virtual     bool     `yaml:"virtual"`
	PreInstall  []string `yaml:"pre-install"`
	PostInstall []string `yaml:"post-install"`
}

func LoadPackages(osinf osinfo.Info) (map[string]PackageConfig, error) {
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
			packages[pkg.Name] = pkg
		}
	}

	return packages, nil
}
