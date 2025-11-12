package daemon

import (
	"context"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/shirou/gopsutil/v3/process"
)

func FindProcess(ctx context.Context) (*process.Process, error) {
	return oscore.FindProcessByName(ctx, daemonProcessName)
}
