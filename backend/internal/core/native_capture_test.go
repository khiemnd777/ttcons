package core

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNativeDailyCaptureKeepsUnknownAndUsageSeparate(t *testing.T) {
	rows, err := normalizeConstructionSupervisionAssessments(nil)
	if err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
	for _, items := range [][]ConstructionSupervisionAssessment{
		{{Code: "01", Assessment: "pass"}}, {{Code: "quality", Assessment: "approved"}}, {{Code: "quality", Assessment: "pass"}, {Code: "quality", Assessment: "fail"}},
	} {
		if _, err := normalizeConstructionSupervisionAssessments(items); err == nil {
			t.Fatal("invalid assessment accepted")
		}
	}
	zero := 0.0
	nan := math.NaN()
	for _, items := range [][]ConstructionMaterialArrival{
		{{MaterialName: "Tile", Quantity: &zero}}, {{MaterialName: "Tile", Quantity: &nan, Unit: "m2"}}, {{MaterialName: "Tile", RowID: "bad"}},
	} {
		if _, err := normalizeConstructionMaterialArrivals(items); err == nil {
			t.Fatal("invalid arrival accepted")
		}
	}
	arrivals, err := normalizeConstructionMaterialArrivals([]ConstructionMaterialArrival{{MaterialName: "Tile"}, {MaterialName: "Sand", Quantity: &zero, Unit: "m3"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := arrivals[0]["quantity"]; exists {
		t.Fatal("unknown quantity converted to zero")
	}
	if arrivals[1]["quantity"] != 0.0 || !validActivityUUID(cleanID(arrivals[0]["row_id"])) {
		t.Fatal(arrivals)
	}
	if _, exists := arrivals[0]["source_id"]; exists {
		t.Fatal("arrival fabricated warehouse receipt")
	}
}

func TestNativeEvidenceCaptureStrictAndDoesNotInferTime(t *testing.T) {
	for _, value := range []any{nil, "text", Record{"captured_at": "2026-10-08T08:00:00"}, Record{"photo_phase": "completed"}, Record{"checksum_sha256": "override"}, Record{"location": 42}} {
		req := WorkItemEvidenceRequest{EvidenceType: "photo", Payload: Record{"capture_metadata": value}}
		if err := normalizeEvidenceCaptureMetadata(&req); err == nil {
			t.Fatalf("accepted %#v", value)
		}
	}
	req := WorkItemEvidenceRequest{EvidenceType: "photo", Caption: "Fault observed", Payload: Record{"capture_metadata": Record{"photo_phase": "defect", "location": " A "}}}
	if err := normalizeEvidenceCaptureMetadata(&req); err != nil {
		t.Fatal(err)
	}
	data := recordFromAny(req.Payload["capture_metadata"])
	if data["photo_phase"] != "defect" || data["location"] != "A" || data["captured_at"] != nil || req.Caption != "Fault observed" {
		t.Fatal(req)
	}
	req.EvidenceType = "document"
	if normalizeEvidenceCaptureMetadata(&req) == nil {
		t.Fatal("capture metadata accepted on document")
	}
}

func TestNativeCaptureFormRevisionsAndEvidenceSnapshot(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_capture_", nil)
	ctx := context.Background()
	// Newly published definitions add fields without changing any old version.
	assertCount(t, db, `SELECT count(*) FROM workflow_form_fields field JOIN workflow_form_versions version ON version.id=field.form_version_id JOIN workflow_form_templates template ON template.id=version.template_id WHERE template.code='builtin_construction_daily_report' AND version.schema_json->>'native_capture_revision'='1' AND NOT(version.schema_json ? 'native_risk_assignment_revision') AND field.field_key IN('supervision_assessments','material_arrival_items')`, 2)
	assertCount(t, db, `SELECT count(*) FROM workflow_form_fields field JOIN workflow_form_versions version ON version.id=field.form_version_id JOIN workflow_form_templates template ON template.id=version.template_id WHERE template.code='builtin_construction_daily_report' AND NOT(version.schema_json ? 'native_capture_revision') AND field.field_key IN('supervision_assessments','material_arrival_items')`, 0)
	// V157 clones both V154 definitions; validate original and assessment
	// revisions separately rather than treating the inherited marker as unique.
	for _, assessment := range []bool{false, true} {
		assertCount(t, db, `SELECT count(*) FROM workflow_form_fields field JOIN workflow_form_versions version ON version.id=field.form_version_id JOIN workflow_form_templates template ON template.id=version.template_id WHERE template.code IN('builtin_subcontractor_checklist','client_subcontractor_acceptance_package') AND version.schema_json->>'native_field_capture_revision'='1' AND (version.schema_json ? 'native_subcontractor_assessment_revision')=$1 AND field.field_key='checklist_items' AND field.validation_json->'item_fields' @> '[{"key":"actual_quantity"}]'::jsonb`, 2, assessment)
	}
	fixture := setupWorkItemFlowFixture(t, db, false)
	service := NewService(NewRepository(db), "test-secret")
	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{EvidenceType: "photo", Name: "Original.jpg", FileURL: "https://example.com/original.jpg", Caption: "Observed crack", Payload: Record{"capture_metadata": Record{"photo_phase": "defect", "captured_at": "2026-10-08T08:00:00+07:00", "location": "Area A"}}}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	source := Record{"project_id": fixture.projectID, "primary_evidence_id": cleanID(evidence["id"]), "structured_json": Record{"material_usage_items": []Record{{"material_name": "Tile", "quantity": 1}}, "material_arrival_items": []Record{{"material_name": "Sand"}}}}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = freezeNativeWorkReportEvidence(ctx, tx, source, true); err != nil {
		t.Fatal(err)
	}
	snapshot := workReportSnapshot(source)
	before, _ := json.Marshal(snapshot)
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE evidence_records SET payload_json='{"capture_metadata":{"location":"Area B"}}'::jsonb WHERE id=$1`, cleanID(evidence["id"]))
	data := recordFromAny(source["bm_gs_evidence"].([]Record)[0]["payload_json"])
	for _, key := range []string{"bm_gs_json", "bm_gs_version", "bm_gs_updated_by", "bm_gs_updated_at"} {
		if _, exists := source["bm_gs_evidence"].([]Record)[0][key]; exists {
			t.Fatalf("native snapshot contains supplemental field %s", key)
		}
	}
	if recordFromAny(data["capture_metadata"])["location"] != "Area A" {
		t.Fatal("snapshot used live metadata")
	}
	after, _ := json.Marshal(snapshot)
	if string(before) != string(after) {
		t.Fatal("snapshot changed")
	}
	if workReportSnapshot(Record{"project_id": fixture.projectID})["bm_gs_evidence"] != nil {
		t.Fatal("legacy missing evidence metadata fabricated")
	}
}

func TestNativeMigrationAllocatesAboveDraftAndHidesRowIDs(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_draft_", nil)
	for _, code := range []string{"builtin_construction_daily_report", "client_site_measurement_record", "client_survey_work_package", "builtin_subcontractor_checklist", "client_subcontractor_acceptance_package"} {
		mustExec(t, db, `UPDATE workflow_form_templates SET code=code||'_migration_fixture' WHERE code=$1`, code)
		templateID, publishedID, draftID := newID(), newID(), newID()
		mustExec(t, db, `INSERT INTO workflow_form_templates(id,code,name,form_type,status) VALUES($1,$2,'Native fixture','daily_report','active')`, templateID, code)
		mustExec(t, db, `INSERT INTO workflow_form_versions(id,template_id,version_no,status,schema_json) VALUES($1,$3,1,'published','{"marker":"published"}'),($2,$3,9,'draft','{"marker":"draft"}')`, publishedID, draftID, templateID)
		for _, key := range []string{"crew_items", "dimension_items", "checklist_items"} {
			mustExec(t, db, `INSERT INTO workflow_form_fields(form_version_id,field_key,label,field_type,validation_json) VALUES($1,$2,$2,'structured_list','{"item_fields":[{"key":"title","type":"text"}]}')`, publishedID, key)
		}
	}
	daily, err := os.ReadFile("../../migrations/V153__native_daily_report_capture_revision.sql")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, string(daily))
	remaining, err := os.ReadFile("../../migrations/V154__native_field_capture_gaps.sql")
	if err != nil {
		t.Fatal(err)
	}
	// Table additions were verified by the real migration runner. Execute the
	// actual revision body against fixtures containing legitimate higher drafts.
	marker := strings.Index(string(remaining), "DO $$")
	if marker < 0 {
		t.Fatal("missing revision migration")
	}
	mustExec(t, db, string(remaining)[marker:])
	assertCount(t, db, `SELECT count(*) FROM workflow_form_versions version JOIN workflow_form_templates template ON template.id=version.template_id WHERE template.code IN('builtin_construction_daily_report','client_site_measurement_record','client_survey_work_package','builtin_subcontractor_checklist','client_subcontractor_acceptance_package') AND version.version_no=10 AND version.status='published'`, 5)
	assertCount(t, db, `SELECT count(*) FROM workflow_form_versions version JOIN workflow_form_templates template ON template.id=version.template_id WHERE template.code IN('builtin_construction_daily_report','client_site_measurement_record','client_survey_work_package','builtin_subcontractor_checklist','client_subcontractor_acceptance_package') AND version.version_no=9 AND version.status='draft' AND version.schema_json='{"marker":"draft"}'::jsonb`, 5)
	assertCount(t, db, `SELECT count(*) FROM workflow_form_fields field JOIN workflow_form_versions version ON version.id=field.form_version_id JOIN workflow_form_templates template ON template.id=version.template_id WHERE template.code='builtin_construction_daily_report' AND version.version_no=10 AND field.field_key='material_arrival_items' AND field.validation_json->'item_fields' @> '[{"key":"row_id"}]'::jsonb`, 0)
}

func TestNativeOmittedDailySectionsUseLockedCurrentSource(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_omit_", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fixture := setupProjectScopeFixture(t, db, []string{"operation.view", "operation.report"})
	id := newID()
	mustExec(t, db, `INSERT INTO operation_daily_reports(id,project_id,report_date,reported_by,report_delivery_eligible,structured_json) VALUES($1,$2,'2026-10-08',$3,true,'{"material_arrival_items":[{"material_name":"Old"}]}')`, id, fixture.projectID, fixture.userID)
	holder, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback()
	var holderPID int
	if err = holder.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatal(err)
	}
	if _, err = holder.ExecContext(ctx, `SELECT id FROM operation_daily_reports WHERE id=$1 FOR UPDATE`, id); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := NewRepository(db).UpdateDailyReportWithDelivery(ctx, id, Record{"structured_json": Record{"material_arrival_items": []Record{{"material_name": "Stale"}}}}, WorkReportDeliveryRequest{}, fixture.userID, "Actor", "material_arrival_items")
		result <- err
	}()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, holderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("save did not wait for source lock")
		case <-ticker.C:
		}
	}
	if _, err = holder.ExecContext(ctx, `UPDATE operation_daily_reports SET structured_json='{"material_arrival_items":[{"material_name":"New"}]}' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if err = holder.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-result; err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM operation_daily_reports WHERE id=$1 AND structured_json->'material_arrival_items'->0->>'material_name'='New'`, 1, id)
}

func TestNativeActionAndHandoverCaptureKeepOwningLifecycle(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_owner_", nil)
	ctx := context.Background()
	owner := setupProjectScopeFixture(t, db, []string{"construction.action.edit", "handover.verify"})
	other := setupProjectScopeFixture(t, db, []string{"construction.action.edit", "handover.verify"})
	service := NewService(NewRepository(db), "test-secret")
	actionID := newID()
	mustExec(t, db, `INSERT INTO construction_action_items(id,project_id,title,assigned_user_id,due_at,created_by) VALUES($1,$2,'Crack',$3,'2026-10-10T08:00:00+07:00',$3)`, actionID, owner.projectID, owner.userID)
	severity, location := "critical", "Area A"
	request := ConstructionActionUpdateRequest{Title: "Crack", Priority: "normal", AssignedUserID: owner.userID, DueAt: "2026-10-10T08:00:00+07:00", Severity: &severity, Location: &location}
	if _, err := service.UpdateConstructionAction(ctx, actionID, request, other.userID, "Other"); err == nil {
		t.Fatal("cross-project field update accepted")
	}
	if _, err := service.UpdateConstructionAction(ctx, actionID, request, owner.userID, "Owner"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM construction_action_items WHERE id=$1 AND severity='critical' AND priority='normal' AND status='open'`, 1, actionID)
	request.Severity = nil
	request.Location = nil
	if _, err := service.UpdateConstructionAction(ctx, actionID, request, owner.userID, "Owner"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM construction_action_items WHERE id=$1 AND severity='critical' AND location='Area A'`, 1, actionID)
	mustExec(t, db, `UPDATE construction_action_items SET status='waiting_confirmation' WHERE id=$1`, actionID)
	if _, err := service.UpdateConstructionAction(ctx, actionID, request, owner.userID, "Owner"); err == nil {
		t.Fatal("locked action editable")
	}
	handoverID, itemID := newID(), newID()
	mustExec(t, db, `INSERT INTO handover_records(id,project_id,handover_code,status,readiness_snapshot_json) VALUES($1,$2,'Native handover','draft','{"quality_ready":false}')`, handoverID, owner.projectID)
	mustExec(t, db, `INSERT INTO handover_checklist_items(id,handover_id,project_id,item_code,category,title,required) VALUES($1,$2,$3,'native_test','other','Physical condition',false)`, itemID, handoverID, owner.projectID)
	fail, defect := "fail", "Unfinished joint"
	handoverRequest := HandoverChecklistItemRequest{Status: "pending", PhysicalResult: &fail, Defect: &defect}
	if _, err := service.UpdateHandoverChecklistItem(ctx, itemID, handoverRequest, other.userID, "Other"); err == nil {
		t.Fatal("cross-project handover update accepted")
	}
	if _, err := service.UpdateHandoverChecklistItem(ctx, itemID, handoverRequest, owner.userID, "Owner"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM handover_checklist_items WHERE id=$1 AND physical_result='fail' AND status='pending'`, 1, itemID)
	assertCount(t, db, `SELECT count(*) FROM handover_records WHERE id=$1 AND status='draft' AND readiness_snapshot_json='{"quality_ready":false}'::jsonb`, 1, handoverID)
	if _, err := service.UpdateHandoverChecklistItem(ctx, itemID, HandoverChecklistItemRequest{Status: "pending"}, owner.userID, "Owner"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM handover_checklist_items WHERE id=$1 AND physical_result='fail' AND defect='Unfinished joint'`, 1, itemID)
	mustExec(t, db, `UPDATE handover_records SET status='submitted' WHERE id=$1`, handoverID)
	if _, err := service.HandoverCheck(ctx, handoverID, owner.userID, "Owner"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM handover_records WHERE id=$1 AND readiness_snapshot_json->'checklist_items' @> '[{"physical_result":"fail","defect":"Unfinished joint"}]'::jsonb`, 1, handoverID)
	mustExec(t, db, `UPDATE handover_records SET status='handed_over' WHERE id=$1`, handoverID)
	if _, err := service.UpdateHandoverChecklistItem(ctx, itemID, handoverRequest, owner.userID, "Owner"); err == nil {
		t.Fatal("completed handover editable")
	}
}

func TestNativeNewFormFieldsValidateIndependentMeasuredFacts(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_form_", nil)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fields, err := repo.Query(ctx, `SELECT * FROM workflow_form_fields WHERE form_version_id=(SELECT version.id FROM workflow_form_versions version JOIN workflow_form_templates template ON template.id=version.template_id WHERE template.code='builtin_subcontractor_checklist' AND version.status='published' ORDER BY version.version_no DESC LIMIT 1)`)
	if err != nil {
		t.Fatal(err)
	}
	instance := Record{"form_code": "builtin_subcontractor_checklist", "form_type": "checklist"}
	answers := Record{"checklist_items": []Record{{"title": "Internal item", "claimed_quantity": 12, "confirmed_quantity": 12, "actual_quantity": 0, "design_quantity": 3, "unit": "m2", "status": "done"}}}
	normalized, err := service.validateWorkflowFormAnswers(ctx, instance, fields, answers, false)
	if err != nil {
		t.Fatal(err)
	}
	row := workflowAnswerRecordList(normalized["checklist_items"])[0]
	if number(row["actual_quantity"]) != 0 || number(row["design_quantity"]) != 3 {
		t.Fatal(row)
	}
	delete(answers["checklist_items"].([]Record)[0], "actual_quantity")
	delete(answers["checklist_items"].([]Record)[0], "design_quantity")
	normalized, err = service.validateWorkflowFormAnswers(ctx, instance, fields, answers, false)
	if err != nil {
		t.Fatal(err)
	}
	if workflowAnswerRecordList(normalized["checklist_items"])[0]["actual_quantity"] != nil {
		t.Fatal("claimed or confirmed inferred actual quantity")
	}
	answers["checklist_items"].([]Record)[0]["actual_quantity"] = 1
	delete(answers["checklist_items"].([]Record)[0], "unit")
	if _, err = service.validateWorkflowFormAnswers(ctx, instance, fields, answers, false); err == nil {
		t.Fatal("measurement without unit accepted")
	}
	fields, err = repo.Query(ctx, `SELECT * FROM workflow_form_fields WHERE form_version_id=(SELECT version.id FROM workflow_form_versions version JOIN workflow_form_templates template ON template.id=version.template_id WHERE template.code='builtin_construction_daily_report' AND version.status='published' ORDER BY version.version_no DESC LIMIT 1)`)
	if err != nil {
		t.Fatal(err)
	}
	rowID := newID()
	dailyAnswers := Record{"material_arrival_items": []Record{{"row_id": rowID, "material_name": "Tile", "quantity": 0, "unit": "m2"}}}
	normalized, err = service.validateWorkflowFormAnswers(ctx, Record{"form_code": "builtin_construction_daily_report", "form_type": "daily_report"}, fields, dailyAnswers, false)
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(workflowAnswerRecordList(normalized["material_arrival_items"])[0]["row_id"]) != rowID {
		t.Fatal("hidden native row identity lost")
	}
}
