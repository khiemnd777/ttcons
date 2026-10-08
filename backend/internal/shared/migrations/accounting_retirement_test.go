package migrations

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAccountingRetirementFreshInstallAndEmptyUpgrade(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	for _, name := range []string{"fresh", "empty_V149_upgrade", "V149_missing_book_regression"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
			defer cleanup()
			database := openMigrationTestDatabase(t, ctx, dsn, schema)
			defer database.Close()
			if name != "fresh" {
				if _, err := Run(ctx, database, retirementPredecessorDir(t)); err != nil {
					t.Fatalf("apply V149 predecessor: %v", err)
				}
				if _, err := database.ExecContext(ctx, `INSERT INTO system_settings (key,value_json)
					VALUES ('client_payment_min_percent','30'::jsonb) ON CONFLICT (key) DO NOTHING`); err != nil {
					t.Fatal(err)
				}
			}
			if name == "V149_missing_book_regression" {
				if _, err := database.ExecContext(ctx, `DELETE FROM tax_periods; DELETE FROM tax_codes; DELETE FROM accounting_posting_rules;
					DELETE FROM accounting_periods; DELETE FROM accounting_accounts; DELETE FROM accounting_books;`); err != nil {
					t.Fatal(err)
				}
				_, err := database.ExecContext(ctx, `INSERT INTO projects (name,code,client_name)
					VALUES ('Công trình mới','PRJ-78805','Khách hàng mới')`)
				if err == nil || !strings.Contains(err.Error(), "projects_accounting_book_fk") {
					t.Fatalf("V149 must reproduce the missing-book startup defect, got %v", err)
				}
			}
			if _, err := Run(ctx, database, "../../../migrations"); err != nil {
				t.Fatalf("apply retirement chain: %v", err)
			}
			assertRetirementSchema(t, ctx, database)
			// This is the exact INSERT that failed when the default book was missing.
			if _, err := database.ExecContext(ctx, `INSERT INTO projects (name,code,client_name)
				VALUES ('Công trình mới','PRJ-78805','Khách hàng mới')`); err != nil {
				t.Fatalf("create a project without an accounting book: %v", err)
			}
			if _, err := Run(ctx, database, "../../../migrations"); err != nil {
				t.Fatalf("migration rerun must skip the applied preproduction cleanup: %v", err)
			}
		})
	}
}

func TestAccountingRetirementRejectsBusinessDataAtomically(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()
	if _, err := Run(ctx, database, retirementPredecessorDir(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO projects (name,code,client_name)
		VALUES ('Không được xóa','RETIREMENT-GUARD','Khách hàng kiểm thử')`); err != nil {
		t.Fatal(err)
	}
	_, err := Run(ctx, database, "../../../migrations")
	if err == nil || !strings.Contains(err.Error(), "requires empty business tables") || !strings.Contains(err.Error(), "projects") {
		t.Fatalf("expected fail-closed data precondition, got %v", err)
	}
	var intact bool
	if err := database.QueryRowContext(ctx, `SELECT to_regclass('accounting_books') IS NOT NULL
		AND EXISTS (SELECT 1 FROM projects WHERE code='RETIREMENT-GUARD')
		AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version='150')`).Scan(&intact); err != nil || !intact {
		t.Fatalf("rejected cleanup changed existing schema/data/history: intact=%v err=%v", intact, err)
	}
}

func TestAccountingRetirementRejectsUnexpectedBookAndClosedPeriods(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	for _, fixture := range []struct {
		name string
		sql  string
		want string
	}{
		{"extra book", `INSERT INTO accounting_books(book_code,name) VALUES ('UNEXPECTED','Sổ tự tạo')`, "migration-owned default book"},
		{"closed accounting period", `UPDATE accounting_periods SET status='closed',closed_at=now(),
			closed_by='79900000-0000-0000-0000-000000000001'`, "closed accounting or tax periods"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
			defer cleanup()
			database := openMigrationTestDatabase(t, ctx, dsn, schema)
			defer database.Close()
			if _, err := Run(ctx, database, retirementPredecessorDir(t)); err != nil {
				t.Fatal(err)
			}
			if _, err := database.ExecContext(ctx, fixture.sql); err != nil {
				t.Fatal(err)
			}
			_, err := Run(ctx, database, "../../../migrations")
			if err == nil || !strings.Contains(err.Error(), fixture.want) {
				t.Fatalf("expected retirement master-data guard, got %v", err)
			}
			var intact bool
			if err := database.QueryRowContext(ctx, `SELECT to_regclass(format('%I.accounting_books',current_schema())) IS NOT NULL
				AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version='150')`).Scan(&intact); err != nil || !intact {
				t.Fatalf("rejected cleanup changed schema/history: intact=%v err=%v", intact, err)
			}
		})
	}
}

func TestCurrentSeedIsAdditiveAndIdempotent(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()
	if _, err := Run(ctx, database, "../../../migrations"); err != nil {
		t.Fatal(err)
	}
	// An existing local record must survive intentional demo bootstrap.
	if _, err := database.ExecContext(ctx, `INSERT INTO projects (name,code,client_name)
		VALUES ('Công trình đang thiết lập','SEED-PRESERVE','Khách hàng kiểm thử')`); err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		for _, file := range []string{"seed.sql", "validate_current_seed.sql"} {
			raw, err := os.ReadFile(filepath.Join("../../../seed", file))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := database.ExecContext(ctx, string(raw)); err != nil {
				t.Fatalf("seed pass %d file %s: %v", pass, file, err)
			}
		}
	}
	var preserved, duplicated bool
	if err := database.QueryRowContext(ctx, `SELECT
		EXISTS (SELECT 1 FROM projects WHERE code='SEED-PRESERVE') AND (SELECT count(*) FROM projects)=1,
		EXISTS (SELECT 1 FROM user_roles WHERE project_id IS NULL GROUP BY user_id,role_id HAVING count(*)>1)
	`).Scan(&preserved, &duplicated); err != nil || !preserved || duplicated {
		t.Fatalf("seed must preserve data and not duplicate grants: preserved=%v duplicated=%v err=%v", preserved, duplicated, err)
	}
}

func assertRetirementSchema(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()
	for _, table := range []string{
		"accounting_books", "accounting_accounts", "accounting_periods", "accounting_posting_rules",
		"journal_entries", "journal_entry_lines", "accounting_backfill_issues", "tax_periods",
		"tax_adjustments", "vat_return_snapshots", "tax_period_events", "project_material_costs", "project_cash_requests",
		"accounting_budgets", "budget_movements", "payment_reversals", "payables", "receivables", "payments",
		"input_invoices", "output_invoices", "invoice_tax_lines", "tax_codes", "payment_allocations", "receipt_allocations",
		"payable_payment_schedules", "receivable_payment_schedules", "advance_requests", "client_advance_receipts",
		"contract_retentions", "contract_retention_releases",
		"accounting_request_bases", "accounting_requests", "accounting_request_events",
	} {
		var exists bool
		if err := database.QueryRowContext(ctx, `SELECT to_regclass(format('%I.%I',current_schema(),$1::text)) IS NOT NULL`, table).Scan(&exists); err != nil || exists {
			t.Fatalf("retired table %s still exists: %v", table, err)
		}
	}
	for _, table := range []string{"accounting_handoffs", "accounting_reconciliation_bases",
		"accounting_misa_batches", "accounting_misa_records", "accounting_misa_allocations",
		"accounting_misa_reviews", "projects", "contracts", "contract_versions", "contract_milestones", "contract_variations"} {
		var exists bool
		if err := database.QueryRowContext(ctx, `SELECT to_regclass(format('%I.%I',current_schema(),$1::text)) IS NOT NULL`, table).Scan(&exists); err != nil || !exists {
			t.Fatalf("retained table %s is missing: %v", table, err)
		}
	}
	var dimensions int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.columns
		WHERE table_schema=current_schema() AND column_name IN ('accounting_book_id','book_id','cost_account_id',
		'revenue_account_id','cash_account_id','expense_account_id','journal_entry_id',
		'original_journal_entry_id','reversal_journal_entry_id')`).Scan(&dimensions); err != nil || dimensions != 0 {
		t.Fatalf("retired dimensions remain: count=%d err=%v", dimensions, err)
	}
	var stale int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM permissions WHERE key LIKE 'accounting.%'
		OR key LIKE 'project_finance.%' OR key IN ('report.finance.view','contract.retention_release')`).Scan(&stale); err != nil || stale != 0 {
		t.Fatalf("retired permissions remain: count=%d err=%v", stale, err)
	}
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM system_settings WHERE key='client_payment_min_percent'`).Scan(&stale); err != nil || stale != 0 {
		t.Fatalf("retired collection setting remains: count=%d err=%v", stale, err)
	}
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM pipeline_templates template
		JOIN pipeline_template_nodes node ON node.template_id=template.id
		JOIN pipeline_template_steps step ON step.node_id=node.id WHERE template.status='published'
		AND (step.required_permission LIKE 'accounting.%' OR step.required_permission LIKE 'project_finance.%'
		OR step.required_permission IN ('report.finance.view','contract.retention_release')
		OR step.domain_action_key IN ('source.project_finance_close_ready','contract.open_obligation','contract.release_retention'))
	`).Scan(&stale); err != nil || stale != 0 {
		t.Fatalf("published templates still reference retired capabilities: count=%d err=%v", stale, err)
	}
}

func retirementPredecessorDir(t *testing.T) string {
	return retirementMigrationDirThrough(t, "149")
}

func retirementMigrationDirThrough(t *testing.T, maxVersion string) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir("../../../migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		version, _ := parseName(entry.Name())
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") || compareVersion(version, maxVersion) > 0 {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("../../../migrations", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
