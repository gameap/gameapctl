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
		name      string
		json      string
		os        string
		arch      string
		wantURL   string
		wantTag   string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "amd64",
			json:    daemonReleasesJSON,
			os:      "linux",
			arch:    "amd64",
			wantURL: "https://github.com/gameap/daemon/releases/download/v3.2.0/gameap-daemon-v3.2.0-linux-amd64.tar.gz",
			wantTag: "v3.2.0",
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
			// BUG: strings.Contains("mips64", "mips") == true, and mips64 asset
			// comes before mips in the API response, so mips returns the mips64 URL.
			name:    "mips_matches_mips64_bug",
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
			release, err := findReleaseFromBytes([]byte(tt.json), tt.os, tt.arch)

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
		result := calculateWaitDuration(errors.New("generic"), 4*time.Second, 2*time.Minute)
		assert.Equal(t, 4*time.Second, result)
	})

	t.Run("rate_limit_with_zero_reset_returns_backoff", func(t *testing.T) {
		err := rateLimitError{resetTime: time.Time{}}
		result := calculateWaitDuration(err, 4*time.Second, 2*time.Minute)
		assert.Equal(t, 4*time.Second, result)
	})

	t.Run("rate_limit_with_future_reset", func(t *testing.T) {
		resetTime := time.Now().Add(30 * time.Second)
		err := rateLimitError{resetTime: resetTime}
		result := calculateWaitDuration(err, 4*time.Second, 2*time.Minute)
		assert.InDelta(t, 31.0, result.Seconds(), 2.0)
	})

	t.Run("rate_limit_reset_capped_at_max_wait", func(t *testing.T) {
		resetTime := time.Now().Add(5 * time.Minute)
		err := rateLimitError{resetTime: resetTime}
		result := calculateWaitDuration(err, 4*time.Second, 2*time.Minute)
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
