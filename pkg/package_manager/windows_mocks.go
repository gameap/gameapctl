//go:build !windows

package packagemanager

import (
	"context"
	"errors"

	osinfo "github.com/gameap/gameapctl/pkg/os_info"
)

var errNotAvailableForNonWindows = errors.New("package manager is not available for non-Windows OS")

type WindowsPackageManager struct{}

func NewWindowsPackageManager(_ context.Context, _ osinfo.Info) (*WindowsPackageManager, error) {
	return &WindowsPackageManager{}, nil
}

func (pm *WindowsPackageManager) Search(_ context.Context, _ string) ([]PackageInfo, error) {
	return nil, errNotAvailableForNonWindows
}

func (pm *WindowsPackageManager) Install(_ context.Context, _ ...string) error {
	return errNotAvailableForNonWindows
}

func (pm *WindowsPackageManager) CheckForUpdates(_ context.Context) error {
	return errNotAvailableForNonWindows
}

func (pm *WindowsPackageManager) Remove(_ context.Context, _ ...string) error {
	return errNotAvailableForNonWindows
}

func (pm *WindowsPackageManager) Purge(_ context.Context, _ ...string) error {
	return errNotAvailableForNonWindows
}
