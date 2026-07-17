package auth

import (
	"context"
	"embed"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"opportunity-radar/internal/shared/websecurity"
)

//go:embed templates/*.html
var authTemplatesFS embed.FS

type AccountService interface {
	Register(ctx context.Context, email, password string) (*AuthResult, error)
	Login(ctx context.Context, email, password string) (*AuthResult, error)
	Logout(ctx context.Context, rawSessionToken string) error
	RequestEmailVerification(ctx context.Context, userID int64) (string, error)
	VerifyEmail(ctx context.Context, rawToken string) error
	RequestPasswordReset(ctx context.Context, email string) (string, error)
	ResetPassword(ctx context.Context, rawToken, newPassword string) error
}

type AccountEmailNotifier interface {
	SendVerification(ctx context.Context, email, rawToken string) error
	SendPasswordReset(ctx context.Context, email, rawToken string) error
}

type HandlerConfig struct {
	RegistrationEnabled bool
	TrustProxyHeaders   bool
}

type Handler struct {
	service    AccountService
	middleware *Middleware
	notifier   AccountEmailNotifier
	limiter    AttemptLimiter
	config     HandlerConfig
	templates  *template.Template
	logger     *slog.Logger
	now        func() time.Time
}

func NewHandler(
	service AccountService,
	middleware *Middleware,
	notifier AccountEmailNotifier,
	limiter AttemptLimiter,
	config HandlerConfig,
	logger *slog.Logger,
) *Handler {
	if limiter == nil {
		limiter = NewMemoryRateLimiter(RateLimiterConfig{})
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		service:    service,
		middleware: middleware,
		notifier:   notifier,
		limiter:    limiter,
		config:     config,
		templates:  template.Must(template.ParseFS(authTemplatesFS, "templates/*.html")),
		logger:     logger,
		now:        time.Now,
	}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if principal, ok := PrincipalFromContext(r.Context()); ok {
		http.Redirect(w, r, destinationForPrincipal(principal), http.StatusSeeOther)
		return
	}
	if !h.config.RegistrationEnabled {
		http.Error(w, "registration is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodGet {
		h.render(w, "register.html", authPageData{
			Title:     "Create your account",
			CSRFToken: websecurity.CSRFToken(r.Context()),
		})
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.limiter.Allow("register:ip:"+requestIP(r, h.config.TrustProxyHeaders), h.now()) {
		h.renderStatus(w, "register.html", authPageData{
			Title:     "Create your account",
			Email:     r.FormValue("email"),
			Error:     "Too many registration attempts. Please wait and try again.",
			CSRFToken: websecurity.CSRFToken(r.Context()),
		}, http.StatusTooManyRequests)
		return
	}

	result, err := h.service.Register(
		r.Context(),
		r.FormValue("email"),
		r.FormValue("password"),
	)
	if err != nil {
		message := "We could not create your account. Please try again."
		switch {
		case errors.Is(err, ErrInvalidEmail):
			message = "Enter a valid email address."
		case errors.Is(err, ErrInvalidPassword):
			message = "Use a password between 12 and 72 characters."
		case errors.Is(err, ErrEmailAlreadyExists):
			message = "An account with that email already exists."
		}
		h.renderStatus(w, "register.html", authPageData{
			Title:     "Create your account",
			Email:     r.FormValue("email"),
			Error:     message,
			CSRFToken: websecurity.CSRFToken(r.Context()),
		}, http.StatusUnprocessableEntity)
		return
	}

	h.middleware.SetSessionCookie(w, result.SessionToken, result.SessionExpiresAt)
	if result.VerificationToken != "" && h.notifier != nil {
		if err := h.notifier.SendVerification(
			r.Context(),
			result.Principal.User.Email,
			result.VerificationToken,
		); err != nil {
			h.logger.Error("failed to send registration verification email",
				"user_id", result.Principal.User.ID,
				"error", err,
			)
		}
	}
	http.Redirect(w, r, "/verify-email/pending", http.StatusSeeOther)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if principal, ok := PrincipalFromContext(r.Context()); ok {
			http.Redirect(w, r, destinationForPrincipal(principal), http.StatusSeeOther)
			return
		}
		h.render(w, "login.html", authPageData{
			Title:     "Sign in",
			Next:      safeNext(r.URL.Query().Get("next")),
			Flash:     r.URL.Query().Get("flash"),
			CSRFToken: websecurity.CSRFToken(r.Context()),
		})
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	email := r.FormValue("email")
	normalized, _ := normalizeEmail(email)
	now := h.now()
	if !h.limiter.Allow("login:ip:"+requestIP(r, h.config.TrustProxyHeaders), now) ||
		!h.limiter.Allow("login:email:"+normalized, now) {
		h.renderStatus(w, "login.html", authPageData{
			Title:     "Sign in",
			Email:     email,
			Next:      safeNext(r.FormValue("next")),
			Error:     "Too many sign-in attempts. Please wait and try again.",
			CSRFToken: websecurity.CSRFToken(r.Context()),
		}, http.StatusTooManyRequests)
		return
	}

	result, err := h.service.Login(r.Context(), email, r.FormValue("password"))
	if err != nil {
		message := "The email or password is incorrect."
		if errors.Is(err, ErrAccountDisabled) {
			message = "This account is unavailable."
		} else if errors.Is(err, ErrInternal) {
			message = "Sign-in is temporarily unavailable. Please try again."
		}
		h.renderStatus(w, "login.html", authPageData{
			Title:     "Sign in",
			Email:     email,
			Next:      safeNext(r.FormValue("next")),
			Error:     message,
			CSRFToken: websecurity.CSRFToken(r.Context()),
		}, http.StatusUnprocessableEntity)
		return
	}

	h.middleware.SetSessionCookie(w, result.SessionToken, result.SessionExpiresAt)
	next := safeNext(r.FormValue("next"))
	if next == "" {
		next = destinationForPrincipal(&result.Principal)
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.service.Logout(r.Context(), h.middleware.SessionToken(r)); err != nil {
		http.Error(w, "could not sign out", http.StatusServiceUnavailable)
		return
	}
	h.middleware.ClearSessionCookie(w)
	http.Redirect(w, r, "/login?flash=You+have+been+signed+out.", http.StatusSeeOther)
}

func (h *Handler) VerificationPending(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	if principal.EmailVerified() {
		http.Redirect(w, r, destinationForPrincipal(principal), http.StatusSeeOther)
		return
	}
	h.render(w, "verification_pending.html", authPageData{
		Title:     "Verify your email",
		Email:     principal.User.Email,
		Flash:     r.URL.Query().Get("flash"),
		CSRFToken: websecurity.CSRFToken(r.Context()),
	})
}

func (h *Handler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	if !h.limiter.Allow("verify:user:"+strconv.FormatInt(principal.User.ID, 10), h.now()) {
		http.Redirect(w, r, "/verify-email/pending?flash=Please+wait+before+requesting+another+email.", http.StatusSeeOther)
		return
	}
	token, err := h.service.RequestEmailVerification(r.Context(), principal.User.ID)
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyVerified) {
			http.Redirect(w, r, destinationForPrincipal(principal), http.StatusSeeOther)
			return
		}
		http.Error(w, "verification email temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	if h.notifier != nil {
		if err := h.notifier.SendVerification(r.Context(), principal.User.Email, token); err != nil {
			h.logger.Error("failed to resend verification email", "user_id", principal.User.ID, "error", err)
			http.Error(w, "verification email temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
	}
	http.Redirect(w, r, "/verify-email/pending?flash=Verification+email+sent.", http.StatusSeeOther)
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if r.Method == http.MethodGet {
		h.render(w, "verify_email.html", authPageData{
			Title:     "Confirm your email",
			Token:     token,
			CSRFToken: websecurity.CSRFToken(r.Context()),
		})
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.service.VerifyEmail(r.Context(), r.FormValue("token")); err != nil {
		h.renderStatus(w, "verify_email.html", authPageData{
			Title:     "Confirm your email",
			Error:     "This verification link is invalid or has expired.",
			CSRFToken: websecurity.CSRFToken(r.Context()),
		}, http.StatusUnprocessableEntity)
		return
	}
	http.Redirect(w, r, "/login?flash=Email+verified.+You+can+continue.", http.StatusSeeOther)
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.render(w, "forgot_password.html", authPageData{
			Title:     "Reset your password",
			CSRFToken: websecurity.CSRFToken(r.Context()),
		})
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	email := r.FormValue("email")
	if h.limiter.Allow("reset:ip:"+requestIP(r, h.config.TrustProxyHeaders), h.now()) {
		token, _ := h.service.RequestPasswordReset(r.Context(), email)
		if token != "" && h.notifier != nil {
			if normalized, err := normalizeEmail(email); err == nil {
				if err := h.notifier.SendPasswordReset(r.Context(), normalized, token); err != nil {
					h.logger.Error("failed to send password reset email", "error", err)
				}
			}
		}
	}
	h.render(w, "forgot_password_sent.html", authPageData{
		Title: "Check your email",
	})
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if r.Method == http.MethodGet {
		h.render(w, "reset_password.html", authPageData{
			Title:     "Choose a new password",
			Token:     token,
			CSRFToken: websecurity.CSRFToken(r.Context()),
		})
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.limiter.Allow("password-reset:ip:"+requestIP(r, h.config.TrustProxyHeaders), h.now()) {
		h.renderStatus(w, "reset_password.html", authPageData{
			Title:     "Choose a new password",
			Token:     r.FormValue("token"),
			Error:     "Too many reset attempts. Please wait and request a new link if needed.",
			CSRFToken: websecurity.CSRFToken(r.Context()),
		}, http.StatusTooManyRequests)
		return
	}
	err := h.service.ResetPassword(
		r.Context(),
		r.FormValue("token"),
		r.FormValue("password"),
	)
	if err != nil {
		message := "This reset link is invalid or has expired."
		if errors.Is(err, ErrInvalidPassword) {
			message = "Use a password between 12 and 72 characters."
		}
		h.renderStatus(w, "reset_password.html", authPageData{
			Title:     "Choose a new password",
			Token:     r.FormValue("token"),
			Error:     message,
			CSRFToken: websecurity.CSRFToken(r.Context()),
		}, http.StatusUnprocessableEntity)
		return
	}
	h.middleware.ClearSessionCookie(w)
	http.Redirect(w, r, "/login?flash=Password+updated.+Sign+in+again.", http.StatusSeeOther)
}

func (h *Handler) AccountPending(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	h.render(w, "account_pending.html", authPageData{
		Title:      "Your workspace is ready",
		Email:      principal.User.Email,
		TenantName: principal.Tenant.Name,
		CSRFToken:  websecurity.CSRFToken(r.Context()),
	})
}

func (h *Handler) render(w http.ResponseWriter, name string, data authPageData) {
	h.renderStatus(w, name, data, http.StatusOK)
}

func (h *Handler) renderStatus(
	w http.ResponseWriter,
	name string,
	data authPageData,
	status int,
) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		h.logger.Error("failed to render auth template", "template", name, "error", err)
	}
}

func destinationForPrincipal(principal *Principal) string {
	if principal == nil || !principal.EmailVerified() {
		return "/verify-email/pending"
	}
	if principal.Tenant.IsLegacy {
		return "/"
	}
	return "/account/pending"
}

func safeNext(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return ""
	}
	return value
}

type authPageData struct {
	Title      string
	Email      string
	TenantName string
	Next       string
	Token      string
	Error      string
	Flash      string
	CSRFToken  string
}
