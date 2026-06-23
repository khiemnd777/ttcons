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
	dsn := os.Getenv("CONSFLOW_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CONSFLOW_TEST_DATABASE_URL to run DB-backed migration tests")
	}

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

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
