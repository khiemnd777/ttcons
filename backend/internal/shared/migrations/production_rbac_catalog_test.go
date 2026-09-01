package migrations

import (
	"context"
	"database/sql"
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

	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM roles WHERE system_role`, 18)
	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM roles WHERE name='client' AND external_only`, 1)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM permissions permission
		WHERE permission.key NOT IN ('settings.update','staff.import','sample_data.create','project.reconciliation.view')
		  AND NOT EXISTS (
			SELECT 1 FROM role_permissions assignment
			JOIN roles role ON role.id=assignment.role_id AND role.name='admin'
			WHERE assignment.permission_id=permission.id
		  )
	`, 0)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM permissions permission
		WHERE permission.key <> 'project.reconciliation.view'
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
