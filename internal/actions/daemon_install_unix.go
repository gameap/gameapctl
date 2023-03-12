//go:build linux || darwin
// +build linux darwin

package actions

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"

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

func serviceConfigure(ctx context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	tempDir, err := os.MkdirTemp("", "gameap-daemon-service")
	if err != nil {
		return state, errors.WithMessage(err, "failed to create temp dir")
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			log.Println(err)
		}
	}(tempDir)

	if utils.IsCommandAvailable("systemctl") {
		log.Println("Downloading systemctl service configuration")
		err = utils.Download(
			ctx,
			"https://packages.gameap.ru/gameap-daemon/systemd-service.tar.gz",
			tempDir,
		)
		if err != nil {
			return state, errors.WithMessage(err, "failed to download service configuration")
		}

		err = utils.Copy(filepath.Join(tempDir, "gameap-daemon.service"), "/etc/systemd/system/gameap-daemon.service")
		if err != nil {
			return state, errors.WithMessage(err, "failed to copy service configuration")
		}

		err = utils.ExecCommand("systemctl", "daemon-reload")
		if err != nil {
			return state, errors.WithMessage(err, "failed to reload systemctl")
		}
	}

	return state, nil
}
