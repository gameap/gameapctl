//go:build !windows

package packagemanager

import "github.com/pkg/errors"

var errWindowsOnly = errors.New("windows service control manager is not available on this platform")

func windowsServiceBinaryPath(_ string) (string, error) {
	return "", errWindowsOnly
}
