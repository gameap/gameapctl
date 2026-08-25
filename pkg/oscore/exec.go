package oscore

import (
	"bytes"
	"context"
	"log"
	"os/exec"

	"github.com/pkg/errors"
)

func ExecCommand(ctx context.Context, command string, args ...string) error {
	cmd := exec.CommandContext(ctx, command, args...)

	stdout := NewOutputDecoder(log.Writer())
	stderr := NewOutputDecoder(log.Writer())

	cmd.Stdout = stdout
	cmd.Stderr = stderr
	log.Println("\n" + cmd.String())

	err := cmd.Run()

	stdout.Flush()
	stderr.Flush()

	return err
}

func ExecCommandWithOutput(ctx context.Context, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	buf := &bytes.Buffer{}
	buf.Grow(1024) //nolint:mnd
	stderr := NewOutputDecoder(log.Writer())

	cmd.Stdout = buf
	cmd.Stderr = stderr
	log.Println("\n" + cmd.String())

	err := cmd.Run()

	stderr.Flush()

	if err != nil {
		return "", errors.Wrapf(err, "failed to run command %s", command)
	}

	return buf.String(), nil
}
