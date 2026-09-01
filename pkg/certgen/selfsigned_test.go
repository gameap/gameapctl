package certgen_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"testing"
	"time"

	"github.com/gameap/gameapctl/pkg/certgen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSelfSigned(t *testing.T) {
	opts := certgen.SelfSignedOptions{
		DNSNames:    []string{"panel.example.com", "localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("192.168.1.10")},
		ValidFor:    825 * 24 * time.Hour,
	}

	certPEM, keyPEM, err := certgen.GenerateSelfSigned(opts)
	require.NoError(t, err)

	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	require.Len(t, pair.Certificate, 1)

	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	require.NoError(t, err)

	assert.Equal(t, "panel.example.com", leaf.Subject.CommonName)
	assert.Equal(t, []string{"GameAP"}, leaf.Subject.Organization)
	assert.True(t, leaf.IsCA)
	assert.Equal(t, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, leaf.ExtKeyUsage)

	for _, name := range []string{"panel.example.com", "localhost", "127.0.0.1", "192.168.1.10"} {
		assert.NoErrorf(t, leaf.VerifyHostname(name), "certificate must be valid for %s", name)
	}

	assert.Error(t, leaf.VerifyHostname("other.example.com"))

	key, ok := pair.PrivateKey.(*ecdsa.PrivateKey)
	require.True(t, ok, "private key must be ECDSA")
	assert.Equal(t, elliptic.P256(), key.Curve)
}

func TestGenerateSelfSignedValidity(t *testing.T) {
	const validFor = 30 * 24 * time.Hour

	certPEM, _, err := certgen.GenerateSelfSigned(certgen.SelfSignedOptions{
		DNSNames: []string{"localhost"},
		ValidFor: validFor,
	})
	require.NoError(t, err)

	leaf := parseCertificate(t, certPEM)

	assert.WithinDuration(t, time.Now().Add(validFor), leaf.NotAfter, time.Minute)
	assert.True(t, leaf.NotBefore.Before(time.Now()), "certificate must be backdated for clock skew")
}

func TestGenerateSelfSignedCommonName(t *testing.T) {
	tests := []struct {
		name string
		opts certgen.SelfSignedOptions
		want string
	}{
		{
			name: "explicit common name",
			opts: certgen.SelfSignedOptions{CommonName: "gameap", DNSNames: []string{"localhost"}},
			want: "gameap",
		},
		{
			name: "first dns name",
			opts: certgen.SelfSignedOptions{DNSNames: []string{"panel.example.com", "localhost"}},
			want: "panel.example.com",
		},
		{
			name: "first ip when there is no dns name",
			opts: certgen.SelfSignedOptions{IPAddresses: []net.IP{net.ParseIP("10.0.0.5")}},
			want: "10.0.0.5",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.opts.ValidFor = time.Hour

			certPEM, _, err := certgen.GenerateSelfSigned(test.opts)
			require.NoError(t, err)

			assert.Equal(t, test.want, parseCertificate(t, certPEM).Subject.CommonName)
		})
	}
}

func TestGenerateSelfSignedInvalidOptions(t *testing.T) {
	tests := []struct {
		name string
		opts certgen.SelfSignedOptions
	}{
		{
			name: "no subject alternative name",
			opts: certgen.SelfSignedOptions{ValidFor: time.Hour},
		},
		{
			name: "zero validity",
			opts: certgen.SelfSignedOptions{DNSNames: []string{"localhost"}},
		},
		{
			name: "negative validity",
			opts: certgen.SelfSignedOptions{DNSNames: []string{"localhost"}, ValidFor: -time.Hour},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := certgen.GenerateSelfSigned(test.opts)
			assert.Error(t, err)
		})
	}
}

func parseCertificate(t *testing.T, certPEM []byte) *x509.Certificate {
	t.Helper()

	block, _ := pem.Decode(certPEM)
	require.NotNil(t, block)
	assert.Equal(t, certgen.CertificatePEMType, block.Type)

	leaf, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	return leaf
}
