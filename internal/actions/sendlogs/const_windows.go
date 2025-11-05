//go:build windows

package sendlogs

const (
	defaultPanelInstallPath = "C:\\gameap\\web"

	apiPath = "https://api.gameap.io/send-logs"

	// Default log file names.
	logsPathGamectl = "C:\\gameap\\logs"
	logsPathDaemon  = "C:\\gameap\\daemon\\logs"
)
