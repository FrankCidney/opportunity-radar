package websecurity

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"
)

const (
	csrfTokenBytes = 32
	csrfFieldName  = "_csrf"
)

var ErrInvalidCSRFKey = errors.New("CSRF signing key must contain at least 32 bytes")

type csrfContextKey struct{}

type CSRFConfig struct {
	Key        []byte
	CookieName string
	Secure     bool
	MaxAge     time.Duration
}

type CSRF struct {
	key        []byte
	cookieName string
	secure     bool
	maxAge     time.Duration
}

func NewCSRF(config CSRFConfig) (*CSRF, error) {
	if len(config.Key) < 32 {
		return nil, ErrInvalidCSRFKey
	}
	if config.CookieName == "" {
		config.CookieName = "opportunity_radar_csrf"
	}
	if config.MaxAge <= 0 {
		config.MaxAge = 12 * time.Hour
	}

	key := make([]byte, len(config.Key))
	copy(key, config.Key)
	return &CSRF{
		key:        key,
		cookieName: config.CookieName,
		secure:     config.Secure,
		maxAge:     config.MaxAge,
	}, nil
}

func (c *CSRF) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, validCookie := c.readCookie(r)
		if !validCookie {
			var err error
			token, err = newCSRFToken()
			if err != nil {
				http.Error(w, "security token unavailable", http.StatusInternalServerError)
				return
			}
			c.setCookie(w, token)
		}

		if isUnsafeMethod(r.Method) {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form submission", http.StatusBadRequest)
				return
			}
			submitted := r.PostForm.Get(csrfFieldName)
			if !validCookie || !constantTimeEqual(token, submitted) {
				http.Error(w, "invalid or expired security token", http.StatusForbidden)
				return
			}
		}

		ctx := context.WithValue(r.Context(), csrfContextKey{}, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func CSRFToken(ctx context.Context) string {
	token, _ := ctx.Value(csrfContextKey{}).(string)
	return token
}

func CSRFFieldName() string {
	return csrfFieldName
}

func (c *CSRF) readCookie(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(c.cookieName)
	if err != nil {
		return "", false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	expected := c.signature(parts[0])
	actual, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(expected, actual) {
		return "", false
	}
	return parts[0], true
}

func (c *CSRF) setCookie(w http.ResponseWriter, token string) {
	signature := base64.RawURLEncoding.EncodeToString(c.signature(token))
	http.SetCookie(w, &http.Cookie{
		Name:     c.cookieName,
		Value:    token + "." + signature,
		Path:     "/",
		MaxAge:   int(c.maxAge.Seconds()),
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (c *CSRF) signature(token string) []byte {
	mac := hmac.New(sha256.New, c.key)
	_, _ = mac.Write([]byte(token))
	return mac.Sum(nil)
}

func newCSRFToken() (string, error) {
	random := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
}

func constantTimeEqual(first, second string) bool {
	if len(first) != len(second) {
		return false
	}
	return hmac.Equal([]byte(first), []byte(second))
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}
