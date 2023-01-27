//go:build linux || darwin
// +build linux darwin

package utils

import (
	"os"
	"syscall"
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
