package utils

import (
	"net"
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
