//go:build windows
// +build windows

package utils

func uidAndGIDForFile(_ string) (uint32, uint32) {
	return 0, 0
}
