package migrations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStaffAccountSecurityPermissionMigrationGrantsOnlyAdmin(t *testing.T) {
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
			key TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			description TEXT,
			module TEXT NOT NULL
		);
		CREATE TABLE role_permissions (
			role_id UUID NOT NULL REFERENCES roles(id),
			permission_id UUID NOT NULL REFERENCES permissions(id),
			PRIMARY KEY (role_id, permission_id)
		);
		INSERT INTO roles (id, name) VALUES
			('20000000-0000-0000-0000-000000000109', 'admin'),
			('20000000-0000-0000-0000-000000000110', 'director');
	`); err != nil {
		t.Fatal(err)
	}

	migrationPath := filepath.Join("..", "..", "..", "migrations", "V109__staff_account_security_permission.sql")
	migrationSQL, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, string(migrationSQL)); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, string(migrationSQL)); err != nil {
		t.Fatalf("migration must be idempotent: %v", err)
	}

	var permissionName, permissionDescription, permissionModule string
	if err := database.QueryRowContext(ctx, `
		SELECT name, description, module
		FROM permissions
		WHERE key='staff.account_security.manage'
	`).Scan(&permissionName, &permissionDescription, &permissionModule); err != nil {
		t.Fatal(err)
	}
	if permissionName != "Quản trị bảo mật tài khoản nhân sự" || permissionDescription == "" || permissionModule != "staff" {
		t.Fatalf("unexpected permission metadata: name=%q description=%q module=%q", permissionName, permissionDescription, permissionModule)
	}

	var adminCount, directorCount int
	if err := database.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE role.name='admin'),
			COUNT(*) FILTER (WHERE role.name='director')
		FROM role_permissions role_permission
		JOIN roles role ON role.id=role_permission.role_id
		JOIN permissions permission ON permission.id=role_permission.permission_id
		WHERE permission.key='staff.account_security.manage'
	`).Scan(&adminCount, &directorCount); err != nil {
		t.Fatal(err)
	}
	if adminCount != 1 || directorCount != 0 {
		t.Fatalf("permission grants admin=%d director=%d, want 1/0", adminCount, directorCount)
	}
}
