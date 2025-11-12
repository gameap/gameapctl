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

	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	log.Println('\n', cmd.String())

	return cmd.Run()
}

func ExecCommandWithOutput(ctx context.Context, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	buf := &bytes.Buffer{}
	buf.Grow(1024) //nolint:mnd
	cmd.Stdout = buf
	cmd.Stderr = log.Writer()
	log.Println('\n', cmd.String())
	err := cmd.Run()
	if err != nil {
		return "", errors.Wrapf(err, "failed to run command %s", command)
	}

	return buf.String(), nil
}
