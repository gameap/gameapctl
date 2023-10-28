package packagemanager

import "fmt"

type UnsupportedDistributionError struct {
	distro string
}

func NewErrUnsupportedDistribution(distro string) *UnsupportedDistributionError {
	return &UnsupportedDistributionError{
		distro: distro,
	}
}

func (e *UnsupportedDistributionError) Error() string {
	return fmt.Sprintf("unsupported distribution '%s'", e.distro)
}

type InvalidDirContentsError struct {
	path string
}

func NewErrInvalidDirContents(path string) *InvalidDirContentsError {
	return &InvalidDirContentsError{
		path: path,
	}
}

func (e *InvalidDirContentsError) Error() string {
	return fmt.Sprintf("invalid contents in '%s' directory", e.path)
}
