package panel

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gameap/gameapctl/pkg/releasefinder"
)

// The GameAP plugin store and its mirror, which serves the same API for regions
// where the primary host is unreachable.
const (
	PluginsStoreURL       = "https://plugins.gameap.dev/api"
	PluginsStoreMirrorURL = "https://plugins.gameap.ru/api"
)

const (
	pluginsStoreProbeTimeout = 5 * time.Second

	pluginsStoreKey       = "PLUGINS_STORE_URL"
	pluginsStoreLegacyKey = "PLUGIN_STORE_URL"

	// pluginsStoreKeyRenamedIn is the panel release that started reading
	// pluginsStoreKey instead of pluginsStoreLegacyKey.
	pluginsStoreKeyRenamedIn = "v4.5"
)

var pluginsStoreAlternatives = map[string]string{
	PluginsStoreURL:       PluginsStoreMirrorURL,
	PluginsStoreMirrorURL: PluginsStoreURL,
}

// PluginsStoreAvailability reports which GameAP plugin stores answered the
// health probe, keyed by store URL.
type PluginsStoreAvailability map[string]bool

// ProbePluginsStores checks the GameAP plugin store and its mirror in parallel.
func ProbePluginsStores(ctx context.Context) PluginsStoreAvailability {
	client := &http.Client{Timeout: pluginsStoreProbeTimeout}

	availability := probePluginsStores(ctx, client, []string{PluginsStoreURL, PluginsStoreMirrorURL})

	if !availability[PluginsStoreURL] && !availability[PluginsStoreMirrorURL] {
		log.Printf(
			"Warning: neither %s nor %s is reachable, plugin store falls back to %s\n",
			PluginsStoreURL, PluginsStoreMirrorURL, PluginsStoreURL,
		)
	}

	return availability
}

func probePluginsStores(ctx context.Context, client *http.Client, urls []string) PluginsStoreAvailability {
	results := make([]bool, len(urls))

	var wg sync.WaitGroup
	for i, storeURL := range urls {
		wg.Add(1)
		go func(i int, storeURL string) {
			defer wg.Done()
			results[i] = probePluginsStore(ctx, client, storeURL)
		}(i, storeURL)
	}
	wg.Wait()

	availability := make(PluginsStoreAvailability, len(urls))
	for i, storeURL := range urls {
		availability[storeURL] = results[i]
	}

	return availability
}

func probePluginsStore(ctx context.Context, client *http.Client, storeURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, storeURL+"/health", nil)
	if err != nil {
		log.Println("Plugin store", storeURL, "probe failed:", err)

		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Plugin store", storeURL, "is not available:", err)

		return false
	}

	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("Plugin store", storeURL, "responded with status", resp.StatusCode)

		return false
	}

	log.Println("Plugin store", storeURL, "is available")

	return true
}

// choose picks the plugin store address to configure, given the one already
// configured (empty when there is none). With nothing configured the primary
// store wins unless only the mirror answers. A configured GameAP store is
// swapped for the other one only when it is unreachable and the other one
// answers. An address the operator set by hand is always kept.
func (a PluginsStoreAvailability) choose(current string) string {
	normalized := strings.TrimSuffix(strings.TrimSpace(current), "/")

	if normalized == "" {
		if !a[PluginsStoreURL] && a[PluginsStoreMirrorURL] {
			return PluginsStoreMirrorURL
		}

		return PluginsStoreURL
	}

	alternative, known := pluginsStoreAlternatives[normalized]
	if known && !a[normalized] && a[alternative] {
		return alternative
	}

	return current
}

// pluginsStoreKeyFor returns the config.env key the panel release tag reads the
// store address from. A tag without a version, such as a GitHub branch build,
// is newer than every release.
func pluginsStoreKeyFor(tag string) string {
	if releasefinder.HasMajorMinor(tag) && !releasefinder.IsAtLeast(tag, pluginsStoreKeyRenamedIn) {
		return pluginsStoreLegacyKey
	}

	return pluginsStoreKey
}
