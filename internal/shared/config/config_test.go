package config

import "testing"

func TestLoadReturnsErrorWhenDatabaseURLMissing(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected missing DATABASE_URL to return an error")
	}
}

func TestLoadReturnsErrorWhenBoolInvalid(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("SCHEDULER_ENABLED", "maybe")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected invalid bool env to return an error")
	}
}

func TestLoadReturnsDefaultsForOptionalValues(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("SCHEDULER_ENABLED", "")
	t.Setenv("SCHEDULER_INTERVAL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected load to succeed: %v", err)
	}

	if !cfg.SchedulerEnabled {
		t.Fatalf("expected scheduler enabled default to be true")
	}
	if !cfg.RegistrationEnabled {
		t.Fatal("expected open registration by default")
	}
	if len(cfg.AuthCSRFKey) < 32 {
		t.Fatal("expected development CSRF key fallback")
	}
	if cfg.PublicBaseURL != "http://localhost:8080" {
		t.Fatalf("public base URL = %q, want local default", cfg.PublicBaseURL)
	}
}

func TestLoadRequiresProductionAuthenticationSecrets(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("AUTH_CSRF_KEY", "")
	t.Setenv("PUBLIC_BASE_URL", "")
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("RESEND_FROM_EMAIL", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted production config without authentication secrets")
	}
}

func TestLoadAcceptsSecureProductionAuthenticationConfig(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("AUTH_CSRF_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("PUBLIC_BASE_URL", "https://radar.example.com")
	t.Setenv("RESEND_API_KEY", "secret")
	t.Setenv("RESEND_FROM_EMAIL", "updates@example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.PublicBaseURL != "https://radar.example.com" {
		t.Fatalf("public base URL = %q", cfg.PublicBaseURL)
	}
}

func TestLoadRejectsPartialBootstrapCredentials(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("BOOTSTRAP_ADMIN_EMAIL", "owner@example.com")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted partial bootstrap credentials")
	}
}
