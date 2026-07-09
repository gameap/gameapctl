// Package releasesource resolves and downloads GameAP component releases from
// the best available source: the GitHub releases API or one of the CDN mirrors
// (cdn.gameap.com, cdn.gameap.ru). Sources are probed in parallel once per
// process and used in latency order with fallback to the next source.
package releasesource

import (
	"context"
	"log"

	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

// EnvSource forces a specific release source ("github", "cdn.gameap.com",
// "cdn.gameap.ru") and disables probing and fallback.
const EnvSource = "GAMEAP_RELEASE_SOURCE"

type Release struct {
	Tag       string
	AssetName string
	// URLs holds candidate download locations for the same asset, ordered by
	// preference: the source that resolved the release first, then the
	// remaining sources in probed order.
	URLs []string
}

func (r *Release) PrimaryURL() string {
	if len(r.URLs) == 0 {
		return ""
	}

	return r.URLs[0]
}

// FindRelease resolves a component release using the best available source.
func FindRelease(
	ctx context.Context, component Component, kernel, platform string, opts releasefinder.FindOptions,
) (*Release, error) {
	return defaultSelector.findRelease(ctx, component, kernel, platform, opts)
}

// Download fetches and extracts the release archive into the dst directory,
// trying candidate URLs in order.
func Download(ctx context.Context, release *Release, dst string) error {
	return downloadWith(ctx, release, dst, utils.Download)
}

// DownloadFile fetches the release asset as a single file into dst, trying
// candidate URLs in order.
func DownloadFile(ctx context.Context, release *Release, dst string) error {
	return downloadWith(ctx, release, dst, utils.DownloadFile)
}

func downloadWith(
	ctx context.Context,
	release *Release,
	dst string,
	downloadFunc func(ctx context.Context, source string, dst string) error,
) error {
	if release == nil || len(release.URLs) == 0 {
		return errors.New("release has no download URLs")
	}

	var lastErr error
	for _, u := range release.URLs {
		err := downloadFunc(ctx, u, dst)
		if err == nil {
			return nil
		}

		lastErr = err
		if ctx.Err() != nil {
			return lastErr
		}
		log.Printf("Download from %s failed: %v", u, err)
	}

	return errors.WithMessage(lastErr, "failed to download from all sources")
}
