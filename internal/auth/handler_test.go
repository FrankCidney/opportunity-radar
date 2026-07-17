package auth

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRegisterSetsSessionCookieAndSendsVerification(t *testing.T) {
	t.Parallel()

	expiresAt := time.Now().Add(time.Hour)
	principal := testPrincipal("person@example.com")
	service := &stubAccountService{
		registerResult: &AuthResult{
			Principal:         principal,
			SessionToken:      "raw-session",
			SessionExpiresAt:  expiresAt,
			VerificationToken: "raw-verification",
		},
	}
	notifier := &stubAccountEmailNotifier{}
	middleware := NewMiddleware(
		&stubAuthenticator{},
		MiddlewareConfig{CookieName: "test_session", Secure: true},
	)
	handler := NewHandler(
		service,
		middleware,
		notifier,
		&allowAllLimiter{},
		HandlerConfig{RegistrationEnabled: true},
		authTestLogger(),
	)
	form := url.Values{
		"email":    {"person@example.com"},
		"password": {"correct horse battery staple"},
	}
	request := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()

	handler.Register(recorder, request)

	if recorder.Code != http.StatusSeeOther ||
		recorder.Header().Get("Location") != "/verify-email/pending" {
		t.Fatalf("response = %d %q, want verification redirect",
			recorder.Code, recorder.Header().Get("Location"))
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value != "raw-session" {
		t.Fatalf("session cookies = %#v", cookies)
	}
	if !cookies[0].HttpOnly || !cookies[0].Secure {
		t.Fatalf("session cookie flags = HttpOnly:%v Secure:%v", cookies[0].HttpOnly, cookies[0].Secure)
	}
	if notifier.verificationEmail != "person@example.com" ||
		notifier.verificationToken != "raw-verification" {
		t.Fatalf("verification notification = %q %q",
			notifier.verificationEmail, notifier.verificationToken)
	}
}

func TestLoginRejectsExternalNextRedirect(t *testing.T) {
	t.Parallel()

	verifiedAt := time.Now()
	principal := testPrincipal("person@example.com")
	principal.User.EmailVerifiedAt = &verifiedAt
	service := &stubAccountService{
		loginResult: &AuthResult{
			Principal:        principal,
			SessionToken:     "raw-session",
			SessionExpiresAt: time.Now().Add(time.Hour),
		},
	}
	handler := NewHandler(
		service,
		NewMiddleware(&stubAuthenticator{}, MiddlewareConfig{}),
		nil,
		&allowAllLimiter{},
		HandlerConfig{RegistrationEnabled: true},
		authTestLogger(),
	)
	form := url.Values{
		"email":    {"person@example.com"},
		"password": {"correct horse battery staple"},
		"next":     {"https://attacker.example/phish"},
	}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()

	handler.Login(recorder, request)

	if got := recorder.Header().Get("Location"); got != "/account/pending" {
		t.Fatalf("redirect = %q, want safe workspace destination", got)
	}
}

func TestForgotPasswordAlwaysUsesGenericConfirmation(t *testing.T) {
	t.Parallel()

	handler := NewHandler(
		&stubAccountService{},
		NewMiddleware(&stubAuthenticator{}, MiddlewareConfig{}),
		nil,
		&allowAllLimiter{},
		HandlerConfig{RegistrationEnabled: true},
		authTestLogger(),
	)
	form := url.Values{"email": {"missing@example.com"}}
	request := httptest.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()

	handler.ForgotPassword(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "If an account exists") {
		t.Fatalf("generic confirmation missing from body: %q", body)
	}
	if strings.Contains(body, "missing@example.com") {
		t.Fatal("confirmation disclosed submitted account identity")
	}
}

func TestSafeNextAllowsOnlyLocalPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "/settings/profile?tab=one", want: "/settings/profile?tab=one"},
		{input: "https://attacker.example", want: ""},
		{input: "//attacker.example", want: ""},
		{input: "settings/profile", want: ""},
	}
	for _, tt := range tests {
		if got := safeNext(tt.input); got != tt.want {
			t.Errorf("safeNext(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func authTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type allowAllLimiter struct{}

func (*allowAllLimiter) Allow(string, time.Time) bool {
	return true
}

type stubAccountEmailNotifier struct {
	verificationEmail string
	verificationToken string
	resetEmail        string
	resetToken        string
}

func (n *stubAccountEmailNotifier) SendVerification(
	_ context.Context,
	email string,
	rawToken string,
) error {
	n.verificationEmail = email
	n.verificationToken = rawToken
	return nil
}

func (n *stubAccountEmailNotifier) SendPasswordReset(
	_ context.Context,
	email string,
	rawToken string,
) error {
	n.resetEmail = email
	n.resetToken = rawToken
	return nil
}

type stubAccountService struct {
	registerResult *AuthResult
	registerErr    error
	loginResult    *AuthResult
	loginErr       error
	resetToken     string
}

func (s *stubAccountService) Register(context.Context, string, string) (*AuthResult, error) {
	return s.registerResult, s.registerErr
}

func (s *stubAccountService) Login(context.Context, string, string) (*AuthResult, error) {
	return s.loginResult, s.loginErr
}

func (*stubAccountService) Logout(context.Context, string) error {
	return nil
}

func (*stubAccountService) RequestEmailVerification(context.Context, int64) (string, error) {
	return "", nil
}

func (*stubAccountService) VerifyEmail(context.Context, string) error {
	return nil
}

func (s *stubAccountService) RequestPasswordReset(context.Context, string) (string, error) {
	return s.resetToken, nil
}

func (*stubAccountService) ResetPassword(context.Context, string, string) error {
	return nil
}
