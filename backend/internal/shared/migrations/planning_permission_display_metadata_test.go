package migrations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPlanningPermissionDisplayMetadataMigrationRepairsExistingRow(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()

	if _, err := database.ExecContext(ctx, `
		CREATE TABLE permissions (
			id UUID PRIMARY KEY,
			key TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			description TEXT,
			module TEXT NOT NULL
		);
		INSERT INTO permissions (id, key, name, description, module)
		VALUES (
			'21000000-0000-0000-0000-000000000096',
			'planning.edit',
			'Sửa planning',
			NULL,
			'legacy'
		);
	`); err != nil {
		t.Fatal(err)
	}

	migrationPath := filepath.Join("..", "..", "..", "migrations", "V96__planning_permission_display_metadata.sql")
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

	var name, description, module string
	var count int
	if err := database.QueryRowContext(ctx, `
		SELECT name, description, module,
			(SELECT COUNT(*) FROM permissions WHERE key='planning.edit')
		FROM permissions
		WHERE key='planning.edit'
	`).Scan(&name, &description, &module, &count); err != nil {
		t.Fatal(err)
	}
	if name != "Cập nhật kế hoạch" || description != "Cập nhật nội dung kế hoạch được phân công" || module != "planning" || count != 1 {
		t.Fatalf("planning.edit metadata = name=%q description=%q module=%q count=%d", name, description, module, count)
	}
}
