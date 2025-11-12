package windows

import (
	"embed"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/goccy/go-yaml"
)

//go:embed *.yaml
var embedFS embed.FS

// DurationWithDefault is a time.Duration that defaults to 1 hour if parsing fails or value is empty.
type DurationWithDefault struct {
	time.Duration
}

// UnmarshalYAML implements custom YAML unmarshaling for DurationWithDefault.
//
//nolint:nilerr
func (d *DurationWithDefault) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		d.Duration = time.Hour

		return nil
	}

	if s == "" {
		d.Duration = time.Hour

		return nil
	}

	duration, err := time.ParseDuration(s)
	if err != nil {
		d.Duration = time.Hour

		return nil
	}

	d.Duration = duration

	return nil
}

// config represents the root configuration.
type config struct {
	Packages []Package `yaml:"packages"`
}

// Package represents a software package configuration.
type Package struct {
	Name         string          `yaml:"name"`
	LookupPaths  []string        `yaml:"lookup-paths,omitempty"`
	DownloadURLs []string        `yaml:"download-urls,omitempty"`
	InstallPath  string          `yaml:"install-path,omitempty"`
	PreInstall   []PreInstall    `yaml:"pre-install,omitempty"`
	Install      []InstallStep   `yaml:"install,omitempty"`
	PathEnv      []string        `yaml:"path-env,omitempty"`
	Uninstall    []UninstallStep `yaml:"uninstall,omitempty"`
	Dependencies []string        `yaml:"dependencies,omitempty"`
	Service      *Service        `yaml:"service,omitempty"`
}

type InstallStep struct {
	GrantPermissions        []Permission     `yaml:"grant-permissions,omitempty"`
	RunCommands             []string         `yaml:"run-commands,omitempty"`
	Env                     []EnvironmentVar `yaml:"env,omitempty"`
	AllowedInstallExitCodes []int            `yaml:"allowed-install-exit-codes,omitempty"`
	WaitForService          string           `yaml:"wait-for-service,omitempty"`
	WaitForFiles            []string         `yaml:"wait-for-files,omitempty"`
}

type UninstallStep struct {
	RunCommands               []string `yaml:"run-commands,omitempty"`
	AllowedUninstallExitCodes []int    `yaml:"allowed-uninstall-exit-codes,omitempty"`
}

// Service represents a Windows service configuration.
type Service struct {
	ID               string              `yaml:"id"`
	Name             string              `yaml:"name"`
	Executable       string              `yaml:"executable"`
	Arguments        string              `yaml:"arguments,omitempty"`
	WorkingDirectory string              `yaml:"working-directory,omitempty"`
	LogDirectory     string              `yaml:"log-directory,omitempty"`
	StopExecutable   string              `yaml:"stop-executable,omitempty"`
	StopArguments    string              `yaml:"stop-arguments,omitempty"`
	OnFailure        []ServiceOnFailure  `yaml:"on-failure,omitempty"`
	ResetFailure     DurationWithDefault `yaml:"reset-failure,omitempty"`
	ServiceAccount   *ServiceAccount     `yaml:"service-account,omitempty"`
	Env              []EnvironmentVar    `yaml:"env,omitempty"`
}

type ServiceOnFailure struct {
	Action string              `yaml:"action"`
	Delay  DurationWithDefault `yaml:"delay,omitempty"`
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

func (e EnvironmentVar) String() string {
	sb := strings.Builder{}
	sb.Grow(len(e.Name) + len(e.Value) + 1)

	sb.WriteString(e.Name)
	sb.WriteString("=")
	sb.WriteString(e.Value)

	return sb.String()
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

var (
	packageCache      = make(map[string]map[string]Package)
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

func LoadPackages(osinf osinfo.Info) (map[string]Package, error) {
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

	packageCache[cacheKey] = packages

	return packages, nil
}

func replaceValuesInPackage(pkg Package, osinf osinfo.Info) Package {
	pkg.LookupPaths = expandEnvSlice(replaceValuesSlice(pkg.LookupPaths, osinf, pkg))
	pkg.DownloadURLs = replaceValuesSlice(pkg.DownloadURLs, osinf, pkg)
	pkg.PathEnv = expandEnvSlice(replaceValuesSlice(pkg.PathEnv, osinf, pkg))
	pkg.InstallPath = os.ExpandEnv(replaceValues(pkg.InstallPath, osinf, pkg))

	for i := range pkg.PreInstall {
		pkg.PreInstall[i].Commands = normalizeCommands(
			expandEnvSlice(replaceValuesSlice(pkg.PreInstall[i].Commands, osinf, pkg)),
		)

		for j := range pkg.PreInstall[i].GrantPermissions {
			pkg.PreInstall[i].GrantPermissions[j].Path = os.ExpandEnv(replaceValues(
				pkg.PreInstall[i].GrantPermissions[j].Path,
				osinf,
				pkg,
			))
		}
	}

	for i := range pkg.Install {
		pkg.Install[i].RunCommands = normalizeCommands(
			expandEnvSlice(replaceValuesSlice(pkg.Install[i].RunCommands, osinf, pkg)),
		)
		pkg.Install[i].WaitForFiles = expandEnvSlice(replaceValuesSlice(pkg.Install[i].WaitForFiles, osinf, pkg))

		for j := range pkg.Install[i].Env {
			pkg.Install[i].Env[j].Value = os.ExpandEnv(replaceValues(
				pkg.Install[i].Env[j].Value,
				osinf,
				pkg,
			))
		}

		for j := range pkg.Install[i].GrantPermissions {
			pkg.Install[i].GrantPermissions[j].Path = os.ExpandEnv(replaceValues(
				pkg.Install[i].GrantPermissions[j].Path,
				osinf,
				pkg,
			))
		}
	}

	for i := range pkg.Uninstall {
		pkg.Uninstall[i].RunCommands = normalizeCommands(
			expandEnvSlice(replaceValuesSlice(pkg.Uninstall[i].RunCommands, osinf, pkg)),
		)
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

func normalizeCommands(commands []string) []string {
	normalized := make([]string, 0, len(commands))
	for _, cmd := range commands {
		normalized = append(normalized, normalizeCommand(cmd))
	}

	return normalized
}

func normalizeCommand(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")

	var normalized strings.Builder
	prevSpace := false

	for _, r := range s {
		if r == ' ' || r == '\t' {
			if !prevSpace {
				normalized.WriteRune(' ')
				prevSpace = true
			}
		} else {
			normalized.WriteRune(r)
			prevSpace = false
		}
	}

	return strings.TrimSpace(normalized.String())
}
