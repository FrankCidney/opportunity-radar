package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/lib/pq"
)

const postgresUniqueViolation = "23505"

type PostgresRepository struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewPostgresRepository(db *sql.DB, logger *slog.Logger) *PostgresRepository {
	return &PostgresRepository{db: db, logger: logger}
}

func (r *PostgresRepository) CreateRegistration(
	ctx context.Context,
	registration Registration,
) (*Principal, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, r.mapError("begin registration", err)
	}
	defer tx.Rollback()

	var user User
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, email_verified_at, disabled_at, created_at, updated_at
	`, registration.Email, registration.PasswordHash).Scan(
		&user.ID,
		&user.Email,
		&user.EmailVerifiedAt,
		&user.DisabledAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, r.mapError("create registration user", err)
	}

	var tenant Tenant
	err = tx.QueryRowContext(ctx, `
		INSERT INTO tenants (name)
		VALUES ($1)
		RETURNING id, name, is_legacy, created_at, updated_at
	`, registration.TenantName).Scan(
		&tenant.ID,
		&tenant.Name,
		&tenant.IsLegacy,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	)
	if err != nil {
		return nil, r.mapError("create registration tenant", err)
	}

	var membership Membership
	err = tx.QueryRowContext(ctx, `
		INSERT INTO tenant_memberships (tenant_id, user_id, role)
		VALUES ($1, $2, 'owner')
		RETURNING tenant_id, user_id, role, created_at
	`, tenant.ID, user.ID).Scan(
		&membership.TenantID,
		&membership.UserID,
		&membership.Role,
		&membership.CreatedAt,
	)
	if err != nil {
		return nil, r.mapError("create owner membership", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, user.ID, registration.SessionTokenHash, registration.SessionExpiresAt); err != nil {
		return nil, r.mapError("create registration session", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO account_tokens (
			user_id, purpose, token_hash, target_email, expires_at
		)
		VALUES ($1, 'verify_email', $2, $3, $4)
	`, user.ID, registration.VerificationTokenHash, user.Email, registration.VerificationExpiresAt); err != nil {
		return nil, r.mapError("create registration verification token", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, r.mapError("commit registration", err)
	}

	return &Principal{
		User:       user,
		Tenant:     tenant,
		Membership: membership,
	}, nil
}

func (r *PostgresRepository) GetCredentialByEmail(
	ctx context.Context,
	email string,
) (*Credential, error) {
	var credential Credential
	err := r.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, email_verified_at, disabled_at, created_at, updated_at
		FROM users
		WHERE email = $1
	`, email).Scan(
		&credential.User.ID,
		&credential.User.Email,
		&credential.PasswordHash,
		&credential.User.EmailVerifiedAt,
		&credential.User.DisabledAt,
		&credential.User.CreatedAt,
		&credential.User.UpdatedAt,
	)
	if err != nil {
		return nil, r.mapError("get credential by email", err)
	}
	return &credential, nil
}

func (r *PostgresRepository) GetUserByID(
	ctx context.Context,
	userID int64,
) (*User, error) {
	return r.scanUser(r.db.QueryRowContext(ctx, `
		SELECT id, email, email_verified_at, disabled_at, created_at, updated_at
		FROM users
		WHERE id = $1
	`, userID), "get user by id")
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (r *PostgresRepository) scanUser(row rowScanner, operation string) (*User, error) {
	var user User
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.EmailVerifiedAt,
		&user.DisabledAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, r.mapError(operation, err)
	}
	return &user, nil
}

func (r *PostgresRepository) CreateSession(
	ctx context.Context,
	userID int64,
	tokenHash []byte,
	expiresAt time.Time,
) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, tokenHash, expiresAt)
	if err != nil {
		return r.mapError("create session", err)
	}
	return nil
}

func (r *PostgresRepository) GetPrincipalBySessionHash(
	ctx context.Context,
	tokenHash []byte,
	now time.Time,
) (*Principal, error) {
	var principal Principal
	err := r.db.QueryRowContext(ctx, `
		SELECT
			u.id, u.email, u.email_verified_at, u.disabled_at,
			u.created_at, u.updated_at,
			t.id, t.name, t.is_legacy, t.created_at, t.updated_at,
			m.tenant_id, m.user_id, m.role, m.created_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		JOIN tenant_memberships m ON m.user_id = u.id
		JOIN tenants t ON t.id = m.tenant_id
		WHERE s.token_hash = $1
		  AND s.expires_at > $2
		  AND u.disabled_at IS NULL
		ORDER BY CASE WHEN m.role = 'owner' THEN 0 ELSE 1 END, m.created_at
		LIMIT 1
	`, tokenHash, now).Scan(
		&principal.User.ID,
		&principal.User.Email,
		&principal.User.EmailVerifiedAt,
		&principal.User.DisabledAt,
		&principal.User.CreatedAt,
		&principal.User.UpdatedAt,
		&principal.Tenant.ID,
		&principal.Tenant.Name,
		&principal.Tenant.IsLegacy,
		&principal.Tenant.CreatedAt,
		&principal.Tenant.UpdatedAt,
		&principal.Membership.TenantID,
		&principal.Membership.UserID,
		&principal.Membership.Role,
		&principal.Membership.CreatedAt,
	)
	if err != nil {
		return nil, r.mapError("get principal by session", err)
	}
	return &principal, nil
}

func (r *PostgresRepository) DeleteSession(
	ctx context.Context,
	tokenHash []byte,
) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE token_hash = $1
	`, tokenHash)
	if err != nil {
		return r.mapError("delete session", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return r.mapError("read deleted session count", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) ReplaceAccountToken(
	ctx context.Context,
	userID int64,
	purpose TokenPurpose,
	tokenHash []byte,
	targetEmail string,
	expiresAt time.Time,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return r.mapError("begin replace account token", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		DELETE FROM account_tokens
		WHERE user_id = $1
		  AND purpose = $2
		  AND consumed_at IS NULL
	`, userID, purpose)
	if err != nil {
		return r.mapError("delete previous account tokens", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO account_tokens (
			user_id, purpose, token_hash, target_email, expires_at
		)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5)
	`, userID, purpose, tokenHash, targetEmail, expiresAt)
	if err != nil {
		return r.mapError("create account token", err)
	}

	if err := tx.Commit(); err != nil {
		return r.mapError("commit account token replacement", err)
	}
	return nil
}

func (r *PostgresRepository) ConsumeEmailVerification(
	ctx context.Context,
	tokenHash []byte,
	now time.Time,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return r.mapError("begin email verification", err)
	}
	defer tx.Rollback()

	var userID int64
	var targetEmail string
	err = tx.QueryRowContext(ctx, `
		UPDATE account_tokens
		SET consumed_at = $2
		WHERE token_hash = $1
		  AND purpose = 'verify_email'
		  AND consumed_at IS NULL
		  AND expires_at > $2
		RETURNING user_id, COALESCE(target_email, '')
	`, tokenHash, now).Scan(&userID, &targetEmail)
	if err != nil {
		return r.mapError("consume email verification token", err)
	}

	if targetEmail == "" {
		return fmt.Errorf("consume email verification: %w", ErrInternal)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE users
		SET email = $2,
		    email_verified_at = $3,
		    updated_at = $3
		WHERE id = $1
		  AND disabled_at IS NULL
	`, userID, targetEmail, now)
	if err != nil {
		return r.mapError("verify user email", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return r.mapError("read verified user count", err)
	}
	if rows != 1 {
		return ErrNotFound
	}

	if err := tx.Commit(); err != nil {
		return r.mapError("commit email verification", err)
	}
	return nil
}

func (r *PostgresRepository) ConsumePasswordReset(
	ctx context.Context,
	tokenHash []byte,
	passwordHash string,
	now time.Time,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return r.mapError("begin password reset", err)
	}
	defer tx.Rollback()

	var userID int64
	err = tx.QueryRowContext(ctx, `
		UPDATE account_tokens
		SET consumed_at = $2
		WHERE token_hash = $1
		  AND purpose = 'reset_password'
		  AND consumed_at IS NULL
		  AND expires_at > $2
		RETURNING user_id
	`, tokenHash, now).Scan(&userID)
	if err != nil {
		return r.mapError("consume password reset token", err)
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE users
		SET password_hash = $2,
		    updated_at = $3
		WHERE id = $1
		  AND disabled_at IS NULL
	`, userID, passwordHash, now)
	if err != nil {
		return r.mapError("update reset password", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return r.mapError("read reset user count", err)
	}
	if rows != 1 {
		return ErrNotFound
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE user_id = $1
	`, userID); err != nil {
		return r.mapError("delete sessions after password reset", err)
	}

	if err := tx.Commit(); err != nil {
		return r.mapError("commit password reset", err)
	}
	return nil
}

func (r *PostgresRepository) LegacyTenantNeedsClaim(
	ctx context.Context,
) (bool, error) {
	var needsClaim bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM tenants t
			WHERE t.is_legacy = TRUE
			  AND NOT EXISTS (
				SELECT 1
				FROM tenant_memberships m
				WHERE m.tenant_id = t.id
			  )
		)
	`).Scan(&needsClaim)
	if err != nil {
		return false, r.mapError("inspect legacy tenant claim", err)
	}
	return needsClaim, nil
}

func (r *PostgresRepository) LegacyTenantReady(
	ctx context.Context,
) (bool, error) {
	var ready bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM tenants t
			JOIN tenant_memberships m ON m.tenant_id = t.id
			JOIN users u ON u.id = m.user_id
			WHERE t.is_legacy = TRUE
			  AND m.role = 'owner'
			  AND u.email_verified_at IS NOT NULL
			  AND u.disabled_at IS NULL
		)
	`).Scan(&ready)
	if err != nil {
		return false, r.mapError("inspect legacy tenant readiness", err)
	}
	return ready, nil
}

func (r *PostgresRepository) ClaimLegacyTenant(
	ctx context.Context,
	email string,
	passwordHash string,
	verifiedAt time.Time,
) (*Principal, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, r.mapError("begin legacy tenant claim", err)
	}
	defer tx.Rollback()

	var tenant Tenant
	err = tx.QueryRowContext(ctx, `
		SELECT id, name, is_legacy, created_at, updated_at
		FROM tenants
		WHERE is_legacy = TRUE
		FOR UPDATE
	`).Scan(
		&tenant.ID,
		&tenant.Name,
		&tenant.IsLegacy,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	)
	if err != nil {
		return nil, r.mapError("load legacy tenant for claim", err)
	}

	var membershipExists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM tenant_memberships
			WHERE tenant_id = $1
		)
	`, tenant.ID).Scan(&membershipExists)
	if err != nil {
		return nil, r.mapError("check legacy tenant membership", err)
	}
	if membershipExists {
		return nil, ErrLegacyAlreadyClaimed
	}

	var user User
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (
			email, password_hash, email_verified_at
		)
		VALUES ($1, $2, $3)
		RETURNING id, email, email_verified_at, disabled_at, created_at, updated_at
	`, email, passwordHash, verifiedAt).Scan(
		&user.ID,
		&user.Email,
		&user.EmailVerifiedAt,
		&user.DisabledAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, r.mapError("create legacy owner", err)
	}

	var membership Membership
	err = tx.QueryRowContext(ctx, `
		INSERT INTO tenant_memberships (tenant_id, user_id, role)
		VALUES ($1, $2, 'owner')
		RETURNING tenant_id, user_id, role, created_at
	`, tenant.ID, user.ID).Scan(
		&membership.TenantID,
		&membership.UserID,
		&membership.Role,
		&membership.CreatedAt,
	)
	if err != nil {
		return nil, r.mapError("create legacy owner membership", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, r.mapError("commit legacy tenant claim", err)
	}
	return &Principal{
		User:       user,
		Tenant:     tenant,
		Membership: membership,
	}, nil
}

func (r *PostgresRepository) mapError(operation string, err error) error {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("%s: %w", operation, ErrNotFound)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%s: %w", operation, err)
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == postgresUniqueViolation {
		return fmt.Errorf("%s: %w", operation, ErrConflict)
	}

	if r.logger != nil {
		r.logger.Error("authentication repository operation failed",
			"operation", operation,
			"error", err,
		)
	}
	return fmt.Errorf("%s: %w", operation, ErrInternal)
}
