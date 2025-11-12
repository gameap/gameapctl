//go:build !windows

package service

import (
	"context"

	"github.com/pkg/errors"
)

type WindowsNil struct{}

func NewWindows() *WindowsNil {
	return &WindowsNil{}
}

func (s *WindowsNil) Start(_ context.Context, _ string) error {
	return errors.New("unsupported")
}

func (s *WindowsNil) Stop(_ context.Context, _ string) error {
	return errors.New("unsupported")
}

func (s *WindowsNil) Restart(_ context.Context, _ string) error {
	return errors.New("unsupported")
}

func (s *WindowsNil) Status(_ context.Context, _ string) error {
	return errors.New("unsupported")
}

func IsExists(_ context.Context, _ string) bool {
	panic("function is not implemented")
}
