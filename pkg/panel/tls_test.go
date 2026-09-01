package panel

import (
	"testing"

	"github.com/gameap/gameapctl/pkg/configenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEffectiveCertSource(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		want   CertSource
	}{
		{
			name:   "nothing configured",
			values: map[string]string{"HTTP_PORT": "8025"},
			want:   CertSourceNone,
		},
		{
			name: "certificate files",
			values: map[string]string{
				"TLS_CERT_FILE": "/etc/gameap/certs/panel.crt",
				"TLS_KEY_FILE":  "/etc/gameap/certs/panel.key",
			},
			want: CertSourceFile,
		},
		{
			name:   "certificate file without a key",
			values: map[string]string{"TLS_CERT_FILE": "/etc/gameap/certs/panel.crt"},
			want:   CertSourceNone,
		},
		{
			name: "inline certificate",
			values: map[string]string{
				"TLS_CERT": "-----BEGIN CERTIFICATE-----",
				"TLS_KEY":  "-----BEGIN EC PRIVATE KEY-----",
			},
			want: CertSourceInline,
		},
		{
			name: "acme wins over certificate files",
			values: map[string]string{
				"ACME_ENABLED":  "true",
				"ACME_EMAIL":    "admin@example.com",
				"ACME_DOMAINS":  "panel.example.com",
				"TLS_CERT_FILE": "/etc/gameap/certs/panel.crt",
				"TLS_KEY_FILE":  "/etc/gameap/certs/panel.key",
			},
			want: CertSourceACME,
		},
		{
			name: "incomplete acme falls back to the certificate files",
			values: map[string]string{
				"ACME_ENABLED":  "true",
				"ACME_DOMAINS":  "panel.example.com",
				"TLS_CERT_FILE": "/etc/gameap/certs/panel.crt",
				"TLS_KEY_FILE":  "/etc/gameap/certs/panel.key",
			},
			want: CertSourceFile,
		},
		{
			name: "dns-01 without a provider is not enabled",
			values: map[string]string{
				"ACME_ENABLED":        "true",
				"ACME_EMAIL":          "admin@example.com",
				"ACME_DOMAINS":        "panel.example.com",
				"ACME_CHALLENGE_TYPE": "dns-01",
			},
			want: CertSourceNone,
		},
		{
			name: "dns-01 with a provider",
			values: map[string]string{
				"ACME_ENABLED":        "true",
				"ACME_EMAIL":          "admin@example.com",
				"ACME_DOMAINS":        "panel.example.com",
				"ACME_CHALLENGE_TYPE": "dns-01",
				"ACME_DNS_PROVIDER":   "cloudflare:cloudflare",
			},
			want: CertSourceACME,
		},
		{
			name: "quoted values are read the way the panel reads them",
			values: map[string]string{
				"TLS_CERT_FILE": `"/etc/gameap/certs/panel.crt"`,
				"TLS_KEY_FILE":  `"/etc/gameap/certs/panel.key"`,
			},
			want: CertSourceFile,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, EffectiveCertSource(test.values))
			assert.Equal(t, test.want != CertSourceNone, TLSEnabled(test.values))
		})
	}
}

func TestHTTPSPort(t *testing.T) {
	assert.Equal(t, DefaultHTTPSPort, HTTPSPort(map[string]string{}))
	assert.Equal(t, "8443", HTTPSPort(map[string]string{"HTTPS_PORT": "8443"}))
	assert.Equal(t, "8443", HTTPSPort(map[string]string{"HTTPS_PORT": `"8443"`}))
}

func TestForceHTTPS(t *testing.T) {
	tests := map[string]bool{
		"":      false,
		"false": false,
		"true":  true,
		"1":     true,
		"TRUE":  true,
		"yes":   false,
	}

	for value, want := range tests {
		t.Run(value, func(t *testing.T) {
			assert.Equal(t, want, ForceHTTPS(map[string]string{"TLS_FORCE_HTTPS": value}))
		})
	}
}

func TestTLSEnableUpdates(t *testing.T) {
	updates := TLSEnableUpdates("/etc/gameap/certs/panel.crt", "/etc/gameap/certs/panel.key", "443", nil)

	assert.Equal(t, "/etc/gameap/certs/panel.crt", updates["TLS_CERT_FILE"])
	assert.Equal(t, "/etc/gameap/certs/panel.key", updates["TLS_KEY_FILE"])
	assert.Equal(t, "443", updates["HTTPS_PORT"])
	assert.Equal(t, configenv.RemoveMarker, updates["TLS_CERT"])
	assert.Equal(t, configenv.RemoveMarker, updates["TLS_KEY"])

	_, forceHTTPSTouched := updates["TLS_FORCE_HTTPS"]
	assert.False(t, forceHTTPSTouched, "a redirect configured by hand must survive a rerun")
}

func TestTLSEnableUpdatesForceHTTPS(t *testing.T) {
	for _, want := range []bool{true, false} {
		updates := TLSEnableUpdates("cert", "key", "443", &want)
		assert.Equal(t, map[bool]string{true: "true", false: "false"}[want], updates["TLS_FORCE_HTTPS"])
	}
}

func TestTLSDisableUpdates(t *testing.T) {
	updates := TLSDisableUpdates()

	require.Len(t, updates, len(tlsKeysOwned))

	for _, key := range tlsKeysOwned {
		assert.Equalf(t, configenv.RemoveMarker, updates[key], "%s must be removed", key)
	}
}

func TestTLSUpdatesRoundTrip(t *testing.T) {
	path := writeConfigEnv(t, "HTTP_HOST=panel.example.com\nHTTP_PORT=80\nTLS_CERT=inline\nTLS_KEY=inline\n")

	lines, _, err := configenv.Read(path)
	require.NoError(t, err)

	forceHTTPS := true
	require.NoError(t, configenv.Update(path, lines, TLSEnableUpdates("/c.crt", "/c.key", "443", &forceHTTPS)))

	_, values, err := configenv.Read(path)
	require.NoError(t, err)

	assert.Equal(t, CertSourceFile, EffectiveCertSource(values))
	assert.Equal(t, "443", HTTPSPort(values))
	assert.True(t, ForceHTTPS(values))
	assert.NotContains(t, values, "TLS_CERT")

	lines, _, err = configenv.Read(path)
	require.NoError(t, err)
	require.NoError(t, configenv.Update(path, lines, TLSDisableUpdates()))

	_, values, err = configenv.Read(path)
	require.NoError(t, err)

	assert.Equal(t, CertSourceNone, EffectiveCertSource(values))
	assert.Equal(t, "panel.example.com", values["HTTP_HOST"])
	assert.Equal(t, "80", values["HTTP_PORT"])
}
