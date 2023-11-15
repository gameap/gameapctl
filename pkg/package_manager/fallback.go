package packagemanager

import (
	"context"

	"go.uber.org/multierr"
)

type fallback struct {
	basePackageManager     PackageManager
	fallbackPackageManager PackageManager
}

//nolint:ireturn,nolintlint
func newFallbackPackageManager(basePackageManager, fallbackPackageManager PackageManager) PackageManager {
	return &fallback{
		basePackageManager:     basePackageManager,
		fallbackPackageManager: fallbackPackageManager,
	}
}

func (fb *fallback) Search(ctx context.Context, name string) ([]PackageInfo, error) {
	var err error
	bpackages, berr := fb.basePackageManager.Search(ctx, name)
	if berr != nil {
		err = multierr.Append(err, berr)
	}

	fpackages, ferr := fb.fallbackPackageManager.Search(ctx, name)
	if ferr != nil {
		err = multierr.Append(err, ferr)

		return nil, err
	}

	return append(bpackages, fpackages...), nil
}

func (fb *fallback) Install(ctx context.Context, packs ...string) error {
	if err := fb.basePackageManager.Install(ctx, packs...); err != nil {
		if ferr := fb.fallbackPackageManager.Install(ctx, packs...); ferr != nil {
			return multierr.Append(err, ferr)
		}
	}

	return nil
}

func (fb *fallback) CheckForUpdates(ctx context.Context) error {
	if err := fb.basePackageManager.CheckForUpdates(ctx); err != nil {
		if ferr := fb.fallbackPackageManager.CheckForUpdates(ctx); ferr != nil {
			return multierr.Append(err, ferr)
		}
	}

	return nil
}

func (fb *fallback) Remove(ctx context.Context, packs ...string) error {
	return fb.basePackageManager.Remove(ctx, packs...)
}

func (fb *fallback) Purge(ctx context.Context, packs ...string) error {
	return fb.basePackageManager.Purge(ctx, packs...)
}
