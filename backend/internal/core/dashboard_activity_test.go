package core

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestDashboardActivityWindowAndCursor(t *testing.T) {
	now := time.Date(2026, 9, 5, 17, 5, 0, 123456000, time.UTC)
	q, err := normalizeDashboardActivities(DashboardActivitiesRequest{}, "actor", now)
	if err != nil || q.Range != "7d" || q.Limit != 20 || q.From.UTC().Format(time.RFC3339) != "2026-08-30T17:00:00Z" {
		t.Fatalf("default Vietnam window = %#v, %v", q, err)
	}
	today, err := normalizeDashboardActivities(DashboardActivitiesRequest{Range: "today"}, "actor", now)
	if err != nil || today.From.UTC().Format(time.RFC3339) != "2026-09-05T17:00:00Z" {
		t.Fatalf("today = %#v, %v", today, err)
	}
	last := DashboardActivityItem{ID: newID(), CreatedAt: now.Add(-time.Minute).Format(time.RFC3339Nano)}
	cursor := encodeDashboardActivityCursor(q, last)
	next, err := normalizeDashboardActivities(DashboardActivitiesRequest{Cursor: cursor}, "actor", now.Add(24*time.Hour))
	if err != nil || !next.AsOf.Equal(q.AsOf) || !next.From.Equal(q.From) || next.BeforeID != last.ID {
		t.Fatalf("cursor lost snapshot: %#v, %v", next, err)
	}
	for _, req := range []DashboardActivitiesRequest{
		{Range: "invalid"}, {Limit: -1}, {Limit: 51}, {Search: strings.Repeat("a", 201)}, {ActivityType: "comment"},
		{Cursor: "broken"}, {Cursor: cursor, Range: "all"}, {Cursor: cursor, Search: "changed"}, {Cursor: cursor, Limit: 2}, {Cursor: cursor, ActivityType: "started"},
	} {
		if _, err := normalizeDashboardActivities(req, "actor", now); err == nil {
			t.Fatalf("accepted invalid filter %#v", req)
		}
	}
	if _, err := normalizeDashboardActivities(DashboardActivitiesRequest{Cursor: cursor}, "other-actor", now); err == nil {
		t.Fatal("accepted another actor's cursor")
	}
	all, err := normalizeDashboardActivities(DashboardActivitiesRequest{Range: "all", Search: "  khảo sát  "}, "actor", now)
	if err != nil || !all.From.IsZero() || all.Search != "khảo sát" {
		t.Fatalf("all = %#v, %v", all, err)
	}
}

func TestDashboardActivityScopePaginationAndReadOnly(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	viewer := setupProjectScopeFixture(t, db, []string{"project.view", "report.view"})
	other := setupProjectScopeFixture(t, db, []string{"project.view"})
	mustExec(t, db, `INSERT INTO project_members(project_id,staff_id,role_ids,active) VALUES ($1,$2,'["Member"]',true)`, other.projectID, viewer.staffID)
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM project_members WHERE project_id=$1 AND staff_id=$2`, other.projectID, viewer.staffID)
	})
	service := NewService(NewRepository(db), "dashboard-test")
	now := time.Now().UTC().Add(-time.Second).Truncate(time.Microsecond)
	add := func(projectID, workID, kind, visibility string, at time.Time) string {
		t.Helper()
		id := newID()
		scope, target := "project", projectID
		if workID != "" {
			scope, target = "work_item", workID
		}
		mustExec(t, db, `INSERT INTO work_item_activities(id,project_id,work_item_id,scope_type,scope_id,actor_id,activity_type,comment,visibility,created_at)
			VALUES ($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7,'Khảo sát dashboard',$8,$9)`, id, projectID, workID, scope, target, other.userID, kind, visibility, at)
		return id
	}
	first := add(viewer.projectID, "", "started", "internal", now)
	second := add(other.projectID, "", "completed", "internal", now)
	older := add(viewer.projectID, "", "approved", "internal", now.Add(-time.Hour))
	mustExec(t, db, `UPDATE work_item_activities SET from_status='ready',to_status='in_progress',metadata_json='{"form_version_id":"safe-version","private":"never expose"}' WHERE id=$1`, first)
	add(viewer.otherProjectID, "", "started", "internal", now)
	add(viewer.projectID, "", "comment", "internal", now)
	withdrawn := add(viewer.projectID, "", "started", "internal", now)
	mustExec(t, db, `UPDATE work_item_activities SET withdrawn_at=now() WHERE id=$1`, withdrawn)
	add(viewer.projectID, "", "started", "internal", now.Add(-8*24*time.Hour))
	add(viewer.projectID, "", "started", "internal", now.Add(24*time.Hour))
	mustExec(t, db, `UPDATE projects SET status='cancelled' WHERE id=$1`, other.otherProjectID)
	add(other.otherProjectID, "", "started", "internal", now)
	for _, status := range []string{"ready", "cancelled"} {
		id := newID()
		mustExec(t, db, `INSERT INTO work_items(id,project_id,source_type,source_id,title,work_type,status,role_ids) VALUES ($1,$2,'issue',gen_random_uuid(),'Hidden','task',$3,'["Different role"]')`, id, viewer.projectID, status)
		t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM work_items WHERE id=$1`, id) })
		add(viewer.projectID, id, "started", "internal", now)
		// Alternate target scopes must not reveal the same hidden Work Item.
		alternate := add(viewer.projectID, id, "system_event", "internal", now)
		mustExec(t, db, `UPDATE work_item_activities SET scope_type='boq_item' WHERE id=$1`, alternate)
	}
	page, err := service.DashboardActivities(ctx, DashboardActivitiesRequest{Limit: 1}, viewer.userID)
	if err != nil || !page.CanViewActivity || !page.HasMore || len(page.Items) != 1 {
		t.Fatalf("page = %#v, %v", page, err)
	}
	wantFirst, wantSecond := first, second
	if second > first {
		wantFirst, wantSecond = second, first
	}
	if page.Items[0].ID != wantFirst {
		t.Fatalf("same-time order = %#v", page.Items)
	}
	// A new event cannot shift or enter the previously started pagination snapshot.
	asOf, _ := time.Parse(time.RFC3339Nano, page.AsOf)
	add(viewer.projectID, "", "started", "internal", asOf.Add(time.Millisecond))
	next, err := service.DashboardActivities(ctx, DashboardActivitiesRequest{Limit: 1, Cursor: page.NextCursor}, viewer.userID)
	if err != nil || len(next.Items) != 1 || next.Items[0].ID != wantSecond || !next.HasMore {
		t.Fatalf("next = %#v, %v", next, err)
	}
	last, err := service.DashboardActivities(ctx, DashboardActivitiesRequest{Limit: 1, Cursor: next.NextCursor}, viewer.userID)
	if err != nil || len(last.Items) != 1 || last.Items[0].ID != older || last.HasMore {
		t.Fatalf("last = %#v, %v", last, err)
	}
	filtered, err := service.DashboardActivities(ctx, DashboardActivitiesRequest{Search: "khảo sát", ActivityType: "approved"}, viewer.userID)
	if err != nil || len(filtered.Items) != 1 || filtered.Items[0].ID != older {
		t.Fatalf("filter = %#v, %v", filtered, err)
	}
	missing, err := service.DashboardActivities(ctx, DashboardActivitiesRequest{Search: "no-matching-activity"}, viewer.userID)
	if err != nil || !missing.CanViewActivity || len(missing.Items) != 0 {
		t.Fatalf("empty is not denial: %#v, %v", missing, err)
	}
	// Every Dashboard row is also visible in the canonical single-project feed.
	all, err := service.DashboardActivities(ctx, DashboardActivitiesRequest{}, viewer.userID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range all.Items {
		rows, err := service.ProjectActivity(ctx, item.ProjectID, ProjectActivityFilter{Kind: "system"}, viewer.userID)
		if err != nil || recordByID(rows, "id", item.ID) == nil {
			t.Fatalf("scope mismatch for %s: %v", item.ID, err)
		}
	}
	serialized, _ := json.Marshal(all)
	if strings.Contains(string(serialized), "never expose") || strings.Contains(string(serialized), "metadata_json") {
		t.Fatal("raw metadata exposed")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM project_system_activity_read_cursors WHERE user_id=$1`, 0, viewer.userID)
	assertCount(t, db, `SELECT COUNT(*) FROM activity_read_states WHERE user_id=$1`, 0, viewer.userID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE id=$1 AND from_status='ready' AND to_status='in_progress'`, 1, first)
	app := batch4TestApp(viewer.userID, func(app *fiber.App, h *Handler) { registerReportingRoutes(app, h) }, NewHandler(service))
	for path, status := range map[string]int{"/reports/dashboard/activities?range=7d": 200, "/reports/dashboard/activities?limit=bad": 400, "/reports/dashboard/activities?activity_type=comment": 400} {
		response, err := app.Test(httptest.NewRequest("GET", path, nil))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != status {
			t.Fatalf("%s = %d", path, response.StatusCode)
		}
	}
	deniedApp := batch4TestApp(other.userID, func(app *fiber.App, h *Handler) { registerReportingRoutes(app, h) }, NewHandler(service))
	response, err := deniedApp.Test(httptest.NewRequest("GET", "/reports/dashboard/activities", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatalf("report permission = %d", response.StatusCode)
	}
	// Revocation is applied again on subsequent pages, including cached cursor requests.
	mustExec(t, db, `DELETE FROM role_permissions WHERE role_id=$1 AND permission_id IN (SELECT id FROM permissions WHERE key='project.view')`, viewer.roleID)
	denied, err := service.DashboardActivities(ctx, DashboardActivitiesRequest{Limit: 1, Cursor: page.NextCursor}, viewer.userID)
	if err != nil || denied.CanViewActivity || len(denied.Items) != 0 {
		t.Fatalf("revoked scope = %#v, %v", denied, err)
	}
}

func TestDashboardActivityClientAndCompanyObserver(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	client := setupProjectScopeFixture(t, db, []string{"project.view", "report.view"})
	mustExec(t, db, `UPDATE roles SET display_name='Chủ đầu tư' WHERE id=$1`, client.roleID)
	for _, visibility := range []string{"internal", "client"} {
		mustExec(t, db, `INSERT INTO work_item_activities(project_id,scope_type,scope_id,activity_type,comment,visibility) VALUES ($1,'project',$1,'system_event',$2,$2)`, client.projectID, visibility)
	}
	service := NewService(NewRepository(db), "dashboard-test")
	page, err := service.DashboardActivities(ctx, DashboardActivitiesRequest{}, client.userID)
	if err != nil || len(page.Items) != 1 || page.Items[0].Comment != "client" {
		t.Fatalf("client visibility = %#v, %v", page, err)
	}
	observer := setupProjectScopeFixture(t, db, []string{"project.reconciliation.view", "report.view"})
	mustExec(t, db, `INSERT INTO work_item_activities(project_id,scope_type,scope_id,activity_type,comment) VALUES ($1,'project',$1,'system_event','Company observer')`, client.otherProjectID)
	page, err = service.DashboardActivities(ctx, DashboardActivitiesRequest{Search: "Company observer"}, observer.userID)
	if err != nil || len(page.Items) != 1 || page.Items[0].ProjectID != client.otherProjectID {
		t.Fatalf("company visibility = %#v, %v", page, err)
	}
}

func TestDashboardActivityBoundedQuery(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	fixture := setupProjectScopeFixture(t, db, []string{"project.view", "report.view"})
	ctx := context.Background()
	repo := NewRepository(db)
	projects, err := repo.Query(ctx, `INSERT INTO projects(name,code,client_name,status,created_by)
		SELECT 'Dashboard load test',$1 || '-' || i,'Test','draft',$2 FROM generate_series(1,100) i RETURNING id`, newID(), fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(projects))
	for _, row := range projects {
		ids = append(ids, cleanID(row["id"]))
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM projects WHERE id=ANY($1::uuid[])`, ids) })
	mustExec(t, db, `INSERT INTO work_item_activities(project_id,scope_type,scope_id,activity_type,comment,created_at)
		SELECT p.id,'project',p.id,'system_event','Dashboard history',now()-i*INTERVAL '1 minute'
		FROM projects p CROSS JOIN generate_series(1,100) i WHERE p.id=ANY($1::uuid[])`, ids)
	profile, err := repo.WorkItemAccessProfile(ctx, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	q, err := normalizeDashboardActivities(DashboardActivitiesRequest{}, fixture.userID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	page, err := NewService(repo, "dashboard-perf").DashboardActivities(ctx, DashboardActivitiesRequest{}, fixture.userID)
	duration := time.Since(started)
	if err != nil || len(page.Items) != 20 || !page.HasMore {
		t.Fatalf("bounded page = %#v, %v", page, err)
	}
	t.Logf("100 projects / 10,000 events: first 20 rows in %s", duration)
	query, args := dashboardActivitiesSQL(profile, q)
	tx, err := repo.beginReadTransactionWithJITDisabled(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var raw []byte
	if err := tx.QueryRowContext(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+query, args...).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var plans []struct {
		ExecutionTime float64 `json:"Execution Time"`
		Plan          struct {
			ActualRows int `json:"Actual Rows"`
		} `json:"Plan"`
	}
	if err := json.Unmarshal(raw, &plans); err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Plan.ActualRows != 21 {
		t.Fatalf("query limit violated: %s", raw)
	}
	t.Logf("EXPLAIN ANALYZE: %.2f ms, %d rows including lookahead", plans[0].ExecutionTime, plans[0].Plan.ActualRows)
}
