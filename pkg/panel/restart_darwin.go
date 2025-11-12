package panel

import (
	"context"
)

func Restart(_ context.Context) error {
	return NewNotImplementedError("restart", "MacOS")
}
