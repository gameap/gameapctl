package oscore

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/pkg/errors"
)

type GrantFlag string

// Permission flags for Windows ACL permissions.
const (
	GrantFlagFullControl = "F"  // Full Control (read, write, delete, change permissions)
	GrantFlagModify      = "M"  // Modify (read, write, delete)
	GrantFlagReadExecute = "RX" // Read & Execute
	GrantFlagRead        = "R"  // Read-only
	GrantFlagWrite       = "W"  // Write-only
)

// ChownRecursive is a no-op on Windows.
func ChownRecursive(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

// ChownR is a no-op on Windows.
func ChownR(_ context.Context, _ string, _, _ int) error {
	return nil
}

// Grant grants access permissions to a user for the specified path on Windows.
// This function uses the icacls command to set Windows ACL permissions.
//
// Parameters:
//   - ctx: The context
//   - path: The file or directory path to grant permissions on
//   - userName: The Windows user name or SID to grant permissions to
//   - permission: The permission flag to grant (GrantFlagFullControl, GrantFlagModify,
//     GrantFlagReadExecute, GrantFlagRead, or GrantFlagWrite)
//
// The permissions are applied recursively with inheritance flags:
//   - (OI) - Object Inherit: files inherit permissions
//   - (CI) - Container Inherit: subdirectories inherit permissions
func Grant(ctx context.Context, path string, userName string, permission GrantFlag) error {
	userName = strings.TrimSpace(userName)

	userIdentifier := WindowsAccountIdentifier(userName)
	grantParam := fmt.Sprintf("%s:(OI)(CI)%s", userIdentifier, string(permission))

	output, err := ExecCommandWithOutput(ctx, "icacls", path, "/grant", grantParam, "/T")
	if err != nil {
		log.Printf(
			"failed to grant %s permissions to user %s on path %s\nOutput:\n %s\n\n",
			permission, userName, path, output,
		)

		return errors.WithMessagef(
			err,
			"failed to grant %s permissions to user %s on path %s",
			permission, userName, path,
		)
	}

	return nil
}

// GrantFullControl grants full control permissions.
// Full control includes read, write, delete, and change permissions.
func GrantFullControl(ctx context.Context, path string, userName string) error {
	return Grant(ctx, path, userName, GrantFlagFullControl)
}

// GrantModify grants modify permissions.
// Modify includes read, write, and delete permissions, but not change permissions.
func GrantModify(ctx context.Context, path string, userName string) error {
	return Grant(ctx, path, userName, GrantFlagModify)
}

// GrantReadExecute grants read and execute permissions.
func GrantReadExecute(ctx context.Context, path string, userName string) error {
	return Grant(ctx, path, userName, GrantFlagReadExecute)
}

// RestrictDirectoryAccess gives the directory a protected DACL and makes Administrators its
// owner: the entries inherited from its parent are removed, SYSTEM and Administrators get full
// control and Users get read and execute access. Explicit entries of other accounts are kept,
// so calling it again changes nothing.
//
// A directory created in the root of a drive inherits the ACL of the root, which lets every
// local user create and modify files in it.
func RestrictDirectoryAccess(ctx context.Context, path string) error {
	if err := ExecCommand(ctx, "icacls", restrictDirectoryAccessArgs(path)...); err != nil {
		return errors.WithMessagef(err, "failed to restrict access to %s", path)
	}

	if err := ExecCommand(ctx, "icacls", restrictDirectoryOwnerArgs(path)...); err != nil {
		return errors.WithMessagef(err, "failed to make Administrators the owner of %s", path)
	}

	return nil
}
