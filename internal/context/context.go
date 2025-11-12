package context

import (
	"context"

	osinfo "github.com/gameap/gameapctl/pkg/os_info"
)

type contextKey int

const (
	osInfo contextKey = iota
)

func OSInfoFromContext(ctx context.Context) osinfo.Info {
	info, _ := ctx.Value(osInfo).(osinfo.Info)

	return info
}

func contextWithOSInfo(ctx context.Context, info osinfo.Info) context.Context {
	return context.WithValue(ctx, osInfo, info)
}

func SetOSContext(ctx context.Context) (context.Context, error) {
	osInfo, err := osinfo.GetOSInfo(ctx)
	if err != nil {
		return ctx, err
	}

	ctx = contextWithOSInfo(ctx, osInfo)

	return ctx, nil
}
