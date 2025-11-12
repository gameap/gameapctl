package oscore

import "strings"

const (
	defaultGrowSize = 64
)

type GroupAlreadyExistsError struct {
	name string
}

func NewGroupAlreadyExistsError(groupName string) *GroupAlreadyExistsError {
	return &GroupAlreadyExistsError{name: groupName}
}

func (e *GroupAlreadyExistsError) Error() string {
	sb := strings.Builder{}
	sb.Grow(defaultGrowSize)

	sb.WriteString("group ")
	sb.WriteString(e.name)
	sb.WriteString(" already exists")

	return sb.String()
}

type UserAlreadyExistsError struct {
	name string
}

func NewUserAlreadyExistsError(userName string) *UserAlreadyExistsError {
	return &UserAlreadyExistsError{name: userName}
}

func (e *UserAlreadyExistsError) Error() string {
	sb := strings.Builder{}
	sb.Grow(defaultGrowSize)

	sb.WriteString("user ")
	sb.WriteString(e.name)
	sb.WriteString(" already exists")

	return sb.String()
}
