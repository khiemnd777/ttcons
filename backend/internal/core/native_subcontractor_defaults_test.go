package core

import (
	"context"
	"encoding/json"
	"testing"
)

func TestNativeSubcontractorAssessmentDefaults(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_bm03_defaults_", []string{"workflow_form_versions", "workflow_form_fields", "pipeline_template_steps"})
	ctx := context.Background()
	repo := NewRepository(db)
	catalog, err := repo.WorkflowFormCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"builtin_subcontractor_checklist", "client_subcontractor_acceptance_package"} {
		row, err := repo.QueryOne(ctx, `SELECT v.id FROM workflow_form_versions v JOIN workflow_form_templates t ON t.id=v.template_id WHERE t.code=$1 AND v.status='published' AND v.schema_json->>'native_subcontractor_assessment_revision'='1' ORDER BY v.version_no DESC LIMIT 1`, code)
		if err != nil {
			t.Fatal(err)
		}
		id := cleanID(row["id"])
		found := false
		for _, form := range catalog {
			if cleanID(form["version_id"]) == id {
				found = true
				if !boolValue(form["default_eligible"]) || !boolValue(form["latest_published"]) {
					t.Fatalf("complete assessment %s must be latest and default eligible", code)
				}
			}
		}
		if !found {
			t.Fatalf("assessment %s absent from catalog", code)
		}
		if code == "builtin_subcontractor_checklist" {
			got, err := repo.DefaultWorkflowFormVersionID(ctx, "checklist", true, "Checklist thầu phụ")
			if err != nil || got != id {
				t.Fatalf("automatic BM03 default=%s, want %s: %v", got, id, err)
			}
		}
	}
}

func TestNativeSubcontractorDefaultsRejectIncompleteProfilesAndPreservePins(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_bm03_pins_", []string{"workflow_form_versions", "workflow_form_fields", "workflow_form_instances", "pipeline_run_steps"})
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)
	pmRole := ensureRole(t, db, "pm", "PM")
	for _, code := range []string{"pipeline.run", "operation.view"} {
		permission := ensurePermission(t, db, code, code, "pipeline")
		mustExec(t, db, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, pmRole, permission)
	}
	completeID, err := repo.DefaultWorkflowFormVersionID(ctx, "checklist", true, "Checklist thầu phụ")
	if err != nil || completeID == "" {
		t.Fatalf("assessment default: %s %v", completeID, err)
	}
	legacy, err := repo.QueryOne(ctx, `SELECT v.id FROM workflow_form_versions v JOIN workflow_form_templates t ON t.id=v.template_id WHERE t.code='builtin_subcontractor_checklist' AND v.status='published' AND COALESCE(v.schema_json->>'variant','')<>'operational_capture_v3' ORDER BY v.version_no DESC LIMIT 1`)
	if err != nil {
		t.Fatal(err)
	}
	createRun := func(explicitID, expectedID string) string {
		t.Helper()
		template, err := service.CreateTemplate(ctx, Record{"name": "BM03 default regression", "nodes": []any{map[string]any{"name": "Thi công", "type": "construction", "responsible_role_ids": []string{"PM"}}}}, fixture.pmID)
		if err != nil {
			t.Fatal(err)
		}
		templateID := cleanID(template["id"])
		nodes := template["nodes"].([]Record)
		body := Record{"node_id": nodes[0]["id"], "name": "Checklist thầu phụ", "step_type": "checklist", "required_role_ids": []string{"PM"}, "checklist_required": true, "document_required": false, "photo_required": false, "form_required": true, "order_index": 1}
		if explicitID != "" {
			body["form_version_id"] = explicitID
		}
		step, err := service.CreateTemplateStep(ctx, body)
		if err != nil || cleanID(step["form_version_id"]) != expectedID {
			t.Fatalf("step form=%v want=%s err=%v", step, expectedID, err)
		}
		// Updating unrelated authoring fields must retain the chosen version.
		if _, err = service.UpdateTemplateStep(ctx, cleanID(step["id"]), Record{"name": "Checklist thầu phụ updated"}); err != nil {
			t.Fatal(err)
		}
		if _, err = service.PublishTemplate(ctx, templateID, fixture.pmID, "PM"); err != nil {
			t.Fatal(err)
		}
		projectID := newID()
		mustExec(t, db, `INSERT INTO projects(id,name,code,client_name,status,pipeline_template_id) VALUES($1,'BM03 regression',$2,'Client','draft',$3)`, projectID, "BM03-"+projectID[:8], templateID)
		mustExec(t, db, `INSERT INTO project_members(project_id,staff_id,role_ids,active) SELECT $1,id,'["PM"]'::jsonb,true FROM staff WHERE user_id=$2`, projectID, fixture.pmID)
		if _, err = service.StartPipeline(ctx, projectID, templateID, fixture.pmID, "PM"); err != nil {
			t.Fatal(err)
		}
		assertCount(t, db, `SELECT count(*) FROM pipeline_template_steps step JOIN pipeline_template_nodes node ON node.id=step.node_id WHERE node.template_id=$1 AND step.form_version_id=$2`, 1, templateID, expectedID)
		assertCount(t, db, `SELECT count(*) FROM pipeline_run_steps WHERE project_id=$1 AND form_version_id=$2`, 1, projectID, expectedID)
		assertCount(t, db, `SELECT count(*) FROM workflow_form_instances WHERE project_id=$1 AND form_version_id=$2`, 1, projectID, expectedID)
		return projectID
	}
	legacyProject := createRun(cleanID(legacy["id"]), cleanID(legacy["id"]))
	snapshot := func() string {
		t.Helper()
		row, err := repo.QueryOne(ctx, `SELECT jsonb_build_object('template',t,'steps',(SELECT jsonb_agg(s ORDER BY s.id) FROM pipeline_template_steps s JOIN pipeline_template_nodes n ON n.id=s.node_id WHERE n.template_id=t.id),'runs',(SELECT jsonb_agg(r ORDER BY r.id) FROM pipeline_runs r WHERE r.project_id=p.id),'run_steps',(SELECT jsonb_agg(s ORDER BY s.id) FROM pipeline_run_steps s WHERE s.project_id=p.id),'instances',(SELECT jsonb_agg(i ORDER BY i.id) FROM workflow_form_instances i WHERE i.project_id=p.id)) AS snapshot FROM projects p JOIN pipeline_templates t ON t.id=p.pipeline_template_id WHERE p.id=$1`, legacyProject)
		if err != nil {
			t.Fatal(err)
		}
		value, err := json.Marshal(row["snapshot"])
		if err != nil {
			t.Fatal(err)
		}
		return string(value)
	}
	before := snapshot()
	createRun("", completeID)
	dailyBefore, err := repo.DefaultWorkflowFormVersionID(ctx, "daily_report", false, "Thi công")
	if err != nil {
		t.Fatal(err)
	}
	cloneDraft := func(sourceID string) string {
		t.Helper()
		id := newID()
		mustExec(t, db, `INSERT INTO workflow_form_versions(id,template_id,version_no,status,schema_json,validation_json) SELECT $1,v.template_id,(SELECT max(version_no)+1 FROM workflow_form_versions WHERE template_id=v.template_id),'draft',v.schema_json,v.validation_json FROM workflow_form_versions v WHERE v.id=$2`, id, sourceID)
		mustExec(t, db, `INSERT INTO workflow_form_fields(form_version_id,field_key,label,field_type,required,options_json,validation_json,help_text,order_index,visibility_condition_json) SELECT $1,field_key,label,field_type,required,options_json,validation_json,help_text,order_index,visibility_condition_json FROM workflow_form_fields WHERE form_version_id=$2`, id, sourceID)
		return id
	}
	for _, tc := range []struct{ name, sql string }{
		{"missing assessment marker", `UPDATE workflow_form_versions SET schema_json=schema_json-'native_subcontractor_assessment_revision' WHERE id=$1`},
		{"missing field marker", `UPDATE workflow_form_versions SET schema_json=schema_json-'native_field_capture_revision' WHERE id=$1`},
		{"missing reference", `DELETE FROM workflow_form_fields WHERE form_version_id=$1 AND field_key='reference_no'`},
		{"wrong conclusion type", `UPDATE workflow_form_fields SET field_type='text' WHERE form_version_id=$1 AND field_key='acceptance_conclusion'`},
		{"missing conclusion option", `UPDATE workflow_form_fields SET options_json='[{"value":"accepted"}]'::jsonb WHERE form_version_id=$1 AND field_key='acceptance_conclusion'`},
		{"missing quality note", `UPDATE workflow_form_fields SET validation_json=jsonb_set(validation_json,'{item_fields}',(SELECT jsonb_agg(item) FROM jsonb_array_elements(validation_json->'item_fields') item WHERE item->>'key'<>'quality_note')) WHERE form_version_id=$1 AND field_key='checklist_items'`},
		{"missing quality options", `UPDATE workflow_form_fields SET validation_json=jsonb_set(validation_json,'{item_fields}',(SELECT jsonb_agg(CASE WHEN item->>'key'='quality_result' THEN item||'{"options":[{"value":"pass"}]}'::jsonb ELSE item END) FROM jsonb_array_elements(validation_json->'item_fields') item)) WHERE form_version_id=$1 AND field_key='checklist_items'`},
		{"missing measurements", `UPDATE workflow_form_fields SET validation_json=jsonb_set(validation_json,'{item_fields}',(SELECT jsonb_agg(item) FROM jsonb_array_elements(validation_json->'item_fields') item WHERE item->>'key'<>'actual_quantity')) WHERE form_version_id=$1 AND field_key='checklist_items'`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := cloneDraft(completeID)
			mustExec(t, db, tc.sql, id)
			mustExec(t, db, `UPDATE workflow_form_versions SET status='published',published_at=now() WHERE id=$1`, id)
			got, err := repo.DefaultWorkflowFormVersionID(ctx, "checklist", true, "thầu phụ")
			if err != nil || got != completeID {
				t.Fatalf("incomplete version selected: %s want=%s err=%v", got, completeID, err)
			}
			catalog, err := repo.WorkflowFormCatalog(ctx)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, form := range catalog {
				if cleanID(form["version_id"]) == id {
					found = true
					if boolValue(form["default_eligible"]) {
						t.Fatal("incomplete catalog version default eligible")
					}
				}
			}
			// Without field marker the native definition is outside catalog admission.
			if !found && tc.name != "missing field marker" {
				t.Fatal("explicit native definition disappeared")
			}
		})
	}
	// Copying every assessment field and marker to an unrelated template must
	// not opt that template into automatic selection.
	unrelatedTemplate := newID()
	mustExec(t, db, `INSERT INTO workflow_form_templates(id,code,name,form_type,status) VALUES($1,'unrelated_assessment_profile','Unrelated profile','checklist','active')`, unrelatedTemplate)
	unrelatedVersion := cloneDraft(completeID)
	mustExec(t, db, `UPDATE workflow_form_versions SET template_id=$2,status='published',published_at=now() WHERE id=$1`, unrelatedVersion, unrelatedTemplate)
	catalog, err := repo.WorkflowFormCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	unrelatedFound := false
	for _, form := range catalog {
		if cleanID(form["version_id"]) == unrelatedVersion {
			unrelatedFound = true
			if boolValue(form["default_eligible"]) {
				t.Fatal("assessment markers admitted unrelated template")
			}
		}
	}
	if !unrelatedFound {
		t.Fatal("unrelated explicit native version disappeared")
	}
	// A higher, complete published revision wins; a still-higher draft does not.
	newComplete := cloneDraft(completeID)
	mustExec(t, db, `UPDATE workflow_form_versions SET status='published',published_at=now() WHERE id=$1`, newComplete)
	cloneDraft(completeID)
	got, err := repo.DefaultWorkflowFormVersionID(ctx, "checklist", true, "thầu phụ")
	if err != nil || got != newComplete {
		t.Fatalf("newest eligible published default=%s want=%s err=%v", got, newComplete, err)
	}
	createRun("", newComplete)
	dailyAfter, err := repo.DefaultWorkflowFormVersionID(ctx, "daily_report", false, "Thi công")
	if err != nil || dailyAfter != dailyBefore {
		t.Fatalf("daily default changed: %s -> %s, err=%v", dailyBefore, dailyAfter, err)
	}
	if after := snapshot(); after != before {
		t.Fatal("new defaults changed an existing template, runtime or form snapshot")
	}
}
