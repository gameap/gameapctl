package oscore

import "context"

// ChownRecursiveToUser is a no-op on Windows.
func ChownRecursiveToUser(_ context.Context, _ string, _ string) error {
	return nil
}

// ChownR is a no-op on Windows.
func ChownR(_ context.Context, _ string, _, _ int) error {
	return nil
}
