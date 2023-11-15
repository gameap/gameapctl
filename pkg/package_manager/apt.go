package packagemanager

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	contextInternal "github.com/gameap/gameapctl/internal/context"
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

		return nil, errors.WithMessage(err, "failed to run dpkg-query")
	}

	return parseAPTCacheShowOutput(out), nil
}

func parseAPTCacheShowOutput(out []byte) []PackageInfo {
	scanner := bufio.NewScanner(bytes.NewReader(out))

	var packageInfos []PackageInfo

	info := PackageInfo{}
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 2)
		if len(parts) < 2 {
			continue
		}

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
	apt *apt
}

func newExtendedAPT(apt *apt) *extendedAPT {
	return &extendedAPT{
		apt: apt,
	}
}

func (e *extendedAPT) Search(ctx context.Context, name string) ([]PackageInfo, error) {
	return e.apt.Search(ctx, name)
}

func (e *extendedAPT) Install(ctx context.Context, packs ...string) error {
	var err error
	packs = e.replaceAliases(ctx, packs)

	packs, err = e.findAndRunViaFuncs(ctx, packs...)
	if err != nil {
		return err
	}

	packs, err = e.preInstallationSteps(ctx, packs...)
	if err != nil {
		return errors.WithMessage(err, "failed to run pre installation steps")
	}

	return e.apt.Install(ctx, packs...)
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

func (e *extendedAPT) replaceAliases(ctx context.Context, packs []string) []string {
	replacedPacks := make([]string, 0, len(packs))

	osInfo := contextInternal.OSInfoFromContext(ctx)

	for _, pack := range packs {
		if aliases, exists :=
			packageAliases[osInfo.Distribution][osInfo.DistributionCodename][osInfo.Platform][pack]; exists {
			replacedPacks = append(replacedPacks, aliases...)
		} else if aliases, exists =
			packageAliases[osInfo.Distribution][osInfo.DistributionCodename]["default"][pack]; exists {
			replacedPacks = append(replacedPacks, aliases...)
		} else {
			replacedPacks = append(replacedPacks, pack)
		}
	}

	return replacedPacks
}

func (e *extendedAPT) findAndRunViaFuncs(ctx context.Context, packs ...string) ([]string, error) {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	var funcsByDistroAndArch map[string]map[string]installationFunc
	var funcsByArch map[string]installationFunc

	updatedPacks := make([]string, 0, len(packs))

	for _, p := range packs {
		funcsByDistroAndArch = installationFuncs[p]
		if funcsByDistroAndArch == nil {
			updatedPacks = append(updatedPacks, p)

			continue
		}

		funcsByArch = funcsByDistroAndArch[osInfo.Distribution]
		if funcsByArch == nil {
			funcsByArch = funcsByDistroAndArch[Default]
			if funcsByArch == nil {
				updatedPacks = append(updatedPacks, p)

				continue
			}
		}

		f := funcsByArch[osInfo.Platform]
		if f == nil {
			f = funcsByArch[ArchDefault]
			if f == nil {
				updatedPacks = append(updatedPacks, p)

				continue
			}
		}

		err := f(ctx)
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to run installation func for package %s", p)
		}
	}

	return updatedPacks, nil
}

func (e *extendedAPT) preInstallationSteps(ctx context.Context, packs ...string) ([]string, error) {
	updatedPacks := make([]string, 0, len(packs))

	for _, pack := range packs {
		switch pack {
		case PHPPackage:
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

func (e *extendedAPT) preRemovingSteps(ctx context.Context, packs ...string) error {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	for _, pack := range packs {
		if pack == MySQLServerPackage &&
			osInfo.Distribution == DistributionUbuntu &&
			utils.Contains([]string{"focal", "jammy", "kinetic", "lunar"}, osInfo.DistributionCodename) {
			err := e.Purge(ctx, "mysql-server-8.0")
			if err != nil {
				return err
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

//nolint:funlen
func (e *extendedAPT) findPHPPackages(ctx context.Context) ([]string, error) {
	var versionAvailable string
	log.Println("Searching for PHP packages...")

	var repoAdded bool
	for {
		log.Println("Checking for PHP 8.2 version available...")
		pk, err := e.apt.Search(ctx, "php8.2")
		if err != nil {
			return nil, err
		}
		if len(pk) > 0 {
			versionAvailable = "8.2"
			log.Println("PHP 8.2 version found")

			break
		}

		log.Println("PHP 8.2 version not found")

		log.Println("Checking for PHP 8.1 version available...")
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

	//nolint:goconst
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
	osInfo := contextInternal.OSInfoFromContext(ctx)

	err := utils.DownloadFile(ctx, "https://nginx.org/keys/nginx_signing.key", "/etc/apt/trusted.gpg.d/nginx.key")
	if err != nil {
		return err
	}

	err = utils.ExecCommand("gpg", "--keyserver", "keyserver.ubuntu.com", "--recv-keys", "ABF5BD827BD9BF62")
	if err != nil {
		return errors.WithMessage(err, "failed to receive nginx gpg key")
	}

	cmd := exec.Command("gpg", "--export", "ABF5BD827BD9BF62")
	log.Println('\n', cmd.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.WithMessage(err, "failed to export nginx gpg key")
	}
	//nolint:gosec
	err = os.WriteFile("/etc/apt/trusted.gpg.d/nginx.gpg", out, 0644)
	if err != nil {
		return errors.WithMessage(err, "failed to write nginx gpg key")
	}

	if osInfo.Distribution == DistributionUbuntu && !utils.IsFileExists(sourcesListNginx) {
		err := utils.WriteContentsToFile(
			[]byte(fmt.Sprintf("deb http://nginx.org/packages/ubuntu/ %s nginx", osInfo.DistributionCodename)),
			sourcesListNginx,
		)
		if err != nil {
			return err
		}
	}

	if osInfo.Distribution == DistributionDebian && !utils.IsFileExists(sourcesListNginx) {
		err := utils.WriteContentsToFile(
			[]byte(fmt.Sprintf("deb http://nginx.org/packages/debian/ %s nginx", osInfo.DistributionCodename)),
			sourcesListNginx,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *extendedAPT) addNodeJSRepositories(_ context.Context) error {
	var err error
	if !utils.IsFileExists("/etc/apt/keyrings") {
		err = os.Mkdir("/etc/apt/keyrings", 0755)
		if err != nil {
			return errors.WithMessage(err, "failed to create /etc/apt/keyrings directory")
		}
	}

	if !utils.IsFileExists("/etc/apt/keyrings/nodesource.gpg") {
		err = utils.ExecCommand(
			"bash", "-c",
			"curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key |"+
				" gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg",
		)
		if err != nil {
			return errors.WithMessage(err, "failed to receive nodejs gpg key")
		}
	}

	err = utils.WriteContentsToFile(
		[]byte("deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_16.x nodistro main"),
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

type installationFunc func(ctx context.Context) error

var installationFuncs = map[string]map[string]map[string]installationFunc{
	ComposerPackage: {
		Default: {
			ArchDefault: func(ctx context.Context) error {
				if utils.IsFileExists(filepath.Join(chrootPHPPath, packageMarkFile)) {
					return nil
				}

				err := utils.ExecCommand(
					"bash", "-c",
					"curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer",
				)
				if err != nil {
					return errors.WithMessage(err, "failed to install composer")
				}

				return nil
			},
		},
	},
}
