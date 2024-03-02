//go:build windows
// +build windows

package daemoninstall

import (
	"context"
	"os/user"
	"strings"

	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
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
	if err != nil &&
		!errors.As(err, &errUnknownUser) &&
		!strings.Contains(err.Error(), "No mapping between account") {
		return state, errors.WithMessage(err, "failed to lookup user")
	}

	password, err := utils.CryptoRandomString(24)
	if err != nil {
		return state, errors.WithMessage(err, "failed to generate password")
	}

	if systemUser != nil {
		// Change password
		err = utils.ExecCommand("net", "user", userName, password)
		if err != nil {
			return state, errors.WithMessagef(err, "failed to change password for user %s", userName)
		}

		state.User = userName
		state.Password = password

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
	defaultProcessManager = "winsw"
)

func configureProcessManager(_ context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	state.ProcessManager = defaultProcessManager

	return state, nil
}

func installOSSpecificPackages(
	ctx context.Context,
	pm packagemanager.PackageManager,
	state daemonsInstallState,
) (daemonsInstallState, error) {
	// Some game services require Visual C++ Redistributable x86
	// For example, Counter-Strike 1.6
	err := pm.Install(ctx, packagemanager.VCRedist17X86)
	if err != nil {
		return state, errors.WithMessage(err, "failed to install Visual C++ Redistributable 2017 x86")
	}

	return state, nil
}
