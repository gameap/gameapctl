package packagemanager

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/gameap/gameapctl/pkg/gameap"
	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

type chRoot struct{}

func newChRoot() *chRoot {
	return &chRoot{}
}

func (ch *chRoot) Search(ctx context.Context, name string) ([]PackageInfo, error) {
	osInfo := contextInternal.OSInfoFromContext(ctx)
	var packages []PackageInfo

	if _, ok := chrootPackages[name]; !ok {
		return packages, nil
	}

	if _, ok := chrootPackages[name][osInfo.Platform]; !ok {
		return packages, nil
	}

	packages = append(packages, chrootPackages[name][osInfo.Platform].PackageInfo)

	return packages, nil
}

func (ch *chRoot) Install(ctx context.Context, packs ...string) error {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	for _, pack := range packs {
		if _, ok := skipChrootPackages[pack]; ok {
			continue
		}

		if _, ok := chrootPackages[pack]; !ok {
			return NewErrPackageNotFound(pack)
		}

		if _, ok := chrootPackages[pack][osInfo.Platform]; !ok {
			return NewErrPackageNotFound(pack)
		}
	}

	for _, pack := range packs {
		err := ch.installPackage(ctx, pack)
		if err != nil {
			return errors.WithMessagef(err, "failed to install %s package", pack)
		}
		log.Println("Package", pack, "installed")
	}

	return nil
}

//nolint:funlen
func (ch *chRoot) installPackage(ctx context.Context, pack string) error {
	if _, ok := skipChrootPackages[pack]; ok {
		return nil
	}

	osInfo := contextInternal.OSInfoFromContext(ctx)

	if _, ok := chrootPackages[pack]; !ok {
		return NewErrPackageNotFound(pack)
	}

	p, ok := chrootPackages[pack][osInfo.Platform]
	if !ok {
		return NewErrPackageNotFound(pack)
	}

	if _, err := os.Stat(filepath.Join(p.InstallationPath, packageMarkFile)); err == nil {
		// Package already installed
		return nil
	}

	tmpDir, err := os.MkdirTemp("", "gameapctl-package-manager")
	if err != nil {
		return errors.WithMessage(err, "failed to create temp dir")
	}
	defer func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			log.Println(err)
		}
	}()

	log.Println("Downloading ", p.ArchiveURL)
	err = utils.DownloadFileOrArchive(
		ctx,
		p.ArchiveURL,
		filepath.Join(tmpDir, path.Base(p.ArchiveURL)),
	)
	if err != nil {
		return errors.WithMessage(err, "failed to download chroot package")
	}

	err = os.MkdirAll(p.InstallationPath, 0755)
	if err != nil {
		return errors.WithMessage(err, "failed to create chroot package installation path")
	}

	log.Println("Extracting ", path.Base(p.ArchiveURL))
	err = utils.ExecCommand(
		"tar", "-xvf", filepath.Join(tmpDir, path.Base(p.ArchiveURL)), "-C", p.InstallationPath,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to extract chroot package")
	}

	log.Println("Downloading ", p.SystemdUnitURL)
	err = utils.DownloadFile(
		ctx,
		p.SystemdUnitURL,
		filepath.Join("/etc/systemd/system", path.Base(p.SystemdUnitURL)),
	)
	if err != nil {
		return errors.WithMessage(err, "failed to download chroot systemd unit")
	}

	if p.AfterInstallFunc != nil {
		err = p.AfterInstallFunc(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to run after install function")
		}
	}

	return nil
}

func (ch *chRoot) CheckForUpdates(_ context.Context) error {
	return nil
}

func (ch *chRoot) Remove(ctx context.Context, packs ...string) error {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	for _, pack := range packs {
		if _, ok := chrootPackages[pack]; !ok {
			return NewErrPackageNotFound(pack)
		}

		if _, ok := chrootPackages[pack][osInfo.Platform]; !ok {
			return NewErrPackageNotFound(pack)
		}
	}

	for _, pack := range packs {
		err := ch.removePackage(ctx, pack)
		if err != nil {
			return errors.WithMessagef(err, "failed to remove %s package", pack)
		}
		log.Println("Package", pack, "removed")
	}

	return nil
}

func (ch *chRoot) removePackage(ctx context.Context, pack string) error {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	if _, ok := chrootPackages[pack]; !ok {
		return NewErrPackageNotFound(pack)
	}

	p, ok := chrootPackages[pack][osInfo.Platform]
	if !ok {
		return NewErrPackageNotFound(pack)
	}

	if _, err := os.Stat(p.InstallationPath); errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	if _, err := os.Stat(filepath.Join(p.InstallationPath, packageMarkFile)); errors.Is(err, fs.ErrNotExist) {
		return errors.New("package is not marked as installed by gameapctl")
	}

	log.Println("Removing ", p.InstallationPath)
	err := os.RemoveAll(p.InstallationPath)
	if err != nil {
		return errors.WithMessage(err, "failed to remove chroot package")
	}

	systemdUnitPath := filepath.Join("/etc/systemd/system", path.Base(p.SystemdUnitURL))

	log.Println("Removing ", systemdUnitPath)
	err = os.Remove(systemdUnitPath)
	if err != nil {
		return errors.WithMessage(err, "failed to remove chroot systemd unit")
	}

	return nil
}

func (ch *chRoot) Purge(ctx context.Context, packs ...string) error {
	return ch.Remove(ctx, packs...)
}

type chrootPackage struct {
	ArchiveURL       string
	SystemdUnitURL   string
	InstallationPath string
	PackageInfo      PackageInfo
	AfterInstallFunc func(ctx context.Context) error
}

//nolint:mnd
var chrootPackages = map[string]map[osinfo.Platform]chrootPackage{
	PHPPackage: {
		ArchAMD64: {
			ArchiveURL:       gameap.Repository() + "/chroots/php/php8.1-amd64.tar.gz",
			SystemdUnitURL:   gameap.Repository() + "/chroots/php/php8.1-fpm.service",
			InstallationPath: chrootPHPPath,
			PackageInfo: PackageInfo{
				Name:            "php",
				Architecture:    ArchAMD64,
				Version:         "8.1",
				Size:            "49 MB",
				InstalledSizeKB: 72000,
				Description:     "PHP 8.1",
			},
		},
	},
}

// Package will be skipped from chroot installation without errors.
var skipChrootPackages = map[string]struct{}{
	PHPExtensionsPackage: {}, // PHP extensions will be installed with PHP
}
