package packagemanager

import (
	"context"
	"fmt"
	"log"
	"os/exec"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/gameap/gameapctl/pkg/package_manager/pkgconfig"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

// aptStrategy implements packageInstallStrategy with the apt-specific logic:
// dependency installation, PHP/Apache repository handling and MySQL purge on
// removal.
type aptStrategy struct {
	packages map[string]pkgconfig.PackageConfig
	apt      PackageManager
}

func (s *aptStrategy) installDependencies(ctx context.Context, packs ...string) error {
	dependencies := make([]string, 0)

	for _, packName := range packs {
		config, exists := s.packages[packName]
		if !exists {
			continue
		}

		dependencies = append(dependencies, config.Dependencies...)
	}

	if len(dependencies) == 0 {
		return nil
	}

	for _, dep := range dependencies {
		err := s.apt.Install(ctx, dep)
		if err != nil {
			return errors.WithMessage(err, "failed to install dependencies")
		}
	}

	return nil
}

func (s *aptStrategy) transformPreInstallPackages(ctx context.Context, packs []string) ([]string, error) {
	updatedPacks := make([]string, 0, len(packs))

	// Hardcode. To be refactored later.
	for _, pack := range packs {
		switch pack {
		case PHPPackage, PHPExtensionsPackage:
			err := s.installAPTRepositoriesDependencies(ctx)
			if err != nil {
				return nil, err
			}

			packages, err := s.findPHPPackages(ctx)
			if err != nil {
				return nil, err
			}

			updatedPacks = append(updatedPacks, packages...)
		case ApachePackage:
			err := s.apachePackageProcess(ctx)
			if err != nil {
				return nil, errors.WithMessage(err, "failed to process apache packages")
			}
		default:
			updatedPacks = append(updatedPacks, pack)
		}
	}

	return updatedPacks, nil
}

func (s *aptStrategy) preRemove(ctx context.Context, packs ...string) error {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	for _, pack := range packs {
		if pack == MySQLServerPackage &&
			(osInfo.Distribution == DistributionUbuntu || osInfo.Distribution == DistributionDebian) {
			err := s.apt.Purge(ctx, "mysql*")
			if err != nil {
				return errors.WithMessage(err, "failed to purge mysql packages")
			}
		}
	}

	return nil
}

func (s *aptStrategy) installAPTRepositoriesDependencies(ctx context.Context) error {
	installPackages := make([]string, 0, 2)

	pk, err := s.apt.Search(ctx, "software-properties-common")
	if err != nil {
		return err
	}
	if len(pk) > 0 {
		installPackages = append(installPackages, "software-properties-common")
	}

	pk, err = s.apt.Search(ctx, "apt-transport-https")
	if err != nil {
		return err
	}
	if len(pk) > 0 {
		installPackages = append(installPackages, "apt-transport-https")
	}

	for _, pkg := range installPackages {
		err = s.apt.Install(ctx, pkg)
		if err != nil {
			return err
		}
	}

	return nil
}

//nolint:gocognit,funlen
func (s *aptStrategy) findPHPPackages(ctx context.Context) ([]string, error) {
	var versionAvailable string
	log.Println("Searching for PHP packages...")

	var repoAdded bool
	for {
		log.Println("Checking for PHP 8.4 version availability...")
		pk, err := s.apt.Search(ctx, "php8.4")
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
		pk, err = s.apt.Search(ctx, "php8.3")
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
		pk, err = s.apt.Search(ctx, "php8.2")
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
		pk, err = s.apt.Search(ctx, "php8.1")
		if err != nil {
			return nil, err
		}
		if len(pk) > 0 {
			versionAvailable = "8.1"
			log.Println("PHP 8.1 version found")

			break
		}

		log.Println("PHP 8.1 version not found")

		pk, err = s.apt.Search(ctx, "php8.0")
		if err != nil {
			return nil, err
		}
		if len(pk) > 0 {
			versionAvailable = "8.0"
			log.Println("PHP 8.0 version found")

			break
		}

		log.Println("PHP 8.0 version not found")

		pk, err = s.apt.Search(ctx, "php7.4")
		if err != nil {
			return nil, err
		}
		if len(pk) > 0 {
			versionAvailable = "7.4"
			log.Println("PHP 7.4 version found")

			break
		}

		log.Println("PHP 7.4 version not found")

		pk, err = s.apt.Search(ctx, "php7.3")
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

		repoAdded, err = s.addPHPRepositories(ctx)
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

func (s *aptStrategy) addPHPRepositories(ctx context.Context) (bool, error) {
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

func (s *aptStrategy) apachePackageProcess(ctx context.Context) error {
	phpVersion, err := DefinePHPVersion()
	if err != nil {
		return errors.WithMessage(err, "failed to define php version")
	}

	err = s.apt.Install(ctx, ApachePackage)
	if err != nil {
		return err
	}
	err = s.apt.Install(ctx, "libapache2-mod-php"+phpVersion)
	if err != nil {
		return err
	}

	return nil
}
