package panel

import (
	"path/filepath"

	"github.com/gameap/gameapctl/pkg/gameap"
)

const (
	defaultDataDir        = gameap.DefaultDataPath
	defaultBinaryPath     = gameap.DefaultBinaryPath
	defaultSystemdUnitDir = "/etc/systemd/system"
	defaultUser           = "gameap"
	defaultGroup          = "gameap"
)

const (
	processName = "gameap"
)

var (
	defaultConfigDir = filepath.Dir(gameap.DefaultConfigFilePath)
)
