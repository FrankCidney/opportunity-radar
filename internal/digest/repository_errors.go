package digest

import "errors"

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("resource already exists")
	ErrTimeout  = errors.New("repository operation timed out")
	ErrInternal = errors.New("internal repository error")
)
