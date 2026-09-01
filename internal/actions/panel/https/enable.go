package https

import (
	"bytes"
	"context"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	panelpkg "github.com/gameap/gameapctl/internal/pkg/panel"
	"github.com/gameap/gameapctl/pkg/certgen"
	"github.com/gameap/gameapctl/pkg/configenv"
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/panel"
	"github.com/gameap/gameapctl/pkg/tlsprobe"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

const privilegedPortLimit = 1024

type certificateSetup struct {
	certPath string
	keyPath  string
	leaf     *x509.Certificate
}

func Enable(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	paths, err := panelpkg.ResolveScope(ctx, cliCtx.String("scope"))
	if err != nil {
		return err
	}

	if err = panelpkg.CheckBinaryInstalled(paths); err != nil {
		return err
	}

	configPath := paths.ConfigFilePath
	log.Printf("Reading config from: %s\n", configPath)

	lines, values, err := configenv.Read(configPath)
	if err != nil {
		return err
	}

	if panel.EffectiveCertSource(values) == panel.CertSourceACME {
		return errors.New(
			"ACME is enabled and the panel ignores a certificate on disk while it is; " +
				"run 'gameapctl panel https letsencrypt disable' first",
		)
	}

	setup, err := prepareCertificate(ctx, cliCtx, paths, values)
	if err != nil {
		return err
	}

	httpsPort, err := resolveHTTPSPort(cliCtx.Int("port"), values, paths.Scope)
	if err != nil {
		return err
	}

	if err = checkHTTPSPort(values, httpsPort, paths.Scope); err != nil {
		return err
	}

	updates := panel.TLSEnableUpdates(setup.certPath, setup.keyPath, httpsPort, forceHTTPSUpdate(cliCtx))

	if err = configenv.Update(configPath, lines, updates); err != nil {
		return errors.WithMessage(err, "failed to write config")
	}

	log.Println("config.env updated. Restarting gameap ...")

	if err = restartAndVerify(ctx, paths, httpsPort, setup.leaf); err != nil {
		return rollback(ctx, paths, lines, err)
	}

	reportEnabled(cliCtx, values, setup, httpsPort)

	return nil
}

func prepareCertificate(
	ctx context.Context, cliCtx *cli.Context, paths gameap.PanelPaths, values map[string]string,
) (certificateSetup, error) {
	certFlag := strings.TrimSpace(cliCtx.String("cert"))
	keyFlag := strings.TrimSpace(cliCtx.String("key"))

	switch {
	case (certFlag == "") != (keyFlag == ""):
		return certificateSetup{}, errors.New("--cert and --key have to be given together")
	case certFlag != "" && cliCtx.Bool("self-signed"):
		return certificateSetup{}, errors.New("--self-signed and --cert are mutually exclusive")
	case certFlag != "":
		return useProvidedCertificate(certFlag, keyFlag, paths)
	default:
		return issueSelfSigned(ctx, cliCtx, paths, values)
	}
}

func useProvidedCertificate(certFlag, keyFlag string, paths gameap.PanelPaths) (certificateSetup, error) {
	certPath, err := filepath.Abs(certFlag)
	if err != nil {
		return certificateSetup{}, errors.Wrap(err, "failed to resolve the certificate path")
	}

	keyPath, err := filepath.Abs(keyFlag)
	if err != nil {
		return certificateSetup{}, errors.Wrap(err, "failed to resolve the private key path")
	}

	leaf, err := loadLeaf(certPath, keyPath)
	if err != nil {
		return certificateSetup{}, err
	}

	warnUnreachablePath(paths, certPath)
	warnUnreachablePath(paths, keyPath)

	log.Printf("Using the certificate at %s\n", certPath)

	return certificateSetup{certPath: certPath, keyPath: keyPath, leaf: leaf}, nil
}

func issueSelfSigned(
	ctx context.Context, cliCtx *cli.Context, paths gameap.PanelPaths, values map[string]string,
) (certificateSetup, error) {
	certPath, keyPath := certPaths(paths)

	opts, err := buildSelfSignedOptions(collectSANInput(cliCtx, values), validity(cliCtx))
	if err != nil {
		return certificateSetup{}, err
	}

	regenerate, reason := needsRegeneration(certPath, keyPath, opts, time.Now())

	switch {
	case cliCtx.Bool("force"):
		log.Println("Reissuing the self-signed certificate.")
	case regenerate:
		log.Printf("Issuing a self-signed certificate: %s.\n", reason)
	default:
		leaf, loadErr := loadLeaf(certPath, keyPath)
		if loadErr != nil {
			return certificateSetup{}, loadErr
		}

		log.Printf("Keeping the certificate at %s, it covers every requested name.\n", certPath)

		return certificateSetup{certPath: certPath, keyPath: keyPath, leaf: leaf}, nil
	}

	certPEM, keyPEM, err := certgen.GenerateSelfSigned(opts)
	if err != nil {
		return certificateSetup{}, err
	}

	if err = writeCertificate(certPath, keyPath, certPEM, keyPEM); err != nil {
		return certificateSetup{}, err
	}

	if err = applyCertPermissions(ctx, paths, certDir(paths)); err != nil {
		return certificateSetup{}, err
	}

	leaf, err := loadLeaf(certPath, keyPath)
	if err != nil {
		return certificateSetup{}, err
	}

	log.Printf("Certificate written to %s\n", certPath)

	return certificateSetup{certPath: certPath, keyPath: keyPath, leaf: leaf}, nil
}

func collectSANInput(cliCtx *cli.Context, values map[string]string) sanInput {
	hostname, err := os.Hostname()
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to detect the host name"))
	}

	return sanInput{
		httpHost:    panel.ConfigValue(values, panel.HTTPHostKey),
		hostname:    hostname,
		detectedIPs: utils.DetectIPs(),
		domains:     cliCtx.StringSlice("domain"),
		ips:         cliCtx.StringSlice("ip"),
	}
}

func validity(cliCtx *cli.Context) time.Duration {
	days := cliCtx.Int("days")
	if days <= 0 {
		days = defaultValidityDays
	}

	return time.Duration(days) * day
}

func forceHTTPSUpdate(cliCtx *cli.Context) *bool {
	if !cliCtx.IsSet("force-https") {
		return nil
	}

	value := cliCtx.Bool("force-https")

	return &value
}

// checkHTTPSPort probes the port only when the panel is not serving it already:
// a rerun that keeps the port would otherwise fail against the panel's own
// listener.
func checkHTTPSPort(values map[string]string, httpsPort, scope string) error {
	if panel.TLSEnabled(values) && panel.HTTPSPort(values) == httpsPort {
		return nil
	}

	if err := utils.CheckPortAvailability("", httpsPort); err != nil {
		return errors.WithMessage(err, portUnavailableMessage(httpsPort, scope))
	}

	return nil
}

func portUnavailableMessage(httpsPort, scope string) string {
	message := fmt.Sprintf("port %s is not available", httpsPort)

	port, err := strconv.Atoi(httpsPort)
	if err == nil && port < privilegedPortLimit && scope == gameap.ScopeUser {
		message += ", and a systemd user unit cannot be granted CAP_NET_BIND_SERVICE, " +
			"so pass --port with a port above 1024 or lower net.ipv4.ip_unprivileged_port_start"
	}

	return message
}

// warnUnreachablePath reports a certificate the panel will not be able to read:
// the system unit runs with ProtectHome=true, which hides these directories
// however the files themselves are permissioned.
func warnUnreachablePath(paths gameap.PanelPaths, path string) {
	if runtime.GOOS == "windows" || paths.Scope != gameap.ScopeSystem {
		return
	}

	for _, prefix := range []string{"/home/", "/root/", "/run/user/"} {
		if !strings.HasPrefix(path, prefix) {
			continue
		}

		log.Printf(
			"Warning: %s is under %s, which ProtectHome=true hides from the gameap service. "+
				"Move the certificate elsewhere, %s for example.\n",
			path, strings.TrimSuffix(prefix, "/"), certDir(paths),
		)

		return
	}
}

func restartAndVerify(
	ctx context.Context, paths gameap.PanelPaths, httpsPort string, expected *x509.Certificate,
) error {
	if err := panel.Restart(ctx, panel.Options{Scope: paths.Scope}); err != nil {
		return errors.WithMessage(err, "failed to restart gameap")
	}

	return waitForHTTPS(ctx, httpsPort, expected)
}

// waitForHTTPS blocks until the panel answers a handshake with the certificate
// that was just configured. This is not a nicety: the panel exits when it cannot
// load the configured certificate, taking the plain HTTP listener down with it,
// so an unverified change can leave the installation unreachable.
func waitForHTTPS(ctx context.Context, httpsPort string, expected *x509.Certificate) error {
	addr := net.JoinHostPort("127.0.0.1", httpsPort)

	err := waitFor(ctx, verifyInterval, func() error {
		return probeHTTPS(ctx, addr, expected)
	})
	if err != nil {
		return errors.WithMessagef(err, "the panel is not serving HTTPS on port %s", httpsPort)
	}

	return nil
}

func probeHTTPS(ctx context.Context, addr string, expected *x509.Certificate) error {
	result, err := tlsprobe.Leaf(ctx, addr, probeTimeout)
	if err != nil {
		return err
	}

	if result.Leaf == nil {
		return errors.Errorf("%s answered without a certificate", addr)
	}

	if !bytes.Equal(result.Leaf.Raw, expected.Raw) {
		return errors.New("the panel is still serving another certificate")
	}

	return nil
}

func rollback(ctx context.Context, paths gameap.PanelPaths, lines []string, cause error) error {
	log.Println("Restoring the previous config.env ...")

	if err := configenv.Write(paths.ConfigFilePath, lines); err != nil {
		return errors.WithMessagef(err, "failed to restore config.env after: %s", cause)
	}

	// The context that reached here is usually already cancelled: an interrupt
	// during the verification is one of the ways the rollback is entered, and
	// the panel still has to be brought back up on the restored config.
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
	defer cancel()

	if err := panel.Restart(cleanupCtx, panel.Options{Scope: paths.Scope}); err != nil {
		return errors.WithMessagef(err, "config.env restored, but gameap failed to restart after: %s", cause)
	}

	return errors.WithMessage(cause, "HTTPS is not enabled, the previous configuration is restored")
}

func reportEnabled(cliCtx *cli.Context, values map[string]string, setup certificateSetup, httpsPort string) {
	log.Println("HTTPS is enabled.")
	log.Println("  URL:        ", panelURL(values, httpsPort))
	log.Println("  Certificate:", setup.certPath)
	log.Println("  Private key:", setup.keyPath)
	log.Println("  Names:      ", certificateNames(setup.leaf))
	log.Println("  Expires:    ", setup.leaf.NotAfter.Format(time.DateOnly))
	log.Println("  Fingerprint:", fingerprint(setup.leaf))

	if isSelfSigned(setup.leaf) {
		log.Println(
			"The certificate is self-signed, so browsers warn about it until it is added to the " +
				"trust store of every machine that opens the panel.",
		)
	}

	if forceHTTPS := forceHTTPSUpdate(cliCtx); forceHTTPS != nil && *forceHTTPS {
		log.Println("HTTP requests are redirected to HTTPS.")

		return
	}

	httpPort := panel.ConfigValue(values, panel.HTTPPortKey)
	if httpPort == "" {
		httpPort = gameap.DefaultPanelPort
	}

	log.Printf("HTTP is still served on port %s. Pass --force-https to redirect it.\n", httpPort)
}

func panelURL(values map[string]string, httpsPort string) string {
	host := panel.ConfigValue(values, panel.HTTPHostKey)
	if host == "" || host == wildcardHost {
		host = localhostName
	}

	if httpsPort == panel.DefaultHTTPSPort {
		return "https://" + urlHost(host)
	}

	return "https://" + net.JoinHostPort(host, httpsPort)
}

// urlHost brackets an IPv6 literal, which a URL needs even where there is no
// port to separate it from. net.JoinHostPort does it for every other case.
func urlHost(host string) string {
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}

	return host
}

func isSelfSigned(cert *x509.Certificate) bool {
	return cert.Issuer.String() == cert.Subject.String()
}
