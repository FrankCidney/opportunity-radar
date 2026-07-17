package websecurity

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCSRFProtectIssuesSecureCookieAndExposesToken(t *testing.T) {
	t.Parallel()

	protection := newTestCSRF(t, true)
	var token string
	handler := protection.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token = CSRFToken(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if token == "" {
		t.Fatal("CSRF token was not placed in request context")
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie flags = HttpOnly:%v Secure:%v SameSite:%v", cookie.HttpOnly, cookie.Secure, cookie.SameSite)
	}
	if strings.Contains(cookie.Value, token) == false {
		t.Fatal("signed cookie does not contain the form token")
	}
}

func TestCSRFProtectAcceptsMatchingSignedCookieAndFormToken(t *testing.T) {
	t.Parallel()

	protection := newTestCSRF(t, false)
	cookie, token := issueCSRFCookie(t, protection)
	form := url.Values{CSRFFieldName(): {token}}
	request := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()

	protection.Protect(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestCSRFProtectRejectsMissingOrTamperedToken(t *testing.T) {
	t.Parallel()

	protection := newTestCSRF(t, false)
	cookie, _ := issueCSRFCookie(t, protection)

	tests := []struct {
		name string
		form url.Values
	}{
		{name: "missing", form: url.Values{}},
		{name: "tampered", form: url.Values{CSRFFieldName(): {"attacker-token"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(tt.form.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.AddCookie(cookie)
			recorder := httptest.NewRecorder()

			protection.Protect(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("protected handler was called")
			})).ServeHTTP(recorder, request)

			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
			}
		})
	}
}

func TestCSRFProtectRejectsOversizedFormBeforeHandler(t *testing.T) {
	t.Parallel()

	protection, err := NewCSRF(CSRFConfig{
		Key:          []byte("0123456789abcdef0123456789abcdef"),
		CookieName:   "test_csrf",
		MaxBodyBytes: 32,
	})
	if err != nil {
		t.Fatalf("NewCSRF() error = %v", err)
	}
	cookie, token := issueCSRFCookie(t, protection)
	form := url.Values{
		CSRFFieldName(): {token},
		"payload":       {strings.Repeat("a", 128)},
	}
	request := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()

	protection.Protect(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("protected handler was called")
	})).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestNewCSRFRejectsShortSigningKey(t *testing.T) {
	t.Parallel()

	if _, err := NewCSRF(CSRFConfig{Key: []byte("short")}); err == nil {
		t.Fatal("NewCSRF() accepted a short signing key")
	}
}

func newTestCSRF(t *testing.T, secure bool) *CSRF {
	t.Helper()
	protection, err := NewCSRF(CSRFConfig{
		Key:        []byte("0123456789abcdef0123456789abcdef"),
		CookieName: "test_csrf",
		Secure:     secure,
	})
	if err != nil {
		t.Fatalf("NewCSRF() error = %v", err)
	}
	return protection
}

func issueCSRFCookie(t *testing.T, protection *CSRF) (*http.Cookie, string) {
	t.Helper()
	var token string
	recorder := httptest.NewRecorder()
	protection.Protect(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		token = CSRFToken(r.Context())
	})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	return recorder.Result().Cookies()[0], token
}
