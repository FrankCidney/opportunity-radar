package auth

import (
	"context"
	"time"
)

type Repository interface {
	CreateRegistration(ctx context.Context, registration Registration) (*Principal, error)
	GetCredentialByEmail(ctx context.Context, email string) (*Credential, error)
	GetUserByID(ctx context.Context, userID int64) (*User, error)

	CreateSession(
		ctx context.Context,
		userID int64,
		tokenHash []byte,
		expiresAt time.Time,
	) error
	GetPrincipalBySessionHash(
		ctx context.Context,
		tokenHash []byte,
		now time.Time,
	) (*Principal, error)
	DeleteSession(ctx context.Context, tokenHash []byte) error

	ReplaceAccountToken(
		ctx context.Context,
		userID int64,
		purpose TokenPurpose,
		tokenHash []byte,
		targetEmail string,
		expiresAt time.Time,
	) error
	ConsumeEmailVerification(
		ctx context.Context,
		tokenHash []byte,
		now time.Time,
	) error
	ConsumePasswordReset(
		ctx context.Context,
		tokenHash []byte,
		passwordHash string,
		now time.Time,
	) error
}

type Registration struct {
	Email                 string
	PasswordHash          string
	TenantName            string
	SessionTokenHash      []byte
	SessionExpiresAt      time.Time
	VerificationTokenHash []byte
	VerificationExpiresAt time.Time
}
