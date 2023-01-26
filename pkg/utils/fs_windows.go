//go:build windows
// +build windows

package utils

func uidAndGidForFile(_ string) (uint32, uint32) {
	return 0, 0
}
