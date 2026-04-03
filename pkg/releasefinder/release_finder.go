package releasefinder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type Release struct {
	URL string
	Tag string
}

func Find(ctx context.Context, api, kernel, platform string) (*Release, error) {
	const (
		maxRetries     = 5
		initialBackoff = 2 * time.Second
	)

	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			waitDuration := calculateWaitDuration(lastErr, backoff)
			if waitDuration > 0 {
				if err := waitWithContext(ctx, waitDuration); err != nil {
					return nil, err
				}
			}
			backoff *= 2
		}

		release, err := fetchRelease(ctx, api, kernel, platform)
		if err == nil {
			return release, nil
		}

		lastErr = err

		if !shouldRetry(err) {
			return nil, err
		}
	}

	return nil, errors.WithMessage(lastErr, "failed after retries")
}

func calculateWaitDuration(lastErr error, backoff time.Duration) time.Duration {
	const maxWait = 2 * time.Minute

	var rateLimitErr rateLimitError
	if !errors.As(lastErr, &rateLimitErr) || rateLimitErr.resetTime.IsZero() {
		log.Printf("Rate limited by GitHub API, retrying after %v", backoff)

		return backoff
	}

	waitDuration := time.Until(rateLimitErr.resetTime) + time.Second
	if waitDuration > maxWait {
		waitDuration = maxWait
	}
	if waitDuration > 0 {
		log.Printf(
			"Rate limited by GitHub API. Waiting until %s (%v)...",
			rateLimitErr.resetTime.Format("15:04:05"),
			waitDuration.Round(time.Second),
		)
	}

	return waitDuration
}

func waitWithContext(ctx context.Context, duration time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration):
		return nil
	}
}

func fetchRelease(_ context.Context, api, kernel, platform string) (*Release, error) {
	resp, err := http.Get(api) //nolint:noctx,gosec
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get releases")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read response body")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, handleErrorResponse(resp.StatusCode, bodyBytes, resp.Header)
	}

	return findReleaseFromBytes(bodyBytes, kernel, platform)
}

type rateLimitError struct {
	statusCode int
	message    string
	resetTime  time.Time
}

func (e rateLimitError) Error() string {
	return e.message
}

func handleErrorResponse(statusCode int, bodyBytes []byte, headers http.Header) error {
	var errorResponse struct {
		Message string `json:"message"`
	}

	var errMsg string
	if json.Unmarshal(bodyBytes, &errorResponse) == nil && errorResponse.Message != "" {
		errMsg = fmt.Sprintf("GitHub API error (status %d): %s", statusCode, errorResponse.Message)
	} else {
		errMsg = fmt.Sprintf("GitHub API returned status code %d", statusCode)
	}

	if statusCode == http.StatusForbidden || statusCode == http.StatusTooManyRequests {
		var resetTime time.Time
		if resetHeader := headers.Get("X-RateLimit-Reset"); resetHeader != "" {
			if resetUnix, err := strconv.ParseInt(resetHeader, 10, 64); err == nil {
				resetTime = time.Unix(resetUnix, 0)
			}
		}

		return rateLimitError{statusCode: statusCode, message: errMsg, resetTime: resetTime}
	}

	return errors.New(errMsg)
}

func shouldRetry(err error) bool {
	var rateLimitErr rateLimitError

	return errors.As(err, &rateLimitErr)
}

type FailedToFindReleaseError struct {
	OS   string
	Arch string
}

func (e FailedToFindReleaseError) Error() string {
	return fmt.Sprintf("failed to find release for %s (arch: %s)", e.OS, e.Arch)
}

type releases struct {
	TagName string  `json:"tag_name"` //nolint:tagliatelle
	Assets  []asset `json:"assets"`
}

type asset struct {
	URL                string `json:"url"`
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"` //nolint:tagliatelle
}

func findReleaseFromBytes(bodyBytes []byte, os string, arch string) (*Release, error) {
	var r []releases
	err := json.Unmarshal(bodyBytes, &r)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to decode GitHub API response")
	}

	for _, release := range r {
		for _, asset := range release.Assets {
			if strings.Contains(asset.Name, release.TagName+"-"+os+"-"+arch+".") {
				return &Release{
					URL: asset.BrowserDownloadURL,
					Tag: release.TagName,
				}, nil
			}
		}
	}

	return nil, FailedToFindReleaseError{os, arch}
}
