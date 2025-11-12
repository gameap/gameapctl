//go:build linux || darwin

package runhelper

import (
	"context"
	"log"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/process"
)

const (
	InitUnknown = "unknown"
	InitSystemd = "systemd"
)

func DetectInit(ctx context.Context) (string, error) {
	result := InitUnknown

	p, err := process.NewProcessWithContext(ctx, 1)
	if err != nil {
		return result, errors.WithMessage(err, "failed to load process with pid 1")
	}

	processName, _ := p.Name()
	log.Println("Found process name:", processName)

	exe, err := p.Exe()
	if err != nil {
		return result, errors.WithMessage(err, "failed to get executable path of the process")
	}

	originalExe, err := filepath.EvalSymlinks(exe)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to evaluate symlink"))
	}

	filename := originalExe
	if filename == "" {
		filename = exe
	}

	switch filepath.Base(filename) {
	case "systemd":
		log.Println("Detected systemd init")
		result = InitSystemd
	default:
		log.Println("Unsupported init:", filename)
	}

	return result, nil
}
