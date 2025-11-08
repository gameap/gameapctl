package releasefinder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
		maxRetries     = 3
		initialBackoff = 2 * time.Second
	)

	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}

		release, err := fetchRelease(ctx, api, kernel, platform)
		if err == nil {
			return release, nil
		}

		lastErr = err

		if shouldRetry(err) {
			log.Printf("Rate limited by GitHub API (attempt %d/%d), retrying after %v", attempt+1, maxRetries+1, backoff)

			continue
		}

		return nil, err
	}

	return nil, errors.WithMessage(lastErr, "failed after retries")
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
		return nil, handleErrorResponse(resp.StatusCode, bodyBytes)
	}

	return findReleaseFromBytes(bodyBytes, kernel, platform)
}

type rateLimitError struct {
	statusCode int
	message    string
}

func (e rateLimitError) Error() string {
	return e.message
}

func handleErrorResponse(statusCode int, bodyBytes []byte) error {
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
		return rateLimitError{statusCode: statusCode, message: errMsg}
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
			if strings.Contains(asset.Name, release.TagName+"-"+os+"-"+arch) {
				return &Release{
					URL: asset.BrowserDownloadURL,
					Tag: release.TagName,
				}, nil
			}
		}
	}

	return nil, FailedToFindReleaseError{os, arch}
}
