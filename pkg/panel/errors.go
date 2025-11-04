package panel

import "errors"

var ErrGameAPAlreadyInstalled = errors.New("GameAP is already installed")
var ErrGameAPNotInstalled = errors.New("GameAP is not installed")

type NotImplementedError struct {
	Feature string
	OS      string
}

func NewNotImplementedError(feature, os string) *NotImplementedError {
	return &NotImplementedError{
		Feature: feature,
		OS:      os,
	}
}

func (e *NotImplementedError) Error() string {
	return e.Feature + " on " + e.OS + " is not implemented"
}
