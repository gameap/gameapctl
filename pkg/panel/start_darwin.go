package panel

import (
	"context"
)

func Start(_ context.Context, opts ...Options) error {
	if err := checkScopeSupported(opts); err != nil {
		return err
	}

	return NewNotImplementedError("start", "MacOS")
}
