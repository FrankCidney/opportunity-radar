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

	isGolangMigrate, err := isGolangMigrateTable(ctx, db)
	if err != nil {
		return err
	}

	if isGolangMigrate {
		if err := migrateFromGolangMigrate(ctx, db, migrations, logger); err != nil {
			return err
		}
	} else {
		if err := ensureSchemaMigrationsTable(ctx, db); err != nil {
			return err
		}
	}

	if err := autoDetectAppliedMigrations(ctx, db, migrations, logger); err != nil {
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

func isGolangMigrateTable(ctx context.Context, db *sql.DB) (bool, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT column_name 
		FROM information_schema.columns 
		WHERE table_name = 'schema_migrations' 
		  AND table_schema = current_schema()
	`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	hasDirty := false
	hasName := false
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return false, err
		}
		if col == "dirty" {
			hasDirty = true
		}
		if col == "name" {
			hasName = true
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}

	return hasDirty && !hasName, nil
}

func migrateFromGolangMigrate(ctx context.Context, db *sql.DB, migrations []Migration, logger *slog.Logger) error {
	if logger != nil {
		logger.Info("detecting migration table from golang-migrate; upgrading schema_migrations schema")
	}

	var version int64
	var dirty bool
	err := db.QueryRowContext(ctx, `SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&version, &dirty)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			version = 0
		} else {
			return fmt.Errorf("read golang-migrate version: %w", err)
		}
	}

	if dirty {
		if logger != nil {
			logger.Warn("golang-migrate table is marked as dirty; proceeding with caution", "version", version)
		}
	}

	if _, err := db.ExecContext(ctx, `DROP TABLE schema_migrations`); err != nil {
		return fmt.Errorf("drop old schema_migrations table: %w", err)
	}

	if err := ensureSchemaMigrationsTable(ctx, db); err != nil {
		return fmt.Errorf("create new schema_migrations table: %w", err)
	}

	versionStr := fmt.Sprintf("%d", version)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin backfill tx: %w", err)
	}
	defer tx.Rollback()

	for _, m := range migrations {
		if m.Version <= versionStr {
			if logger != nil {
				logger.Info("backfilling migration record from golang-migrate", "version", m.Version, "name", m.Name)
			}
			_, err := tx.ExecContext(ctx, 
				`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
				m.Version, m.Name,
			)
			if err != nil {
				return fmt.Errorf("backfill migration %s: %w", m.Version, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit backfill tx: %w", err)
	}

	return nil
}

func autoDetectAppliedMigrations(ctx context.Context, db *sql.DB, migrations []Migration, logger *slog.Logger) error {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return nil
	}

	var companiesExists bool
	err = db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.tables 
			WHERE table_name = 'companies' 
			  AND table_schema = current_schema()
		)
	`).Scan(&companiesExists)
	if err != nil {
		return err
	}

	if !companiesExists {
		return nil
	}

	if logger != nil {
		logger.Info("database tables already exist, but schema_migrations is empty; auto-detecting applied migrations")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, m := range migrations {
		applied := false
		switch m.Version {
		case "20260220142958":
			applied = true
		case "20260401152300":
			err = tx.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1 
					FROM information_schema.columns 
					WHERE table_name = 'companies' 
					  AND column_name = 'source' 
					  AND table_schema = current_schema()
				)
			`).Scan(&applied)
		case "20260403103000":
			err = tx.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1 
					FROM information_schema.tables 
					WHERE table_name = 'digest_deliveries' 
					  AND table_schema = current_schema()
				)
			`).Scan(&applied)
		case "20260404113000":
			err = tx.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1 
					FROM information_schema.tables 
					WHERE table_name = 'app_settings' 
					  AND table_schema = current_schema()
				)
			`).Scan(&applied)
		case "20260405090000":
			err = tx.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1 
					FROM information_schema.columns 
					WHERE table_name = 'app_settings' 
					  AND column_name = 'desired_roles' 
					  AND table_schema = current_schema()
				)
			`).Scan(&applied)
		default:
			applied = false
		}

		if err != nil {
			return fmt.Errorf("auto-detect migration %s: %w", m.Version, err)
		}

		if applied {
			if logger != nil {
				logger.Info("detected migration already applied; recording in schema_migrations", "version", m.Version, "name", m.Name)
			}
			_, err = tx.ExecContext(ctx, 
				`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
				m.Version, m.Name,
			)
			if err != nil {
				return fmt.Errorf("record auto-detected migration %s: %w", m.Version, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}
