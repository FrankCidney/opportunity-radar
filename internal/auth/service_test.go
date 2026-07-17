package auth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestRegisterNormalizesEmailAndCreatesHashedCredentials(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	repo := &stubRepository{
		registrationPrincipal: testPrincipal("person@example.com"),
	}
	hasher := &stubPasswordHasher{hash: "password-hash"}
	tokens := &sequenceTokenGenerator{tokens: []string{"session-raw", "verify-raw"}}
	service := testService(repo, hasher, tokens, now)

	result, err := service.Register(
		context.Background(),
		"  Person@Example.COM ",
		"correct horse battery staple",
	)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if repo.registrationEmail != "person@example.com" {
		t.Fatalf("registration email = %q, want normalized email", repo.registrationEmail)
	}
	if repo.registrationPasswordHash != "password-hash" {
		t.Fatalf("password hash = %q, want hasher output", repo.registrationPasswordHash)
	}
	if repo.registrationPasswordHash == "correct horse battery staple" {
		t.Fatal("repository received plaintext password")
	}
	if repo.registrationTenantName != "person's workspace" {
		t.Fatalf("tenant name = %q, want derived workspace name", repo.registrationTenantName)
	}
	if result.SessionToken != "session-raw" {
		t.Fatalf("session token = %q, want raw generated token", result.SessionToken)
	}
	if result.VerificationToken != "verify-raw" {
		t.Fatalf("verification token = %q, want raw generated token", result.VerificationToken)
	}
	if bytes.Equal(repo.sessionHash, []byte(result.SessionToken)) {
		t.Fatal("repository received raw session token instead of a hash")
	}
	if bytes.Equal(repo.accountTokenHash, []byte(result.VerificationToken)) {
		t.Fatal("repository received raw verification token instead of a hash")
	}
	if got, want := repo.sessionExpiresAt, now.Add(30*24*time.Hour); !got.Equal(want) {
		t.Fatalf("session expiry = %v, want %v", got, want)
	}
	if got, want := repo.accountTokenExpiresAt, now.Add(24*time.Hour); !got.Equal(want) {
		t.Fatalf("verification expiry = %v, want %v", got, want)
	}
}

func TestRegisterRejectsInvalidInputBeforePersistence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		email    string
		password string
		wantErr  error
	}{
		{
			name:     "invalid email",
			email:    "not-an-email",
			password: "correct horse battery staple",
			wantErr:  ErrInvalidEmail,
		},
		{
			name:     "short password",
			email:    "person@example.com",
			password: "too-short",
			wantErr:  ErrInvalidPassword,
		},
		{
			name:     "bcrypt length limit",
			email:    "person@example.com",
			password: string(bytes.Repeat([]byte("a"), maximumPasswordBytes+1)),
			wantErr:  ErrInvalidPassword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &stubRepository{}
			service := testService(
				repo,
				&stubPasswordHasher{hash: "hash"},
				&sequenceTokenGenerator{},
				time.Now(),
			)
			_, err := service.Register(context.Background(), tt.email, tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Register() error = %v, want %v", err, tt.wantErr)
			}
			if repo.createRegistrationCalled {
				t.Fatal("invalid registration reached repository")
			}
		})
	}
}

func TestRegisterTranslatesUniqueConflict(t *testing.T) {
	t.Parallel()

	repo := &stubRepository{createRegistrationErr: ErrConflict}
	service := testService(
		repo,
		&stubPasswordHasher{hash: "hash"},
		&sequenceTokenGenerator{tokens: []string{"session-raw", "verify-raw"}},
		time.Now(),
	)

	_, err := service.Register(
		context.Background(),
		"person@example.com",
		"correct horse battery staple",
	)
	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("Register() error = %v, want %v", err, ErrEmailAlreadyExists)
	}
}

func TestLoginUsesGenericCredentialFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		repo   *stubRepository
		hasher *stubPasswordHasher
		email  string
	}{
		{
			name:   "unknown email",
			repo:   &stubRepository{getUserByEmailErr: ErrNotFound},
			hasher: &stubPasswordHasher{},
			email:  "missing@example.com",
		},
		{
			name: "wrong password",
			repo: &stubRepository{
				credentialByEmail: &Credential{
					User:         User{ID: 1, Email: "person@example.com"},
					PasswordHash: "hash",
				},
			},
			hasher: &stubPasswordHasher{compareErr: errors.New("mismatch")},
			email:  "person@example.com",
		},
		{
			name:   "invalid email syntax",
			repo:   &stubRepository{},
			hasher: &stubPasswordHasher{},
			email:  "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := testService(tt.repo, tt.hasher, &sequenceTokenGenerator{}, time.Now())
			_, err := service.Login(context.Background(), tt.email, "wrong password")
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("Login() error = %v, want generic credentials error", err)
			}
		})
	}
}

func TestLoginPerformsPasswordComparisonForUnknownEmail(t *testing.T) {
	t.Parallel()

	hasher := &stubPasswordHasher{hash: "dummy-hash"}
	service := testService(
		&stubRepository{getUserByEmailErr: ErrNotFound},
		hasher,
		&sequenceTokenGenerator{},
		time.Now(),
	)

	_, err := service.Login(
		context.Background(),
		"missing@example.com",
		"candidate password",
	)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want %v", err, ErrInvalidCredentials)
	}
	if hasher.compareCalls != 1 {
		t.Fatalf("password comparisons = %d, want timing-equalizing comparison", hasher.compareCalls)
	}
}

func TestLoginCreatesHashedSessionAndReturnsPrincipal(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	principal := testPrincipal("person@example.com")
	repo := &stubRepository{
		credentialByEmail: &Credential{User: principal.User, PasswordHash: "stored-hash"},
		sessionPrincipal:  &principal,
	}
	service := testService(
		repo,
		&stubPasswordHasher{},
		&sequenceTokenGenerator{tokens: []string{"login-raw"}},
		now,
	)

	result, err := service.Login(
		context.Background(),
		"person@example.com",
		"correct horse battery staple",
	)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.Principal.User.ID != principal.User.ID {
		t.Fatalf("principal user ID = %d, want %d", result.Principal.User.ID, principal.User.ID)
	}
	if bytes.Equal(repo.sessionHash, []byte(result.SessionToken)) {
		t.Fatal("repository received raw login token instead of hash")
	}
	if repo.sessionUserID != principal.User.ID {
		t.Fatalf("session user ID = %d, want %d", repo.sessionUserID, principal.User.ID)
	}
}

func TestAuthenticateHashesCookieToken(t *testing.T) {
	t.Parallel()

	principal := testPrincipal("person@example.com")
	repo := &stubRepository{sessionPrincipal: &principal}
	tokens := &sequenceTokenGenerator{}
	service := testService(repo, &stubPasswordHasher{}, tokens, time.Now())

	got, err := service.Authenticate(context.Background(), "cookie-raw")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if got.User.ID != principal.User.ID {
		t.Fatalf("principal user ID = %d, want %d", got.User.ID, principal.User.ID)
	}
	if bytes.Equal(repo.lookupSessionHash, []byte("cookie-raw")) {
		t.Fatal("repository received raw cookie token instead of hash")
	}
}

func TestRequestPasswordResetDoesNotRevealAccountExistence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		email string
		repo  *stubRepository
	}{
		{
			name:  "invalid email",
			email: "invalid",
			repo:  &stubRepository{},
		},
		{
			name:  "unknown email",
			email: "missing@example.com",
			repo:  &stubRepository{getUserByEmailErr: ErrNotFound},
		},
		{
			name:  "disabled user",
			email: "disabled@example.com",
			repo: &stubRepository{credentialByEmail: &Credential{
				User: User{
					ID:         1,
					Email:      "disabled@example.com",
					DisabledAt: ptrTime(time.Now()),
				},
				PasswordHash: "stored-hash",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := testService(
				tt.repo,
				&stubPasswordHasher{},
				&sequenceTokenGenerator{},
				time.Now(),
			)
			token, err := service.RequestPasswordReset(context.Background(), tt.email)
			if err != nil {
				t.Fatalf("RequestPasswordReset() error = %v, want nil generic response", err)
			}
			if token != "" {
				t.Fatalf("token = %q, want empty", token)
			}
		})
	}
}

func TestResetPasswordHashesPasswordAndConsumesHashedToken(t *testing.T) {
	t.Parallel()

	repo := &stubRepository{}
	service := testService(
		repo,
		&stubPasswordHasher{hash: "new-password-hash"},
		&sequenceTokenGenerator{},
		time.Now(),
	)

	if err := service.ResetPassword(
		context.Background(),
		"reset-raw",
		"a new secure password",
	); err != nil {
		t.Fatalf("ResetPassword() error = %v", err)
	}
	if repo.resetPasswordHash != "new-password-hash" {
		t.Fatalf("password hash = %q, want hasher output", repo.resetPasswordHash)
	}
	if bytes.Equal(repo.resetTokenHash, []byte("reset-raw")) {
		t.Fatal("repository received raw reset token instead of hash")
	}
}

func TestBcryptHasherDoesNotStorePlaintextAndVerifiesPassword(t *testing.T) {
	t.Parallel()

	hasher := BcryptHasher{Cost: 4}
	password := "correct horse battery staple"
	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if hash == password || bytes.Contains([]byte(hash), []byte(password)) {
		t.Fatal("bcrypt hash contains plaintext password")
	}
	if err := hasher.Compare(hash, password); err != nil {
		t.Fatalf("Compare() correct password error = %v", err)
	}
	if err := hasher.Compare(hash, "incorrect password"); err == nil {
		t.Fatal("Compare() accepted incorrect password")
	}
}

func TestSecureTokenGeneratorProducesOpaqueHashedTokens(t *testing.T) {
	t.Parallel()

	generator := SecureTokenGenerator{}
	firstRaw, firstHash, err := generator.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	secondRaw, secondHash, err := generator.Generate()
	if err != nil {
		t.Fatalf("Generate() second error = %v", err)
	}

	if firstRaw == secondRaw {
		t.Fatal("generated tokens are equal")
	}
	if len(firstHash) != 32 || len(secondHash) != 32 {
		t.Fatalf("hash lengths = %d and %d, want 32", len(firstHash), len(secondHash))
	}
	if bytes.Equal(firstHash, []byte(firstRaw)) {
		t.Fatal("generated token hash equals raw token")
	}
	if !bytes.Equal(firstHash, generator.Hash(firstRaw)) {
		t.Fatal("Hash() does not reproduce generated token hash")
	}
}

func TestBootstrapLegacyOwnerCreatesVerifiedOwnerWithoutPlaintextPassword(t *testing.T) {
	t.Parallel()

	principal := testPrincipal("owner@example.com")
	verifiedAt := time.Now()
	principal.User.EmailVerifiedAt = &verifiedAt
	principal.Tenant.IsLegacy = true
	repo := &stubRepository{legacyPrincipal: &principal}
	service := testService(
		repo,
		&stubPasswordHasher{hash: "bootstrap-password-hash"},
		&sequenceTokenGenerator{},
		time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC),
	)

	got, err := service.BootstrapLegacyOwner(
		context.Background(),
		" Owner@Example.com ",
		"correct horse battery staple",
	)
	if err != nil {
		t.Fatalf("BootstrapLegacyOwner() error = %v", err)
	}
	if !got.Tenant.IsLegacy {
		t.Fatal("bootstrap did not return legacy tenant")
	}
	if repo.legacyEmail != "owner@example.com" {
		t.Fatalf("legacy email = %q, want normalized email", repo.legacyEmail)
	}
	if repo.legacyPasswordHash != "bootstrap-password-hash" {
		t.Fatalf("legacy password hash = %q, want hasher output", repo.legacyPasswordHash)
	}
	if repo.legacyPasswordHash == "correct horse battery staple" {
		t.Fatal("repository received plaintext bootstrap password")
	}
}

func testService(
	repo Repository,
	hasher PasswordHasher,
	tokens TokenGenerator,
	now time.Time,
) *Service {
	service := NewService(repo, hasher, tokens, ServiceConfig{}, slog.New(
		slog.NewTextHandler(io.Discard, nil),
	))
	service.now = func() time.Time { return now }
	return service
}

func testPrincipal(email string) Principal {
	return Principal{
		User: User{
			ID:    11,
			Email: email,
		},
		Tenant: Tenant{
			ID:   22,
			Name: "Personal workspace",
		},
		Membership: Membership{
			TenantID: 22,
			UserID:   11,
			Role:     RoleOwner,
		},
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

type stubPasswordHasher struct {
	hash         string
	hashErr      error
	compareErr   error
	compareCalls int
}

func (h *stubPasswordHasher) Hash(string) (string, error) {
	return h.hash, h.hashErr
}

func (h *stubPasswordHasher) Compare(string, string) error {
	h.compareCalls++
	return h.compareErr
}

type sequenceTokenGenerator struct {
	tokens []string
	index  int
}

func (g *sequenceTokenGenerator) Generate() (string, []byte, error) {
	if g.index >= len(g.tokens) {
		return "", nil, errors.New("no test token configured")
	}
	raw := g.tokens[g.index]
	g.index++
	return raw, hashToken(raw), nil
}

func (*sequenceTokenGenerator) Hash(raw string) []byte {
	return hashToken(raw)
}

type stubRepository struct {
	createRegistrationCalled bool
	registrationEmail        string
	registrationPasswordHash string
	registrationTenantName   string
	registrationPrincipal    Principal
	createRegistrationErr    error

	credentialByEmail *Credential
	getUserByEmailErr error
	userByID          *User
	getUserByIDErr    error

	sessionUserID     int64
	sessionHash       []byte
	sessionExpiresAt  time.Time
	createSessionErr  error
	sessionPrincipal  *Principal
	lookupSessionHash []byte
	getSessionErr     error
	deleteSessionHash []byte
	deleteSessionErr  error

	accountTokenUserID    int64
	accountTokenPurpose   TokenPurpose
	accountTokenHash      []byte
	accountTokenTarget    string
	accountTokenExpiresAt time.Time
	accountTokenErr       error

	verifyTokenHash []byte
	verifyErr       error

	resetTokenHash    []byte
	resetPasswordHash string
	resetErr          error

	legacyNeedsClaim   bool
	legacyNeedsErr     error
	legacyReady        bool
	legacyReadyErr     error
	legacyPrincipal    *Principal
	legacyClaimErr     error
	legacyEmail        string
	legacyPasswordHash string
}

func (r *stubRepository) CreateRegistration(
	_ context.Context,
	registration Registration,
) (*Principal, error) {
	r.createRegistrationCalled = true
	r.registrationEmail = registration.Email
	r.registrationPasswordHash = registration.PasswordHash
	r.registrationTenantName = registration.TenantName
	r.sessionHash = registration.SessionTokenHash
	r.sessionExpiresAt = registration.SessionExpiresAt
	r.accountTokenHash = registration.VerificationTokenHash
	r.accountTokenExpiresAt = registration.VerificationExpiresAt
	if r.createRegistrationErr != nil {
		return nil, r.createRegistrationErr
	}
	principal := r.registrationPrincipal
	return &principal, nil
}

func (r *stubRepository) GetCredentialByEmail(_ context.Context, _ string) (*Credential, error) {
	if r.getUserByEmailErr != nil {
		return nil, r.getUserByEmailErr
	}
	return r.credentialByEmail, nil
}

func (r *stubRepository) GetUserByID(_ context.Context, _ int64) (*User, error) {
	if r.getUserByIDErr != nil {
		return nil, r.getUserByIDErr
	}
	return r.userByID, nil
}

func (r *stubRepository) CreateSession(
	_ context.Context,
	userID int64,
	tokenHash []byte,
	expiresAt time.Time,
) error {
	r.sessionUserID = userID
	r.sessionHash = tokenHash
	r.sessionExpiresAt = expiresAt
	return r.createSessionErr
}

func (r *stubRepository) GetPrincipalBySessionHash(
	_ context.Context,
	tokenHash []byte,
	_ time.Time,
) (*Principal, error) {
	r.lookupSessionHash = tokenHash
	if r.getSessionErr != nil {
		return nil, r.getSessionErr
	}
	return r.sessionPrincipal, nil
}

func (r *stubRepository) DeleteSession(_ context.Context, tokenHash []byte) error {
	r.deleteSessionHash = tokenHash
	return r.deleteSessionErr
}

func (r *stubRepository) ReplaceAccountToken(
	_ context.Context,
	userID int64,
	purpose TokenPurpose,
	tokenHash []byte,
	targetEmail string,
	expiresAt time.Time,
) error {
	r.accountTokenUserID = userID
	r.accountTokenPurpose = purpose
	r.accountTokenHash = tokenHash
	r.accountTokenTarget = targetEmail
	r.accountTokenExpiresAt = expiresAt
	return r.accountTokenErr
}

func (r *stubRepository) ConsumeEmailVerification(
	_ context.Context,
	tokenHash []byte,
	_ time.Time,
) error {
	r.verifyTokenHash = tokenHash
	return r.verifyErr
}

func (r *stubRepository) ConsumePasswordReset(
	_ context.Context,
	tokenHash []byte,
	passwordHash string,
	_ time.Time,
) error {
	r.resetTokenHash = tokenHash
	r.resetPasswordHash = passwordHash
	return r.resetErr
}

func (r *stubRepository) LegacyTenantNeedsClaim(context.Context) (bool, error) {
	return r.legacyNeedsClaim, r.legacyNeedsErr
}

func (r *stubRepository) LegacyTenantReady(context.Context) (bool, error) {
	return r.legacyReady, r.legacyReadyErr
}

func (r *stubRepository) ClaimLegacyTenant(
	_ context.Context,
	email string,
	passwordHash string,
	_ time.Time,
) (*Principal, error) {
	r.legacyEmail = email
	r.legacyPasswordHash = passwordHash
	return r.legacyPrincipal, r.legacyClaimErr
}
