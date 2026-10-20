package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestPipelineDetailIncludesValidPinnedFormHiddenFromSelectionCatalog(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "pinned_form_detail_", nil)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	row, err := repo.QueryOne(ctx, `
		SELECT v.id AS version_id FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.status='published' AND t.status='active'
		  AND v.schema_json->>'variant'='operational_capture_v3'
		  AND COALESCE(v.schema_json->>'native_capture_revision','')<>'1'
		  AND COALESCE(v.schema_json->>'native_field_capture_revision','')<>'1'
		  AND COALESCE(v.schema_json->>'native_risk_assignment_revision','')<>'1'
		LIMIT 1
	`)
	if err != nil {
		t.Fatal(err)
	}
	versionID := cleanID(row["version_id"])
	template, err := service.CreateTemplate(ctx, Record{"name": "Pinned form detail", "nodes": []any{map[string]any{"name": "Stage", "type": "task", "responsible_role_ids": []string{"PM"}}}}, "")
	if err != nil {
		t.Fatal(err)
	}
	templateID := cleanID(template["id"])
	nodes := template["nodes"].([]Record)
	if _, err = service.CreateTemplateStep(ctx, Record{"node_id": nodes[0]["id"], "name": "Pinned form", "step_type": "task", "required_role_ids": []string{"PM"}, "form_required": true, "form_version_id": versionID}); err != nil {
		t.Fatal(err)
	}
	detail, err := repo.GetTemplateDetail(ctx, templateID)
	if err != nil {
		t.Fatal(err)
	}
	pinned := detail["pinned_workflow_forms"].([]Record)
	if len(pinned) != 1 || cleanID(pinned[0]["version_id"]) != versionID {
		t.Fatalf("pinned forms = %#v, want only %s", pinned, versionID)
	}
	if cleanText(pinned[0]["version_status"]) != "published" || cleanText(pinned[0]["template_status"]) != "active" {
		t.Fatalf("pinned form lost status: %#v", pinned[0])
	}
	mustExec(t, db, `UPDATE workflow_form_templates SET status='archived' WHERE id=(SELECT template_id FROM workflow_form_versions WHERE id=$1)`, versionID)
	detail, err = repo.GetTemplateDetail(ctx, templateID)
	if err != nil {
		t.Fatal(err)
	}
	if archived := detail["pinned_workflow_forms"].([]Record); len(archived) != 0 {
		t.Fatalf("archived form still marked valid: %#v", archived)
	}
}

func TestNativeCaptureCatalogExplicitPipelineSelectionPreservesLegacyPins(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_catalog_", nil)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)
	compatibleID, err := repo.DefaultWorkflowFormVersionID(ctx, "daily_report", false, "Thi công")
	if err != nil || compatibleID == "" {
		t.Fatalf("compatibility default: id=%s err=%v", compatibleID, err)
	}
	permissionID := ensurePermission(t, db, "pipeline.run", "Chạy quy trình", "pipeline")
	pmRoleID := ensureRole(t, db, "pm", "PM")
	mustExec(t, db, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, pmRoleID, permissionID)
	viewPermissionID := ensurePermission(t, db, "operation.view", "Xem thi công", "operation")
	mustExec(t, db, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, pmRoleID, viewPermissionID)

	// Exercise the actual explicit step selection, publication and runtime copy.
	createRun := func(versionID string) (string, string) {
		t.Helper()
		template, createErr := service.CreateTemplate(ctx, Record{"name": "Native catalog pipeline", "nodes": []any{map[string]any{"name": "Thi công", "type": "construction", "responsible_role_ids": []string{"PM"}}}}, fixture.pmID)
		if createErr != nil {
			t.Fatal(createErr)
		}
		templateID := cleanID(template["id"])
		nodes := template["nodes"].([]Record)
		step, createErr := service.CreateTemplateStep(ctx, Record{"node_id": nodes[0]["id"], "name": "Daily report thi công", "step_type": "daily_report", "required_role_ids": []string{"PM"}, "checklist_required": false, "document_required": false, "photo_required": false, "form_required": true, "form_version_id": versionID, "order_index": 1})
		if createErr != nil {
			t.Fatal(createErr)
		}
		if cleanID(step["form_version_id"]) != versionID {
			t.Fatal("explicit selected version replaced by default")
		}
		if _, createErr = service.PublishTemplate(ctx, templateID, fixture.pmID, "PM"); createErr != nil {
			t.Fatal(createErr)
		}
		projectID := newID()
		mustExec(t, db, `INSERT INTO projects(id,name,code,client_name,status,pipeline_template_id) VALUES($1,'Native catalog project',$2,'Client','draft',$3)`, projectID, "NCP-"+projectID[:8], templateID)
		mustExec(t, db, `INSERT INTO project_members(project_id,staff_id,role_ids,active) SELECT $1,id,'["PM"]'::jsonb,true FROM staff WHERE user_id=$2`, projectID, fixture.pmID)
		if _, createErr = service.StartPipeline(ctx, projectID, templateID, fixture.pmID, "PM"); createErr != nil {
			t.Fatal(createErr)
		}
		assertCount(t, db, `SELECT count(*) FROM pipeline_template_steps step JOIN pipeline_template_nodes node ON node.id=step.node_id WHERE node.template_id=$1 AND step.form_version_id=$2`, 1, templateID, versionID)
		assertCount(t, db, `SELECT count(*) FROM pipeline_run_steps WHERE project_id=$1 AND form_version_id=$2`, 1, projectID, versionID)
		assertCount(t, db, `SELECT count(*) FROM workflow_form_instances WHERE project_id=$1 AND form_version_id=$2`, 1, projectID, versionID)
		return templateID, projectID
	}
	legacyTemplateID, legacyProjectID := createRun(compatibleID)
	legacySnapshot := func() string {
		t.Helper()
		row, readErr := repo.QueryOne(ctx, `SELECT jsonb_build_object('template',template,'steps',(SELECT jsonb_agg(step ORDER BY step.id) FROM pipeline_template_steps step JOIN pipeline_template_nodes node ON node.id=step.node_id WHERE node.template_id=template.id),'runs',(SELECT jsonb_agg(run ORDER BY run.id) FROM pipeline_runs run WHERE run.project_id=$2),'run_steps',(SELECT jsonb_agg(step ORDER BY step.id) FROM pipeline_run_steps step WHERE step.project_id=$2),'instances',(SELECT jsonb_agg(instance ORDER BY instance.id) FROM workflow_form_instances instance WHERE instance.project_id=$2)) AS snapshot FROM pipeline_templates template WHERE template.id=$1`, legacyTemplateID, legacyProjectID)
		if readErr != nil {
			t.Fatal(readErr)
		}
		bytes, marshalErr := json.Marshal(row["snapshot"])
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		return string(bytes)
	}
	before := legacySnapshot()
	catalog, err := repo.WorkflowFormCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var latest Record
	nativeCodes := map[string]bool{"builtin_construction_daily_report": false, "client_site_measurement_record": false, "client_survey_work_package": false, "builtin_subcontractor_checklist": false, "client_subcontractor_acceptance_package": false, "client_site_risk_assessment": false}
	nativeFieldKeys := map[string][]string{
		"client_site_measurement_record":          {"survey_unit", "client_representative", "mep"},
		"client_survey_work_package":              {"survey_unit", "client_representative", "issue_code"},
		"builtin_subcontractor_checklist":         {"standard_reference", "design_quantity", "actual_quantity"},
		"client_subcontractor_acceptance_package": {"standard_reference", "design_quantity", "actual_quantity"},
		"client_site_risk_assessment":             {"issue_code", "detected_date", "cause"},
	}
	for _, row := range catalog {
		code := cleanText(row["template_code"])
		if _, expected := nativeCodes[code]; expected && boolValue(row["latest_published"]) {
			nativeCodes[code] = true
			assertCount(t, db, `SELECT count(*) FROM workflow_form_versions WHERE id=$1 AND (schema_json->>'native_capture_revision'='1' OR schema_json->>'native_field_capture_revision'='1' OR schema_json->>'native_risk_assignment_revision'='1')`, 1, cleanID(row["version_id"]))
			if boolValue(row["default_eligible"]) != (code == "builtin_subcontractor_checklist" || code == "client_subcontractor_acceptance_package") {
				t.Fatalf("unexpected native default eligibility for %s", code)
			}
			fields, marshalErr := json.Marshal(row["fields"])
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			for _, key := range nativeFieldKeys[code] {
				if !strings.Contains(string(fields), `"`+key+`"`) {
					t.Fatalf("catalog %s does not expose native field %s", code, key)
				}
			}
		}
		if cleanID(row["version_id"]) == compatibleID && !boolValue(row["default_eligible"]) {
			t.Fatal("compatible catalog version no longer default eligible")
		}
		if cleanText(row["template_code"]) == "builtin_construction_daily_report" && boolValue(row["latest_published"]) {
			latest = row
		}
	}
	for code, present := range nativeCodes {
		if !present {
			t.Fatalf("latest approved native revision missing: %s", code)
		}
	}
	if latest == nil || cleanID(latest["version_id"]) == compatibleID {
		t.Fatal("native daily revision missing from normal catalog")
	}
	latestID := cleanID(latest["version_id"])
	if boolValue(latest["default_eligible"]) {
		t.Fatal("native v3 catalog version unexpectedly eligible as implicit default")
	}
	fieldsJSON, err := json.Marshal(latest["fields"])
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"representative", "material_arrival_items", "receiver"} {
		if !strings.Contains(string(fieldsJSON), `"`+key+`"`) {
			t.Fatalf("catalog field schema does not expose %s", key)
		}
	}
	assertCount(t, db, `SELECT count(*) FROM workflow_form_versions WHERE id=$1 AND schema_json->>'native_capture_revision'='1' AND schema_json ? 'native_risk_assignment_revision'`, 1, latestID)
	assertCount(t, db, `SELECT count(*) FROM workflow_form_fields WHERE form_version_id=$1 AND ((field_key='crew_items' AND validation_json->'item_fields' @> '[{"key":"representative"}]'::jsonb) OR field_key='material_arrival_items' OR (field_key='next_plan_items' AND validation_json->'item_fields' @> '[{"key":"receiver"}]'::jsonb))`, 3, latestID)
	for _, row := range catalog {
		assertCount(t, db, `SELECT count(*) FROM workflow_form_versions WHERE id=$1 AND schema_json->>'variant'='operational_capture_v3' AND COALESCE(schema_json->>'native_capture_revision','')<>'1' AND COALESCE(schema_json->>'native_field_capture_revision','')<>'1' AND COALESCE(schema_json->>'native_risk_assignment_revision','')<>'1'`, 0, cleanID(row["version_id"]))
	}
	assertCount(t, db, `SELECT count(*) FROM (SELECT version.id FROM workflow_form_versions version JOIN workflow_form_templates template ON template.id=version.template_id WHERE template.code='builtin_construction_daily_report' AND version.status='published' AND version.schema_json->>'variant'='operational_capture_v3' AND COALESCE(version.schema_json->>'native_capture_revision','')<>'1' LIMIT 1) unmarked`, 1)
	createRun(latestID)
	defaultAfter, err := repo.DefaultWorkflowFormVersionID(ctx, "daily_report", false, "Thi công")
	if err != nil || defaultAfter != compatibleID {
		t.Fatalf("compatibility default changed: before=%s after=%s err=%v", compatibleID, defaultAfter, err)
	}
	if after := legacySnapshot(); after != before {
		t.Fatal("explicit native selection rewrote existing pins or definition hash")
	}
}
