package core

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

func TestDeveloperProjectSupportScopeAndBusinessBoundaries(t *testing.T) {
	db := openIsolatedAccountingTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	actor, scopedActor, ordinaryActor := newID(), newID(), newID()
	project, sampleProject, cancelledProject, workItem := newID(), newID(), newID(), newID()
	for _, userID := range []string{actor, scopedActor, ordinaryActor} {
		mustExec(t, db, `INSERT INTO users(id,email,password_hash,full_name,status) VALUES($1,$2,'demo:test','Support Scope Test','active')`, userID, userID+"@test.local")
	}
	mustExec(t, db, `INSERT INTO projects(id,name,code,client_name,status,is_sample) VALUES
		($1,'Support Real','SUP-REAL','Client','planning',false),
		($2,'Support Sample','SUP-SAMPLE','Client','completed',true),
		($3,'Support Cancelled','SUP-CANCELLED','Client','cancelled',false)`, project, sampleProject, cancelledProject)
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id) SELECT $1,id FROM roles WHERE name='developer'`, actor)
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id,project_id) SELECT $1,id,$2 FROM roles WHERE name='developer'`, scopedActor, project)
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id) SELECT $1,id FROM roles WHERE name='pm'`, ordinaryActor)
	mustExec(t, db, `INSERT INTO work_items(id,project_id,source_type,source_id,title,status,role_ids) VALUES($1,$2,'acceptance',$1,'Support Observation','ready','["PM"]')`, workItem, project)

	profile, err := repo.WorkItemAccessProfile(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	if !profile.IsDeveloper || profile.IsAdmin || profile.IsDirector || profile.IsManager {
		t.Fatalf("support identity incorrectly classified: %+v", profile)
	}
	for _, projectID := range []string{project, sampleProject} {
		for _, permission := range []string{"project.view", "project.update"} {
			allowed, err := repo.CanAccessProjectPermission(ctx, actor, projectID, []string{permission})
			if err != nil || !allowed {
				t.Fatalf("project=%s permission=%s allowed=%v err=%v", projectID, permission, allowed, err)
			}
		}
		assertCanViewProjectOperations(t, repo, ctx, projectID, actor, true)
	}
	assertCanViewProjectOperations(t, repo, ctx, cancelledProject, actor, false)
	assertCanViewProjectOperations(t, repo, ctx, project, scopedActor, true)
	assertCanViewProjectOperations(t, repo, ctx, sampleProject, scopedActor, false)
	assertCanViewProjectOperations(t, repo, ctx, project, ordinaryActor, false)
	for _, projectID := range []string{cancelledProject, newID()} {
		allowed, err := repo.CanAccessProjectPermission(ctx, actor, projectID, []string{"project.view"})
		if err != nil || allowed {
			t.Fatalf("inactive/missing project allowed=%v err=%v", allowed, err)
		}
	}
	rows, err := repo.ListProjects(ctx, actor)
	if err != nil || len(rows) != 2 || recordByID(rows, "id", project) == nil || recordByID(rows, "id", sampleProject) == nil {
		t.Fatalf("company project list=%v err=%v", rows, err)
	}
	visible, err := repo.ProjectOperationsVisibleProjectIDs(ctx, actor)
	if err != nil || len(visible) != 2 {
		t.Fatalf("operations scope=%v err=%v", visible, err)
	}
	scopeSQL, scopeArgs := projectOperationsScopeSQL("p", 1), projectOperationsScopeArgs(profile)
	bulk, err := repo.Query(ctx, `SELECT p.id FROM projects p WHERE `+scopeSQL, scopeArgs...)
	if err != nil || len(bulk) != len(visible) {
		t.Fatalf("bulk/direct scope mismatch: rows=%v err=%v", bulk, err)
	}
	if allowed, err := repo.CanViewWorkItem(ctx, workItem, actor); err != nil || !allowed {
		t.Fatalf("support cannot observe unrelated work item: allowed=%v err=%v", allowed, err)
	}
	item, err := repo.GetByID(ctx, "work_items", workItem)
	if err != nil {
		t.Fatal(err)
	}
	for name, check := range map[string]func() (bool, error){
		"execute":     func() (bool, error) { return repo.CanActOnWorkItem(ctx, workItem, actor) },
		"responsible": func() (bool, error) { return repo.IsWorkItemResponsible(ctx, workItem, actor) },
		"decide work": func() (bool, error) { return repo.CanDecideWorkItem(ctx, item, actor) },
		"decide approval": func() (bool, error) {
			return repo.CanDecideApprovalRequest(ctx, Record{"project_id": project, "required_roles": []string{"PM"}}, actor)
		},
	} {
		if allowed, err := check(); err != nil || allowed {
			t.Fatalf("support gained %s authority: allowed=%v err=%v", name, allowed, err)
		}
	}
	// Company supervision on a second role must not restrict Developer's sample scope.
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id) SELECT $1,id FROM roles WHERE name='deputy_manager'`, actor)
	assertCanViewProjectOperations(t, repo, ctx, sampleProject, actor, true)
	sampleWork := newID()
	mustExec(t, db, `INSERT INTO work_items(id,project_id,source_type,source_id,title,status) VALUES($1,$2,'acceptance',$1,'Sample Observation','ready')`, sampleWork, sampleProject)
	if allowed, err := repo.CanViewWorkItem(ctx, sampleWork, actor); err != nil || !allowed {
		t.Fatalf("mixed support roles lost sample work observation: allowed=%v err=%v", allowed, err)
	}
	mustExec(t, db, `DELETE FROM user_roles WHERE user_id=$1 AND role_id IN(SELECT id FROM roles WHERE name='deputy_manager')`, actor)
	permissions, err := repo.UserProjectPermissions(ctx, actor, project)
	if err != nil {
		t.Fatal(err)
	}
	visibility := projectOperationsVisibility(profile, permissions)
	if visibility["scope"] != "developer" || visibility["scoped"] != false {
		t.Fatalf("support visibility=%v", visibility)
	}
	auditID := newID()
	mustExec(t, db, `INSERT INTO audit_logs(id,project_id,actor_id,event_type,target_type,target_id,description) VALUES($1,$2,$3,'support_scope_test','project',$2,'Support scope regression')`, auditID, project, actor)
	audit, err := repo.ProjectOperationsAudit(ctx, project, profile)
	if err != nil || recordByID(audit, "id", auditID) == nil {
		t.Fatalf("support audit observation=%v err=%v", audit, err)
	}

	// Exercise the actual list/detail/update handlers with an injected test actor.
	// Authentication itself is covered by the existing session integration suite.
	handler := NewHandler(NewService(repo, "test-secret"))
	app := batch4TestApp(actor, func(app *fiber.App, h *Handler) {
		app.Get("/projects", h.ListProjects)
		app.Get("/projects/:id", h.GetProject)
		app.Put("/projects/:id", h.UpdateProject)
	}, handler)
	for _, endpoint := range []string{"/projects", "/projects/" + project, "/projects/" + sampleProject} {
		response, err := app.Test(httptest.NewRequest(http.MethodGet, endpoint, nil))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status=%d", endpoint, response.StatusCode)
		}
	}
	update := func(want int) {
		t.Helper()
		request := httptest.NewRequest(http.MethodPut, "/projects/"+project, strings.NewReader(`{"name":"Support Updated"}`))
		request.Header.Set("Content-Type", "application/json")
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("update status=%d want=%d", response.StatusCode, want)
		}
	}
	update(http.StatusOK)
	var projectName string
	if err := db.QueryRowContext(ctx, `SELECT name FROM projects WHERE id=$1`, project).Scan(&projectName); err != nil || projectName != "Support Updated" {
		t.Fatalf("persisted project name=%s err=%v", projectName, err)
	}
	mustExec(t, db, `DELETE FROM role_permissions WHERE role_id IN(SELECT id FROM roles WHERE name='developer') AND permission_id IN(SELECT id FROM permissions WHERE key='project.update')`)
	update(http.StatusForbidden)
	mustExec(t, db, `DELETE FROM role_permissions WHERE role_id IN(SELECT id FROM roles WHERE name='developer') AND permission_id IN(SELECT id FROM permissions WHERE key IN('project.view','operation.view','pipeline.view'))`)
	assertCanViewProjectOperations(t, repo, ctx, project, actor, false)
	visible, err = repo.ProjectOperationsVisibleProjectIDs(ctx, actor)
	if err != nil || len(visible) != 0 {
		t.Fatalf("scope bypass granted missing operations permission: %v err=%v", visible, err)
	}
	mustExec(t, db, `DELETE FROM role_permissions WHERE role_id IN(SELECT id FROM roles WHERE name='developer') AND permission_id IN(SELECT id FROM permissions WHERE key='audit.view')`)
	audit, err = repo.ProjectOperationsAudit(ctx, project, profile)
	if err != nil || len(audit) != 0 {
		t.Fatalf("support gained audit without permission: %v err=%v", audit, err)
	}
}

func TestDeveloperExplorerWarmCacheRechecksSupportIdentity(t *testing.T) {
	redisURL := os.Getenv("CONSFLOW_TEST_REDIS_CACHE_URL")
	if redisURL == "" {
		t.Skip("set CONSFLOW_TEST_REDIS_CACHE_URL for Developer warm-cache regression")
	}
	db := openIsolatedAccountingTestDB(t)
	ctx := context.Background()
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })
	actor, project := newID(), newID()
	mustExec(t, db, `INSERT INTO users(id,email,password_hash,full_name,status) VALUES($1,$2,'demo:test','Support Cache Test','active')`, actor, actor+"@test.local")
	mustExec(t, db, `INSERT INTO projects(id,name,code,client_name) VALUES($1,'Support Cache',$2,'Client')`, project, "SUPC-"+project[:8])
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id) SELECT $1,id FROM roles WHERE name='developer'`, actor)
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	service.ConfigureProjectExplorerCache(client, ProjectExplorerCacheConfig{Mode: "serve", OperationTimeout: time.Second, IndexTTL: time.Minute, RowTTL: time.Minute, LockTTL: time.Second, MaxValueBytes: 256 * 1024}, nil)
	request, err := normalizeProjectExplorerRequest(ProjectExplorerRequest{Page: 1, PageSize: 25})
	if err != nil {
		t.Fatal(err)
	}
	revision, err := repo.ProjectExplorerRevisionSnapshot(ctx, actor)
	if err != nil || !revision.BypassAccess {
		t.Fatalf("Developer cache revision=%+v err=%v", revision, err)
	}
	// A rollout must not reuse a still-valid restricted index from the old key format.
	oldKey := fmt.Sprintf("consflow:project-explorer:%s:index:%d:%s:%d:%t:%s", projectExplorerCacheSchemaVersion, revision.CatalogVersion, actor, revision.ActorVersion, revision.SupervisionScope, request.cacheHash())
	if err := service.projectExplorer.cache.setIndex(ctx, oldKey, projectExplorerIndex{}); err != nil {
		t.Fatal(err)
	}
	check := func(want int) {
		t.Helper()
		for range 2 {
			page, err := service.ProjectExplorer(ctx, actor, request)
			if err != nil || page.Total != want || len(page.Items) != want {
				t.Fatalf("warm cache scope total=%d rows=%d want=%d err=%v", page.Total, len(page.Items), want, err)
			}
		}
		allowed, err := repo.CanAccessProjectPermission(ctx, actor, project, []string{"project.view"})
		if err != nil || allowed != (want == 1) {
			t.Fatalf("cache/direct access mismatch allowed=%v want=%d err=%v", allowed, want, err)
		}
	}
	check(1)
	// A role bearing the canonical name must also be an internal system role.
	mustExec(t, db, `UPDATE roles SET system_role=false WHERE name='developer'`)
	check(0)
	mustExec(t, db, `UPDATE roles SET system_role=true WHERE name='developer'`)
	check(1)
	for _, status := range []string{"inactive", "active"} {
		mustExec(t, db, `UPDATE users SET status=$2 WHERE id=$1`, actor, status)
		want := 0
		if status == "active" {
			want = 1
		}
		check(want)
	}
	for _, deleted := range []bool{true, false} {
		mustExec(t, db, `UPDATE users SET status=CASE WHEN $2 THEN 'inactive' ELSE 'active' END, deleted_at=CASE WHEN $2 THEN now() ELSE NULL END WHERE id=$1`, actor, deleted)
		want := 1
		if deleted {
			want = 0
		}
		check(want)
	}
	mustExec(t, db, `DELETE FROM user_roles WHERE user_id=$1`, actor)
	check(0)
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id,project_id) SELECT $1,id,$2 FROM roles WHERE name='developer'`, actor, project)
	// A project-scoped Developer grant keeps only its normal explicit project access.
	profile, err := repo.WorkItemAccessProfile(ctx, actor)
	if err != nil || profile.IsDeveloper {
		t.Fatalf("project-scoped Developer gained company support identity: %+v err=%v", profile, err)
	}
	mustExec(t, db, `DELETE FROM user_roles WHERE user_id=$1`, actor)
	check(0)
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id) SELECT $1,id FROM roles WHERE name='developer'`, actor)
	check(1)
	mustExec(t, db, `UPDATE projects SET status='cancelled' WHERE id=$1`, project)
	check(0)
}
