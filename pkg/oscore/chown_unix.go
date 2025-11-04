package oscore

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
)

func ChownRecursive(ctx context.Context, path string, userName string, groupName string) error {
	u, err := user.Lookup(userName)
	if err != nil {
		return errors.WithMessage(err, "failed to lookup user")
	}

	g, err := user.LookupGroup(groupName)
	if err != nil {
		return errors.WithMessage(err, "failed to lookup group")
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return errors.WithMessage(err, "failed to convert uid to int")
	}

	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return errors.WithMessage(err, "failed to convert gid to int")
	}

	err = ChownR(ctx, path, uid, gid)
	if err != nil {
		return errors.Wrap(err, "failed to chown")
	}

	return nil
}

// ChownR recursively changes the ownership of all files and directories under path.
// Based on https://github.com/gutengo/fil/blob/6109b2e0b5cfdefdef3a254cc1a3eaa35bc89284/file.go#L27
func ChownR(ctx context.Context, path string, uid, gid int) error {
	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			// Ignore invalid
			//nolint:nilerr
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			symlinkFile, err := os.Readlink(name)
			if err != nil {
				// Ignore invalid symlink
				//nolint:nilerr
				return nil
			}

			if _, err = os.Stat(symlinkFile); err != nil {
				// Ignore invalid symlink
				//nolint:nilerr
				return nil
			}
		}

		return os.Chown(name, uid, gid)
	})
}
