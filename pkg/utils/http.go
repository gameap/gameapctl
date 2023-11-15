package utils

import (
	"context"

	"github.com/hashicorp/go-getter"
)

func Download(ctx context.Context, source string, dst string) error {
	c := getter.Client{
		Ctx:  ctx,
		Src:  source,
		Dst:  dst,
		Mode: getter.ClientModeAny,
	}

	return c.Get()
}

func DownloadFile(ctx context.Context, source string, dst string) error {
	c := getter.Client{
		Ctx:  ctx,
		Src:  source,
		Dst:  dst,
		Mode: getter.ClientModeFile,
	}

	return c.Get()
}

func DownloadFileOrArchive(ctx context.Context, source string, dst string) error {
	c := getter.Client{
		Ctx:           ctx,
		Src:           source,
		Dst:           dst,
		Mode:          getter.ClientModeFile,
		Decompressors: map[string]getter.Decompressor{},
	}

	return c.Get()
}
