package panel

import (
	"context"
)

func install(_ context.Context, _ InstallConfig) error {
	return NewNotImplementedError("installation", "MacOS")
}
