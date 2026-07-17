package migrator

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	if migrations[0].SQL != "SELECT 1;" {
		t.Fatalf("first migration SQL = %q, want file contents", migrations[0].SQL)
	}
}

func TestIdentityMigrationUsesHashedTokensAndProtectedLegacyCreation(t *testing.T) {
	t.Parallel()

	path := filepath.Join(
		"..", "..", "..", "migrations",
		"20260718100000_add_identity_and_authentication.up.sql",
	)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read identity migration: %v", err)
	}
	sqlText := string(content)

	required := []string{
		"CREATE TABLE tenants",
		"CREATE TABLE users",
		"CREATE TABLE tenant_memberships",
		"CREATE TABLE sessions",
		"token_hash BYTEA NOT NULL UNIQUE",
		"CREATE TABLE account_tokens",
		"users_email_normalized",
		"WHERE EXISTS (SELECT 1 FROM app_settings)",
	}
	for _, fragment := range required {
		if !strings.Contains(sqlText, fragment) {
			t.Errorf("identity migration missing %q", fragment)
		}
	}

	forbidden := []string{
		"password TEXT",
		"token TEXT",
		"INSERT INTO users",
	}
	for _, fragment := range forbidden {
		if strings.Contains(sqlText, fragment) {
			t.Errorf("identity migration contains unsafe fragment %q", fragment)
		}
	}
}

func TestApplyExecutesMigrationAndRecordsItInOneTransaction(t *testing.T) {
	t.Parallel()

	connection := &recordingConnection{}
	db := sql.OpenDB(recordingConnector{connection: connection})
	t.Cleanup(func() { _ = db.Close() })

	migration := Migration{
		Version: "20260717090000",
		Name:    "add_characterized_table",
		SQL:     "CREATE TABLE characterized (id BIGINT PRIMARY KEY);",
	}
	if err := apply(context.Background(), db, migration); err != nil {
		t.Fatalf("apply() error = %v", err)
	}

	if !connection.began {
		t.Fatal("transaction was not started")
	}
	if !connection.committed {
		t.Fatal("transaction was not committed")
	}
	if connection.rolledBack {
		t.Fatal("transaction was rolled back after successful apply")
	}
	if len(connection.executions) != 2 {
		t.Fatalf("transaction executions = %d, want migration and version record", len(connection.executions))
	}
	if connection.executions[0].query != migration.SQL {
		t.Fatalf("first execution = %q, want migration SQL", connection.executions[0].query)
	}
	if !strings.Contains(connection.executions[1].query, "INSERT INTO schema_migrations") {
		t.Fatalf("second execution = %q, want migration record insert", connection.executions[1].query)
	}
	if got := connection.executions[1].args; len(got) != 2 ||
		got[0].Value != migration.Version || got[1].Value != migration.Name {
		t.Fatalf("migration record args = %#v, want version and name", got)
	}
}

func TestApplyRollsBackWhenMigrationSQLFails(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("migration failed")
	connection := &recordingConnection{execErr: wantErr}
	db := sql.OpenDB(recordingConnector{connection: connection})
	t.Cleanup(func() { _ = db.Close() })

	err := apply(context.Background(), db, Migration{
		Version: "20260717090100",
		Name:    "broken",
		SQL:     "BROKEN SQL",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("apply() error = %v, want wrapped %v", err, wantErr)
	}
	if connection.committed {
		t.Fatal("failed migration was committed")
	}
	if !connection.rolledBack {
		t.Fatal("failed migration was not rolled back")
	}
	if len(connection.executions) != 1 {
		t.Fatalf("transaction executions = %d, want only failed migration SQL", len(connection.executions))
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %q: %v", path, err)
	}
}

type recordingConnector struct {
	connection *recordingConnection
}

func (c recordingConnector) Connect(context.Context) (driver.Conn, error) {
	return c.connection, nil
}

func (recordingConnector) Driver() driver.Driver {
	return recordingDriver{}
}

type recordingDriver struct{}

func (recordingDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("recording driver must be opened through its connector")
}

type execution struct {
	query string
	args  []driver.NamedValue
}

type recordingConnection struct {
	began      bool
	committed  bool
	rolledBack bool
	execErr    error
	executions []execution
}

func (c *recordingConnection) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}

func (c *recordingConnection) Close() error {
	return nil
}

func (c *recordingConnection) Begin() (driver.Tx, error) {
	c.began = true
	return &recordingTransaction{connection: c}, nil
}

func (c *recordingConnection) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return c.Begin()
}

func (c *recordingConnection) ExecContext(
	_ context.Context,
	query string,
	args []driver.NamedValue,
) (driver.Result, error) {
	c.executions = append(c.executions, execution{query: query, args: args})
	if c.execErr != nil {
		err := c.execErr
		c.execErr = nil
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

type recordingTransaction struct {
	connection *recordingConnection
}

func (tx *recordingTransaction) Commit() error {
	tx.connection.committed = true
	return nil
}

func (tx *recordingTransaction) Rollback() error {
	tx.connection.rolledBack = true
	return nil
}
