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
)

func TestS3StorageProviderMigrationConstraintsAndImmutableRelocation(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()

	mustExecStorageProviderSQL(t, ctx, database, `
		CREATE TABLE evidence_records (
			id INTEGER PRIMARY KEY,
			storage_provider TEXT NOT NULL,
			storage_key TEXT,
			CONSTRAINT evidence_records_storage_provider_check
				CHECK (storage_provider IN ('external', 'local')),
			CONSTRAINT evidence_records_storage_key_provider_check
				CHECK (storage_key IS NULL OR storage_provider = 'local')
		);
		CREATE TABLE workflow_form_signatures (
			id INTEGER PRIMARY KEY,
			payload TEXT NOT NULL,
			visual_storage_provider TEXT NOT NULL,
			visual_storage_key TEXT,
			CONSTRAINT workflow_form_signatures_visual_storage_provider_check
				CHECK (visual_storage_provider IN ('none', 'local')),
			CONSTRAINT workflow_form_signatures_visual_storage_key_check CHECK (
				(visual_storage_provider = 'none' AND visual_storage_key IS NULL)
				OR
				(visual_storage_provider = 'local' AND visual_storage_key IS NOT NULL)
			)
		);
		CREATE TABLE workflow_form_pdf_exports (
			id INTEGER PRIMARY KEY,
			payload TEXT NOT NULL,
			storage_provider TEXT NOT NULL,
			storage_key TEXT NOT NULL,
			CONSTRAINT workflow_form_pdf_exports_provider_check
				CHECK (storage_provider = 'local')
		);
		CREATE TABLE input_invoices (
			id INTEGER PRIMARY KEY,
			storage_provider TEXT NOT NULL,
			storage_key TEXT NOT NULL,
			CONSTRAINT input_invoices_storage_provider_check
				CHECK (storage_provider IN ('local'))
		);
		CREATE TABLE output_invoices (
			id INTEGER PRIMARY KEY,
			storage_provider TEXT NOT NULL,
			storage_key TEXT NOT NULL,
			CONSTRAINT output_invoices_storage_provider_check
				CHECK (storage_provider IN ('local'))
		);

		CREATE FUNCTION prevent_workflow_form_signature_mutation()
		RETURNS trigger
		LANGUAGE plpgsql
		AS $$
		BEGIN
			RAISE EXCEPTION 'workflow_form_signatures are immutable';
		END;
		$$;
		CREATE TRIGGER trg_workflow_form_signatures_immutable
		BEFORE UPDATE OR DELETE ON workflow_form_signatures
		FOR EACH ROW
		EXECUTE FUNCTION prevent_workflow_form_signature_mutation();

		CREATE FUNCTION prevent_workflow_form_pdf_export_mutation()
		RETURNS trigger
		LANGUAGE plpgsql
		AS $$
		BEGIN
			RAISE EXCEPTION 'workflow_form_pdf_exports are immutable';
		END;
		$$;
		CREATE TRIGGER trg_workflow_form_pdf_exports_immutable
		BEFORE UPDATE OR DELETE ON workflow_form_pdf_exports
		FOR EACH ROW
		EXECUTE FUNCTION prevent_workflow_form_pdf_export_mutation();

		INSERT INTO evidence_records (id, storage_provider, storage_key) VALUES
			(1, 'external', NULL),
			(2, 'local', 'evidence/local');
		INSERT INTO workflow_form_signatures
			(id, payload, visual_storage_provider, visual_storage_key)
		VALUES
			(1, 'unchanged', 'none', NULL),
			(2, 'unchanged', 'local', 'signatures/2');
		INSERT INTO workflow_form_pdf_exports
			(id, payload, storage_provider, storage_key)
		VALUES
			(1, 'unchanged', 'local', 'exports/1');
		INSERT INTO input_invoices (id, storage_provider, storage_key)
		VALUES (1, 'local', 'input/1');
		INSERT INTO output_invoices (id, storage_provider, storage_key)
		VALUES (1, 'local', 'output/1');
	`)

	migrationPath := filepath.Join("..", "..", "..", "migrations", "V70__s3_storage_provider.sql")
	migrationSQL, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read V70 migration: %v", err)
	}
	dir := t.TempDir()
	writeMigrationFixture(t, dir, "V70__s3_storage_provider.sql", string(migrationSQL))
	if _, err := RunWithOptions(ctx, database, dir, Options{LockTimeout: time.Second}); err != nil {
		t.Fatalf("apply V70 migration: %v", err)
	}

	var preservedRows int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM (
			SELECT 1 FROM evidence_records
			WHERE id = 1 AND storage_provider = 'external' AND storage_key IS NULL
			UNION ALL
			SELECT 1 FROM evidence_records
			WHERE id = 2 AND storage_provider = 'local' AND storage_key = 'evidence/local'
			UNION ALL
			SELECT 1 FROM workflow_form_signatures
			WHERE id = 1 AND visual_storage_provider = 'none' AND visual_storage_key IS NULL
			UNION ALL
			SELECT 1 FROM workflow_form_signatures
			WHERE id = 2 AND visual_storage_provider = 'local' AND visual_storage_key = 'signatures/2'
			UNION ALL
			SELECT 1 FROM workflow_form_pdf_exports
			WHERE id = 1 AND storage_provider = 'local' AND storage_key = 'exports/1'
			UNION ALL
			SELECT 1 FROM input_invoices
			WHERE id = 1 AND storage_provider = 'local' AND storage_key = 'input/1'
			UNION ALL
			SELECT 1 FROM output_invoices
			WHERE id = 1 AND storage_provider = 'local' AND storage_key = 'output/1'
		) preserved
	`).Scan(&preservedRows); err != nil {
		t.Fatalf("verify pre-V70 rows: %v", err)
	}
	if preservedRows != 7 {
		t.Fatalf("preserved pre-V70 rows = %d, want 7", preservedRows)
	}

	mustExecStorageProviderSQL(t, ctx, database, `
		INSERT INTO evidence_records (id, storage_provider, storage_key)
		VALUES (3, 's3', 'evidence/s3');
		INSERT INTO workflow_form_signatures
			(id, payload, visual_storage_provider, visual_storage_key)
		VALUES (3, 'unchanged', 's3', 'signatures/3');
		INSERT INTO workflow_form_pdf_exports
			(id, payload, storage_provider, storage_key)
		VALUES (2, 'unchanged', 's3', 'exports/2');
		INSERT INTO input_invoices (id, storage_provider, storage_key)
		VALUES (2, 's3', 'input/2');
		INSERT INTO output_invoices (id, storage_provider, storage_key)
		VALUES (2, 's3', 'output/2');
	`)

	expectStorageProviderSQLError(t, ctx, database,
		`INSERT INTO evidence_records (id, storage_provider, storage_key) VALUES (4, 'external', 'external/key')`, "evidence external key")
	expectStorageProviderSQLError(t, ctx, database,
		`INSERT INTO evidence_records (id, storage_provider, storage_key) VALUES (5, 'unknown', NULL)`, "evidence unknown provider")
	expectStorageProviderSQLError(t, ctx, database,
		`INSERT INTO workflow_form_signatures (id, payload, visual_storage_provider, visual_storage_key) VALUES (4, 'x', 's3', NULL)`, "signature s3 key")
	expectStorageProviderSQLError(t, ctx, database,
		`INSERT INTO workflow_form_signatures (id, payload, visual_storage_provider, visual_storage_key) VALUES (5, 'x', 'unknown', NULL)`, "signature unknown provider")
	expectStorageProviderSQLError(t, ctx, database,
		`INSERT INTO workflow_form_pdf_exports (id, payload, storage_provider, storage_key) VALUES (3, 'x', 'unknown', 'exports/3')`, "PDF unknown provider")
	expectStorageProviderSQLError(t, ctx, database,
		`INSERT INTO input_invoices (id, storage_provider, storage_key) VALUES (3, 'unknown', 'input/3')`, "input invoice unknown provider")
	expectStorageProviderSQLError(t, ctx, database,
		`INSERT INTO output_invoices (id, storage_provider, storage_key) VALUES (3, 'unknown', 'output/3')`, "output invoice unknown provider")

	mustExecStorageProviderSQL(t, ctx, database,
		`UPDATE workflow_form_signatures SET visual_storage_provider = 's3' WHERE id = 2`)
	expectStorageProviderSQLError(t, ctx, database,
		`UPDATE workflow_form_signatures SET visual_storage_provider = 'local' WHERE id = 2`, "signature reverse relocation")
	expectStorageProviderSQLError(t, ctx, database,
		`UPDATE workflow_form_signatures SET visual_storage_key = 'signatures/changed' WHERE id = 2`, "signature key mutation")
	expectStorageProviderSQLError(t, ctx, database,
		`UPDATE workflow_form_signatures SET payload = 'changed' WHERE id = 2`, "signature payload mutation")
	expectStorageProviderSQLError(t, ctx, database,
		`DELETE FROM workflow_form_signatures WHERE id = 2`, "signature delete")

	mustExecStorageProviderSQL(t, ctx, database,
		`UPDATE workflow_form_pdf_exports SET storage_provider = 's3' WHERE id = 1`)
	expectStorageProviderSQLError(t, ctx, database,
		`UPDATE workflow_form_pdf_exports SET storage_provider = 'local' WHERE id = 1`, "PDF reverse relocation")
	expectStorageProviderSQLError(t, ctx, database,
		`UPDATE workflow_form_pdf_exports SET storage_key = 'exports/changed' WHERE id = 1`, "PDF key mutation")
	expectStorageProviderSQLError(t, ctx, database,
		`UPDATE workflow_form_pdf_exports SET payload = 'changed' WHERE id = 1`, "PDF payload mutation")
	expectStorageProviderSQLError(t, ctx, database,
		`DELETE FROM workflow_form_pdf_exports WHERE id = 1`, "PDF delete")
}

func TestStorageKeyCanonicalMigrationPreservesLegacyRowsAndRejectsFutureInvalidWrites(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()

	mustExecStorageProviderSQL(t, ctx, database, `
		CREATE TABLE evidence_records (
			id INTEGER PRIMARY KEY,
			storage_provider TEXT NOT NULL,
			storage_key TEXT
		);
		CREATE TABLE workflow_form_signatures (
			id INTEGER PRIMARY KEY,
			payload TEXT NOT NULL,
			visual_storage_provider TEXT NOT NULL,
			visual_storage_key TEXT
		);
		CREATE TABLE workflow_form_pdf_exports (
			id INTEGER PRIMARY KEY,
			payload TEXT NOT NULL,
			storage_provider TEXT NOT NULL,
			storage_key TEXT NOT NULL
		);
		CREATE TABLE input_invoices (
			id INTEGER PRIMARY KEY,
			storage_provider TEXT NOT NULL,
			storage_key TEXT NOT NULL
		);
		CREATE TABLE output_invoices (
			id INTEGER PRIMARY KEY,
			storage_provider TEXT NOT NULL,
			storage_key TEXT NOT NULL
		);
	`)

	dir := t.TempDir()
	copyStorageProviderMigrationFixture(t, dir, "V70__s3_storage_provider.sql")
	if _, err := RunWithOptions(ctx, database, dir, Options{LockTimeout: time.Second}); err != nil {
		t.Fatalf("apply V70 fixture: %v", err)
	}

	mustExecStorageProviderSQL(t, ctx, database, `
		INSERT INTO evidence_records (id, storage_provider, storage_key) VALUES
			(1, 'local', NULL),
			(2, 'local', ''),
			(3, 's3', ' evidence/padded ');
		INSERT INTO workflow_form_signatures
			(id, payload, visual_storage_provider, visual_storage_key)
		VALUES
			(1, 'legacy', 'local', ''),
			(2, 'legacy', 's3', ' signatures/padded ');
		INSERT INTO workflow_form_pdf_exports
			(id, payload, storage_provider, storage_key)
		VALUES
			(1, 'legacy', 'local', ''),
			(2, 'legacy', 's3', ' exports/padded ');
		INSERT INTO input_invoices (id, storage_provider, storage_key) VALUES
			(1, 'local', ''),
			(2, 's3', ' input/padded ');
		INSERT INTO output_invoices (id, storage_provider, storage_key) VALUES
			(1, 'local', ''),
			(2, 's3', ' output/padded ');
	`)

	copyStorageProviderMigrationFixture(t, dir, "V71__storage_key_integrity.sql")
	if _, err := RunWithOptions(ctx, database, dir, Options{LockTimeout: time.Second}); err != nil {
		t.Fatalf("apply V71 fixture with legacy invalid rows: %v", err)
	}

	var legacyRows int
	if err := database.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM evidence_records)
			+ (SELECT COUNT(*) FROM workflow_form_signatures)
			+ (SELECT COUNT(*) FROM workflow_form_pdf_exports)
			+ (SELECT COUNT(*) FROM input_invoices)
			+ (SELECT COUNT(*) FROM output_invoices)
	`).Scan(&legacyRows); err != nil {
		t.Fatalf("count preserved legacy storage rows: %v", err)
	}
	if legacyRows != 11 {
		t.Fatalf("preserved legacy rows = %d, want 11", legacyRows)
	}

	canonicalConstraints := []string{
		"evidence_records_internal_storage_key_canonical_check",
		"workflow_form_signatures_visual_storage_key_canonical_check",
		"workflow_form_pdf_exports_storage_key_canonical_check",
		"input_invoices_storage_key_canonical_check",
		"output_invoices_storage_key_canonical_check",
	}
	var notValidConstraints int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM pg_constraint
		WHERE conname = ANY($1)
		  AND connamespace = current_schema()::regnamespace
		  AND NOT convalidated
		  AND pg_get_constraintdef(oid) LIKE '%btrim%'
	`, canonicalConstraints).Scan(&notValidConstraints); err != nil {
		t.Fatalf("verify V71 NOT VALID constraints: %v", err)
	}
	if notValidConstraints != len(canonicalConstraints) {
		t.Fatalf("V71 NOT VALID canonical constraints = %d, want %d", notValidConstraints, len(canonicalConstraints))
	}

	for _, constraint := range []struct {
		table string
		name  string
	}{
		{table: "evidence_records", name: canonicalConstraints[0]},
		{table: "workflow_form_signatures", name: canonicalConstraints[1]},
		{table: "workflow_form_pdf_exports", name: canonicalConstraints[2]},
		{table: "input_invoices", name: canonicalConstraints[3]},
		{table: "output_invoices", name: canonicalConstraints[4]},
	} {
		expectStorageProviderSQLError(t, ctx, database,
			fmt.Sprintf("ALTER TABLE %s VALIDATE CONSTRAINT %s", constraint.table, constraint.name),
			constraint.table+" legacy validation")
	}

	mustExecStorageProviderSQL(t, ctx, database, `
		INSERT INTO evidence_records (id, storage_provider, storage_key) VALUES
			(100, 'local', 'evidence/local'),
			(101, 's3', 'evidence/s3');
		INSERT INTO workflow_form_signatures
			(id, payload, visual_storage_provider, visual_storage_key)
		VALUES
			(100, 'canonical', 'local', 'signatures/local'),
			(101, 'canonical', 's3', 'signatures/s3');
		INSERT INTO workflow_form_pdf_exports
			(id, payload, storage_provider, storage_key)
		VALUES
			(100, 'canonical', 'local', 'exports/local'),
			(101, 'canonical', 's3', 'exports/s3');
		INSERT INTO input_invoices (id, storage_provider, storage_key) VALUES
			(100, 'local', 'input/local'),
			(101, 's3', 'input/s3');
		INSERT INTO output_invoices (id, storage_provider, storage_key) VALUES
			(100, 'local', 'output/local'),
			(101, 's3', 'output/s3');
	`)

	invalidFamilies := []struct {
		name      string
		statement string
	}{
		{name: "evidence", statement: "INSERT INTO evidence_records (id, storage_provider, storage_key) VALUES (%d, '%s', %s)"},
		{name: "signature", statement: "INSERT INTO workflow_form_signatures (id, payload, visual_storage_provider, visual_storage_key) VALUES (%d, 'invalid', '%s', %s)"},
		{name: "workflow PDF", statement: "INSERT INTO workflow_form_pdf_exports (id, payload, storage_provider, storage_key) VALUES (%d, 'invalid', '%s', %s)"},
		{name: "input invoice", statement: "INSERT INTO input_invoices (id, storage_provider, storage_key) VALUES (%d, '%s', %s)"},
		{name: "output invoice", statement: "INSERT INTO output_invoices (id, storage_provider, storage_key) VALUES (%d, '%s', %s)"},
	}
	invalidKeys := []struct {
		name string
		sql  string
	}{
		{name: "null", sql: "NULL"},
		{name: "blank", sql: "''"},
		{name: "padded", sql: "' padded/key '"},
	}
	nextID := 200
	for _, family := range invalidFamilies {
		for _, provider := range []string{"local", "s3"} {
			for _, key := range invalidKeys {
				expectStorageProviderSQLError(t, ctx, database,
					fmt.Sprintf(family.statement, nextID, provider, key.sql),
					fmt.Sprintf("%s %s %s key", family.name, provider, key.name))
				nextID++
			}
		}
	}

	mustExecStorageProviderSQL(t, ctx, database,
		`UPDATE workflow_form_signatures SET visual_storage_provider = 's3' WHERE id = 100`)
	mustExecStorageProviderSQL(t, ctx, database,
		`UPDATE workflow_form_pdf_exports SET storage_provider = 's3' WHERE id = 100`)
	expectStorageProviderSQLError(t, ctx, database,
		`UPDATE workflow_form_signatures SET visual_storage_key = 'signatures/changed' WHERE id = 100`, "V71 signature key mutation")
	expectStorageProviderSQLError(t, ctx, database,
		`UPDATE workflow_form_pdf_exports SET storage_key = 'exports/changed' WHERE id = 100`, "V71 PDF key mutation")
	expectStorageProviderSQLError(t, ctx, database,
		`DELETE FROM workflow_form_signatures WHERE id = 100`, "V71 signature delete")
	expectStorageProviderSQLError(t, ctx, database,
		`DELETE FROM workflow_form_pdf_exports WHERE id = 100`, "V71 PDF delete")
}

func TestS3StorageProviderMigrationsFullChainPreservesImmutableTriggers(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()

	migrationsDir := filepath.Join("..", "..", "..", "migrations")
	if _, err := Run(ctx, database, migrationsDir); err != nil {
		t.Fatalf("apply full migration chain through V71: %v", err)
	}

	assertStorageProviderFunctionContract(t, ctx, database,
		"prevent_workflow_form_signature_mutation()",
		[]string{"OLD.visual_storage_provider = 'local'", "NEW.visual_storage_provider = 's3'", "to_jsonb(NEW) - 'visual_storage_provider'"})
	assertStorageProviderFunctionContract(t, ctx, database,
		"prevent_workflow_form_pdf_export_mutation()",
		[]string{"OLD.storage_provider = 'local'", "NEW.storage_provider = 's3'", "to_jsonb(NEW) - 'storage_provider'"})

	for _, trigger := range []struct {
		table string
		name  string
	}{
		{table: "workflow_form_signatures", name: "trg_workflow_form_signatures_immutable"},
		{table: "workflow_form_pdf_exports", name: "trg_workflow_form_pdf_exports_immutable"},
	} {
		var definition string
		if err := database.QueryRowContext(ctx, `
			SELECT pg_get_triggerdef(oid)
			FROM pg_trigger
			WHERE tgrelid = $1::regclass
			  AND tgname = $2
			  AND NOT tgisinternal
		`, trigger.table, trigger.name).Scan(&definition); err != nil {
			t.Fatalf("read trigger %s: %v", trigger.name, err)
		}
		if !strings.Contains(definition, "BEFORE") ||
			!strings.Contains(definition, "UPDATE") ||
			!strings.Contains(definition, "DELETE") {
			t.Fatalf("trigger %s definition = %q, want UPDATE OR DELETE guard", trigger.name, definition)
		}
	}

	var s3Constraints int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM pg_constraint
		WHERE conname = ANY($1)
		  AND connamespace = current_schema()::regnamespace
		  AND pg_get_constraintdef(oid) LIKE '%s3%'
	`, []string{
		"evidence_records_storage_provider_check",
		"evidence_records_storage_key_provider_check",
		"workflow_form_signatures_visual_storage_provider_check",
		"workflow_form_signatures_visual_storage_key_check",
		"workflow_form_pdf_exports_provider_check",
	}).Scan(&s3Constraints); err != nil {
		t.Fatalf("verify full-chain S3 constraints: %v", err)
	}
	if s3Constraints != 5 {
		t.Fatalf("full-chain S3 constraints = %d, want 5 current evidence/signature/PDF constraints", s3Constraints)
	}

	var canonicalConstraints int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM pg_constraint
		WHERE conname = ANY($1)
		  AND connamespace = current_schema()::regnamespace
		  AND NOT convalidated
		  AND pg_get_constraintdef(oid) LIKE '%btrim%'
	`, []string{
		"evidence_records_internal_storage_key_canonical_check",
		"workflow_form_signatures_visual_storage_key_canonical_check",
		"workflow_form_pdf_exports_storage_key_canonical_check",
	}).Scan(&canonicalConstraints); err != nil {
		t.Fatalf("verify full-chain V71 canonical constraints: %v", err)
	}
	if canonicalConstraints != 3 {
		t.Fatalf("full-chain V71 canonical constraints = %d, want 3 current evidence/signature/PDF constraints", canonicalConstraints)
	}
}

func copyStorageProviderMigrationFixture(t *testing.T, dir, name string) {
	t.Helper()
	migrationPath := filepath.Join("..", "..", "..", "migrations", name)
	migrationSQL, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	writeMigrationFixture(t, dir, name, string(migrationSQL))
}

func assertStorageProviderFunctionContract(t *testing.T, ctx context.Context, database *sql.DB, function string, fragments []string) {
	t.Helper()
	var definition string
	if err := database.QueryRowContext(ctx,
		`SELECT pg_get_functiondef($1::regprocedure)`, function).Scan(&definition); err != nil {
		t.Fatalf("read function %s: %v", function, err)
	}
	for _, fragment := range fragments {
		if !strings.Contains(definition, fragment) {
			t.Fatalf("function %s missing %q in definition %q", function, fragment, definition)
		}
	}
}

func mustExecStorageProviderSQL(t *testing.T, ctx context.Context, database *sql.DB, statement string) {
	t.Helper()
	if _, err := database.ExecContext(ctx, statement); err != nil {
		t.Fatalf("execute storage provider SQL: %v", err)
	}
}

func expectStorageProviderSQLError(t *testing.T, ctx context.Context, database *sql.DB, statement, label string) {
	t.Helper()
	if _, err := database.ExecContext(ctx, statement); err == nil {
		t.Fatalf("%s unexpectedly succeeded: %s", label, strings.TrimSpace(statement))
	}
}
