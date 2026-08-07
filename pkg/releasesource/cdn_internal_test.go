package releasesource

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFailingServer(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	return srv
}

func Test_cdnAvailability_bothUp(t *testing.T) {
	first := newProbeServer(t, 0)
	second := newProbeServer(t, 0)

	sel := newSelector([]source{
		{name: CDNGameAPCom, kind: kindCDN, baseURL: first.URL},
		{name: CDNGameAPRu, kind: kindCDN, baseURL: second.URL},
	})

	availability := sel.cdnAvailability(context.Background())
	require.Len(t, availability, 2)
	assert.True(t, availability[CDNGameAPCom])
	assert.True(t, availability[CDNGameAPRu])
}

func Test_cdnAvailability_oneDown(t *testing.T) {
	ok := newProbeServer(t, 0)
	failing := newFailingServer(t)

	sel := newSelector([]source{
		{name: CDNGameAPCom, kind: kindCDN, baseURL: failing.URL},
		{name: CDNGameAPRu, kind: kindCDN, baseURL: ok.URL},
	})

	availability := sel.cdnAvailability(context.Background())
	require.Len(t, availability, 2)
	assert.False(t, availability[CDNGameAPCom])
	assert.True(t, availability[CDNGameAPRu])
}

func Test_cdnAvailability_bothDown(t *testing.T) {
	failing := newFailingServer(t)

	closed := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	closedURL := closed.URL
	closed.Close()

	sel := newSelector([]source{
		{name: CDNGameAPCom, kind: kindCDN, baseURL: failing.URL},
		{name: CDNGameAPRu, kind: kindCDN, baseURL: closedURL},
	})

	availability := sel.cdnAvailability(context.Background())
	require.Len(t, availability, 2)
	assert.False(t, availability[CDNGameAPCom])
	assert.False(t, availability[CDNGameAPRu])
}

func Test_cdnAvailability_skipsGitHubSource(t *testing.T) {
	first := newProbeServer(t, 0)
	second := newProbeServer(t, 0)

	sel := newSelector([]source{
		{name: sourceNameGitHub, kind: kindGitHub, baseURL: "http://github.invalid"},
		{name: CDNGameAPCom, kind: kindCDN, baseURL: first.URL},
		{name: CDNGameAPRu, kind: kindCDN, baseURL: second.URL},
	})

	availability := sel.cdnAvailability(context.Background())
	require.Len(t, availability, 2)
	_, hasGithub := availability[sourceNameGitHub]
	assert.False(t, hasGithub)
}

func Test_cdnAvailability_ignoresEnvOverride(t *testing.T) {
	t.Setenv(EnvSource, sourceNameGitHub)

	ok := newProbeServer(t, 0)

	sel := newSelector([]source{
		{name: sourceNameGitHub, kind: kindGitHub, baseURL: "http://github.invalid"},
		{name: CDNGameAPCom, kind: kindCDN, baseURL: ok.URL},
	})

	availability := sel.cdnAvailability(context.Background())
	require.Len(t, availability, 1)
	assert.True(t, availability[CDNGameAPCom])
}
