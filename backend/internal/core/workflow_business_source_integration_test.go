package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRev9BusinessSourceIsFailClosedAndUsesCanonicalBOQ(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	actorID := newID()
	staffID := newID()
	projectID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, staffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','Rev9 Actor','active')`, actorID, "rev9-source-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,department,status) VALUES ($1,$2,'Rev9 Actor',$3,'operations','active')`, staffID, actorID, "rev9-source-staff-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status,pipeline_template_id) VALUES ($1,'Rev9 Source Project',$2,'Client','planning','40000000-0000-0000-0000-000000000028')`, projectID, "R9S-"+projectID[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id,staff_id,role_ids,active) VALUES ($1,$2,'["PM","QS"]'::jsonb,true)`, projectID, staffID)
	if _, err := repo.StartPipeline(ctx, projectID, "40000000-0000-0000-0000-000000000028", actorID, "Rev9 Actor"); err != nil {
		t.Fatal(err)
	}
	workItemID := clientOfficialWorkItemIDByAction(t, db, projectID, "source.boq_release_ready")
	source, err := repo.WorkflowBusinessSourceForWorkItem(ctx, workItemID)
	if err != nil {
		t.Fatal(err)
	}
	if boolValue(source["ready"]) || source["status"] != "blocked" {
		t.Fatalf("empty BOQ source=%#v, want fail-closed", source)
	}

	service := NewService(repo, "test-secret")
	item, err := service.CreateBOQ(ctx, projectID, Record{"category": "A", "work_item_name": "Canonical BOQ row", "unit": "m", "quantity": 1, "unit_price": 100}, actorID, "Rev9 Actor")
	if err != nil {
		t.Fatal(err)
	}
	itemID := cleanID(item["id"])
	// A draft row is insufficient for the current released-BOQ source gate.
	source, err = repo.WorkflowBusinessSourceForWorkItem(ctx, workItemID)
	if err != nil || boolValue(source["ready"]) {
		t.Fatalf("draft BOQ must remain blocked: %#v, %v", source, err)
	}
	if _, err := service.BOQProjectAction(ctx, projectID, "submit", WorkItemMutationContext{}, actorID, "Rev9 Actor"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BOQProjectAction(ctx, projectID, "approve", WorkItemMutationContext{}, actorID, "Rev9 Actor"); err == nil {
		t.Fatal("unlocked norm price must prevent BOQ release")
	}
	if _, err := service.LockBOQNormUnitPrice(ctx, itemID, BOQUnitPriceRequest{UnitPrice: 100, Comment: "Norm price checked"}, actorID, "Rev9 Actor"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BOQProjectAction(ctx, projectID, "approve", WorkItemMutationContext{}, actorID, "Rev9 Actor"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.LockBOQFinalUnitPrice(ctx, itemID, BOQUnitPriceRequest{UnitPrice: 100, Comment: "Approved manual price without supplier comparison for regression fixture"}, actorID, "Rev9 Actor"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM boq_releases WHERE project_id=$1 AND status='released' AND item_count=1 AND items_snapshot_json @> jsonb_build_array(jsonb_build_object('id',$2::text))`, 1, projectID, itemID)
	source, err = repo.WorkflowBusinessSourceForWorkItem(ctx, workItemID)
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(source["ready"]) || source["status"] != "succeeded" {
		t.Fatalf("BOQ-backed source=%#v, want ready", source)
	}
	if source["route"] != "/projects/"+projectID+"/boq" {
		t.Fatalf("source route=%#v", source["route"])
	}
}

func TestLegacyOfficialProcurementGateExposesExactBlockingRequest(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	fixture := setupWorkItemFlowFixture(t, db, false)
	bindWorkItemDomainExecutionContext(t, db, fixture, "approval", "procurement", "generic", "procurement.require_approved")

	approvedID := newID()
	blockingID := newID()
	secondBlockingID := newID()
	mustExec(t, db, `
		INSERT INTO procurement_requests(id,project_id,request_code,title,status,created_at)
		VALUES
			($1,$4,'REQ-APPROVED','Yêu cầu đã đặt hàng','ordered',now()-interval '3 minutes'),
			($2,$4,'REQ-BLOCKED','Yêu cầu đang chờ duyệt','waiting_approval',now()-interval '2 minutes'),
			($3,$4,'REQ-BLOCKED-02','Yêu cầu nháp','draft',now()-interval '1 minute')
	`, approvedID, blockingID, secondBlockingID, fixture.projectID)

	source, err := repo.WorkflowBusinessSourceForWorkItem(ctx, fixture.itemID)
	if err != nil {
		t.Fatal(err)
	}
	if boolValue(source["ready"]) || cleanID(source["status"]) != "blocked" {
		t.Fatalf("source=%#v, want blocked", source)
	}
	if cleanID(source["target_type"]) != "procurement_request" || cleanID(source["target_id"]) != blockingID {
		t.Fatalf("source target=%#v/%#v, want procurement_request/%s", source["target_type"], source["target_id"], blockingID)
	}
	expectedSourceRoute := "/projects/" + fixture.projectID + "/procurement?record=" + blockingID + "&record_type=procurement_request"
	if cleanID(source["route"]) != expectedSourceRoute {
		t.Fatalf("source route=%#v want %q", source["route"], expectedSourceRoute)
	}
	if message := cleanOptionalText(source["message"]); message == "" || !strings.Contains(message, "REQ-BLOCKED") {
		t.Fatalf("source message=%q, want exact blocker code", message)
	}
	blockingRequests := anySlice(recordFromAny(source["facts"])["blocking_requests"])
	if len(blockingRequests) != 2 {
		t.Fatalf("blocking requests=%#v, want both active blockers", blockingRequests)
	}
	target := workItemExecutionTarget(Record{
		"id": fixture.itemID, "project_id": fixture.projectID, "domain_action_key": "procurement.require_approved",
	}, source)
	expectedTargetRoute := expectedSourceRoute + "&task=" + fixture.itemID
	if cleanID(target["route"]) != expectedTargetRoute {
		t.Fatalf("execution target route=%#v want %q", target["route"], expectedTargetRoute)
	}

	err = repo.RunRequiredWorkflowAutomationForWorkItem(ctx, fixture.itemID, fixture.workerID, "Worker")
	var statusErr StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("required automation error=%v, want StatusError", err)
	}
	details := recordFromAny(statusErr.Details)
	if cleanID(details["target_type"]) != "procurement_request" || cleanID(details["target_id"]) != blockingID {
		t.Fatalf("error details=%#v, want exact blocking request", details)
	}

	mustExec(t, db, `UPDATE procurement_requests SET status='approved' WHERE id IN ($1,$2)`, blockingID, secondBlockingID)
	source, err = repo.WorkflowBusinessSourceForWorkItem(ctx, fixture.itemID)
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(source["ready"]) || cleanID(source["status"]) != "succeeded" {
		t.Fatalf("source after approval=%#v, want succeeded", source)
	}
	if err := repo.RunRequiredWorkflowAutomationForWorkItem(ctx, fixture.itemID, fixture.workerID, "Worker"); err != nil {
		t.Fatalf("required automation after approval: %v", err)
	}
}
