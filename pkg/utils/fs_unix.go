//go:build linux || darwin

package utils

import (
	"context"
	"os"
	"syscall"

	"github.com/gameap/gameapctl/pkg/oscore"
)

func uidAndGIDForFile(path string) (uint32, uint32) {
	stat, err := os.Stat(path)
	if err != nil {
		return 0, 0
	}
	var uid uint32
	var gid uint32

	if sysStat, ok := stat.Sys().(*syscall.Stat_t); ok {
		uid = sysStat.Uid
		gid = sysStat.Gid
	}

	return uid, gid
}

// Deprecated: use oscore.ChownR instead.
func ChownR(path string, uid, gid int) error {
	return oscore.ChownR(context.TODO(), path, uid, gid)
}
