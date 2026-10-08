package core

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"consflow/backend/internal/shared/migrations"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func openIsolatedAccountingTestDB(t *testing.T) *sql.DB {
	return openIsolatedCoreTestDB(t, "accounting_test_", []string{"schema_migrations", "projects", "users", "accounting_reconciliation_bases"})
}

func openIsolatedProjectOperationsTestDB(t *testing.T) *sql.DB {
	return openIsolatedCoreTestDB(t, "project_test_", []string{"schema_migrations", "projects", "users", "roles", "role_permissions", "user_roles", "project_members"})
}

func openIsolatedCoreTestDB(t *testing.T, prefix string, relations []string) *sql.DB {
	t.Helper()
	dsn := os.Getenv("CONSFLOW_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CONSFLOW_TEST_DATABASE_URL to run isolated DB-backed tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	adminDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := prefix + strings.ReplaceAll(newID(), "-", "")
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		_ = adminDB.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = adminDB.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		_ = adminDB.Close()
	})
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	// Keep the test schema first while retaining extension functions installed in public
	// (for example pgcrypto.digest used by immutable migration definitions).
	query.Set("search_path", schema+",public")
	parsed.RawQuery = query.Encode()
	database, err := sql.Open("pgx", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	// Reserve a schema-local catalog before the runner can resolve public history.
	if _, err := database.ExecContext(ctx, `CREATE TABLE `+schema+`.schema_migrations (
		version TEXT PRIMARY KEY,name TEXT NOT NULL,checksum_sha256 TEXT NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now(),execution_ms INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := migrations.Run(ctx, database, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatal(err)
	}
	// A fallback to public would make a green integration test modify another suite's data.
	for _, relation := range relations {
		var owner string
		if err := database.QueryRowContext(ctx, `SELECT n.nspname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.oid=to_regclass($1)`, relation).Scan(&owner); err != nil {
			t.Fatal(err)
		}
		if owner != schema {
			t.Fatalf("test relation %s resolved outside isolated schema: %s", relation, owner)
		}
	}
	return database
}

func mustAccountingExec(t *testing.T, ctx context.Context, database *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := database.ExecContext(ctx, query, args...); err != nil {
		t.Fatal(err)
	}
}

func assertAccountingAmount(t *testing.T, ctx context.Context, database *sql.DB, query string, want float64, args ...any) {
	t.Helper()
	var got float64
	if err := database.QueryRowContext(ctx, query, args...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if mathAbs(got-want) > 0.005 {
		t.Fatalf("query %q amount=%v, want %v", query, got, want)
	}
}

func mathAbs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
