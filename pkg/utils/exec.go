package utils

import (
	"bytes"
	"log"
	"os/exec"
)

// Deprecated: use oscore.ExecCommand.
func ExecCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	log.Println('\n', cmd.String())

	return cmd.Run()
}

// Deprecated: use oscore.ExecCommandWithOutput.
func ExecCommandWithOutput(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	buf := &bytes.Buffer{}
	buf.Grow(1024) //nolint:mnd
	cmd.Stdout = buf
	cmd.Stderr = log.Writer()
	log.Println('\n', cmd.String())
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
