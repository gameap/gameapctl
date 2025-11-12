//go:build linux || darwin

package install

import (
	"context"
	"fmt"
	"os/user"
	"strconv"

	"github.com/gameap/gameapctl/pkg/oscore"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/pkg/errors"
)

func createUser(ctx context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	fmt.Println("Checking for gameap group existence ...")
	_, err := user.LookupGroup("gameap")

	if err != nil {
		fmt.Println("Creating group...")
		err = oscore.CreateGroup(ctx, "gameap")
		if err != nil {
			return daemonsInstallState{}, errors.WithMessage(err, "failed to create group")
		}
	}

	fmt.Println("Checking for gameap user existence ...")
	_, err = user.Lookup("gameap")
	if err != nil {
		var unknownUserError user.UnknownUserError
		switch {
		case errors.As(err, &unknownUserError):
			fmt.Println("Creating user...")
			err = oscore.CreateUser(ctx, "gameap", oscore.WithWorkDir(state.WorkPath))

			if err != nil {
				return daemonsInstallState{}, errors.WithMessage(err, "failed to create user")
			}
		default:
			return state, errors.WithMessage(err, "failed to lookup user")
		}
	}

	return state, nil
}

func setUserPrivileges(ctx context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	gameapUser, err := user.Lookup("gameap")
	if err != nil {
		return state, errors.WithMessage(err, "failed to lookup user")
	}

	uid, err := strconv.Atoi(gameapUser.Uid)
	if err != nil {
		return state, errors.WithMessage(err, "failed to convert uid to int")
	}

	gid, err := strconv.Atoi(gameapUser.Gid)
	if err != nil {
		return state, errors.WithMessage(err, "failed to convert gid to int")
	}

	err = oscore.ChownR(ctx, state.WorkPath, uid, gid)
	if err != nil {
		return daemonsInstallState{}, err
	}

	return state, nil
}

func setFirewallRules(_ context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	return state, nil
}

func installOSSpecificPackages(
	_ context.Context,
	_ packagemanager.PackageManager,
	state daemonsInstallState,
) (daemonsInstallState, error) {
	return state, nil
}
