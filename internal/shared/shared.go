package shared

import "errors"

var ErrNotFound = errors.New("not found")

type DomainError struct {
	Expected bool
	Err      error
	Msg      string
}

func NewDomainError(err error, msg string) *DomainError {
	return &DomainError{
		Expected: true,
		Err:      err,
		Msg:      msg,
	}
}

func NewUnexpectedError(err error, msg string) *DomainError {
	return &DomainError{
		Expected: false,
		Err:      err,
		Msg:      msg,
	}
}
