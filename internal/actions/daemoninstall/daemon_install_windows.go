//go:build windows
// +build windows

package daemoninstall

import (
	"context"
	"crypto/rand"
	"math/big"
	"os/user"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

const (
	defaultUserName = "gameap"
)

func createUser(_ context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	userName := defaultUserName
	var errUnknownUser user.UnknownUserError
	systemUser, err := user.Lookup(userName)
	if err != nil && !errors.As(err, &errUnknownUser) {
		return state, errors.WithMessage(err, "failed to lookup user")
	}

	password, err := generatePassword(24)
	if err != nil {
		return state, errors.WithMessage(err, "failed to generate password")
	}

	if systemUser != nil {
		// Change password
		err = utils.ExecCommand("net", "user", userName, password)
		if err != nil {
			return state, errors.WithMessagef(err, "failed to change password for user %s", userName)
		}
		return state, nil
	}

	err = utils.ExecCommand("net", "user", userName, password, "/HOMEDIR:C:\\gameap", "/ADD", "/Y")
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
