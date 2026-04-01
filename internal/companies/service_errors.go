package companies

import "errors"

// Service errors express business meaning.
// Handlers map these to HTTP responses.
var (
	ErrCompanyNotFound      = errors.New("company not found")
	ErrCompanyAlreadyExists = errors.New("company already exists")
	ErrServiceInternal      = errors.New("internal service error")
)
