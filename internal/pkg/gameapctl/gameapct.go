package gameapctl

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

func stateDirectory() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.WithMessage(err, "failed to get user home dir")
	}

	dir := filepath.Join(homeDir, ".gameapctl")
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		err = os.Mkdir(dir, 0600)
		if err != nil {
			return "", errors.WithMessage(err, "failed to create state directory")
		}
	}

	return dir, nil
}
