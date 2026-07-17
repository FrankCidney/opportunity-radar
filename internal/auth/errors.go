package auth

import "errors"

var (
	ErrInvalidCredentials   = errors.New("invalid email or password")
	ErrEmailAlreadyExists   = errors.New("email address is already registered")
	ErrInvalidEmail         = errors.New("invalid email address")
	ErrInvalidPassword      = errors.New("password does not meet requirements")
	ErrUnauthenticated      = errors.New("authentication required")
	ErrAccountDisabled      = errors.New("account is disabled")
	ErrEmailAlreadyVerified = errors.New("email address is already verified")
	ErrInvalidToken         = errors.New("token is invalid or expired")
	ErrLegacyAlreadyClaimed = errors.New("legacy workspace already has an owner")
	ErrNotFound             = errors.New("record not found")
	ErrConflict             = errors.New("record conflict")
	ErrInternal             = errors.New("authentication service unavailable")
)
