package daemon

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckGRPCConnectivity_reachable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	err = CheckGRPCConnectivity(listener.Addr().String())
	assert.NoError(t, err)
}

func TestCheckGRPCConnectivity_unreachable(t *testing.T) {
	err := CheckGRPCConnectivity("127.0.0.1:1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot reach gRPC server")
}
