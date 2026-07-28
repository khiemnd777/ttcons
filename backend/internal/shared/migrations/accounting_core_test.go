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

func TestAccountingUpgradeAcceptsLegacyFinanceStatuses(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()

	migrationsDir := filepath.Join("..", "..", "..", "migrations")
	legacyDir := t.TempDir()
	copyAccountingMigrationsThrough(t, migrationsDir, legacyDir, "72")
	if _, err := Run(ctx, database, legacyDir); err != nil {
		t.Fatalf("apply legacy migrations through V72: %v", err)
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO users (id,email,password_hash,full_name)
		VALUES ('73910000-0000-0000-0000-000000000001','accounting-upgrade@test.local','test','Kế toán nâng cấp');
		INSERT INTO projects (id,name,code,client_name)
		VALUES ('73910000-0000-0000-0000-000000000002','Công trình nâng cấp kế toán','ACC-UPGRADE-TEST','Khách hàng kiểm thử');
		INSERT INTO payables (
			id,project_id,payable_code,vendor_type,vendor_name,gross_amount,remaining_amount,status,created_by
		) VALUES (
			'73910000-0000-0000-0000-000000000010','73910000-0000-0000-0000-000000000002',
			'PAY-ACC-UPGRADE-001','supplier','Nhà cung cấp kiểm thử',100000,100000,'recorded',
			'73910000-0000-0000-0000-000000000001'
		);
		INSERT INTO receivables (
			id,project_id,receivable_code,client_name,contract_amount,gross_amount,remaining_amount,status,created_by
		) VALUES (
			'73910000-0000-0000-0000-000000000011','73910000-0000-0000-0000-000000000002',
			'REC-ACC-UPGRADE-001','Khách hàng kiểm thử',100000,100000,100000,'recorded',
			'73910000-0000-0000-0000-000000000001'
		);
		INSERT INTO payments (
			id,project_id,payment_code,payment_type,target_type,amount,payment_method,status,created_by
		) VALUES (
			'73910000-0000-0000-0000-000000000012','73910000-0000-0000-0000-000000000002',
			'PMT-ACC-UPGRADE-001','outbound',NULL,100000,'bank_transfer','waiting_approval',
			'73910000-0000-0000-0000-000000000001'
		),(
			'73910000-0000-0000-0000-000000000013','73910000-0000-0000-0000-000000000002',
			'PMT-ACC-UPGRADE-PAID','supplier_payment','payable',100000,'bank_transfer','paid',
			'73910000-0000-0000-0000-000000000001'
		),(
			'73910000-0000-0000-0000-000000000014','73910000-0000-0000-0000-000000000002',
			'PMT-ACC-UPGRADE-RECEIVED','client_receipt','receivable',100000,'bank_transfer','received',
			'73910000-0000-0000-0000-000000000001'
		);
	`); err != nil {
		t.Fatalf("seed legacy finance statuses: %v", err)
	}

	if _, err := Run(ctx, database, migrationsDir); err != nil {
		t.Fatalf("upgrade legacy finance rows through current schema: %v", err)
	}

	for table, wantStatus := range map[string]string{
		"payables":    "recorded",
		"receivables": "recorded",
	} {
		var status string
		if err := database.QueryRowContext(ctx, "SELECT status FROM "+table+" WHERE project_id=$1", "73910000-0000-0000-0000-000000000002").Scan(&status); err != nil {
			t.Fatalf("read upgraded %s status: %v", table, err)
		}
		if status != wantStatus {
			t.Fatalf("upgraded %s status=%q, want %q", table, status, wantStatus)
		}
	}
	for id, wantStatus := range map[string]string{
		"73910000-0000-0000-0000-000000000012": "waiting_approval",
		"73910000-0000-0000-0000-000000000013": "approved",
		"73910000-0000-0000-0000-000000000014": "approved",
	} {
		var status string
		var paidAt sql.NullTime
		if err := database.QueryRowContext(ctx, `SELECT status,paid_at FROM payments WHERE id=$1`, id).Scan(&status, &paidAt); err != nil {
			t.Fatalf("read upgraded payment %s: %v", id, err)
		}
		if status != wantStatus {
			t.Fatalf("upgraded payment %s status=%q, want %q", id, status, wantStatus)
		}
		if wantStatus == "approved" && !paidAt.Valid {
			t.Fatalf("upgraded payment %s must preserve an effective cash date", id)
		}
		if wantStatus == "approved" {
			var targetType sql.NullString
			if err := database.QueryRowContext(ctx, `SELECT target_type FROM payments WHERE id=$1`, id).Scan(&targetType); err != nil {
				t.Fatalf("read upgraded payment target %s: %v", id, err)
			}
			if targetType.Valid {
				t.Fatalf("upgraded payment %s retained an unusable target_type=%q without target_id", id, targetType.String)
			}
		}
	}
}

func TestAccountingCoreMigrationEnforcesBalancedImmutableJournal(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()

	migrationsDir := filepath.Join("..", "..", "..", "migrations")
	if _, err := Run(ctx, database, migrationsDir); err != nil {
		t.Fatal(err)
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO users (id,email,password_hash,full_name)
		VALUES ('73900000-0000-0000-0000-000000000001','accounting-core@test.local','test','Kế toán kiểm thử');
		INSERT INTO projects (id,name,code,client_name)
		VALUES ('73900000-0000-0000-0000-000000000002','Công trình kiểm thử kế toán','ACC-CORE-TEST','Khách hàng kiểm thử');
	`); err != nil {
		t.Fatal(err)
	}

	periodID := accountingCoreScalar(t, ctx, database, `
		SELECT id::text FROM accounting_periods
		WHERE book_id='73000000-0000-0000-0000-000000000001'
		  AND CURRENT_DATE BETWEEN date_from AND date_to
	`)
	debitAccountID := accountingCoreScalar(t, ctx, database, `
		SELECT id::text FROM accounting_accounts
		WHERE book_id='73000000-0000-0000-0000-000000000001' AND account_code='154'
	`)
	creditAccountID := accountingCoreScalar(t, ctx, database, `
		SELECT id::text FROM accounting_accounts
		WHERE book_id='73000000-0000-0000-0000-000000000001' AND account_code='331'
	`)

	entryID := "73900000-0000-0000-0000-000000000010"
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO journal_entries (
			id,book_id,project_id,period_id,entry_no,entry_date,document_date,description,
			source_type,source_id,source_event,status,created_by
		) VALUES (
			$1,'73000000-0000-0000-0000-000000000001','73900000-0000-0000-0000-000000000002',$2,
			'JV-ACC-CORE-001',CURRENT_DATE,CURRENT_DATE,'Ghi nhận chi phí kiểm thử',
			'test_source','73900000-0000-0000-0000-000000000020','test_post','draft',
			'73900000-0000-0000-0000-000000000001'
		)
	`, entryID, periodID); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO journal_entry_lines (entry_id,book_id,line_no,account_id,debit_amount,credit_amount)
		VALUES
			($1,'73000000-0000-0000-0000-000000000001',1,$2,100000,0),
			($1,'73000000-0000-0000-0000-000000000001',2,$3,0,100000)
	`, entryID, debitAccountID, creditAccountID); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE journal_entries
		SET status='posted',posted_by='73900000-0000-0000-0000-000000000001',posted_at=now()
		WHERE id=$1
	`, entryID); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("balanced journal commit failed: %v", err)
	}

	if _, err := database.ExecContext(ctx, `UPDATE journal_entries SET description='Không được sửa' WHERE id=$1`, entryID); err == nil || !strings.Contains(err.Error(), "bút toán đã ghi sổ là bất biến") {
		t.Fatalf("posted journal mutation error=%v, want immutable rejection", err)
	}

	unbalancedTx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unbalancedTx.ExecContext(ctx, `
		INSERT INTO journal_entries (
			id,book_id,project_id,period_id,entry_no,entry_date,description,
			source_type,source_id,source_event,status,created_by
		) VALUES (
			'73900000-0000-0000-0000-000000000011','73000000-0000-0000-0000-000000000001',
			'73900000-0000-0000-0000-000000000002',$1,'JV-ACC-CORE-002',CURRENT_DATE,
			'Bút toán lệch','test_source','73900000-0000-0000-0000-000000000021','test_unbalanced','draft',
			'73900000-0000-0000-0000-000000000001'
		)
	`, periodID); err != nil {
		_ = unbalancedTx.Rollback()
		t.Fatal(err)
	}
	if _, err := unbalancedTx.ExecContext(ctx, `
		INSERT INTO journal_entry_lines (entry_id,book_id,line_no,account_id,debit_amount,credit_amount)
		VALUES
			('73900000-0000-0000-0000-000000000011','73000000-0000-0000-0000-000000000001',1,$1,100000,0),
			('73900000-0000-0000-0000-000000000011','73000000-0000-0000-0000-000000000001',2,$2,0,90000)
	`, debitAccountID, creditAccountID); err != nil {
		_ = unbalancedTx.Rollback()
		t.Fatal(err)
	}
	if _, err := unbalancedTx.ExecContext(ctx, `
		UPDATE journal_entries
		SET status='posted',posted_by='73900000-0000-0000-0000-000000000001',posted_at=now()
		WHERE id='73900000-0000-0000-0000-000000000011'
	`); err != nil {
		_ = unbalancedTx.Rollback()
		t.Fatal(err)
	}
	if err := unbalancedTx.Commit(); err == nil || !strings.Contains(err.Error(), "chưa cân") {
		t.Fatalf("unbalanced journal commit error=%v, want balance rejection", err)
	}
}

func accountingCoreScalar(t *testing.T, ctx context.Context, database *sql.DB, query string) string {
	t.Helper()
	var value string
	if err := database.QueryRowContext(ctx, query).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func copyAccountingMigrationsThrough(t *testing.T, sourceDir, targetDir, maxVersion string) {
	t.Helper()
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		t.Fatalf("read migrations for accounting fixture: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, _ := parseName(entry.Name())
		if version == "" || compareVersion(version, maxVersion) > 0 {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(sourceDir, entry.Name()))
		if err != nil {
			t.Fatalf("read migration fixture %s: %v", entry.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, entry.Name()), raw, 0o600); err != nil {
			t.Fatalf("write migration fixture %s: %v", entry.Name(), err)
		}
	}
}
