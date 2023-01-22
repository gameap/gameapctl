package utils

import (
	"log"
	"os"
	"os/exec"
)

func ExecCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Println('\n', cmd.String())
	return cmd.Run()
}
