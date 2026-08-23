package core

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestGenerateWorkflowDefinitionWorkbookIncludesPortableSheets(t *testing.T) {
	catalog := workflowDefinitionCatalog{Fingerprint: strings.Repeat("a", 64), Permissions: []Record{}, Roles: []Record{}}
	result, err := generateWorkflowDefinitionWorkbook(workflowDefinitionPackage{Scope: "bundle"}, catalog, time.Date(2026, 8, 23, 8, 0, 0, 0, time.UTC), "mau")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) < 4 || !bytes.Equal(result.Data[:2], []byte("PK")) {
		t.Fatalf("unexpected workbook bytes=%d", len(result.Data))
	}
	files := unzipXLSXFiles(t, result.Data)
	workbook := files["xl/workbook.xml"]
	for _, sheet := range []string{
		"Huong_dan", "Manifest", "Pipelines", "Pipeline_Nodes", "Pipeline_Steps", "Pipeline_Edges",
		"Form_Templates", "Form_Versions", "Form_Fields", "Ref_Permissions", "Ref_Roles", "Ref_Edge_Conditions",
	} {
		if !strings.Contains(workbook, `name="`+sheet+`"`) {
			t.Fatalf("workflow definition workbook missing sheet %s", sheet)
		}
	}
}

func TestWorkflowFormDefinitionHashIgnoresPersistenceIdentity(t *testing.T) {
	base := []Record{{
		"id": "source-field", "form_version_id": "source-version", "field_key": "summary", "label": "Tóm tắt",
		"field_type": "text", "required": true, "options_json": []any{}, "validation_json": Record{"max_length": 200},
		"help_text": "", "order_index": 10, "visibility_condition_json": Record{}, "created_at": "yesterday",
	}}
	other := []Record{{
		"id": "target-field", "form_version_id": "target-version", "field_key": "summary", "label": "Tóm tắt",
		"field_type": "text", "required": true, "options_json": []any{}, "validation_json": Record{"max_length": 200},
		"help_text": "", "order_index": 10, "visibility_condition_json": Record{}, "created_at": "today",
	}}
	left, err := workflowFormDefinitionHash(Record{"variant": "portable"}, Record{}, base)
	if err != nil {
		t.Fatal(err)
	}
	right, err := workflowFormDefinitionHash(Record{"variant": "portable"}, Record{}, other)
	if err != nil {
		t.Fatal(err)
	}
	if left != right || len(left) != 64 {
		t.Fatalf("portable form hashes differ left=%q right=%q", left, right)
	}
}

func TestWorkflowDefinitionEdgeFieldMustBelongToSourceForm(t *testing.T) {
	nodes := []Record{
		{"node_key": "source", "steps": []Record{{"form_ref": "form:test"}}},
		{"node_key": "target", "steps": []Record{}},
	}
	forms := map[string]Record{
		"form:test": {"fields": []Record{{"field_key": "decision"}}},
	}
	validRows := []Record{{
		"edge_key": "edge:valid", "from_node_key": "source", "to_node_key": "target",
		"condition_type": "workflow_form_answer_equals", "condition_config_json": `{"field_key":"decision","value":"yes"}`, "priority": 100,
	}}
	blockers := []Record{}
	edges := normalizeWorkflowDefinitionEdges("pipeline:test", validRows, nodes, workflowDefinitionCatalog{}, forms, &blockers)
	if len(edges) != 1 || len(blockers) != 0 {
		t.Fatalf("expected source form field to resolve edges=%#v blockers=%#v", edges, blockers)
	}

	invalidRows := []Record{{
		"edge_key": "edge:invalid", "from_node_key": "source", "to_node_key": "target",
		"condition_type": "workflow_form_answer_equals", "condition_config_json": `{"field_key":"missing","value":"yes"}`, "priority": 100,
	}}
	blockers = []Record{}
	edges = normalizeWorkflowDefinitionEdges("pipeline:test", invalidRows, nodes, workflowDefinitionCatalog{}, forms, &blockers)
	if len(edges) != 0 || len(blockers) != 1 || workflowDefinitionText(blockers[0]["code"]) != "EDGE_FORM_FIELD_UNRESOLVED" {
		t.Fatalf("expected unresolved field blocker edges=%#v blockers=%#v", edges, blockers)
	}
}

func TestWorkflowDefinitionJSONImportCreatesDraftOnlyAndIsIdempotent(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "workflow-definition-test-secret")
	suffix := strings.ReplaceAll(newID(), "-", "")[:10]
	actorID := newID()
	actorEmail := "workflow-definition-actor-" + suffix + "@test.local"
	formCode := "portable_form_" + suffix
	idempotencyKey := newID()

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM workflow_definition_imports WHERE actor_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE actor_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM pipeline_templates WHERE created_by=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM workflow_form_templates WHERE code=$1`, formCode)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})

	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:test','Workflow Definition Actor','active')`, actorID, actorEmail)
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) SELECT $1,id FROM roles WHERE name='admin'`, actorID)

	request := WorkflowDefinitionImportPreviewRequest{
		Scope: "bundle", PackageVersion: dataPackageVersionV1, SourceFormat: dataPackageFormatJSON, SourceFilename: "workflow-bundle.json", SourceSizeBytes: 4096,
		FormTemplates: []Record{{"form_key": "form:test", "template_code": formCode, "name": "Portable Form", "description": "", "form_type": "generic_submission"}},
		FormVersions:  []Record{{"form_key": "form:test", "source_version_no": "1", "schema_json": `{"variant":"portable"}`, "validation_json": `{}`}},
		FormFields: []Record{{
			"form_key": "form:test", "field_key": "summary", "label": "Tóm tắt", "field_type": "text", "required": "true",
			"options_json": `[]`, "validation_json": `{"max_length":200}`, "help_text": "", "order_index": "10", "visibility_condition_json": `{}`,
		}},
		Pipelines: []Record{{
			"pipeline_key": "pipeline:test", "name": "Portable Pipeline " + suffix, "description": "", "project_type": "test",
			"engine_version": "3", "report_config_json": `{}`, "source_revision_no": "1",
		}},
		PipelineNodes: []Record{{
			"pipeline_key": "pipeline:test", "node_key": "node:start", "business_code": "portable.start", "name": "Bắt đầu", "type": "survey",
			"order_index": "1", "responsible_roles_json": `["pm"]`, "required_permission": "pipeline.view", "permissions_json": `["pipeline.view"]`,
			"entry_conditions_json": `{}`, "exit_conditions_json": `{}`, "approval_policy_json": `{"mode":"none"}`, "status": "active",
		}},
		PipelineSteps: []Record{{
			"pipeline_key": "pipeline:test", "node_key": "node:start", "step_key": "step:capture", "business_code": "portable.start.capture",
			"name": "Ghi nhận", "step_type": "task", "required_roles_json": `["pm"]`, "required_permission": "pipeline.view",
			"form_ref": "form:test", "form_required": "true", "surface_kind": "workflow_form", "order_index": "1",
		}},
	}
	preview, err := service.PreviewWorkflowDefinitionExcelImport(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if number(preview["blocker_count"]) != 0 || workflowDefinitionText(preview["preview_hash"]) == "" {
		t.Fatalf("unexpected workflow definition preview: %#v", preview)
	}
	result, err := service.ExecuteWorkflowDefinitionExcelImport(ctx, actorID, actorEmail, WorkflowDefinitionImportExecuteRequest{
		WorkflowDefinitionImportPreviewRequest: request,
		PreviewHash:                            workflowDefinitionText(preview["preview_hash"]), IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	if number(result["pipeline_count"]) != 1 || number(result["form_count"]) != 1 || workflowDefinitionText(result["status"]) != "draft" {
		t.Fatalf("unexpected import result: %#v", result)
	}
	createdPipelines := workflowFieldRecords(result["pipelines"])
	createdForms := workflowFieldRecords(result["forms"])
	if len(createdPipelines) != 1 || len(createdForms) != 1 {
		t.Fatalf("unexpected created definitions: %#v", result)
	}
	pipelineID := cleanID(createdPipelines[0]["template_id"])
	versionID := cleanID(createdForms[0]["version_id"])
	assertCount(t, db, `SELECT COUNT(*) FROM pipeline_templates WHERE id=$1 AND status='draft' AND engine_version=3`, 1, pipelineID)
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_versions WHERE id=$1 AND status='draft' AND definition_hash ~ '^[0-9a-f]{64}$'`, 1, versionID)
	assertCount(t, db, `SELECT COUNT(*) FROM pipeline_template_steps step JOIN pipeline_template_nodes node ON node.id=step.node_id WHERE node.template_id=$1 AND step.form_version_id=$2`, 1, pipelineID, versionID)
	assertCount(t, db, `SELECT COUNT(*) FROM pipeline_runs WHERE template_id=$1`, 0, pipelineID)
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_instances WHERE form_version_id=$1`, 0, versionID)
	exported, err := service.ExportWorkflowDefinitionsExcel(ctx, actorID, actorEmail, WorkflowDefinitionExportRequest{
		Scope: "pipeline", IDs: []string{pipelineID}, IncludeDependencies: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	exportedFiles := unzipXLSXFiles(t, exported.Data)
	foundDraftDependency := false
	for _, content := range exportedFiles {
		if strings.Contains(content, formCode) {
			foundDraftDependency = true
			break
		}
	}
	if !foundDraftDependency {
		t.Fatal("pipeline export omitted its draft Form/Fields dependency")
	}
	jsonExport, err := service.ExportWorkflowDefinitionsExcel(ctx, actorID, actorEmail, WorkflowDefinitionExportRequest{
		Format: dataPackageFormatJSON, Scope: "pipeline", IDs: []string{pipelineID}, IncludeDependencies: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if jsonExport.ContentType != exportJSONContentType || !strings.Contains(string(jsonExport.Data), `"schema": "consflow.workflow-definition-package"`) || !strings.Contains(string(jsonExport.Data), `"fields"`) {
		t.Fatalf("unexpected workflow JSON export: %s", jsonExport.Data)
	}
	conflictPreview, err := service.PreviewWorkflowDefinitionExcelImport(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if number(conflictPreview["blocker_count"]) == 0 || workflowDefinitionText(conflictPreview["preview_hash"]) != "" {
		t.Fatalf("expected existing draft conflict to block re-preview: %#v", conflictPreview)
	}
	updatedForm, err := service.UpdateWorkflowFormVersion(ctx, versionID, WorkflowFormVersionRequest{
		FieldsProvided: true,
		Fields: []WorkflowFormFieldRequest{{
			FieldKey: "summary", Label: "Tóm tắt đã cập nhật", FieldType: "text", Required: true,
			Options: []any{}, Validation: Record{"max_length": 200}, OrderIndex: 10, VisibilityCondition: Record{},
		}},
	}, actorID, actorEmail)
	if err != nil {
		t.Fatal(err)
	}
	if workflowDefinitionText(recordFromAny(updatedForm["schema_json"])["variant"]) != "portable" {
		t.Fatalf("partial field update erased schema_json: %#v", updatedForm["schema_json"])
	}
	cloned, err := repo.CloneTemplate(ctx, pipelineID, actorID)
	if err != nil {
		t.Fatal(err)
	}
	if workflowDefinitionInt(cloned["engine_version"], 0) != 3 {
		t.Fatalf("clone changed engine version: source=3 clone=%v", cloned["engine_version"])
	}

	replay, err := service.ExecuteWorkflowDefinitionExcelImport(ctx, actorID, actorEmail, WorkflowDefinitionImportExecuteRequest{
		WorkflowDefinitionImportPreviewRequest: request,
		PreviewHash:                            workflowDefinitionText(preview["preview_hash"]), IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(replay["idempotent_replay"]) {
		t.Fatalf("expected idempotent replay: %#v", replay)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_definition_imports WHERE actor_id=$1 AND source_format='json'`, 1, actorID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE actor_id=$1 AND event_type IN ('workflow_form_definition_imported','pipeline_definition_imported','workflow_definitions_imported')`, 3, actorID)
}

func TestWorkflowDefinitionPermissionMigrationAssignmentsAreFailClosed(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	for _, test := range []struct {
		role       string
		permission string
		want       int
	}{
		{"admin", "workflow_definition.import", 1}, {"admin", "workflow_definition.export", 1},
		{"developer", "workflow_definition.import", 1}, {"developer", "workflow_definition.export", 1},
		{"pm", "workflow_definition.import", 0}, {"pm", "workflow_definition.export", 1},
	} {
		assertCount(t, db, `
			SELECT COUNT(*) FROM role_permissions assignment
			JOIN roles role ON role.id=assignment.role_id
			JOIN permissions permission ON permission.id=assignment.permission_id
			WHERE role.name=$1 AND permission.key=$2
		`, test.want, test.role, test.permission)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO role_permissions (role_id,permission_id)
		SELECT role.id,permission.id FROM roles role CROSS JOIN permissions permission
		WHERE role.name='pm' AND permission.key='workflow_definition.import'
	`); err == nil {
		t.Fatal("database trigger allowed workflow_definition.import to be assigned to PM")
	}
	var pmRoleID, importPermissionID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM roles WHERE name='pm'`).Scan(&pmRoleID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM permissions WHERE key='workflow_definition.import'`).Scan(&importPermissionID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReplaceRolePermissions(ctx, pmRoleID, []string{importPermissionID}); err == nil {
		t.Fatal("repository allowed workflow_definition.import to be assigned to PM")
	}
	if _, err := db.ExecContext(ctx, `UPDATE roles SET name='workflow_definition_pm_renamed' WHERE name='pm'`); err == nil {
		t.Fatal("database trigger allowed a role with sensitive export permissions to be renamed out of the allowlist")
	}
}
