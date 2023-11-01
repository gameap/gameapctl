//go:build linux || darwin
// +build linux darwin

package utils

import (
	"os"
	"path/filepath"
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

// https://github.com/gutengo/fil/blob/6109b2e0b5cfdefdef3a254cc1a3eaa35bc89284/file.go#L27
func ChownR(path string, uid, gid int) error {
	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			// Ignore invalid
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			symlinkFile, err := os.Readlink(name)
			if err != nil {
				// Ignore invalid symlink
				return nil
			}

			if _, err = os.Stat(symlinkFile); err != nil {
				// Ignore invalid symlink
				return nil
			}
		}

		return os.Chown(name, uid, gid)
	})
}
