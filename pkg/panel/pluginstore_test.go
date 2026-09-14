package panel

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProbePluginsStores(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			http.NotFound(w, r)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(healthy.Close)

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(failing.Close)

	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()

	healthyStore := healthy.URL + "/api"
	wrongPath := healthy.URL + "/other"
	failingStore := failing.URL + "/api"
	closedStore := closed.URL + "/api"

	client := &http.Client{Timeout: pluginsStoreProbeTimeout}

	availability := probePluginsStores(t.Context(), client, []string{healthyStore, wrongPath, failingStore, closedStore})

	assert.Equal(t, PluginsStoreAvailability{
		healthyStore: true,
		wrongPath:    false,
		failingStore: false,
		closedStore:  false,
	}, availability)
}

func TestPluginsStoreAvailability_Choose(t *testing.T) {
	const custom = "https://plugins.example.com/api"

	both := PluginsStoreAvailability{PluginsStoreURL: true, PluginsStoreMirrorURL: true}
	onlyPrimary := PluginsStoreAvailability{PluginsStoreURL: true, PluginsStoreMirrorURL: false}
	onlyMirror := PluginsStoreAvailability{PluginsStoreURL: false, PluginsStoreMirrorURL: true}
	none := PluginsStoreAvailability{PluginsStoreURL: false, PluginsStoreMirrorURL: false}

	tests := []struct {
		name         string
		availability PluginsStoreAvailability
		current      string
		want         string
	}{
		{name: "nothing_configured_both_reachable", availability: both, want: PluginsStoreURL},
		{name: "nothing_configured_only_primary_reachable", availability: onlyPrimary, want: PluginsStoreURL},
		{name: "nothing_configured_only_mirror_reachable", availability: onlyMirror, want: PluginsStoreMirrorURL},
		{name: "nothing_configured_none_reachable", availability: none, want: PluginsStoreURL},
		{name: "primary_configured_and_reachable", availability: both, current: PluginsStoreURL, want: PluginsStoreURL},
		{name: "primary_configured_but_unreachable", availability: onlyMirror, current: PluginsStoreURL, want: PluginsStoreMirrorURL},
		{name: "primary_with_trailing_slash_unreachable", availability: onlyMirror, current: PluginsStoreURL + "/", want: PluginsStoreMirrorURL},
		{name: "mirror_configured_and_reachable", availability: both, current: PluginsStoreMirrorURL, want: PluginsStoreMirrorURL},
		{name: "mirror_configured_but_unreachable", availability: onlyPrimary, current: PluginsStoreMirrorURL, want: PluginsStoreURL},
		{name: "configured_store_kept_when_none_reachable", availability: none, current: PluginsStoreMirrorURL, want: PluginsStoreMirrorURL},
		{name: "custom_store_kept", availability: onlyMirror, current: custom, want: custom},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.availability.choose(tt.current))
		})
	}
}

func TestPluginsStoreKeyFor(t *testing.T) {
	tests := []struct {
		tag  string
		want string
	}{
		{tag: "v4.1.0", want: pluginsStoreLegacyKey},
		{tag: "v4.4.9", want: pluginsStoreLegacyKey},
		{tag: "v4.5.0-rc.1", want: pluginsStoreKey},
		{tag: "v4.5.3", want: pluginsStoreKey},
		{tag: "v5.0.0", want: pluginsStoreKey},
		{tag: "", want: pluginsStoreKey},
		{tag: "development", want: pluginsStoreKey},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			assert.Equal(t, tt.want, pluginsStoreKeyFor(tt.tag))
		})
	}
}

func TestPluginsStoreKeyRenamedIn_MatchesMigrationTable(t *testing.T) {
	for _, migration := range configEnvMigrations {
		for _, rename := range migration.Renames {
			if rename.Old != pluginsStoreLegacyKey {
				continue
			}

			assert.Equal(t, pluginsStoreKey, rename.New)
			assert.Equal(t, pluginsStoreKeyRenamedIn, migration.MinVersion)

			return
		}
	}

	t.Fatalf("configEnvMigrations has no rename of %s", pluginsStoreLegacyKey)
}
