package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestNativeHandoverAssessmentValidation(t *testing.T) {
	for _, body := range []Record{{"physical_result": "approved"}, {"due_date": "2026-02-30"}, {"defect": strings.Repeat("x", 1001)}, {"status": "verified"}, {"requirement": nil}, {"requirement": 42}, {}} {
		if _, err := normalizeHandoverFieldAssessment(body, false, false); err == nil {
			t.Fatalf("accepted invalid observation: %#v", body)
		}
	}
	for _, body := range []Record{{"handover_conclusion": "ready"}, {"handover_stage": strings.Repeat("x", 161)}} {
		if _, err := normalizeHandoverFieldAssessment(body, true, false); err == nil {
			t.Fatal("accepted invalid header")
		}
	}
	if _, err := normalizeHandoverFieldAssessment(Record{"title": "  "}, false, true); err == nil {
		t.Fatal("accepted blank item")
	}
}

func TestNativeHandoverAssessmentHTTPProjectScope(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_handover_http_", []string{"handover_records", "handover_checklist_items", "audit_logs"})
	owner := setupProjectScopeFixture(t, db, []string{"handover.verify"})
	other := setupProjectScopeFixture(t, db, []string{"handover.verify"})
	denied := setupProjectScopeFixture(t, db, []string{"handover.view"})
	// This actor belongs to the target project but lacks the mutation permission.
	mustExec(t, db, `INSERT INTO project_members(project_id,staff_id,role_ids,active) SELECT $1,$2,jsonb_build_array(name),true FROM roles WHERE id=$3`, owner.projectID, denied.staffID, denied.roleID)
	handoverID, itemID := newID(), newID()
	mustExec(t, db, `INSERT INTO handover_records(id,project_id,handover_code,status) VALUES($1,$2,'HTTP field assessment','draft')`, handoverID, owner.projectID)
	mustExec(t, db, `INSERT INTO handover_checklist_items(id,handover_id,project_id,item_code,category,title,required) VALUES($1,$2,$3,'http_physical','other','Door finish',false)`, itemID, handoverID, owner.projectID)
	service := NewService(NewRepository(db), "test-secret")
	request := func(actorID, route, body string, want int) {
		t.Helper()
		// Seed the authenticated actor as the application's auth middleware does.
		// Exercise registered routes, handler permission checks and persistence.
		app := batch4TestApp(actorID, func(app *fiber.App, h *Handler) { registerAcceptanceHandoverRoutes(app, h) }, NewHandler(service))
		req := httptest.NewRequest(http.MethodPut, route, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		response, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("PUT %s actor=%s status=%d, want%d", route, actorID, response.StatusCode, want)
		}
	}
	route := "/handover-checklist-items/" + itemID + "/field-assessment"
	request(owner.userID, route, `{"physical_result":"fail","defect":"Scratch"}`, http.StatusOK)
	assertCount(t, db, `SELECT count(*) FROM handover_checklist_items WHERE id=$1 AND physical_result='fail' AND defect='Scratch' AND status='pending' AND verified_at IS NULL`, 1, itemID)
	request(other.userID, route, `{"physical_result":"pass"}`, http.StatusForbidden)
	request(denied.userID, route, `{"physical_result":"pass"}`, http.StatusForbidden)
	request(owner.userID, "/handover-checklist-items/"+itemID, `{"status":"pending","notes":"Existing editor works"}`, http.StatusOK)
	request(other.userID, "/handover-checklist-items/"+itemID, `{"status":"pending","notes":"Denied"}`, http.StatusForbidden)
	assertCount(t, db, `SELECT count(*) FROM handover_checklist_items WHERE id=$1 AND physical_result='fail' AND defect='Scratch' AND notes='Existing editor works'`, 1, itemID)
	assertCount(t, db, `SELECT count(*) FROM audit_logs WHERE target_id=$1 AND event_type='handover_item_field_assessment_updated' AND actor_id=$2`, 1, itemID, owner.userID)
}

func TestNativeHandoverAssessmentIsolationAndDecisions(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_handover_", []string{"handover_records", "handover_checklist_items", "audit_logs", "users", "projects"})
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)
	handover, err := service.CreateHandover(ctx, fixture.projectID, HandoverCreateRequest{ClientRepresentative: "Client", CompanyRepresentative: "Company", HandoverStage: "Floor 1"}, fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	id := cleanID(handover["id"])
	assertCount(t, db, `SELECT count(*) FROM handover_records WHERE id=$1 AND handover_stage='Floor 1' AND handover_conclusion IS NULL`, 1, id)
	assertDenied := func(err error) {
		t.Helper()
		var status StatusError
		if !errors.As(err, &status) || status.Status != 403 {
			t.Fatalf("want403 got %v", err)
		}
	}
	_, err = service.UpdateHandoverFieldAssessment(ctx, id, Record{"handover_conclusion": "eligible"}, fixture.workerID, "Worker")
	assertDenied(err)
	// Permission on a different project does not authorize this project's records.
	otherProject := newID()
	mustExec(t, db, `INSERT INTO projects(id,name,code,client_name,status) VALUES($1,'Other',$2,'Client','planning')`, otherProject, "OTHER-"+otherProject[:8])
	permission := ensurePermission(t, db, "handover.verify", "Verify handover", "handover")
	role := ensureRole(t, db, "native_verifier", "Native verifier")
	mustExec(t, db, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, role, permission)
	mustExec(t, db, `INSERT INTO project_members(project_id,staff_id,role_ids,active) SELECT $1,id,'["Native verifier"]'::jsonb,true FROM staff WHERE user_id=$2`, otherProject, fixture.workerID)
	_, err = service.CreateHandoverPhysicalItem(ctx, id, Record{"title": "Unauthorized"}, fixture.workerID, "Worker")
	assertDenied(err)
	beforeHeader, err := repo.QueryOne(ctx, `SELECT status,readiness_snapshot_json,documents_ready,warehouse_closed FROM handover_records WHERE id=$1`, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.UpdateHandoverFieldAssessment(ctx, id, Record{"handover_conclusion": "not_eligible"}, fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM handover_records WHERE id=$1 AND handover_conclusion='not_eligible' AND handover_conclusion_by=$2 AND handover_conclusion_at IS NOT NULL AND handover_stage='Floor 1'`, 1, id, fixture.pmID)
	if _, err = service.UpdateHandoverFieldAssessment(ctx, id, Record{"handover_stage": "Floor 2"}, fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM handover_records WHERE id=$1 AND handover_conclusion='not_eligible'`, 1, id)
	if _, err = service.UpdateHandoverFieldAssessment(ctx, id, Record{"handover_conclusion": ""}, fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM handover_records WHERE id=$1 AND handover_conclusion IS NULL AND handover_conclusion_by IS NULL AND handover_conclusion_at IS NULL`, 1, id)
	for _, item := range handover["checklist_items"].([]Record) {
		itemID := cleanID(item["id"])
		if _, err = service.UpdateHandoverItemFieldAssessment(ctx, itemID, Record{"requirement": "Visual inspection", "physical_result": "fail", "defect": "Scratch", "due_date": "2026-10-20"}, fixture.pmID, "PM"); err != nil {
			t.Fatal(err)
		}
		assertCount(t, db, `SELECT count(*) FROM handover_checklist_items WHERE id=$1 AND status='pending' AND verified_at IS NULL AND verified_by IS NULL AND physical_result='fail'`, 1, itemID)
		if _, err = service.UpdateHandoverItemFieldAssessment(ctx, itemID, Record{"defect": ""}, fixture.pmID, "PM"); err != nil {
			t.Fatal(err)
		}
		assertCount(t, db, `SELECT count(*) FROM handover_checklist_items WHERE id=$1 AND defect IS NULL AND physical_result='fail' AND due_date='2026-10-20'`, 1, itemID)
	}
	physical, err := service.CreateHandoverPhysicalItem(ctx, id, Record{"title": "Door finish", "physical_result": "not_assessed"}, fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM handover_checklist_items WHERE id=$1 AND required=false AND category='other' AND source_type IS NULL AND status='pending' AND item_code LIKE 'physical_%'`, 1, physical["id"])
	// Linked and already verified items preserve their exact verification metadata.
	documentID := newID()
	mustExec(t, db, `INSERT INTO documents(id,project_id,name,file_url,document_type,status) VALUES($1,$2,'Drawing','/test.pdf','drawing','active')`, documentID, fixture.projectID)
	if _, err = repo.UpdateHandoverChecklistItem(ctx, cleanID(physical["id"]), Record{"status": "verified", "document_id": documentID}, fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.UpdateHandoverItemFieldAssessment(ctx, cleanID(physical["id"]), Record{"physical_result": "pass"}, fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM handover_checklist_items WHERE id=$1 AND status='verified' AND document_id=$2 AND verified_by=$3 AND verified_at IS NOT NULL`, 1, physical["id"], documentID, fixture.pmID)
	afterHeader, err := repo.QueryOne(ctx, `SELECT status,readiness_snapshot_json,documents_ready,warehouse_closed FROM handover_records WHERE id=$1`, id)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range beforeHeader {
		if !reflect.DeepEqual(afterHeader[key], value) {
			t.Fatalf("observation changed readiness %s", key)
		}
	}
	assertCount(t, db, `SELECT count(*) FROM audit_logs WHERE target_id=$1 AND event_type='handover_field_assessment_updated' AND metadata_json ? 'before' AND metadata_json ? 'after'`, 3, id)
	for _, state := range []string{"handed_over", "cancelled", "deleted"} {
		mustExec(t, db, `UPDATE handover_records SET status=$2 WHERE id=$1`, id, state)
		_, err = service.UpdateHandoverFieldAssessment(ctx, id, Record{"handover_stage": "Locked"}, fixture.pmID, "PM")
		assertValidationError(t, err)
		_, err = service.UpdateHandoverItemFieldAssessment(ctx, cleanID(physical["id"]), Record{"physical_result": "fail"}, fixture.pmID, "PM")
		assertValidationError(t, err)
		_, err = service.CreateHandoverPhysicalItem(ctx, id, Record{"title": "Locked"}, fixture.pmID, "PM")
		assertValidationError(t, err)
	}
	// A concurrent terminal transaction holds the parent. A child write must
	// wait, then reject the now-committed terminal state without creating a row.
	mustExec(t, db, `UPDATE handover_records SET status='draft' WHERE id=$1`, id)
	completionTx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer completionTx.Rollback()
	if _, err := completionTx.ExecContext(ctx, `UPDATE handover_records SET status='handed_over' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := service.CreateHandoverPhysicalItem(ctx, id, Record{"title": "Concurrent locked item"}, fixture.pmID, "PM")
		result <- err
	}()
	select {
	case err := <-result:
		t.Fatalf("child write bypassed parent lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := completionTx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		assertValidationError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("child write did not resume after parent commit")
	}
	assertCount(t, db, `SELECT count(*) FROM handover_checklist_items WHERE handover_id=$1 AND title='Concurrent locked item'`, 0, id)
}
