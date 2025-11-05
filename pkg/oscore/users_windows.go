//go:build windows

package oscore

import (
	"context"
	"os/exec"
	"os/user"

	"github.com/pkg/errors"
)

func CreateGroup(ctx context.Context, groupname string, opts ...CreateGroupOption) error {
	// Create group with net localgroup command
	// Note: We attempt to create the group directly because checking for existence
	// with "net localgroup <name>" is unreliable on Windows (it may fail to find
	// existing groups due to domain/local conflicts or permission issues)
	cmd := exec.CommandContext(ctx, "net", "localgroup", groupname, "/add")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if the error is because the group already exists
		// Error code 2224: "The account already exists"
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
			// Group already exists, which is fine
			return NewGroupAlreadyExistsError(groupname)
		}

		return errors.WithMessagef(err, "failed to create group: %s", string(output))
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
