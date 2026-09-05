package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestProjectUpdateIDs(t *testing.T) {
	id := newID()
	ids, err := normalizeProjectUpdateIDs(" " + strings.ToUpper(id) + "," + id)
	if err != nil || len(ids) != 1 || ids[0] != id {
		t.Fatalf("normalization = %v, %v", ids, err)
	}
	for _, raw := range []string{"", "invalid", id + ",", strings.Repeat(id+",", 100) + id} {
		if _, err := normalizeProjectUpdateIDs(raw); err == nil {
			t.Fatalf("accepted invalid batch %q", raw)
		}
	}
}

func TestProjectUpdatesScopeCountsAndReadThrough(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	first := setupProjectScopeFixture(t, db, []string{"project.view"})
	second := setupProjectScopeFixture(t, db, []string{"project.view"})
	mustExec(t, db, `INSERT INTO project_members(project_id,staff_id,role_ids,active) VALUES ($1,$2,'["Member"]',true)`, first.projectID, second.staffID)
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM project_members WHERE project_id=$1 AND staff_id=$2`, first.projectID, second.staffID)
	})
	repo := NewRepository(db)
	service := NewService(repo, "project-updates-test")
	addActivity := func(id, projectID, actorID, kind string, seconds int) {
		t.Helper()
		mustExec(t, db, `INSERT INTO work_item_activities(id,project_id,scope_type,scope_id,actor_id,activity_type,comment,created_at)
			SELECT $1,$2,'project',$2,NULLIF($3,'')::uuid,$4,'Update test',enabled_at + $5 * INTERVAL '1 second'
			FROM project_activity_tracking_start`, id, projectID, actorID, kind, seconds)
	}
	history, system1, system2, self, discussion, withdrawn := newID(), newID(), newID(), newID(), newID(), newID()
	addActivity(history, first.projectID, second.userID, "system_event", -1)
	addActivity(system1, first.projectID, second.userID, "system_event", 1)
	addActivity(system2, first.projectID, second.userID, "completed", 2)
	addActivity(self, first.projectID, first.userID, "system_event", 3)
	addActivity(discussion, first.projectID, second.userID, "comment", 4)
	mustExec(t, db, `INSERT INTO activity_read_states(activity_id,project_id,user_id) VALUES ($1,$2,$3)`, discussion, first.projectID, first.userID)
	addActivity(withdrawn, first.projectID, second.userID, "comment", 5)
	mustExec(t, db, `UPDATE work_item_activities SET withdrawn_at=now(),comment='' WHERE id=$1`, withdrawn)
	mustExec(t, db, `UPDATE work_item_activities SET from_status='in_progress',to_status='completed' WHERE id=$1`, system2)
	addActivity(newID(), first.otherProjectID, second.userID, "system_event", 6)
	// An activity attached to a cancelled work item cannot appear in a project summary.
	cancelledItem := newID()
	mustExec(t, db, `INSERT INTO work_items(id,project_id,source_type,source_id,title,work_type,status,role_ids)
		VALUES ($1,$2,'issue',gen_random_uuid(),'Cancelled','task','cancelled','[]')`, cancelledItem, first.projectID)
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM work_items WHERE id=$1`, cancelledItem) })
	mustExec(t, db, `INSERT INTO work_item_activities(project_id,work_item_id,scope_type,scope_id,activity_type,created_at)
		SELECT $1,$2,'work_item',$2,'system_event',enabled_at+INTERVAL '10 seconds' FROM project_activity_tracking_start`, first.projectID, cancelledItem)
	// Ordinary workers must not see an unassigned work item's activity.
	hiddenItem := newID()
	mustExec(t, db, `INSERT INTO work_items(id,project_id,source_type,source_id,title,work_type,status,role_ids)
		VALUES ($1,$2,'issue',gen_random_uuid(),'Hidden','task','ready','["Unassigned other role"]')`, hiddenItem, first.projectID)
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM work_items WHERE id=$1`, hiddenItem) })
	mustExec(t, db, `INSERT INTO work_item_activities(project_id,work_item_id,scope_type,scope_id,activity_type,created_at)
		SELECT $1,$2,'work_item',$2,'system_event',enabled_at+INTERVAL '11 seconds' FROM project_activity_tracking_start`, first.projectID, hiddenItem)
	addNotification := func(recipient, projectID string, archived, expired, denied bool) string {
		t.Helper()
		id := newID()
		permissions := []string{}
		if denied {
			permissions = []string{"settings.update"}
		}
		mustExec(t, db, `INSERT INTO user_notifications(id,recipient_user_id,project_id,notification_type,source_kind,source_id,source_event_id,title_snapshot,message_snapshot,action_key,idempotency_key,archived_at,expires_at,permissions_any_snapshot,created_at)
			SELECT $1::uuid,$2::uuid,$3::uuid,'activity.mentioned','project',$3::uuid,gen_random_uuid(),'Latest notification','Message','open_activity',$1::text,
			CASE WHEN $4 THEN now() END,CASE WHEN $5 THEN now()-INTERVAL '1 second' END,$6::text[],enabled_at+INTERVAL '20 seconds'
			FROM project_activity_tracking_start`, id, recipient, projectID, archived, expired, permissions)
		return id
	}
	notificationID := addNotification(first.userID, first.projectID, false, false, false)
	addNotification(first.userID, first.projectID, true, false, false)
	addNotification(first.userID, first.projectID, false, true, false)
	addNotification(first.userID, first.projectID, false, false, true)
	addNotification(second.userID, first.projectID, false, false, false)
	addNotification(first.userID, first.otherProjectID, false, false, false)
	getSummary := func(actor string) Record {
		t.Helper()
		rows, err := service.ProjectUpdatesSummary(ctx, []string{first.projectID, first.otherProjectID}, actor)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("scoped rows = %#v", rows)
		}
		return rows[0]
	}
	initial := getSummary(first.userID)
	if number(initial["notification_unread"]) != 1 || number(initial["discussion_unread"]) != 1 || number(initial["system_unread"]) != 2 {
		t.Fatalf("initial summary = %#v", initial)
	}
	latest, _ := initial["latest"].(map[string]any)
	if latest["kind"] != "notification" {
		t.Fatalf("latest = %#v", latest)
	}
	app := batch4TestApp(first.userID, func(app *fiber.App, handler *Handler) { registerProjectsRoutes(app, handler) }, NewHandler(service))
	response, err := app.Test(httptest.NewRequest("GET", "/projects/updates-summary?project_ids="+first.projectID+","+first.otherProjectID, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var envelope struct {
		Data struct {
			Items []Record `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || len(envelope.Data.Items) != 1 {
		t.Fatalf("HTTP summary = %d, %#v", response.StatusCode, envelope)
	}
	invalid, err := app.Test(httptest.NewRequest("GET", "/projects/updates-summary?project_ids=invalid", nil))
	if err != nil {
		t.Fatal(err)
	}
	invalid.Body.Close()
	if invalid.StatusCode != 400 {
		t.Fatalf("invalid batch status = %d", invalid.StatusCode)
	}
	page, err := service.ProjectActivityPage(ctx, first.projectID, ProjectActivityFilter{Kind: "system", Attention: "system_unread", Limit: 1}, first.userID)
	if err != nil {
		t.Fatal(err)
	}
	rows := page["items"].([]Record)
	if len(rows) != 1 || cleanID(rows[0]["id"]) != system2 || page["has_more"] != true || rows[0]["is_system_unread"] != true {
		t.Fatalf("first unread page = %#v", page)
	}
	page2, err := service.ProjectActivityPage(ctx, first.projectID, ProjectActivityFilter{Kind: "system", Attention: "system_unread", Limit: 1, Cursor: fmt.Sprint(page["next_cursor"])}, first.userID)
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(page2["items"].([]Record)[0]["id"]) != system1 || page2["has_more"] != false {
		t.Fatalf("second page = %#v", page2)
	}
	transitions, err := service.ProjectActivityPage(ctx, first.projectID, ProjectActivityFilter{Kind: "system", TransitionsOnly: true}, first.userID)
	if err != nil || len(transitions["items"].([]Record)) != 1 {
		t.Fatalf("transitions = %#v, %v", transitions, err)
	}
	if _, err := service.markProjectActivityReadThrough(ctx, first.projectID, ProjectDiscussionReadRequest{ActivityID: system1}, first.userID, true); err != nil {
		t.Fatal(err)
	}
	if number(getSummary(first.userID)["system_unread"]) != 1 {
		t.Fatal("read-through consumed a later event")
	}
	// Older replay cannot move the cursor back.
	if _, err := service.markProjectActivityReadThrough(ctx, first.projectID, ProjectDiscussionReadRequest{ActivityID: history}, first.userID, true); err != nil {
		t.Fatal(err)
	}
	if number(getSummary(first.userID)["system_unread"]) != 1 {
		t.Fatal("cursor regressed")
	}
	if number(getSummary(second.userID)["system_unread"]) != 1 {
		t.Fatal("another user's cursor changed or self activity counted")
	}
	for _, req := range []struct {
		project, activity string
		system            bool
	}{
		{first.projectID, discussion, true}, {first.projectID, system2, false}, {first.otherProjectID, system2, true}, {first.projectID, withdrawn, false},
	} {
		if _, err := service.markProjectActivityReadThrough(ctx, req.project, ProjectDiscussionReadRequest{ActivityID: req.activity}, first.userID, req.system); err == nil {
			t.Fatalf("accepted mismatched read: %#v", req)
		}
	}
	if _, err := service.MarkProjectDiscussionReadThrough(ctx, first.projectID, ProjectDiscussionReadRequest{ActivityID: discussion}, first.userID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.MarkUserNotificationRead(ctx, notificationID, first.userID); err != nil {
		t.Fatal(err)
	}
	final := getSummary(first.userID)
	if number(final["notification_unread"]) != 0 || number(final["discussion_unread"]) != 0 || number(final["system_unread"]) != 1 {
		t.Fatalf("independent read state = %#v", final)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM activity_read_states WHERE activity_id=$1 AND user_id=$2 AND read_at IS NULL`, 1, discussion, first.userID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE id=$1 AND from_status='in_progress' AND to_status='completed'`, 1, system2)
	// A later arrival at the same timestamp stays unread by its UUID tie-breaker.
	low, high := newID(), newID()
	if low > high {
		low, high = high, low
	}
	addActivity(low, first.projectID, second.userID, "system_event", 30)
	addActivity(high, first.projectID, second.userID, "system_event", 30)
	if _, err := service.markProjectActivityReadThrough(ctx, first.projectID, ProjectDiscussionReadRequest{ActivityID: low}, first.userID, true); err != nil {
		t.Fatal(err)
	}
	if number(getSummary(first.userID)["system_unread"]) != 1 {
		t.Fatal("read-through swallowed same-time later arrival")
	}
	if _, err := service.markProjectActivityReadThrough(ctx, first.projectID, ProjectDiscussionReadRequest{ActivityID: system2}, first.userID, true); err != nil {
		t.Fatal(err)
	}
	if number(getSummary(first.userID)["system_unread"]) != 1 {
		t.Fatal("stale tab regressed cursor")
	}
	mustExec(t, db, `UPDATE project_members SET active=false WHERE project_id=$1 AND staff_id=$2`, first.projectID, first.staffID)
	rowsAfterRevoke, err := service.ProjectUpdatesSummary(ctx, []string{first.projectID}, first.userID)
	if err != nil || len(rowsAfterRevoke) != 0 {
		t.Fatalf("revoked scope = %#v, %v", rowsAfterRevoke, err)
	}
	if _, err := service.markProjectActivityReadThrough(ctx, first.projectID, ProjectDiscussionReadRequest{ActivityID: system2}, first.userID, true); err == nil {
		t.Fatal("revoked actor advanced cursor")
	}
}

func TestProjectUpdatesBoundedBatch(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	fixture := setupProjectScopeFixture(t, db, []string{"project.view"})
	ctx := context.Background()
	repo := NewRepository(db)
	rows, err := repo.Query(ctx, `INSERT INTO projects(name,code,client_name,status,created_by)
		SELECT 'Batch update test',$1 || '-' || i,'Test client','draft',$2 FROM generate_series(1,100) i RETURNING id`, newID(), fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, cleanID(row["id"]))
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM projects WHERE id=ANY($1::uuid[])`, ids) })
	start := time.Now()
	items, err := NewService(repo, "updates-batch").ProjectUpdatesSummary(ctx, ids, fixture.userID)
	if err != nil || len(items) != 100 {
		t.Fatalf("100-row batch = %d, %v", len(items), err)
	}
	t.Logf("100-project empty-history summary: %s", time.Since(start))
	mustExec(t, db, `INSERT INTO work_item_activities(project_id,scope_type,scope_id,activity_type,comment,created_at)
		SELECT p.id,'project',p.id,CASE WHEN i % 2 = 0 THEN 'system_event' ELSE 'comment' END,'Batch history',tracking.enabled_at+i*INTERVAL '1 second'
		FROM projects p CROSS JOIN generate_series(1,100) i CROSS JOIN project_activity_tracking_start tracking
		WHERE p.id=ANY($1::uuid[])`, ids)
	start = time.Now()
	items, err = NewService(repo, "updates-batch").ProjectUpdatesSummary(ctx, ids, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if number(item["discussion_unread"]) != 50 || number(item["system_unread"]) != 50 {
			t.Fatalf("dense batch count = %#v", item)
		}
	}
	t.Logf("100-project / 10,000-event summary: %s", time.Since(start))
}
