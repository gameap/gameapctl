package install

import (
	"bytes"
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/gameap/gameapctl/pkg/tlsprobe"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	primaryProbeTimeout   = 5 * time.Second
	candidateProbeTimeout = 3 * time.Second
)

type resolveDeps struct {
	probe    func(ctx context.Context, addr string, timeout time.Duration) (tlsprobe.Result, error)
	localIPs func() []string
	printf   func(format string, a ...interface{})
}

func defaultResolveDeps() resolveDeps {
	return resolveDeps{
		probe:    tlsprobe.Leaf,
		localIPs: utils.DetectIPs,
		printf: func(format string, a ...interface{}) {
			fmt.Printf(format, a...)
		},
	}
}

// resolveConnectAddress checks that the connect URL points at an address covered
// by the panel's gRPC TLS certificate. The panel builds the URL from the HTTP
// request host, so on a NAT'ed machine it may carry the public address while the
// certificate only covers local ones; the daemon would then enroll successfully
// but fail TLS verification on every subsequent connection. In that case the
// host is rewritten to a certificate-covered address of this machine.
func resolveConnectAddress(ctx context.Context, deps resolveDeps, rawURL string) (string, error) {
	info, err := ParseConnectURL(rawURL)
	if err != nil {
		return "", errors.WithMessage(err, "invalid connect URL")
	}

	deps.printf("Checking gRPC connection to %s ...\n", info.Address())

	result, err := deps.probe(ctx, info.Address(), primaryProbeTimeout)
	if err != nil {
		suggestLocalPanel(ctx, deps, info)

		return "", err
	}

	if result.Leaf == nil {
		return rawURL, nil
	}

	if certCoversHost(result.Leaf, info.Host) {
		return rawURL, nil
	}

	return rewriteToCoveredCandidate(ctx, deps, info, result.Leaf)
}

func rewriteToCoveredCandidate(
	ctx context.Context, deps resolveDeps, info ConnectInfo, origLeaf *x509.Certificate,
) (string, error) {
	candidates := candidateHostsFromCert(origLeaf, deps.localIPs(), info.Host)

	for _, candidate := range candidates {
		candInfo := ConnectInfo{Host: candidate, Port: info.Port, SetupKey: info.SetupKey}

		res, err := deps.probe(ctx, candInfo.Address(), candidateProbeTimeout)
		if err != nil || res.Leaf == nil {
			continue
		}

		if !sameCertificate(res.Leaf, origLeaf) || !certCoversHost(res.Leaf, candidate) {
			continue
		}

		deps.printf(
			"WARNING: the panel gRPC certificate does not cover host %q.\n"+
				"Certificate covers: IPs %v, DNS %v.\n"+
				"This usually means the panel runs on this machine behind NAT and the\n"+
				"connect URL contains the public address.\n"+
				"Panel found at %s with the same certificate; using %s instead.\n"+
				"Hint: to keep the public address, set GRPC_EXTERNAL_HOST in the panel\n"+
				"config.env and restart the panel (its gRPC certificate is regenerated automatically).\n",
			info.Host, certIPStrings(origLeaf), origLeaf.DNSNames, candidate, candInfo.URL(),
		)

		return candInfo.URL(), nil
	}

	return "", errors.WithMessagef(errConnectHostNotCovered,
		"host %q is not in the panel gRPC certificate (IPs: %v, DNS: %v) "+
			"and no working alternative address was found (tried: %s); "+
			"set GRPC_EXTERNAL_HOST=%s in the panel config.env and restart the panel "+
			"(its gRPC certificate is regenerated automatically), "+
			"or use a --connect address covered by the certificate",
		info.Host, certIPStrings(origLeaf), origLeaf.DNSNames,
		strings.Join(candidates, ", "), info.Host,
	)
}

// suggestLocalPanel handles the case when the connect host does not answer at
// all, e.g. the panel is on this same machine behind a NAT without hairpin
// support. With the original host down there is no trusted certificate to
// anchor a rewrite to (the probe does not verify what a local listener
// presents), so a discovered panel is only suggested, never used
// automatically: the operator confirms it by re-running the installation with
// the printed address.
func suggestLocalPanel(ctx context.Context, deps resolveDeps, info ConnectInfo) {
	for _, candidate := range localProbeCandidates(deps.localIPs(), info.Host) {
		candInfo := ConnectInfo{Host: candidate, Port: info.Port, SetupKey: info.SetupKey}

		res, err := deps.probe(ctx, candInfo.Address(), candidateProbeTimeout)
		if err != nil || res.Leaf == nil {
			continue
		}

		if !certCoversHost(res.Leaf, candidate) {
			continue
		}

		deps.printf(
			"WARNING: the panel is unreachable at %s, but a gRPC server with a certificate\n"+
				"covering %s answers on this machine. If the panel runs here (e.g. behind a\n"+
				"NAT without hairpin support), re-run the installation with:\n"+
				"  gameapctl daemon install --connect=%s\n",
			info.Address(), candInfo.Address(), candInfo.URL(),
		)

		return
	}
}

func certCoversHost(cert *x509.Certificate, host string) bool {
	return cert.VerifyHostname(host) == nil
}

func sameCertificate(a, b *x509.Certificate) bool {
	return bytes.Equal(a.Raw, b.Raw)
}

// candidateHostsFromCert returns hosts from the certificate SANs worth dialing
// instead of the original host, best first: certificate IPs assigned to a local
// interface, then loopback IPs, then DNS names ("localhost" last). SAN IPs not
// present on this machine are skipped — they belong to another network anyway.
func candidateHostsFromCert(cert *x509.Certificate, localIPs []string, origHost string) []string {
	local := make(map[string]struct{}, len(localIPs))
	for _, ip := range localIPs {
		if parsed := net.ParseIP(ip); parsed != nil {
			local[parsed.String()] = struct{}{}
		}
	}

	origNormalized := normalizeHost(origHost)

	var localSAN, loopbackSAN []string
	for _, sanIP := range cert.IPAddresses {
		normalized := sanIP.String()
		if normalized == origNormalized || sanIP.IsUnspecified() || sanIP.IsLinkLocalUnicast() {
			continue
		}

		if sanIP.IsLoopback() {
			loopbackSAN = append(loopbackSAN, normalized)

			continue
		}

		if _, ok := local[normalized]; ok {
			localSAN = append(localSAN, normalized)
		}
	}

	dnsSAN := make([]string, 0, len(cert.DNSNames))
	hasLocalhost := false
	for _, name := range cert.DNSNames {
		if strings.Contains(name, "*") || strings.EqualFold(name, origHost) {
			continue
		}

		if strings.EqualFold(name, "localhost") {
			hasLocalhost = true

			continue
		}

		dnsSAN = append(dnsSAN, name)
	}

	candidates := make([]string, 0, len(localSAN)+len(loopbackSAN)+len(dnsSAN)+1)
	candidates = append(candidates, sortIPsByPreference(localSAN)...)
	candidates = append(candidates, loopbackSAN...)
	candidates = append(candidates, dnsSAN...)
	if hasLocalhost {
		candidates = append(candidates, "localhost")
	}

	return dedupStrings(candidates)
}

// localProbeCandidates returns addresses of this machine to look for a locally
// running panel at: non-loopback interface IPs best first, loopback last.
func localProbeCandidates(localIPs []string, origHost string) []string {
	origNormalized := normalizeHost(origHost)

	candidates := make([]string, 0, len(localIPs)+1)
	for _, ip := range localIPs {
		parsed := net.ParseIP(ip)
		if parsed == nil || parsed.IsLoopback() || parsed.IsUnspecified() || parsed.IsLinkLocalUnicast() {
			continue
		}

		normalized := parsed.String()
		if normalized == origNormalized {
			continue
		}

		candidates = append(candidates, normalized)
	}

	candidates = sortIPsByPreference(candidates)
	candidates = append(candidates, "127.0.0.1")

	return dedupStrings(candidates)
}

func sortIPsByPreference(ips []string) []string {
	sorted := make([]string, len(ips))
	copy(sorted, ips)
	sort.SliceStable(sorted, func(i, j int) bool {
		return ipPreferenceWeight(sorted[i]) > ipPreferenceWeight(sorted[j])
	})

	return sorted
}

func normalizeHost(host string) string {
	if parsed := net.ParseIP(host); parsed != nil {
		return parsed.String()
	}

	return host
}

func certIPStrings(cert *x509.Certificate) []string {
	out := make([]string, 0, len(cert.IPAddresses))
	for _, ip := range cert.IPAddresses {
		out = append(out, ip.String())
	}

	return out
}

func dedupStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	return out
}
