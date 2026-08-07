package releasesource

import (
	"context"
	"sync"
)

// CDNAvailability probes the GameAP CDN mirrors in parallel and reports
// availability by host name (CDNGameAPCom, CDNGameAPRu). The
// GAMEAP_RELEASE_SOURCE override is intentionally ignored: it forces the
// release download source, while this probe reflects real CDN reachability.
func CDNAvailability(ctx context.Context) map[string]bool {
	return defaultSelector.cdnAvailability(ctx)
}

func (s *selector) cdnAvailability(ctx context.Context) map[string]bool {
	cdns := make([]source, 0, len(s.sources))
	for _, src := range s.sources {
		if src.kind == kindCDN {
			cdns = append(cdns, src)
		}
	}

	results := make([]probeResult, len(cdns))

	var wg sync.WaitGroup
	for i, src := range cdns {
		wg.Add(1)
		go func(i int, src source) {
			defer wg.Done()
			results[i] = s.probe(ctx, src)
		}(i, src)
	}
	wg.Wait()

	availability := make(map[string]bool, len(results))
	for _, result := range results {
		availability[result.src.name] = result.available
	}

	return availability
}
