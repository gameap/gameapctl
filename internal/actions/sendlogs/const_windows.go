//go:build windows

package sendlogs

const (
	defaultPanelInstallPath = "C:\\gameap\\web"

	apiPath = "https://api.gameap.io/send-logs"

	// Default log file names.
	logsPathGamectl = "C:\\gameap\\logs"
	logsPathDaemon  = "C:\\gameap\\daemon\\logs"

	// Service definitions and logs of the services wrapped by shawl.
	// GameAP v4 writes nothing on its own, its stdout and stderr are captured
	// by shawl into servicesLogsPath.
	servicesPath     = "C:\\gameap\\services"
	servicesLogsPath = "C:\\gameap\\services\\logs"
)
