package util

type InternalError struct {
	s string
}

func (e *InternalError) Error() string {
	return e.s
}

func NewInternalError(s string) error {
	return &InternalError{s}
}

type UserError struct {
	s string
}

func (e *UserError) Error() string {
	return e.s
}

func NewUserError(s string) error {
	return &UserError{s}
}
