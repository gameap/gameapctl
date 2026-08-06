package panel

import (
	"context"
)

func Stop(_ context.Context, opts ...Options) error {
	if err := checkScopeSupported(opts); err != nil {
		return err
	}

	return NewNotImplementedError("stop", "MacOS")
}
