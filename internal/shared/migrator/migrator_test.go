package migrator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMigrationName(t *testing.T) {
	t.Parallel()

	version, name := parseMigrationName("20260404113000_add_app_settings.up.sql")
	if version != "20260404113000" {
		t.Fatalf("unexpected version: got %q", version)
	}
	if name != "add_app_settings" {
		t.Fatalf("unexpected name: got %q", name)
	}
}

func TestLoadMigrationsSortsUpFilesOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "20260405090000_expand_app_settings_for_ui.up.sql"), "SELECT 2;")
	writeTestFile(t, filepath.Join(dir, "20260404113000_add_app_settings.down.sql"), "SELECT 0;")
	writeTestFile(t, filepath.Join(dir, "20260404113000_add_app_settings.up.sql"), "SELECT 1;")

	migrations, err := loadMigrations(dir)
	if err != nil {
		t.Fatalf("expected loadMigrations to succeed: %v", err)
	}

	if len(migrations) != 2 {
		t.Fatalf("expected 2 up migrations, got %d", len(migrations))
	}
	if migrations[0].Version != "20260404113000" {
		t.Fatalf("unexpected first migration version: %q", migrations[0].Version)
	}
	if migrations[1].Version != "20260405090000" {
		t.Fatalf("unexpected second migration version: %q", migrations[1].Version)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %q: %v", path, err)
	}
}
