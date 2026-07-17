package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoadPrincipalStoresTypedPrincipalInContext(t *testing.T) {
	t.Parallel()

	principal := testPrincipal("person@example.com")
	middleware := NewMiddleware(
		&stubAuthenticator{principal: &principal},
		MiddlewareConfig{CookieName: "test_session", Secure: true},
	)
	handler := middleware.LoadPrincipal(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := PrincipalFromContext(r.Context())
		if !ok || got.User.ID != principal.User.ID {
			t.Fatalf("principal = %#v, want user %d", got, principal.User.ID)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "test_session", Value: "raw-session"})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if middleware.authenticator.(*stubAuthenticator).rawToken != "raw-session" {
		t.Fatal("authenticator did not receive session cookie")
	}
}

func TestLoadPrincipalClearsInvalidSession(t *testing.T) {
	t.Parallel()

	middleware := NewMiddleware(
		&stubAuthenticator{err: ErrUnauthenticated},
		MiddlewareConfig{CookieName: "test_session", Secure: true},
	)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "test_session", Value: "invalid"})
	recorder := httptest.NewRecorder()

	middleware.LoadPrincipal(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want downstream request", recorder.Code)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge != -1 {
		t.Fatalf("cleared cookies = %#v, want expired session", cookies)
	}
}

func TestRequireAuthRedirectsWithoutLeakingExternalNextURL(t *testing.T) {
	t.Parallel()

	middleware := NewMiddleware(&stubAuthenticator{}, MiddlewareConfig{})
	request := httptest.NewRequest(http.MethodGet, "https://app.example/settings/profile?tab=one", nil)
	recorder := httptest.NewRecorder()

	middleware.RequireAuth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("protected handler was called")
	})).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	location := recorder.Header().Get("Location")
	if location != "/login?next=%2Fsettings%2Fprofile%3Ftab%3Done" {
		t.Fatalf("redirect = %q, want local request URI", location)
	}
}

func TestRequireVerifiedRejectsUnverifiedPrincipal(t *testing.T) {
	t.Parallel()

	principal := testPrincipal("person@example.com")
	request := httptest.NewRequest(http.MethodPost, "/run-once", nil)
	request = request.WithContext(context.WithValue(
		request.Context(),
		principalContextKey{},
		&principal,
	))
	recorder := httptest.NewRecorder()
	middleware := NewMiddleware(&stubAuthenticator{}, MiddlewareConfig{})

	middleware.RequireVerified(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("verified handler was called")
	})).ServeHTTP(recorder, request)

	if recorder.Header().Get("Location") != "/verify-email/pending" {
		t.Fatalf("redirect = %q, want verification pending", recorder.Header().Get("Location"))
	}
}

func TestSessionCookieUsesProductionSecurityFlags(t *testing.T) {
	t.Parallel()

	middleware := NewMiddleware(
		&stubAuthenticator{},
		MiddlewareConfig{CookieName: "test_session", Secure: true},
	)
	recorder := httptest.NewRecorder()
	middleware.SetSessionCookie(recorder, "raw-session", time.Now().Add(time.Hour))

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie flags = HttpOnly:%v Secure:%v SameSite:%v", cookie.HttpOnly, cookie.Secure, cookie.SameSite)
	}
}

func TestLoadPrincipalReturnsServiceUnavailableOnRepositoryFailure(t *testing.T) {
	t.Parallel()

	middleware := NewMiddleware(
		&stubAuthenticator{err: errors.New("database unavailable")},
		MiddlewareConfig{CookieName: "test_session"},
	)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "test_session", Value: "session"})
	recorder := httptest.NewRecorder()

	middleware.LoadPrincipal(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("downstream handler was called")
	})).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

type stubAuthenticator struct {
	principal *Principal
	err       error
	rawToken  string
}

func (a *stubAuthenticator) Authenticate(
	_ context.Context,
	rawSessionToken string,
) (*Principal, error) {
	a.rawToken = rawSessionToken
	return a.principal, a.err
}
