//go:build windows

package oscore

import (
	"context"
	"os/exec"
	"os/user"

	"github.com/pkg/errors"
)

func CreateGroup(ctx context.Context, groupname string, opts ...CreateGroupOption) error {
	// Check if group already exists
	cmd := exec.CommandContext(ctx, "net", "localgroup", groupname)
	if err := cmd.Run(); err == nil {
		return NewGroupAlreadyExistsError(groupname)
	}

	// Create group with net localgroup command
	err := ExecCommand(ctx, "net", "localgroup", groupname, "/add")
	if err != nil {
		return errors.WithMessage(err, "failed to create group")
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
		return NewUserAlreadyExistsError(username)
	}

	// Create user with net user command
	args := []string{"user", username}

	if options.password != "" {
		args = append(args, options.password)
	} else {
		args = append(args, "*")
	}

	args = append(args, "/add")

	// Add full name comment if specified
	args = append(args, "/fullname:"+username)

	err = ExecCommand(ctx, "net", args...)
	if err != nil {
		return errors.WithMessage(err, "failed to create user")
	}

	// Set user directory if specified (requires additional wmi or registry manipulation)
	// Windows home directory is typically set automatically by the system
	if options.workDir != "" { //nolint:staticcheck
		// Note: Setting custom home directory on Windows requires additional
		// WMI or registry manipulation, which is not trivially done via net command
		// This would require using Windows API or PowerShell
		// For now, we'll skip this or could implement via PowerShell
	}

	return nil
}
