package utils

import (
	"net"
	"strconv"

	"github.com/pkg/errors"
)

func IsIPv4(ip string) bool {
	if net.ParseIP(ip) == nil {
		return false
	}

	for i := 0; i < len(ip); i++ {
		if ip[i] == '.' {
			return true
		}
	}

	return false
}

func IsIPv6(ip string) bool {
	if net.ParseIP(ip) == nil {
		return false
	}

	for i := 0; i < len(ip); i++ {
		if ip[i] == ':' {
			return true
		}
	}

	return false
}

func DetectIPs() []string {
	ips := make([]string, 0)

	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}

		for _, a := range addrs {
			switch v := a.(type) {
			case *net.IPNet:
				ips = append(ips, v.IP.String())
			case *net.IPAddr:
				ips = append(ips, v.IP.String())
			}
		}
	}

	return ips
}

func RemoveLocalIPs(ips []string) []string {
	result := make([]string, 0, len(ips))

	for _, ip := range ips {
		if IsIPv4(ip) {
			if ip[:4] == "127." {
				continue
			}
		}

		if IsIPv6(ip) {
			if ip == "::1" || ip[:2] == "fc" || ip[:2] == "fd" || ip[:2] == "fe" {
				continue
			}
		}

		result = append(result, ip)
	}

	return result
}

const (
	// fallbackPortsLimit bounds how many fallback ports are probed after the preferred
	// one: when none of them is free, the cause is the network configuration rather
	// than an occupied port.
	fallbackPortsLimit = 10
	firstFallbackPort  = 8025

	wildcardIPv4 = "0.0.0.0"
)

// CheckPortAvailability reports whether a TCP listener can be opened on the given
// address and port. The bind error is wrapped as is, so that the caller is able to
// tell a permission problem from an occupied port.
func CheckPortAvailability(listenAddr, port string) error {
	err := listenAndClose("tcp", listenAddr, port)
	if err != nil {
		return err
	}

	if listenAddr != "" && listenAddr != wildcardIPv4 {
		return nil
	}

	// A wildcard address needs a second, IPv4-only probe: "tcp" binds a dual-stack
	// IPv6 socket, which on BSD-derived systems happily coexists with an IPv4-only
	// listener and would report an occupied port as free.
	return listenAndClose("tcp4", wildcardIPv4, port)
}

func listenAndClose(network, listenAddr, port string) error {
	listener, err := net.Listen(network, net.JoinHostPort(listenAddr, port))
	if err != nil {
		return errors.Wrap(err, "failed to open listener")
	}

	err = listener.Close()
	if err != nil {
		return errors.Wrap(err, "failed to close listener")
	}

	return nil
}

// FindAvailablePort returns the first port bindable on listenAddr: preferred first,
// then fallbackPortsLimit ports starting from 8025. Nothing free among them means a
// network configuration problem rather than an occupied port, so the search gives up
// instead of walking the whole port range.
func FindAvailablePort(listenAddr, preferred string) (string, bool) {
	if preferred != "" && CheckPortAvailability(listenAddr, preferred) == nil {
		return preferred, true
	}

	probed := 0
	for port := firstFallbackPort; probed < fallbackPortsLimit; port++ {
		candidate := strconv.Itoa(port)
		if candidate == preferred {
			continue
		}

		probed++

		if CheckPortAvailability(listenAddr, candidate) == nil {
			return candidate, true
		}
	}

	return "", false
}
