package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

const maximumEmailBytes = 254

type ServiceConfig struct {
	SessionTTL       time.Duration
	VerificationTTL  time.Duration
	PasswordResetTTL time.Duration
}

type Service struct {
	repo              Repository
	hasher            PasswordHasher
	tokens            TokenGenerator
	config            ServiceConfig
	logger            *slog.Logger
	now               func() time.Time
	dummyPasswordHash string
}

func NewService(
	repo Repository,
	hasher PasswordHasher,
	tokens TokenGenerator,
	config ServiceConfig,
	logger *slog.Logger,
) *Service {
	if hasher == nil {
		hasher = BcryptHasher{}
	}
	if tokens == nil {
		tokens = SecureTokenGenerator{}
	}
	if config.SessionTTL <= 0 {
		config.SessionTTL = 30 * 24 * time.Hour
	}
	if config.VerificationTTL <= 0 {
		config.VerificationTTL = 24 * time.Hour
	}
	if config.PasswordResetTTL <= 0 {
		config.PasswordResetTTL = time.Hour
	}
	if logger == nil {
		logger = slog.Default()
	}
	dummyPasswordHash, err := hasher.Hash("dummy password used only for timing equalization")
	if err != nil {
		logger.Error("failed to initialize login timing equalizer", "error", err)
	}

	return &Service{
		repo:              repo,
		hasher:            hasher,
		tokens:            tokens,
		config:            config,
		logger:            logger,
		now:               time.Now,
		dummyPasswordHash: dummyPasswordHash,
	}
}

func (s *Service) Register(
	ctx context.Context,
	email string,
	password string,
) (*AuthResult, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return nil, err
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}

	sessionToken, sessionHash, err := s.tokens.Generate()
	if err != nil {
		s.logger.Error("failed to generate registration session token", "error", err)
		return nil, ErrInternal
	}
	verificationToken, verificationHash, err := s.tokens.Generate()
	if err != nil {
		s.logger.Error("failed to generate registration verification token", "error", err)
		return nil, ErrInternal
	}
	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		s.logger.Error("failed to hash registration password", "error", err)
		return nil, ErrInternal
	}

	now := s.now().UTC()
	sessionExpiresAt := now.Add(s.config.SessionTTL)
	principal, err := s.repo.CreateRegistration(ctx, Registration{
		Email:                 normalizedEmail,
		PasswordHash:          passwordHash,
		TenantName:            defaultTenantName(normalizedEmail),
		SessionTokenHash:      sessionHash,
		SessionExpiresAt:      sessionExpiresAt,
		VerificationTokenHash: verificationHash,
		VerificationExpiresAt: now.Add(s.config.VerificationTTL),
	})
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return nil, ErrEmailAlreadyExists
		}
		s.logger.Error("failed to create registration", "error", err)
		return nil, ErrInternal
	}

	return &AuthResult{
		Principal:         *principal,
		SessionToken:      sessionToken,
		SessionExpiresAt:  sessionExpiresAt,
		VerificationToken: verificationToken,
	}, nil
}

func (s *Service) Login(
	ctx context.Context,
	email string,
	password string,
) (*AuthResult, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	credential, err := s.repo.GetCredentialByEmail(ctx, normalizedEmail)
	if err != nil {
		if s.dummyPasswordHash != "" {
			_ = s.hasher.Compare(s.dummyPasswordHash, password)
		}
		if !errors.Is(err, ErrNotFound) {
			s.logger.Error("failed to load login user", "error", err)
		}
		return nil, ErrInvalidCredentials
	}
	user := &credential.User
	if user.DisabledAt != nil {
		return nil, ErrAccountDisabled
	}
	if err := s.hasher.Compare(credential.PasswordHash, password); err != nil {
		return nil, ErrInvalidCredentials
	}

	rawToken, tokenHash, err := s.tokens.Generate()
	if err != nil {
		s.logger.Error("failed to generate login session token", "user_id", user.ID, "error", err)
		return nil, ErrInternal
	}
	expiresAt := s.now().UTC().Add(s.config.SessionTTL)
	if err := s.repo.CreateSession(ctx, user.ID, tokenHash, expiresAt); err != nil {
		s.logger.Error("failed to create login session", "user_id", user.ID, "error", err)
		return nil, ErrInternal
	}

	principal, err := s.repo.GetPrincipalBySessionHash(ctx, tokenHash, s.now().UTC())
	if err != nil {
		s.logger.Error("failed to load login principal", "user_id", user.ID, "error", err)
		return nil, ErrInternal
	}

	return &AuthResult{
		Principal:        *principal,
		SessionToken:     rawToken,
		SessionExpiresAt: expiresAt,
	}, nil
}

func (s *Service) Authenticate(
	ctx context.Context,
	rawSessionToken string,
) (*Principal, error) {
	if strings.TrimSpace(rawSessionToken) == "" {
		return nil, ErrUnauthenticated
	}

	principal, err := s.repo.GetPrincipalBySessionHash(
		ctx,
		s.tokens.Hash(rawSessionToken),
		s.now().UTC(),
	)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrUnauthenticated
		}
		s.logger.Error("failed to authenticate session", "error", err)
		return nil, ErrInternal
	}
	if principal.User.DisabledAt != nil {
		return nil, ErrAccountDisabled
	}
	return principal, nil
}

func (s *Service) Logout(ctx context.Context, rawSessionToken string) error {
	if strings.TrimSpace(rawSessionToken) == "" {
		return nil
	}
	if err := s.repo.DeleteSession(ctx, s.tokens.Hash(rawSessionToken)); err != nil &&
		!errors.Is(err, ErrNotFound) {
		s.logger.Error("failed to delete session", "error", err)
		return ErrInternal
	}
	return nil
}

func (s *Service) RequestEmailVerification(
	ctx context.Context,
	userID int64,
) (string, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("failed to load user for email verification", "user_id", userID, "error", err)
		return "", ErrInternal
	}
	if user.EmailVerified() {
		return "", ErrEmailAlreadyVerified
	}
	return s.createAccountToken(
		ctx,
		user.ID,
		TokenPurposeVerifyEmail,
		user.Email,
		s.config.VerificationTTL,
	)
}

func (s *Service) VerifyEmail(ctx context.Context, rawToken string) error {
	if strings.TrimSpace(rawToken) == "" {
		return ErrInvalidToken
	}
	err := s.repo.ConsumeEmailVerification(
		ctx,
		s.tokens.Hash(rawToken),
		s.now().UTC(),
	)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrInvalidToken
		}
		s.logger.Error("failed to consume email verification token", "error", err)
		return ErrInternal
	}
	return nil
}

func (s *Service) RequestPasswordReset(
	ctx context.Context,
	email string,
) (string, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return "", nil
	}
	credential, err := s.repo.GetCredentialByEmail(ctx, normalizedEmail)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			s.logger.Error("failed to load password-reset user", "error", err)
		}
		return "", nil
	}
	user := &credential.User
	if user.DisabledAt != nil {
		return "", nil
	}

	return s.createAccountToken(
		ctx,
		user.ID,
		TokenPurposeResetPassword,
		"",
		s.config.PasswordResetTTL,
	)
}

func (s *Service) ResetPassword(
	ctx context.Context,
	rawToken string,
	newPassword string,
) error {
	if strings.TrimSpace(rawToken) == "" {
		return ErrInvalidToken
	}
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	passwordHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		s.logger.Error("failed to hash reset password", "error", err)
		return ErrInternal
	}

	err = s.repo.ConsumePasswordReset(
		ctx,
		s.tokens.Hash(rawToken),
		passwordHash,
		s.now().UTC(),
	)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrInvalidToken
		}
		s.logger.Error("failed to consume password reset token", "error", err)
		return ErrInternal
	}
	return nil
}

func (s *Service) LegacyTenantNeedsClaim(ctx context.Context) (bool, error) {
	needsClaim, err := s.repo.LegacyTenantNeedsClaim(ctx)
	if err != nil {
		s.logger.Error("failed to inspect legacy workspace claim state", "error", err)
		return false, ErrInternal
	}
	return needsClaim, nil
}

func (s *Service) LegacyTenantReady(ctx context.Context) (bool, error) {
	ready, err := s.repo.LegacyTenantReady(ctx)
	if err != nil {
		s.logger.Error("failed to inspect legacy workspace readiness", "error", err)
		return false, ErrInternal
	}
	return ready, nil
}

func (s *Service) BootstrapLegacyOwner(
	ctx context.Context,
	email string,
	password string,
) (*Principal, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return nil, err
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		s.logger.Error("failed to hash legacy bootstrap password", "error", err)
		return nil, ErrInternal
	}
	principal, err := s.repo.ClaimLegacyTenant(
		ctx,
		normalizedEmail,
		passwordHash,
		s.now().UTC(),
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound), errors.Is(err, ErrLegacyAlreadyClaimed):
			return nil, err
		case errors.Is(err, ErrConflict):
			return nil, ErrEmailAlreadyExists
		default:
			s.logger.Error("failed to claim legacy workspace", "error", err)
			return nil, ErrInternal
		}
	}
	return principal, nil
}

func (s *Service) createAccountToken(
	ctx context.Context,
	userID int64,
	purpose TokenPurpose,
	targetEmail string,
	ttl time.Duration,
) (string, error) {
	rawToken, tokenHash, err := s.tokens.Generate()
	if err != nil {
		return "", fmt.Errorf("generate account token: %w", err)
	}
	if err := s.repo.ReplaceAccountToken(
		ctx,
		userID,
		purpose,
		tokenHash,
		targetEmail,
		s.now().UTC().Add(ttl),
	); err != nil {
		return "", fmt.Errorf("persist account token: %w", err)
	}
	return rawToken, nil
}

func normalizeEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" || len(normalized) > maximumEmailBytes || !utf8.ValidString(normalized) {
		return "", ErrInvalidEmail
	}
	address, err := mail.ParseAddress(normalized)
	if err != nil || address.Address != normalized || !strings.Contains(normalized, "@") {
		return "", ErrInvalidEmail
	}
	return normalized, nil
}

func validatePassword(password string) error {
	length := len([]byte(password))
	if length < minimumPasswordBytes || length > maximumPasswordBytes {
		return ErrInvalidPassword
	}
	return nil
}

func defaultTenantName(email string) string {
	local, _, ok := strings.Cut(email, "@")
	if !ok || strings.TrimSpace(local) == "" {
		return "My workspace"
	}
	return local + "'s workspace"
}
