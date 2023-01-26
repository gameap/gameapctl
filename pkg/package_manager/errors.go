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
	return fmt.Sprintf("unsupported distributin '%s'", e.distro)
}
