package install

import "github.com/pkg/errors"

var errEmptyHost = errors.New("empty host")

var errConnectHostNotCovered = errors.New("connect host is not covered by the panel gRPC certificate")
