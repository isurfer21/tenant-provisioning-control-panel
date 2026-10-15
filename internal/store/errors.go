package store

import "errors"

var (
	ErrNotFound      = errors.New("record not found")
	ErrConflict      = errors.New("conflict: concurrent update detected")
	ErrAlreadyExists = errors.New("tenant already exists")
)

// ErrValidation wraps a model validation failure.
type ErrValidation struct {
	Message string
}

func (e *ErrValidation) Error() string {
	return "validation error: " + e.Message
}
