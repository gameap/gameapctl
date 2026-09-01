// Package https configures the HTTPS listener built into the panel: it issues
// the self-signed certificate an installation without a public domain needs,
// points config.env at a certificate pair and reports what is in effect.
package https

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gameap/gameapctl/pkg/certgen"
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/panel"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	certDirName  = "certs"
	certFileName = "panel.crt"
	keyFileName  = "panel.key"

	// stagedSuffix and backupSuffix name the two intermediate states a
	// certificate pair passes through while it is being replaced.
	stagedSuffix = ".new"
	backupSuffix = ".old"

	certFileMode = 0644
	keyFileMode  = 0600
	certDirMode  = 0755

	day                 = 24 * time.Hour
	defaultValidityDays = 825

	// renewBefore is how close to expiry an existing certificate has to be for
	// enable to replace it. Reissuing earlier would throw away the browser
	// exception the operator has already accepted for no reason.
	renewBefore = 30 * day

	// userScopeHTTPSPort keeps the rootless installation off a privileged port:
	// a systemd user unit cannot be granted CAP_NET_BIND_SERVICE.
	userScopeHTTPSPort = "8443"

	maxPort = 65535

	verifyTimeout  = 20 * time.Second
	verifyInterval = time.Second
	healthInterval = 3 * time.Second
	probeTimeout   = 3 * time.Second

	// rollbackTimeout bounds the restart that puts the previous configuration
	// back, which runs on a context of its own.
	rollbackTimeout = 2 * time.Minute

	localhostName = "localhost"
	wildcardHost  = "0.0.0.0"
)

func certDir(paths gameap.PanelPaths) string {
	return filepath.Join(paths.ConfigDir, certDirName)
}

func certPaths(paths gameap.PanelPaths) (certPath, keyPath string) {
	dir := certDir(paths)

	return filepath.Join(dir, certFileName), filepath.Join(dir, keyFileName)
}

// sanInput carries everything the subject alternative names are derived from,
// so that the derivation itself stays free of the CLI and the file system.
type sanInput struct {
	httpHost    string
	hostname    string
	detectedIPs []string
	domains     []string
	ips         []string
}

// buildSelfSignedOptions decides which names the certificate has to cover.
// Explicit --domain/--ip values replace what would otherwise be detected;
// loopback is always included so that a check from the panel host itself works.
func buildSelfSignedOptions(in sanInput, validFor time.Duration) (certgen.SelfSignedOptions, error) {
	names := newNameSet()

	for _, value := range in.ips {
		if net.ParseIP(strings.TrimSpace(value)) == nil {
			return certgen.SelfSignedOptions{}, errors.Errorf("invalid IP address %q", value)
		}
	}

	explicit := append(append([]string{}, in.domains...), in.ips...)

	if len(explicit) > 0 {
		names.addAll(explicit)
	} else {
		names.add(in.httpHost)
		names.add(in.hostname)
		names.addAll(in.detectedIPs)
	}

	primary := names.primary()

	names.add(localhostName)
	names.add("127.0.0.1")
	names.add("::1")

	dnsNames, ipAddresses := names.split()
	if len(dnsNames) == 0 && len(ipAddresses) == 0 {
		return certgen.SelfSignedOptions{}, errors.New("no host names to issue a certificate for")
	}

	return certgen.SelfSignedOptions{
		CommonName:  primary,
		DNSNames:    dnsNames,
		IPAddresses: ipAddresses,
		ValidFor:    validFor,
	}, nil
}

// needsRegeneration reports whether the certificate on disk still serves the
// requested names for long enough, together with the reason it does not.
func needsRegeneration(certPath, keyPath string, opts certgen.SelfSignedOptions, now time.Time) (bool, string) {
	if !utils.IsFileExists(certPath) || !utils.IsFileExists(keyPath) {
		return true, "no certificate found"
	}

	leaf, err := loadLeaf(certPath, keyPath)
	if err != nil {
		return true, "the existing certificate and key are not a usable pair"
	}

	if now.Add(renewBefore).After(leaf.NotAfter) {
		return true, fmt.Sprintf("the existing certificate expires on %s", leaf.NotAfter.Format(time.DateOnly))
	}

	for _, name := range opts.DNSNames {
		if leaf.VerifyHostname(name) != nil {
			return true, fmt.Sprintf("the existing certificate does not cover %s", name)
		}
	}

	for _, ip := range opts.IPAddresses {
		if leaf.VerifyHostname(ip.String()) != nil {
			return true, fmt.Sprintf("the existing certificate does not cover %s", ip)
		}
	}

	return false, ""
}

// resolveHTTPSPort picks the port the HTTPS listener binds: an explicit flag
// first, then whatever the panel is already configured with, then the default
// for the scope.
func resolveHTTPSPort(flagPort int, values map[string]string, scope string) (string, error) {
	if flagPort != 0 {
		if flagPort < 1 || flagPort > maxPort {
			return "", errors.Errorf("port %d is out of range", flagPort)
		}

		return strconv.Itoa(flagPort), nil
	}

	if port := panel.ConfigValue(values, panel.HTTPSPortKey); port != "" {
		return port, nil
	}

	if scope == gameap.ScopeUser {
		return userScopeHTTPSPort, nil
	}

	return panel.DefaultHTTPSPort, nil
}

func loadLeaf(certPath, keyPath string) (*x509.Certificate, error) {
	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load the certificate and key")
	}

	if len(pair.Certificate) == 0 {
		return nil, errors.New("certificate file contains no certificate")
	}

	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse the certificate")
	}

	return leaf, nil
}

// fingerprint identifies a certificate in log output and lets enable tell the
// certificate it has just issued from the one an old process is still serving.
func fingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	encoded := strings.ToUpper(hex.EncodeToString(sum[:]))

	parts := make([]string, 0, len(sum))
	for i := 0; i < len(encoded); i += 2 {
		parts = append(parts, encoded[i:i+2])
	}

	return strings.Join(parts, ":")
}

// writeCertificate stages the new pair beside the old one and moves both files
// into place only once both are written. The panel exits when the configured
// certificate and key do not load as a pair, so a replacement that fails halfway
// through would take the installation down with it.
func writeCertificate(certPath, keyPath string, certPEM, keyPEM []byte) error {
	if err := os.MkdirAll(filepath.Dir(certPath), certDirMode); err != nil {
		return errors.Wrap(err, "failed to create the certificate directory")
	}

	stagedCert, stagedKey := certPath+stagedSuffix, keyPath+stagedSuffix

	defer func() {
		removeQuietly(stagedCert)
		removeQuietly(stagedKey)
	}()

	if err := writeStaged(stagedCert, certPEM, certFileMode); err != nil {
		return errors.Wrap(err, "failed to write the certificate")
	}

	if err := writeStaged(stagedKey, keyPEM, keyFileMode); err != nil {
		return errors.Wrap(err, "failed to write the private key")
	}

	return promoteCertificate(certPath, keyPath, stagedCert, stagedKey)
}

// promoteCertificate swaps the staged pair in, putting the pair that was there
// back when either move fails.
func promoteCertificate(certPath, keyPath, stagedCert, stagedKey string) error {
	certBackup, err := moveAside(certPath)
	if err != nil {
		return err
	}

	keyBackup, err := moveAside(keyPath)
	if err != nil {
		restore(certBackup, certPath)

		return err
	}

	if err = os.Rename(stagedCert, certPath); err != nil {
		restore(certBackup, certPath)
		restore(keyBackup, keyPath)

		return errors.Wrap(err, "failed to replace the certificate")
	}

	if err = os.Rename(stagedKey, keyPath); err != nil {
		restore(certBackup, certPath)
		restore(keyBackup, keyPath)

		return errors.Wrap(err, "failed to replace the private key")
	}

	removeQuietly(certBackup)
	removeQuietly(keyBackup)

	return nil
}

// moveAside renames path out of the way, reporting where it went or an empty
// string when there was nothing there to keep.
func moveAside(path string) (string, error) {
	if !utils.IsFileExists(path) {
		return "", nil
	}

	backup := path + backupSuffix
	if err := os.Rename(path, backup); err != nil {
		return "", errors.Wrapf(err, "failed to move %s aside", path)
	}

	return backup, nil
}

// restore puts a file that was moved aside back, or clears what took its place
// when there was no file to begin with.
func restore(backup, path string) {
	if backup == "" {
		removeQuietly(path)

		return
	}

	if err := os.Rename(backup, path); err != nil {
		log.Printf("Failed to restore %s from %s: %v\n", path, backup, err)
	}
}

// writeStaged drops a leftover from an interrupted run first, so that the file
// is created with the mode asked for rather than keeping the one it had.
func writeStaged(path string, data []byte, mode os.FileMode) error {
	removeQuietly(path)

	return os.WriteFile(path, data, mode)
}

func removeQuietly(path string) {
	if path == "" {
		return
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("Failed to remove %s: %v\n", path, err)
	}
}

func certificateNames(cert *x509.Certificate) string {
	names := make([]string, 0, len(cert.DNSNames)+len(cert.IPAddresses))
	names = append(names, cert.DNSNames...)

	for _, ip := range cert.IPAddresses {
		names = append(names, ip.String())
	}

	if len(names) == 0 {
		return "-"
	}

	return strings.Join(names, ", ")
}

// nameSet collects host names in first-seen order, dropping duplicates and the
// placeholders that name no host at all.
type nameSet struct {
	seen  map[string]struct{}
	order []string
}

func newNameSet() *nameSet {
	return &nameSet{seen: map[string]struct{}{}}
}

func (s *nameSet) add(value string) {
	value = strings.TrimSpace(value)
	if value == "" || value == wildcardHost || value == "::" {
		return
	}

	if _, ok := s.seen[value]; ok {
		return
	}

	s.seen[value] = struct{}{}
	s.order = append(s.order, value)
}

func (s *nameSet) addAll(values []string) {
	for _, value := range values {
		s.add(value)
	}
}

func (s *nameSet) primary() string {
	if len(s.order) == 0 {
		return localhostName
	}

	return s.order[0]
}

func (s *nameSet) split() (dnsNames []string, ipAddresses []net.IP) {
	for _, value := range s.order {
		if ip := net.ParseIP(value); ip != nil {
			ipAddresses = append(ipAddresses, ip)

			continue
		}

		dnsNames = append(dnsNames, value)
	}

	sort.Strings(dnsNames)
	sort.Slice(ipAddresses, func(i, j int) bool {
		return ipAddresses[i].String() < ipAddresses[j].String()
	})

	return dnsNames, ipAddresses
}

// waitFor retries probe every interval until it succeeds or verifyTimeout
// elapses, reporting the failure of the last attempt.
func waitFor(ctx context.Context, interval time.Duration, probe func() error) error {
	deadline := time.Now().Add(verifyTimeout)

	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return errors.Wrap(ctx.Err(), "interrupted while waiting for the panel")
			case <-time.After(interval):
			}
		}

		err := probe()
		if err == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return err
		}
	}
}

// expirySummary renders a certificate's expiry the way an operator reads it:
// the date, and how far away it is.
func expirySummary(notAfter, now time.Time) string {
	date := notAfter.Format(time.DateOnly)

	if now.After(notAfter) {
		return fmt.Sprintf("%s (expired %d days ago)", date, int(now.Sub(notAfter)/day))
	}

	return fmt.Sprintf("%s (in %d days)", date, int(notAfter.Sub(now)/day))
}
