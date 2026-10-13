package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestNativeSubcontractorAssessmentRevisionAndCapture(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_subcontractor_", []string{"workflow_form_versions", "workflow_form_fields", "workflow_form_instances"})
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	for _, code := range []string{"builtin_subcontractor_checklist", "client_subcontractor_acceptance_package"} {
		latest, err := repo.QueryOne(ctx, `SELECT v.* FROM workflow_form_versions v JOIN workflow_form_templates t ON t.id=v.template_id WHERE t.code=$1 AND v.status='published' ORDER BY v.version_no DESC LIMIT 1`, code)
		if err != nil {
			t.Fatal(err)
		}
		assertCount(t, db, `SELECT count(*) FROM workflow_form_fields WHERE form_version_id=$1 AND field_key IN ('reference_no','acceptance_conclusion') AND NOT required`, 2, latest["id"])
		assertCount(t, db, `SELECT count(*) FROM workflow_form_fields conclusion WHERE conclusion.form_version_id=$1 AND conclusion.field_key='acceptance_conclusion' AND conclusion.order_index>(SELECT max(other.order_index) FROM workflow_form_fields other WHERE other.form_version_id=$1 AND other.field_key<>'acceptance_conclusion')`, 1, latest["id"])
		assertCount(t, db, `SELECT count(*) FROM workflow_form_fields reference WHERE reference.form_version_id=$1 AND reference.field_key='reference_no' AND reference.order_index<(SELECT min(other.order_index) FROM workflow_form_fields other WHERE other.form_version_id=$1 AND other.field_key<>'reference_no')`, 1, latest["id"])
		assertCount(t, db, `SELECT count(*) FROM workflow_form_fields WHERE form_version_id=$1 AND field_key='acceptance_conclusion' AND label='Kết luận nghiệm thu' AND options_json @> '[{"value":"accepted","label":"Đồng ý nghiệm thu"},{"value":"rework_required","label":"Cần sửa chữa tồn tại"}]'::jsonb`, 1, latest["id"])
		assertCount(t, db, `SELECT count(*) FROM workflow_form_fields WHERE form_version_id=$1 AND field_key='checklist_items' AND validation_json->'item_fields' @> '[{"key":"quality_result","label":"Kết quả"},{"key":"quality_note","label":"Nhận xét nghiệm thu"}]'::jsonb`, 1, latest["id"])
		assertCount(t, db, `SELECT count(*) FROM workflow_form_fields f JOIN workflow_form_versions v ON v.id=f.form_version_id WHERE v.template_id=$1 AND v.version_no<$2 AND f.field_key IN ('reference_no','acceptance_conclusion')`, 0, latest["template_id"], latest["version_no"])
		fields, err := repo.Query(ctx, `SELECT * FROM workflow_form_fields WHERE form_version_id=$1 ORDER BY order_index`, latest["id"])
		if err != nil {
			t.Fatal(err)
		}
		instance := Record{"form_code": code, "form_type": "checklist"}
		answers := Record{"reference_no": "BM03-01", "acceptance_conclusion": "rework_required", "checklist_items": []any{map[string]any{"title": "Wall finish", "status": "done", "quality_result": "fail", "quality_note": "Surface uneven"}}}
		normalized, err := service.validateWorkflowFormAnswers(ctx, instance, fields, answers, false)
		if err != nil {
			t.Fatalf("%s valid assessment: %v", code, err)
		}
		for _, invalid := range []Record{{"reference_no": strings.Repeat("x", 161)}, {"acceptance_conclusion": "pass"}, {"checklist_items": []any{map[string]any{"title": "Wall finish", "status": "done", "quality_result": "accepted"}}}, {"checklist_items": []any{map[string]any{"title": "Wall finish", "status": "done", "quality_note": strings.Repeat("x", 1001)}}}} {
			if _, err := service.validateWorkflowFormAnswers(ctx, instance, fields, invalid, false); err == nil {
				t.Fatalf("%s accepted invalid %#v", code, invalid)
			}
		}
		fixture := setupWorkItemFlowFixture(t, db, false)
		id, oldID := newID(), newID()
		mustExec(t, db, `INSERT INTO workflow_form_instances(id,project_id,work_item_id,form_template_id,form_version_id,form_type,title,created_by) SELECT $1,$2,$3,template_id,id,'checklist','Native assessment',$4 FROM workflow_form_versions WHERE id=$5`, id, fixture.projectID, fixture.itemID, fixture.workerID, latest["id"])
		// Historical instance keeps its old version and answers while current capture saves.
		mustExec(t, db, `INSERT INTO workflow_form_instances(id,project_id,form_template_id,form_version_id,form_type,title,answers_snapshot_json,created_by) SELECT $1,$2,template_id,id,'checklist','Historical assessment','{"legacy":"keep"}'::jsonb,$3 FROM workflow_form_versions WHERE template_id=$4 AND version_no=1`, oldID, fixture.projectID, fixture.workerID, latest["template_id"])
		if _, err = repo.SaveWorkflowForm(ctx, id, normalized, "", false, fixture.workerID, "Worker", nil); err != nil {
			t.Fatal(err)
		}
		assertCount(t, db, `SELECT count(*) FROM workflow_form_instances WHERE id=$1 AND answers_snapshot_json->>'reference_no'='BM03-01' AND answers_snapshot_json->>'acceptance_conclusion'='rework_required' AND answers_snapshot_json->'checklist_items'->0->>'quality_result'='fail'`, 1, id)
		assertCount(t, db, `SELECT count(*) FROM workflow_form_instances i JOIN workflow_form_versions v ON v.id=i.form_version_id WHERE i.id=$1 AND v.version_no=1 AND i.answers_snapshot_json='{"legacy":"keep"}'::jsonb`, 1, oldID)
		bytes, _ := json.Marshal(normalized)
		if !strings.Contains(string(bytes), `"quality_note":"Surface uneven"`) {
			t.Fatal("quality note lost")
		}
	}
}

func TestNativeWorkflowExportSourceKeepsPinnedSelectOptions(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_pinned_options_", []string{"workflow_form_fields", "workflow_form_instances"})
	fixture := setupWorkItemFlowFixture(t, db, false)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	id := newID()
	mustExec(t, db, `INSERT INTO workflow_form_instances(id,project_id,work_item_id,form_template_id,form_version_id,form_type,title,answers_snapshot_json,created_by) SELECT $1,$2,$3,t.id,v.id,t.form_type,'Pinned subcontractor','{"work_package":"flooring"}'::jsonb,$4 FROM workflow_form_templates t JOIN workflow_form_versions v ON v.template_id=t.id AND v.version_no=1 WHERE t.code='builtin_subcontractor_checklist'`, id, fixture.projectID, fixture.itemID, fixture.workerID)
	source, err := service.nativeWorkflowExportSource(ctx, id, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	fields, ok := source["fields"].([]Record)
	if !ok {
		t.Fatalf("source does not expose pinned fields: %T", source["fields"])
	}
	var workPackage Record
	for _, field := range fields {
		if cleanText(field["field_key"]) == "work_package" {
			workPackage = field
		}
	}
	if workPackage == nil {
		t.Fatal("pinned work_package absent")
	}
	options := workflowAnswerRecordList(workPackage["options_json"])
	label := ""
	for _, option := range options {
		if cleanText(option["value"]) == "flooring" {
			label = cleanText(option["label"])
		}
	}
	if label != "Sàn" {
		t.Fatalf("pinned flooring label=%q", label)
	}
	for _, field := range fields {
		if statusIn(cleanText(field["field_key"]), "reference_no", "acceptance_conclusion") {
			t.Fatal("export fields came from latest revision instead of pinned version1")
		}
	}
	assertCount(t, db, `SELECT count(*) FROM workflow_form_instances i JOIN workflow_form_versions v ON v.id=i.form_version_id WHERE i.id=$1 AND v.version_no=1 AND i.answers_snapshot_json->>'work_package'='flooring'`, 1, id)
}
