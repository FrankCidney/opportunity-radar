package jobs

import "errors"

// Service errors - these express business meaning.
// Handlers map these to HTTP responses.
var (
	ErrJobNotFound = errors.New("job not found")
	ErrJobAlreadyExists = errors.New("job already exists")
	ErrInvalidTransition = errors.New("invalid status transition")
    ErrServiceInternal   = errors.New("internal service error")
)