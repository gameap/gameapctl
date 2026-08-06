package daemon

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"time"

	"github.com/pkg/errors"
)

type TLSProbeResult struct {
	// Leaf is the server certificate captured during the handshake.
	// Nil when the server did not present one (e.g. plaintext gRPC).
	Leaf *x509.Certificate
	// HandshakeErr is a TLS failure that happened after the TCP connection
	// succeeded. Leaf may still be set: the certificate arrives before an
	// mTLS-required server aborts the handshake.
	HandshakeErr error
}

// ProbeTLSLeaf connects to addr and captures the server's leaf certificate
// without verifying it. A non-nil error means the TCP connection itself failed.
func ProbeTLSLeaf(ctx context.Context, addr string, timeout time.Duration) (TLSProbeResult, error) {
	dialer := &net.Dialer{Timeout: timeout}

	rawConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return TLSProbeResult{}, errors.Wrapf(err, "cannot reach gRPC server at %s", addr)
	}
	defer func() {
		_ = rawConn.Close()
	}()

	result := TLSProbeResult{}

	//nolint:gosec // the probe only captures the certificate; no data is exchanged
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return nil
			}

			leaf, parseErr := x509.ParseCertificate(rawCerts[0])
			if parseErr != nil {
				return nil //nolint:nilerr // unparseable certificate is treated as absent
			}
			result.Leaf = leaf

			return nil
		},
	}

	host, _, err := net.SplitHostPort(addr)
	if err == nil && net.ParseIP(host) == nil {
		tlsConfig.ServerName = host
	}

	handshakeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	tlsConn := tls.Client(rawConn, tlsConfig)
	result.HandshakeErr = tlsConn.HandshakeContext(handshakeCtx)

	return result, nil
}
