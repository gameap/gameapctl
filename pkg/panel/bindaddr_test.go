package panel

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveBindAddress(t *testing.T) {
	const domain = "panel.example.com"

	tests := []struct {
		name           string
		values         map[string]string
		resolved       []string
		resolveErr     error
		localIPs       []string
		wantIP         string
		wantKey        string
		wantListenAddr string
		wantProbeHosts []string
		wantLookups    []string
	}{
		{
			name:           "nothing configured",
			values:         map[string]string{},
			wantIP:         "",
			wantKey:        HTTPHostKey,
			wantListenAddr: "",
			wantProbeHosts: []string{"127.0.0.1"},
		},
		{
			name:           "ipv4 wildcard",
			values:         map[string]string{HTTPHostKey: "0.0.0.0"},
			wantIP:         "0.0.0.0",
			wantKey:        HTTPHostKey,
			wantListenAddr: "",
			wantProbeHosts: []string{"127.0.0.1"},
		},
		{
			name:           "ipv6 wildcard",
			values:         map[string]string{HTTPHostKey: "::"},
			wantIP:         "::",
			wantKey:        HTTPHostKey,
			wantListenAddr: "",
			wantProbeHosts: []string{"::1", "127.0.0.1"},
		},
		{
			name:           "public ipv4 literal",
			values:         map[string]string{HTTPHostKey: "2.29.29.94"},
			wantIP:         "2.29.29.94",
			wantKey:        HTTPHostKey,
			wantListenAddr: "2.29.29.94",
			wantProbeHosts: []string{"2.29.29.94", "127.0.0.1"},
		},
		{
			name:           "ipv6 literal",
			values:         map[string]string{HTTPHostKey: "2001:db8::1"},
			wantIP:         "2001:db8::1",
			wantKey:        HTTPHostKey,
			wantListenAddr: "2001:db8::1",
			wantProbeHosts: []string{"2001:db8::1", "::1"},
		},
		{
			name:           "quoted and padded value",
			values:         map[string]string{HTTPHostKey: `  "2.29.29.94"  `},
			wantIP:         "2.29.29.94",
			wantKey:        HTTPHostKey,
			wantListenAddr: "2.29.29.94",
			wantProbeHosts: []string{"2.29.29.94", "127.0.0.1"},
		},
		{
			name:           "bind ip wins over a domain host",
			values:         map[string]string{HTTPBindIPKey: "10.0.0.5", HTTPHostKey: domain},
			wantIP:         "10.0.0.5",
			wantKey:        HTTPBindIPKey,
			wantListenAddr: "10.0.0.5",
			wantProbeHosts: []string{"10.0.0.5", "127.0.0.1"},
		},
		{
			name:           "wildcard bind ip wins over a domain host",
			values:         map[string]string{HTTPBindIPKey: "0.0.0.0", HTTPHostKey: domain},
			wantIP:         "0.0.0.0",
			wantKey:        HTTPBindIPKey,
			wantListenAddr: "",
			wantProbeHosts: []string{"127.0.0.1"},
		},
		{
			name:           "zoned bind ip is passed through unparsed",
			values:         map[string]string{HTTPBindIPKey: "fe80::1%eth0"},
			wantIP:         "fe80::1%eth0",
			wantKey:        HTTPBindIPKey,
			wantListenAddr: "fe80::1%eth0",
			wantProbeHosts: []string{"fe80::1%eth0", "127.0.0.1"},
		},
		{
			name:           "domain on a local interface",
			values:         map[string]string{HTTPHostKey: domain},
			resolved:       []string{"203.0.113.10"},
			localIPs:       []string{"127.0.0.1", "203.0.113.10"},
			wantIP:         "203.0.113.10",
			wantKey:        HTTPHostKey,
			wantListenAddr: "203.0.113.10",
			wantProbeHosts: []string{"203.0.113.10", "127.0.0.1"},
			wantLookups:    []string{domain},
		},
		{
			name:           "first local address of several wins",
			values:         map[string]string{HTTPHostKey: domain},
			resolved:       []string{"198.51.100.7", "203.0.113.10"},
			localIPs:       []string{"203.0.113.10"},
			wantIP:         "203.0.113.10",
			wantKey:        HTTPHostKey,
			wantListenAddr: "203.0.113.10",
			wantProbeHosts: []string{"203.0.113.10", "127.0.0.1"},
			wantLookups:    []string{domain},
		},
		{
			name:           "ipv6 domain on a local interface",
			values:         map[string]string{HTTPHostKey: domain},
			resolved:       []string{"2001:db8::1"},
			localIPs:       []string{"2001:db8::1"},
			wantIP:         "2001:db8::1",
			wantKey:        HTTPHostKey,
			wantListenAddr: "2001:db8::1",
			wantProbeHosts: []string{"2001:db8::1", "::1"},
			wantLookups:    []string{domain},
		},
		{
			name:           "localhost resolves to loopback without repeating it",
			values:         map[string]string{HTTPHostKey: "localhost"},
			resolved:       []string{"127.0.0.1", "::1"},
			localIPs:       []string{"127.0.0.1", "::1"},
			wantIP:         "127.0.0.1",
			wantKey:        HTTPHostKey,
			wantListenAddr: "127.0.0.1",
			wantProbeHosts: []string{"127.0.0.1"},
			wantLookups:    []string{"localhost"},
		},
		{
			name:           "unresolvable domain",
			values:         map[string]string{HTTPHostKey: domain},
			resolveErr:     assert.AnError,
			wantIP:         "0.0.0.0",
			wantKey:        HTTPHostKey,
			wantListenAddr: "",
			wantProbeHosts: []string{"127.0.0.1"},
			wantLookups:    []string{domain},
		},
		{
			name:           "domain resolving off box",
			values:         map[string]string{HTTPHostKey: domain},
			resolved:       []string{"198.51.100.7"},
			localIPs:       []string{"127.0.0.1", "10.0.0.5"},
			wantIP:         "0.0.0.0",
			wantKey:        HTTPHostKey,
			wantListenAddr: "",
			wantProbeHosts: []string{"127.0.0.1"},
			wantLookups:    []string{domain},
		},
		{
			name:           "no interface addresses",
			values:         map[string]string{HTTPHostKey: domain},
			resolved:       []string{"203.0.113.10"},
			localIPs:       nil,
			wantIP:         "0.0.0.0",
			wantKey:        HTTPHostKey,
			wantListenAddr: "",
			wantProbeHosts: []string{"127.0.0.1"},
			wantLookups:    []string{domain},
		},
		{
			name:           "host carrying a port is treated as a name",
			values:         map[string]string{HTTPHostKey: "example.com:8080"},
			resolveErr:     assert.AnError,
			wantIP:         "0.0.0.0",
			wantKey:        HTTPHostKey,
			wantListenAddr: "",
			wantProbeHosts: []string{"127.0.0.1"},
			wantLookups:    []string{"example.com:8080"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var lookups []string

			resolver := bindResolver{
				lookupHost: func(ctx context.Context, host string) ([]string, error) {
					_, ok := ctx.Deadline()
					assert.True(t, ok, "the lookup has to be bounded by a deadline")

					lookups = append(lookups, host)

					return test.resolved, test.resolveErr
				},
				localIPs: func() []string {
					return test.localIPs
				},
			}

			bind := resolver.resolve(t.Context(), test.values)

			assert.Equal(t, test.wantIP, bind.IP)
			assert.Equal(t, test.wantKey, bind.Key)
			assert.Equal(t, test.wantListenAddr, bind.ListenAddr())
			assert.Equal(t, test.wantProbeHosts, bind.ProbeHosts())
			assert.Equal(t, test.wantLookups, lookups)
		})
	}
}

func TestProbeEach(t *testing.T) {
	const (
		bound    = "2.29.29.94:443"
		loopback = "127.0.0.1:443"
	)

	tests := []struct {
		name      string
		addrs     []string
		answering string
		wantCalls []string
		wantErr   []string
	}{
		{
			name:      "the bound address answers",
			addrs:     []string{bound, loopback},
			answering: bound,
			wantCalls: []string{bound},
		},
		{
			name:      "the bound address is refused and loopback answers",
			addrs:     []string{bound, loopback},
			answering: loopback,
			wantCalls: []string{bound, loopback},
		},
		{
			name:      "neither answers",
			addrs:     []string{bound, loopback},
			wantCalls: []string{bound, loopback},
			wantErr:   []string{bound, loopback},
		},
		{
			name:      "a single address keeps its own error",
			addrs:     []string{loopback},
			wantCalls: []string{loopback},
			wantErr:   []string{loopback},
		},
		{
			name:    "no address at all is a failure",
			wantErr: []string{"no address"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls []string

			err := ProbeEach(test.addrs, func(addr string) error {
				calls = append(calls, addr)

				if addr == test.answering {
					return nil
				}

				return errors.Errorf("%s refused the connection", addr)
			})

			assert.Equal(t, test.wantCalls, calls)

			if len(test.wantErr) == 0 {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)

			for _, want := range test.wantErr {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

func TestBindAddressProbeAddrs(t *testing.T) {
	tests := []struct {
		name string
		bind BindAddress
		port string
		want []string
	}{
		{
			name: "wildcard",
			bind: BindAddress{IP: "0.0.0.0"},
			port: "443",
			want: []string{"127.0.0.1:443"},
		},
		{
			name: "ipv4 literal",
			bind: BindAddress{IP: "2.29.29.94"},
			port: "443",
			want: []string{"2.29.29.94:443", "127.0.0.1:443"},
		},
		{
			name: "ipv6 literal is bracketed",
			bind: BindAddress{IP: "2001:db8::1"},
			port: "8443",
			want: []string{"[2001:db8::1]:8443", "[::1]:8443"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			addrs := test.bind.ProbeAddrs(test.port)

			require.Len(t, addrs, len(test.want))
			assert.Equal(t, test.want, addrs)
		})
	}
}

func TestBindAddressString(t *testing.T) {
	assert.Equal(t, "every interface", BindAddress{}.String())
	assert.Equal(t, "every interface", BindAddress{IP: "0.0.0.0"}.String())
	assert.Equal(t, "every interface", BindAddress{IP: "::"}.String())
	assert.Equal(t, "2.29.29.94", BindAddress{IP: "2.29.29.94"}.String())
}
