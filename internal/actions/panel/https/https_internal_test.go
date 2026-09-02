package https

import (
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gameap/gameapctl/pkg/certgen"
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/panel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSelfSignedOptions(t *testing.T) {
	tests := []struct {
		name       string
		in         sanInput
		wantCN     string
		wantDNS    []string
		wantIPs    []string
		wantErrMsg string
	}{
		{
			name: "host name, machine name and detected addresses",
			in: sanInput{
				httpHost:    "panel.example.com",
				hostname:    "gameap-host",
				detectedIPs: []string{"192.168.1.10", "127.0.0.1"},
			},
			wantCN:  "panel.example.com",
			wantDNS: []string{"gameap-host", "localhost", "panel.example.com"},
			wantIPs: []string{"127.0.0.1", "192.168.1.10", "::1"},
		},
		{
			name:    "an address in HTTP_HOST becomes an address, not a name",
			in:      sanInput{httpHost: "203.0.113.10", hostname: "gameap-host"},
			wantCN:  "203.0.113.10",
			wantDNS: []string{"gameap-host", "localhost"},
			wantIPs: []string{"127.0.0.1", "203.0.113.10", "::1"},
		},
		{
			name:    "the wildcard host names nothing and is skipped",
			in:      sanInput{httpHost: "0.0.0.0", hostname: "gameap-host"},
			wantCN:  "gameap-host",
			wantDNS: []string{"gameap-host", "localhost"},
			wantIPs: []string{"127.0.0.1", "::1"},
		},
		{
			name: "explicit names replace the detected ones",
			in: sanInput{
				httpHost:    "panel.example.com",
				hostname:    "gameap-host",
				detectedIPs: []string{"192.168.1.10"},
				domains:     []string{"gameap.example.org"},
				ips:         []string{"198.51.100.7"},
			},
			wantCN:  "gameap.example.org",
			wantDNS: []string{"gameap.example.org", "localhost"},
			wantIPs: []string{"127.0.0.1", "198.51.100.7", "::1"},
		},
		{
			name:    "nothing detected still yields a usable certificate",
			in:      sanInput{},
			wantCN:  "localhost",
			wantDNS: []string{"localhost"},
			wantIPs: []string{"127.0.0.1", "::1"},
		},
		{
			name:       "an unparsable address is rejected",
			in:         sanInput{ips: []string{"not-an-ip"}},
			wantErrMsg: `invalid IP address "not-an-ip"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opts, err := buildSelfSignedOptions(test.in, time.Hour)

			if test.wantErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.wantErrMsg)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.wantCN, opts.CommonName)
			assert.Equal(t, test.wantDNS, opts.DNSNames)
			assert.Equal(t, test.wantIPs, ipStrings(opts.IPAddresses))
			assert.Equal(t, time.Hour, opts.ValidFor)
		})
	}
}

func TestNeedsRegeneration(t *testing.T) {
	now := time.Now()

	opts := certgen.SelfSignedOptions{
		DNSNames:    []string{"panel.example.com", "localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		ValidFor:    365 * day,
	}

	t.Run("no certificate", func(t *testing.T) {
		dir := t.TempDir()

		regenerate, reason := needsRegeneration(
			filepath.Join(dir, certFileName), filepath.Join(dir, keyFileName), opts, now,
		)

		assert.True(t, regenerate)
		assert.Equal(t, "no certificate found", reason)
	})

	t.Run("certificate and key from different pairs", func(t *testing.T) {
		certPath, _ := writePair(t, opts)
		_, keyPath := writePair(t, opts)

		regenerate, reason := needsRegeneration(certPath, keyPath, opts, now)

		assert.True(t, regenerate)
		assert.Contains(t, reason, "not a usable pair")
	})

	t.Run("certificate still fits", func(t *testing.T) {
		certPath, keyPath := writePair(t, opts)

		regenerate, reason := needsRegeneration(certPath, keyPath, opts, now)

		assert.False(t, regenerate)
		assert.Empty(t, reason)
	})

	t.Run("certificate about to expire", func(t *testing.T) {
		shortLived := opts
		shortLived.ValidFor = 10 * day

		certPath, keyPath := writePair(t, shortLived)

		regenerate, reason := needsRegeneration(certPath, keyPath, opts, now)

		assert.True(t, regenerate)
		assert.Contains(t, reason, "expires on")
	})

	t.Run("certificate does not cover a requested name", func(t *testing.T) {
		certPath, keyPath := writePair(t, opts)

		wanted := opts
		wanted.DNSNames = append([]string{"new.example.com"}, opts.DNSNames...)

		regenerate, reason := needsRegeneration(certPath, keyPath, wanted, now)

		assert.True(t, regenerate)
		assert.Contains(t, reason, "new.example.com")
	})

	t.Run("certificate does not cover a requested address", func(t *testing.T) {
		certPath, keyPath := writePair(t, opts)

		wanted := opts
		wanted.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("198.51.100.7")}

		regenerate, reason := needsRegeneration(certPath, keyPath, wanted, now)

		assert.True(t, regenerate)
		assert.Contains(t, reason, "198.51.100.7")
	})
}

func TestResolveHTTPSPort(t *testing.T) {
	tests := []struct {
		name     string
		flagPort int
		values   map[string]string
		scope    string
		want     string
		wantErr  bool
	}{
		{
			name:  "system scope default",
			scope: gameap.ScopeSystem,
			want:  "443",
		},
		{
			name:  "user scope stays off a privileged port",
			scope: gameap.ScopeUser,
			want:  userScopeHTTPSPort,
		},
		{
			name:   "the configured port is kept",
			values: map[string]string{"HTTPS_PORT": "9443"},
			scope:  gameap.ScopeSystem,
			want:   "9443",
		},
		{
			name:     "the flag wins over the configured port",
			flagPort: 8443,
			values:   map[string]string{"HTTPS_PORT": "9443"},
			scope:    gameap.ScopeSystem,
			want:     "8443",
		},
		{
			name:     "a port outside the range is rejected",
			flagPort: 70000,
			scope:    gameap.ScopeSystem,
			wantErr:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			port, err := resolveHTTPSPort(test.flagPort, test.values, test.scope)

			if test.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, port)
		})
	}
}

func TestExpirySummary(t *testing.T) {
	now := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)

	assert.Equal(t, "2026-09-11 (in 10 days)", expirySummary(now.Add(10*day), now))
	assert.Equal(t, "2026-08-22 (expired 10 days ago)", expirySummary(now.Add(-10*day), now))
}

func TestDisableUpdates(t *testing.T) {
	withoutACME := disableUpdates(map[string]string{
		"TLS_CERT_FILE": "/etc/gameap/certs/panel.crt",
		"TLS_KEY_FILE":  "/etc/gameap/certs/panel.key",
	})

	assert.NotContains(t, withoutACME, "ACME_ENABLED")

	withACME := disableUpdates(map[string]string{
		"ACME_ENABLED": "true",
		"ACME_EMAIL":   "admin@example.com",
		"ACME_DOMAINS": "panel.example.com",
	})

	assert.Equal(t, "false", withACME["ACME_ENABLED"])
	assert.Contains(t, withACME, "TLS_CERT_FILE")
}

func TestWriteCertificate(t *testing.T) {
	t.Run("the pair is replaced and nothing is left beside it", func(t *testing.T) {
		dir := t.TempDir()
		certPath, keyPath := existingPair(t, dir)

		require.NoError(t, writeCertificate(certPath, keyPath, []byte("new cert"), []byte("new key")))

		assert.Equal(t, "new cert", readFile(t, certPath))
		assert.Equal(t, "new key", readFile(t, keyPath))
		assert.ElementsMatch(t, []string{certFileName, keyFileName}, dirEntries(t, dir))

		if runtime.GOOS != "windows" {
			info, err := os.Stat(keyPath)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(keyFileMode), info.Mode().Perm())
		}
	})

	t.Run("a leftover staged key is not reused with its old mode", func(t *testing.T) {
		dir := t.TempDir()
		certPath, keyPath := existingPair(t, dir)
		require.NoError(t, os.WriteFile(keyPath+stagedSuffix, []byte("leftover"), certFileMode))

		require.NoError(t, writeCertificate(certPath, keyPath, []byte("new cert"), []byte("new key")))

		assert.Equal(t, "new key", readFile(t, keyPath))

		if runtime.GOOS != "windows" {
			info, err := os.Stat(keyPath)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(keyFileMode), info.Mode().Perm())
		}
	})

	t.Run("a failed promotion leaves the previous pair in place", func(t *testing.T) {
		dir := t.TempDir()
		certPath, keyPath := existingPair(t, dir)

		stagedCert := certPath + stagedSuffix
		require.NoError(t, os.WriteFile(stagedCert, []byte("new cert"), certFileMode))

		err := promoteCertificate(certPath, keyPath, stagedCert, keyPath+stagedSuffix)

		require.Error(t, err)
		assert.Equal(t, "old cert", readFile(t, certPath))
		assert.Equal(t, "old key", readFile(t, keyPath))
	})
}

func TestPanelURL(t *testing.T) {
	tests := []struct {
		name string
		host string
		port string
		want string
	}{
		{name: "the default port is left out", host: "panel.example.com", port: "443",
			want: "https://panel.example.com"},
		{name: "any other port is appended", host: "panel.example.com", port: "8443",
			want: "https://panel.example.com:8443"},
		{name: "an address is bracketed without a port", host: "2001:db8::1", port: "443",
			want: "https://[2001:db8::1]"},
		{name: "an address is bracketed with a port", host: "2001:db8::1", port: "8443",
			want: "https://[2001:db8::1]:8443"},
		{name: "the wildcard host becomes loopback", host: "0.0.0.0", port: "8443",
			want: "https://localhost:8443"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, panelURL(map[string]string{"HTTP_HOST": test.host}, test.port))
		})
	}
}

func existingPair(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()

	certPath = filepath.Join(dir, certFileName)
	keyPath = filepath.Join(dir, keyFileName)

	require.NoError(t, os.WriteFile(certPath, []byte("old cert"), certFileMode))
	require.NoError(t, os.WriteFile(keyPath, []byte("old key"), keyFileMode))

	return certPath, keyPath
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(content)
}

func dirEntries(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	return names
}

func writePair(t *testing.T, opts certgen.SelfSignedOptions) (certPath, keyPath string) {
	t.Helper()

	certPEM, keyPEM, err := certgen.GenerateSelfSigned(opts)
	require.NoError(t, err)

	dir := t.TempDir()
	certPath = filepath.Join(dir, certFileName)
	keyPath = filepath.Join(dir, keyFileName)

	require.NoError(t, os.WriteFile(certPath, certPEM, certFileMode))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, keyFileMode))

	return certPath, keyPath
}

func ipStrings(ips []net.IP) []string {
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ip.String())
	}

	return out
}

func TestProbeCertificate(t *testing.T) {
	opts := certgen.SelfSignedOptions{
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		ValidFor:    day,
	}

	certPath, keyPath := writePair(t, opts)

	expected, err := loadLeaf(certPath, keyPath)
	require.NoError(t, err)

	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	require.NoError(t, err)

	addr := serveTLS(t, pair)

	t.Run("the expected certificate passes", func(t *testing.T) {
		require.NoError(t, probeCertificate(t.Context(), addr, expected))
	})

	t.Run("another certificate names the address", func(t *testing.T) {
		otherPath, otherKeyPath := writePair(t, opts)

		other, loadErr := loadLeaf(otherPath, otherKeyPath)
		require.NoError(t, loadErr)

		probeErr := probeCertificate(t.Context(), addr, other)

		require.Error(t, probeErr)
		assert.Contains(t, probeErr.Error(), addr)
		assert.Contains(t, probeErr.Error(), "another certificate")
	})

	t.Run("a refused address is reported", func(t *testing.T) {
		probeErr := probeCertificate(t.Context(), closedAddr(t), expected)

		require.Error(t, probeErr)
		assert.Contains(t, probeErr.Error(), "cannot reach TLS server")
	})
}

// closedAddr is an address nothing listens on, taken by opening a listener and
// closing it again so that the port is known to be free.
func closedAddr(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	return addr
}

func TestPortUnavailableMessage(t *testing.T) {
	tests := []struct {
		name        string
		bind        panel.BindAddress
		scope       string
		wantParts   []string
		wantMissing []string
	}{
		{
			name:      "a bound address names the key it came from",
			bind:      panel.BindAddress{IP: "2.29.29.94", Key: panel.HTTPHostKey},
			scope:     gameap.ScopeSystem,
			wantParts: []string{"443", "2.29.29.94", panel.HTTPHostKey},
		},
		{
			name:        "a wildcard has no key to name",
			bind:        panel.BindAddress{IP: "0.0.0.0", Key: panel.HTTPHostKey},
			scope:       gameap.ScopeSystem,
			wantParts:   []string{"443", "every interface"},
			wantMissing: []string{panel.HTTPHostKey},
		},
		{
			name:      "the user scope keeps its privileged port hint",
			bind:      panel.BindAddress{Key: panel.HTTPHostKey},
			scope:     gameap.ScopeUser,
			wantParts: []string{"CAP_NET_BIND_SERVICE"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message := portUnavailableMessage(test.bind, "443", test.scope)

			for _, part := range test.wantParts {
				assert.Contains(t, message, part)
			}

			for _, part := range test.wantMissing {
				assert.NotContains(t, message, part)
			}
		})
	}
}

// serveTLS answers handshakes with pair on loopback until the test ends.
func serveTLS(t *testing.T, pair tls.Certificate) string {
	t.Helper()

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{pair},
		MinVersion:   tls.VersionTLS12,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = listener.Close()
	})

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}

			go func() {
				defer func() {
					_ = conn.Close()
				}()

				if tlsConn, ok := conn.(*tls.Conn); ok {
					_ = tlsConn.Handshake()
				}
			}()
		}
	}()

	return listener.Addr().String()
}
