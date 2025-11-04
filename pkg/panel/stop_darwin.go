package panel

import (
	"context"
)

func Stop(_ context.Context) error {
	return NewNotImplementedError("stop", "MacOS")
}
