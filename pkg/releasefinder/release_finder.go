package releasefinder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type Release struct {
	URL string
	Tag string
}

// FindOptions controls how Find selects a release.
type FindOptions struct {
	// Tag pins the lookup to a specific tag. If empty, list mode is used.
	Tag string
	// TagPrefix filters list-mode results to releases whose tag_name starts with this prefix.
	TagPrefix string
	// AllowPrerelease, when true, includes prerelease/draft releases in list-mode selection.
	AllowPrerelease bool
}

// Find returns the latest stable release that has an asset matching the given kernel/platform.
// Prerelease and draft releases are skipped. Use FindWithOptions for finer control.
func Find(ctx context.Context, api, kernel, platform string) (*Release, error) {
	return FindWithOptions(ctx, api, kernel, platform, FindOptions{})
}

// FindWithOptions resolves a Release using the given options. When opts.Tag is set,
// the GitHub `releases/tags/<tag>` endpoint is queried directly and a missing tag
// surfaces as TagNotFoundError. Otherwise the list endpoint is queried and filtered
// by opts.TagPrefix and opts.AllowPrerelease.
func FindWithOptions(ctx context.Context, api, kernel, platform string, opts FindOptions) (*Release, error) {
	const (
		maxRetries     = 5
		initialBackoff = 2 * time.Second
	)

	requestURL, mode := buildRequestURL(api, opts)

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

		release, err := fetchRelease(ctx, requestURL, mode, kernel, platform, opts)
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

type fetchMode int

const (
	listMode fetchMode = iota
	singularMode
)

func buildRequestURL(api string, opts FindOptions) (string, fetchMode) {
	if opts.Tag != "" {
		return strings.TrimSuffix(api, "/") + "/tags/" + url.PathEscape(opts.Tag), singularMode
	}

	requestURL := api
	separator := "?"
	if strings.Contains(requestURL, "?") {
		separator = "&"
	}

	return requestURL + separator + "per_page=100", listMode
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

func fetchRelease(
	ctx context.Context,
	requestURL string,
	mode fetchMode,
	kernel, platform string,
	opts FindOptions,
) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create request")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get releases")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read response body")
	}

	if resp.StatusCode == http.StatusNotFound && mode == singularMode {
		return nil, TagNotFoundError{Tag: opts.Tag}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, handleErrorResponse(resp.StatusCode, bodyBytes, resp.Header)
	}

	if mode == singularMode {
		return findReleaseFromSingleResponse(bodyBytes, kernel, platform)
	}

	return findReleaseFromList(bodyBytes, kernel, platform, opts)
}

type rateLimitError struct {
	statusCode int
	message    string
	resetTime  time.Time
}

func (e rateLimitError) Error() string {
	return e.message
}

// TagNotFoundError is returned when a release with the requested tag does not exist.
type TagNotFoundError struct {
	Tag string
}

func (e TagNotFoundError) Error() string {
	return fmt.Sprintf("release with tag %q not found", e.Tag)
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
	TagName    string  `json:"tag_name"` //nolint:tagliatelle
	Prerelease bool    `json:"prerelease"`
	Draft      bool    `json:"draft"`
	Assets     []asset `json:"assets"`
}

type asset struct {
	URL                string `json:"url"`
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"` //nolint:tagliatelle
}

func findReleaseFromList(bodyBytes []byte, os, arch string, opts FindOptions) (*Release, error) {
	var r []releases
	if err := json.Unmarshal(bodyBytes, &r); err != nil {
		return nil, errors.WithMessage(err, "failed to decode GitHub API response")
	}

	for _, release := range r {
		if !opts.AllowPrerelease && (release.Prerelease || release.Draft) {
			continue
		}
		if opts.TagPrefix != "" && !strings.HasPrefix(release.TagName, opts.TagPrefix) {
			continue
		}
		if matched := matchAsset(release, os, arch); matched != nil {
			return matched, nil
		}
	}

	return nil, FailedToFindReleaseError{os, arch}
}

func findReleaseFromSingleResponse(bodyBytes []byte, os, arch string) (*Release, error) {
	var release releases
	if err := json.Unmarshal(bodyBytes, &release); err != nil {
		return nil, errors.WithMessage(err, "failed to decode GitHub API response")
	}

	if matched := matchAsset(release, os, arch); matched != nil {
		return matched, nil
	}

	return nil, FailedToFindReleaseError{os, arch}
}

func matchAsset(release releases, os, arch string) *Release {
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, release.TagName+"-"+os+"-"+arch+".") {
			return &Release{
				URL: asset.BrowserDownloadURL,
				Tag: release.TagName,
			}
		}
	}

	return nil
}

// NormalizedTag describes a parsed version string.
//
// Full is set when the input contains all three numeric segments (e.g. "v4.1.2",
// "v4.1.2beta1"). Prefix is set when the input was partial (e.g. "v4." for "4",
// "v4.1." for "4.1") and the caller should pick the latest matching release.
// Both are empty when the input was empty (latest stable).
type NormalizedTag struct {
	Full   string
	Prefix string
}

// HasPrereleaseSuffix reports whether the resolved Full tag carries a non-numeric
// suffix (e.g. "v4.1.2beta1", "v4.1.0-rc1"). Callers use this to decide whether
// the FindWithOptions call should set AllowPrerelease=true.
func (t NormalizedTag) HasPrereleaseSuffix() bool {
	if t.Full == "" {
		return false
	}

	s := strings.TrimPrefix(t.Full, "v")
	for _, c := range s {
		if c != '.' && (c < '0' || c > '9') {
			return true
		}
	}

	return false
}

// IsMajorV3 reports whether the normalized version targets the GameAP v3 major line.
func IsMajorV3(t NormalizedTag) bool {
	if t.Full != "" {
		return t.Full == "v3" || strings.HasPrefix(t.Full, "v3.")
	}
	if t.Prefix != "" {
		return strings.HasPrefix(t.Prefix, "v3.")
	}

	return false
}

var versionSuffixSplitRegex = regexp.MustCompile(`^([0-9.]*)(.*)$`)

// NormalizeTag converts a user-supplied version string into a NormalizedTag.
//
// Empty input yields an empty NormalizedTag (caller treats this as "latest stable").
// Inputs like "4" or "v4" yield Prefix="v4." (latest stable for that major).
// "4.1" yields Prefix="v4.1." (latest stable patch in that minor line).
// "4.1.2" or "v4.1.2beta1" yield Full pinned to that exact tag.
func NormalizeTag(input string) (NormalizedTag, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return NormalizedTag{}, nil
	}

	stripped := input
	if len(stripped) > 0 && (stripped[0] == 'v' || stripped[0] == 'V') {
		stripped = stripped[1:]
	}

	matches := versionSuffixSplitRegex.FindStringSubmatch(stripped)
	if matches == nil {
		return NormalizedTag{}, errors.Errorf("invalid version %q", input)
	}

	versionPart := matches[1]
	suffix := matches[2]

	if versionPart == "" {
		return NormalizedTag{}, errors.Errorf("invalid version %q", input)
	}

	parts := strings.Split(versionPart, ".")
	for _, p := range parts {
		if p == "" {
			return NormalizedTag{}, errors.Errorf("invalid version %q (empty segment)", input)
		}
	}

	const (
		segmentsMajor      = 1
		segmentsMajorMinor = 2
		segmentsFull       = 3
	)

	switch len(parts) {
	case segmentsMajor:
		if suffix != "" {
			return NormalizedTag{}, errors.Errorf("invalid version %q (suffix without minor/patch)", input)
		}

		return NormalizedTag{Prefix: "v" + parts[0] + "."}, nil
	case segmentsMajorMinor:
		if suffix != "" {
			return NormalizedTag{}, errors.Errorf("invalid version %q (suffix without patch)", input)
		}

		return NormalizedTag{Prefix: "v" + parts[0] + "." + parts[1] + "."}, nil
	case segmentsFull:
		return NormalizedTag{Full: "v" + parts[0] + "." + parts[1] + "." + parts[2] + suffix}, nil
	default:
		return NormalizedTag{}, errors.Errorf("invalid version %q (too many segments)", input)
	}
}
