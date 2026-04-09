package install

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

var errInvalidConnectURL = errors.New("invalid connect URL")

type ConnectInfo struct {
	Host     string
	Port     uint16
	SetupKey string
}

func ParseConnectURL(rawURL string) (ConnectInfo, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ConnectInfo{}, errors.WithMessage(errInvalidConnectURL, "failed to parse URL")
	}

	if u.Scheme != "grpc" {
		return ConnectInfo{}, errors.WithMessage(errInvalidConnectURL, "scheme must be \"grpc\"")
	}

	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		return ConnectInfo{}, errors.WithMessage(errInvalidConnectURL, "host and port are required")
	}

	if host == "" {
		return ConnectInfo{}, errors.WithMessage(errInvalidConnectURL, "host is required")
	}

	portNum, err := strconv.Atoi(portStr)
	if err != nil || portNum < 1 || portNum > 65535 {
		return ConnectInfo{}, errors.WithMessage(errInvalidConnectURL, "port must be between 1 and 65535")
	}

	key := strings.TrimPrefix(u.Path, "/")
	if key == "" {
		return ConnectInfo{}, errors.WithMessage(errInvalidConnectURL, "setup key is required")
	}

	return ConnectInfo{
		Host:     host,
		Port:     uint16(portNum),
		SetupKey: key,
	}, nil
}
