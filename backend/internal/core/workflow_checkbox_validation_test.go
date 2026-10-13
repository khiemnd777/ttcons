package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func checkboxListField(required bool) Record {
	return Record{"field_key": "inspection_items", "label": "Items", "field_type": "structured_list", "validation_json": map[string]any{"item_fields": []any{map[string]any{"key": "title", "label": "Title", "type": "text", "required": true}, map[string]any{"key": "evidence_required", "label": "Evidence", "type": "checkbox", "required": required}}}}
}

func TestWorkflowStructuredCheckboxStrictValidation(t *testing.T) {
	s := &Service{}
	ctx := context.Background()
	field := checkboxListField(false)
	for _, value := range []any{true, false} {
		got, err := s.validateWorkflowFormAnswers(ctx, Record{}, []Record{field}, Record{"inspection_items": []Record{{"title": "Item", "evidence_required": value}}}, true)
		if err != nil {
			t.Errorf("optional %v rejected: %v", value, err)
			continue
		}
		rows, _ := workflowRecordList(got["inspection_items"])
		if rows[0]["evidence_required"] != value {
			t.Errorf("optional %v lost/changed: %#v", value, rows)
		}
	}
	for _, value := range []any{"true", "false", "", 0, 1, Record{}, Record{"value": true}, []any{}, []any{true}} {
		_, err := s.validateWorkflowFormAnswers(ctx, Record{}, []Record{field}, Record{"inspection_items": []Record{{"title": "Item", "evidence_required": value}}}, false)
		if err == nil {
			t.Errorf("invalid checkbox %#v accepted", value)
		}
	}
	for _, value := range []any{nil, false} {
		_, err := s.validateWorkflowFormAnswers(ctx, Record{}, []Record{checkboxListField(true)}, Record{"inspection_items": []Record{{"title": "Item", "evidence_required": value}}}, true)
		if err == nil {
			t.Errorf("required %#v accepted", value)
		}
	}
	for _, row := range []Record{{"title": "Item"}, {"title": "Item", "evidence_required": nil}} {
		got, err := s.validateWorkflowFormAnswers(ctx, Record{}, []Record{field}, Record{"inspection_items": []Record{row}}, true)
		if err != nil {
			t.Fatal(err)
		}
		rows, _ := workflowRecordList(got["inspection_items"])
		if _, exists := rows[0]["evidence_required"]; exists {
			t.Fatal("optional omitted/null checkbox should remain absent")
		}
	}
	if _, err := s.validateWorkflowFormAnswers(ctx, Record{}, []Record{checkboxListField(true)}, Record{"inspection_items": []Record{{"title": "Item"}}}, true); err == nil {
		t.Fatal("required omitted checkbox accepted")
	}
	if _, err := s.validateWorkflowFormAnswers(ctx, Record{}, []Record{checkboxListField(true)}, Record{"inspection_items": []Record{{"title": "Item", "evidence_required": true}}}, true); err != nil {
		t.Fatal("required true rejected", err)
	}
}

func TestWorkflowTopLevelCheckboxStrictValidation(t *testing.T) {
	s := &Service{}
	ctx := context.Background()
	field := Record{"field_key": "confirmed", "field_type": "checkbox", "label": "Confirmed", "required": false}
	for _, submit := range []bool{false, true} {
		for _, value := range []any{true, false, nil, "", "true", "false", 0, 1, Record{}, Record{"value": true}, []any{}, []any{true}} {
			got, err := s.validateWorkflowFormAnswers(ctx, Record{}, []Record{field}, Record{"confirmed": value}, submit)
			_, isBool := value.(bool)
			if value == nil || isBool {
				if err != nil {
					t.Fatalf("valid top checkbox %#v rejected %v", value, err)
				}
				if value == true && got["confirmed"] != true {
					t.Fatal("true lost")
				}
				if value != true {
					if _, exists := got["confirmed"]; exists {
						t.Fatal("legacy false/nil optional behavior changed")
					}
				}
			} else if err == nil {
				t.Fatalf("nonbool top %#v accepted", value)
			}
		}
	}
	field["required"] = true
	for _, value := range []any{false, nil} {
		if _, err := s.validateWorkflowFormAnswers(ctx, Record{}, []Record{field}, Record{"confirmed": value}, true); err == nil {
			t.Fatalf("required top %#v accepted", value)
		}
	}
}

func TestWorkflowHandoffCheckboxDefaultsKeepInvalidTypes(t *testing.T) {
	s := &Service{}
	ctx := context.Background()
	field := checkboxListField(false)
	instance := Record{"form_code": clientConstructionHandoffPlanFormCode}
	for _, value := range []any{nil, false, true, "false", 0, Record{}, []any{}} {
		answers := Record{"inspection_items": []Record{{"title": "Item", "evidence_required": value}}}
		before, _ := json.Marshal(answers)
		got, err := s.validateWorkflowFormAnswers(ctx, instance, []Record{field}, answers, false)
		_, validBool := value.(bool)
		if value == nil || validBool {
			if err != nil {
				t.Errorf("handoff %#v rejected %v", value, err)
				continue
			}
			rows, _ := workflowRecordList(got["inspection_items"])
			if rows[0]["evidence_required"] != true {
				t.Errorf("handoff policy not forced %#v", rows)
			}
		} else if err == nil {
			t.Errorf("invalid handoff %#v coerced", value)
		}
		after, _ := json.Marshal(answers)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("input source mutated")
		}
	}
	omitted := Record{"inspection_items": []Record{{"title": "Item"}}}
	got, err := s.validateWorkflowFormAnswers(ctx, instance, []Record{field}, omitted, true)
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := workflowRecordList(got["inspection_items"])
	if rows[0]["evidence_required"] != true {
		t.Fatal("omitted handoff checkbox not forced true")
	}
	original, _ := workflowRecordList(omitted["inspection_items"])
	if _, exists := original[0]["evidence_required"]; exists {
		t.Fatal("omitted handoff input mutated")
	}
}

func TestWorkflowHandoffCheckboxSaveSubmitDB(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "handoff_checkbox_", []string{"workflow_form_instances", "workflow_form_answers", "work_items"})
	f := setupWorkItemFlowFixture(t, db, false)
	bindWorkItemDomainExecutionContext(t, db, f, "planning", "construction_planning", "workflow_form", "construction.activate_handoff")
	formID := newID()
	mustExec(t, db, `INSERT INTO workflow_form_instances(id,project_id,work_item_id,pipeline_run_id,run_node_id,run_step_id,form_template_id,form_version_id,form_type,title,status,current_revision,created_by) SELECT $1,wi.project_id,wi.id,wi.pipeline_run_id,wi.run_node_id,wi.run_step_id,version.template_id,version.id,template.form_type,'Handoff checkbox test','draft',0,$2 FROM work_items wi CROSS JOIN workflow_form_versions version JOIN workflow_form_templates template ON template.id=version.template_id WHERE wi.id=$3 AND template.code='client_construction_handoff_plan' AND version.status='published' ORDER BY version.version_no DESC LIMIT 1`, formID, f.workerID, f.itemID)
	s := NewService(NewRepository(db), "test-secret")
	ctx := context.Background()
	answers := Record{"handoff_title": "Test handoff", "inspection_items": []Record{{"title": "Inspect wall", "criteria": "Approved drawing", "responsible_role": "QA/QC", "evidence_required": true}}}
	if _, err := s.SaveWorkflowFormDraft(ctx, formID, WorkflowFormSaveRequest{Answers: answers}, f.workerID, "Worker"); err != nil {
		t.Fatalf("draft checkbox save: %#v", err)
	}
	var sourceHash string
	if err := db.QueryRow(`SELECT md5(answers_snapshot_json::text) FROM workflow_form_instances WHERE id=$1`, formID).Scan(&sourceHash); err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{"true", "false", "", 0, 1, Record{}, []any{}} {
		invalid := Record{"handoff_title": "Test handoff", "inspection_items": []Record{{"title": "Inspect wall", "criteria": "Approved drawing", "responsible_role": "QA/QC", "evidence_required": value}}}
		if _, err := s.SaveWorkflowFormDraft(ctx, formID, WorkflowFormSaveRequest{Answers: invalid}, f.workerID, "Worker"); err == nil {
			t.Fatalf("invalidhandoff %#v saved", value)
		}
		if _, err := s.SubmitWorkflowForm(ctx, formID, WorkflowFormSaveRequest{Answers: invalid}, f.workerID, "Worker"); err == nil {
			t.Fatalf("invalidhandoff %#v submitted", value)
		}
		var same bool
		if err := db.QueryRow(`SELECT md5(answers_snapshot_json::text)=$2 FROM workflow_form_instances WHERE id=$1`, formID, sourceHash).Scan(&same); err != nil || !same {
			t.Fatal("failedvalidation mutated source")
		}
	}
	if _, err := s.SubmitWorkflowForm(ctx, formID, WorkflowFormSaveRequest{Answers: answers}, f.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT count(*) FROM workflow_form_instances WHERE id=$1 AND status='submitted' AND answers_snapshot_json->'inspection_items'->0->'evidence_required'='true'::jsonb`, 1, formID)
}

func genericCheckboxFormFixture(t *testing.T, db *sql.DB, required bool) (*Service, workItemFlowFixture, string) {
	t.Helper()
	f := setupWorkItemFlowFixture(t, db, false)
	formID, templateID, versionID := newID(), newID(), newID()
	mustExec(t, db, `INSERT INTO workflow_form_templates(id,code,name,form_type) VALUES($1,$2,'Isolated boolean schema','generic_submission')`, templateID, "checkbox_test_"+templateID)
	mustExec(t, db, `INSERT INTO workflow_form_versions(id,template_id,version_no,status) VALUES($1,$2,1,'draft')`, versionID, templateID)
	schema, _ := json.Marshal(checkboxListField(required)["validation_json"])
	mustExec(t, db, `INSERT INTO workflow_form_fields(form_version_id,field_key,label,field_type,required,validation_json) VALUES($1,'confirmed','Confirmed','checkbox',$2,'{}'),($1,'inspection_items','Items','structured_list',true,$3::jsonb)`, versionID, required, string(schema))
	mustExec(t, db, `UPDATE workflow_form_versions SET status='published',published_at=now() WHERE id=$1`, versionID)
	mustExec(t, db, `INSERT INTO workflow_form_instances(id,project_id,work_item_id,form_template_id,form_version_id,form_type,title,status,created_by) VALUES($1,$2,$3,$4,$5,'generic_submission','Checkbox persistence','draft',$6)`, formID, f.projectID, f.itemID, templateID, versionID, f.workerID)
	return NewService(NewRepository(db), "test-secret"), f, formID
}

func TestWorkflowCheckboxActualPersistenceDB(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "checkbox_persist_", []string{"workflow_form_instances", "workflow_form_answers", "work_items"})
	ctx := context.Background()
	for _, required := range []bool{false, true} {
		t.Run(fmt.Sprintf("required_%v", required), func(t *testing.T) {
			s, f, formID := genericCheckboxFormFixture(t, db, required)
			for _, value := range []any{true, false, nil, "true", "false", "", 0, 1, Record{}, []any{}} {
				answers := Record{"confirmed": value, "inspection_items": []Record{{"title": "Item", "evidence_required": value}}}
				_, isBool := value.(bool)
				valid := isBool || value == nil
				_, err := s.SaveWorkflowFormDraft(ctx, formID, WorkflowFormSaveRequest{Answers: answers}, f.workerID, "Worker")
				if required && value != true {
					valid = false
				}
				if valid && err != nil {
					t.Fatalf("draft %#v error%v", value, err)
				}
				if !valid && err == nil {
					t.Fatalf("invaliddraft %#v accepted", value)
				}
				if !valid {
					if _, err = s.SubmitWorkflowForm(ctx, formID, WorkflowFormSaveRequest{Answers: answers}, f.workerID, "Worker"); err == nil {
						t.Fatalf("invalidsubmit %#v accepted", value)
					}
				}
				if !required && isBool {
					var persisted bool
					if err = db.QueryRow(`SELECT (answers_snapshot_json->'inspection_items'->0->>'evidence_required')::boolean FROM workflow_form_instances WHERE id=$1`, formID).Scan(&persisted); err != nil || persisted != value {
						t.Fatalf("typed boolean not retained %#v %v", value, err)
					}
				}
				if !required && value == nil {
					assertCount(t, db, `SELECT count(*) FROM workflow_form_instances WHERE id=$1 AND NOT (answers_snapshot_json->'inspection_items'->0 ? 'evidence_required')`, 1, formID)
				}
			}
			answers := Record{"confirmed": required, "inspection_items": []Record{{"title": "Item", "evidence_required": required}}}
			if _, err := s.SubmitWorkflowForm(ctx, formID, WorkflowFormSaveRequest{Answers: answers}, f.workerID, "Worker"); err != nil {
				t.Fatal(err)
			}
			var persisted bool
			if err := db.QueryRow(`SELECT (answers_snapshot_json->'inspection_items'->0->>'evidence_required')::boolean FROM workflow_form_instances WHERE id=$1`, formID).Scan(&persisted); err != nil || persisted != required {
				t.Fatal("submitted typed boolean not retained", err)
			}
		})
	}
}
