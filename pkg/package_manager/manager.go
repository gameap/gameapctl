package packagemanager

import (
	"context"

	contextInternal "github.com/gameap/gameapctl/internal/context"
)

type Package struct {
	Name             string
	Status           string
	Architecture     string
	Version          string
	ShortDescription string
	InstalledSizeKB  int
}

type PackageManager interface {
	Search(ctx context.Context, name string) ([]*Package, error)
	Install(ctx context.Context, packs ...string) (output []byte, err error)
	CheckForUpdates(ctx context.Context) (output []byte, err error)
	Remove(ctx context.Context, packs ...string) (output []byte, err error)
}

func Load(ctx context.Context) (PackageManager, error) {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	switch osInfo.Distribution {
	case "debian", "ubuntu":
		return NewExtendedAPT(&APT{}), nil
	}
	return nil, nil
}
