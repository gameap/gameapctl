package gameap

const (
	ScopeSystem = "system"
	ScopeUser   = "user"
)

type DaemonPaths struct {
	Scope string

	WorkPath     string
	SteamCMDPath string
	ToolsPath    string

	CertsPath            string
	DaemonFilePath       string
	DaemonConfigFilePath string
	DaemonConfigDir      string
	OutputLogPath        string

	SystemdUnitPath string
	SystemdUnitDir  string
}
