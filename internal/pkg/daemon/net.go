package daemon

import (
	"net"
	"time"

	"github.com/pkg/errors"
)

const grpcDialTimeout = 5 * time.Second

func CheckGRPCConnectivity(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, grpcDialTimeout)
	if err != nil {
		return errors.Wrapf(err, "cannot reach gRPC server at %s", addr)
	}
	_ = conn.Close()

	return nil
}
