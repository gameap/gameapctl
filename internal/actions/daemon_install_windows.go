//go:build windows
// +build windows

package actions

import "context"

func createUser(_ context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	return state, nil
}

func serviceConfigure(ctx context.Context, state daemonsInstallState) (daemonsInstallState, error) {
	return state, nil
}
