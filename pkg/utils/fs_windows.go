//go:build windows

package utils

// FileOwner returns 0, 0: Windows has no uid/gid to report.
func FileOwner(_ string) (uint32, uint32) {
	return 0, 0
}

func ChownR(_ string, _, _ int) error {
	return nil
}
