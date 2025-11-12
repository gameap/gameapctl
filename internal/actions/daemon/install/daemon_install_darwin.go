//go:build darwin

package install

import "context"

const (
	defaultProcessManager = "tmux"
)

func defineProcessManager(_ context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	state.ProcessManager = defaultProcessManager

	return state, nil
}
