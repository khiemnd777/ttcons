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

func nativeRiskPlanText(value string) *string { return &value }

func TestNativeDailyRiskPlanValidationAndNullPercentage(t *testing.T) {
	zero := 0.0
	if _, _, err := normalizeConstructionReportNextItems([]ConstructionDailyReportWorkItemRequest{{ProgressPercent: &zero}}); err == nil {
		t.Fatal("zero progress without task title silently dropped")
	}
	if _, err := normalizeConstructionReportBlockers([]ConstructionDailyReportBlockerRequest{{Owner: nativeRiskPlanText("Named engineer")}}); err == nil {
		t.Fatal("risk metadata without content silently dropped")
	}
	if rows, _, err := normalizeConstructionReportNextItems([]ConstructionDailyReportWorkItemRequest{{RowID: newID(), Receiver: nativeRiskPlanText("")}}); err != nil || len(rows) != 0 {
		t.Fatal("blank placeholder rejected", err)
	}
	for _, row := range []ConstructionDailyReportBlockerRequest{
		{Content: "Risk", RowID: "00000000-0000-0000-0000-000000000000"},
		{Content: "Risk", RowID: "invalid"},
		{Content: "Risk", DetectedAt: nativeRiskPlanText("2026-02-30")},
		{Content: "Risk", TargetDate: nativeRiskPlanText("2026-10-08T08:00:00Z")},
		{Content: "Risk", Severity: nativeRiskPlanText("urgent")},
		{Content: "Risk", Cause: nativeRiskPlanText(strings.Repeat("界", 1001))},
	} {
		if _, err := normalizeConstructionReportBlockers([]ConstructionDailyReportBlockerRequest{row}); err == nil {
			t.Fatalf("accepted invalid blocker: %+v", row)
		}
	}
	id := newID()
	if _, err := normalizeConstructionReportBlockers([]ConstructionDailyReportBlockerRequest{{Content: "A", RowID: id}, {Content: "B", RowID: id}}); err == nil {
		t.Fatal("duplicate blocker identity accepted")
	}
	for _, percent := range []float64{-1, 101, math.NaN(), math.Inf(1)} {
		if _, _, err := normalizeConstructionReportNextItems([]ConstructionDailyReportWorkItemRequest{{Title: "Plan", ProgressPercent: &percent}}); err == nil {
			t.Fatal("invalid task percentage accepted")
		}
	}
	if _, _, err := normalizeConstructionReportNextItems([]ConstructionDailyReportWorkItemRequest{{Title: "Plan", IssuedDate: nativeRiskPlanText("2026-13-01")}}); err == nil {
		t.Fatal("invalid issued date accepted")
	}
	for _, test := range []struct {
		json    string
		present bool
		value   any
	}{{`{"title":"Plan"}`, false, nil}, {`{"title":"Plan","progress_percent":null}`, true, nil}, {`{"title":"Plan","progress_percent":0}`, true, 0.0}} {
		var request ConstructionDailyReportWorkItemRequest
		if err := json.Unmarshal([]byte(test.json), &request); err != nil {
			t.Fatal(err)
		}
		rows, _, err := normalizeConstructionReportNextItems([]ConstructionDailyReportWorkItemRequest{request})
		if err != nil {
			t.Fatal(err)
		}
		value, present := rows[0]["progress_percent"]
		if present != test.present || value != test.value || !validActivityUUID(cleanID(rows[0]["row_id"])) {
			t.Fatal("omitted, null and zero task percentage collapsed", rows)
		}
	}
}

func TestNativeOperationsRiskPlanCreateEditClearAndProjection(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_risk_plan_", nil)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	f := setupWorkItemFlowFixture(t, db, false)
	bindWorkItemDomainExecutionContext(t, db, f, "construction", "construction", "workflow_form", "")
	evidence, err := service.AddWorkItemEvidence(ctx, f.itemID, WorkItemEvidenceRequest{EvidenceType: "photo", Name: "Risk photo", FileURL: "https://example.com/risk.jpg"}, f.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	zero := 0.0
	request := ConstructionDailyReportCreateRequest{ReportDate: "2026-10-08", Weather: "normal", WorkItemID: f.itemID, DoneItems: []ConstructionDailyReportWorkItemRequest{{Title: "Installed wall"}}, EvidenceIDs: []string{cleanID(evidence["id"])},
		Blockers:  []ConstructionDailyReportBlockerRequest{{Content: "Crack observed", RiskCode: nativeRiskPlanText("R-001"), DetectedAt: nativeRiskPlanText("2026-10-07"), Cause: nativeRiskPlanText("Joint movement"), Impact: nativeRiskPlanText("Inspection delayed"), Severity: nativeRiskPlanText("high"), ProposedAction: nativeRiskPlanText("Inspect joint"), Owner: nativeRiskPlanText("Named engineer"), TargetDate: nativeRiskPlanText("2026-10-09"), Status: nativeRiskPlanText("Recorded")}},
		NextItems: []ConstructionDailyReportWorkItemRequest{{Title: "Inspect joint", InstructionCode: nativeRiskPlanText("I-001"), Location: nativeRiskPlanText("Area A"), Subcontractor: nativeRiskPlanText("Crew A"), Receiver: nativeRiskPlanText("External representative"), Contact: nativeRiskPlanText("Capture contact"), IssuedDate: nativeRiskPlanText("2026-10-08"), DueDate: nativeRiskPlanText("2026-10-09"), ProgressPercent: &zero, State: nativeRiskPlanText("Issued")}}}
	report, err := service.CreateDailyReport(ctx, f.projectID, request, f.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	reportID := cleanID(report["id"])
	structured := recordFromAny(report["structured_json"])
	blockerID := cleanID(workflowAnswerRecordList(structured["blockers"])[0]["row_id"])
	nextID := cleanID(workflowAnswerRecordList(structured["next_plan_items"])[0]["row_id"])
	if !validActivityUUID(blockerID) || !validActivityUUID(nextID) {
		t.Fatal("canonical row identity not persisted")
	}
	ref := BMGSSourceRef{Kind: "daily_report", ID: reportID}
	for template, expected := range map[string]Record{"BM-GS-05": {"risk_code": "R-001", "detected_at": "2026-10-07", "owner": "Named engineer", "proposed_action": "Inspect joint"}, "BM-GS-06": {"instruction_code": "I-001", "representative": "External representative", "issued_at": "2026-10-08", "progress_percent": 0.0}} {
		sheet, projectErr := nativeBMGSProjectSheet(template, ref, report)
		if projectErr != nil || len(sheet.Rows) != 1 {
			t.Fatal(sheet, projectErr)
		}
		for key, value := range expected {
			if sheet.Rows[0][key] != value {
				t.Fatalf("%s.%s = %v want %v", template, key, sheet.Rows[0][key], value)
			}
		}
	}
	for _, grant := range []struct{ actor, permission string }{{f.workerID, "work_report.submit"}, {f.pmID, "work_report.review"}, {f.pmID, "work_report.view"}} {
		permissionID := ensurePermission(t, db, grant.permission, grant.permission, "work_report")
		mustExec(t, db, `INSERT INTO role_permissions(role_id,permission_id) SELECT role_id,$2 FROM user_roles WHERE user_id=$1 ON CONFLICT DO NOTHING`, grant.actor, permissionID)
	}
	if _, err = service.SubmitWorkReport(ctx, reportID, WorkReportSubmitRequest{RecipientID: f.pmID}, f.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	var deliveryID, frozenHash string
	if err = db.QueryRowContext(ctx, `SELECT report.id,revision.snapshot_sha256 FROM work_reports report JOIN work_report_revisions revision ON revision.report_id=report.id AND revision.revision_no=1 WHERE report.source_id=$1`, reportID).Scan(&deliveryID, &frozenHash); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM work_report_revisions WHERE report_id=$1 AND revision_no=1 AND snapshot_json->'structured_json'->'blockers'->0->>'owner'='Named engineer' AND snapshot_json->'structured_json'->'next_plan_items'->0->>'progress_percent'='0'`, 1, deliveryID)
	if _, err = service.UpdateDailyReport(ctx, reportID, request, f.workerID, "Worker"); err == nil {
		t.Fatal("delivered source capture still editable")
	}
	if _, err = service.ReviewWorkReport(ctx, deliveryID, "request-changes", WorkReportReviewRequest{Comment: "Correct observations"}, f.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	request.Blockers = []ConstructionDailyReportBlockerRequest{{RowID: blockerID, Content: "Crack updated"}}
	request.NextItems = []ConstructionDailyReportWorkItemRequest{{RowID: nextID, Title: "Inspect updated joint"}}
	if _, err = service.UpdateDailyReport(ctx, reportID, request, f.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM operation_daily_reports WHERE id=$1 AND structured_json->'blockers'->0->>'owner'='Named engineer' AND structured_json->'next_plan_items'->0->>'receiver'='External representative' AND structured_json->'next_plan_items'->0->>'progress_percent'='0'`, 1, reportID)
	var clear ConstructionDailyReportWorkItemRequest
	if err = json.Unmarshal([]byte(`{"title":"Inspect updated joint","row_id":"`+nextID+`","receiver":"","progress_percent":null}`), &clear); err != nil {
		t.Fatal(err)
	}
	request.NextItems = []ConstructionDailyReportWorkItemRequest{clear}
	request.Blockers[0].Owner = nativeRiskPlanText("")
	if _, err = service.UpdateDailyReport(ctx, reportID, request, f.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM operation_daily_reports WHERE id=$1 AND structured_json->'blockers'->0->>'owner'='' AND structured_json->'next_plan_items'->0->>'receiver'='' AND structured_json->'next_plan_items'->0->'progress_percent'='null'::jsonb AND structured_json->'next_plan_items'->0->>'instruction_code'='I-001'`, 1, reportID)
	request.NextItems = []ConstructionDailyReportWorkItemRequest{{Title: "New independent plan"}}
	request.Blockers = []ConstructionDailyReportBlockerRequest{}
	if _, err = service.UpdateDailyReport(ctx, reportID, request, f.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM operation_daily_reports WHERE id=$1 AND jsonb_array_length(structured_json->'blockers')=0 AND NOT(structured_json->'next_plan_items'->0 ? 'instruction_code') AND structured_json->'next_plan_items'->0->>'row_id'<>$2`, 1, reportID, nextID)
	assertCount(t, db, `SELECT count(*) FROM work_report_revisions WHERE report_id=$1 AND revision_no=1 AND snapshot_sha256=$2 AND snapshot_json->'structured_json'->'blockers'->0->>'owner'='Named engineer' AND snapshot_json->'structured_json'->'next_plan_items'->0->>'receiver'='External representative'`, 1, deliveryID, frozenHash)
	assertCount(t, db, `SELECT count(*) FROM audit_logs WHERE target_id=$1 AND event_type='daily_report_updated' AND metadata_json->'capture_before'->'blockers'->0->>'owner'='Named engineer' AND metadata_json->'capture_after'->'blockers'->0->>'owner'=''`, 1, reportID)
}

func TestNativeRiskPlanOmissionUsesLockedCurrentRows(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_risk_lock_", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f := setupProjectScopeFixture(t, db, []string{"operation.view", "operation.report"})
	id, blockerID, nextID := newID(), newID(), newID()
	initial := Record{"blockers": []Record{{"row_id": blockerID, "content": "Risk", "owner": "Old owner"}}, "next_plan_items": []Record{{"row_id": nextID, "title": "Plan", "receiver": "Old receiver", "progress_percent": 50}}}
	raw, _ := json.Marshal(initial)
	mustExec(t, db, `INSERT INTO operation_daily_reports(id,project_id,report_date,reported_by,report_delivery_eligible,structured_json) VALUES($1,$2,'2026-10-08',$3,true,$4::jsonb)`, id, f.projectID, f.userID, string(raw))
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
		_, saveErr := NewRepository(db).UpdateDailyReportWithDelivery(ctx, id, Record{"structured_json": Record{"blockers": []Record{{"row_id": blockerID, "content": "Updated risk"}}, "next_plan_items": []Record{{"row_id": nextID, "title": "Updated plan"}}}}, WorkReportDeliveryRequest{}, f.userID, "Actor")
		result <- saveErr
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
			t.Fatal("capture save did not wait for lock")
		case <-ticker.C:
		}
	}
	if _, err = holder.ExecContext(ctx, `UPDATE operation_daily_reports SET structured_json=jsonb_set(jsonb_set(jsonb_set(structured_json,'{blockers,0,owner}','"Latest owner"'),'{next_plan_items,0,receiver}','"Latest receiver"'),'{next_plan_items,0,progress_percent}','0') WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if err = holder.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-result; err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM operation_daily_reports WHERE id=$1 AND structured_json->'blockers'->0->>'owner'='Latest owner' AND structured_json->'next_plan_items'->0->>'receiver'='Latest receiver' AND structured_json->'next_plan_items'->0->>'progress_percent'='0'`, 1, id)
}

func TestNativeDailyBlockerRevisionIsImmutableAndSchemaValid(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_blocker_form_", nil)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	compatible, err := repo.DefaultWorkflowFormVersionID(ctx, "daily_report", false, "Thi công")
	if err != nil {
		t.Fatal(err)
	}
	before, err := repo.QueryOne(ctx, `SELECT md5(jsonb_agg(jsonb_build_object('version',version,'fields',(SELECT jsonb_agg(field ORDER BY field.id) FROM workflow_form_fields field WHERE field.form_version_id=version.id)) ORDER BY version.id)::text) AS digest FROM workflow_form_versions version JOIN workflow_form_templates template ON template.id=version.template_id WHERE template.code='builtin_construction_daily_report'`)
	if err != nil {
		t.Fatal(err)
	}
	var templateID string
	if err = db.QueryRowContext(ctx, `SELECT id FROM workflow_form_templates WHERE code='builtin_construction_daily_report'`).Scan(&templateID); err != nil {
		t.Fatal(err)
	}
	draftID := newID()
	mustExec(t, db, `INSERT INTO workflow_form_versions(id,template_id,version_no,status,schema_json) VALUES($1,$2,1000,'draft','{"draft":"untouched"}')`, draftID, templateID)
	migration, err := os.ReadFile("../../migrations/V156__native_daily_blocker_capture_revision.sql")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, string(migration))
	after, err := repo.QueryOne(ctx, `SELECT md5(jsonb_agg(jsonb_build_object('version',version,'fields',(SELECT jsonb_agg(field ORDER BY field.id) FROM workflow_form_fields field WHERE field.form_version_id=version.id)) ORDER BY version.id)::text) AS digest FROM workflow_form_versions version WHERE template_id=$1 AND version_no<1000`, templateID)
	if err != nil || before["digest"] != after["digest"] {
		t.Fatal("old form definitions changed", err)
	}
	assertCount(t, db, `SELECT count(*) FROM workflow_form_versions WHERE id=$1 AND version_no=1000 AND status='draft' AND schema_json='{"draft":"untouched"}'::jsonb`, 1, draftID)
	fields, err := repo.Query(ctx, `SELECT field.* FROM workflow_form_fields field JOIN workflow_form_versions version ON version.id=field.form_version_id WHERE version.template_id=$1 AND version.version_no=1001`, templateID)
	if err != nil {
		t.Fatal(err)
	}
	rowID := newID()
	answers := Record{"blockers": []Record{{"row_id": rowID, "content": "Native risk", "impact": "schedule", "severity": "high", "owner_role": "Supervisor", "target_date": "2026-10-10", "risk_code": "R-001", "detected_at": "2026-10-08", "owner": "Named engineer", "cause": "Delay", "proposed_action": "Inspect", "status": "Captured"}}, "next_plan_items": []Record{{"title": "Inspect", "owner_role": "Supervisor", "due_date": "2026-10-10", "priority": "normal", "receiver": "External", "progress_percent": 0}}}
	normalized, err := service.validateWorkflowFormAnswers(ctx, Record{"form_code": "builtin_construction_daily_report", "form_type": "daily_report"}, fields, answers, false)
	if err != nil {
		t.Fatal(err)
	}
	blocker := workflowAnswerRecordList(normalized["blockers"])[0]
	if blocker["row_id"] != rowID || blocker["owner_role"] != "Supervisor" || blocker["owner"] != "Named engineer" || blocker["target_date"] != "2026-10-10" {
		t.Fatal("normalized blocker lost native fields", blocker)
	}
	assertCount(t, db, `SELECT count(*) FROM workflow_form_fields field JOIN workflow_form_versions version ON version.id=field.form_version_id WHERE version.template_id=$1 AND version.version_no=1001 AND field.field_key='blockers' AND NOT(field.validation_json->'item_fields' @> '[{"key":"row_id"}]'::jsonb) AND field.validation_json->'item_fields' @> '[{"key":"impact","required":true},{"key":"severity","required":true},{"key":"risk_code","required":false}]'::jsonb`, 1, templateID)
	catalog, err := repo.WorkflowFormCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range catalog {
		if row["template_code"] == "builtin_construction_daily_report" && number(row["version_no"]) == 1001 {
			found = boolValue(row["latest_published"]) && !boolValue(row["default_eligible"])
		}
	}
	if !found {
		t.Fatal("new blocker revision unreachable as explicit catalog selection")
	}
	defaultAfter, err := repo.DefaultWorkflowFormVersionID(ctx, "daily_report", false, "Thi công")
	if err != nil || defaultAfter != compatible {
		t.Fatal("implicit default changed", err)
	}
}
