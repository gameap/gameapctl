package packagemanager

import (
	"context"
	"os/exec"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	osinfo "github.com/gameap/gameapctl/pkg/os_info"
	"github.com/pkg/errors"
)

const (
	ConfigValueDBRootPassword = "db-root-password"
	ConfigValueDBUser         = "db-user"
	ConfigValueDBPassword     = "db-password"
	ConfigValueDBName         = "db-name"
)

type PackageInfo struct {
	Name            string
	Architecture    string
	Version         string
	Size            string
	Description     string
	InstalledSizeKB int
}

type installOptions struct {
	configValues map[string]string
}

type InstallOptions func(*installOptions)

func WithConfigValue(key, value string) InstallOptions {
	return func(opts *installOptions) {
		if opts.configValues == nil {
			opts.configValues = make(map[string]string)
		}
		opts.configValues[key] = value
	}
}

type PackageManager interface {
	Search(ctx context.Context, name string) ([]PackageInfo, error)
	Install(ctx context.Context, pack string, opts ...InstallOptions) error
	CheckForUpdates(ctx context.Context) error
	Remove(ctx context.Context, packs ...string) error
	Purge(ctx context.Context, packs ...string) error
}

//nolint:ireturn,nolintlint
func Load(ctx context.Context) (PackageManager, error) {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	switch osInfo.Distribution {
	case DistributionDebian:
		return loadDebianPackageManager(ctx, osInfo)
	case DistributionUbuntu:
		return loadUbuntuPackageManager(ctx, osInfo)
	case DistributionCentOS:
		return loadCentOSPackageManager(ctx, osInfo)
	case DistributionWindows:
		return NewWindowsPackageManager(ctx, osInfo)
	}

	return detectAndLoadPackageManager(ctx, osInfo)
}

//nolint:ireturn,nolintlint
func loadDebianPackageManager(_ context.Context, osInfo osinfo.Info) (PackageManager, error) {
	switch osInfo.DistributionCodename {
	case "buster", "bullseye", "bookworm":
		return newExtendedAPT(osInfo, &apt{})
	default:
		pm, err := newExtendedAPT(osInfo, &apt{})
		if err != nil {
			return nil, err
		}

		// other distributions with fallback
		return newFallbackPackageManager(
			pm,
			newChRoot(),
		), nil
	}
}

//nolint:ireturn,nolintlint
func loadUbuntuPackageManager(_ context.Context, osInfo osinfo.Info) (PackageManager, error) {
	switch osInfo.DistributionCodename {
	case "focal", "jammy":
		return newExtendedAPT(osInfo, &apt{})
	default:
		pm, err := newExtendedAPT(osInfo, &apt{})
		if err != nil {
			return nil, errors.WithMessage(err, "failed to load ubuntu package")
		}

		// other distributions with fallback
		return newFallbackPackageManager(
			pm,
			newChRoot(),
		), nil
	}
}

//nolint:ireturn,nolintlint
func loadCentOSPackageManager(
	_ context.Context,
	osinfo osinfo.Info,
) (PackageManager, error) {
	if _, err := exec.LookPath("dnf"); err == nil {
		return newExtendedDNF(osinfo, &dnf{})
	}

	return newExtendedDNF(osinfo, &yum{})
}

//nolint:ireturn,nolintlint
func detectAndLoadPackageManager(
	_ context.Context, osinfo osinfo.Info,
) (PackageManager, error) {
	if _, err := exec.LookPath("apt"); err == nil {
		return newExtendedAPT(osinfo, &apt{})
	}

	if _, err := exec.LookPath("dnf"); err == nil {
		return newExtendedDNF(osinfo, &dnf{})
	}

	if _, err := exec.LookPath("yum"); err == nil {
		return newExtendedDNF(osinfo, &yum{})
	}

	return nil, NewErrUnsupportedDistribution(string(osinfo.Distribution))
}
