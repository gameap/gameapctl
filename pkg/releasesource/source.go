package releasesource

import (
	"net/http"

	"github.com/gameap/gameapctl/pkg/releasefinder"
)

type Component string

const (
	ComponentGameAPCtl Component = "gameapctl"
	ComponentPanel     Component = "gameap"
	ComponentDaemon    Component = "gameap-daemon"
)

type sourceKind int

const (
	kindGitHub sourceKind = iota
	kindCDN
)

const sourceNameGitHub = "github"

type source struct {
	name    string
	kind    sourceKind
	baseURL string
}

func defaultSources() []source {
	return []source{
		{name: sourceNameGitHub, kind: kindGitHub, baseURL: "https://api.github.com"},
		{name: "cdn.gameap.com", kind: kindCDN, baseURL: "https://cdn.gameap.com"},
		{name: "cdn.gameap.ru", kind: kindCDN, baseURL: "https://cdn.gameap.ru"},
	}
}

// The daemon GitHub repository is gameap/daemon while its CDN path is gameap-daemon.
var githubReleasesPath = map[Component]string{
	ComponentGameAPCtl: "/repos/gameap/gameapctl/releases",
	ComponentPanel:     "/repos/gameap/gameap/releases",
	ComponentDaemon:    "/repos/gameap/daemon/releases",
}

func (s source) probeRequest() (string, string) {
	if s.kind == kindGitHub {
		// The rate_limit endpoint does not count against the REST API quota
		// and does not support HEAD.
		return http.MethodGet, s.baseURL + "/rate_limit"
	}

	// CDN root responds with 403, probe a real object instead.
	return http.MethodHead, s.baseURL + "/" + string(ComponentGameAPCtl) + "/releases.json"
}

func (s source) releasesURL(component Component) string {
	if s.kind == kindGitHub {
		return s.baseURL + githubReleasesPath[component]
	}

	return s.baseURL + "/" + string(component) + "/releases.json"
}

func (s source) downloadURL(component Component, rel *releasefinder.Release) string {
	if s.kind == kindGitHub {
		// browser_download_url points at github.com even in CDN-mirrored release lists.
		return rel.URL
	}

	return s.baseURL + "/" + string(component) + "/" + rel.Tag + "/" + rel.AssetName
}
