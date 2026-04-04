//go:build windows

package sendlogs

import "context"

func collectWebServerLogs(_ context.Context, _ string) error {
	return nil
}
