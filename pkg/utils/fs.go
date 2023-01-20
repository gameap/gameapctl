package utils

import (
	"io/fs"
	"os"

	"github.com/otiai10/copy"
	"github.com/pkg/errors"
)

func Move(src string, dst string) error {
	if _, err := os.Stat(src); errors.Is(err, fs.ErrNotExist) {
		return errors.Errorf("source file %s not found", src)
	}
	if _, err := os.Stat(dst); errors.Is(err, fs.ErrNotExist) {
		err = os.MkdirAll(dst, 0755)
		if err != nil {
			return errors.WithMessagef(err, "failed to create destination directory %s", dst)
		}
	}
	return os.Rename(src, dst)
}

func Copy(src string, dst string) error {
	return copy.Copy(src, dst)
}
