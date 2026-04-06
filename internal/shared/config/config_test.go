package config

import "testing"

func TestLoadReturnsErrorWhenDatabaseURLMissing(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected missing DATABASE_URL to return an error")
	}
}

func TestLoadReturnsErrorWhenBoolInvalid(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("SCHEDULER_ENABLED", "maybe")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected invalid bool env to return an error")
	}
}

func TestLoadReturnsDefaultsForOptionalValues(t *testing.T) {
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
}
