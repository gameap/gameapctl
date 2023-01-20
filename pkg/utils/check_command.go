package utils

import "os/exec"

func IsCommandAvailable(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}
