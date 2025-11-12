//go:build windows

package install

import (
	"context"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/pkg/errors"
)

const (
	defaultUserName = "NT AUTHORITY\\NETWORK SERVICE"
)

func createUser(_ context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	return state, nil
}

func setUserPrivileges(ctx context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	if err := oscore.GrantFullControl(
		ctx,
		gameap.DefaultWorkPath,
		defaultUserName,
	); err != nil {
		return state, errors.WithMessage(err, "failed to set permissions for working directory")
	}

	return state, nil
}

func setFirewallRules(ctx context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	err := oscore.ExecCommand(
		ctx,
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

func defineProcessManager(_ context.Context, state daemonsInstallState) (daemonsInstallState, error) {
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
