package core

import (
	"context"
	"strings"
	"testing"
)

func TestNormalizeMyWorkItemsFilterAndCursor(t *testing.T) {
	filter, offset, err := normalizeMyWorkItemsFilter(MyWorkItemsFilter{
		Queue: " Pending_Approval ", Sort: " Due_Asc ", Bucket: " Waiting_Decision ", Limit: 25,
		Cursor: encodeMyWorkItemsCursor(50),
	})
	if err != nil {
		t.Fatal(err)
	}
	if filter.Queue != "pending_approval" || filter.Sort != "due_asc" || filter.Bucket != "waiting_decision" || offset != 50 {
		t.Fatalf("normalized filter = %#v, offset = %d", filter, offset)
	}
	if _, _, err := normalizeMyWorkItemsFilter(MyWorkItemsFilter{Queue: "invalid"}); err == nil {
		t.Fatal("expected invalid queue error")
	}
	if _, _, err := normalizeMyWorkItemsFilter(MyWorkItemsFilter{Cursor: "not-base64"}); err == nil {
		t.Fatal("expected invalid cursor error")
	}
	if _, _, err := normalizeMyWorkItemsFilter(MyWorkItemsFilter{ProjectID: "not-a-uuid"}); err == nil {
		t.Fatal("expected invalid project id error")
	}
	if _, _, err := normalizeMyWorkItemsFilter(MyWorkItemsFilter{Limit: 101}); err == nil {
		t.Fatal("expected invalid limit error")
	}
}

func TestMyWorkDecisionSQLPreservesAuthoritativeScopeParity(t *testing.T) {
	query := myWorkItemsBaseSQL()
	if strings.Contains(query, "status IN ('waiting_review','waiting_approval') AND NOT is_watching") {
		t.Fatal("decision SQL must not deny an otherwise-authorized actor only because they watch the item")
	}
	for _, fragment := range []string{
		"NULLIF(ar.required_roles, '[]'::jsonb)",
		"scoped_role.project_id=sw.project_id",
		"actor_staff.status='active' AND pm.project_id=sw.project_id",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("decision SQL missing parity fragment %q", fragment)
		}
	}
}

func TestMyWorkItemAvailableActionsAreCapabilityAndStatusBound(t *testing.T) {
	ready := Record{"status": "ready", "can_act": true, "can_decide": false}
	actions := myWorkItemAvailableActions(ready)
	if !containsMyWorkString(actions, "start") || !containsMyWorkString(actions, "complete") || containsMyWorkString(actions, "approve") {
		t.Fatalf("ready actions = %#v", actions)
	}
	waiting := Record{"status": "waiting_review", "can_act": false, "can_decide": true}
	actions = myWorkItemAvailableActions(waiting)
	if !containsMyWorkString(actions, "approve") || !containsMyWorkString(actions, "reject") || containsMyWorkString(actions, "start") {
		t.Fatalf("waiting actions = %#v", actions)
	}
	terminal := Record{"status": "completed", "can_act": true, "can_decide": true}
	if actions := myWorkItemAvailableActions(terminal); len(actions) != 0 {
		t.Fatalf("terminal actions = %#v, want none", actions)
	}
}

func TestMyActivityAttentionStreamMapsWorkItemInvalidation(t *testing.T) {
	eventName, output := myActivityAttentionStreamEvent(ActivityStreamEvent{
		Type: "work_item_changed", ProjectID: "project-1", WorkItemID: "work-item-1",
	})
	if eventName != "work_queue_changed" || output.Type != "work_queue_changed" || output.ProjectID != "project-1" {
		t.Fatalf("mapped event = %q %#v", eventName, output)
	}
	if output.WorkItemID != "" {
		t.Fatalf("work queue invalidation leaked unverified item id: %#v", output)
	}
	eventName, output = myActivityAttentionStreamEvent(ActivityStreamEvent{Type: "activity_changed", ProjectID: "project-1"})
	if eventName != "attention_changed" || output.Type != "attention_changed" {
		t.Fatalf("activity mapping = %q %#v", eventName, output)
	}
}

func TestMyWorkItemsPageContractPaginationAndDecisionCapability(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	for index, status := range []string{"ready", "blocked"} {
		itemID := newID()
		mustExec(t, db, `
			INSERT INTO work_items
				(id, project_id, source_type, source_id, title, work_type, status, priority, role_ids, evidence_requirement_json, created_by, updated_by)
			VALUES
				($1,$2,'issue',$3,$4,'task',$5,'high','["Đội thi công"]'::jsonb,'{"required_permission":"operation.report"}'::jsonb,$6,$6)
		`, itemID, fixture.projectID, newID(), "My Work page item "+string(rune('A'+index)), status, fixture.workerID)
	}

	page, err := service.MyWorkItems(ctx, MyWorkItemsFilter{ProjectID: fixture.projectID, Limit: 1}, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	items, ok := page["items"].([]Record)
	if !ok || len(items) != 1 {
		t.Fatalf("items = %#v", page["items"])
	}
	if number(page["total"]) < 3 || !boolValue(page["has_more"]) || strings.TrimSpace(cleanID(page["next_cursor"])) == "" {
		t.Fatalf("pagination contract = %#v", page)
	}
	if _, ok := items[0]["capabilities"].(Record); !ok {
		t.Fatalf("capabilities = %#v", items[0]["capabilities"])
	}
	if _, ok := page["summary"].(Record); !ok {
		t.Fatalf("summary = %#v", page["summary"])
	}
	if _, ok := page["facets"].(Record); !ok {
		t.Fatalf("facets = %#v", page["facets"])
	}

	if _, err := service.WorkItemAction(ctx, fixture.itemID, "request-review", WorkItemActionRequest{}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	var pmStaffID, workerRoleID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM staff WHERE user_id=$1`, fixture.pmID).Scan(&pmStaffID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT role_id::text FROM user_roles WHERE user_id=$1 AND project_id IS NULL LIMIT 1`, fixture.workerID).Scan(&workerRoleID); err != nil {
		t.Fatal(err)
	}
	outsideUserID := newID()
	outsideStaffID := newID()
	suffix := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `UPDATE work_items SET assignee_staff_id=NULL WHERE id=$1`, fixture.itemID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, outsideUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, outsideStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, outsideUserID)
	})
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','Outside role','active')`, outsideUserID, "my-work-outside-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,department,position,status) VALUES ($1,$2,'Outside role',$3,'Thi công','Worker','active')`, outsideStaffID, outsideUserID, "my-work-outside-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) VALUES ($1,$2)`, outsideUserID, workerRoleID)
	mustExec(t, db, `UPDATE work_items SET assignee_staff_id=$2, watcher_staff_ids=jsonb_build_array($3::text) WHERE id=$1`, fixture.itemID, outsideStaffID, pmStaffID)

	decisionPage, err := service.MyWorkItems(ctx, MyWorkItemsFilter{
		ProjectID: fixture.projectID, Queue: "pending_approval", Limit: 20,
	}, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	decisionItems, ok := decisionPage["items"].([]Record)
	if !ok || len(decisionItems) != 1 {
		t.Fatalf("decision items = %#v", decisionPage["items"])
	}
	capabilities, _ := decisionItems[0]["capabilities"].(Record)
	if !boolValue(capabilities["can_decide"]) || !containsMyWorkString(stringSlice(decisionItems[0]["available_actions"]), "approve") {
		t.Fatalf("decision item = %#v", decisionItems[0])
	}
	outsidePage, err := service.MyWorkItems(ctx, MyWorkItemsFilter{
		ProjectID: fixture.projectID, Queue: "pending_approval", Limit: 20,
	}, outsideUserID)
	if err != nil {
		t.Fatal(err)
	}
	if number(outsidePage["total"]) != 0 {
		t.Fatalf("global-role actor without project scope pending total = %v, want 0", outsidePage["total"])
	}

	requesterPage, err := service.MyWorkItems(ctx, MyWorkItemsFilter{
		ProjectID: fixture.projectID, Queue: "pending_approval", Limit: 20,
	}, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if number(requesterPage["total"]) != 0 {
		t.Fatalf("requester pending approval total = %v, want 0", requesterPage["total"])
	}
}

func containsMyWorkString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
