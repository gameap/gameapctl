//go:build !windows

package sendlogs

import "context"

func collectServiceLogs(_ context.Context, _ string) error {
	return nil
}

func collectServiceStatus(_ context.Context, _ string) error {
	return nil
}
