package store

import "errors"

var (
	ErrNotFound      = errors.New("record not found")
	ErrConflict      = errors.New("conflict: concurrent update detected")
	ErrAlreadyExists = errors.New("tenant already exists")
)
