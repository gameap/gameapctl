// Package certgen generates the key material gameapctl has to produce itself,
// currently the self-signed certificate the panel serves HTTPS with while no
// certificate from a certificate authority is available.
package certgen

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"

	"github.com/pkg/errors"
)

const (
	// CertificatePEMType and PrivateKeyPEMType are the PEM block types of the
	// pair GenerateSelfSigned returns.
	CertificatePEMType = "CERTIFICATE"
	PrivateKeyPEMType  = "EC PRIVATE KEY"

	organization = "GameAP"

	// serialNumberBits sizes the random serial. 128 bits is what the CA/Browser
	// Forum requires of a public CA, and costs nothing to match here.
	serialNumberBits = 128

	// backdate absorbs the clock skew between the panel host and a client that
	// would otherwise see a certificate from the future.
	backdate = time.Hour
)

// SelfSignedOptions describes the certificate to issue. At least one DNS name
// or IP address is required: a certificate with no subject alternative name
// matches no host and is rejected by every current client.
type SelfSignedOptions struct {
	CommonName  string
	DNSNames    []string
	IPAddresses []net.IP
	ValidFor    time.Duration
}

// GenerateSelfSigned issues a self-signed ECDSA P-256 certificate for opts and
// returns it PEM-encoded along with its private key.
//
// The certificate is marked as a CA. It is its own trust anchor, so an operator
// importing it into a system trust store needs it to pass basic-constraints
// validation; this is the same shape `openssl req -x509` produces by default.
func GenerateSelfSigned(opts SelfSignedOptions) (certPEM, keyPEM []byte, err error) {
	if len(opts.DNSNames) == 0 && len(opts.IPAddresses) == 0 {
		return nil, nil, errors.New("certificate needs at least one DNS name or IP address")
	}

	if opts.ValidFor <= 0 {
		return nil, nil, errors.New("certificate validity must be positive")
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate private key")
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), serialNumberBits))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate serial number")
	}

	now := time.Now()

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   commonName(opts),
			Organization: []string{organization},
		},
		NotBefore:             now.Add(-backdate),
		NotAfter:              now.Add(opts.ValidFor),
		DNSNames:              opts.DNSNames,
		IPAddresses:           opts.IPAddresses,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create certificate")
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to marshal private key")
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: CertificatePEMType, Bytes: certDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: PrivateKeyPEMType, Bytes: keyDER})

	return certPEM, keyPEM, nil
}

func commonName(opts SelfSignedOptions) string {
	if opts.CommonName != "" {
		return opts.CommonName
	}

	if len(opts.DNSNames) > 0 {
		return opts.DNSNames[0]
	}

	return opts.IPAddresses[0].String()
}
