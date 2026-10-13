package core

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestNativeWorkflowSourceReuseManualClearSurvivesAmendment(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_workflow_reuse_", []string{"workflow_form_instances", "workflow_form_events", "incoming_acceptance_records", "incoming_acceptance_items", "operation_daily_reports"})
	ctx := context.Background()
	f := setupWorkItemFlowFixture(t, db, false)
	s := NewService(NewRepository(db), "test-secret")
	s.ConfigureUploads(t.TempDir(), 12*1024*1024)
	var content bytes.Buffer
	if err := png.Encode(&content, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	stored, _, err := s.storeGeneratedFile(ctx, "evidence/source-reuse.png", content.Bytes(), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	photoID, formID, receiptID := newID(), newID(), newID()
	mustExec(t, db, `INSERT INTO evidence_records(id,project_id,work_item_id,evidence_type,name,status,uploaded_by,storage_provider,storage_key,mime_type,file_size,checksum_sha256) VALUES($1,$2,$3,'photo','source-reuse.png','submitted',$4,$5,$6,'image/png',$7,$8)`, photoID, f.projectID, f.itemID, f.workerID, stored.StorageProvider, stored.StorageKey, content.Len(), sha256Hex(content.Bytes()))
	mustExec(t, db, `INSERT INTO workflow_form_instances(id,project_id,work_item_id,form_template_id,form_version_id,form_type,title,created_by) SELECT $1,$2,$3,t.id,v.id,t.form_type,'Frozen source reuse',$4 FROM workflow_form_templates t JOIN LATERAL(SELECT id FROM workflow_form_versions WHERE template_id=t.id AND status='published' ORDER BY version_no DESC LIMIT 1)v ON true WHERE t.code='builtin_construction_daily_report'`, formID, f.projectID, f.itemID, f.workerID)
	mustExec(t, db, `INSERT INTO incoming_acceptance_records(id,acceptance_code,project_id,work_item_id,status,received_at) VALUES($1::uuid,'workflow-reuse-'||$1::text,$2,$3,'submitted','2026-10-09')`, receiptID, f.projectID, f.itemID)
	mustExec(t, db, `INSERT INTO incoming_acceptance_items(incoming_acceptance_id,project_id,item_index,material_name,unit,delivered_quantity,accepted_quantity) VALUES($1,$2,0,'Receipt sand','m3',10,3)`, receiptID, f.projectID)
	answers := Record{
		"report_date": "2026-10-09", "reported_by": "Worker", "construction_scope": "civil", "weather": "normal",
		"completed_items":        []Record{{"title": "Wall", "result": "done", "evidence_id": photoID}},
		"crew_items":             []Record{{"crew_name": "Crew", "worker_count": 3, "work_hours": 8}},
		"material_arrival_items": []Record{},
		"next_plan_items": []Record{
			{"row_id": newID(), "title": "Default state", "owner_role": "Đội thi công", "due_date": "2026-10-10", "priority": "normal", "state": "", "progress_percent": 0},
			{"row_id": newID(), "title": "Manual state", "owner_role": "Đội thi công", "due_date": "2026-10-10", "priority": "normal", "state": "Observed on site", "progress_percent": 0, "receiver": "Crew representative"},
		},
	}
	submitted, err := s.SubmitWorkflowForm(ctx, formID, WorkflowFormSaveRequest{Answers: answers}, f.workerID, "Worker")
	if err != nil {
		nativeDailyFormFatal(t, "submit", err)
	}
	canonical := recordFromAny(submitted["answers"])
	exportState := func(wantFirst, wantSecond string) {
		t.Helper()
		result, err := s.ExportNativeWorkflowForm(ctx, formID, "next_plan", f.workerID, "Worker")
		if err != nil {
			t.Fatal(err)
		}
		book, err := excelize.OpenReader(bytes.NewReader(result.Data))
		if err != nil {
			t.Fatal(err)
		}
		defer book.Close()
		first, _ := book.GetCellValue(book.GetSheetList()[0], "J8")
		second, _ := book.GetCellValue(book.GetSheetList()[0], "J9")
		if first != wantFirst || second != wantSecond {
			t.Fatalf("export state=(%q,%q), want=(%q,%q)", first, second, wantFirst, wantSecond)
		}
		progress, _ := book.GetCellValue(book.GetSheetList()[0], "I9")
		if progress != "0" {
			t.Fatalf("manual zero lost: %q", progress)
		}
	}
	exportState("Sẵn sàng", "Observed on site")
	result, err := s.ExportNativeWorkflowForm(ctx, formID, "default", f.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Builder{}
	for _, sheet := range book.GetSheetList() {
		rows, _ := book.GetRows(sheet)
		for _, row := range rows {
			all.WriteString(strings.Join(row, " "))
		}
	}
	book.Close()
	if !strings.Contains(all.String(), "Receipt sand") {
		t.Fatal("default-empty arrivals blocked exact receiving source")
	}
	var originalEventID, originalMetadata string
	if err := db.QueryRowContext(ctx, `SELECT id::text,metadata_json::text FROM workflow_form_events WHERE instance_id=$1 AND event_type='submitted' ORDER BY created_at,id LIMIT 1`, formID).Scan(&originalEventID, &originalMetadata); err != nil {
		t.Fatal(err)
	}
	oldEvents := map[string]string{originalEventID: originalMetadata}
	rememberAmendment := func(revision string) {
		t.Helper()
		var id, metadata string
		if err := db.QueryRowContext(ctx, `SELECT id::text,metadata_json::text FROM workflow_form_events WHERE instance_id=$1 AND event_type='amended' AND metadata_json->'native_daily_sources'->>'form_revision_no'=$2`, formID, revision).Scan(&id, &metadata); err != nil {
			t.Fatal(err)
		}
		oldEvents[id] = metadata
	}
	mustExec(t, db, `UPDATE work_items SET status='completed' WHERE source_type='workflow_form_action_item' AND source_id=$1`, formID)
	exportState("Sẵn sàng", "Observed on site")
	rows := workflowAnswerRecordList(canonical["next_plan_items"])
	rows[1]["state"] = "Manual follow-up state"
	canonical["next_plan_items"] = rows
	amended, err := s.AmendWorkflowForm(ctx, formID, WorkflowFormAmendRequest{Answers: canonical, ExpectedRevision: 1, Reason: "Record manual state"}, f.workerID, "Worker")
	if err != nil {
		nativeDailyFormFatal(t, "manual amendment", err)
	}
	exportState("Hoàn thành", "Manual follow-up state")
	rememberAmendment("2")
	canonical = recordFromAny(amended["answers"])
	rows = workflowAnswerRecordList(canonical["next_plan_items"])
	rows[1]["state"] = ""
	canonical["next_plan_items"] = rows
	amended, err = s.AmendWorkflowForm(ctx, formID, WorkflowFormAmendRequest{Answers: canonical, ExpectedRevision: 2, Reason: "Clear manual state"}, f.workerID, "Worker")
	if err != nil {
		nativeDailyFormFatal(t, "clear amendment", err)
	}
	canonical = recordFromAny(amended["answers"])
	if _, exists := workflowAnswerRecordList(canonical["next_plan_items"])[1]["state"]; exists {
		t.Fatal("test must reproduce canonical normalization removing optional blank")
	}
	exportState("Hoàn thành", "")
	var dailyID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM operation_daily_reports WHERE workflow_form_instance_id=$1`, formID).Scan(&dailyID); err != nil {
		t.Fatal(err)
	}
	exportDailyState := func(first, second string) {
		t.Helper()
		result, err := s.ExportNativeDailyBusiness(ctx, dailyID, "next_plan", f.workerID, "Worker")
		if err != nil {
			t.Fatal(err)
		}
		book, err := excelize.OpenReader(bytes.NewReader(result.Data))
		if err != nil {
			t.Fatal(err)
		}
		defer book.Close()
		gotFirst, _ := book.GetCellValue(book.GetSheetList()[0], "J8")
		gotSecond, _ := book.GetCellValue(book.GetSheetList()[0], "J9")
		if gotFirst != first || gotSecond != second {
			t.Fatalf("current daily state=(%q,%q), want=(%q,%q)", gotFirst, gotSecond, first, second)
		}
	}
	exportDailyState("Hoàn thành", "")
	rememberAmendment("3")
	var factsRaw string
	if err := db.QueryRowContext(ctx, `SELECT (metadata_json->'native_daily_sources')::text FROM workflow_form_events WHERE instance_id=$1 AND event_type='amended' AND metadata_json->'native_daily_sources'->>'form_revision_no'='3'`, formID).Scan(&factsRaw); err != nil {
		t.Fatal(err)
	}
	var facts Record
	if err := json.Unmarshal([]byte(factsRaw), &facts); err != nil {
		t.Fatal(err)
	}
	clear := workflowAnswerRecordList(recordFromAny(facts["structured_json"])["next_plan_items"])[1]
	if clear["state"] != "" || cleanText(recordFromAny(clear["state_source"])["source_type"]) != "explicit_clear" {
		t.Fatal("clear provenance missing", clear)
	}
	canonical["weather"] = "hot"
	rows = workflowAnswerRecordList(canonical["next_plan_items"])
	canonical["next_plan_items"] = []Record{rows[1], rows[0]}
	if _, err = s.AmendWorkflowForm(ctx, formID, WorkflowFormAmendRequest{Answers: canonical, ExpectedRevision: 3, Reason: "Update weather only"}, f.workerID, "Worker"); err != nil {
		nativeDailyFormFatal(t, "unrelated amendment", err)
	}
	exportState("", "Hoàn thành")
	exportDailyState("", "Hoàn thành")
	for id, before := range oldEvents {
		var afterMetadata string
		if err := db.QueryRowContext(ctx, `SELECT metadata_json::text FROM workflow_form_events WHERE id=$1`, id).Scan(&afterMetadata); err != nil {
			t.Fatal(err)
		}
		if before != afterMetadata {
			t.Fatal("old submitted/amended event changed", id)
		}
	}
}
