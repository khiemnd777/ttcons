package migrations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProjectManagerDisplayNameUpgradePreservesCatalogAndWorkflow(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	db := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer db.Close()
	partial := t.TempDir()
	sources, err := filepath.Glob("../../../migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		if filepath.Base(source) == "V149__project_manager_display_name.sql" {
			continue
		}
		contents, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(partial, filepath.Base(source)), contents, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Run(ctx, db, partial); err != nil {
		t.Fatal(err)
	}
	fingerprintSQL := `SELECT jsonb_build_object(
  'roles',(SELECT jsonb_agg(CASE WHEN name='pm' THEN to_jsonb(r)-'display_name' ELSE to_jsonb(r) END ORDER BY id) FROM roles r),
  'permissions',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM permissions p),
  'grants',(SELECT jsonb_agg(to_jsonb(g) ORDER BY role_id,permission_id) FROM role_permissions g),
  'members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM project_members m),
  'assignments',(SELECT jsonb_agg(to_jsonb(a) ORDER BY user_id,role_id,project_id) FROM user_roles a),
  'nodes',(SELECT jsonb_agg(to_jsonb(n) ORDER BY id) FROM pipeline_template_nodes n),
  'steps',(SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM pipeline_template_steps s),
  'run_nodes',(SELECT jsonb_agg(to_jsonb(n) ORDER BY id) FROM pipeline_run_nodes n),
  'run_steps',(SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM pipeline_run_steps s),
  'work',(SELECT jsonb_agg(to_jsonb(w) ORDER BY id) FROM work_items w)
 )::text`
	var before, after, name string
	if err := db.QueryRowContext(ctx, fingerprintSQL).Scan(&before); err != nil {
		t.Fatal(err)
	}
	result, err := Run(ctx, db, "../../../migrations")
	if err != nil || len(result.Applied) != 1 {
		t.Fatalf("upgrade: %#v, %v", result, err)
	}
	if err := db.QueryRowContext(ctx, fingerprintSQL).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("display-name upgrade changed role identity, grants, assignments or workflow snapshots")
	}
	if err := db.QueryRowContext(ctx, `SELECT display_name FROM roles WHERE name='pm'`).Scan(&name); err != nil || name != "Quản lý dự án" {
		t.Fatalf("display = %q, %v", name, err)
	}
	rerun, err := Run(ctx, db, "../../../migrations")
	if err != nil || len(rerun.Applied) != 0 {
		t.Fatalf("rerun not idempotent: %#v, %v", rerun, err)
	}
}
