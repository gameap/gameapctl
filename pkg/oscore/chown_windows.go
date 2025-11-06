package oscore

import (
	"context"
	"fmt"
	"log"

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
//   - userName: The Windows user name to grant permissions to
//   - permission: The permission flag to grant (GrantFlagFullControl, GrantFlagModify,
//     GrantFlagReadExecute, GrantFlagRead, or GrantFlagWrite)
//
// The permissions are applied recursively with inheritance flags:
//   - (OI) - Object Inherit: files inherit permissions
//   - (CI) - Container Inherit: subdirectories inherit permissions
func Grant(ctx context.Context, path string, userName string, permission GrantFlag) error {
	grantParam := fmt.Sprintf("%s:(OI)(CI)%s", userName, string(permission))

	// Execute icacls command
	// /grant - grants specified permissions
	// /T - applies recursively to all files and subdirectories
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
