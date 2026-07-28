package migrations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOfficialWorkflowActorPermissionsMigrationGrantsExistingPMRole(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()

	if _, err := database.ExecContext(ctx, `
		CREATE TABLE roles (
			id UUID PRIMARY KEY,
			name TEXT NOT NULL UNIQUE
		);
		CREATE TABLE permissions (
			id UUID PRIMARY KEY,
			key TEXT NOT NULL UNIQUE
		);
		CREATE TABLE role_permissions (
			role_id UUID NOT NULL REFERENCES roles(id),
			permission_id UUID NOT NULL REFERENCES permissions(id),
			PRIMARY KEY (role_id, permission_id)
		);
		INSERT INTO roles (id, name)
		VALUES ('20000000-0000-0000-0000-000000000002', 'pm');
		INSERT INTO permissions (id, key)
		VALUES
			('21000000-0000-0000-0000-000000000001', 'planning.view'),
			('21000000-0000-0000-0000-000000000002', 'planning.edit');
	`); err != nil {
		t.Fatal(err)
	}

	migrationPath := filepath.Join("..", "..", "..", "migrations", "V72__official_workflow_actor_permissions.sql")
	migrationSQL, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, string(migrationSQL)); err != nil {
		t.Fatal(err)
	}

	var permissionCount int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM role_permissions role_permission
		JOIN roles role ON role.id = role_permission.role_id
		JOIN permissions permission ON permission.id = role_permission.permission_id
		WHERE role.name = 'pm'
			AND permission.key IN ('planning.view', 'planning.edit')
	`).Scan(&permissionCount); err != nil {
		t.Fatal(err)
	}
	if permissionCount != 2 {
		t.Fatalf("PM planning permission count = %d, want 2", permissionCount)
	}
}
