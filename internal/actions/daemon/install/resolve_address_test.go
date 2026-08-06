package install

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	daemonpkg "github.com/gameap/gameapctl/internal/pkg/daemon"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type probeOutcome struct {
	result daemonpkg.TLSProbeResult
	err    error
}

func fakeResolveDeps(
	outcomes map[string]probeOutcome, localIPs []string, out *strings.Builder, calls *[]string,
) resolveDeps {
	return resolveDeps{
		probe: func(_ context.Context, addr string, _ time.Duration) (daemonpkg.TLSProbeResult, error) {
			*calls = append(*calls, addr)
			if outcome, ok := outcomes[addr]; ok {
				return outcome.result, outcome.err
			}

			return daemonpkg.TLSProbeResult{}, errors.Errorf("cannot reach gRPC server at %s", addr)
		},
		localIPs: func() []string { return localIPs },
		printf: func(format string, a ...interface{}) {
			fmt.Fprintf(out, format, a...)
		},
	}
}

func tlsResult(leaf *x509.Certificate, handshakeErr error) daemonpkg.TLSProbeResult {
	return daemonpkg.TLSProbeResult{Leaf: leaf, HandshakeErr: handshakeErr}
}

func Test_resolveConnectAddress(t *testing.T) {
	panelCert := &x509.Certificate{
		Raw:         []byte("panel-cert"),
		IPAddresses: []net.IP{net.ParseIP("10.73.43.20"), net.ParseIP("127.0.0.1")},
		DNSNames:    []string{"localhost"},
	}
	otherCert := &x509.Certificate{
		Raw:         []byte("other-cert"),
		IPAddresses: []net.IP{net.ParseIP("10.73.43.20"), net.ParseIP("127.0.0.1")},
	}
	ipv6Cert := &x509.Certificate{
		Raw:         []byte("ipv6-cert"),
		IPAddresses: []net.IP{net.ParseIP("fd00::5")},
	}
	localIPs := []string{"10.73.43.20", "127.0.0.1", "fe80::1"}

	tests := []struct {
		name            string
		rawURL          string
		outcomes        map[string]probeOutcome
		localIPs        []string
		want            string
		wantProbeCalls  int
		wantErrIs       error
		wantErrContains string
		wantOutContains string
	}{
		{
			name:   "host covered, url unchanged",
			rawURL: "grpc://10.73.43.20:31718/key",
			outcomes: map[string]probeOutcome{
				"10.73.43.20:31718": {result: tlsResult(panelCert, nil)},
			},
			localIPs:       localIPs,
			want:           "grpc://10.73.43.20:31718/key",
			wantProbeCalls: 1,
		},
		{
			name:   "host not covered, rewritten to local san ip",
			rawURL: "grpc://158.160.157.244:31718/key",
			outcomes: map[string]probeOutcome{
				"158.160.157.244:31718": {result: tlsResult(panelCert, nil)},
				"10.73.43.20:31718":     {result: tlsResult(panelCert, nil)},
			},
			localIPs:        localIPs,
			want:            "grpc://10.73.43.20:31718/key",
			wantOutContains: "does not cover host \"158.160.157.244\"",
		},
		{
			name:   "candidate with foreign certificate skipped",
			rawURL: "grpc://158.160.157.244:31718/key",
			outcomes: map[string]probeOutcome{
				"158.160.157.244:31718": {result: tlsResult(panelCert, nil)},
				"10.73.43.20:31718":     {result: tlsResult(otherCert, nil)},
				"127.0.0.1:31718":       {result: tlsResult(panelCert, nil)},
			},
			localIPs: localIPs,
			want:     "grpc://127.0.0.1:31718/key",
		},
		{
			name:   "no working alternative, install aborted",
			rawURL: "grpc://158.160.157.244:31718/key",
			outcomes: map[string]probeOutcome{
				"158.160.157.244:31718": {result: tlsResult(panelCert, nil)},
			},
			localIPs:        localIPs,
			wantErrIs:       errConnectHostNotCovered,
			wantErrContains: "GRPC_EXTERNAL_HOST",
		},
		{
			name:   "plaintext grpc, url unchanged",
			rawURL: "grpc://158.160.157.244:31718/key",
			outcomes: map[string]probeOutcome{
				"158.160.157.244:31718": {result: tlsResult(nil, errors.New("handshake failed"))},
			},
			localIPs:       localIPs,
			want:           "grpc://158.160.157.244:31718/key",
			wantProbeCalls: 1,
		},
		{
			name:   "mtls handshake error with covered host, url unchanged",
			rawURL: "grpc://10.73.43.20:31718/key",
			outcomes: map[string]probeOutcome{
				"10.73.43.20:31718": {result: tlsResult(panelCert, errors.New("tls: certificate required"))},
			},
			localIPs:       localIPs,
			want:           "grpc://10.73.43.20:31718/key",
			wantProbeCalls: 1,
		},
		{
			name:   "unreachable host, panel found locally",
			rawURL: "grpc://158.160.157.244:31718/key",
			outcomes: map[string]probeOutcome{
				"10.73.43.20:31718": {result: tlsResult(panelCert, nil)},
			},
			localIPs:        localIPs,
			want:            "grpc://10.73.43.20:31718/key",
			wantOutContains: "unreachable at 158.160.157.244:31718",
		},
		{
			name:            "unreachable host, no local panel",
			rawURL:          "grpc://158.160.157.244:31718/key",
			outcomes:        map[string]probeOutcome{},
			localIPs:        localIPs,
			wantErrContains: "cannot reach gRPC server at 158.160.157.244:31718",
		},
		{
			name:   "ipv6 candidate produces bracketed url",
			rawURL: "grpc://158.160.157.244:31718/key",
			outcomes: map[string]probeOutcome{
				"158.160.157.244:31718": {result: tlsResult(ipv6Cert, nil)},
				"[fd00::5]:31718":       {result: tlsResult(ipv6Cert, nil)},
			},
			localIPs: []string{"fd00::5"},
			want:     "grpc://[fd00::5]:31718/key",
		},
		{
			name:            "invalid url",
			rawURL:          "http://example.com/key",
			outcomes:        map[string]probeOutcome{},
			localIPs:        localIPs,
			wantErrContains: "invalid connect URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			var calls []string
			deps := fakeResolveDeps(tt.outcomes, tt.localIPs, &out, &calls)

			got, err := resolveConnectAddress(context.Background(), deps, tt.rawURL)

			if tt.wantErrIs != nil || tt.wantErrContains != "" {
				require.Error(t, err)
				if tt.wantErrIs != nil {
					assert.ErrorIs(t, err, tt.wantErrIs)
				}
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			if tt.wantProbeCalls > 0 {
				require.Len(t, calls, tt.wantProbeCalls)
			}
			if tt.wantOutContains != "" {
				assert.Contains(t, out.String(), tt.wantOutContains)
			}
		})
	}
}

func Test_candidateHostsFromCert(t *testing.T) {
	cert := &x509.Certificate{
		IPAddresses: []net.IP{
			net.ParseIP("192.168.1.5"),
			net.ParseIP("10.0.0.7"),
			net.ParseIP("127.0.0.1"),
			net.ParseIP("169.254.1.1"),
			net.ParseIP("0.0.0.0"),
			net.ParseIP("203.0.113.8"),
		},
		DNSNames: []string{"*.wild.example.com", "panel.example.com", "localhost"},
	}
	localIPs := []string{"10.0.0.7", "192.168.1.5", "127.0.0.1"}

	tests := []struct {
		name     string
		origHost string
		want     []string
	}{
		{
			name:     "local san ips by weight, loopback, dns, localhost last",
			origHost: "158.160.157.244",
			want:     []string{"192.168.1.5", "10.0.0.7", "127.0.0.1", "panel.example.com", "localhost"},
		},
		{
			name:     "orig host excluded",
			origHost: "10.0.0.7",
			want:     []string{"192.168.1.5", "127.0.0.1", "panel.example.com", "localhost"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := candidateHostsFromCert(cert, localIPs, tt.origHost)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_localProbeCandidates(t *testing.T) {
	localIPs := []string{"127.0.0.1", "10.0.0.7", "192.168.1.5", "fe80::1", "not-an-ip"}

	tests := []struct {
		name     string
		origHost string
		want     []string
	}{
		{
			name:     "non loopback by weight then loopback",
			origHost: "158.160.157.244",
			want:     []string{"192.168.1.5", "10.0.0.7", "127.0.0.1"},
		},
		{
			name:     "orig host excluded",
			origHost: "10.0.0.7",
			want:     []string{"192.168.1.5", "127.0.0.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := localProbeCandidates(localIPs, tt.origHost)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_certCoversHost(t *testing.T) {
	ipCert := &x509.Certificate{
		IPAddresses: []net.IP{net.ParseIP("10.73.43.20"), net.ParseIP("127.0.0.1")},
		DNSNames:    []string{"localhost"},
	}
	wildcardCert := &x509.Certificate{
		DNSNames: []string{"*.example.com"},
	}

	tests := []struct {
		name string
		cert *x509.Certificate
		host string
		want bool
	}{
		{name: "san ip match", cert: ipCert, host: "10.73.43.20", want: true},
		{name: "san ip miss", cert: ipCert, host: "158.160.157.244", want: false},
		{name: "dns match", cert: ipCert, host: "localhost", want: true},
		{name: "wildcard match", cert: wildcardCert, host: "foo.example.com", want: true},
		{name: "wildcard does not cover apex", cert: wildcardCert, host: "example.com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, certCoversHost(tt.cert, tt.host))
		})
	}
}
