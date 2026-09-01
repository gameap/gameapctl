package tlsprobe

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestCert(t *testing.T) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-panel"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
	}
}

func runTLSServer(t *testing.T, cfg *tls.Config) net.Listener {
	t.Helper()

	listener, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
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

			go func(c net.Conn) {
				defer c.Close()
				if tlsConn, ok := c.(*tls.Conn); ok {
					_ = tlsConn.Handshake()
				}
			}(conn)
		}
	}()

	return listener
}

func TestLeaf_tlsServer(t *testing.T) {
	cert := generateTestCert(t)
	listener := runTLSServer(t, &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})

	result, err := Leaf(context.Background(), listener.Addr().String(), 3*time.Second)

	require.NoError(t, err)
	require.NotNil(t, result.Leaf)
	assert.NoError(t, result.HandshakeErr)
	assert.Equal(t, "test-panel", result.Leaf.Subject.CommonName)
}

func TestLeaf_mtlsRequired(t *testing.T) {
	cert := generateTestCert(t)

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)

	listener := runTLSServer(t, &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		MinVersion:   tls.VersionTLS12,
		MaxVersion:   tls.VersionTLS12,
	})

	result, err := Leaf(context.Background(), listener.Addr().String(), 3*time.Second)

	require.NoError(t, err)
	require.NotNil(t, result.Leaf)
	assert.Error(t, result.HandshakeErr)
	assert.Equal(t, "test-panel", result.Leaf.Subject.CommonName)
}

func TestLeaf_unreachable(t *testing.T) {
	_, err := Leaf(context.Background(), "127.0.0.1:1", 1*time.Second)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot reach TLS server")
}

func TestLeaf_plainTCP(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
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
			_ = conn.Close()
		}
	}()

	result, err := Leaf(context.Background(), listener.Addr().String(), 1*time.Second)

	require.NoError(t, err)
	assert.Nil(t, result.Leaf)
	assert.Error(t, result.HandshakeErr)
}
