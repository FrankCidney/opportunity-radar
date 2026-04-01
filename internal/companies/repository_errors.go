package companies

import "errors"

var (
	// These errors are only meant to indicate what happened.
	// The service layer translates "what happened" into "what does it mean"

	// ErrNotFound is returned when a queried resource does not exist
	ErrNotFound = errors.New("not found")

	// ErrConflict is returned on a unique violation (23505)
	ErrConflict = errors.New("resource already exists")

	// ErrTimeout is returned when the query is cancelled or exceeds its deadline (57014)
	ErrTimeout = errors.New("repository operation timed out")

	// ErrInternal is returned for all other unexpected database errors.
	// The raw error is wrapped inside for logging purposes in the service.
	ErrInternal = errors.New("internal repository error")
)
