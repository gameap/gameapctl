package panel

import (
	"context"
)

func Restart(_ context.Context, opts ...Options) error {
	if err := checkScopeSupported(opts); err != nil {
		return err
	}

	return NewNotImplementedError("restart", "MacOS")
}
