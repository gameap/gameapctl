package windows

import (
	"embed"
	"fmt"
	"os"
	"strings"

	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/goccy/go-yaml"
)

//go:embed *.yaml
var embedFS embed.FS

// config represents the root configuration.
type config struct {
	Packages []Package `yaml:"packages"`
}

// Package represents a software package configuration.
type Package struct {
	Name              string        `yaml:"name"`
	LookupPaths       []string      `yaml:"lookup-paths,omitempty"`
	DownloadURLs      []string      `yaml:"download-urls,omitempty"`
	InstallPath       string        `yaml:"install-path,omitempty"`
	PreInstall        []PreInstall  `yaml:"pre-install,omitempty"`
	InstallCommands   []string      `yaml:"install-commands,omitempty"`
	Install           []InstallStep `yaml:"install,omitempty"`
	UninstallCommands []string      `yaml:"uninstall-commands,omitempty"`
	PathEnv           []string      `yaml:"path-env,omitempty"`
	Dependencies      []string      `yaml:"dependencies,omitempty"`
	Service           *Service      `yaml:"service,omitempty"`
}

type InstallStep struct {
	RunCommands             []string `yaml:"run-commands,omitempty"`
	AllowedInstallExitCodes []int    `yaml:"allowed-install-exit-codes,omitempty"`
	WaitForService          string   `yaml:"wait-for-service,omitempty"`
	WaitForFiles            []string `yaml:"wait-for-files,omitempty"`
}

// Service represents a Windows service configuration.
type Service struct {
	ID               string             `yaml:"id"`
	Name             string             `yaml:"name"`
	Executable       string             `yaml:"executable"`
	Arguments        string             `yaml:"arguments,omitempty"`
	WorkingDirectory string             `yaml:"working-directory,omitempty"`
	StopExecutable   string             `yaml:"stop-executable,omitempty"`
	StopArguments    string             `yaml:"stop-arguments,omitempty"`
	OnFailure        []ServiceOnFailure `yaml:"on-failure,omitempty"`
	ServiceAccount   *ServiceAccount    `yaml:"service-account,omitempty"`
	Env              []EnvironmentVar   `yaml:"env,omitempty"`
}

type ServiceOnFailure struct {
	Action string `yaml:"action"`
	Delay  string `yaml:"delay"`
}

// ServiceAccount represents the service account configuration.
type ServiceAccount struct {
	Username string `yaml:"username"`
	Password string `yaml:"password,omitempty"`
}

// EnvironmentVar represents an environment variable.
type EnvironmentVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// PreInstall represents pre-installation steps.
type PreInstall struct {
	GrantPermissions []Permission `yaml:"grant-permissions,omitempty"`
	Commands         []string     `yaml:"commands,omitempty"`
}

// Permission represents file/folder permission settings.
type Permission struct {
	Path   string `yaml:"path"`
	User   string `yaml:"user"`
	Access string `yaml:"access"`
}

func LoadPackages(osinf osinfo.Info) (map[string]Package, error) {
	packages := make(map[string]Package)

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
		data, err := embedFS.ReadFile(filename)
		if err != nil {
			continue
		}

		var cfg config
		err = yaml.Unmarshal(data, &cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s: %w", filename, err)
		}

		for _, pkg := range cfg.Packages {
			packages[pkg.Name] = replaceValuesInPackage(pkg, osinf)
		}
	}

	return packages, nil
}

func replaceValuesInPackage(pkg Package, osinf osinfo.Info) Package {
	pkg.LookupPaths = expandEnvSlice(replaceValuesSlice(pkg.LookupPaths, osinf, pkg))
	pkg.DownloadURLs = replaceValuesSlice(pkg.DownloadURLs, osinf, pkg)
	pkg.InstallCommands = expandEnvSlice(replaceValuesSlice(pkg.InstallCommands, osinf, pkg))
	pkg.UninstallCommands = expandEnvSlice(replaceValuesSlice(pkg.UninstallCommands, osinf, pkg))
	pkg.PathEnv = expandEnvSlice(replaceValuesSlice(pkg.PathEnv, osinf, pkg))
	pkg.InstallPath = os.ExpandEnv(replaceValues(pkg.InstallPath, osinf, pkg))

	for i := range pkg.PreInstall {
		pkg.PreInstall[i].Commands = expandEnvSlice(replaceValuesSlice(pkg.PreInstall[i].Commands, osinf, pkg))

		for j := range pkg.PreInstall[i].GrantPermissions {
			pkg.PreInstall[i].GrantPermissions[j].Path = os.ExpandEnv(replaceValues(
				pkg.PreInstall[i].GrantPermissions[j].Path,
				osinf,
				pkg,
			))
		}
	}

	for i := range pkg.Install {
		pkg.Install[i].RunCommands = expandEnvSlice(replaceValuesSlice(pkg.Install[i].RunCommands, osinf, pkg))
		pkg.Install[i].WaitForFiles = expandEnvSlice(replaceValuesSlice(pkg.Install[i].WaitForFiles, osinf, pkg))
	}

	return pkg
}

func replaceValuesSlice(slice []string, osinf osinfo.Info, pkg Package) []string {
	result := make([]string, 0, len(slice))

	for _, s := range slice {
		result = append(result, replaceValues(s, osinf, pkg))
	}

	return result
}

func replaceValues(s string, osinf osinfo.Info, pkg Package) string {
	// OS-specific replacements
	result := strings.ReplaceAll(s, "{{architecture}}", osinf.Platform.String())
	result = strings.ReplaceAll(result, "{{distname}}", osinf.Distribution.String())
	result = strings.ReplaceAll(result, "{{distversion}}", osinf.DistributionVersion)
	result = strings.ReplaceAll(result, "{{codename}}", osinf.DistributionCodename)

	// Package-specific replacements
	result = strings.ReplaceAll(result, "{{package_install_path}}", pkg.InstallPath)

	return result
}

func expandEnvSlice(s []string) []string {
	result := make([]string, 0, len(s))
	for _, item := range s {
		result = append(result, os.ExpandEnv(item))
	}

	return result
}
