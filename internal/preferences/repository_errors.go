package preferences

import "errors"

var (
	ErrNotFound = errors.New("preferences not found")
	ErrTimeout  = errors.New("preferences timeout")
	ErrInternal = errors.New("preferences internal")
)
