//go:build windows
// +build windows

package daemon

import (
	"context"

	"github.com/pkg/errors"
)

func Restart(ctx context.Context) error {
	return errors.New("not implemented")
}
