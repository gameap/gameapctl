package releasefinder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

type Release struct {
	URL string
	Tag string
}

func Find(_ context.Context, api, kernel, platform string) (*Release, error) {
	resp, err := http.Get(api) //nolint:bodyclose,noctx,gosec
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get releases")
	}
	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			log.Println(err)
		}
	}(resp.Body)

	link, err := findRelease(resp.Body, kernel, platform)
	if err != nil {
		return nil, err
	}

	return link, nil
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

func findRelease(reader io.Reader, os string, arch string) (*Release, error) {
	r := []releases{}
	d := json.NewDecoder(reader)
	err := d.Decode(&r)
	if err != nil {
		return nil, err
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
