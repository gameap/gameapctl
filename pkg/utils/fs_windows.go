//go:build windows

package utils

func uidAndGIDForFile(_ string) (uint32, uint32) {
	return 0, 0
}

func ChownR(_ string, _, _ int) error {
	return nil
}
