//go:build darwin

package oscore

import (
	"context"
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

func CreateGroup(ctx context.Context, groupname string, opts ...CreateGroupOption) error {
	options := applyCreateGroupOptions(opts...)

	// Check if group already exists using dscl
	cmd := exec.CommandContext(ctx, "dscl", ".", "-read", fmt.Sprintf("/Groups/%s", groupname))
	if err := cmd.Run(); err == nil {
		return fmt.Errorf("group %s already exists", groupname)
	}

	// Find the next available GID if not provided
	gid := options.gid
	if gid == "" {
		var err error
		gid, err = findNextAvailableGID(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to find available GID")
		}
	}

	// Create group with dscl
	err := ExecCommand(ctx, "dscl", ".", "-create", fmt.Sprintf("/Groups/%s", groupname))
	if err != nil {
		return errors.WithMessage(err, "failed to create group record")
	}

	// Set the group ID
	err = ExecCommand(ctx, "dscl", ".", "-create", fmt.Sprintf("/Groups/%s", groupname), "PrimaryGroupID", gid)
	if err != nil {
		return errors.WithMessage(err, "failed to set GID")
	}

	// Set the group's real name
	err = ExecCommand(ctx, "dscl", ".", "-create", fmt.Sprintf("/Groups/%s", groupname), "RealName", groupname)
	if err != nil {
		return errors.WithMessage(err, "failed to set group name")
	}

	// Set password (usually not used for groups)
	err = ExecCommand(ctx, "dscl", ".", "-create", fmt.Sprintf("/Groups/%s", groupname), "Password", "*")
	if err != nil {
		return errors.WithMessage(err, "failed to set group password")
	}

	return nil
}

func findNextAvailableGID(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "dscl", ".", "-list", "/Groups", "PrimaryGroupID")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	maxGID := 500
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			gid, err := strconv.Atoi(fields[1])
			if err == nil && gid >= 500 && gid < 600 {
				if gid > maxGID {
					maxGID = gid
				}
			}
		}
	}

	return strconv.Itoa(maxGID + 1), nil
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

	// Find the next available UID
	uid, err := findNextAvailableUID(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to find available UID")
	}

	// Create user with dscl
	// Create the user record
	err = ExecCommand(ctx, "dscl", ".", "-create", fmt.Sprintf("/Users/%s", username))
	if err != nil {
		return errors.WithMessage(err, "failed to create user record")
	}

	// Set the user shell
	shell := "/bin/bash"
	if options.shell != "" {
		shell = options.shell
	}
	err = ExecCommand(ctx, "dscl", ".", "-create", fmt.Sprintf("/Users/%s", username), "UserShell", shell)
	if err != nil {
		return errors.WithMessage(err, "failed to set user shell")
	}

	// Set the user's full name
	err = ExecCommand(ctx, "dscl", ".", "-create", fmt.Sprintf("/Users/%s", username), "RealName", username)
	if err != nil {
		return errors.WithMessage(err, "failed to set real name")
	}

	// Set the user ID
	err = ExecCommand(ctx, "dscl", ".", "-create", fmt.Sprintf("/Users/%s", username), "UniqueID", uid)
	if err != nil {
		return errors.WithMessage(err, "failed to set UID")
	}

	// Set the primary group ID (staff = 20)
	err = ExecCommand(ctx, "dscl", ".", "-create", fmt.Sprintf("/Users/%s", username), "PrimaryGroupID", "20")
	if err != nil {
		return errors.WithMessage(err, "failed to set primary group")
	}

	// Set the home directory
	homeDir := fmt.Sprintf("/Users/%s", username)
	if options.workDir != "" {
		homeDir = options.workDir
	}
	err = ExecCommand(ctx, "dscl", ".", "-create", fmt.Sprintf("/Users/%s", username), "NFSHomeDirectory", homeDir)
	if err != nil {
		return errors.WithMessage(err, "failed to set home directory")
	}

	// Create the home directory
	err = ExecCommand(ctx, "createhomedir", "-c", "-u", username)
	if err != nil {
		return errors.WithMessage(err, "failed to create home directory")
	}

	// Set password if provided
	if options.password != "" {
		passwdCmd := exec.CommandContext(ctx, "dscl", ".", "-passwd", fmt.Sprintf("/Users/%s", username), options.password)
		if err := passwdCmd.Run(); err != nil {
			return errors.WithMessage(err, "failed to set password")
		}
	}

	return nil
}

func findNextAvailableUID(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "dscl", ".", "-list", "/Users", "UniqueID")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	maxUID := 500
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			uid, err := strconv.Atoi(fields[1])
			if err == nil && uid >= 500 && uid < 600 {
				if uid > maxUID {
					maxUID = uid
				}
			}
		}
	}

	return strconv.Itoa(maxUID + 1), nil
}
