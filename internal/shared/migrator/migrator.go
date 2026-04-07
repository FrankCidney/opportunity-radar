package migrator

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Migration struct {
	Version string
	Name    string
	Path    string
	SQL     string
}

func Run(ctx context.Context, db *sql.DB, logger *slog.Logger) error {
	if db == nil {
		return fmt.Errorf("db is required")
	}

	migrationsDir, err := resolveMigrationsDir()
	if err != nil {
		return err
	}

	migrations, err := loadMigrations(migrationsDir)
	if err != nil {
		return err
	}

	if len(migrations) == 0 {
		return fmt.Errorf("no up migrations found in %q", migrationsDir)
	}

	if err := ensureSchemaMigrationsTable(ctx, db); err != nil {
		return err
	}

	for _, migration := range migrations {
		applied, err := isApplied(ctx, db, migration.Version)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", migration.Version, err)
		}
		if applied {
			continue
		}

		if logger != nil {
			logger.Info("applying database migration", "version", migration.Version, "name", migration.Name)
		}

		if err := apply(ctx, db, migration); err != nil {
			return fmt.Errorf("apply migration %s: %w", migration.Version, err)
		}
	}

	return nil
}

func resolveMigrationsDir() (string, error) {
	candidates := []string{
		"migrations",
		filepath.Join(".", "migrations"),
	}

	if executablePath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executablePath), "migrations"))
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate, nil
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("stat migrations dir %q: %w", candidate, err)
		}
	}

	return "", fmt.Errorf("could not find migrations directory")
}

func loadMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}

		version, label := parseMigrationName(name)
		if version == "" {
			return nil, fmt.Errorf("invalid migration filename %q", name)
		}

		path := filepath.Join(dir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", name, err)
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    label,
			Path:    path,
			SQL:     string(content),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func parseMigrationName(filename string) (string, string) {
	filename = strings.TrimSuffix(filename, ".up.sql")
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) != 2 {
		return "", ""
	}

	return parts[0], parts[1]
}

func ensureSchemaMigrationsTable(ctx context.Context, db *sql.DB) error {
	const query = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version TEXT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);`

	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}
	return nil
}

func isApplied(ctx context.Context, db *sql.DB, version string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(
		ctx,
		`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`,
		version,
	).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func apply(ctx context.Context, db *sql.DB, migration Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		return fmt.Errorf("exec sql: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
		migration.Version,
		migration.Name,
	); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}
