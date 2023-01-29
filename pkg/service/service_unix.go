//go:build !windows
// +build !windows

package service

import (
	"context"

	"github.com/pkg/errors"
)

type WindowsNil struct{}

func NewWindows() *WindowsNil {
	return &WindowsNil{}
}

func (s *WindowsNil) Start(ctx context.Context, serviceName string) error {
	return errors.New("unsupported")
}

func (s *WindowsNil) Stop(ctx context.Context, serviceName string) error {
	return errors.New("unsupported")
}

func (s *WindowsNil) Restart(ctx context.Context, serviceName string) error {
	return errors.New("unsupported")
}
