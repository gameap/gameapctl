package packagemanager

import "fmt"

type UnsupportedDistributionError string

func NewErrUnsupportedDistribution(distro string) UnsupportedDistributionError {
	return UnsupportedDistributionError(distro)
}

func (e UnsupportedDistributionError) Error() string {
	return fmt.Sprintf("unsupported distribution '%s'", string(e))
}

type InvalidDirContentsError string

func NewErrInvalidDirContents(path string) InvalidDirContentsError {
	return InvalidDirContentsError(path)
}

func (e InvalidDirContentsError) Error() string {
	return fmt.Sprintf("invalid contents in '%s' directory", string(e))
}

type NotFoundError string

func NewErrNotFound(e string) NotFoundError {
	return NotFoundError(e)
}

func (e NotFoundError) Error() string {
	return string(e)
}

type PackageNotFoundError NotFoundError

func NewErrPackageNotFound(name string) PackageNotFoundError {
	return PackageNotFoundError(name)
}

func (e PackageNotFoundError) Error() string {
	return fmt.Sprintf("package '%s' not found", string(e))
}
