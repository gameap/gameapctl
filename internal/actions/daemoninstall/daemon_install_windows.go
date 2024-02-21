//go:build windows
// +build windows

package daemoninstall

import (
	"context"
	"crypto/rand"
	"math/big"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	defaultUserName = "gameap"
)

func createUser(_ context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	password, err := generatePassword(24)
	if err != nil {
		return state, errors.WithMessage(err, "failed to generate password")
	}

	userName := defaultUserName

	err = utils.ExecCommand("net", "user", userName, password, "/HOMEDIR:C:\\gameap", "/ADD")
	if err != nil {
		return state, errors.WithMessage(err, "failed to add user")
	}

	state.User = userName
	state.Password = password

	return state, nil
}

func setUserPrivileges(_ context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	return state, nil
}

func setFirewallRules(_ context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	err := utils.ExecCommand(
		"netsh",
		"advfirewall",
		"firewall",
		"add",
		"rule",
		"name=GameAP_Daemon",
		"dir=in",
		"action=allow",
		"protocol=TCP",
		"localport=31717",
	)
	if err != nil {
		return state, errors.WithMessage(err, "failed to execute netsh command")
	}

	return state, nil
}

const (
	characterSet = "abcdedfghijklmnopqrstABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

func generatePassword(passwordLength int) (string, error) {
	password := make([]byte, 0, passwordLength)
	m := big.NewInt(int64(len(characterSet)))
	for i := 0; i < passwordLength; i++ {
		n, err := rand.Int(rand.Reader, m)
		if err != nil {
			return "", errors.WithMessage(err, "failed to generate random number")
		}
		character := characterSet[n.Int64()]
		password = append(password, character)
	}

	return string(password), nil
}
