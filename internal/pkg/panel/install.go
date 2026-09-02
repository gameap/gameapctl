package panel

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func SetupGameAPFromGithubV3(
	ctx context.Context,
	pm packagemanager.PackageManager,
	path string,
	branch string,
) error {
	var err error

	fmt.Println("Installing git ...")
	if err = pm.Install(ctx, packagemanager.GitPackage); err != nil {
		return errors.WithMessage(err, "failed to install git")
	}

	fmt.Println("Installing composer ...")
	if err = pm.Install(ctx, packagemanager.ComposerPackage); err != nil {
		return errors.WithMessage(err, "failed to install composer")
	}

	fmt.Println("Installing nodejs ...")
	if err = pm.Install(ctx, packagemanager.NodeJSPackage); err != nil {
		return errors.WithMessage(err, "failed to install nodejs")
	}

	fmt.Println("Cloning gameap ...")
	err = oscore.ExecCommand(
		ctx, "git", "clone", "-b", branch, gameap.GithubRepositoryPanelV3, path,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to clone gameap from github")
	}

	fmt.Println("Installing composer dependencies ...")

	cmdName, args, err := packagemanager.DefinePHPComposerCommandAndArgs(
		"update", "--no-dev", "--optimize-autoloader", "--no-interaction", "--working-dir", path,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to define php composer command and args")
	}

	err = oscore.ExecCommand(ctx, cmdName, args...)
	if err != nil {
		return errors.WithMessage(err, "failed to run composer update")
	}

	fmt.Println("Building styles ...")
	err = BuildStylesV3(ctx, path)
	if err != nil {
		return errors.WithMessage(err, "failed to build styles")
	}

	return nil
}

func SetupGameAPFromGithubV4(
	ctx context.Context,
	pm packagemanager.PackageManager,
	branch string,
	outputPath string,
	userScope bool,
) error {
	if err := ensureGithubBuildToolsV4(ctx, pm, userScope); err != nil {
		return err
	}

	path, err := os.MkdirTemp("", "gameapctl")
	if err != nil {
		return errors.WithMessage(err, "failed to create temp dir")
	}
	defer func() {
		if removeErr := os.RemoveAll(path); removeErr != nil {
			log.Printf("Failed to remove temp dir %s: %v\n", path, removeErr)
		}
	}()

	fmt.Println("Cloning gameap ...")
	err = oscore.ExecCommand(
		ctx, "git", "clone", "-b", branch, gameap.GithubRepositoryPanelV4, path,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to clone gameap from github")
	}

	fmt.Println("Building styles ...")
	err = BuildStylesV4(ctx, path)
	if err != nil {
		return errors.WithMessage(err, "failed to build styles")
	}

	fmt.Println("Building gameap ...")
	err = BuildGoPanel(ctx, path, outputPath)
	if err != nil {
		return errors.WithMessage(err, "failed to build game ap")
	}

	return nil
}

func ensureGithubBuildToolsV4(ctx context.Context, pm packagemanager.PackageManager, userScope bool) error {
	if userScope {
		for _, tool := range []string{"git", "go", "npm"} {
			if !utils.IsCommandAvailable(tool) {
				return errors.Errorf(
					"%s is required to build GameAP from GitHub in user scope; "+
						"please install it manually (e.g. via sudo) and retry",
					tool,
				)
			}
		}

		return nil
	}

	fmt.Println("Installing git ...")
	if err := pm.Install(ctx, packagemanager.GitPackage); err != nil {
		return errors.WithMessage(err, "failed to install git")
	}

	fmt.Println("Installing nodejs ...")
	if err := pm.Install(ctx, packagemanager.NodeJSPackage); err != nil {
		return errors.WithMessage(err, "failed to install nodejs")
	}

	fmt.Println("Installing golang ...")
	if err := pm.Install(ctx, packagemanager.GOPackage); err != nil {
		return errors.WithMessage(err, "failed to install golang")
	}
	packagemanager.UpdateEnvPath(ctx)

	return nil
}

func SetupGameAPFromRepo(ctx context.Context, path string) error {
	tempDir, err := os.MkdirTemp("", "gameap")
	if err != nil {
		return errors.WithMessage(err, "failed to create temp dir")
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			log.Println(err)
		}
	}(tempDir)

	fmt.Println("Downloading GameAP ...")
	downloadPath, err := url.JoinPath(gameap.Repository(), "gameap/latest.tar.gz")
	if err != nil {
		return errors.WithMessage(err, "failed to join url")
	}

	log.Println("Download path: ", downloadPath)
	err = utils.Download(ctx, downloadPath, tempDir)
	if err != nil {
		return errors.WithMessagef(err, "failed to download gameap from '%s'", downloadPath)
	}

	err = utils.Move(tempDir+string(os.PathSeparator)+"gameap", path)
	if err != nil {
		return errors.WithMessage(err, "failed to move gameap")
	}

	return nil
}

func CheckInstallation(ctx context.Context, host, port string, https bool) error {
	return checkInstallation(ctx, createHealthURL(host, port, https, "/api/healthz"))
}

func CheckInstallationV4(ctx context.Context, host, port string, https bool) error {
	return checkInstallation(ctx, createHealthURL(host, port, https, "/api/health"))
}

func createHealthURL(host, port string, https bool, endpoint string) string {
	if strings.Contains(host, ":") {
		host = "[" + strings.Trim(host, "[]") + "]"
	}

	scheme, defaultPort := "http", "80"
	if https {
		scheme, defaultPort = "https", "443"
	}

	hostPort := host
	if port != defaultPort {
		hostPort = host + ":" + port
	}

	return scheme + "://" + hostPort + endpoint
}

// localProbeClient deliberately does not verify the panel's certificate. An
// installation serving HTTPS from a self-signed certificate is the common case
// here, and a probe of the machine gameapctl is running on is a liveness check
// against the panel that was just installed or restarted, not a trust decision.
// Nothing but the certificate is unverified: the transport keeps the timeouts
// the default one carries. The proxy is dropped: a probe that stays on this
// machine has no business leaving it, and neither the unspecified address nor
// the machine's own public one is bypassed by the environment proxy on its own.
var localProbeClient = newLocalProbeClient()

func newLocalProbeClient() *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return http.DefaultClient
	}

	transport = transport.Clone()
	transport.Proxy = nil
	//nolint:gosec // loopback liveness probe, see above
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	return &http.Client{Transport: transport}
}

// healthCheckClient picks the client for a probe. A probe that leaves this
// machine is verified and proxied like any other request; a probe that stays on
// it is neither.
func healthCheckClient(u *url.URL) *http.Client {
	if !isLocalHost(u.Hostname()) {
		return http.DefaultClient
	}

	return localProbeClient
}

// isLocalHost reports whether a request to host stays on this machine. The
// unspecified address counts: the panel is configured with it to listen
// everywhere, and a connection to it lands on loopback. So does an address of
// one of the interfaces, which is what HTTP_HOST holds on an installation that
// answers on its public address and nowhere else.
func isLocalHost(host string) bool {
	if host == "localhost" {
		return true
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	if ip.IsLoopback() || ip.IsUnspecified() {
		return true
	}

	for _, local := range utils.DetectIPs() {
		if ip.Equal(net.ParseIP(local)) {
			return true
		}
	}

	return false
}

func checkInstallation(ctx context.Context, healthURL string) error {
	log.Printf("Checking installation at %s\n", healthURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return err
	}
	//nolint:bodyclose
	response, err := healthCheckClient(req.URL).Do(req)
	if err != nil {
		return err
	}
	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to close response body"))
		}
	}(response.Body)

	if response.StatusCode != http.StatusOK {
		log.Println("unsuccessful response from panel, invalid status code")
		body, _ := io.ReadAll(response.Body)
		log.Println(string(body))

		return errors.New("unsuccessful response from panel, invalid status code")
	}

	r := struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return errors.WithMessage(err, "failed to read response body")
	}

	err = json.Unmarshal(body, &r)
	if err != nil {
		log.Printf("Response body: %s\n", string(body))

		return errors.WithMessage(err, "failed to decode response")
	}

	if r.Status != "ok" {
		return errors.New("unsuccessful response from panel, invalid status in response")
	}

	return nil
}
