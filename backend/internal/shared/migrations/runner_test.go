package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestRunBackfillsDuplicateOutgoingAcceptanceCodes(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schema := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	quotedSchema := quoteIdentifier(schema)
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+quotedSchema+" CASCADE")
	})

	if _, err := db.ExecContext(ctx, "SET search_path TO "+quotedSchema+", public"); err != nil {
		t.Fatal(err)
	}

	migrationsDir := filepath.Join("..", "..", "..", "migrations")
	if _, err := Run(ctx, db, migrationsDir); err != nil {
		t.Fatal(err)
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO projects (id, name, code, client_name)
		VALUES
			('30000000-0000-0000-0000-000000000001', 'Project A', 'P-A', 'Client A'),
			('30000000-0000-0000-0000-000000000002', 'Project B', 'P-B', 'Client B')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO acceptance_records (id, project_id, type, title)
		VALUES
			('71000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001', 'partial', 'Nghiệm thu trần khu A'),
			('71000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000002', 'partial', 'Nghiệm thu chống thấm tầng 1')
	`); err != nil {
		t.Fatal(err)
	}

	if _, err := db.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version = '26'"); err != nil {
		t.Fatal(err)
	}
	// V93 installs a deferred constraint trigger on acceptance_records. Real
	// databases apply V26 before V93; remove the later trigger while this test
	// reconstructs the historical pre-V26 schema so V26 can be exercised alone.
	if _, err := db.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS trg_acceptance_pdf_export_available ON acceptance_records
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE acceptance_records
			DROP COLUMN IF EXISTS acceptance_code,
			DROP COLUMN IF EXISTS work_item_id,
			DROP COLUMN IF EXISTS form_instance_id,
			DROP COLUMN IF EXISTS pdf_export_id,
			DROP COLUMN IF EXISTS acceptance_date,
			DROP COLUMN IF EXISTS location,
			DROP COLUMN IF EXISTS client_representative,
			DROP COLUMN IF EXISTS contractor_representative,
			DROP COLUMN IF EXISTS scope_summary,
			DROP COLUMN IF EXISTS signature_status,
			DROP COLUMN IF EXISTS document_status,
			DROP COLUMN IF EXISTS summary_json,
			DROP COLUMN IF EXISTS submitted_by,
			DROP COLUMN IF EXISTS submitted_at
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE acceptance_checklist_items
			DROP COLUMN IF EXISTS item_index,
			DROP COLUMN IF EXISTS evidence_id,
			DROP COLUMN IF EXISTS decision,
			DROP COLUMN IF EXISTS severity,
			DROP COLUMN IF EXISTS required_action,
			DROP COLUMN IF EXISTS due_date,
			DROP COLUMN IF EXISTS payload_json,
			DROP COLUMN IF EXISTS updated_at
	`); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(ctx, db, migrationsDir); err != nil {
		t.Fatal(err)
	}

	rows, err := db.QueryContext(ctx, `
		SELECT acceptance_code
		FROM acceptance_records
		WHERE id IN (
			'71000000-0000-0000-0000-000000000001',
			'71000000-0000-0000-0000-000000000002'
		)
		ORDER BY id
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			t.Fatal(err)
		}
		codes = append(codes, code)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	want := []string{"OA-71000000", "OA-71000000-02"}
	if strings.Join(codes, ",") != strings.Join(want, ",") {
		t.Fatalf("acceptance codes = %v, want %v", codes, want)
	}
}

func TestRunSerializesConcurrentRunners(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	first := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer first.Close()
	second := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer second.Close()

	dir := t.TempDir()
	writeMigrationFixture(t, dir, "V1__concurrent_probe.sql", `
		CREATE TABLE concurrent_probe (id INTEGER PRIMARY KEY);
		SELECT pg_sleep(0.25);
		INSERT INTO concurrent_probe (id) VALUES (1);
	`)

	type runResult struct {
		result Result
		err    error
	}
	start := make(chan struct{})
	results := make(chan runResult, 2)
	for _, database := range []*sql.DB{first, second} {
		go func(db *sql.DB) {
			<-start
			result, err := RunWithOptions(ctx, db, dir, Options{LockTimeout: 3 * time.Second})
			results <- runResult{result: result, err: err}
		}(database)
	}
	close(start)

	applied := 0
	skipped := 0
	for range 2 {
		outcome := <-results
		if outcome.err != nil {
			t.Fatalf("concurrent migration run failed: %v", outcome.err)
		}
		applied += len(outcome.result.Applied)
		skipped += len(outcome.result.Skipped)
	}
	if applied != 1 || skipped != 1 {
		t.Fatalf("concurrent results applied=%d skipped=%d, want 1/1", applied, skipped)
	}

	var rows int
	if err := first.QueryRowContext(ctx, `SELECT COUNT(*) FROM concurrent_probe`).Scan(&rows); err != nil {
		t.Fatalf("query concurrent probe: %v", err)
	}
	if rows != 1 {
		t.Fatalf("concurrent probe rows=%d, want 1", rows)
	}
}

func TestRunFailsClearlyWhenAdvisoryLockTimesOut(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	blockerDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer blockerDB.Close()
	blocker, err := blockerDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	if _, err := blocker.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationAdvisoryLockKey); err != nil {
		t.Fatalf("hold migration advisory lock: %v", err)
	}
	defer func() {
		var released bool
		_ = blocker.QueryRowContext(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationAdvisoryLockKey).Scan(&released)
	}()

	runnerDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer runnerDB.Close()
	dir := t.TempDir()
	writeMigrationFixture(t, dir, "V1__noop.sql", "SELECT 1;")

	_, err = RunWithOptions(ctx, runnerDB, dir, Options{LockTimeout: 150 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "migration advisory lock timeout after 150ms") {
		t.Fatalf("lock timeout error=%v, want explicit advisory lock timeout", err)
	}
}

func TestRunPreservesChecksumAndTransactionRollback(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()
	dir := t.TempDir()
	original := "CREATE TABLE checksum_probe (id INTEGER PRIMARY KEY);"
	writeMigrationFixture(t, dir, "V1__checksum_probe.sql", original)
	if _, err := RunWithOptions(ctx, database, dir, Options{LockTimeout: time.Second}); err != nil {
		t.Fatalf("apply checksum fixture: %v", err)
	}

	writeMigrationFixture(t, dir, "V1__checksum_probe.sql", original+"\n-- changed")
	if _, err := RunWithOptions(ctx, database, dir, Options{LockTimeout: time.Second}); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("checksum mismatch error=%v, want mismatch rejection", err)
	}

	writeMigrationFixture(t, dir, "V1__checksum_probe.sql", original)
	writeMigrationFixture(t, dir, "V2__rollback_probe.sql", `
		CREATE TABLE rollback_probe (id INTEGER PRIMARY KEY);
		INSERT INTO rollback_probe (id) VALUES (1);
		SELECT 1 / 0;
	`)
	if _, err := RunWithOptions(ctx, database, dir, Options{LockTimeout: time.Second}); err == nil {
		t.Fatal("failing migration unexpectedly succeeded")
	}
	var relationName sql.NullString
	if err := database.QueryRowContext(ctx, `SELECT to_regclass('rollback_probe')::text`).Scan(&relationName); err != nil {
		t.Fatalf("check rollback relation: %v", err)
	}
	if relationName.Valid {
		t.Fatalf("rollback_probe still exists after failed migration: %s", relationName.String)
	}
	var historyRows int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = '2'`).Scan(&historyRows); err != nil {
		t.Fatalf("check rollback migration history: %v", err)
	}
	if historyRows != 0 {
		t.Fatalf("failed migration history rows=%d, want 0", historyRows)
	}
}

func TestRunRejectsAppliedVersionMissingFromImageManifest(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()
	currentManifest := t.TempDir()
	writeMigrationFixture(t, currentManifest, "V1__current.sql", "CREATE TABLE manifest_probe (id INTEGER PRIMARY KEY);")
	if _, err := RunWithOptions(ctx, database, currentManifest, Options{LockTimeout: time.Second}); err != nil {
		t.Fatalf("apply current image manifest: %v", err)
	}

	olderImageManifest := t.TempDir()
	_, err := RunWithOptions(ctx, database, olderImageManifest, Options{LockTimeout: time.Second})
	if err == nil || !strings.Contains(err.Error(), "version 1 absent from the image migration manifest") {
		t.Fatalf("missing image migration error=%v, want fail-closed version mismatch", err)
	}
}

func requireMigrationTestDatabase(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("CONSFLOW_TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("CI") == "true" {
			t.Fatal("CONSFLOW_TEST_DATABASE_URL is required in CI; DB-backed migration tests must not skip")
		}
		t.Skip("set CONSFLOW_TEST_DATABASE_URL to run DB-backed migration concurrency tests")
	}
	return dsn
}

func createMigrationTestSchema(t *testing.T, ctx context.Context, dsn string) (string, func()) {
	t.Helper()
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("migration_runner_test_%d", time.Now().UnixNano())
	quotedSchema := quoteIdentifier(schema)
	if _, err := database.ExecContext(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	return schema, func() {
		_, _ = database.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+quotedSchema+" CASCADE")
		_ = database.Close()
	}
}

func openMigrationTestDatabase(t *testing.T, ctx context.Context, dsn, schema string) *sql.DB {
	t.Helper()
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	if _, err := database.ExecContext(ctx, "SET search_path TO "+quoteIdentifier(schema)+", public"); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	return database
}

func writeMigrationFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write migration fixture %s: %v", name, err)
	}
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
