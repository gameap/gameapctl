//go:build darwin
// +build darwin

package daemoninstall

import "context"

const (
	defaultProcessManager = "tmux"
)

func configureProcessManager(_ context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	state.ProcessManager = defaultProcessManager

	return state, nil
}
