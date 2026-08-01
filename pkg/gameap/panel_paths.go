package gameap

type PanelPaths struct {
	Scope string

	ConfigDir      string
	ConfigFilePath string
	DataDir        string
	FilesBasePath  string
	BinaryPath     string

	SystemdUnitPath string
	SystemdUnitDir  string

	User  string
	Group string
}
