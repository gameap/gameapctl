package packagemanager

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/gameap/gameapctl/pkg/oscore"
	pmapt "github.com/gameap/gameapctl/pkg/package_manager/apt"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
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

// Install installs a set of packages.
func (apt *apt) Install(_ context.Context, packs ...string) error {
	args := []string{"install", "-y"}
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

func (e *extendedAPT) Install(ctx context.Context, packs ...string) error {
	var err error

	packs, err = e.preInstallationSteps(ctx, packs...)
	if err != nil {
		return errors.WithMessage(err, "failed to run pre installation steps")
	}

	packs = e.replaceAliases(ctx, packs)

	err = e.apt.Install(ctx, packs...)
	if err != nil {
		return errors.WithMessage(err, "failed to install packages")
	}

	err = e.postInstallationSteps(ctx, packs...)
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
		if pkgConfig, exists := e.packages[packName]; exists {
			replacedPacks = append(replacedPacks, pkgConfig.ReplaceWith...)
		} else {
			replacedPacks = append(replacedPacks, packName)
		}
	}

	return replacedPacks
}

func (e *extendedAPT) preInstallationSteps(ctx context.Context, packs ...string) ([]string, error) {
	updatedPacks := make([]string, 0, len(packs))

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
		case NginxPackage:
			err := e.addNginxRepositories(ctx)
			if err != nil {
				return nil, err
			}

			err = e.apt.CheckForUpdates(ctx)
			if err != nil {
				return nil, err
			}

			updatedPacks = append(updatedPacks, pack)
		case ApachePackage:
			err := e.apachePackageProcess(ctx)
			if err != nil {
				return nil, errors.WithMessage(err, "failed to process apache packages")
			}
		case NodeJSPackage:
			err := e.addNodeJSRepositories(ctx)
			if err != nil {
				return nil, err
			}

			err = e.apt.CheckForUpdates(ctx)
			if err != nil {
				return nil, err
			}

			updatedPacks = append(updatedPacks, pack)
		default:
			updatedPacks = append(updatedPacks, pack)
		}
	}

	return updatedPacks, nil
}

func (e *extendedAPT) postInstallationSteps(ctx context.Context, packs ...string) error {
	err := e.executeInstallationSteps(
		ctx,
		packs,
		func(config pmapt.PackageConfig) []string { return config.PostInstall },
	)
	if err != nil {
		return errors.WithMessage(err, "failed to run post-installation steps")
	}

	return nil
}

func (e *extendedAPT) executeInstallationSteps(
	ctx context.Context,
	packs []string,
	getCommands func(pmapt.PackageConfig) []string,
) error {
	executedPackages := make(map[string]bool)

	for _, packName := range packs {
		config, exists := e.packages[packName]
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
			if err := e.executeCommand(ctx, cmd); err != nil {
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

	err = e.apt.Install(ctx, installPackages...)
	if err != nil {
		return err
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

func (e *extendedAPT) addNginxRepositories(ctx context.Context) error {
	if utils.IsFileExists(sourcesListNginx) {
		return nil
	}

	osInfo := contextInternal.OSInfoFromContext(ctx)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("https://nginx.org/packages/%s/dists/%s/",
			strings.ToLower(string(osInfo.Distribution)),
			strings.ToLower(osInfo.DistributionCodename),
		),
		nil,
	)
	if err != nil {
		return err
	}

	//nolint:bodyclose
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to get nginx repository"))

		return nil
	}
	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to close response body"))
		}
	}(response.Body)

	if response.StatusCode != http.StatusOK {
		return nil
	}

	if !utils.IsFileExists("/usr/share/keyrings/") {
		err = os.Mkdir("/usr/share/keyrings/", 0755)
		if err != nil {
			return errors.WithMessage(err, "failed to create /usr/share/keyrings/ directory")
		}
	}

	if !utils.IsFileExists("/usr/share/keyrings/nginx-archive-keyring.gpg") {
		err = utils.ExecCommand(
			"bash", "-c",
			"curl -fsSL https://nginx.org/keys/nginx_signing.key | "+
				"gpg --dearmor -o /usr/share/keyrings/nginx-archive-keyring.gpg",
		)
		if err != nil {
			return errors.WithMessage(err, "failed to receive nodejs gpg key")
		}
	}

	err = utils.WriteContentsToFile(
		[]byte(
			fmt.Sprintf(
				"deb [signed-by=/usr/share/keyrings/nginx-archive-keyring.gpg] http://nginx.org/packages/%s/ %s nginx",
				osInfo.Distribution,
				osInfo.DistributionCodename,
			),
		),
		sourcesListNginx,
	)
	if err != nil {
		return errors.WithMessagef(err, "failed to write nginx source list to %s", sourcesListNginx)
	}

	return nil
}

func (e *extendedAPT) addNodeJSRepositories(_ context.Context) error {
	var err error
	if !utils.IsFileExists("/usr/share/keyrings/") {
		err = os.Mkdir("/usr/share/keyrings/", 0755)
		if err != nil {
			return errors.WithMessage(err, "failed to create /usr/share/keyrings/ directory")
		}
	}

	if !utils.IsFileExists("/usr/share/keyrings/nodesource.gpg") {
		err = utils.ExecCommand(
			"bash", "-c",
			"curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key |"+
				" gpg --dearmor -o /usr/share/keyrings/nodesource.gpg",
		)
		if err != nil {
			return errors.WithMessage(err, "failed to receive nodejs gpg key")
		}
	}

	err = utils.WriteContentsToFile(
		[]byte("deb [signed-by=/usr/share/keyrings/nodesource.gpg] https://deb.nodesource.com/node_20.x nodistro main"),
		sourcesListNode,
	)
	if err != nil {
		return err
	}

	return nil
}

func (e *extendedAPT) apachePackageProcess(ctx context.Context) error {
	phpVersion, err := DefinePHPVersion()
	if err != nil {
		return errors.WithMessage(err, "failed to define php version")
	}

	err = e.apt.Install(ctx, ApachePackage, "libapache2-mod-php"+phpVersion)
	if err != nil {
		return err
	}

	return nil
}
