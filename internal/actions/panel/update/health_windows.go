//go:build windows

package update

import "context"

// newCrashDetector has nothing to watch on Windows: the wait relies on its
// timeout.
func newCrashDetector(context.Context, string) crashDetector {
	return noCrashDetection
}
