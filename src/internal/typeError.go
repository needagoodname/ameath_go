package internal

import (
	"errors"
)

type typeError struct {
	msg string
	err error
}

func (msg string, err error) *typeError {
	return &typeError{msg: msg, err: err}
}