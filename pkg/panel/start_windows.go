package panel

import "context"

func Start(_ context.Context) error {
	return NewNotImplementedError("start", "Windows")
}
