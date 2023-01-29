package utils

import (
	"log"
	"os/exec"
)

func IsCommandAvailable(command string) bool {
	path, err := exec.LookPath(command)

	if err == nil {
		log.Printf("command '%s' available in '%s'\n", command, path)
		return true
	}

	return false
}
