package service

import "fmt"

type NotFoundError struct {
	ServiceName string
}

func NewNotFoundError(serviceName string) *NotFoundError {
	return &NotFoundError{
		ServiceName: serviceName,
	}
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("service %s not found", e.ServiceName)
}
