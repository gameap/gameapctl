package releasesource

import (
	"cmp"
	"context"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/pkg/errors"
)

const (
	probeTimeout       = 5 * time.Second
	findAttemptTimeout = time.Minute
)

type selector struct {
	sources []source
	client  *http.Client

	once    sync.Once
	ordered []source
}

func newSelector(sources []source) *selector {
	return &selector{
		sources: sources,
		client:  &http.Client{Timeout: probeTimeout},
	}
}

var defaultSelector = newSelector(defaultSources())

func (s *selector) findRelease(
	ctx context.Context, component Component, kernel, platform string, opts releasefinder.FindOptions,
) (*Release, error) {
	if _, ok := githubReleasesPath[component]; !ok {
		return nil, errors.Errorf("unknown component %q", component)
	}

	srcs, err := s.sourcesForFind(ctx)
	if err != nil {
		return nil, err
	}

	var firstErr, contentErr error
	githubRateLimited := false

	for i, src := range srcs {
		rel, findErr := s.findFromSource(ctx, src, component, kernel, platform, opts, len(srcs) > 1)
		if findErr == nil {
			log.Printf("Resolved %s release %s via %s", component, rel.Tag, src.name)

			return buildRelease(src, srcs, component, rel), nil
		}

		if ctx.Err() != nil {
			return nil, findErr
		}

		if src.kind == kindGitHub && releasefinder.IsRateLimitError(findErr) {
			githubRateLimited = true
		}
		if firstErr == nil {
			firstErr = findErr
		}
		if contentErr == nil && isContentError(findErr) {
			contentErr = findErr
		}
		if i < len(srcs)-1 {
			log.Printf("Release source %s failed: %v; trying next source", src.name, findErr)
		}
	}

	// A content error (missing tag or platform asset) is a definitive answer
	// from a complete mirror — waiting out the GitHub rate limit cannot
	// improve on it.
	if githubRateLimited && contentErr == nil {
		if release, retryErr := s.retryGitHub(ctx, srcs, component, kernel, platform, opts); retryErr == nil {
			return release, nil
		}
	}

	if contentErr != nil {
		return nil, contentErr
	}

	return nil, firstErr
}

// findFromSource runs a single bounded resolution attempt. With quickFail set,
// a rate-limited GitHub costs one request before the caller moves on to the
// next source.
func (s *selector) findFromSource(
	ctx context.Context,
	src source,
	component Component,
	kernel, platform string,
	opts releasefinder.FindOptions,
	quickFail bool,
) (*releasefinder.Release, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, findAttemptTimeout)
	defer cancel()

	if src.kind == kindGitHub {
		githubOpts := opts
		if quickFail {
			githubOpts.NoRetry = true
		}

		return releasefinder.FindWithOptions(attemptCtx, src.releasesURL(component), kernel, platform, githubOpts)
	}

	return releasefinder.FindFromStaticList(attemptCtx, src.releasesURL(component), kernel, platform, opts)
}

// retryGitHub restores the full rate-limit-aware retry behavior for the case
// when GitHub is rate-limited and no other source succeeded.
func (s *selector) retryGitHub(
	ctx context.Context, srcs []source, component Component, kernel, platform string, opts releasefinder.FindOptions,
) (*Release, error) {
	for _, src := range srcs {
		if src.kind != kindGitHub {
			continue
		}

		log.Println("All release sources failed, waiting for GitHub rate limit reset...")

		rel, err := releasefinder.FindWithOptions(ctx, src.releasesURL(component), kernel, platform, opts)
		if err != nil {
			return nil, err
		}

		return buildRelease(src, srcs, component, rel), nil
	}

	return nil, errors.New("no GitHub source configured")
}

func buildRelease(chosen source, ordered []source, component Component, rel *releasefinder.Release) *Release {
	urls := make([]string, 0, len(ordered))
	urls = append(urls, chosen.downloadURL(component, rel))
	for _, src := range ordered {
		if src.name == chosen.name {
			continue
		}
		urls = append(urls, src.downloadURL(component, rel))
	}

	return &Release{Tag: rel.Tag, AssetName: rel.AssetName, URLs: urls}
}

func isContentError(err error) bool {
	var tagNotFound releasefinder.TagNotFoundError
	if errors.As(err, &tagNotFound) {
		return true
	}

	var failedToFind releasefinder.FailedToFindReleaseError

	return errors.As(err, &failedToFind)
}

func (s *selector) sourcesForFind(ctx context.Context) ([]source, error) {
	forced := os.Getenv(EnvSource)
	if forced == "" {
		return s.orderedSources(ctx), nil
	}

	for _, src := range s.sources {
		if src.name == forced {
			log.Printf("Release source forced to %s via %s", src.name, EnvSource)

			return []source{src}, nil
		}
	}

	return nil, errors.Errorf(
		"unknown %s value %q (expected one of: %s)",
		EnvSource, forced, strings.Join(sourceNames(s.sources), ", "),
	)
}

// orderedSources probes all sources in parallel and memoizes the resulting
// order for the rest of the process. Probes only affect the order: unavailable
// sources are appended after available ones and still act as fallbacks.
func (s *selector) orderedSources(ctx context.Context) []source {
	s.once.Do(func() {
		s.ordered = s.probeAndSort(ctx)
	})

	return s.ordered
}

type probeResult struct {
	src       source
	latency   time.Duration
	available bool
}

func (s *selector) probeAndSort(ctx context.Context) []source {
	results := make([]probeResult, len(s.sources))

	var wg sync.WaitGroup
	for i, src := range s.sources {
		wg.Add(1)
		go func(i int, src source) {
			defer wg.Done()
			results[i] = s.probe(ctx, src)
		}(i, src)
	}
	wg.Wait()

	available := make([]probeResult, 0, len(results))
	unavailable := make([]source, 0, len(results))
	for _, r := range results {
		if r.available {
			available = append(available, r)
		} else {
			unavailable = append(unavailable, r.src)
		}
	}

	slices.SortStableFunc(available, func(a, b probeResult) int {
		return cmp.Compare(a.latency, b.latency)
	})

	ordered := make([]source, 0, len(results))
	for _, r := range available {
		ordered = append(ordered, r.src)
	}
	ordered = append(ordered, unavailable...)

	log.Println("Release sources order:", strings.Join(sourceNames(ordered), ", "))

	return ordered
}

func (s *selector) probe(ctx context.Context, src source) probeResult {
	method, probeURL := src.probeRequest()

	req, err := http.NewRequestWithContext(ctx, method, probeURL, nil)
	if err != nil {
		log.Println("Release source", src.name, "probe failed:", err)

		return probeResult{src: src}
	}

	started := time.Now()

	resp, err := s.client.Do(req)
	if err != nil {
		log.Println("Release source", src.name, "is not available:", err)

		return probeResult{src: src}
	}

	latency := time.Since(started)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("Release source", src.name, "responded with status", resp.StatusCode)

		return probeResult{src: src}
	}

	// The rate_limit endpoint responds with 200 even when the quota is
	// exhausted; deprioritize GitHub so a mirror is tried first.
	if src.kind == kindGitHub && resp.Header.Get("X-RateLimit-Remaining") == "0" {
		log.Println("Release source", src.name, "is rate limited")

		return probeResult{src: src}
	}

	log.Println("Release source", src.name, "latency:", latency.Round(time.Millisecond))

	return probeResult{src: src, latency: latency, available: true}
}

func sourceNames(srcs []source) []string {
	names := make([]string, 0, len(srcs))
	for _, src := range srcs {
		names = append(names, src.name)
	}

	return names
}
