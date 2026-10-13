package core

import (
	"context"
	"errors"
	"testing"
)

func TestNativeReceiptCanonicalFormAndDeliveryRevision(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_receipt_canonical_", []string{"incoming_acceptance_records", "incoming_acceptance_items", "workflow_form_instances", "workflow_form_events", "operation_daily_reports", "work_report_revisions"})
	ctx := context.Background()
	f := setupWorkItemFlowFixture(t, db, false)
	s := NewService(NewRepository(db), "test-secret")
	receivingID, photoID, formID := newID(), newID(), newID()
	mustExec(t, db, `INSERT INTO work_items(id,project_id,source_type,source_id,title,work_type,status,role_ids) VALUES($1,$2,'issue',$1,'Receiving task','task','ready','["Đội thi công"]')`, receivingID, f.projectID)
	receivingFixture := f
	receivingFixture.itemID = receivingID
	bindWorkItemDomainExecutionContext(t, db, receivingFixture, "acceptance", "incoming_acceptance", "incoming_acceptance", "acceptance.incoming_material")
	mustExec(t, db, `INSERT INTO evidence_records(id,project_id,work_item_id,evidence_type,name,status,uploaded_by,file_url) VALUES($1,$2,$3,'photo','Site photo','submitted',$4,'https://example.com/test.png')`, photoID, f.projectID, f.itemID, f.workerID)
	mustExec(t, db, `INSERT INTO workflow_form_instances(id,project_id,work_item_id,form_template_id,form_version_id,form_type,title,created_by) SELECT $1,$2,$3,t.id,v.id,t.form_type,'Daily receipt revision',$4 FROM workflow_form_templates t JOIN LATERAL(SELECT id FROM workflow_form_versions WHERE template_id=t.id AND status='published' ORDER BY version_no DESC LIMIT 1)v ON true WHERE t.code='builtin_construction_daily_report'`, formID, f.projectID, f.itemID, f.workerID)
	createReceipt := func(name string, quantity float64) string {
		t.Helper()
		receipt, err := s.CreateIncomingAcceptance(ctx, f.projectID, IncomingAcceptanceRequest{WorkItemID: receivingID, ReceivedAt: "2026-10-12", SupplierName: "Supplier is not origin", Items: []IncomingAcceptanceItemRequest{{MaterialName: name, Unit: "m2", DeliveredQuantity: &quantity, AcceptedQuantity: &quantity, Comment: "Recorded receipt"}}}, f.workerID, "Worker")
		if err != nil {
			t.Fatal(err)
		}
		id := cleanID(receipt["id"])
		if _, err := s.SubmitIncomingAcceptance(ctx, id, f.workerID, "Worker"); err != nil {
			t.Fatal(err)
		}
		return id
	}
	first := createReceipt("First tiles", 12)
	answers := Record{"report_date": "2026-10-12", "reported_by": "Worker", "construction_scope": "civil", "weather": "normal", "completed_items": []Record{{"title": "Wall", "result": "done", "evidence_id": photoID}}, "crew_items": []Record{{"crew_name": "Crew", "worker_count": 3, "work_hours": 8}}, "material_arrival_items": []Record{}}
	answers["next_plan_items"] = []Record{
		{"row_id": newID(), "title": "Generated follow-up", "owner_role": "Đội thi công", "due_date": "2026-10-13", "priority": "normal"},
		{"row_id": newID(), "title": "Manual follow-up", "owner_role": "Đội thi công", "due_date": "2026-10-13", "priority": "normal", "state": "Observed on site"},
	}
	form, err := s.SubmitWorkflowForm(ctx, formID, WorkflowFormSaveRequest{Answers: answers}, f.workerID, "Worker")
	if err != nil {
		nativeDailyFormFatal(t, "submit", err)
	}
	var originalEvent, dailyID string
	if err := db.QueryRowContext(ctx, `SELECT metadata_json::text FROM workflow_form_events WHERE instance_id=$1 AND event_type='submitted'`, formID).Scan(&originalEvent); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM workflow_form_events WHERE instance_id=$1 AND event_type='submitted' AND metadata_json->'native_daily_sources'->'structured_json'->'material_arrival_items'->0->>'source_id'=$2 AND metadata_json->'native_daily_sources'->'structured_json'->'material_arrival_items'->0->>'source_work_item_id'=$3`, 1, formID, first, receivingID)
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM operation_daily_reports WHERE workflow_form_instance_id=$1`, formID).Scan(&dailyID); err != nil {
		t.Fatal(err)
	}
	createReceipt("Later tiles", 5)
	// Persisted submitted forms restore arrivals while mutable BM06 task
	// observations stay live until a new form revision captures them.
	mustExec(t, db, `UPDATE work_items SET status='completed' WHERE source_type='workflow_form_action_item' AND source_id=$1`, formID)
	currentSource := Record{"id": dailyID, "project_id": f.projectID, "work_item_id": f.itemID, "workflow_form_instance_id": formID, "report_date": "2026-10-12", "structured_json": recordFromAny(form["answers"])}
	if err := enrichNativeDailySourceForActor(ctx, db, currentSource, false, f.workerID); err != nil {
		t.Fatal(err)
	}
	currentPlans := workflowAnswerRecordList(recordFromAny(currentSource["structured_json"])["next_plan_items"])
	if len(workflowAnswerRecordList(recordFromAny(currentSource["structured_json"])["material_arrival_items"])) != 1 || currentPlans[0]["state"] != "Hoàn thành" || currentPlans[1]["state"] != "Observed on site" {
		t.Fatal("persisted form suppressed live BM06 state or reread arrivals", currentSource)
	}
	for _, grant := range []struct{ actor, permission string }{{f.workerID, "work_report.submit"}, {f.pmID, "work_report.review"}, {f.pmID, "work_report.view"}} {
		permissionID := ensurePermission(t, db, grant.permission, grant.permission, "work_report")
		mustExec(t, db, `INSERT INTO role_permissions(role_id,permission_id) SELECT role_id,$2 FROM user_roles WHERE user_id=$1 ON CONFLICT DO NOTHING`, grant.actor, permissionID)
	}
	delivered, err := s.SubmitWorkReport(ctx, dailyID, WorkReportSubmitRequest{RecipientID: f.pmID}, f.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	deliveryID := cleanID(delivered["work_report_id"])
	assertCount(t, db, `SELECT count(*) FROM work_report_revisions WHERE report_id=$1 AND revision_no=1 AND jsonb_array_length(snapshot_json->'structured_json'->'material_arrival_items')=1 AND snapshot_json->'structured_json'->'material_arrival_items'->0->>'source_id'=$2`, 1, deliveryID, first)
	assertCount(t, db, `SELECT count(*) FROM work_report_revisions WHERE report_id=$1 AND revision_no=1 AND snapshot_json->'structured_json'->'next_plan_items'->0->>'state'='Sẵn sàng' AND snapshot_json->'structured_json'->'next_plan_items'->1->>'state'='Observed on site'`, 1, deliveryID)
	var oldDelivery string
	if err := db.QueryRowContext(ctx, `SELECT snapshot_json::text FROM work_report_revisions WHERE report_id=$1 AND revision_no=1`, deliveryID).Scan(&oldDelivery); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReviewWorkReport(ctx, deliveryID, "request-changes", WorkReportReviewRequest{Comment: "Update source revision"}, f.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	comparison, err := s.repo.WorkReportSource(ctx, dailyID)
	if err != nil {
		t.Fatal(err)
	}
	if boolValue(comparison["delivery_source_changed"]) {
		t.Fatal("live BM06 task state enabled resubmission without a captured form amendment")
	}
	if _, err := s.SubmitWorkReport(ctx, dailyID, WorkReportSubmitRequest{RecipientID: f.pmID}, f.workerID, "Worker"); err == nil {
		t.Fatal("resubmission accepted unchanged form facts")
	} else {
		var statusErr StatusError
		if !errors.As(err, &statusErr) || cleanText(recordFromAny(statusErr.Details)["reason"]) != "unchanged_since_change_request" {
			t.Fatalf("unexpected unchanged resubmission error: %v", err)
		}
	}
	canonical := recordFromAny(form["answers"])
	canonical["weather"] = "hot"
	plans := workflowAnswerRecordList(canonical["next_plan_items"])
	plans[1]["state"] = ""
	canonical["next_plan_items"] = plans
	if _, err := s.AmendWorkflowForm(ctx, formID, WorkflowFormAmendRequest{Answers: canonical, ExpectedRevision: 1, Reason: "Updated receipt observations"}, f.workerID, "Worker"); err != nil {
		nativeDailyFormFatal(t, "amend", err)
	}
	assertCount(t, db, `SELECT count(*) FROM workflow_form_events WHERE instance_id=$1 AND event_type='amended' AND jsonb_array_length(metadata_json->'native_daily_sources'->'structured_json'->'material_arrival_items')=2`, 1, formID)
	comparison, err = s.repo.WorkReportSource(ctx, dailyID)
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(comparison["delivery_source_changed"]) {
		t.Fatal("captured form amendment did not enable resubmission")
	}
	if _, err := s.SubmitWorkReport(ctx, dailyID, WorkReportSubmitRequest{RecipientID: f.pmID}, f.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM work_report_revisions WHERE report_id=$1 AND revision_no=2 AND jsonb_array_length(snapshot_json->'structured_json'->'material_arrival_items')=2`, 1, deliveryID)
	assertCount(t, db, `SELECT count(*) FROM work_report_revisions WHERE report_id=$1 AND revision_no=2 AND snapshot_json->'structured_json'->'next_plan_items'->0->>'state'='Hoàn thành' AND snapshot_json->'structured_json'->'next_plan_items'->1->>'state'='' AND snapshot_json->'structured_json'->'next_plan_items'->1->'state_source'->>'source_type'='explicit_clear'`, 1, deliveryID)
	var afterEvent, afterDelivery string
	if err := db.QueryRowContext(ctx, `SELECT metadata_json::text FROM workflow_form_events WHERE instance_id=$1 AND event_type='submitted'`, formID).Scan(&afterEvent); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT snapshot_json::text FROM work_report_revisions WHERE report_id=$1 AND revision_no=1`, deliveryID).Scan(&afterDelivery); err != nil {
		t.Fatal(err)
	}
	if afterEvent != originalEvent || afterDelivery != oldDelivery {
		t.Fatal("historical form or delivery changed")
	}
	quantity := 7.0
	ownerless, err := s.CreateIncomingAcceptance(ctx, f.projectID, IncomingAcceptanceRequest{ReceivedAt: "2026-10-12", Items: []IncomingAcceptanceItemRequest{{MaterialName: "Project receipt", Unit: "m2", DeliveredQuantity: &quantity, AcceptedQuantity: &quantity}}}, f.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SubmitIncomingAcceptance(ctx, cleanID(ownerless["id"]), f.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	manualSource := Record{"id": newID(), "project_id": f.projectID, "work_item_id": f.itemID, "report_date": "2026-10-12", "structured_json": Record{}}
	if err := enrichNativeDailySourceForActor(ctx, db, manualSource, false, f.workerID); err != nil {
		t.Fatal(err)
	}
	if rows := workflowAnswerRecordList(recordFromAny(manualSource["structured_json"])["material_arrival_items"]); len(rows) != 3 {
		t.Fatal("canonical ownerless receipt excluded", rows)
	}
	// A legacy submitted form with no internal snapshot never gains current receipts.
	mustExec(t, db, `UPDATE workflow_form_events SET metadata_json=metadata_json-'native_daily_sources' WHERE instance_id=$1`, formID)
	legacy := Record{"id": dailyID, "project_id": f.projectID, "work_item_id": f.itemID, "workflow_form_instance_id": formID, "reported_by": f.workerID, "report_date": "2026-10-12", "structured_json": Record{}}
	if err := freezeNativeWorkReportEvidence(ctx, db, legacy, true); err != nil {
		t.Fatal(err)
	}
	if rows := workflowAnswerRecordList(recordFromAny(legacy["structured_json"])["material_arrival_items"]); len(rows) != 0 {
		t.Fatal("legacy missing provenance enriched live", rows)
	}
	bindWorkItemDomainExecutionContext(t, db, f, "construction", "construction", "workflow_form", "")
	manual, err := s.CreateDailyReport(ctx, f.projectID, ConstructionDailyReportCreateRequest{WorkItemID: f.itemID, ReportDate: "2026-10-12", Weather: "normal", DoneItems: []ConstructionDailyReportWorkItemRequest{{Title: "Manual daily observation"}}, EvidenceIDs: []string{photoID}}, f.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	manualDelivery, err := s.SubmitWorkReport(ctx, cleanID(manual["id"]), WorkReportSubmitRequest{RecipientID: f.pmID}, f.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM work_report_revisions WHERE report_id=$1 AND revision_no=1 AND jsonb_array_length(snapshot_json->'structured_json'->'material_arrival_items')=3`, 1, cleanID(manualDelivery["work_report_id"]))
}
