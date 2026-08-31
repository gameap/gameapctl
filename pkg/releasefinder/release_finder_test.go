package releasefinder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Trimmed real response from https://api.github.com/repos/gameap/daemon/releases
// Asset order matches the real API response — this matters for the mips/mips64 bug.
const daemonReleasesJSON = `[
  {
    "tag_name": "v3.2.2",
    "assets": []
  },
  {
    "tag_name": "v3.2.1",
    "assets": []
  },
  {
    "tag_name": "v3.2.0",
    "assets": [
      {
        "url": "https://api.github.com/repos/gameap/daemon/releases/assets/372590667",
        "name": "gameap-daemon-v3.2.0-linux-mips64.tar.gz",
        "browser_download_url": "https://github.com/gameap/daemon/releases/download/v3.2.0/gameap-daemon-v3.2.0-linux-mips64.tar.gz"
      },
      {
        "url": "https://api.github.com/repos/gameap/daemon/releases/assets/372590678",
        "name": "gameap-daemon-v3.2.0-linux-arm.tar.gz",
        "browser_download_url": "https://github.com/gameap/daemon/releases/download/v3.2.0/gameap-daemon-v3.2.0-linux-arm.tar.gz"
      },
      {
        "url": "https://api.github.com/repos/gameap/daemon/releases/assets/372590688",
        "name": "gameap-daemon-v3.2.0-linux-amd64.tar.gz",
        "browser_download_url": "https://github.com/gameap/daemon/releases/download/v3.2.0/gameap-daemon-v3.2.0-linux-amd64.tar.gz"
      },
      {
        "url": "https://api.github.com/repos/gameap/daemon/releases/assets/372590692",
        "name": "gameap-daemon-v3.2.0-linux-arm64.tar.gz",
        "browser_download_url": "https://github.com/gameap/daemon/releases/download/v3.2.0/gameap-daemon-v3.2.0-linux-arm64.tar.gz"
      },
      {
        "url": "https://api.github.com/repos/gameap/daemon/releases/assets/372590695",
        "name": "gameap-daemon-v3.2.0-linux-mips.tar.gz",
        "browser_download_url": "https://github.com/gameap/daemon/releases/download/v3.2.0/gameap-daemon-v3.2.0-linux-mips.tar.gz"
      },
      {
        "url": "https://api.github.com/repos/gameap/daemon/releases/assets/372590699",
        "name": "gameap-daemon-v3.2.0-linux-386.tar.gz",
        "browser_download_url": "https://github.com/gameap/daemon/releases/download/v3.2.0/gameap-daemon-v3.2.0-linux-386.tar.gz"
      }
    ]
  }
]`

func Test_findReleaseFromBytes(t *testing.T) {
	tests := []struct {
		name          string
		json          string
		os            string
		arch          string
		wantURL       string
		wantTag       string
		wantAssetName string
		wantErr       bool
		errSubstr     string
	}{
		{
			name:          "amd64",
			json:          daemonReleasesJSON,
			os:            "linux",
			arch:          "amd64",
			wantURL:       "https://github.com/gameap/daemon/releases/download/v3.2.0/gameap-daemon-v3.2.0-linux-amd64.tar.gz",
			wantTag:       "v3.2.0",
			wantAssetName: "gameap-daemon-v3.2.0-linux-amd64.tar.gz",
		},
		{
			name:    "386",
			json:    daemonReleasesJSON,
			os:      "linux",
			arch:    "386",
			wantURL: "https://github.com/gameap/daemon/releases/download/v3.2.0/gameap-daemon-v3.2.0-linux-386.tar.gz",
			wantTag: "v3.2.0",
		},
		{
			name:    "arm",
			json:    daemonReleasesJSON,
			os:      "linux",
			arch:    "arm",
			wantURL: "https://github.com/gameap/daemon/releases/download/v3.2.0/gameap-daemon-v3.2.0-linux-arm.tar.gz",
			wantTag: "v3.2.0",
		},
		{
			name:    "arm64",
			json:    daemonReleasesJSON,
			os:      "linux",
			arch:    "arm64",
			wantURL: "https://github.com/gameap/daemon/releases/download/v3.2.0/gameap-daemon-v3.2.0-linux-arm64.tar.gz",
			wantTag: "v3.2.0",
		},
		{
			name:    "mips64",
			json:    daemonReleasesJSON,
			os:      "linux",
			arch:    "mips64",
			wantURL: "https://github.com/gameap/daemon/releases/download/v3.2.0/gameap-daemon-v3.2.0-linux-mips64.tar.gz",
			wantTag: "v3.2.0",
		},
		{
			name:    "mips_not_confused_with_mips64",
			json:    daemonReleasesJSON,
			os:      "linux",
			arch:    "mips",
			wantURL: "https://github.com/gameap/daemon/releases/download/v3.2.0/gameap-daemon-v3.2.0-linux-mips.tar.gz",
			wantTag: "v3.2.0",
		},
		{
			name:    "not_found",
			json:    daemonReleasesJSON,
			os:      "windows",
			arch:    "amd64",
			wantErr: true,
		},
		{
			name:    "empty_releases",
			json:    `[]`,
			os:      "linux",
			arch:    "amd64",
			wantErr: true,
		},
		{
			name:      "invalid_json",
			json:      `{invalid`,
			os:        "linux",
			arch:      "amd64",
			wantErr:   true,
			errSubstr: "failed to decode",
		},
		{
			name: "picks_first_release_with_match",
			json: `[
				{"tag_name":"v3.3.0","assets":[
					{"name":"gameap-daemon-v3.3.0-linux-amd64.tar.gz","browser_download_url":"https://example.com/v3.3.0-linux-amd64.tar.gz"}
				]},
				{"tag_name":"v3.2.0","assets":[
					{"name":"gameap-daemon-v3.2.0-linux-amd64.tar.gz","browser_download_url":"https://example.com/v3.2.0-linux-amd64.tar.gz"}
				]}
			]`,
			os:      "linux",
			arch:    "amd64",
			wantURL: "https://example.com/v3.3.0-linux-amd64.tar.gz",
			wantTag: "v3.3.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release, err := findReleaseFromList([]byte(tt.json), tt.os, tt.arch, FindOptions{})

			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				if tt.os != "" && tt.arch != "" && tt.errSubstr == "" {
					var notFound FailedToFindReleaseError
					assert.ErrorAs(t, err, &notFound)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, release)
			assert.Equal(t, tt.wantURL, release.URL)
			assert.Equal(t, tt.wantTag, release.Tag)
			if tt.wantAssetName != "" {
				assert.Equal(t, tt.wantAssetName, release.AssetName)
			}
		})
	}
}

func Test_handleErrorResponse(t *testing.T) {
	resetUnix := time.Now().Add(5 * time.Minute).Unix()
	resetHeader := strconv.FormatInt(resetUnix, 10)

	tests := []struct {
		name          string
		statusCode    int
		body          string
		headers       http.Header
		wantRateLimit bool
		wantResetZero bool
		errContains   string
	}{
		{
			name:       "403_with_rate_limit_reset",
			statusCode: http.StatusForbidden,
			body:       `{"message":"API rate limit exceeded"}`,
			headers: http.Header{
				"X-Ratelimit-Reset": {resetHeader},
			},
			wantRateLimit: true,
			errContains:   "API rate limit exceeded",
		},
		{
			name:       "429_with_rate_limit_reset",
			statusCode: http.StatusTooManyRequests,
			body:       `{"message":"Too many requests"}`,
			headers: http.Header{
				"X-Ratelimit-Reset": {resetHeader},
			},
			wantRateLimit: true,
			errContains:   "Too many requests",
		},
		{
			name:          "403_without_reset_header",
			statusCode:    http.StatusForbidden,
			body:          `{"message":"Forbidden"}`,
			headers:       http.Header{},
			wantRateLimit: true,
			wantResetZero: true,
			errContains:   "Forbidden",
		},
		{
			name:        "500_server_error",
			statusCode:  http.StatusInternalServerError,
			body:        `{"message":"Internal Server Error"}`,
			headers:     http.Header{},
			errContains: "Internal Server Error",
		},
		{
			name:        "500_invalid_json_body",
			statusCode:  http.StatusInternalServerError,
			body:        `not json`,
			headers:     http.Header{},
			errContains: "status code 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handleErrorResponse(tt.statusCode, []byte(tt.body), tt.headers)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)

			var rlErr rateLimitError
			if tt.wantRateLimit {
				require.ErrorAs(t, err, &rlErr)
				assert.Equal(t, tt.statusCode, rlErr.statusCode)
				if tt.wantResetZero {
					assert.True(t, rlErr.resetTime.IsZero())
				} else {
					assert.Equal(t, time.Unix(resetUnix, 0), rlErr.resetTime)
				}
			} else {
				assert.False(t, errors.As(err, &rlErr))
			}
		})
	}
}

func Test_shouldRetry(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "rate_limit_error",
			err:  rateLimitError{statusCode: 429, message: "rate limited"},
			want: true,
		},
		{
			name: "failed_to_find_release",
			err:  FailedToFindReleaseError{OS: "linux", Arch: "amd64"},
			want: false,
		},
		{
			name: "plain_error",
			err:  errors.New("some error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldRetry(tt.err))
		})
	}
}

func Test_calculateWaitDuration(t *testing.T) {
	t.Run("non_rate_limit_error_returns_backoff", func(t *testing.T) {
		result := calculateWaitDuration(errors.New("generic"), 4*time.Second)
		assert.Equal(t, 4*time.Second, result)
	})

	t.Run("rate_limit_with_zero_reset_returns_backoff", func(t *testing.T) {
		err := rateLimitError{resetTime: time.Time{}}
		result := calculateWaitDuration(err, 4*time.Second)
		assert.Equal(t, 4*time.Second, result)
	})

	t.Run("rate_limit_with_future_reset", func(t *testing.T) {
		resetTime := time.Now().Add(30 * time.Second)
		err := rateLimitError{resetTime: resetTime}
		result := calculateWaitDuration(err, 4*time.Second)
		assert.InDelta(t, 31.0, result.Seconds(), 2.0)
	})

	t.Run("rate_limit_reset_capped_at_max_wait", func(t *testing.T) {
		resetTime := time.Now().Add(5 * time.Minute)
		err := rateLimitError{resetTime: resetTime}
		result := calculateWaitDuration(err, 4*time.Second)
		assert.Equal(t, 2*time.Minute, result)
	})
}

func Test_waitWithContext(t *testing.T) {
	t.Run("returns_nil_after_duration", func(t *testing.T) {
		err := waitWithContext(context.Background(), 10*time.Millisecond)
		assert.NoError(t, err)
	})

	t.Run("returns_error_on_cancelled_context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := waitWithContext(ctx, time.Second)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func Test_Find(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(daemonReleasesJSON))
		}))
		defer srv.Close()

		release, err := Find(context.Background(), srv.URL, "linux", "amd64")
		require.NoError(t, err)
		require.NotNil(t, release)
		assert.Equal(t, "v3.2.0", release.Tag)
		assert.Contains(t, release.URL, "linux-amd64")
	})

	t.Run("not_found_no_retry", func(t *testing.T) {
		var reqCount atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reqCount.Add(1)
			_, _ = w.Write([]byte(daemonReleasesJSON))
		}))
		defer srv.Close()

		_, err := Find(context.Background(), srv.URL, "windows", "amd64")
		require.Error(t, err)

		var notFound FailedToFindReleaseError
		assert.ErrorAs(t, err, &notFound)
		assert.Equal(t, int32(1), reqCount.Load())
	})

	t.Run("rate_limit_then_success", func(t *testing.T) {
		var reqCount atomic.Int32
		resetUnix := time.Now().Add(100 * time.Millisecond).Unix()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			n := reqCount.Add(1)
			if n == 1 {
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetUnix, 10))
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"message":"rate limited"}`))

				return
			}
			_, _ = w.Write([]byte(daemonReleasesJSON))
		}))
		defer srv.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		release, err := Find(ctx, srv.URL, "linux", "amd64")
		require.NoError(t, err)
		require.NotNil(t, release)
		assert.Equal(t, "v3.2.0", release.Tag)
		assert.True(t, reqCount.Load() >= 2)
	})

	t.Run("server_error_no_retry", func(t *testing.T) {
		var reqCount atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reqCount.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"Internal Server Error"}`))
		}))
		defer srv.Close()

		_, err := Find(context.Background(), srv.URL, "linux", "amd64")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status 500")
		assert.Equal(t, int32(1), reqCount.Load())
	})
}

const releasesWithPrerelease = `[
  {
    "tag_name": "v5.0.0-beta1",
    "prerelease": true,
    "draft": false,
    "assets": [
      {"name": "gameap-v5.0.0-beta1-linux-amd64.tar.gz", "browser_download_url": "https://example.com/v5.0.0-beta1-linux-amd64.tar.gz"}
    ]
  },
  {
    "tag_name": "v4.2.0-draft",
    "prerelease": false,
    "draft": true,
    "assets": [
      {"name": "gameap-v4.2.0-draft-linux-amd64.tar.gz", "browser_download_url": "https://example.com/v4.2.0-draft-linux-amd64.tar.gz"}
    ]
  },
  {
    "tag_name": "v4.1.5",
    "prerelease": false,
    "draft": false,
    "assets": [
      {"name": "gameap-v4.1.5-linux-amd64.tar.gz", "browser_download_url": "https://example.com/v4.1.5-linux-amd64.tar.gz"}
    ]
  },
  {
    "tag_name": "v4.1.0",
    "prerelease": false,
    "draft": false,
    "assets": [
      {"name": "gameap-v4.1.0-linux-amd64.tar.gz", "browser_download_url": "https://example.com/v4.1.0-linux-amd64.tar.gz"}
    ]
  }
]`

func Test_findReleaseFromList_filterPrereleaseAndDraft(t *testing.T) {
	t.Run("default_skips_prerelease_and_draft", func(t *testing.T) {
		release, err := findReleaseFromList([]byte(releasesWithPrerelease), "linux", "amd64", FindOptions{})
		require.NoError(t, err)
		assert.Equal(t, "v4.1.5", release.Tag)
	})

	t.Run("allow_prerelease_returns_first", func(t *testing.T) {
		release, err := findReleaseFromList(
			[]byte(releasesWithPrerelease), "linux", "amd64",
			FindOptions{AllowPrerelease: true},
		)
		require.NoError(t, err)
		assert.Equal(t, "v5.0.0-beta1", release.Tag)
	})
}

func Test_findReleaseFromList_byPrefix(t *testing.T) {
	t.Run("prefix_picks_latest_in_minor_line", func(t *testing.T) {
		release, err := findReleaseFromList(
			[]byte(releasesWithPrerelease), "linux", "amd64",
			FindOptions{TagPrefix: "v4.1."},
		)
		require.NoError(t, err)
		assert.Equal(t, "v4.1.5", release.Tag)
	})

	t.Run("prefix_no_match", func(t *testing.T) {
		_, err := findReleaseFromList(
			[]byte(releasesWithPrerelease), "linux", "amd64",
			FindOptions{TagPrefix: "v9.9."},
		)
		require.Error(t, err)
		var notFound FailedToFindReleaseError
		assert.ErrorAs(t, err, &notFound)
	})

	t.Run("prefix_with_allow_prerelease", func(t *testing.T) {
		release, err := findReleaseFromList(
			[]byte(releasesWithPrerelease), "linux", "amd64",
			FindOptions{TagPrefix: "v5.", AllowPrerelease: true},
		)
		require.NoError(t, err)
		assert.Equal(t, "v5.0.0-beta1", release.Tag)
	})
}

// Mirrors the gameapctl GitHub state: every release marked as prerelease.
const allPrereleasesJSON = `[
  {"tag_name":"v0.26.0","prerelease":true,"draft":false,"assets":[
    {"name":"gameapctl-v0.26.0-linux-amd64.tar.gz","browser_download_url":"https://example.com/v0.26.0-linux-amd64.tar.gz"}
  ]},
  {"tag_name":"v0.25.0","prerelease":true,"draft":false,"assets":[
    {"name":"gameapctl-v0.25.0-linux-amd64.tar.gz","browser_download_url":"https://example.com/v0.25.0-linux-amd64.tar.gz"}
  ]}
]`

func Test_findReleaseFromList_LatestTagOnNoMatch(t *testing.T) {
	t.Run("all_prerelease_without_allow_yields_empty_latest_tag", func(t *testing.T) {
		_, err := findReleaseFromList([]byte(allPrereleasesJSON), "linux", "amd64", FindOptions{})
		require.Error(t, err)

		var notFound FailedToFindReleaseError
		require.ErrorAs(t, err, &notFound)
		assert.Empty(t, notFound.LatestTag)
		assert.Equal(t, "failed to find release for linux (arch: amd64)", err.Error())
	})

	t.Run("all_prerelease_with_allow_returns_first", func(t *testing.T) {
		release, err := findReleaseFromList(
			[]byte(allPrereleasesJSON), "linux", "amd64",
			FindOptions{AllowPrerelease: true},
		)
		require.NoError(t, err)
		assert.Equal(t, "v0.26.0", release.Tag)
	})

	t.Run("no_asset_for_platform_populates_latest_tag", func(t *testing.T) {
		_, err := findReleaseFromList(
			[]byte(allPrereleasesJSON), "darwin", "arm64",
			FindOptions{AllowPrerelease: true},
		)
		require.Error(t, err)

		var notFound FailedToFindReleaseError
		require.ErrorAs(t, err, &notFound)
		assert.Equal(t, "v0.26.0", notFound.LatestTag)
		assert.Equal(t, "no darwin/arm64 asset in release v0.26.0", err.Error())
	})

	t.Run("default_skips_prerelease_then_uses_first_stable_for_latest_tag", func(t *testing.T) {
		_, err := findReleaseFromList([]byte(releasesWithPrerelease), "darwin", "arm64", FindOptions{})
		require.Error(t, err)

		var notFound FailedToFindReleaseError
		require.ErrorAs(t, err, &notFound)
		assert.Equal(t, "v4.1.5", notFound.LatestTag)
	})

	t.Run("prefix_no_match_yields_empty_latest_tag", func(t *testing.T) {
		_, err := findReleaseFromList(
			[]byte(releasesWithPrerelease), "linux", "amd64",
			FindOptions{TagPrefix: "v9.9."},
		)
		require.Error(t, err)

		var notFound FailedToFindReleaseError
		require.ErrorAs(t, err, &notFound)
		assert.Empty(t, notFound.LatestTag)
	})
}

func Test_findReleaseFromSingleResponse_PopulatesLatestTag(t *testing.T) {
	const singleJSON = `{
		"tag_name": "v4.1.2",
		"prerelease": false,
		"draft": false,
		"assets": [
			{"name": "gameap-v4.1.2-linux-amd64.tar.gz", "browser_download_url": "https://example.com/v4.1.2-linux-amd64.tar.gz"}
		]
	}`

	_, err := findReleaseFromSingleResponse([]byte(singleJSON), "darwin", "arm64")
	require.Error(t, err)

	var notFound FailedToFindReleaseError
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, "v4.1.2", notFound.LatestTag)
	assert.Equal(t, "no darwin/arm64 asset in release v4.1.2", err.Error())
}

func Test_FailedToFindReleaseError_Error(t *testing.T) {
	t.Run("without_latest_tag", func(t *testing.T) {
		e := FailedToFindReleaseError{OS: "linux", Arch: "amd64"}
		assert.Equal(t, "failed to find release for linux (arch: amd64)", e.Error())
	})

	t.Run("with_latest_tag", func(t *testing.T) {
		e := FailedToFindReleaseError{OS: "darwin", Arch: "arm64", LatestTag: "v0.26.0"}
		assert.Equal(t, "no darwin/arm64 asset in release v0.26.0", e.Error())
	})
}

func Test_FindWithOptions_byTag(t *testing.T) {
	tagJSON := `{
		"tag_name": "v4.1.2",
		"prerelease": false,
		"draft": false,
		"assets": [
			{"name": "gameap-v4.1.2-linux-amd64.tar.gz", "browser_download_url": "https://example.com/v4.1.2-linux-amd64.tar.gz"}
		]
	}`

	t.Run("tag_found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/repo/releases/tags/v4.1.2", r.URL.Path)
			_, _ = w.Write([]byte(tagJSON))
		}))
		defer srv.Close()

		release, err := FindWithOptions(
			context.Background(), srv.URL+"/repo/releases", "linux", "amd64",
			FindOptions{Tag: "v4.1.2"},
		)
		require.NoError(t, err)
		assert.Equal(t, "v4.1.2", release.Tag)
	})

	t.Run("tag_not_found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		}))
		defer srv.Close()

		_, err := FindWithOptions(
			context.Background(), srv.URL+"/repo/releases", "linux", "amd64",
			FindOptions{Tag: "v9.9.9"},
		)
		require.Error(t, err)
		var notFound TagNotFoundError
		assert.ErrorAs(t, err, &notFound)
		assert.Equal(t, "v9.9.9", notFound.Tag)
	})

	t.Run("tag_found_but_no_matching_asset", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(tagJSON))
		}))
		defer srv.Close()

		_, err := FindWithOptions(
			context.Background(), srv.URL+"/repo/releases", "windows", "amd64",
			FindOptions{Tag: "v4.1.2"},
		)
		require.Error(t, err)
		var notFound FailedToFindReleaseError
		assert.ErrorAs(t, err, &notFound)
	})
}

func Test_FindWithOptions_listAddsPerPageQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))
		_, _ = w.Write([]byte(daemonReleasesJSON))
	}))
	defer srv.Close()

	_, err := FindWithOptions(context.Background(), srv.URL, "linux", "amd64", FindOptions{})
	require.NoError(t, err)
}

func Test_FindFromStaticList(t *testing.T) {
	serve := func(t *testing.T, body string) *httptest.Server {
		t.Helper()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		t.Cleanup(srv.Close)

		return srv
	}

	t.Run("list_mode_fetches_url_verbatim", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/gameap-daemon/releases.json", r.URL.Path)
			assert.Empty(t, r.URL.RawQuery)
			_, _ = w.Write([]byte(daemonReleasesJSON))
		}))
		defer srv.Close()

		release, err := FindFromStaticList(
			context.Background(), srv.URL+"/gameap-daemon/releases.json", "linux", "amd64", FindOptions{},
		)
		require.NoError(t, err)
		assert.Equal(t, "v3.2.0", release.Tag)
		assert.Equal(t, "gameap-daemon-v3.2.0-linux-amd64.tar.gz", release.AssetName)
	})

	t.Run("exact_tag_found", func(t *testing.T) {
		srv := serve(t, releasesWithPrerelease)

		release, err := FindFromStaticList(
			context.Background(), srv.URL, "linux", "amd64", FindOptions{Tag: "v4.1.0"},
		)
		require.NoError(t, err)
		assert.Equal(t, "v4.1.0", release.Tag)
		assert.Equal(t, "https://example.com/v4.1.0-linux-amd64.tar.gz", release.URL)
	})

	t.Run("exact_tag_matches_prerelease_without_allow", func(t *testing.T) {
		srv := serve(t, releasesWithPrerelease)

		release, err := FindFromStaticList(
			context.Background(), srv.URL, "linux", "amd64", FindOptions{Tag: "v5.0.0-beta1"},
		)
		require.NoError(t, err)
		assert.Equal(t, "v5.0.0-beta1", release.Tag)
	})

	t.Run("exact_tag_missing_returns_tag_not_found", func(t *testing.T) {
		srv := serve(t, releasesWithPrerelease)

		_, err := FindFromStaticList(
			context.Background(), srv.URL, "linux", "amd64", FindOptions{Tag: "v9.9.9"},
		)
		require.Error(t, err)

		var notFound TagNotFoundError
		require.ErrorAs(t, err, &notFound)
		assert.Equal(t, "v9.9.9", notFound.Tag)
	})

	t.Run("exact_tag_without_platform_asset", func(t *testing.T) {
		srv := serve(t, releasesWithPrerelease)

		_, err := FindFromStaticList(
			context.Background(), srv.URL, "windows", "amd64", FindOptions{Tag: "v4.1.5"},
		)
		require.Error(t, err)

		var notFound FailedToFindReleaseError
		require.ErrorAs(t, err, &notFound)
		assert.Equal(t, "v4.1.5", notFound.LatestTag)
	})

	t.Run("tag_prefix_picks_latest_in_line", func(t *testing.T) {
		srv := serve(t, releasesWithPrerelease)

		release, err := FindFromStaticList(
			context.Background(), srv.URL, "linux", "amd64", FindOptions{TagPrefix: "v4.1."},
		)
		require.NoError(t, err)
		assert.Equal(t, "v4.1.5", release.Tag)
	})

	t.Run("prerelease_filtered_in_list_mode", func(t *testing.T) {
		srv := serve(t, releasesWithPrerelease)

		release, err := FindFromStaticList(context.Background(), srv.URL, "linux", "amd64", FindOptions{})
		require.NoError(t, err)
		assert.Equal(t, "v4.1.5", release.Tag)
	})

	t.Run("non_200_fails_without_retry", func(t *testing.T) {
		var reqCount atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reqCount.Add(1)
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		_, err := FindFromStaticList(context.Background(), srv.URL, "linux", "amd64", FindOptions{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status code 403")
		assert.False(t, IsRateLimitError(err))
		assert.Equal(t, int32(1), reqCount.Load())
	})
}

func Test_FindWithOptions_NoRetry(t *testing.T) {
	var reqCount atomic.Int32
	resetUnix := time.Now().Add(time.Hour).Unix()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reqCount.Add(1)
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetUnix, 10))
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"rate limited"}`))
	}))
	defer srv.Close()

	_, err := FindWithOptions(context.Background(), srv.URL, "linux", "amd64", FindOptions{NoRetry: true})
	require.Error(t, err)
	assert.True(t, IsRateLimitError(err))
	assert.Equal(t, int32(1), reqCount.Load())
}

func Test_IsRateLimitError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "rate_limit",
			err:  rateLimitError{statusCode: 429, message: "rate limited"},
			want: true,
		},
		{
			name: "wrapped_rate_limit",
			err:  errors.WithMessage(rateLimitError{statusCode: 403, message: "forbidden"}, "failed to find release"),
			want: true,
		},
		{
			name: "failed_to_find_release",
			err:  FailedToFindReleaseError{OS: "linux", Arch: "amd64"},
			want: false,
		},
		{
			name: "plain_error",
			err:  errors.New("some error"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsRateLimitError(tt.err))
		})
	}
}

func Test_NormalizeTag(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantFull   string
		wantPrefix string
		wantErr    bool
	}{
		{name: "empty", input: "", wantFull: "", wantPrefix: ""},
		{name: "major_only", input: "4", wantPrefix: "v4."},
		{name: "major_only_v", input: "v4", wantPrefix: "v4."},
		{name: "major_only_uppercase_v", input: "V4", wantPrefix: "v4."},
		{name: "major_three", input: "3", wantPrefix: "v3."},
		{name: "major_three_v", input: "v3", wantPrefix: "v3."},
		{name: "major_minor", input: "4.1", wantPrefix: "v4.1."},
		{name: "major_minor_v", input: "v4.1", wantPrefix: "v4.1."},
		{name: "major_minor_three", input: "v3.2", wantPrefix: "v3.2."},
		{name: "full_three_segments", input: "4.1.2", wantFull: "v4.1.2"},
		{name: "full_three_v", input: "v4.1.2", wantFull: "v4.1.2"},
		{name: "full_with_beta", input: "4.1.2beta1", wantFull: "v4.1.2beta1"},
		{name: "full_with_dash_rc", input: "v4.1.0-rc1", wantFull: "v4.1.0-rc1"},
		{name: "full_three_v3", input: "v3.2.1", wantFull: "v3.2.1"},
		{name: "garbage", input: "foo", wantErr: true},
		{name: "too_many_segments", input: "4.1.2.3", wantErr: true},
		{name: "trailing_dot", input: "4.1.", wantErr: true},
		{name: "double_dot", input: "4..1", wantErr: true},
		{name: "leading_dot", input: ".1.2", wantErr: true},
		{name: "major_with_suffix", input: "4-rc1", wantErr: true},
		{name: "major_minor_with_suffix", input: "4.1beta", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeTag(tt.input)
			if tt.wantErr {
				require.Error(t, err)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantFull, got.Full)
			assert.Equal(t, tt.wantPrefix, got.Prefix)
		})
	}
}

func Test_NormalizedTag_HasPrereleaseSuffix(t *testing.T) {
	tests := []struct {
		name string
		tag  NormalizedTag
		want bool
	}{
		{name: "empty", tag: NormalizedTag{}, want: false},
		{name: "stable_full", tag: NormalizedTag{Full: "v4.1.2"}, want: false},
		{name: "beta_full", tag: NormalizedTag{Full: "v4.1.2beta1"}, want: true},
		{name: "rc_full", tag: NormalizedTag{Full: "v4.1.0-rc1"}, want: true},
		{name: "prefix_only", tag: NormalizedTag{Prefix: "v4.1."}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.tag.HasPrereleaseSuffix())
		})
	}
}

func Test_IsMajorV3(t *testing.T) {
	tests := []struct {
		name string
		tag  NormalizedTag
		want bool
	}{
		{name: "empty", tag: NormalizedTag{}, want: false},
		{name: "v4_full", tag: NormalizedTag{Full: "v4.1.2"}, want: false},
		{name: "v3_full", tag: NormalizedTag{Full: "v3.2.1"}, want: true},
		{name: "v3_full_with_beta", tag: NormalizedTag{Full: "v3.2.1beta1"}, want: true},
		{name: "v30_full", tag: NormalizedTag{Full: "v30.0.0"}, want: false},
		{name: "v4_prefix", tag: NormalizedTag{Prefix: "v4."}, want: false},
		{name: "v3_prefix", tag: NormalizedTag{Prefix: "v3."}, want: true},
		{name: "v3_minor_prefix", tag: NormalizedTag{Prefix: "v3.2."}, want: true},
		{name: "v30_prefix", tag: NormalizedTag{Prefix: "v30."}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsMajorV3(tt.tag))
		})
	}
}

func Test_IsAtLeastV4_2(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want bool
	}{
		{name: "empty", tag: "", want: false},
		{name: "spaces_only", tag: "   ", want: false},
		{name: "v4_1_9", tag: "v4.1.9", want: false},
		{name: "v4_1_99", tag: "v4.1.99", want: false},
		{name: "v4_2_0", tag: "v4.2.0", want: true},
		{name: "v4_2_0_no_v_prefix", tag: "4.2.0", want: true},
		{name: "v4_2_0_beta1", tag: "v4.2.0beta1", want: true},
		{name: "v4_2_0_dash_rc1", tag: "v4.2.0-rc1", want: true},
		{name: "v4_10_0", tag: "v4.10.0", want: true},
		{name: "v5_0_0", tag: "v5.0.0", want: true},
		{name: "v3_9_9", tag: "v3.9.9", want: false},
		{name: "v4_only", tag: "v4", want: false},
		{name: "garbage", tag: "garbage", want: false},
		{name: "uppercase_V", tag: "V4.2.0", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsAtLeastV4_2(tt.tag))
		})
	}
}

func Test_CompareMajorMinor(t *testing.T) {
	tests := []struct {
		name    string
		a       string
		b       string
		wantOK  bool
		wantCmp int
	}{
		{name: "prerelease_counts_as_its_minor_line", a: "v4.5.0-beta.1", b: "v4.5", wantOK: true, wantCmp: 0},
		{name: "old_style_prerelease_counts_too", a: "v4.2.0beta1", b: "v4.2", wantOK: true, wantCmp: 0},
		{name: "older_minor", a: "v4.4.2", b: "v4.5", wantOK: true, wantCmp: -1},
		{name: "minors_compare_numerically_not_lexically", a: "v4.10.0", b: "v4.9", wantOK: true, wantCmp: 1},
		{name: "newer_major", a: "v5.0.0", b: "v4.3", wantOK: true, wantCmp: 1},
		{name: "leading_v_is_optional", a: "4.5.0", b: "v4.5", wantOK: true, wantCmp: 0},
		{name: "empty_is_unparseable", a: "", b: "v4.3", wantOK: false},
		{name: "garbage_is_unparseable", a: "garbage", b: "v4.3", wantOK: false},
		{name: "major_only_is_unparseable", a: "v4", b: "v4.3", wantOK: false},
		{name: "unparseable_minimum", a: "v4.5.0", b: "not-a-version", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmp, ok := CompareMajorMinor(tt.a, tt.b)

			assert.Equal(t, tt.wantOK, ok)

			if !tt.wantOK {
				return
			}

			switch {
			case tt.wantCmp == 0:
				assert.Equal(t, 0, cmp)
			case tt.wantCmp < 0:
				assert.Negative(t, cmp)
			default:
				assert.Positive(t, cmp)
			}
		})
	}
}

func Test_IsAtLeast(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		minimum string
		want    bool
	}{
		{name: "prerelease_of_the_same_minor", tag: "v4.5.0-beta.1", minimum: "v4.5", want: true},
		{name: "previous_minor", tag: "v4.4.2", minimum: "v4.5", want: false},
		{name: "later_minor_compares_numerically", tag: "v4.10.0", minimum: "v4.9", want: true},
		{name: "later_major", tag: "v5.0.0", minimum: "v4.3", want: true},
		{name: "unknown_tag_is_older_than_anything", tag: "", minimum: "v4.3", want: false},
		{name: "garbage_tag_is_older_than_anything", tag: "garbage", minimum: "v4.3", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsAtLeast(tt.tag, tt.minimum))
		})
	}
}
