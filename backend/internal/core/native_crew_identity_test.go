package core

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestNativeWorkflowDailyCrewIdentitySurvivesAmendment(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_crew_", nil)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)
	// The native capture contract belongs to the new published revision. The
	// compatibility default intentionally excludes operational_capture_v3 pins.
	var versionID string
	err := db.QueryRowContext(ctx, `SELECT version.id FROM workflow_form_versions version JOIN workflow_form_templates template ON template.id=version.template_id WHERE template.code='builtin_construction_daily_report' AND version.status='published' ORDER BY version.version_no DESC LIMIT 1`).Scan(&versionID)
	if err != nil {
		t.Fatal(err)
	}
	formID := newID()
	mustExec(t, db, `INSERT INTO workflow_form_instances(id,project_id,work_item_id,form_template_id,form_version_id,form_type,title,created_by) SELECT $1,$2,$3,version.template_id,version.id,'daily_report','Native crew',$4 FROM workflow_form_versions version WHERE version.id=$5`, formID, fixture.projectID, fixture.itemID, fixture.workerID, versionID)
	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{EvidenceType: "photo", Name: "Crew photo", FileURL: "https://example.com/crew.jpg"}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	// Rev7 removed the former confirmed/progress/site-photo header fields.
	// Evidence stays linked through the completed native item.
	answers := Record{"report_date": "2026-10-08", "reported_by": "Worker", "construction_scope": "civil", "weather": "normal", "completed_items": []any{map[string]any{"title": "Install wall", "actual_quantity": "2", "unit": "m2", "result": "done", "evidence_id": cleanID(evidence["id"])}}, "crew_items": []any{map[string]any{"crew_name": "Crew A", "worker_count": "3", "work_hours": "8"}}}
	submitted, err := service.SubmitWorkflowForm(ctx, formID, WorkflowFormSaveRequest{Answers: answers}, fixture.workerID, "Worker")
	if err != nil {
		var statusErr StatusError
		if errors.As(err, &statusErr) {
			t.Fatalf("submit: code=%s details=%+v: %v", statusErr.Code, statusErr.Details, err)
		}
		t.Fatal(err)
	}
	canonical := recordFromAny(submitted["answers"])
	crew := workflowAnswerRecordList(canonical["crew_items"])
	if len(crew) != 1 || !validActivityUUID(cleanID(crew[0]["row_id"])) {
		t.Fatal("canonical crew identity missing")
	}
	rowID := cleanID(crew[0]["row_id"])
	crew[0]["representative"] = "Crew representative"
	canonical["weather"] = "hot"
	amended, err := service.AmendWorkflowForm(ctx, formID, WorkflowFormAmendRequest{Answers: canonical, ExpectedRevision: 1, Reason: "Record crew representative"}, fixture.workerID, "Worker")
	if err != nil {
		var statusErr StatusError
		if errors.As(err, &statusErr) {
			t.Fatalf("amend: code=%s details=%+v: %v", statusErr.Code, statusErr.Details, err)
		}
		t.Fatal(err)
	}
	canonical = recordFromAny(amended["answers"])
	var dailyID string
	if err = db.QueryRowContext(ctx, `SELECT id FROM operation_daily_reports WHERE workflow_form_instance_id=$1`, formID).Scan(&dailyID); err != nil {
		t.Fatal(err)
	}
	source, err := repo.GetByID(ctx, "operation_daily_reports", dailyID)
	if err != nil {
		t.Fatal(err)
	}
	sourceCrew := workflowAnswerRecordList(recordFromAny(source["structured_json"])["crew_items"])
	if len(sourceCrew) != 1 || cleanID(sourceCrew[0]["row_id"]) != rowID || sourceCrew[0]["representative"] != "Crew representative" {
		t.Fatal("canonical daily bridge lost crew identity or contact")
	}
	reportID := newID()
	mustExec(t, db, `INSERT INTO work_reports(id,source_kind,source_id,project_id,report_type,report_date,author_id,recipient_id,status,current_revision_no) VALUES($1,'operation_daily_report',$2,$3,'construction_daily','2026-10-08',$4,$5,'submitted',1)`, reportID, dailyID, fixture.projectID, fixture.workerID, fixture.pmID)
	canonical["weather"] = "rain"
	request := WorkflowFormAmendRequest{Answers: canonical, ExpectedRevision: 2, Reason: "Correct conditions"}
	if _, err = service.AmendWorkflowForm(ctx, formID, request, fixture.workerID, "Worker"); err == nil {
		t.Fatal("delivered source permits amendment")
	}
	mustExec(t, db, `UPDATE work_reports SET status='needs_changes',reviewed_by=$2,reviewed_at=now() WHERE id=$1`, reportID, fixture.pmID)
	amended, err = service.AmendWorkflowForm(ctx, formID, request, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(workflowAnswerRecordList(recordFromAny(amended["answers"])["crew_items"])[0]["row_id"]) != rowID {
		t.Fatal("requested amendment changed crew identity")
	}
}

func TestNativeCrewBackfillPreservesHistoricalRevisionHashes(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_backfill_", nil)
	ctx := context.Background()
	fixture := setupWorkItemFlowFixture(t, db, false)
	repo := NewRepository(db)
	versionID, err := repo.DefaultWorkflowFormVersionID(ctx, "daily_report", false, "Daily report thi công")
	if err != nil {
		t.Fatal(err)
	}
	formID, dailyID, historicalID, reportID := newID(), newID(), newID(), newID()
	original := `{"crew_items":[{"crew_name":"Existing crew","worker_count":"3","work_hours":"8"}]}`
	mustExec(t, db, `INSERT INTO workflow_form_instances(id,project_id,work_item_id,form_template_id,form_version_id,form_type,title,status,current_revision,answers_snapshot_json,created_by) SELECT $1,$2,$3,version.template_id,version.id,'daily_report','Existing source','submitted',1,$4::jsonb,$5 FROM workflow_form_versions version WHERE version.id=$6`, formID, fixture.projectID, fixture.itemID, original, fixture.workerID, versionID)
	mustExec(t, db, `INSERT INTO workflow_form_answers(instance_id,field_key,field_type,value_json,revision_no,updated_by) VALUES($1,'crew_items','structured_list',$2::jsonb,1,$3)`, formID, `[{"crew_name":"Existing crew","worker_count":"3","work_hours":"8"}]`, fixture.workerID)
	mustExec(t, db, `INSERT INTO operation_daily_reports(id,project_id,reported_by,report_date,workflow_form_instance_id,work_item_id,structured_json,report_delivery_eligible) VALUES($1,$2,$3,'2026-10-08',$4,$5,$6::jsonb,true)`, dailyID, fixture.projectID, fixture.workerID, formID, fixture.itemID, original)
	mustExec(t, db, `INSERT INTO operation_daily_reports(id,project_id,reported_by,report_date,structured_json,report_delivery_eligible) VALUES($1,$2,$3,'2026-10-08',$4::jsonb,true)`, historicalID, fixture.projectID, fixture.workerID, original)
	mustExec(t, db, `INSERT INTO work_reports(id,source_kind,source_id,project_id,report_type,report_date,author_id,recipient_id,status,current_revision_no) VALUES($1,'operation_daily_report',$2,$3,'construction_daily','2026-10-08',$4,$5,'submitted',1)`, reportID, historicalID, fixture.projectID, fixture.workerID, fixture.pmID)
	mustExec(t, db, `INSERT INTO work_report_revisions(report_id,revision_no,snapshot_json,snapshot_sha256,submitted_by) VALUES($1,1,$2::jsonb,$3,$4)`, reportID, original, sha256Hex([]byte(original)), fixture.workerID)
	migration, err := os.ReadFile("../../migrations/V152__bm_gs_source_fields.sql")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(migration), "-- Only current editable sources")
	if start < 0 {
		t.Fatal("missing backfill boundary")
	}
	mustExec(t, db, string(migration)[start:])
	assertCount(t, db, `SELECT count(*) FROM operation_daily_reports source JOIN workflow_form_instances form ON form.id=source.workflow_form_instance_id WHERE source.id=$1 AND NULLIF(source.structured_json->'crew_items'->0->>'row_id','') IS NOT NULL AND source.structured_json->'crew_items'->0->>'row_id'=form.answers_snapshot_json->'crew_items'->0->>'row_id'`, 1, dailyID)
	assertCount(t, db, `SELECT count(*) FROM work_report_revisions WHERE report_id=$1 AND snapshot_sha256=$2 AND snapshot_json=$3::jsonb`, 1, reportID, sha256Hex([]byte(original)), original)
	assertCount(t, db, `SELECT count(*) FROM operation_daily_reports WHERE id=$1 AND structured_json=$2::jsonb`, 1, historicalID, original)
	var stableID string
	if err = db.QueryRowContext(ctx, `SELECT structured_json->'crew_items'->0->>'row_id' FROM operation_daily_reports WHERE id=$1`, dailyID).Scan(&stableID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, string(migration)[start:])
	assertCount(t, db, `SELECT count(*) FROM operation_daily_reports WHERE id=$1 AND structured_json->'crew_items'->0->>'row_id'=$2`, 1, dailyID, stableID)
	assertCount(t, db, `SELECT count(*) FROM audit_logs WHERE event_type='bm_gs.crew_identity.backfilled' AND target_id IN($1,$2)`, 2, dailyID, formID)
}
