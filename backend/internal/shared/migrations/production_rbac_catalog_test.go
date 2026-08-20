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

	migrationsDir := filepath.Join("..", "..", "..", "migrations")
	if _, err := Run(ctx, database, migrationsDir); err != nil {
		t.Fatalf("apply fresh production migration chain: %v", err)
	}
	migrationSQL, err := os.ReadFile(filepath.Join(migrationsDir, "V112__production_rbac_catalog.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, string(migrationSQL)); err != nil {
		t.Fatalf("V112 must remain idempotent: %v", err)
	}

	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM roles WHERE system_role`, 18)
	assertRBACCount(t, ctx, database, `SELECT COUNT(*) FROM roles WHERE name='client' AND external_only`, 1)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM permissions permission
		WHERE permission.key <> 'settings.update'
		  AND NOT EXISTS (
			SELECT 1 FROM role_permissions assignment
			JOIN roles role ON role.id=assignment.role_id AND role.name='admin'
			WHERE assignment.permission_id=permission.id
		  )
	`, 0)
	assertRBACCount(t, ctx, database, `
		SELECT COUNT(*)
		FROM permissions permission
		WHERE NOT EXISTS (
			SELECT 1 FROM role_permissions assignment
			JOIN roles role ON role.id=assignment.role_id AND role.name='developer'
			WHERE assignment.permission_id=permission.id
		)
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
