package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func procurementDiscussionFixture(t *testing.T, db *sql.DB, permissions []string) (projectScopeFixture, string, string) {
	t.Helper()
	f := setupProjectScopeFixture(t, db, permissions)
	id, supplier, boq := newID(), newID(), newID()
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM projects WHERE id=$1`, f.projectID)
		_, _ = db.Exec(`DELETE FROM partners WHERE id=$1`, supplier)
	})
	mustExec(t, db, `INSERT INTO partners(id,partner_code,name,partner_types,active) VALUES($1,$2,'Supplier A','["supplier"]',true)`, supplier, "PD-"+supplier)
	mustExec(t, db, `INSERT INTO boq_items(id,project_id,category,work_item_name,unit,quantity,unit_price,status,price_waiver_reason) VALUES($1,$2,'A','Tile','m2',2,50,'approved','Approved manual source')`, boq, f.projectID)
	mustExec(t, db, `INSERT INTO procurement_requests(id,project_id,request_code,title,status,total_estimated_amount,supplier_partner_id,supplier_name) VALUES($1,$2,$3,'Original title','draft',100,$4,'Supplier A')`, id, f.projectID, "PD-"+id, supplier)
	mustExec(t, db, `INSERT INTO procurement_request_items(request_id,boq_item_id,quantity_planned,quantity_requested,unit,estimated_unit_price,supplier_partner_id,supplier_name) VALUES($1,$2,2,2,'m2',50,$3,'Supplier A')`, id, boq, supplier)
	return f, id, supplier
}

func TestProcurementDiscussionHistoryVersionsAndQuietDrafts(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	s := NewService(NewRepository(db), "test-secret")
	f, id, supplier := procurementDiscussionFixture(t, db, []string{"operation.view", "operation.report", "procurement.view", "procurement.create", "procurement.approve", "procurement.order"})
	row, err := s.UpdateProcurement(ctx, id, Record{"title": "Reviewed title", "expected_version": 1, "status": "ordered", "total_estimated_amount": 999, "budget_status": "budget_exceeded"}, f.userID, "actor")
	if err != nil {
		t.Fatal(err)
	}
	if number(row["edit_version"]) != 2 || number(row["total_estimated_amount"]) != 100 || row["status"] != "draft" || row["budget_status"] != "within_budget" {
		t.Fatalf("header changed source fields: %#v", row)
	}
	if _, err := s.UpdateProcurement(ctx, id, Record{"title": "stale", "expected_version": 1}, f.userID, "actor"); err == nil {
		t.Fatal("stale header overwrote newer data")
	}
	if _, err := s.UpdateProcurement(ctx, id, Record{"supplier_partner_id": nil, "expected_version": 2}, f.userID, "actor"); err == nil {
		t.Fatal("header supplier contradicted snapshotted lines")
	}
	assertCount(t, db, `SELECT count(*) FROM activity_work_updates WHERE source_id=$1`, 0, id)
	version := 2
	if _, err := s.ProcurementAction(ctx, id, "submit", ActionRequest{ExpectedVersion: &version}, f.userID, "actor"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ProcurementAction(ctx, id, "submit", ActionRequest{ExpectedVersion: &version}, f.userID, "actor"); err == nil {
		t.Fatal("duplicate submit succeeded")
	}
	if _, err := s.UpdateProcurement(ctx, id, Record{"title": "Changed after submission", "expected_version": 3}, f.userID, "actor"); err != nil {
		t.Fatal(err)
	}
	version = 3
	if _, err := s.ProcurementAction(ctx, id, "approve", ActionRequest{ExpectedVersion: &version}, f.userID, "actor"); err == nil {
		t.Fatal("approved a header edited after review")
	}
	version = 4
	if _, err := s.ProcurementAction(ctx, id, "approve", ActionRequest{ExpectedVersion: &version}, f.userID, "actor"); err != nil {
		t.Fatal(err)
	}
	version = 5
	if _, err := s.ProcurementAction(ctx, id, "order", ActionRequest{ExpectedVersion: &version}, f.userID, "actor"); err == nil {
		t.Fatal("ordered without PO and delivery date")
	}
	if _, err := s.UpdateProcurement(ctx, id, Record{"supplier_partner_id": supplier, "order_reference": "PO-42", "expected_delivery_date": "2026-10-01", "expected_version": 5}, f.userID, "actor"); err != nil {
		t.Fatal(err)
	}
	version = 6
	if _, err := s.ProcurementAction(ctx, id, "order", ActionRequest{ExpectedVersion: &version}, f.userID, "actor"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ProcurementAction(ctx, id, "order", ActionRequest{ExpectedVersion: &version}, f.userID, "actor"); err == nil {
		t.Fatal("duplicate order succeeded")
	}
	assertCount(t, db, `SELECT count(*) FROM activity_work_updates WHERE source_id=$1`, 3, id)
	assertCount(t, db, `SELECT count(*) FROM activity_work_updates WHERE source_id=$1 AND source_snapshot_json->>'title'='Reviewed title' AND source_snapshot_json->>'status'='waiting_approval' AND source_snapshot_json->'items'->0->>'estimated_unit_price'='50.00'`, 1, id)
	assertCount(t, db, `SELECT count(*) FROM procurement_requests WHERE id=$1 AND status='ordered' AND edit_version=7`, 1, id)
	for _, table := range []string{"stock_movements", "payables", "receivables"} {
		assertCount(t, db, fmt.Sprintf("SELECT count(*) FROM %s WHERE project_id=$1", table), 0, f.projectID)
	}
	activity, err := s.repo.QueryOne(ctx, `SELECT activity_id FROM activity_work_updates WHERE source_id=$1 ORDER BY source_event_key LIMIT 1`, id)
	if err != nil {
		t.Fatal(err)
	}
	activityID := cleanID(activity["activity_id"])
	if _, err := s.DiscussionWorkUpdate(ctx, activityID, f.userID); err != nil {
		t.Fatal(err)
	}
	reply, err := s.AddProjectActivity(ctx, f.projectID, WorkItemActivityRequest{ParentActivityID: activityID, Comment: "Please confirm delivery"}, f.userID, "actor")
	if err != nil {
		t.Fatal(err)
	}
	if reply["scope_type"] != "procurement_request" || cleanID(reply["scope_id"]) != id {
		t.Fatalf("reply lost source: %#v", reply)
	}
	// Discussion access alone does not expose historical supplier prices.
	mustExec(t, db, `DELETE FROM role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM permissions WHERE key='procurement.view')`, f.roleID)
	if _, err := s.DiscussionWorkUpdate(ctx, activityID, f.userID); err == nil {
		t.Fatal("read prices after procurement view was revoked")
	}
}

func TestProcurementWorkspaceRoutesAndSourceCapabilities(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	s := NewService(NewRepository(db), "test-secret")
	f, id, _ := procurementDiscussionFixture(t, db, []string{"procurement.view", "procurement.order", "operation.view"})
	mustExec(t, db, `UPDATE procurement_requests SET status='approved',order_reference='PO-1',expected_delivery_date='2026-10-01' WHERE id=$1`, id)
	app := batch4TestApp(f.userID, func(app *fiber.App, h *Handler) { registerProcurementRoutes(app, h) }, NewHandler(s))
	request := func(method, path, body string, want int) Record {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var envelope Record
		if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != want {
			t.Fatalf("%s %s returned %d: %#v", method, path, res.StatusCode, envelope)
		}
		return recordFromAny(envelope["data"])
	}
	row := request(http.MethodGet, "/procurement-requests/"+id, "", 200)
	caps := recordFromAny(row["capabilities"])
	if !boolValue(caps["order"]) || boolValue(caps["edit"]) || boolValue(caps["approve"]) {
		t.Fatalf("wrong source capabilities: %#v", caps)
	}
	request(http.MethodPut, "/procurement-requests/"+id, `{"title":"unauthorized"}`, 403)
	request(http.MethodPost, "/procurement-requests/"+id+"/approve", `{}`, 403)
	request(http.MethodPost, "/procurement-requests/"+id+"/order", `{"expected_version":1}`, 200)
	request(http.MethodPost, "/procurement-requests/"+id+"/order", `{"expected_version":1}`, 409)
	otherID := newID()
	mustExec(t, db, `INSERT INTO procurement_requests(id,project_id,request_code,title) VALUES($1,$2,$3,'other project')`, otherID, f.otherProjectID, otherID)
	request(http.MethodGet, "/procurement-requests/"+otherID, "", 403)
	request(http.MethodPost, "/procurement-requests/"+otherID+"/order", `{"expected_version":1}`, 403)
	mustExec(t, db, `UPDATE procurement_requests SET status='cancelled' WHERE id=$1`, id)
	request(http.MethodGet, "/procurement-requests/"+id, "", 404)
}

func TestProcurementWorkspaceListsOnlyActiveProjectScopedApprovers(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	s := NewService(NewRepository(db), "test-secret")
	f, id, _ := procurementDiscussionFixture(t, db, []string{"procurement.view", "procurement.approve"})
	mustExec(t, db, `UPDATE procurement_requests SET status='waiting_approval' WHERE id=$1`, id)

	outsiderID, outsiderStaffID := newID(), newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, outsiderStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, outsiderID)
	})
	mustExec(t, db, `INSERT INTO users(id,email,password_hash,full_name,status) VALUES($1,$2,'demo:pw','Outside Approver','active')`, outsiderID, "outside-approver-"+outsiderID+"@test.local")
	mustExec(t, db, `INSERT INTO staff(id,user_id,full_name,email,status) VALUES($1,$2,'Outside Approver',$3,'active')`, outsiderStaffID, outsiderID, "outside-approver-staff-"+outsiderID+"@test.local")
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id) VALUES($1,$2)`, outsiderID, f.roleID)

	workspace, err := s.ProcurementWorkspace(ctx, id, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	approvalContext := recordFromAny(workspace["approval_context"])
	if approvalContext["permission"] != "procurement.approve" || int(number(approvalContext["candidate_count"])) != 1 {
		t.Fatalf("wrong approval context: %#v", approvalContext)
	}
	candidates, ok := approvalContext["candidates"].([]Record)
	if !ok || len(candidates) != 1 {
		t.Fatalf("wrong approval candidates: %#v", approvalContext["candidates"])
	}
	if cleanID(candidates[0]["user_id"]) != f.userID || fmt.Sprint(candidates[0]["full_name"]) != "Scoped User" {
		t.Fatalf("wrong approval candidate: %#v", candidates[0])
	}
	if roles := stringSlice(candidates[0]["role_names"]); len(roles) != 1 || roles[0] != "Batch4 Scope" {
		t.Fatalf("wrong candidate roles: %#v", candidates[0]["role_names"])
	}

	mustExec(t, db, `UPDATE users SET status='inactive' WHERE id=$1`, f.userID)
	workspace, err = s.ProcurementWorkspace(ctx, id, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	approvalContext = recordFromAny(workspace["approval_context"])
	if int(number(approvalContext["candidate_count"])) != 0 {
		t.Fatalf("inactive approver remained visible: %#v", approvalContext)
	}
}

func TestProcurementConcurrentSavesAndContextRollback(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	s := NewService(NewRepository(db), "test-secret")
	f, id, _ := procurementDiscussionFixture(t, db, []string{"procurement.create"})
	results := make(chan error, 2)
	for _, title := range []string{"first", "second"} {
		go func(title string) {
			_, err := s.UpdateProcurement(ctx, id, Record{"title": title, "expected_version": 1}, f.userID, "actor")
			results <- err
		}(title)
	}
	success := 0
	for range 2 {
		if <-results == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("%d competing saves succeeded", success)
	}
	assertCount(t, db, `SELECT count(*) FROM procurement_requests WHERE id=$1 AND edit_version=2 AND status='draft'`, 1, id)
	foreign := setupWorkItemFlowFixture(t, db, false)
	version := 2
	if _, err := s.ProcurementAction(ctx, id, "submit", ActionRequest{ExpectedVersion: &version, WorkItemContext: WorkItemMutationContext{WorkItemID: foreign.itemID, Intent: workItemMutationIntentStartOnSuccess}}, f.userID, "actor"); err == nil {
		t.Fatal("foreign work context was accepted")
	}
	assertCount(t, db, `SELECT count(*) FROM procurement_requests WHERE id=$1 AND status='draft' AND edit_version=2`, 1, id)
	assertCount(t, db, `SELECT count(*) FROM activity_work_updates WHERE source_id=$1`, 0, id)
}

func TestProcurementHeaderCannotBypassStructuredApproval(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	s := NewService(NewRepository(db), "test-secret")
	f, id, _ := procurementDiscussionFixture(t, db, []string{"procurement.create", "procurement.approve"})
	approvalID := newID()
	t.Cleanup(func() {
		_, _ = db.Exec(`UPDATE procurement_requests SET approval_request_id=NULL WHERE id=$1`, id)
		_, _ = db.Exec(`DELETE FROM approval_requests WHERE id=$1`, approvalID)
	})
	mustExec(t, db, `INSERT INTO approval_requests(id,project_id,target_type,target_id,status) VALUES($1,$2,'procurement_request',$3,'pending')`, approvalID, f.projectID, id)
	mustExec(t, db, `UPDATE procurement_requests SET status='waiting_approval',approval_request_id=$2 WHERE id=$1`, id, approvalID)
	if _, err := s.UpdateProcurement(ctx, id, Record{"title": "Updated", "approval_request_id": nil, "expected_version": 1}, f.userID, "actor"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM procurement_requests WHERE id=$1 AND approval_request_id=$2`, 1, id, approvalID)
	workspace, err := s.ProcurementWorkspace(ctx, id, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if boolValue(recordFromAny(workspace["capabilities"])["approve"]) || cleanOptionalText(workspace["approval_message"]) == "" {
		t.Fatal("pending structured approval advertised an available purchase approval")
	}
	version := 2
	if _, err := s.ProcurementAction(ctx, id, "approve", ActionRequest{ExpectedVersion: &version}, f.userID, "actor"); err == nil {
		t.Fatal("pending structured approval was bypassed")
	}
	assertCount(t, db, `SELECT count(*) FROM activity_work_updates WHERE source_id=$1`, 0, id)
	mustExec(t, db, `UPDATE approval_requests SET status='approved' WHERE id=$1`, approvalID)
	if _, err := s.ProcurementAction(ctx, id, "approve", ActionRequest{ExpectedVersion: &version}, f.userID, "actor"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM activity_work_updates WHERE source_id=$1`, 1, id)
}

func TestProcurementExplicitContextStartsMatchingWorkAndKeepsCompletionSeparate(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	s := NewService(NewRepository(db), "test-secret")
	f := setupWorkItemFlowFixture(t, db, false)
	bindWorkItemDomainExecutionContext(t, db, f, "approval", "procurement", "generic", "procurement.require_approved")
	id := newID()
	mustExec(t, db, `INSERT INTO procurement_requests(id,project_id,request_code,title,status) VALUES($1,$2,$3,'Review purchase','waiting_approval')`, id, f.projectID, id)
	version := 1
	result, err := s.ProcurementAction(ctx, id, "approve", ActionRequest{ExpectedVersion: &version, WorkItemContext: WorkItemMutationContext{WorkItemID: f.itemID, Intent: workItemMutationIntentStartOnSuccess}}, f.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(recordFromAny(result["work_item_transition"])["auto_started"]) {
		t.Fatal("matching explicit context did not start work")
	}
	assertCount(t, db, `SELECT count(*) FROM work_items WHERE id=$1 AND status='in_progress'`, 1, f.itemID)
	assertCount(t, db, `SELECT count(*) FROM activity_work_updates u JOIN work_item_activities a ON a.id=u.activity_id WHERE u.source_id=$1 AND a.work_item_id=$2 AND a.scope_type='work_item'`, 1, id, f.itemID)
}
