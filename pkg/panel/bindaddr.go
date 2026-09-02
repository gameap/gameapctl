package panel

import (
	"context"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	wildcardIPv4 = "0.0.0.0"
	loopbackIPv4 = "127.0.0.1"
	loopbackIPv6 = "::1"

	// bindResolveTimeout matches the timeout the panel gives its own lookup. A
	// shorter one would report a bind on every interface where the panel got an
	// answer in time and bound a single address, which is the mistake this file
	// exists to avoid.
	bindResolveTimeout = 10 * time.Second
)

// BindAddress is where the panel's HTTP and HTTPS listeners land, derived from
// config.env the way the panel derives it at start.
type BindAddress struct {
	// IP is what the panel passes to net.JoinHostPort. Empty or unspecified
	// means every interface.
	IP string

	// Key names the config.env key IP was derived from, so that a message about
	// an address that does not work can point at what has to change.
	Key string
}

// ResolveBindAddress repeats the panel's own derivation: HTTP_BIND_IP is taken
// as it is, and HTTP_HOST is resolved only when it is not already an address.
func ResolveBindAddress(ctx context.Context, values map[string]string) BindAddress {
	return defaultBindResolver().resolve(ctx, values)
}

// Wildcard reports whether the panel listens on every interface, which is what
// an empty or unspecified address means to net.Listen.
func (a BindAddress) Wildcard() bool {
	if a.IP == "" {
		return true
	}

	ip := net.ParseIP(a.IP)

	return ip != nil && ip.IsUnspecified()
}

// ListenAddr is the address to bind when testing whether a port is free, which
// is the empty string for every interface.
func (a BindAddress) ListenAddr() string {
	if a.Wildcard() {
		return ""
	}

	return a.IP
}

// ProbeHosts lists the addresses the listeners answer on from the panel's own
// machine, the one it binds first. Loopback follows it because this derivation
// can disagree with the panel that is actually running: the name in HTTP_HOST
// may resolve differently now than it did when the panel started.
func (a BindAddress) ProbeHosts() []string {
	hosts := make([]string, 0, 2)

	if !a.Wildcard() {
		hosts = append(hosts, a.IP)
	}

	for _, fallback := range loopbackFor(a.IP) {
		if !slices.Contains(hosts, fallback) {
			hosts = append(hosts, fallback)
		}
	}

	return hosts
}

// ProbeAddrs is ProbeHosts with a port on every entry.
func (a BindAddress) ProbeAddrs(port string) []string {
	hosts := a.ProbeHosts()

	addrs := make([]string, 0, len(hosts))
	for _, host := range hosts {
		addrs = append(addrs, net.JoinHostPort(host, port))
	}

	return addrs
}

func (a BindAddress) String() string {
	if a.Wildcard() {
		return "every interface"
	}

	return a.IP
}

// loopbackFor keeps the fallback in the address family of the bind address, so
// that an installation reachable over IPv6 only is still probed.
func loopbackFor(bind string) []string {
	ip := net.ParseIP(bind)
	if ip == nil || ip.To4() != nil {
		return []string{loopbackIPv4}
	}

	if ip.IsUnspecified() {
		return []string{loopbackIPv6, loopbackIPv4}
	}

	return []string{loopbackIPv6}
}

// ProbeEach tries every address in order and gives up only once none of them
// answers, reporting what each one said. The panel binds a single address and
// which one that is depends on config.env, so a refusal from one of them says
// nothing on its own.
func ProbeEach(addrs []string, probe func(addr string) error) error {
	if len(addrs) == 0 {
		return errors.New("no address to probe the panel at")
	}

	var first error

	messages := make([]string, 0, len(addrs))

	for _, addr := range addrs {
		err := probe(addr)
		if err == nil {
			return nil
		}

		if first == nil {
			first = err
		}

		messages = append(messages, err.Error())
	}

	if len(messages) == 1 {
		return first
	}

	return errors.New(strings.Join(messages, "; "))
}

// bindResolver holds the two lookups the derivation needs, so that the tests
// depend on neither DNS nor the interfaces of the machine they run on.
type bindResolver struct {
	lookupHost func(ctx context.Context, host string) ([]string, error)
	localIPs   func() []string
}

func defaultBindResolver() bindResolver {
	return bindResolver{
		lookupHost: net.DefaultResolver.LookupHost,
		localIPs:   utils.DetectIPs,
	}
}

func (r bindResolver) resolve(ctx context.Context, values map[string]string) BindAddress {
	if bindIP := ConfigValue(values, HTTPBindIPKey); bindIP != "" {
		// The panel passes this key straight to net.JoinHostPort without
		// parsing it, so a zoned literal such as fe80::1%eth0 survives.
		return BindAddress{IP: bindIP, Key: HTTPBindIPKey}
	}

	host := ConfigValue(values, HTTPHostKey)

	// An empty host and the two wildcards the panel special-cases all end up
	// here: "0.0.0.0" and "::" parse as addresses like any other.
	if host == "" || net.ParseIP(host) != nil {
		return BindAddress{IP: host, Key: HTTPHostKey}
	}

	return BindAddress{IP: r.resolveDomain(ctx, host), Key: HTTPHostKey}
}

// resolveDomain returns the resolved address the panel would bind, which is the
// first one that belongs to this machine. Everything else leaves the panel on
// every interface: it cannot bind an address it does not have.
func (r bindResolver) resolveDomain(ctx context.Context, host string) string {
	ctx, cancel := context.WithTimeout(ctx, bindResolveTimeout)
	defer cancel()

	resolved, err := r.lookupHost(ctx, host)
	if err != nil {
		return wildcardIPv4
	}

	local := r.localIPs()

	for _, addr := range resolved {
		if slices.Contains(local, addr) {
			return addr
		}
	}

	return wildcardIPv4
}
