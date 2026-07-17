package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"
)

type principalContextKey struct{}

type Authenticator interface {
	Authenticate(ctx context.Context, rawSessionToken string) (*Principal, error)
}

type MiddlewareConfig struct {
	CookieName string
	Secure     bool
}

type Middleware struct {
	authenticator Authenticator
	cookieName    string
	secure        bool
}

func NewMiddleware(
	authenticator Authenticator,
	config MiddlewareConfig,
) *Middleware {
	if config.CookieName == "" {
		config.CookieName = "opportunity_radar_session"
	}
	return &Middleware{
		authenticator: authenticator,
		cookieName:    config.CookieName,
		secure:        config.Secure,
	}
}

func (m *Middleware) LoadPrincipal(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(m.cookieName)
		if err != nil || cookie.Value == "" {
			next.ServeHTTP(w, r)
			return
		}

		principal, err := m.authenticator.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			if errors.Is(err, ErrUnauthenticated) || errors.Is(err, ErrAccountDisabled) {
				m.ClearSessionCookie(w)
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "authentication temporarily unavailable", http.StatusServiceUnavailable)
			return
		}

		ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := PrincipalFromContext(r.Context()); !ok {
			target := "/login"
			if r.Method == http.MethodGet && r.URL.RequestURI() != "/" {
				target += "?next=" + url.QueryEscape(r.URL.RequestURI())
			}
			http.Redirect(w, r, target, http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) RequireVerified(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !principal.EmailVerified() {
			http.Redirect(w, r, "/verify-email/pending", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) RequireLegacyTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !principal.Tenant.IsLegacy {
			http.Redirect(w, r, "/account/pending", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) SetSessionCookie(
	w http.ResponseWriter,
	rawToken string,
	expiresAt time.Time,
) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    rawToken,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *Middleware) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *Middleware) SessionToken(r *http.Request) string {
	cookie, err := r.Cookie(m.cookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (m *Middleware) SecureCookies() bool {
	return m.secure
}

func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(*Principal)
	return principal, ok && principal != nil
}
