package panel

import (
	"strconv"
	"strings"

	"github.com/gameap/gameapctl/pkg/configenv"
)

// Keys of config.env that decide how the panel serves HTTP and HTTPS. The panel
// reads them at start and picks a certificate source from whatever is set;
// gameapctl only ever writes them.
const (
	HTTPHostKey      = "HTTP_HOST"
	HTTPPortKey      = "HTTP_PORT"
	HTTPSPortKey     = "HTTPS_PORT"
	TLSCertFileKey   = "TLS_CERT_FILE"
	TLSKeyFileKey    = "TLS_KEY_FILE"
	TLSCertKey       = "TLS_CERT"
	TLSKeyKey        = "TLS_KEY"
	TLSForceHTTPSKey = "TLS_FORCE_HTTPS"

	ACMEEnabledKey       = "ACME_ENABLED"
	ACMEChallengeTypeKey = "ACME_CHALLENGE_TYPE"
	ACMEEmailKey         = "ACME_EMAIL"
	ACMEDomainsKey       = "ACME_DOMAINS"
	ACMEDNSProviderKey   = "ACME_DNS_PROVIDER"
)

const (
	ACMEChallengeHTTP01 = "http-01"
	ACMEChallengeDNS01  = "dns-01"
)

// DefaultHTTPSPort is the port the panel listens on when HTTPS_PORT is unset.
const DefaultHTTPSPort = "443"

// tlsKeysOwned lists every key the https subcommand writes, and therefore every
// key it has to remove to hand the panel back to plain HTTP.
var tlsKeysOwned = []string{
	TLSCertFileKey,
	TLSKeyFileKey,
	TLSCertKey,
	TLSKeyKey,
	TLSForceHTTPSKey,
	HTTPSPortKey,
}

// CertSource is the certificate the panel would serve for a given config.env.
type CertSource int

const (
	CertSourceNone CertSource = iota
	CertSourceACME
	CertSourceFile
	CertSourceInline
)

func (s CertSource) String() string {
	switch s {
	case CertSourceACME:
		return "acme"
	case CertSourceFile:
		return "file"
	case CertSourceInline:
		return "inline"
	case CertSourceNone:
		return "none"
	default:
		return "unknown"
	}
}

// EffectiveCertSource mirrors the panel's own precedence: ACME wins over a
// certificate on disk, which wins over inline PEM content. Anything else leaves
// the HTTPS listener switched off entirely.
func EffectiveCertSource(values map[string]string) CertSource {
	switch {
	case ACMEEnabled(values):
		return CertSourceACME
	case ConfigValue(values, TLSCertFileKey) != "" && ConfigValue(values, TLSKeyFileKey) != "":
		return CertSourceFile
	case ConfigValue(values, TLSCertKey) != "" && ConfigValue(values, TLSKeyKey) != "":
		return CertSourceInline
	default:
		return CertSourceNone
	}
}

// TLSEnabled reports whether the panel would start its HTTPS listener.
func TLSEnabled(values map[string]string) bool {
	return EffectiveCertSource(values) != CertSourceNone
}

// ACMEEnabled repeats the panel's own check: the flag alone is not enough, an
// incompletely configured ACME section is ignored and the panel falls back to a
// static certificate.
func ACMEEnabled(values map[string]string) bool {
	if !boolValue(values, ACMEEnabledKey) {
		return false
	}

	if ConfigValue(values, ACMEEmailKey) == "" || ConfigValue(values, ACMEDomainsKey) == "" {
		return false
	}

	switch ConfigValue(values, ACMEChallengeTypeKey) {
	case "", ACMEChallengeHTTP01:
		return true
	case ACMEChallengeDNS01:
		return ConfigValue(values, ACMEDNSProviderKey) != ""
	default:
		return false
	}
}

// HTTPSPort reports the port the HTTPS listener binds, falling back to the
// panel's own default.
func HTTPSPort(values map[string]string) string {
	if port := ConfigValue(values, HTTPSPortKey); port != "" {
		return port
	}

	return DefaultHTTPSPort
}

// ForceHTTPS reports whether the panel redirects HTTP to HTTPS.
func ForceHTTPS(values map[string]string) bool {
	return boolValue(values, TLSForceHTTPSKey)
}

// TLSEnableUpdates builds the config.env changes that make the panel serve HTTPS
// from a certificate pair on disk. A nil forceHTTPS leaves TLS_FORCE_HTTPS
// untouched, so a redirect configured by hand survives a rerun.
func TLSEnableUpdates(certPath, keyPath, httpsPort string, forceHTTPS *bool) map[string]string {
	updates := map[string]string{
		TLSCertFileKey: certPath,
		TLSKeyFileKey:  keyPath,
		HTTPSPortKey:   httpsPort,

		// Inline material is dead weight next to the files, which the panel prefers
		// silently, so leaving a stale TLS_CERT behind only misleads the next reader.
		TLSCertKey: configenv.RemoveMarker,
		TLSKeyKey:  configenv.RemoveMarker,
	}

	if forceHTTPS != nil {
		updates[TLSForceHTTPSKey] = strconv.FormatBool(*forceHTTPS)
	}

	return updates
}

// TLSDisableUpdates removes every key the https subcommand owns, returning the
// panel to plain HTTP.
func TLSDisableUpdates() map[string]string {
	updates := make(map[string]string, len(tlsKeysOwned))

	for _, key := range tlsKeysOwned {
		updates[key] = configenv.RemoveMarker
	}

	return updates
}

// ConfigValue reads a config.env value the way the panel's own env file loader
// does, without the surrounding whitespace and quotes.
func ConfigValue(values map[string]string, key string) string {
	return strings.TrimSpace(configenv.Unquote(strings.TrimSpace(values[key])))
}

func boolValue(values map[string]string, key string) bool {
	parsed, err := strconv.ParseBool(ConfigValue(values, key))
	if err != nil {
		return false
	}

	return parsed
}
