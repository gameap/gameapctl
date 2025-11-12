//go:build windows

package packagemanager

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	pathPkg "path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/gameap/gameapctl/pkg/package_manager/windows"
	"github.com/gameap/gameapctl/pkg/service"
	"github.com/gameap/gameapctl/pkg/shellquote"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

const servicesConfigPath = "C:\\gameap\\services"

const defaultServiceUser = "NT AUTHORITY\\NETWORK SERVICE"

// https://curl.se/docs/caextract.html
const caCertURL = "https://curl.se/ca/cacert.pem"

type WindowsPackageManager struct {
	packages map[string]windows.Package
}

func NewWindowsPackageManager(_ context.Context, info osinfo.Info) (*WindowsPackageManager, error) {
	packages, err := windows.LoadPackages(info)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to load windows packages")
	}

	return &WindowsPackageManager{
		packages: packages,
	}, nil
}

func (pm *WindowsPackageManager) Search(_ context.Context, _ string) ([]PackageInfo, error) {
	return nil, nil
}

func (pm *WindowsPackageManager) Install(ctx context.Context, pack string, opts ...InstallOptions) error {
	var err error

	options := &installOptions{}
	for _, opt := range opts {
		opt(options)
	}

	err = pm.installDependencies(ctx, pack, opts...)
	if err != nil {
		return errors.WithMessage(err, "failed to install dependencies")
	}

	p, exists := pm.packages[pack]
	if !exists {
		return nil
	}

	err = pm.installPackage(ctx, p, options)
	if err != nil {
		return err
	}

	UpdateEnvPath(ctx)

	return nil
}

func (pm *WindowsPackageManager) installDependencies(
	ctx context.Context,
	packName string,
	opts ...InstallOptions,
) error {
	dependencies := make([]string, 0)

	config, exists := pm.packages[packName]
	if !exists {
		return nil
	}

	for _, d := range config.Dependencies {
		if d == packName {
			return errors.WithMessagef(
				ErrCannotDependOnSelf,
				"failed to resolve dependencies for package '%s'", packName,
			)
		}
	}

	dependencies = append(dependencies, config.Dependencies...)

	if len(dependencies) == 0 {
		return nil
	}

	for _, dep := range dependencies {
		err := pm.Install(ctx, dep, opts...)
		if err != nil {
			return errors.WithMessage(err, "failed to install dependencies")
		}
	}

	return nil
}

func convertAccessToOSCoreFlag(access string) oscore.GrantFlag {
	switch strings.ToLower(access) {
	case "r", "read":
		return oscore.GrantFlagRead
	case "rx", "read-execute", "readexecute":
		return oscore.GrantFlagReadExecute
	case "w", "write":
		return oscore.GrantFlagWrite
	case "m", "modify":
		return oscore.GrantFlagModify
	case "f", "full-control", "fullcontrol":
		return oscore.GrantFlagFullControl
	default:
		return oscore.GrantFlagRead
	}
}

func (pm *WindowsPackageManager) preInstallationSteps(ctx context.Context, p windows.Package) error {
	for _, pre := range p.PreInstall {
		if len(pre.GrantPermissions) > 0 {
			err := pm.grantPermissions(ctx, pre.GrantPermissions)
			if err != nil {
				return errors.WithMessagef(
					err,
					"failed to grant permissions for package '%s'",
					p.Name,
				)
			}
		}

		for _, cmdStr := range pre.Commands {
			err := pm.executeCommand(ctx, cmdStr)
			if err != nil {
				return errors.WithMessagef(
					err,
					"failed to execute pre install command for package '%s': %s",
					p.Name,
					cmdStr,
				)
			}
		}
	}

	return nil
}

func (pm *WindowsPackageManager) grantPermissions(ctx context.Context, permissions []windows.Permission) error {
	for _, p := range permissions {
		log.Printf("Granting %s to %s for %s\n", p.Access, p.User, p.Path)

		err := oscore.Grant(ctx, p.Path, p.User, convertAccessToOSCoreFlag(p.Access))
		if err != nil {
			return errors.WithMessagef(
				err,
				"failed to grant %s to %s for %s",
				p.Access,
				p.User,
				p.Path,
			)
		}
	}

	return nil
}

//nolint:gocognit,funlen,gocyclo
func (pm *WindowsPackageManager) installPackage(ctx context.Context, p windows.Package, opts *installOptions) error {
	log.Println("Installing", p.Name, "package")
	var err error

	runtimeVars := runtimeTemplateVariables{
		LookupPaths: make(map[string]string, len(p.LookupPaths)),
		Options:     opts,

		// default configValues
		InstallPath: p.InstallPath,
	}

	resolvedPackagePath := ""
	foundCount := 0
	for _, c := range p.LookupPaths {
		resolvedPackagePath, err = exec.LookPath(c)
		if err != nil {
			continue
		}

		foundCount++

		log.Printf("Binary %s for package %s is found in '%s'\n",
			c,
			p.Name,
			resolvedPackagePath,
		)

		runtimeVars.LookupPaths[c] = filepath.Dir(resolvedPackagePath)
	}

	if len(p.LookupPaths) > 0 && foundCount >= len(p.LookupPaths) {
		if p.Service == nil {
			log.Printf(
				"Package %s is already installed, skipping installation (lookup path found, service is nil)\n",
				p.Name,
			)

			return nil
		}

		if service.IsExists(ctx, p.Service.ID) || service.IsExists(ctx, p.Service.Name) {
			log.Printf(
				"Package %s is already installed, skipping installation (lookup path found, service %s exists)\n",
				p.Name,
				p.Service.ID,
			)
		}
	}

	p, err = pm.replaceRuntimeVariables(ctx, p, runtimeVars)
	if err != nil {
		return errors.WithMessage(err, "failed to replace runtimeTemplateVariables variables in package")
	}

	preProcessor, ok := packagePreProcessors[p.Name]
	if ok {
		log.Println("Execute pre processor for ", p.Name)
		err = preProcessor(ctx, resolvedPackagePath)
		if err != nil {
			return err
		}
	}

	dir := p.InstallPath

	if dir == "" {
		dir, err = os.MkdirTemp("", "install")
		if err != nil {
			return errors.WithMessagef(err, "failed to make temp directory")
		}
		defer func(path string) {
			err := os.RemoveAll(path)
			if err != nil {
				log.Println(errors.WithMessage(err, "failed to remove temp directory"))
			}
		}(dir)
	}

	if !utils.IsFileExists(dir) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return errors.WithMessage(err, "failed to make directory")
		}
	}

	err = pm.preInstallationSteps(ctx, p)
	if err != nil {
		return errors.WithMessage(err, "failed to run pre installation steps")
	}

	for _, path := range p.DownloadURLs {
		log.Println("Downloading file from", path, "to", dir)

		var parsedURL *url.URL
		parsedURL, err = url.Parse(path)
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to parse url"))

			continue
		}

		if filepath.Ext(parsedURL.Path) == ".msi" {
			err = utils.DownloadFileOrArchive(
				ctx,
				path,
				filepath.Join(dir, pathPkg.Base(parsedURL.Path)),
			)
		} else {
			err = utils.Download(ctx, path, dir)
		}

		if err != nil {
			log.Println(errors.WithMessage(err, "failed to download file"))

			continue
		}

		err = nil

		break
	}
	if err != nil {
		return errors.WithMessage(err, "failed to download file")
	}

	//nolint:nestif
	if len(p.Install) > 0 {
		log.Println("Running install steps for package ", p.Name)

		for _, step := range p.Install {
			if len(step.GrantPermissions) > 0 {
				err = pm.grantPermissions(ctx, step.GrantPermissions)
				if err != nil {
					return errors.WithMessagef(
						err,
						"failed to grant permissions for package '%s'",
						p.Name,
					)
				}
			}

			if len(step.RunCommands) > 0 {
				log.Println("Running install commands for package ", p.Name)

				for _, cmd := range step.RunCommands {
					log.Println("Running install command:", cmd)

					scmd, err := shellquote.Split(cmd)
					if err != nil || len(scmd) == 0 {
						return errors.WithMessagef(err, "failed to parse install command: %s", cmd)
					}

					env := os.Environ()
					for _, e := range step.Env {
						env = append(env, e.String())
					}

					// Trusted source: Commands come from embedded YAML files (compiled into the binary), not user input
					//
					//nolint:gosec
					execCmd := exec.Command("cmd", append([]string{"/C"}, scmd...)...)
					execCmd.Env = env
					execCmd.Stdout = log.Writer()
					execCmd.Stderr = log.Writer()
					execCmd.Dir = dir

					log.Println('\n', execCmd.String())

					err = execCmd.Run()
					if err != nil {
						if len(step.AllowedInstallExitCodes) > 0 &&
							lo.Contains(step.AllowedInstallExitCodes, execCmd.ProcessState.ExitCode()) {
							log.Println(errors.WithMessage(err, "failed to execute install command"))
							log.Println("Exit code is allowed")
						} else {
							return errors.WithMessage(err, "failed to execute install command")
						}
					}
				}
			}

			if step.WaitForService != "" {
				err = waitUntil(ctx, func(ctx context.Context) (stop bool, err error) {
					if service.IsExists(ctx, step.WaitForService) {
						return true, nil
					}

					return false, nil
				})

				if err != nil {
					return errors.WithMessagef(err, "failed to wait for service '%s'", step.WaitForService)
				}
			}

			if len(step.WaitForFiles) > 0 {
				err = waitUntil(ctx, func(ctx context.Context) (stop bool, err error) {
					allExists := true

					for _, f := range step.WaitForFiles {
						if !utils.IsFileExists(f) {
							allExists = false

							break
						}
					}

					return allExists, nil
				})

				if err != nil {
					return errors.WithMessagef(err, "failed to wait for files '%v'", step.WaitForFiles)
				}
			}
		}
	}

	if len(p.PathEnv) > 0 {
		err = appendPathEnvVariable(p.PathEnv)
		if err != nil {
			return err
		}
	}

	if p.Service != nil {
		err = pm.installService(ctx, p)
		if err != nil {
			return errors.WithMessage(err, "failed to install service")
		}
	}

	return nil
}

func (pm *WindowsPackageManager) CheckForUpdates(_ context.Context) error {
	return nil
}

func (pm *WindowsPackageManager) Remove(ctx context.Context, packs ...string) error {
	for _, packName := range packs {
		p, exists := pm.packages[packName]
		if !exists {
			log.Printf("Package %s not found in configuration, skipping\n", packName)

			continue
		}

		err := pm.removePackage(ctx, p)
		if err != nil {
			return errors.WithMessagef(err, "failed to remove package '%s'", packName)
		}
	}

	return nil
}

func (pm *WindowsPackageManager) removePackage(ctx context.Context, p windows.Package) error {
	log.Println("Removing", p.Name, "package")

	if p.Service != nil {
		err := pm.removeService(ctx, p)
		if err != nil {
			return errors.WithMessage(err, "failed to remove service")
		}
	}

	if len(p.Uninstall) > 0 {
		log.Println("Running uninstall commands for package", p.Name)

		for _, step := range p.Uninstall {
			for _, cmd := range step.RunCommands {
				log.Println("Running uninstall command:", cmd)

				err := pm.executeCommand(ctx, cmd)
				if err != nil {
					log.Println(errors.WithMessagef(err, "failed to execute uninstall command: %s", cmd))
				}
			}
		}
	}

	if len(p.PathEnv) > 0 {
		err := removePathEnvVariable(p.PathEnv)
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to remove paths from PATH env variable"))
		}
	}

	log.Printf("Package %s successfully removed\n", p.Name)

	return nil
}

func (pm *WindowsPackageManager) removeService(ctx context.Context, p windows.Package) error {
	if p.Service == nil {
		return nil
	}

	log.Println("Removing service for package", p.Name)

	serviceName := p.Service.Name
	if serviceName == "" {
		serviceName = p.Service.ID
	}

	if !service.IsExists(ctx, serviceName) && !service.IsExists(ctx, p.Service.ID) {
		log.Printf("Service '%s' does not exist, skipping service removal\n", serviceName)

		return nil
	}

	svc := service.NewWindows()

	log.Printf("Stopping service '%s'\n", serviceName)
	err := svc.Stop(ctx, serviceName)
	if err != nil && !errors.Is(err, service.ErrInactiveService) {
		log.Println(errors.WithMessagef(err, "failed to stop service '%s'", serviceName))
	}

	log.Printf("Deleting service '%s'\n", serviceName)
	err = oscore.ExecCommand(ctx, "sc", "delete", serviceName)
	if err != nil {
		return errors.WithMessagef(err, "failed to delete service '%s'", serviceName)
	}

	configPath := filepath.Join(servicesConfigPath, serviceName+".yaml")
	if utils.IsFileExists(configPath) {
		log.Printf("Removing service config file '%s'\n", configPath)
		err = os.Remove(configPath)
		if err != nil {
			log.Println(errors.WithMessagef(err, "failed to remove service config file '%s'", configPath))
		}
	}

	log.Printf("Service '%s' successfully removed\n", serviceName)

	return nil
}

func removePathEnvVariable(pathsToRemove []string) error {
	currentPath := strings.Split(os.Getenv("PATH"), string(filepath.ListSeparator))
	newPath := make([]string, 0, len(currentPath))

	for _, p := range currentPath {
		shouldRemove := false
		for _, removeCandidate := range pathsToRemove {
			if strings.EqualFold(p, removeCandidate) {
				shouldRemove = true

				break
			}
		}

		if !shouldRemove {
			newPath = append(newPath, p)
		}
	}

	if len(newPath) == len(currentPath) {
		return nil
	}

	newPathValue := strings.Join(newPath, string(filepath.ListSeparator))

	err := os.Setenv("PATH", newPathValue)
	if err != nil {
		return errors.WithMessage(err, "failed to set PATH env variable")
	}

	return nil
}

func (pm *WindowsPackageManager) Purge(ctx context.Context, packs ...string) error {
	return pm.Remove(ctx, packs...)
}

func (pm *WindowsPackageManager) installService(ctx context.Context, p windows.Package) error {
	return pm.installShawlService(ctx, p)
}

// installWinSWService installs a service using WinSW (https://github.com/winsw/winsw)
//
//nolint:funlen,unused
func (pm *WindowsPackageManager) installWinSWService(ctx context.Context, p windows.Package) error {
	var err error

	log.Println("Installing service for package", p.Name)

	if service.IsExists(ctx, p.Service.ID) {
		log.Printf("Service '%s' is already exists", p.Service.ID)

		return nil
	}

	if service.IsExists(ctx, p.Service.Name) {
		log.Printf("Service '%s' is already exists", p.Service.Name)

		return nil
	}

	serviceConfig := newWinSWServiceConfig(p)

	if serviceConfig.WorkingDirectory == "" {
		path, err := exec.LookPath(serviceConfig.Executable)
		if err != nil {
			return errors.WithMessage(err, "failed to look path for service executable")
		}

		if path == "" {
			return errors.New("executable path not found")
		}

		serviceConfig.WorkingDirectory = filepath.Dir(path)
	}

	if !utils.IsFileExists(servicesConfigPath) {
		log.Println("Creating services config directory at", servicesConfigPath)

		err = os.MkdirAll(servicesConfigPath, 0755)
		if err != nil {
			return errors.WithMessage(err, "failed to create services config directory")
		}

		log.Println("Granting full control to ", defaultServiceUser, " for services config directory")
		err = oscore.GrantFullControl(ctx, servicesConfigPath, defaultServiceUser)
		if err != nil {
			return errors.WithMessage(err, "failed to set permissions for services config directory")
		}
	}

	configPath := filepath.Join(servicesConfigPath, p.Service.Name+".xml")

	configOverride := false

	if utils.IsFileExists(configPath) {
		log.Printf("Service config for '%s' is already exists", p.Service.Name)
		// Config already exists, we will override it and try to refresh before installation
		configOverride = true
	}

	out, err := xml.MarshalIndent(struct {
		WinSWServiceConfig
		XMLName struct{} `xml:"service"`
	}{WinSWServiceConfig: serviceConfig}, "", "  ")

	if err != nil {
		return errors.WithMessage(err, "failed to marshal service config")
	}

	log.Println("Marshalled service config")
	log.Println(string(out))

	log.Println("create service config")

	err = utils.WriteContentsToFile(out, configPath)
	if err != nil {
		return errors.WithMessagef(err, "failed to save config for service '%s' ", p.Service.Name)
	}

	if p.Service.ServiceAccount != nil && p.Service.ServiceAccount.Username != "" {
		err = oscore.Grant(ctx, configPath, p.Service.ServiceAccount.Username, oscore.GrantFlagFullControl)
		if err != nil {
			return errors.WithMessagef(
				err,
				"failed to grant full control to user '%s' for service config '%s'",
				p.Service.ServiceAccount.Username,
				configPath,
			)
		}
	}

	if configOverride {
		err = oscore.ExecCommand(ctx, "winsw", "refresh", configPath)
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to refresh service"))
			// There is no need to return error here, because it seems that service
			// config is already exists, but is not installed. We will try to install it next
		}
		if err == nil {
			// Refreshed successfully, no need to run install service command
			return nil
		}
	}

	err = oscore.ExecCommand(ctx, "winsw", "install", configPath)
	if err != nil {
		return errors.WithMessagef(err, "failed to install service '%s'", p.Service.Name)
	}

	return nil
}

// installServyService installs a service using Servy (https://github.com/aelassas/servy)
//
//nolint:funlen,unused
func (pm *WindowsPackageManager) installServyService(ctx context.Context, p windows.Package) error {
	if p.Service == nil {
		return nil
	}

	log.Println("Installing service for package", p.Name)

	if service.IsExists(ctx, p.Service.ID) {
		log.Printf("Service '%s' is already exists", p.Service.ID)

		return nil
	}

	if service.IsExists(ctx, p.Service.Name) {
		log.Printf("Service '%s' is already exists", p.Service.Name)

		return nil
	}

	if !utils.IsFileExists(servicesConfigPath) {
		log.Println("Creating services config directory at", servicesConfigPath)

		err := os.MkdirAll(servicesConfigPath, 0755)
		if err != nil {
			return errors.WithMessage(err, "failed to create services config directory")
		}

		log.Println("Granting full control to", defaultServiceUser, "for services config directory")
		err = oscore.GrantFullControl(ctx, servicesConfigPath, defaultServiceUser)
		if err != nil {
			return errors.WithMessage(err, "failed to set permissions for services config directory")
		}
	}

	executablePath := p.Service.Executable
	if executablePath == "" {
		return errors.New("service executable path is required")
	}

	workingDirectory := p.Service.WorkingDirectory
	if workingDirectory == "" {
		path, err := exec.LookPath(executablePath)
		if err == nil && path != "" {
			workingDirectory = filepath.Dir(path)
		} else {
			workingDirectory = filepath.Dir(executablePath)
		}
	}

	serviceName := p.Service.Name
	if serviceName == "" {
		serviceName = p.Name
	}

	logsDir := filepath.Join(servicesConfigPath, "logs", serviceName)

	config := servyConfig{
		Name:             serviceName,
		Description:      serviceName,
		ExecutablePath:   executablePath,
		StartupDirectory: workingDirectory,
		Parameters:       p.Service.Arguments,
		StartupType:      2,
		EnableRotation:   true,
		RotationSize:     10, //nolint:mnd // 10 MB
		StdoutPath:       filepath.Join(logsDir, "stdout.log"),
		StderrPath:       filepath.Join(logsDir, "stderr.log"),
	}

	if p.Service.ServiceAccount != nil && p.Service.ServiceAccount.Username != "" {
		config.RunAsLocalSystem = false
		config.UserAccount = p.Service.ServiceAccount.Username
		config.Password = p.Service.ServiceAccount.Password
	} else {
		config.RunAsLocalSystem = true
	}

	if len(p.Service.Env) > 0 {
		envVars := make([]string, 0, len(p.Service.Env))
		for _, e := range p.Service.Env {
			envVars = append(envVars, e.String())
		}
		config.EnvironmentVariables = strings.Join(envVars, ";")
	}

	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return errors.WithMessage(err, "failed to marshal service config to JSON")
	}

	log.Println("Marshalled service config:")
	log.Println(string(jsonData))

	configFileName := serviceName + ".json"
	configPath := filepath.Join(servicesConfigPath, configFileName)

	log.Println("Creating service config at", configPath)

	err = utils.WriteContentsToFile(jsonData, configPath)
	if err != nil {
		return errors.WithMessagef(err, "failed to save config for service '%s'", serviceName)
	}

	log.Println("Importing service using servy-cli.exe")

	err = oscore.ExecCommand(ctx, "servy-cli.exe", "import", "-c", "json", "-p", configPath)
	if err != nil {
		return errors.WithMessagef(err, "failed to import service '%s' using servy", serviceName)
	}

	log.Printf("Service '%s' successfully installed", serviceName)

	return nil
}

// installShawlService installs a service using Shawl (https://github.com/mtkennerly/shawl)
//
//nolint:funlen
func (pm *WindowsPackageManager) installShawlService(ctx context.Context, p windows.Package) error {
	if p.Service == nil {
		return nil
	}

	log.Println("Installing service for package", p.Name)

	if service.IsExists(ctx, p.Service.ID) {
		log.Printf("Service '%s' is already exists", p.Service.ID)

		return nil
	}

	if service.IsExists(ctx, p.Service.Name) {
		log.Printf("Service '%s' is already exists", p.Service.Name)

		return nil
	}

	shawlPath, err := exec.LookPath("shawl")
	if err != nil {
		return errors.WithMessage(err, "failed to find shawl executable")
	}

	if shawlPath == "" {
		return errors.New("shawl executable not found")
	}

	serviceName := p.Service.Name
	if serviceName == "" {
		serviceName = p.Service.ID
	}

	serviceAccount := defaultServiceUser
	if p.Service.ServiceAccount != nil && p.Service.ServiceAccount.Username != "" {
		serviceAccount = p.Service.ServiceAccount.Username
	}

	shawlArgs := []string{
		"run",
		"--name", strings.ReplaceAll(serviceName, " ", "_"),
	}

	if len(p.Service.OnFailure) > 0 {
		shawlArgs = append(shawlArgs, "--restart")

		for _, failure := range p.Service.OnFailure {
			if failure.Delay.Duration > 0 {
				shawlArgs = append(
					shawlArgs,
					"--restart-delay",
					fmt.Sprintf("%d", failure.Delay.Milliseconds()),
				)

				break
			}
		}
	}

	shawlArgs = append(shawlArgs, "--stop-timeout", "10000")

	if p.Service.WorkingDirectory != "" {
		shawlArgs = append(shawlArgs, "--cwd", p.Service.WorkingDirectory)
	}

	logDir := "C:\\gameap\\logs"

	if p.Service.LogDirectory != "" {
		logDir = p.Service.LogDirectory
	}

	shawlArgs = append(shawlArgs, "--log-dir", logDir)

	logBaseName := strings.ReplaceAll(serviceName, " ", "_")
	shawlArgs = append(shawlArgs, "--log-as", logBaseName+".log")
	shawlArgs = append(shawlArgs, "--log-rotate", "daily")
	shawlArgs = append(shawlArgs, "--log-retain", "60")

	for _, env := range p.Service.Env {
		shawlArgs = append(shawlArgs, "--env", fmt.Sprintf("%s=%s", env.Name, env.Value))
	}

	shawlArgs = append(shawlArgs, "--")
	shawlArgs = append(shawlArgs, p.Service.Executable)

	if p.Service.Arguments != "" {
		shawlArgs = append(shawlArgs, strings.Split(p.Service.Arguments, " ")...)
	}

	binPath := fmt.Sprintf("%s %s", shawlPath, strings.Join(shawlArgs, " "))

	if !utils.IsFileExists(servicesConfigPath) {
		log.Println("Creating services config directory at", servicesConfigPath)

		err = os.MkdirAll(servicesConfigPath, 0755)
		if err != nil {
			return errors.WithMessage(err, "failed to create services config directory")
		}
	}

	configPath := filepath.Join(servicesConfigPath, serviceName+".yaml")

	yamlConfig, err := yaml.Marshal(p.Service)
	if err != nil {
		return errors.WithMessage(err, "failed to marshal service config to YAML")
	}

	log.Println("Marshalled service config")
	log.Println(string(yamlConfig))

	log.Println("Saving service config to", configPath)

	err = utils.WriteContentsToFile(yamlConfig, configPath)
	if err != nil {
		return errors.WithMessagef(err, "failed to save config for service '%s'", serviceName)
	}

	scArgs := []string{
		"create",
		serviceName,
		"start=auto",
		fmt.Sprintf("obj=%s", serviceAccount),
		fmt.Sprintf("binPath=%s", binPath),
	}

	log.Println("Creating service with sc command:", "sc", strings.Join(scArgs, " "))

	err = oscore.ExecCommand(ctx, "sc", scArgs...)
	if err != nil {
		return errors.WithMessagef(err, "failed to install service '%s'", serviceName)
	}

	if len(p.Service.OnFailure) > 0 {
		err = pm.configureServiceFailureActions(ctx, serviceName, p.Service.OnFailure, p.Service.ResetFailure.Duration)
		if err != nil {
			return errors.WithMessagef(err, "failed to configure failure actions for service '%s'", serviceName)
		}
	}

	log.Printf("Service '%s' successfully installed", serviceName)

	return nil
}

func (pm *WindowsPackageManager) configureServiceFailureActions(
	ctx context.Context,
	serviceName string,
	onFailure []windows.ServiceOnFailure,
	resetFailure time.Duration,
) error {
	if len(onFailure) == 0 {
		return nil
	}

	actions := make([]string, 0, len(onFailure))
	for _, failure := range onFailure {
		action := ""
		delay := "0"

		switch failure.Action {
		case "restart":
			action = "restart"
		case "reboot":
			action = "reboot"
		case "run":
			action = "run"
		default:
			action = ""
		}

		if failure.Delay.Duration > 0 {
			delay = fmt.Sprintf("%d", int(failure.Delay.Milliseconds()))
		}

		actions = append(actions, fmt.Sprintf("%s/%s", action, delay))
	}

	actionsString := strings.Join(actions, "/")

	if resetFailure == 0 {
		resetFailure = time.Hour
	}

	resetSeconds := int(resetFailure.Seconds())

	scFailureArgs := []string{
		"failure",
		serviceName,
		fmt.Sprintf("reset=%d", resetSeconds),
		fmt.Sprintf("actions=%s", actionsString),
	}

	log.Println("Configuring service failure actions:", "sc", strings.Join(scFailureArgs, " "))

	err := oscore.ExecCommand(ctx, "sc", scFailureArgs...)
	if err != nil {
		return errors.WithMessage(err, "failed to configure service failure actions")
	}

	return nil
}

type runtimeTemplateVariables struct {
	// runtime
	LookupPaths map[string]string
	Options     *installOptions

	// some default configValues for package
	InstallPath string
}

//nolint:gocognit,funlen
func (pm *WindowsPackageManager) replaceRuntimeVariables(
	ctx context.Context, p windows.Package, vars runtimeTemplateVariables,
) (windows.Package, error) {
	var err error

	for i := range p.PreInstall {
		for j := range p.PreInstall[i].Commands {
			p.PreInstall[i].Commands[j], err = pm.replaceRuntimeVariablesString(
				ctx,
				p.PreInstall[i].Commands[j],
				vars,
			)
			if err != nil {
				return p, errors.WithMessagef(
					err,
					"failed to replace runtimeTemplateVariables in pre install step %d command %d for package '%s'",
					i,
					j,
					p.Name,
				)
			}
		}
	}

	for i := range p.Install {
		for j := range p.Install[i].RunCommands {
			p.Install[i].RunCommands[j], err = pm.replaceRuntimeVariablesString(
				ctx,
				p.Install[i].RunCommands[j],
				vars,
			)
			if err != nil {
				return p, errors.WithMessagef(
					err,
					"failed to replace runtimeTemplateVariables in install step %d command %d for package '%s'",
					i,
					j,
					p.Name,
				)
			}

			for k := range p.Install[i].Env {
				p.Install[i].Env[k].Value, err = pm.replaceRuntimeVariablesString(
					ctx,
					p.Install[i].Env[k].Value,
					vars,
				)
				if err != nil {
					return p, errors.WithMessagef(
						err,
						"failed to replace runtimeTemplateVariables in install step %d env variable '%s' for package '%s'",
						i,
						p.Install[i].Env[k].Name,
						p.Name,
					)
				}
			}

			for k := range p.Install[i].WaitForFiles {
				p.Install[i].WaitForFiles[k], err = pm.replaceRuntimeVariablesString(
					ctx,
					p.Install[i].WaitForFiles[k],
					vars,
				)
				if err != nil {
					return p, errors.WithMessagef(
						err,
						"failed to replace runtimeTemplateVariables in install step %d wait for file '%s' for package '%s'",
						i,
						p.Install[i].WaitForFiles[k],
						p.Name,
					)
				}
			}

			p.Install[i].WaitForService, err = pm.replaceRuntimeVariablesString(
				ctx,
				p.Install[i].WaitForService,
				vars,
			)
			if err != nil {
				return p, errors.WithMessagef(
					err,
					"failed to replace runtimeTemplateVariables in install step %d wait for service '%s' for package '%s'",
					i,
					p.Install[i].WaitForService,
					p.Name,
				)
			}

			for k := range p.Install[i].GrantPermissions {
				p.Install[i].GrantPermissions[k].Path, err = pm.replaceRuntimeVariablesString(
					ctx,
					p.Install[i].GrantPermissions[k].Path,
					vars,
				)
				if err != nil {
					return p, errors.WithMessagef(
						err,
						"failed to replace runtimeTemplateVariables in install step %d grant permission path '%s' for package '%s'",
						i,
						p.Install[i].GrantPermissions[k].Path,
						p.Name,
					)
				}
			}
		}
	}

	//nolint:nestif
	if p.Service != nil {
		p.Service.Executable, err = pm.replaceRuntimeVariablesString(ctx, p.Service.Executable, vars)
		if err != nil {
			return p, errors.WithMessage(err, "failed to replace runtimeTemplateVariables in service executable")
		}

		p.Service.Arguments, err = pm.replaceRuntimeVariablesString(ctx, p.Service.Arguments, vars)
		if err != nil {
			return p, errors.WithMessage(err, "failed to replace runtimeTemplateVariables in service arguments")
		}

		p.Service.WorkingDirectory, err = pm.replaceRuntimeVariablesString(ctx, p.Service.WorkingDirectory, vars)
		if err != nil {
			return p, errors.WithMessage(err, "failed to replace runtimeTemplateVariables in service working directory")
		}

		p.Service.StopExecutable, err = pm.replaceRuntimeVariablesString(ctx, p.Service.StopExecutable, vars)
		if err != nil {
			return p, errors.WithMessage(err, "failed to replace runtimeTemplateVariables in service stop executable")
		}

		for i := range p.Service.Env {
			p.Service.Env[i].Value, err = pm.replaceRuntimeVariablesString(ctx, p.Service.Env[i].Value, vars)
			if err != nil {
				return p, errors.WithMessagef(
					err,
					"failed to replace runtimeTemplateVariables in service env variable '%s'",
					p.Service.Env[i].Name,
				)
			}
		}
	}

	return p, nil
}

func (pm *WindowsPackageManager) replaceRuntimeVariablesString(
	_ context.Context, v string, vars runtimeTemplateVariables,
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

func (pm *WindowsPackageManager) executeCommand(ctx context.Context, cmdStr string) error {
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return nil
	}

	cmd, err := shellquote.Split(cmdStr)
	if err != nil || len(cmd) == 0 {
		return errors.WithMessagef(err, "failed to parse command: %s", cmdStr)
	}

	args := make([]string, 0, len(cmd)+1)

	args = append(args, "/C")
	args = append(args, cmd...)

	return oscore.ExecCommand(ctx, "cmd", args...)
}

// TODO: Remove this hardcoded pre processors and move them to package definitions.
var packagePreProcessors = map[string]func(ctx context.Context, packagePath string) error{
	PHPExtensionsPackage: func(ctx context.Context, packagePath string) error {
		cmd := exec.Command("php", "-r", "echo php_ini_scanned_files();")
		buf := &bytes.Buffer{}
		buf.Grow(1000) //nolint:mnd
		cmd.Stdout = buf
		cmd.Stderr = log.Writer()
		log.Println("\n", cmd.String())
		err := cmd.Run()
		if err != nil {
			return errors.WithMessage(err, "failed to get scanned files")
		}

		log.Println("Scanned files:", buf.String())

		scannedFiles := strings.Split(buf.String(), "\n")

		if len(scannedFiles) > 0 {
			firstScannedFile := strings.TrimSpace(scannedFiles[0])
			scannedFileDir := filepath.Dir(firstScannedFile)

			exts := []string{
				"bz2", "curl", "fileinfo", "gd", "gmp", "intl",
				"mbstring", "openssl", "pdo_mysql", "pdo_sqlite", "zip",
			}

			for _, e := range exts {
				err = utils.WriteContentsToFile([]byte(`extension=`+e), filepath.Join(scannedFileDir, e+".ini"))
				if err != nil {
					return errors.WithMessagef(err, "failed to create ini for '%s' php extension", e)
				}
			}
		}

		cmd = exec.Command("php", "-r", "echo php_ini_loaded_file();")
		buf = &bytes.Buffer{}
		buf.Grow(1000) //nolint:mnd
		cmd.Stdout = buf
		cmd.Stderr = log.Writer()
		log.Println("\n", cmd.String())
		err = cmd.Run()
		if err != nil {
			return errors.WithMessage(err, "failed to get ini loaded file from php")
		}

		log.Println("Loaded ini file:", buf.String())

		loadedFiles := strings.Split(buf.String(), "\n")
		iniFilePath := ""
		if len(loadedFiles) > 0 {
			iniFilePath = strings.TrimSpace(loadedFiles[0])
		}
		if iniFilePath == "" {
			if packagePath == "" {
				iniFilePath = filepath.Join("C:\\php", "php.ini")
			} else {
				iniFilePath = filepath.Join(filepath.Dir(packagePath), "php.ini")
			}
		}

		if !utils.IsFileExists(iniFilePath) {
			log.Println("Creating php.ini file on", iniFilePath)

			f, err := os.Create(iniFilePath)
			if err != nil {
				return err
			}
			if err = f.Close(); err != nil {
				return err
			}
		}

		if iniFilePath == "" {
			return errors.New("failed to find config edition way to enable php extensions")
		}

		err = utils.FindLineAndReplaceOrAdd(ctx, iniFilePath, map[string]string{
			";?\\s*extension=bz2\\s*":        "extension=bz2",
			";?\\s*extension=curl\\s*":       "extension=curl",
			";?\\s*extension=fileinfo\\s*":   "extension=fileinfo",
			";?\\s*extension=gd\\s*":         "extension=gd",
			";?\\s*extension=gmp\\s*":        "extension=gmp",
			";?\\s*extension=intl\\s*":       "extension=intl",
			";?\\s*extension=mbstring\\s*":   "extension=mbstring",
			";?\\s*extension=openssl\\s*":    "extension=openssl",
			";?\\s*extension=pdo_mysql\\s*":  "extension=pdo_mysql",
			";?\\s*extension=pdo_sqlite\\s*": "extension=pdo_sqlite",
			";?\\s*extension=sqlite\\s*":     "extension=sqlite3",
			";?\\s*extension=sockets\\s*":    "extension=sockets",
			";?\\s*extension=zip\\s*":        "extension=zip",
		})
		if err != nil {
			return errors.WithMessage(err, "failed to update extensions to php.ini")
		}

		cacertPath := filepath.Join(filepath.Dir(iniFilePath), "cacert.pem")

		err = utils.DownloadFile(ctx, caCertURL, cacertPath)
		if err != nil {
			return errors.WithMessage(err, "failed to download cacert.pem")
		}

		err = utils.FindLineAndReplaceOrAdd(ctx, iniFilePath, map[string]string{
			";?\\s*curl\\.cainfo\\s*":    fmt.Sprintf(`curl.cainfo="%s"`, cacertPath),
			";?\\s*openssl\\.cafile\\s*": fmt.Sprintf(`openssl.cafile="%s"`, cacertPath),
		})
		if err != nil {
			return errors.WithMessage(err, "failed to update cacert.pem path in php.ini")
		}

		return nil
	},
}

type WinSWServiceConfig struct {
	ID               string `xml:"id"`
	Name             string `xml:"name"`
	Executable       string `xml:"executable"`
	WorkingDirectory string `xml:"workingdirectory,omitempty"`
	Arguments        string `xml:"arguments,omitempty"`

	StopExecutable string `xml:"stopexecutable,omitempty"`
	StopArguments  string `xml:"stoparguments,omitempty"`

	OnFailure    []WinSWServiceConfigOnFailure `xml:"onfailure,omitempty"`
	ResetFailure string                        `xml:"resetfailure,omitempty"`

	ServiceAccount *WinSWServiceConfigServiceAccount `xml:"serviceaccount,omitempty"`

	Env []WinSWServiceConfigEnv `xml:"env,omitempty"`
}

type WinSWServiceConfigServiceAccount struct {
	Username string `xml:"username"`
	Password string `xml:"password"`
}

type WinSWServiceConfigOnFailure struct {
	Action string `xml:"action,attr"`
	Delay  string `xml:"delay,attr,omitempty"`
}

type WinSWServiceConfigEnv struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

//nolint:unused
func newWinSWServiceConfig(p windows.Package) WinSWServiceConfig {
	config := WinSWServiceConfig{
		ID:               p.Service.ID,
		Name:             p.Service.Name,
		Executable:       p.Service.Executable,
		WorkingDirectory: p.Service.WorkingDirectory,
		Arguments:        p.Service.Arguments,
	}

	if len(p.Service.OnFailure) > 0 {
		onFailures := make([]WinSWServiceConfigOnFailure, 0, len(p.Service.OnFailure))
		for _, of := range p.Service.OnFailure {
			delay := ""
			if of.Delay.Duration > 0 {
				delay = of.Delay.String()
			}

			onFailures = append(onFailures, WinSWServiceConfigOnFailure{
				Action: of.Action,
				Delay:  delay,
			})
		}

		config.OnFailure = onFailures
	}

	if p.Service.ServiceAccount != nil {
		config.ServiceAccount = &WinSWServiceConfigServiceAccount{
			Username: p.Service.ServiceAccount.Username,
			Password: p.Service.ServiceAccount.Password,
		}
	}

	if len(p.Service.Env) > 0 {
		envVars := make([]WinSWServiceConfigEnv, 0, len(p.Service.Env))
		for _, e := range p.Service.Env {
			envVars = append(envVars, WinSWServiceConfigEnv{
				Name:  e.Name,
				Value: e.Value,
			})
		}
		config.Env = envVars
	}

	return config
}

//nolint:unused,tagliatelle
type servyConfig struct {
	Name                           string `json:"Name"`
	Description                    string `json:"Description,omitempty"`
	ExecutablePath                 string `json:"ExecutablePath"`
	StartupDirectory               string `json:"StartupDirectory,omitempty"`
	Parameters                     string `json:"Parameters,omitempty"`
	StartupType                    int    `json:"StartupType,omitempty"`
	Priority                       int    `json:"Priority,omitempty"`
	StdoutPath                     string `json:"StdoutPath,omitempty"`
	StderrPath                     string `json:"StderrPath,omitempty"`
	EnableRotation                 bool   `json:"EnableRotation,omitempty"`
	RotationSize                   int    `json:"RotationSize,omitempty"`
	EnableHealthMonitoring         bool   `json:"EnableHealthMonitoring,omitempty"`
	HeartbeatInterval              int    `json:"HeartbeatInterval,omitempty"`
	MaxFailedChecks                int    `json:"MaxFailedChecks,omitempty"`
	RecoveryAction                 int    `json:"RecoveryAction,omitempty"`
	MaxRestartAttempts             int    `json:"MaxRestartAttempts,omitempty"`
	FailureProgramPath             string `json:"FailureProgramPath,omitempty"`
	FailureProgramStartupDirectory string `json:"FailureProgramStartupDirectory,omitempty"`
	FailureProgramParameters       string `json:"FailureProgramParameters,omitempty"`
	EnvironmentVariables           string `json:"EnvironmentVariables,omitempty"`
	ServiceDependencies            string `json:"ServiceDependencies,omitempty"`
	RunAsLocalSystem               bool   `json:"RunAsLocalSystem,omitempty"`
	UserAccount                    string `json:"UserAccount,omitempty"`
	Password                       string `json:"Password,omitempty"`
	PreLaunchExecutablePath        string `json:"PreLaunchExecutablePath,omitempty"`
	PreLaunchStartupDirectory      string `json:"PreLaunchStartupDirectory,omitempty"`
	PreLaunchParameters            string `json:"PreLaunchParameters,omitempty"`
	PreLaunchEnvironmentVariables  string `json:"PreLaunchEnvironmentVariables,omitempty"`
	PreLaunchStdoutPath            string `json:"PreLaunchStdoutPath,omitempty"`
	PreLaunchStderrPath            string `json:"PreLaunchStderrPath,omitempty"`
	PreLaunchTimeoutSeconds        int    `json:"PreLaunchTimeoutSeconds,omitempty"`
	PreLaunchRetryAttempts         int    `json:"PreLaunchRetryAttempts,omitempty"`
	PreLaunchIgnoreFailure         bool   `json:"PreLaunchIgnoreFailure,omitempty"`
	PostLaunchExecutablePath       string `json:"PostLaunchExecutablePath,omitempty"`
	PostLaunchStartupDirectory     string `json:"PostLaunchStartupDirectory,omitempty"`
	PostLaunchParameters           string `json:"PostLaunchParameters,omitempty"`
}

const (
	waitTriesMax = 10
	waitTime     = 5 * time.Second
)

func waitUntil(ctx context.Context, f func(ctx context.Context) (stop bool, err error)) error {
	if f == nil {
		return nil
	}

	waitTries := waitTriesMax
	ticker := time.NewTicker(waitTime)
	defer ticker.Stop()

	for waitTries > 0 {
		waitTries--

		stop, err := f(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to execute wait after func")
		}
		if stop {
			return nil
		}

		select {
		case <-ticker.C:
			log.Println("Waiting for install command to finish")
		case <-ctx.Done():
			waitTries = 0
		}
	}

	return errors.New("timeout waiting for install command to finish")
}

func appendPathEnvVariable(newPaths []string) error {
	currentPath := strings.Split(os.Getenv("PATH"), string(filepath.ListSeparator))
	pathsToAdd := make([]string, 0, len(newPaths))

	for _, p := range newPaths {
		if !utils.IsFileExists(p) {
			continue
		}

		if utils.Contains(currentPath, p) {
			continue
		}

		if utils.Contains(pathsToAdd, p) {
			continue
		}

		pathsToAdd = append(pathsToAdd, p)
	}

	if len(pathsToAdd) == 0 {
		return nil
	}

	newPathValue := strings.Join(append(currentPath, pathsToAdd...), string(filepath.ListSeparator))

	err := os.Setenv("PATH", newPathValue)
	if err != nil {
		return errors.WithMessage(err, "failed to set PATH env variable")
	}

	return nil
}
