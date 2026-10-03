package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func appendWorkflowDefinitionForm(packageData *workflowDefinitionPackage, detail Record) string {
	code := workflowDefinitionText(detail["template_code"])
	versionNo := workflowDefinitionInt(detail["version_no"], 0)
	formKey := fmt.Sprintf("form:%s@v%d", code, versionNo)
	for _, existing := range packageData.Forms {
		if workflowDefinitionText(existing["form_key"]) == formKey {
			return formKey
		}
	}
	fields := workflowFieldRecords(detail["fields"])
	definitionHash := workflowDefinitionText(detail["definition_hash"])
	if definitionHash == "" {
		definitionHash, _ = workflowFormDefinitionHash(detail["schema_json"], detail["validation_json"], fields)
	}
	packageData.Forms = append(packageData.Forms, Record{
		"form_key": formKey, "template_code": code, "name": detail["template_name"], "description": detail["template_description"],
		"form_type": detail["form_type"], "source_template_id": detail["template_id"], "source_version_id": detail["version_id"],
		"source_version_no": versionNo, "source_definition_hash": definitionHash, "schema_json": detail["schema_json"],
		"validation_json": detail["validation_json"], "fields": fields,
	})
	return formKey
}

func appendWorkflowDefinitionPipeline(packageData *workflowDefinitionPackage, detail Record, catalog workflowDefinitionCatalog, includeDependencies bool, formIDs map[string]bool) {
	revisionNo := workflowDefinitionInt(detail["revision_no"], workflowDefinitionInt(detail["version"], 1))
	hash := workflowDefinitionText(detail["definition_hash"])
	pipelineKey := fmt.Sprintf("pipeline:%s:r%d", workflowDefinitionPortableKey(workflowDefinitionText(detail["name"])), revisionNo)
	if len(hash) >= 8 {
		pipelineKey += ":" + hash[:8]
	}
	nodeIDToKey := map[string]string{}
	nodes := []Record{}
	for index, node := range workflowFieldRecords(detail["nodes"]) {
		nodeKey := workflowDefinitionText(node["business_code"])
		if !validWorkflowDefinitionPackageKey(nodeKey) {
			nodeKey = fmt.Sprintf("node_%03d", index+1)
		}
		nodeIDToKey[cleanID(node["id"])] = nodeKey
		exportedRoles := workflowDefinitionExportRoleNames(node["responsible_role_ids"], catalog)
		exportedPolicy := workflowDefinitionExportApprovalRoles(recordFromAny(node["approval_policy_json"]), catalog)
		exportedNode := Record{
			"node_key": nodeKey, "business_code": node["business_code"], "name": node["name"], "description": node["description"],
			"type": node["type"], "position_x": node["position_x"], "position_y": node["position_y"], "order_index": node["order_index"],
			"responsible_role_ids": exportedRoles, "required_permission": node["required_permission"], "permissions": nodePermissions(node),
			"entry_conditions": node["entry_conditions"], "exit_conditions": node["exit_conditions"], "approval_rule": node["approval_rule"],
			"approval_policy_json": exportedPolicy, "rework_enabled": node["rework_enabled"], "replanning_enabled": node["replanning_enabled"],
			"fail_path": node["fail_path"], "status": node["status"],
		}
		steps := []Record{}
		for stepIndex, step := range workflowFieldRecords(node["steps"]) {
			stepKey := workflowDefinitionText(step["business_code"])
			if !validWorkflowDefinitionPackageKey(stepKey) {
				stepKey = fmt.Sprintf("step_%03d", stepIndex+1)
			}
			formRef := ""
			formVersionID := cleanID(step["form_version_id"])
			if formVersionID != "" {
				for _, form := range catalog.Forms {
					if cleanID(form["version_id"]) == formVersionID {
						formRef = fmt.Sprintf("%s@v%d", workflowDefinitionText(form["code"]), workflowDefinitionInt(form["version_no"], 0))
						if includeDependencies {
							formRef = fmt.Sprintf("form:%s@v%d", workflowDefinitionText(form["code"]), workflowDefinitionInt(form["version_no"], 0))
							formIDs[formVersionID] = true
						}
						break
					}
				}
			}
			steps = append(steps, Record{
				"step_key": stepKey, "business_code": step["business_code"], "name": step["name"], "description": step["description"],
				"step_type": step["step_type"], "required_role_ids": workflowDefinitionExportRoleNames(step["required_role_ids"], catalog),
				"required_permission": step["required_permission"], "checklist_required": step["checklist_required"],
				"document_required": step["document_required"], "photo_required": step["photo_required"], "form_ref": formRef,
				"form_required": step["form_required"], "surface_kind": step["surface_kind"], "domain_action_key": step["domain_action_key"], "order_index": step["order_index"],
			})
		}
		exportedNode["steps"] = steps
		nodes = append(nodes, exportedNode)
	}
	edges := []Record{}
	for index, edge := range workflowFieldRecords(detail["edges"]) {
		edges = append(edges, Record{
			"edge_key": fmt.Sprintf("edge_%03d", index+1), "from_node_key": nodeIDToKey[cleanID(edge["from_node_id"])],
			"to_node_key": nodeIDToKey[cleanID(edge["to_node_id"])], "condition_type": edgeConditionType(edge),
			"condition_config": edge["condition_config"], "label": edge["label"], "priority": edgePriority(edge),
		})
	}
	packageData.Pipelines = append(packageData.Pipelines, Record{
		"pipeline_key": pipelineKey, "target_family_id": "", "source_family_id": detail["template_family_id"],
		"name": detail["name"], "description": detail["description"], "project_type": detail["project_type"],
		"engine_version": detail["engine_version"], "report_config_json": detail["report_config_json"],
		"source_revision_no": revisionNo, "source_definition_hash": detail["definition_hash"], "nodes": nodes, "edges": edges,
	})
}

func workflowDefinitionExportRoleNames(value any, catalog workflowDefinitionCatalog) []string {
	labels := stringSlice(value)
	result := make([]string, 0, len(labels))
	for _, label := range labels {
		if name := catalog.RoleNameByDisplayName[strings.ToLower(strings.TrimSpace(label))]; name != "" {
			result = append(result, name)
			continue
		}
		// Legacy published definitions may still carry PM after catalog rename.
		if normalizeRoleLabel(label) == "pm" {
			if role, exists := catalog.RoleByName["pm"]; exists && !boolValue(role["external_only"]) {
				result = append(result, "pm")
				continue
			}
		}
		result = append(result, label)
	}
	return result
}

func workflowDefinitionExportApprovalRoles(value Record, catalog workflowDefinitionCatalog) Record {
	raw, _ := json.Marshal(value)
	policy := ApprovalPolicy{}
	if err := json.Unmarshal(raw, &policy); err != nil {
		return value
	}
	policy.Roles = workflowDefinitionExportRoleNames(policy.Roles, catalog)
	for index := range policy.Stages {
		policy.Stages[index].Roles = workflowDefinitionExportRoleNames(policy.Stages[index].Roles, catalog)
	}
	raw, _ = json.Marshal(policy)
	out := Record{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func workflowDefinitionPortableKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastSeparator := false
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
			lastSeparator = false
			continue
		}
		if !lastSeparator && builder.Len() > 0 {
			builder.WriteByte('-')
			lastSeparator = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		result = "workflow"
	}
	if len(result) > 60 {
		result = strings.TrimRight(result[:60], "-")
	}
	return result
}

func generateWorkflowDefinitionWorkbook(packageData workflowDefinitionPackage, catalog workflowDefinitionCatalog, generatedAt time.Time, mode string) (tabularExportResult, error) {
	manifest := []Record{
		{"key": "workbook_version", "value": workflowDefinitionWorkbookVersion},
		{"key": "scope", "value": packageData.Scope},
		{"key": "pipeline_count", "value": len(packageData.Pipelines)},
		{"key": "form_count", "value": len(packageData.Forms)},
		{"key": "generated_at", "value": generatedAt.UTC().Format(time.RFC3339)},
		{"key": "reference_fingerprint", "value": catalog.Fingerprint},
	}
	instructions := []Record{
		{"section": "Mục đích", "instruction": "Di chuyển định nghĩa Pipeline và Form/Fields giữa các môi trường bằng package key; không dùng source UUID làm quan hệ nội bộ."},
		{"section": "An toàn", "instruction": "Import chỉ tạo bản nháp. Hệ thống không tự publish, không chạy Pipeline, không tạo Work Item, Form Instance hoặc dữ liệu công trình."},
		{"section": "Thứ tự", "instruction": "Giữ nguyên tên sheet/header. Bundle import Form trước rồi Pipeline trong một transaction."},
		{"section": "Tham chiếu", "instruction": "Role dùng roles.name; quyền dùng permissions.key; form_ref dùng form_key trong bundle hoặc template_code@vN đã published ở đích."},
		{"section": "Xung đột", "instruction": "Import bị chặn nếu Form hoặc Pipeline family đích đang có draft. Không ghi đè draft/published/archived/runtime."},
	}
	pipelineRows := []Record{}
	nodeRows := []Record{}
	stepRows := []Record{}
	edgeRows := []Record{}
	for _, pipeline := range packageData.Pipelines {
		pipelineRows = append(pipelineRows, Record{
			"pipeline_key": pipeline["pipeline_key"], "target_family_id": pipeline["target_family_id"], "source_family_id": pipeline["source_family_id"],
			"name": pipeline["name"], "description": pipeline["description"], "project_type": pipeline["project_type"],
			"engine_version": pipeline["engine_version"], "report_config_json": workflowDefinitionJSONString(pipeline["report_config_json"], Record{}),
			"source_revision_no": pipeline["source_revision_no"], "source_definition_hash": pipeline["source_definition_hash"],
		})
		for _, node := range workflowFieldRecords(pipeline["nodes"]) {
			nodeRows = append(nodeRows, Record{
				"pipeline_key": pipeline["pipeline_key"], "node_key": node["node_key"], "business_code": node["business_code"], "name": node["name"],
				"description": node["description"], "type": node["type"], "position_x": node["position_x"], "position_y": node["position_y"],
				"order_index": node["order_index"], "responsible_roles_json": workflowDefinitionJSONString(node["responsible_role_ids"], []any{}),
				"required_permission": node["required_permission"], "permissions_json": workflowDefinitionJSONString(node["permissions"], []any{}),
				"entry_conditions_json": workflowDefinitionJSONString(node["entry_conditions"], Record{}), "exit_conditions_json": workflowDefinitionJSONString(node["exit_conditions"], Record{}),
				"approval_rule": node["approval_rule"], "approval_policy_json": workflowDefinitionJSONString(node["approval_policy_json"], Record{}),
				"rework_enabled": node["rework_enabled"], "replanning_enabled": node["replanning_enabled"], "fail_path": node["fail_path"], "status": node["status"],
			})
			for _, step := range workflowFieldRecords(node["steps"]) {
				stepRows = append(stepRows, Record{
					"pipeline_key": pipeline["pipeline_key"], "node_key": node["node_key"], "step_key": step["step_key"],
					"business_code": step["business_code"], "name": step["name"], "description": step["description"], "step_type": step["step_type"],
					"required_roles_json": workflowDefinitionJSONString(step["required_role_ids"], []any{}), "required_permission": step["required_permission"],
					"checklist_required": step["checklist_required"], "document_required": step["document_required"], "photo_required": step["photo_required"],
					"form_ref": step["form_ref"], "form_required": step["form_required"], "surface_kind": step["surface_kind"],
					"domain_action_key": step["domain_action_key"], "order_index": step["order_index"],
				})
			}
		}
		for _, edge := range workflowFieldRecords(pipeline["edges"]) {
			edgeRows = append(edgeRows, Record{
				"pipeline_key": pipeline["pipeline_key"], "edge_key": edge["edge_key"], "from_node_key": edge["from_node_key"],
				"to_node_key": edge["to_node_key"], "condition_type": edge["condition_type"],
				"condition_config_json": workflowDefinitionJSONString(edge["condition_config"], Record{}), "label": edge["label"], "priority": edge["priority"],
			})
		}
	}
	formTemplateRows := []Record{}
	formVersionRows := []Record{}
	formFieldRows := []Record{}
	for _, form := range packageData.Forms {
		formTemplateRows = append(formTemplateRows, Record{
			"form_key": form["form_key"], "template_code": form["template_code"], "name": form["name"], "description": form["description"],
			"form_type": form["form_type"], "source_template_id": form["source_template_id"],
		})
		formVersionRows = append(formVersionRows, Record{
			"form_key": form["form_key"], "source_version_no": form["source_version_no"],
			"schema_json": workflowDefinitionJSONString(form["schema_json"], Record{}), "validation_json": workflowDefinitionJSONString(form["validation_json"], Record{}),
			"source_definition_hash": form["source_definition_hash"],
		})
		for _, field := range workflowFieldRecords(form["fields"]) {
			formFieldRows = append(formFieldRows, Record{
				"form_key": form["form_key"], "field_key": field["field_key"], "label": field["label"], "field_type": field["field_type"],
				"required": field["required"], "options_json": workflowDefinitionJSONString(field["options_json"], []any{}),
				"validation_json": workflowDefinitionJSONString(field["validation_json"], Record{}), "help_text": field["help_text"],
				"order_index": field["order_index"], "visibility_condition_json": workflowDefinitionJSONString(field["visibility_condition_json"], Record{}),
			})
		}
	}
	conditionRows := []Record{}
	for _, definition := range PipelineEdgeConditionCatalog() {
		conditionRows = append(conditionRows, Record{"condition_type": definition.Type, "label": definition.Label, "description": definition.Description, "config_schema_json": workflowDefinitionJSONString(definition.ConfigSchema, []any{})})
	}
	sheets := []xlsxWorkbookSheet{
		{Name: "Huong_dan", Columns: workflowDefinitionColumns("section", "instruction"), Rows: instructions},
		{Name: "Manifest", Columns: workflowDefinitionColumns("key", "value"), Rows: manifest},
		{Name: "Pipelines", Columns: workflowDefinitionColumns("pipeline_key", "target_family_id", "source_family_id", "name", "description", "project_type", "engine_version", "report_config_json", "source_revision_no", "source_definition_hash"), Rows: pipelineRows},
		{Name: "Pipeline_Nodes", Columns: workflowDefinitionColumns("pipeline_key", "node_key", "business_code", "name", "description", "type", "position_x", "position_y", "order_index", "responsible_roles_json", "required_permission", "permissions_json", "entry_conditions_json", "exit_conditions_json", "approval_rule", "approval_policy_json", "rework_enabled", "replanning_enabled", "fail_path", "status"), Rows: nodeRows},
		{Name: "Pipeline_Steps", Columns: workflowDefinitionColumns("pipeline_key", "node_key", "step_key", "business_code", "name", "description", "step_type", "required_roles_json", "required_permission", "checklist_required", "document_required", "photo_required", "form_ref", "form_required", "surface_kind", "domain_action_key", "order_index"), Rows: stepRows},
		{Name: "Pipeline_Edges", Columns: workflowDefinitionColumns("pipeline_key", "edge_key", "from_node_key", "to_node_key", "condition_type", "condition_config_json", "label", "priority"), Rows: edgeRows},
		{Name: "Form_Templates", Columns: workflowDefinitionColumns("form_key", "template_code", "name", "description", "form_type", "source_template_id"), Rows: formTemplateRows},
		{Name: "Form_Versions", Columns: workflowDefinitionColumns("form_key", "source_version_no", "schema_json", "validation_json", "source_definition_hash"), Rows: formVersionRows},
		{Name: "Form_Fields", Columns: workflowDefinitionColumns("form_key", "field_key", "label", "field_type", "required", "options_json", "validation_json", "help_text", "order_index", "visibility_condition_json"), Rows: formFieldRows},
		{Name: "Ref_Permissions", Columns: workflowDefinitionColumns("key", "name", "module"), Rows: catalog.Permissions},
		{Name: "Ref_Roles", Columns: workflowDefinitionColumns("name", "display_name", "external_only"), Rows: catalog.Roles},
		{Name: "Ref_Edge_Conditions", Columns: workflowDefinitionColumns("condition_type", "label", "description", "config_schema_json"), Rows: conditionRows},
	}
	data, err := generateWorkbookXLSX("Workflow Definition Workbook v1", sheets, generatedAt)
	if err != nil {
		return tabularExportResult{}, err
	}
	filename := fmt.Sprintf("workflow-definition-%s-%s-%s.xlsx", packageData.Scope, mode, generatedAt.Format("20060102-150405"))
	return tabularExportResult{Data: data, Filename: filename, ContentType: exportXLSXContentType}, nil
}

func generateWorkflowDefinitionJSONPackage(packageData workflowDefinitionPackage, catalog workflowDefinitionCatalog, generatedAt time.Time, mode string) (tabularExportResult, error) {
	pipelines := make([]Record, 0, len(packageData.Pipelines))
	for _, source := range packageData.Pipelines {
		pipelines = append(pipelines, Record{
			"pipeline_key": source["pipeline_key"], "target_family_id": source["target_family_id"],
			"name": source["name"], "description": source["description"], "project_type": source["project_type"],
			"engine_version": source["engine_version"], "report_config_json": defaultWorkflowDefinitionJSON(source["report_config_json"], Record{}),
			"source": Record{
				"family_id": source["source_family_id"], "revision_no": source["source_revision_no"], "definition_hash": source["source_definition_hash"],
			},
			"nodes": workflowFieldRecords(source["nodes"]), "edges": workflowFieldRecords(source["edges"]),
		})
	}
	forms := make([]Record, 0, len(packageData.Forms))
	for _, source := range packageData.Forms {
		fields := make([]Record, 0, len(workflowFieldRecords(source["fields"])))
		for _, field := range workflowFieldRecords(source["fields"]) {
			fields = append(fields, Record{
				"field_key": field["field_key"], "label": field["label"], "field_type": field["field_type"], "required": field["required"],
				"options_json":    defaultWorkflowDefinitionJSON(field["options_json"], []any{}),
				"validation_json": defaultWorkflowDefinitionJSON(field["validation_json"], Record{}), "help_text": field["help_text"],
				"order_index": field["order_index"], "visibility_condition_json": defaultWorkflowDefinitionJSON(field["visibility_condition_json"], Record{}),
			})
		}
		forms = append(forms, Record{
			"form_key": source["form_key"],
			"template": Record{
				"template_code": source["template_code"], "name": source["name"], "description": source["description"], "form_type": source["form_type"],
			},
			"source": Record{
				"template_id": source["source_template_id"], "version_id": source["source_version_id"],
				"version_no": source["source_version_no"], "definition_hash": source["source_definition_hash"],
			},
			"version": Record{
				"schema_json":     defaultWorkflowDefinitionJSON(source["schema_json"], Record{}),
				"validation_json": defaultWorkflowDefinitionJSON(source["validation_json"], Record{}),
				"fields":          fields,
			},
		})
	}
	conditions := make([]Record, 0, len(PipelineEdgeConditionCatalog()))
	for _, definition := range PipelineEdgeConditionCatalog() {
		conditions = append(conditions, Record{
			"condition_type": definition.Type, "label": definition.Label, "description": definition.Description, "config_schema_json": definition.ConfigSchema,
		})
	}
	envelope := struct {
		Schema               string   `json:"schema"`
		SchemaVersion        int      `json:"schema_version"`
		Scope                string   `json:"scope"`
		Mode                 string   `json:"mode"`
		GeneratedAt          string   `json:"generated_at"`
		ReferenceFingerprint string   `json:"reference_fingerprint"`
		Instructions         []string `json:"instructions"`
		Pipelines            []Record `json:"pipelines"`
		Forms                []Record `json:"forms"`
		References           struct {
			Permissions      []Record `json:"permissions"`
			Roles            []Record `json:"roles"`
			EdgeConditions   []Record `json:"edge_conditions"`
			PublishedForms   []Record `json:"published_forms"`
			PipelineFamilies []Record `json:"pipeline_families"`
		} `json:"references"`
	}{
		Schema: "consflow.workflow-definition-package", SchemaVersion: 1, Scope: packageData.Scope, Mode: mode,
		GeneratedAt: generatedAt.UTC().Format(time.RFC3339), ReferenceFingerprint: catalog.Fingerprint,
		Instructions: []string{
			"Import chỉ tạo bản nháp; không publish, chạy Pipeline hoặc tạo dữ liệu runtime.",
			"Quan hệ nội bộ dùng package key; source UUID chỉ là provenance và references là dữ liệu chỉ đọc.",
		},
		Pipelines: pipelines, Forms: forms,
	}
	envelope.References.Permissions = catalog.Permissions
	envelope.References.Roles = catalog.Roles
	envelope.References.EdgeConditions = conditions
	envelope.References.PublishedForms = catalog.Forms
	envelope.References.PipelineFamilies = catalog.PipelineFamilies
	data, err := marshalDataPackageJSON(envelope)
	if err != nil {
		return tabularExportResult{}, err
	}
	filename := fmt.Sprintf("workflow-definition-%s-%s-%s.json", packageData.Scope, mode, generatedAt.Format("20060102-150405"))
	return tabularExportResult{Data: data, Filename: filename, ContentType: exportJSONContentType}, nil
}

func workflowDefinitionColumns(keys ...string) []tabularColumn {
	columns := make([]tabularColumn, 0, len(keys))
	for _, key := range keys {
		columns = append(columns, tabularColumn{Key: key, Header: key})
	}
	return columns
}

func workflowDefinitionJSONString(value, empty any) string {
	parsed := defaultWorkflowDefinitionJSON(value, empty)
	raw, err := json.Marshal(parsed)
	if err != nil {
		raw, _ = json.Marshal(empty)
	}
	return string(raw)
}
