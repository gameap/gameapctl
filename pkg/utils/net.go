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
