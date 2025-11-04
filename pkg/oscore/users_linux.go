//go:build linux

package oscore

import (
	"context"
	"fmt"
	"os/exec"
	"os/user"
	"strings"

	"github.com/pkg/errors"
)

func CreateGroup(ctx context.Context, groupname string, opts ...CreateGroupOption) error {
	options := applyCreateGroupOptions(opts...)

	// Check if group already exists
	err := ExecCommand(ctx, "getent", "group", groupname)
	if err == nil {
		return fmt.Errorf("group %s already exists", groupname)
	}

	// Build groupadd arguments
	args := []string{}

	if options.gid != "" {
		args = append(args, "-g", options.gid)
	}

	args = append(args, groupname)

	// Create group with groupadd
	err = ExecCommand(ctx, "groupadd", args...)
	if err != nil {
		return errors.WithMessage(err, "failed to exec groupadd command")
	}

	return nil
}

func CreateUser(
	ctx context.Context, username string, opts ...CreateUserOption,
) error {
	options := applyCreateUserOptions(opts...)

	// Check if user already exists
	_, err := user.Lookup(username)
	if err == nil {
		return fmt.Errorf("user %s already exists", username)
	}

	// Build useradd arguments
	args := []string{"-m"}

	if options.workDir != "" {
		args = append(args, "-d", options.workDir)
	}

	shell := "/bin/bash"
	if options.shell != "" {
		shell = options.shell
	}
	args = append(args, "-s", shell, username)

	gid, err := user.LookupGroup(username)
	if err == nil {
		args = append(args, "-g", gid.Name)
	}

	// Create user with useradd
	err = ExecCommand(ctx, "useradd", args...)
	if err != nil {
		return errors.WithMessage(err, "failed to exec useradd command")
	}

	// Set password using chpasswd if provided
	if options.password != "" {
		chpasswdCmd := exec.CommandContext(ctx, "chpasswd")
		chpasswdCmd.Stdin = strings.NewReader(fmt.Sprintf("%s:%s", username, options.password))
		if err := chpasswdCmd.Run(); err != nil {
			return errors.WithMessage(err, "failed to run command")
		}
	}

	return nil
}
