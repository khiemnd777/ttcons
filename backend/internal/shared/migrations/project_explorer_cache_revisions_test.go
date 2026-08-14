package migrations

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestProjectExplorerCacheRevisionTriggers(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()
	if _, err := Run(ctx, database, filepath.Join("..", "..", "..", "migrations")); err != nil {
		t.Fatal(err)
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO users(id, email, password_hash, full_name)
		VALUES ('97000000-0000-0000-0000-000000000001', 'cache-user@example.com', 'demo:test', 'Cache User');
		INSERT INTO staff(id, user_id, full_name, email)
		VALUES ('97000000-0000-0000-0000-000000000002', '97000000-0000-0000-0000-000000000001', 'Cache Staff', 'cache-staff@example.com');
		INSERT INTO projects(id, name, code, client_name, pm_staff_id)
		VALUES ('97000000-0000-0000-0000-000000000003', 'Cache Project', 'CACHE-97', 'Cache Client', '97000000-0000-0000-0000-000000000002');
	`); err != nil {
		t.Fatal(err)
	}

	var catalogVersion, projectVersion, actorVersion int64
	if err := database.QueryRowContext(ctx, `
		SELECT state.catalog_version, project.version,
			COALESCE((SELECT actor.version FROM project_explorer_actor_versions actor WHERE actor.user_id='97000000-0000-0000-0000-000000000001'), 1)
		FROM project_explorer_cache_state state
		JOIN project_explorer_project_versions project ON project.project_id='97000000-0000-0000-0000-000000000003'
		WHERE state.id=1
	`).Scan(&catalogVersion, &projectVersion, &actorVersion); err != nil {
		t.Fatal(err)
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO project_members(project_id, staff_id)
		VALUES ('97000000-0000-0000-0000-000000000003', '97000000-0000-0000-0000-000000000002');
		INSERT INTO issues(project_id, title)
		VALUES ('97000000-0000-0000-0000-000000000003', 'Cache issue');
		UPDATE projects SET name='Cache Project Updated' WHERE id='97000000-0000-0000-0000-000000000003';
	`); err != nil {
		t.Fatal(err)
	}

	var nextCatalog, nextProject, nextActor, pendingEvents int64
	if err := database.QueryRowContext(ctx, `
		SELECT state.catalog_version, project.version, actor.version,
			(SELECT COUNT(*) FROM project_explorer_invalidation_outbox WHERE published_at IS NULL)
		FROM project_explorer_cache_state state
		JOIN project_explorer_project_versions project ON project.project_id='97000000-0000-0000-0000-000000000003'
		JOIN project_explorer_actor_versions actor ON actor.user_id='97000000-0000-0000-0000-000000000001'
		WHERE state.id=1
	`).Scan(&nextCatalog, &nextProject, &nextActor, &pendingEvents); err != nil {
		t.Fatal(err)
	}
	if nextCatalog <= catalogVersion || nextProject <= projectVersion || nextActor <= actorVersion || pendingEvents < 3 {
		t.Fatalf("revisions before=(%d,%d,%d) after=(%d,%d,%d) pending=%d", catalogVersion, projectVersion, actorVersion, nextCatalog, nextProject, nextActor, pendingEvents)
	}
}
