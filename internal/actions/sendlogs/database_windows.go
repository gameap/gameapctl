//go:build windows

package sendlogs

import "context"

func collectDatabaseLogs(_ context.Context, _ string) error {
	return nil
}
