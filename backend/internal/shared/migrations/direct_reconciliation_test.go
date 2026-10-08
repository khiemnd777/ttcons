package migrations

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestDirectReconciliationMigrationRejectsExistingBasis(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()
	if _, err := Run(ctx, database, retirementMigrationDirThrough(t, "150")); err != nil {
		t.Fatal(err)
	}
	insertReconciliationSourceFixtures(t, ctx, database, "accounting_request_bases")
	_, err := Run(ctx, database, "../../../migrations")
	if err == nil || !strings.Contains(err.Error(), "requires empty accounting business tables") || !strings.Contains(err.Error(), "accounting_request_bases") {
		t.Fatalf("expected zero-data retirement guard, got %v", err)
	}
	var intact bool
	if err := database.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM accounting_request_bases)
		AND to_regclass(format('%I.accounting_requests',current_schema())) IS NOT NULL
		AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version='151')`).Scan(&intact); err != nil || !intact {
		t.Fatalf("rejected V151 changed basis/proposal schema: intact=%v err=%v", intact, err)
	}
}

func TestDirectReconciliationPermissionMigrationPreservesActualGrants(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()
	if _, err := Run(ctx, database, retirementMigrationDirThrough(t, "150")); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO roles (id,name,display_name)
		VALUES ('15100000-0000-0000-0000-000000000010','fixture_reader','Fixture reader');
		INSERT INTO role_permissions (role_id,permission_id)
		SELECT '15100000-0000-0000-0000-000000000010',id FROM permissions WHERE key='accounting_request.view'`); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, database, "../../../migrations"); err != nil {
		t.Fatal(err)
	}
	var view, manage bool
	if err := database.QueryRowContext(ctx, `SELECT
		EXISTS (SELECT 1 FROM role_permissions assignment JOIN permissions permission ON permission.id=assignment.permission_id
		  WHERE assignment.role_id='15100000-0000-0000-0000-000000000010' AND permission.key='accounting_reconciliation.view'),
		EXISTS (SELECT 1 FROM role_permissions assignment JOIN permissions permission ON permission.id=assignment.permission_id
		  WHERE assignment.role_id='15100000-0000-0000-0000-000000000010' AND permission.key='accounting_reconciliation.manage')
	`).Scan(&view, &manage); err != nil || !view || manage {
		t.Fatalf("permission migration must preserve view without escalating management: view=%v manage=%v err=%v", view, manage, err)
	}
}

func TestDirectReconciliationCapsMISARecordAndPreservesImmutableIdentity(t *testing.T) {
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
	insertReconciliationSourceFixtures(t, ctx, database, "accounting_reconciliation_bases")
	if _, err := database.ExecContext(ctx, `
		INSERT INTO accounting_misa_batches (id,project_id,entity_code,filename,file_content,digest,status,created_by,confirmed_at)
		VALUES ('15100000-0000-0000-0000-000000000004','15100000-0000-0000-0000-000000000002',
		  'TEST','test.csv','fixture','fixture-digest','confirmed','15100000-0000-0000-0000-000000000001',now());
		INSERT INTO accounting_misa_records (id,batch_id,project_id,entity_code,external_id,line_key,kind,direction,partner_id,currency,amount,record_date)
		VALUES ('15100000-0000-0000-0000-000000000005','15100000-0000-0000-0000-000000000004',
		  '15100000-0000-0000-0000-000000000002','TEST','TX-1','1','transaction','outbound',
		  '15100000-0000-0000-0000-000000000003','VND',200,CURRENT_DATE);
	`); err != nil {
		t.Fatal(err)
	}
	// A base is 100; matching 150 is an allowed visible difference, never a
	// capacity violation. Only the imported transaction's 200 is a hard cap.
	if _, err := database.ExecContext(ctx, reconciliationAllocationSQL("15100000-0000-0000-0000-000000000007", "150")); err != nil {
		t.Fatalf("allocation above base must remain allowed: %v", err)
	}
	if _, err := database.ExecContext(ctx, reconciliationAllocationSQL("15100000-0000-0000-0000-000000000008", "51")); err == nil || !strings.Contains(err.Error(), "exceeds imported transaction amount") {
		t.Fatalf("MISA transaction cap must be enforced, got %v", err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE accounting_misa_records SET active=false
		WHERE id='15100000-0000-0000-0000-000000000005'`); err == nil || !strings.Contains(err.Error(), "Revoke active MISA allocations") {
		t.Fatalf("allocated observations must not be replaced, got %v", err)
	}
	for _, statement := range []string{
		`UPDATE accounting_reconciliation_bases SET amount=101`,
		`UPDATE accounting_misa_allocations SET amount=1`,
		`DELETE FROM accounting_misa_allocations`,
	} {
		if _, err := database.ExecContext(ctx, statement); err == nil {
			t.Fatalf("immutable source/allocation mutation accepted: %s", statement)
		}
	}
	if _, err := database.ExecContext(ctx, `UPDATE accounting_misa_allocations SET active=false,
		revoked_by='15100000-0000-0000-0000-000000000001',revoked_at=now(),revoke_reason=NULL`); err == nil {
		t.Fatal("allocation revocation must reject a missing reason at the database boundary")
	}
	if _, err := database.ExecContext(ctx, `UPDATE accounting_misa_allocations SET active=false,
		revoked_by='15100000-0000-0000-0000-000000000001',revoked_at=now(),revoke_reason='Fixture correction';
		UPDATE accounting_misa_records SET active=false WHERE id='15100000-0000-0000-0000-000000000005';`); err != nil {
		t.Fatalf("explicit revocation must allow controlled replacement: %v", err)
	}
	var permissions int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM permissions WHERE key IN
		('accounting_reconciliation.view','accounting_reconciliation.manage','accounting_misa.reconcile')`).Scan(&permissions); err != nil || permissions != 3 {
		t.Fatalf("current reconciliation permissions: %d err=%v", permissions, err)
	}
	var proposals int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM permissions WHERE key LIKE 'accounting_request.%'`).Scan(&proposals); err != nil || proposals != 0 {
		t.Fatalf("retired proposal permissions remain: %d err=%v", proposals, err)
	}
}

func insertReconciliationSourceFixtures(t *testing.T, ctx context.Context, database *sql.DB, basisTable string) {
	t.Helper()
	if basisTable != "accounting_request_bases" && basisTable != "accounting_reconciliation_bases" {
		t.Fatal("unexpected fixture table")
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO users (id,email,password_hash,full_name)
		VALUES ('15100000-0000-0000-0000-000000000001','reconciliation-fixture@test.local','disabled','Fixture');
		INSERT INTO projects (id,name,code,client_name)
		VALUES ('15100000-0000-0000-0000-000000000002','Fixture','RECON-SCHEMA','Fixture');
		INSERT INTO partners (id,partner_code,name,partner_types)
		VALUES ('15100000-0000-0000-0000-000000000003','RECON-PARTNER','Fixture','["supplier"]');
		INSERT INTO procurement_requests (id,project_id,request_code,title,status)
		VALUES ('15100000-0000-0000-0000-000000000009','15100000-0000-0000-0000-000000000002','RECON-SOURCE','Fixture','ordered');
		INSERT INTO `+basisTable+` (id,project_id,source_type,source_id,partner_id,direction,currency,amount,note,snapshot,created_by)
		VALUES ('15100000-0000-0000-0000-000000000006','15100000-0000-0000-0000-000000000002',
		  'procurement_request','15100000-0000-0000-0000-000000000009','15100000-0000-0000-0000-000000000003',
		  'outbound','VND',100,'Fixture','{}','15100000-0000-0000-0000-000000000001');`); err != nil {
		t.Fatal(err)
	}
}

func reconciliationAllocationSQL(id, amount string) string {
	// Inputs are only static test literals, not application SQL construction.
	return `INSERT INTO accounting_misa_allocations (id,project_id,basis_id,record_id,amount,created_by) VALUES ('` + id + `',
		'15100000-0000-0000-0000-000000000002','15100000-0000-0000-0000-000000000006',
		'15100000-0000-0000-0000-000000000005',` + amount + `,'15100000-0000-0000-0000-000000000001')`
}
