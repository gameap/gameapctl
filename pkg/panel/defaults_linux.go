package panel

import (
	"path/filepath"

	"github.com/gameap/gameapctl/pkg/gameap"
)

const (
	defaultDataDir    = gameap.DefaultDataPath
	defaultBinaryPath = gameap.DefaultBinaryPath
	defaultUser       = "gameap"
	defaultGroup      = "gameap"
)

const (
	processName = "gameap"
)

var (
	defaultConfigDir = filepath.Dir(gameap.DefaultConfigFilePath)
)
