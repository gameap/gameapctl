package utils

import (
	"os"

	"github.com/otiai10/copy"
)

func Move(src string, dst string) error {
	return os.Rename(src, dst)
}

func Copy(src string, dst string) error {
	return copy.Copy(src, dst)
}
