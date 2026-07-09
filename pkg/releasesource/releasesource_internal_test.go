package releasesource

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testReleasesJSON = `[
  {
    "tag_name": "v3.2.0",
    "prerelease": false,
    "draft": false,
    "assets": [
      {
        "name": "gameap-daemon-v3.2.0-linux-amd64.tar.gz",
        "browser_download_url": "https://github.com/gameap/daemon/releases/download/v3.2.0/gameap-daemon-v3.2.0-linux-amd64.tar.gz"
      }
    ]
  }
]`

const testGithubDownloadURL = "https://github.com/gameap/daemon/releases/download/v3.2.0/gameap-daemon-v3.2.0-linux-amd64.tar.gz"

func newProbeServer(t *testing.T, delay time.Duration) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	return srv
}

func newReleasesServer(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testReleasesJSON))
	}))
	t.Cleanup(srv.Close)

	return srv
}

// selectorWithOrder returns a selector with a predefined source order,
// bypassing the network probe.
func selectorWithOrder(srcs ...source) *selector {
	sel := newSelector(srcs)
	sel.ordered = srcs
	sel.once.Do(func() {})

	return sel
}

func Test_orderedSources_latencyOrder(t *testing.T) {
	fast := newProbeServer(t, 0)
	medium := newProbeServer(t, 100*time.Millisecond)
	slow := newProbeServer(t, 300*time.Millisecond)

	sel := newSelector([]source{
		{name: "slow", kind: kindCDN, baseURL: slow.URL},
		{name: "fast", kind: kindCDN, baseURL: fast.URL},
		{name: "medium", kind: kindCDN, baseURL: medium.URL},
	})

	ordered := sel.orderedSources(context.Background())
	require.Len(t, ordered, 3)
	assert.Equal(t, "fast", ordered[0].name)
	assert.Equal(t, "medium", ordered[1].name)
	assert.Equal(t, "slow", ordered[2].name)
}

func Test_orderedSources_unavailableLast(t *testing.T) {
	ok := newProbeServer(t, 0)

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)

	closed := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	closedURL := closed.URL
	closed.Close()

	sel := newSelector([]source{
		{name: "failing", kind: kindCDN, baseURL: failing.URL},
		{name: "closed", kind: kindCDN, baseURL: closedURL},
		{name: "ok", kind: kindCDN, baseURL: ok.URL},
	})

	ordered := sel.orderedSources(context.Background())
	require.Len(t, ordered, 3)
	assert.Equal(t, "ok", ordered[0].name)
	assert.Equal(t, "failing", ordered[1].name)
	assert.Equal(t, "closed", ordered[2].name)
}

func Test_orderedSources_allProbesFailedKeepsDefaultOrder(t *testing.T) {
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)

	closed := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	closedURL := closed.URL
	closed.Close()

	sel := newSelector([]source{
		{name: "first", kind: kindGitHub, baseURL: closedURL},
		{name: "second", kind: kindCDN, baseURL: failing.URL},
	})

	ordered := sel.orderedSources(context.Background())
	require.Len(t, ordered, 2)
	assert.Equal(t, "first", ordered[0].name)
	assert.Equal(t, "second", ordered[1].name)
}

func Test_orderedSources_memoized(t *testing.T) {
	var probeCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		probeCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	sel := newSelector([]source{{name: "only", kind: kindCDN, baseURL: srv.URL}})

	sel.orderedSources(context.Background())
	sel.orderedSources(context.Background())

	assert.Equal(t, int32(1), probeCount.Load())
}

func Test_findRelease_viaCDN_buildsCandidateURLs(t *testing.T) {
	cdn := newReleasesServer(t)

	sel := selectorWithOrder(
		source{name: "cdn.gameap.com", kind: kindCDN, baseURL: cdn.URL},
		source{name: sourceNameGitHub, kind: kindGitHub, baseURL: "http://github.invalid"},
		source{name: "cdn.gameap.ru", kind: kindCDN, baseURL: "http://cdn2.invalid"},
	)

	release, err := sel.findRelease(
		context.Background(), ComponentDaemon, "linux", "amd64", releasefinder.FindOptions{},
	)
	require.NoError(t, err)
	assert.Equal(t, "v3.2.0", release.Tag)
	assert.Equal(t, "gameap-daemon-v3.2.0-linux-amd64.tar.gz", release.AssetName)
	require.Len(t, release.URLs, 3)
	assert.Equal(t, cdn.URL+"/gameap-daemon/v3.2.0/gameap-daemon-v3.2.0-linux-amd64.tar.gz", release.URLs[0])
	assert.Equal(t, testGithubDownloadURL, release.URLs[1])
	assert.Equal(t, "http://cdn2.invalid/gameap-daemon/v3.2.0/gameap-daemon-v3.2.0-linux-amd64.tar.gz", release.URLs[2])
	assert.Equal(t, release.URLs[0], release.PrimaryURL())
}

func Test_findRelease_viaGitHub_buildsCandidateURLs(t *testing.T) {
	github := newReleasesServer(t)

	sel := selectorWithOrder(
		source{name: sourceNameGitHub, kind: kindGitHub, baseURL: github.URL},
		source{name: "cdn.gameap.com", kind: kindCDN, baseURL: "http://cdn1.invalid"},
		source{name: "cdn.gameap.ru", kind: kindCDN, baseURL: "http://cdn2.invalid"},
	)

	release, err := sel.findRelease(
		context.Background(), ComponentDaemon, "linux", "amd64", releasefinder.FindOptions{},
	)
	require.NoError(t, err)
	require.Len(t, release.URLs, 3)
	assert.Equal(t, testGithubDownloadURL, release.URLs[0])
	assert.Equal(t, "http://cdn1.invalid/gameap-daemon/v3.2.0/gameap-daemon-v3.2.0-linux-amd64.tar.gz", release.URLs[1])
	assert.Equal(t, "http://cdn2.invalid/gameap-daemon/v3.2.0/gameap-daemon-v3.2.0-linux-amd64.tar.gz", release.URLs[2])
}

func Test_findRelease_fallbackOnFailure(t *testing.T) {
	var failingCount atomic.Int32
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		failingCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)

	working := newReleasesServer(t)

	sel := selectorWithOrder(
		source{name: "cdn.gameap.com", kind: kindCDN, baseURL: failing.URL},
		source{name: "cdn.gameap.ru", kind: kindCDN, baseURL: working.URL},
	)

	release, err := sel.findRelease(
		context.Background(), ComponentDaemon, "linux", "amd64", releasefinder.FindOptions{},
	)
	require.NoError(t, err)
	assert.Equal(t, int32(1), failingCount.Load())
	assert.Equal(t, working.URL+"/gameap-daemon/v3.2.0/gameap-daemon-v3.2.0-linux-amd64.tar.gz", release.URLs[0])
}

func Test_findRelease_cdnTagMissing_fallsBackToGitHub(t *testing.T) {
	cdn := newReleasesServer(t)

	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/gameap/daemon/releases/tags/v9.9.9", r.URL.Path)
		_, _ = w.Write([]byte(`{
			"tag_name": "v9.9.9",
			"assets": [
				{"name": "gameap-daemon-v9.9.9-linux-amd64.tar.gz", "browser_download_url": "https://example.com/v9.9.9-linux-amd64.tar.gz"}
			]
		}`))
	}))
	t.Cleanup(github.Close)

	sel := selectorWithOrder(
		source{name: "cdn.gameap.com", kind: kindCDN, baseURL: cdn.URL},
		source{name: sourceNameGitHub, kind: kindGitHub, baseURL: github.URL},
	)

	release, err := sel.findRelease(
		context.Background(), ComponentDaemon, "linux", "amd64", releasefinder.FindOptions{Tag: "v9.9.9"},
	)
	require.NoError(t, err)
	assert.Equal(t, "v9.9.9", release.Tag)
	require.Len(t, release.URLs, 2)
	assert.Equal(t, "https://example.com/v9.9.9-linux-amd64.tar.gz", release.URLs[0])
	assert.Equal(t, cdn.URL+"/gameap-daemon/v9.9.9/gameap-daemon-v9.9.9-linux-amd64.tar.gz", release.URLs[1])
}

func Test_findRelease_prefersContentErrorOverTransportError(t *testing.T) {
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)

	cdn := newReleasesServer(t)

	sel := selectorWithOrder(
		source{name: "cdn.gameap.com", kind: kindCDN, baseURL: failing.URL},
		source{name: "cdn.gameap.ru", kind: kindCDN, baseURL: cdn.URL},
	)

	_, err := sel.findRelease(
		context.Background(), ComponentDaemon, "linux", "amd64", releasefinder.FindOptions{Tag: "v9.9.9"},
	)
	require.Error(t, err)

	var notFound releasefinder.TagNotFoundError
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, "v9.9.9", notFound.Tag)
}

func Test_findRelease_githubRateLimited_skipsToCDNQuickly(t *testing.T) {
	var githubCount atomic.Int32
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		githubCount.Add(1)
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"rate limited"}`))
	}))
	t.Cleanup(github.Close)

	cdn := newReleasesServer(t)

	sel := selectorWithOrder(
		source{name: sourceNameGitHub, kind: kindGitHub, baseURL: github.URL},
		source{name: "cdn.gameap.com", kind: kindCDN, baseURL: cdn.URL},
	)

	release, err := sel.findRelease(
		context.Background(), ComponentDaemon, "linux", "amd64", releasefinder.FindOptions{},
	)
	require.NoError(t, err)
	assert.Equal(t, "v3.2.0", release.Tag)
	assert.Equal(t, int32(1), githubCount.Load())
	assert.Equal(t, cdn.URL+"/gameap-daemon/v3.2.0/gameap-daemon-v3.2.0-linux-amd64.tar.gz", release.URLs[0])
}

func Test_findRelease_allFailedAndRateLimited_retriesGitHubWithWait(t *testing.T) {
	var githubCount atomic.Int32
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if githubCount.Add(1) == 1 {
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(100*time.Millisecond).Unix(), 10))
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"rate limited"}`))

			return
		}
		_, _ = w.Write([]byte(testReleasesJSON))
	}))
	t.Cleanup(github.Close)

	failingCDN := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failingCDN.Close)

	sel := selectorWithOrder(
		source{name: sourceNameGitHub, kind: kindGitHub, baseURL: github.URL},
		source{name: "cdn.gameap.com", kind: kindCDN, baseURL: failingCDN.URL},
	)

	release, err := sel.findRelease(
		context.Background(), ComponentDaemon, "linux", "amd64", releasefinder.FindOptions{},
	)
	require.NoError(t, err)
	assert.Equal(t, "v3.2.0", release.Tag)
	assert.GreaterOrEqual(t, githubCount.Load(), int32(2))
	assert.Equal(t, testGithubDownloadURL, release.URLs[0])
}

func Test_findRelease_envOverride(t *testing.T) {
	t.Run("forced_source_is_the_only_candidate", func(t *testing.T) {
		cdn := newReleasesServer(t)

		sel := newSelector([]source{
			{name: sourceNameGitHub, kind: kindGitHub, baseURL: "http://github.invalid"},
			{name: "cdn.gameap.com", kind: kindCDN, baseURL: "http://cdn1.invalid"},
			{name: "cdn.gameap.ru", kind: kindCDN, baseURL: cdn.URL},
		})

		t.Setenv(EnvSource, "cdn.gameap.ru")

		release, err := sel.findRelease(
			context.Background(), ComponentDaemon, "linux", "amd64", releasefinder.FindOptions{},
		)
		require.NoError(t, err)
		require.Len(t, release.URLs, 1)
		assert.Equal(t, cdn.URL+"/gameap-daemon/v3.2.0/gameap-daemon-v3.2.0-linux-amd64.tar.gz", release.URLs[0])
	})

	t.Run("unknown_source_fails", func(t *testing.T) {
		sel := newSelector(defaultSources())

		t.Setenv(EnvSource, "bogus")

		_, err := sel.findRelease(
			context.Background(), ComponentDaemon, "linux", "amd64", releasefinder.FindOptions{},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), EnvSource)
		assert.Contains(t, err.Error(), sourceNameGitHub)
	})
}

func Test_findRelease_unknownComponent(t *testing.T) {
	sel := newSelector(defaultSources())

	_, err := sel.findRelease(
		context.Background(), Component("unknown"), "linux", "amd64", releasefinder.FindOptions{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown component")
}

func Test_downloadWith_fallback(t *testing.T) {
	t.Run("first_fails_second_succeeds", func(t *testing.T) {
		var attempts []string
		err := downloadWith(
			context.Background(),
			&Release{URLs: []string{"http://first.invalid/a", "http://second.invalid/a"}},
			"/tmp/dst",
			func(_ context.Context, source string, _ string) error {
				attempts = append(attempts, source)
				if source == "http://first.invalid/a" {
					return errors.New("not found")
				}

				return nil
			},
		)
		require.NoError(t, err)
		require.Len(t, attempts, 2)
	})

	t.Run("all_fail", func(t *testing.T) {
		err := downloadWith(
			context.Background(),
			&Release{URLs: []string{"http://first.invalid/a", "http://second.invalid/a"}},
			"/tmp/dst",
			func(_ context.Context, _ string, _ string) error {
				return errors.New("boom")
			},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to download from all sources")
	})

	t.Run("no_urls", func(t *testing.T) {
		err := downloadWith(
			context.Background(), &Release{}, "/tmp/dst",
			func(_ context.Context, _ string, _ string) error { return nil },
		)
		require.Error(t, err)
	})

	t.Run("nil_release", func(t *testing.T) {
		err := downloadWith(
			context.Background(), nil, "/tmp/dst",
			func(_ context.Context, _ string, _ string) error { return nil },
		)
		require.Error(t, err)
	})
}

func Test_DownloadFile_fallbackOverHTTP(t *testing.T) {
	var notFoundCount atomic.Int32
	notFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		notFoundCount.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(notFound.Close)

	fileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("binary-content"))
	}))
	t.Cleanup(fileSrv.Close)

	dst := filepath.Join(t.TempDir(), "out.bin")
	release := &Release{URLs: []string{notFound.URL + "/file.bin", fileSrv.URL + "/file.bin"}}

	err := DownloadFile(context.Background(), release, dst)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, notFoundCount.Load(), int32(1))

	content, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "binary-content", string(content))
}
