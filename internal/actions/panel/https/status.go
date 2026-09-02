package https

import (
	"context"
	"crypto/x509"
	"log"
	"net"
	"strings"
	"time"

	panelpkg "github.com/gameap/gameapctl/internal/pkg/panel"
	"github.com/gameap/gameapctl/pkg/configenv"
	"github.com/gameap/gameapctl/pkg/panel"
	"github.com/gameap/gameapctl/pkg/tlsprobe"
	"github.com/urfave/cli/v2"
)

func Status(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	paths, err := panelpkg.ResolveScope(ctx, cliCtx.String("scope"))
	if err != nil {
		return err
	}

	_, values, err := configenv.Read(paths.ConfigFilePath)
	if err != nil {
		return err
	}

	source := panel.EffectiveCertSource(values)
	bind := panel.ResolveBindAddress(ctx, values)

	log.Println("Installation scope:", paths.Scope)
	log.Println("Config:            ", paths.ConfigFilePath)
	log.Println("HTTP:              ", httpSummary(values))
	log.Println("Listening on:      ", bind)
	log.Println("Certificate source:", source)

	if source == panel.CertSourceNone {
		log.Println("HTTPS is disabled. Run 'gameapctl panel https enable' to serve it with a self-signed certificate.")

		return nil
	}

	httpsPort := panel.HTTPSPort(values)

	log.Println("HTTPS port:        ", httpsPort)
	log.Println("Redirect HTTP:     ", panel.ForceHTTPS(values))

	reportSource(source, values)
	reportServedCertificate(ctx, bind, httpsPort)

	return nil
}

func httpSummary(values map[string]string) string {
	host := panel.ConfigValue(values, panel.HTTPHostKey)
	if host == "" {
		host = wildcardHost
	}

	port := panel.ConfigValue(values, panel.HTTPPortKey)
	if port == "" {
		return host
	}

	return net.JoinHostPort(host, port)
}

func reportSource(source panel.CertSource, values map[string]string) {
	switch source {
	case panel.CertSourceACME:
		log.Println("  Domains:    ", panel.ConfigValue(values, panel.ACMEDomainsKey))
		log.Println("  Challenge:  ", acmeChallenge(values))
		log.Println("  Account:    ", panel.ConfigValue(values, panel.ACMEEmailKey))
		log.Println("The certificate is managed by the panel; see 'gameapctl panel https letsencrypt'.")
	case panel.CertSourceFile:
		reportCertificateFiles(values)
	case panel.CertSourceInline:
		log.Println("The certificate is stored inline in config.env (TLS_CERT and TLS_KEY).")
	case panel.CertSourceNone:
	}
}

func reportCertificateFiles(values map[string]string) {
	certPath := panel.ConfigValue(values, panel.TLSCertFileKey)
	keyPath := panel.ConfigValue(values, panel.TLSKeyFileKey)

	log.Println("  Certificate:", certPath)
	log.Println("  Private key:", keyPath)

	leaf, err := loadLeaf(certPath, keyPath)
	if err != nil {
		log.Println("  Warning:    ", err)

		return
	}

	reportCertificate(leaf)
}

func reportCertificate(leaf *x509.Certificate) {
	log.Println("  Subject:    ", leaf.Subject.CommonName)
	log.Println("  Names:      ", certificateNames(leaf))
	log.Println("  Self-signed:", isSelfSigned(leaf))
	log.Println("  Expires:    ", expirySummary(leaf.NotAfter, time.Now()))
	log.Println("  Fingerprint:", fingerprint(leaf))
}

// reportServedCertificate reads the certificate the panel is actually serving.
// A fingerprint that differs from the configured one means the running process
// predates the last change and has to be restarted.
func reportServedCertificate(ctx context.Context, bind panel.BindAddress, httpsPort string) {
	addrs := bind.ProbeAddrs(httpsPort)

	var (
		served string
		result tlsprobe.Result
	)

	err := panel.ProbeEach(addrs, func(addr string) error {
		probed, probeErr := tlsprobe.Leaf(ctx, addr, probeTimeout)
		if probeErr != nil {
			return probeErr
		}

		served, result = addr, probed

		return nil
	})
	if err != nil {
		log.Printf("Nothing is listening on %s: %v\n", strings.Join(addrs, ", "), err)

		return
	}

	switch {
	case result.HandshakeErr != nil:
		// The certificate arrives before a server that requires a client one
		// aborts, so a failed handshake is still worth reporting alongside it.
		log.Printf("The TLS handshake with %s failed: %v\n", served, result.HandshakeErr)
	case result.Leaf == nil:
		log.Printf("%s answered without a certificate.\n", served)
	}

	if result.Leaf == nil {
		return
	}

	log.Println("Served on         ", served)
	log.Println("  Names:      ", certificateNames(result.Leaf))
	log.Println("  Expires:    ", expirySummary(result.Leaf.NotAfter, time.Now()))
	log.Println("  Fingerprint:", fingerprint(result.Leaf))
}

// acmeChallenge names the challenge the panel would run, which is http-01 when
// config.env says nothing.
func acmeChallenge(values map[string]string) string {
	if challenge := panel.ConfigValue(values, panel.ACMEChallengeTypeKey); challenge != "" {
		return challenge
	}

	return panel.ACMEChallengeHTTP01
}
