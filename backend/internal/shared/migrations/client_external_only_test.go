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

func TestClientExternalOnlyMigrationDeactivatesAppAccessAndPreservesHistory(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()
	prepareClientExternalOnlySchema(t, ctx, database)

	if _, err := database.ExecContext(ctx, `
		INSERT INTO roles (id,name,display_name,system_role) VALUES
			('20000000-0000-0000-0000-000000000012','client','Chủ đầu tư',true),
			('20000000-0000-0000-0000-000000000003','pm','PM',true);
		INSERT INTO permissions (id,key) VALUES ('21000000-0000-0000-0000-000000000001','project.view');
		INSERT INTO role_permissions (role_id,permission_id) VALUES ('20000000-0000-0000-0000-000000000012','21000000-0000-0000-0000-000000000001');
		INSERT INTO users (id,status) VALUES ('10000000-0000-0000-0000-000000000012','active');
		INSERT INTO user_roles (user_id,role_id) VALUES ('10000000-0000-0000-0000-000000000012','20000000-0000-0000-0000-000000000012');
		INSERT INTO staff (id,user_id,status) VALUES ('22000000-0000-0000-0000-000000000012','10000000-0000-0000-0000-000000000012','active');
		INSERT INTO project_members (id,staff_id,active) VALUES ('23000000-0000-0000-0000-000000000012','22000000-0000-0000-0000-000000000012',true);
		INSERT INTO auth_sessions (id,user_id) VALUES ('24000000-0000-0000-0000-000000000012','10000000-0000-0000-0000-000000000012');
		INSERT INTO work_items (id,status,role_ids) VALUES ('25000000-0000-0000-0000-000000000012','ready','["Chủ đầu tư","PM"]');
		INSERT INTO approval_requests (id,status,required_roles) VALUES ('26000000-0000-0000-0000-000000000012','pending','["Chủ đầu tư","PM"]');
	`); err != nil {
		t.Fatal(err)
	}
	executeClientExternalOnlyMigration(t, ctx, database)

	assertMigrationScalar(t, ctx, database, `SELECT external_only FROM roles WHERE name='client'`, true)
	assertMigrationScalar(t, ctx, database, `SELECT status FROM users WHERE id='10000000-0000-0000-0000-000000000012'`, "inactive")
	assertMigrationScalar(t, ctx, database, `SELECT status FROM staff WHERE id='22000000-0000-0000-0000-000000000012'`, "inactive")
	assertMigrationScalar(t, ctx, database, `SELECT active FROM project_members WHERE id='23000000-0000-0000-0000-000000000012'`, false)
	assertMigrationScalar(t, ctx, database, `SELECT revoked_at IS NOT NULL FROM auth_sessions WHERE id='24000000-0000-0000-0000-000000000012'`, true)
	assertMigrationScalar(t, ctx, database, `SELECT COUNT(*) FROM role_permissions WHERE role_id='20000000-0000-0000-0000-000000000012'`, int64(0))
	assertMigrationScalar(t, ctx, database, `SELECT COUNT(*) FROM user_roles WHERE user_id='10000000-0000-0000-0000-000000000012'`, int64(1))
	assertMigrationScalar(t, ctx, database, `SELECT role_ids::text FROM work_items WHERE id='25000000-0000-0000-0000-000000000012'`, `["Chủ đầu tư", "PM"]`)
}

func TestClientExternalOnlyMigrationFailsClosed(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	scenarios := []struct {
		name      string
		fixture   string
		errorText string
	}{
		{
			name: "mixed internal and external user roles",
			fixture: `
				INSERT INTO users (id,status) VALUES ('10000000-0000-0000-0000-000000000001','active');
				INSERT INTO user_roles (user_id,role_id) VALUES
					('10000000-0000-0000-0000-000000000001','20000000-0000-0000-0000-000000000012'),
					('10000000-0000-0000-0000-000000000001','20000000-0000-0000-0000-000000000003');
			`,
			errorText: "also has an internal role",
		},
		{
			name:      "external-only open work item",
			fixture:   `INSERT INTO work_items (id,status,role_ids) VALUES ('25000000-0000-0000-0000-000000000001','ready','["Chủ đầu tư"]');`,
			errorText: "open work item requires only external roles",
		},
		{
			name:      "external-only pending approval",
			fixture:   `INSERT INTO approval_requests (id,status,required_roles) VALUES ('26000000-0000-0000-0000-000000000001','pending','["Chủ đầu tư"]');`,
			errorText: "pending approval requires only external roles",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
			defer cleanup()
			database := openMigrationTestDatabase(t, ctx, dsn, schema)
			defer database.Close()
			prepareClientExternalOnlySchema(t, ctx, database)
			if _, err := database.ExecContext(ctx, `
				INSERT INTO roles (id,name,display_name,system_role) VALUES
					('20000000-0000-0000-0000-000000000012','client','Chủ đầu tư',true),
					('20000000-0000-0000-0000-000000000003','pm','PM',true);
			`+scenario.fixture); err != nil {
				t.Fatal(err)
			}
			migrationSQL, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "V89__client_external_only.sql"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := database.ExecContext(ctx, string(migrationSQL)); err == nil || !strings.Contains(err.Error(), scenario.errorText) {
				t.Fatalf("migration error = %v, want text %q", err, scenario.errorText)
			}
		})
	}
}

func prepareClientExternalOnlySchema(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()
	if _, err := database.ExecContext(ctx, `
		CREATE TABLE roles (id UUID PRIMARY KEY,name TEXT NOT NULL UNIQUE,display_name TEXT NOT NULL,system_role BOOLEAN NOT NULL DEFAULT false);
		CREATE TABLE permissions (id UUID PRIMARY KEY,key TEXT NOT NULL UNIQUE);
		CREATE TABLE role_permissions (role_id UUID NOT NULL,permission_id UUID NOT NULL,PRIMARY KEY(role_id,permission_id));
		CREATE TABLE users (id UUID PRIMARY KEY,status TEXT NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
		CREATE TABLE user_roles (user_id UUID NOT NULL,role_id UUID NOT NULL);
		CREATE TABLE staff (id UUID PRIMARY KEY,user_id UUID,status TEXT NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
		CREATE TABLE project_members (id UUID PRIMARY KEY,staff_id UUID NOT NULL,active BOOLEAN NOT NULL DEFAULT true,updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
		CREATE TABLE auth_sessions (id UUID PRIMARY KEY,user_id UUID NOT NULL,revoked_at TIMESTAMPTZ);
		CREATE TABLE work_items (id UUID PRIMARY KEY,status TEXT NOT NULL,role_ids JSONB NOT NULL DEFAULT '[]');
		CREATE TABLE approval_requests (id UUID PRIMARY KEY,status TEXT NOT NULL,required_roles JSONB NOT NULL DEFAULT '[]');
	`); err != nil {
		t.Fatal(err)
	}
}

func executeClientExternalOnlyMigration(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()
	migrationSQL, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "V89__client_external_only.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, string(migrationSQL)); err != nil {
		t.Fatal(err)
	}
}

func assertMigrationScalar[T comparable](t *testing.T, ctx context.Context, database *sql.DB, query string, want T) {
	t.Helper()
	var got T
	if err := database.QueryRowContext(ctx, query).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("query %q = %#v, want %#v", query, got, want)
	}
}
