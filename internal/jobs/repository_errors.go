package jobs

import "errors"

var (
	// ErrNotFound is returned when a queried resource does not exist
	ErrNotFound = errors.New("not found")

	// ErrConflict is returned on a unique violation (23505)
	// e.g., a job with the same source and url already exists
	ErrConflict = errors.New("resource already exists")

	// ErrReferenceNotFound is returned when a referenced resource
	// does not exist (23503) e.g., the given company_id is invalid
	ErrReferenceNotFound = errors.New("referenced resource does not exist")

	// ErrTimeout is returned when teh query is cancelled or exceeds its
	// deadline (57014)
	ErrTimeout = errors.New("repository operation timed out")

	// ErrInternal is returned for all other unexpected database errors.
	// The raw error is wrapped inside for logging purposes in the service.
	ErrInternal = errors.New("internal repository error")
)
