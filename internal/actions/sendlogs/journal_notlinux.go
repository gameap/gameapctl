//go:build !linux

package sendlogs

import "context"

func collectJournalLogs(_ context.Context, _ string) error {
	return nil
}
