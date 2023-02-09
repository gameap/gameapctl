package packagemanager

import "fmt"

type ErrUnsupportedDistribution struct {
	distro string
}

func NewErrUnsupportedDistribution(distro string) *ErrUnsupportedDistribution {
	return &ErrUnsupportedDistribution{
		distro: distro,
	}
}

func (e *ErrUnsupportedDistribution) Error() string {
	return fmt.Sprintf("unsupported distribution '%s'", e.distro)
}

type ErrInvalidDirContents struct {
	path string
}

func NewErrInvalidDirContents(path string) *ErrInvalidDirContents {
	return &ErrInvalidDirContents{
		path: path,
	}
}

func (e *ErrInvalidDirContents) Error() string {
	return fmt.Sprintf("invalid contents in '%s' directory", e.path)
}
