//go:build linux || darwin
// +build linux darwin

package actions

import (
	"context"
	"fmt"
	"os/user"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func createUser(_ context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	fmt.Println("Checking for gameap group existence ...")
	_, err := user.LookupGroup("gameap")

	if err != nil {
		fmt.Println("Creating group...")
		err = utils.ExecCommand("groupadd", "gameap")
		if err != nil {
			return daemonsInstallState{}, errors.WithMessage(err, "failed to add group")
		}
	}

	fmt.Println("Checking for gameap user existence ...")
	_, err = user.Lookup("gameap")
	if err != nil {
		switch err.(type) {
		case user.UnknownUserError:
			fmt.Println("Creating user...")
			err = utils.ExecCommand(
				"useradd",
				"-g", "gameap", "-d", state.WorkPath, "-s", "/bin/bash", "gameap")
			if err != nil {
				return daemonsInstallState{}, errors.WithMessage(err, "failed to add group")
			}
		default:
			return state, errors.WithMessage(err, "failed to lookup user")
		}
	}

	return state, nil
}
