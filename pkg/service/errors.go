package service

import "fmt"

type ErrServiceNotFound struct {
	ServiceName string
}

func NewErrServiceNotFound(serviceName string) *ErrServiceNotFound {
	return &ErrServiceNotFound{
		ServiceName: serviceName,
	}
}

func (e *ErrServiceNotFound) Error() string {
	return fmt.Sprintf("service %s not found", e.ServiceName)
}
