package migrations

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProductionRBACCatalogSupportsFreshDatabaseWithoutSeed(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()

	if _, err := Run(ctx, database, "../../../migrations"); err != nil {
		t.Fatalf("apply fresh production migration chain: %v", err)
	}

	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM roles WHERE system_role`, 19)
	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM roles WHERE name='pm' AND display_name='Quản lý dự án' AND system_role AND NOT external_only`, 1)
	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM roles WHERE name='deputy_manager' AND display_name='Phó Quản Lý' AND system_role AND NOT external_only`, 1)
	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM role_permissions grant_row JOIN roles role ON role.id=grant_row.role_id JOIN permissions permission ON permission.id=grant_row.permission_id WHERE role.name='deputy_manager' AND permission.key IN ('project.supervision.view','project.view','pipeline.view','planning.view','operation.view','work_report.view','work_report.review','work_report.export')`, 8)
	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM role_permissions grant_row JOIN roles role ON role.id=grant_row.role_id JOIN permissions permission ON permission.id=grant_row.permission_id WHERE permission.key='project.supervision.view' AND role.name <> 'deputy_manager'`, 0)
	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM roles WHERE name='client' AND external_only`, 1)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM permissions permission
		WHERE permission.key NOT IN ('settings.update','staff.import','sample_data.create','project.reconciliation.view','project.supervision.view','account_recovery.manage')
		  AND NOT EXISTS (
			SELECT 1 FROM role_permissions assignment
			JOIN roles role ON role.id=assignment.role_id AND role.name='admin'
			WHERE assignment.permission_id=permission.id
		  )
	`, 0)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM role_permissions assignment
		JOIN roles role ON role.id=assignment.role_id
		JOIN permissions permission ON permission.id=assignment.permission_id
		WHERE permission.key='account_recovery.manage'
		  AND role.name='developer' AND role.system_role AND NOT role.external_only
	`, 1)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM role_permissions assignment
		JOIN roles role ON role.id=assignment.role_id
		JOIN permissions permission ON permission.id=assignment.permission_id
		WHERE permission.key='account_recovery.manage' AND role.name<>'developer'
	`, 0)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM permissions permission
		WHERE permission.key NOT IN ('project.reconciliation.view','project.supervision.view')
		  AND NOT EXISTS (
			SELECT 1 FROM role_permissions assignment
			JOIN roles role ON role.id=assignment.role_id AND role.name='developer'
			WHERE assignment.permission_id=permission.id
		  )
	`, 0)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM role_permissions assignment
		JOIN roles role ON role.id=assignment.role_id
		JOIN permissions permission ON permission.id=assignment.permission_id
		WHERE permission.key='project.reconciliation.view'
	`, 1)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM role_permissions assignment
		JOIN roles role ON role.id=assignment.role_id
		JOIN permissions permission ON permission.id=assignment.permission_id
		WHERE permission.key='project.reconciliation.view'
		  AND role.name <> 'accounting'
	`, 0)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM role_permissions assignment
		JOIN roles role ON role.id=assignment.role_id
		WHERE role.name='client'
	`, 0)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM role_permissions assignment
		JOIN roles role ON role.id=assignment.role_id
		JOIN permissions permission ON permission.id=assignment.permission_id
		WHERE (permission.key='settings.update' AND role.name <> 'developer')
		   OR (permission.key='staff.import' AND role.name <> 'developer')
		   OR (permission.key='sample_data.create' AND role.name <> 'developer')
		   OR (permission.key='staff.delete' AND role.name NOT IN ('admin','developer'))
		   OR (permission.key='event_log.view' AND role.name NOT IN ('admin','developer'))
		   OR (permission.key LIKE 'accounting.%' AND role.name NOT IN ('admin','developer'))
	`, 0)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM role_permissions assignment
		JOIN roles role ON role.id=assignment.role_id
		JOIN permissions permission ON permission.id=assignment.permission_id
		WHERE (role.name='pm' AND permission.key='construction.handoff.manage')
		   OR (role.name='qa_qc' AND permission.key='quality.inspect')
		   OR (role.name='director' AND permission.key='handover.verify')
		   OR (role.name='contract_officer' AND permission.key='seal.use')
	`, 4)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM role_permissions assignment
		JOIN roles role ON role.id=assignment.role_id
		JOIN permissions permission ON permission.id=assignment.permission_id
		WHERE permission.key='material.view' AND role.name='design'
	`, 1)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM role_permissions assignment
		JOIN roles role ON role.id=assignment.role_id
		JOIN permissions permission ON permission.id=assignment.permission_id
		WHERE role.name='design' AND permission.key LIKE 'warehouse.%'
	`, 0)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM role_permissions assignment
		JOIN roles role ON role.id=assignment.role_id
		JOIN permissions permission ON permission.id=assignment.permission_id
		WHERE permission.key='staff.delete' AND role.name IN ('admin','developer')
	`, 2)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema=current_schema()
		  AND ((table_name='users' AND column_name IN ('deleted_at','deleted_by'))
		    OR (table_name='staff' AND column_name IN ('deleted_at','deleted_by')))
	`, 4)
	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name='staff_deletion_batches'`, 1)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM pg_indexes
		WHERE schemaname=current_schema()
		  AND indexname IN (
		    'idx_projects_pm_staff_reference',
		    'idx_project_members_staff_active',
		    'idx_issues_assigned_to_active',
		    'idx_rework_requests_assigned_to_active',
		    'idx_acceptance_records_inspected_by_active',
		    'idx_construction_action_assignee_active',
		    'idx_recurring_plan_assignee_active',
		    'idx_recurring_occurrence_assignee_active',
		    'idx_construction_handoff_item_assignee_active'
		  )
	`, 9)
	if _, err := database.ExecContext(ctx, `
		INSERT INTO role_permissions (role_id,permission_id)
		SELECT role.id,permission.id
		FROM roles role CROSS JOIN permissions permission
		WHERE role.name='pm' AND permission.key='staff.delete'
	`); err == nil {
		t.Fatal("database trigger allowed staff.delete to be assigned outside Admin/Developer")
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO role_permissions(role_id,permission_id) SELECT role.id,permission.id FROM roles role CROSS JOIN permissions permission WHERE role.name='pm' AND permission.key='project.supervision.view'`); err == nil {
		t.Fatal("database allowed PM to acquire company supervision identity")
	}
	if _, err := database.ExecContext(ctx, `UPDATE roles SET name='deputy_custom' WHERE name='deputy_manager'`); err == nil {
		t.Fatal("database allowed supervision role identity to change")
	}
}

func assertRBACCount(t *testing.T, ctx context.Context, database queryRower, query string, want int) {
	t.Helper()
	var got int
	if err := database.QueryRowContext(ctx, query).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("RBAC count=%d, want %d for query %s", got, want, query)
	}
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func TestDeputyMigrationPreservesHistoryAndBackfillsOnlyActualReceipts(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()
	dir := t.TempDir()
	manifest, err := load("../../../migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range manifest {
		if compareVersion(migration.Version, "146") >= 0 {
			continue
		}
		raw, err := os.ReadFile(migration.Path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(migration.Path)), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Run(ctx, database, dir); err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`INSERT INTO users(id,email,password_hash,full_name,status) VALUES('19000000-0000-0000-0000-000000000001','migration-supervisor@test.local','disabled','Supervisor','active'),('19000000-0000-0000-0000-000000000002','migration-pm@test.local','disabled','PM','active')`,
		`INSERT INTO user_roles(user_id,role_id) SELECT '19000000-0000-0000-0000-000000000001',id FROM roles WHERE name='supervisor'`,
		`INSERT INTO projects(id,name,code,client_name) VALUES('39000000-0000-0000-0000-000000000001','Migration Project','DEP-MIG','Client')`,
		`INSERT INTO work_reports(id,source_kind,source_id,project_id,report_type,report_date,author_id,recipient_id,status,current_revision_no,reviewed_by,reviewed_at) VALUES('49000000-0000-0000-0000-000000000001','operation_daily_report','59000000-0000-0000-0000-000000000001','39000000-0000-0000-0000-000000000001','construction_daily','2026-10-01','19000000-0000-0000-0000-000000000001','19000000-0000-0000-0000-000000000002','acknowledged',2,'19000000-0000-0000-0000-000000000002',now())`,
		`INSERT INTO work_report_revisions(report_id,revision_no,snapshot_json,snapshot_sha256,submitted_by) VALUES('49000000-0000-0000-0000-000000000001',1,'{"work_summary":"Historical content"}',repeat('a',64),'19000000-0000-0000-0000-000000000001'),('49000000-0000-0000-0000-000000000001',2,'{"work_summary":"Current content"}',repeat('b',64),'19000000-0000-0000-0000-000000000001')`,
		`INSERT INTO work_report_events(report_id,revision_no,event_type,actor_id,comment) VALUES('49000000-0000-0000-0000-000000000001',1,'acknowledged','19000000-0000-0000-0000-000000000002','Actually viewed')`,
	}
	for _, statement := range statements {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	// This regression covers the historical V146 backfill. V150 deliberately
	// rejects populated preproduction databases instead of archiving them.
	if _, err := Run(ctx, database, retirementPredecessorDir(t)); err != nil {
		t.Fatal(err)
	}
	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM work_reports WHERE is_supervisor_report`, 1)
	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM work_report_read_receipts WHERE revision_no=1 AND comment='Actually viewed'`, 1)
	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM work_report_read_receipts WHERE revision_no=2`, 0)
	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM work_report_revisions WHERE (revision_no=1 AND snapshot_json->>'work_summary'='Historical content' AND snapshot_sha256=repeat('a',64)) OR (revision_no=2 AND snapshot_json->>'work_summary'='Current content' AND snapshot_sha256=repeat('b',64))`, 2)
	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM work_report_events`, 1)
	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM user_roles`, 1)
}
