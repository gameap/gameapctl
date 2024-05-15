package packagemanager

import (
	"context"
	"os/exec"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	osinfo "github.com/gameap/gameapctl/pkg/os_info"
)

type PackageInfo struct {
	Name            string
	Architecture    string
	Version         string
	Size            string
	Description     string
	InstalledSizeKB int
}

type PackageManager interface {
	Search(ctx context.Context, name string) ([]PackageInfo, error)
	Install(ctx context.Context, packs ...string) error
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
		return NewWindowsPackageManager(), nil
	}

	return detectAndLoadPackageManager(ctx, osInfo)
}

//nolint:ireturn,nolintlint
func loadDebianPackageManager(_ context.Context, osInfo osinfo.Info) (PackageManager, error) {
	switch osInfo.DistributionCodename {
	case "buster", "bullseye", "bookworm":
		return newExtendedAPT(&apt{}), nil
	default:
		// other distributions with fallback
		return newFallbackPackageManager(
			newExtendedAPT(&apt{}),
			newChRoot(),
		), nil
	}
}

//nolint:ireturn,nolintlint
func loadUbuntuPackageManager(_ context.Context, osInfo osinfo.Info) (PackageManager, error) {
	switch osInfo.DistributionCodename {
	case "focal", "jammy":
		return newExtendedAPT(&apt{}), nil
	default:
		// other distributions with fallback
		return newFallbackPackageManager(
			newExtendedAPT(&apt{}),
			newChRoot(),
		), nil
	}
}

//nolint:ireturn,nolintlint
func loadCentOSPackageManager(
	_ context.Context,
	_ osinfo.Info,
) (PackageManager, error) {
	if _, err := exec.LookPath("dnf"); err == nil {
		return newExtendedDNF(&dnf{}), nil
	}

	return newExtendedDNF(&yum{}), nil
}

//nolint:ireturn,nolintlint
func detectAndLoadPackageManager(
	_ context.Context, osInfo osinfo.Info,
) (PackageManager, error) {
	if _, err := exec.LookPath("apt"); err == nil {
		return newExtendedAPT(&apt{}), nil
	}

	if _, err := exec.LookPath("dnf"); err == nil {
		return newExtendedDNF(&dnf{}), nil
	}

	if _, err := exec.LookPath("yum"); err == nil {
		return newExtendedDNF(&yum{}), nil
	}

	return nil, NewErrUnsupportedDistribution(string(osInfo.Distribution))
}
