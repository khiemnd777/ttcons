package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"sort"
	"strconv"
	"strings"
)

func (s *Service) validateWorkflowDefinitionImport(ctx context.Context, req WorkflowDefinitionImportPreviewRequest) (workflowDefinitionValidation, error) {
	scope, err := normalizeWorkflowDefinitionScope(req.Scope)
	if err != nil {
		return workflowDefinitionValidation{}, err
	}
	filename := strings.TrimSpace(req.SourceFilename)
	sourceFormat, packageVersion, err := normalizeDataPackageImportSource(req.SourceFormat, req.PackageVersion, req.WorkbookVersion, filename)
	if err != nil {
		return workflowDefinitionValidation{}, err
	}
	if req.SourceSizeBytes <= 0 || req.SourceSizeBytes > workflowDefinitionMaximumFileBytes {
		return workflowDefinitionValidation{}, validationError("Tệp import phải có dung lượng không quá 10 MB", Record{"field": "source_size_bytes", "maximum": workflowDefinitionMaximumFileBytes})
	}
	if len(req.Pipelines) > workflowDefinitionMaximumPipelines || len(req.FormTemplates) > workflowDefinitionMaximumForms {
		return workflowDefinitionValidation{}, validationError("Gói import vượt giới hạn 25 Pipeline hoặc 100 Form", Record{"pipeline_maximum": workflowDefinitionMaximumPipelines, "form_maximum": workflowDefinitionMaximumForms})
	}
	for sheet, rowCount := range map[string]int{
		"pipeline_nodes": len(req.PipelineNodes), "pipeline_steps": len(req.PipelineSteps), "pipeline_edges": len(req.PipelineEdges),
		"form_versions": len(req.FormVersions), "form_fields": len(req.FormFields),
	} {
		if rowCount > workflowDefinitionMaximumSheetRows {
			return workflowDefinitionValidation{}, validationError("Một sheet dữ liệu vượt giới hạn 10.000 dòng", Record{"sheet": sheet, "maximum": workflowDefinitionMaximumSheetRows})
		}
	}
	if len(req.Pipelines) == 0 && len(req.FormTemplates) == 0 {
		return workflowDefinitionValidation{}, validationError("Gói import cần ít nhất một Pipeline hoặc Form")
	}
	for _, contract := range []struct {
		field   string
		rows    []Record
		allowed []string
	}{
		{"pipelines", req.Pipelines, []string{"pipeline_key", "target_family_id", "source_family_id", "name", "description", "project_type", "engine_version", "report_config_json", "source_revision_no", "source_definition_hash"}},
		{"pipeline_nodes", req.PipelineNodes, []string{"pipeline_key", "node_key", "business_code", "name", "description", "type", "position_x", "position_y", "order_index", "responsible_roles_json", "required_permission", "permissions_json", "entry_conditions_json", "exit_conditions_json", "approval_rule", "approval_policy_json", "rework_enabled", "replanning_enabled", "fail_path", "status"}},
		{"pipeline_steps", req.PipelineSteps, []string{"pipeline_key", "node_key", "step_key", "business_code", "name", "description", "step_type", "required_roles_json", "required_permission", "checklist_required", "document_required", "photo_required", "form_ref", "form_required", "surface_kind", "domain_action_key", "order_index"}},
		{"pipeline_edges", req.PipelineEdges, []string{"pipeline_key", "edge_key", "from_node_key", "to_node_key", "condition_type", "condition_config_json", "label", "priority"}},
		{"form_templates", req.FormTemplates, []string{"form_key", "template_code", "name", "description", "form_type", "source_template_id"}},
		{"form_versions", req.FormVersions, []string{"form_key", "source_version_no", "schema_json", "validation_json", "source_definition_hash"}},
		{"form_fields", req.FormFields, []string{"form_key", "field_key", "label", "field_type", "required", "options_json", "validation_json", "help_text", "order_index", "visibility_condition_json"}},
	} {
		if err := validateDataPackageRecordKeys(contract.field, contract.rows, contract.allowed...); err != nil {
			return workflowDefinitionValidation{}, err
		}
	}
	if scope == "form" && len(req.Pipelines) > 0 {
		return workflowDefinitionValidation{}, validationError("Gói phạm vi Form không được chứa Pipeline")
	}
	if scope == "pipeline" && len(req.FormTemplates) > 0 {
		return workflowDefinitionValidation{}, validationError("Gói phạm vi Pipeline không được chứa Form; dùng phạm vi bundle cho dependency đầy đủ")
	}
	catalog, err := s.repo.WorkflowDefinitionCatalog(ctx)
	if err != nil {
		return workflowDefinitionValidation{}, err
	}
	blockers := []Record{}
	warnings := []Record{}
	forms, formKeys := normalizeWorkflowDefinitionForms(req, catalog, &blockers, &warnings)
	pipelines := normalizeWorkflowDefinitionPipelines(req, catalog, formKeys, &blockers, &warnings)
	packageData := workflowDefinitionPackage{
		Scope: scope, Pipelines: pipelines, Forms: forms,
		PipelineNodes: normalizedChildRows(pipelines, "nodes"),
		PipelineSteps: normalizedNestedChildRows(pipelines, "nodes", "steps"),
		PipelineEdges: normalizedChildRows(pipelines, "edges"),
	}
	entities := make([]Record, 0, len(forms)+len(pipelines))
	for _, form := range forms {
		entities = append(entities, Record{"kind": "form", "key": form["form_key"], "name": form["name"], "action": form["action"], "target": form["target_template_id"]})
	}
	for _, pipeline := range pipelines {
		entities = append(entities, Record{"kind": "pipeline", "key": pipeline["pipeline_key"], "name": pipeline["name"], "action": pipeline["action"], "target": pipeline["target_family_id"]})
	}
	hash := ""
	if len(blockers) == 0 {
		payload, err := json.Marshal(Record{
			"package_version": packageVersion, "source_format": sourceFormat, "source_filename": filename,
			"source_size_bytes": req.SourceSizeBytes, "reference_fingerprint": catalog.Fingerprint, "package": packageData,
		})
		if err != nil {
			return workflowDefinitionValidation{}, err
		}
		mac := hmac.New(sha256.New, []byte(s.jwtSecret))
		if _, err := mac.Write(payload); err != nil {
			return workflowDefinitionValidation{}, err
		}
		hash = hex.EncodeToString(mac.Sum(nil))
	}
	preview := Record{
		"scope": scope, "workbook_version": workflowDefinitionWorkbookVersion, "package_version": packageVersion, "source_format": sourceFormat, "source_filename": filename,
		"pipeline_count": len(pipelines), "form_count": len(forms), "blocker_count": len(blockers),
		"warning_count": len(warnings), "blockers": blockers, "warnings": warnings, "entities": entities,
		"preview_hash": hash, "reference_fingerprint": catalog.Fingerprint,
		"confirmation": "Import chỉ tạo bản nháp; không phát hành, không chạy Pipeline và không tạo dữ liệu runtime.",
	}
	return workflowDefinitionValidation{Package: packageData, Preview: preview, Hash: hash, Catalog: catalog, SourceFormat: sourceFormat}, nil
}

func normalizeWorkflowDefinitionScope(scope string) (string, error) {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if !statusIn(scope, "form", "pipeline", "bundle") {
		return "", validationError("Phạm vi workflow definition không hợp lệ", Record{"scope": scope})
	}
	return scope, nil
}

func normalizeWorkflowDefinitionExportScope(scope string) (string, error) {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if !statusIn(scope, "form", "pipeline") {
		return "", validationError("Phạm vi xuất workflow definition không hợp lệ", Record{"scope": scope})
	}
	return scope, nil
}

func normalizeWorkflowDefinitionForms(req WorkflowDefinitionImportPreviewRequest, catalog workflowDefinitionCatalog, blockers, warnings *[]Record) ([]Record, map[string]Record) {
	templateRows := map[string]Record{}
	for index, raw := range req.FormTemplates {
		key := workflowDefinitionText(raw["form_key"])
		if !validWorkflowDefinitionPackageKey(key) {
			addWorkflowDefinitionIssue(blockers, "FORM_KEY_INVALID", "Form cần form_key ổn định và hợp lệ", "form_templates", index+2, key)
			continue
		}
		if _, exists := templateRows[key]; exists {
			addWorkflowDefinitionIssue(blockers, "FORM_KEY_DUPLICATE", "form_key bị trùng trong gói", "form_templates", index+2, key)
			continue
		}
		templateRows[key] = raw
	}
	versionRows := map[string]Record{}
	for index, raw := range req.FormVersions {
		key := workflowDefinitionText(raw["form_key"])
		if _, exists := versionRows[key]; exists {
			addWorkflowDefinitionIssue(blockers, "FORM_VERSION_DUPLICATE", "Mỗi Form chỉ được có một version trong gói", "form_versions", index+2, key)
			continue
		}
		versionRows[key] = raw
	}
	fieldRows := map[string][]Record{}
	for _, raw := range req.FormFields {
		key := workflowDefinitionText(raw["form_key"])
		fieldRows[key] = append(fieldRows[key], raw)
	}

	keys := make([]string, 0, len(templateRows))
	for key := range templateRows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	forms := make([]Record, 0, len(keys))
	formKeys := map[string]Record{}
	seenCodes := map[string]string{}
	for _, key := range keys {
		rawTemplate := templateRows[key]
		rawVersion, versionExists := versionRows[key]
		if !versionExists {
			addWorkflowDefinitionIssue(blockers, "FORM_VERSION_MISSING", "Form chưa có dòng Form_Versions", "form_versions", 0, key)
			continue
		}
		if len(fieldRows[key]) == 0 {
			addWorkflowDefinitionIssue(blockers, "FORM_FIELDS_MISSING", "Form cần ít nhất một Field", "form_fields", 0, key)
			continue
		}
		templateReq := WorkflowFormTemplateRequest{
			Code: workflowDefinitionText(rawTemplate["template_code"]), Name: workflowDefinitionText(rawTemplate["name"]),
			Description: workflowDefinitionText(rawTemplate["description"]), FormType: workflowDefinitionText(rawTemplate["form_type"]), Status: "active",
		}
		templateBody, err := normalizeWorkflowFormTemplateRequest(templateReq, true)
		if err != nil {
			addWorkflowDefinitionIssue(blockers, "FORM_TEMPLATE_INVALID", err.Error(), "form_templates", 0, key)
			continue
		}
		codeKey := strings.ToLower(workflowDefinitionText(templateBody["code"]))
		if prior := seenCodes[codeKey]; prior != "" && prior != key {
			addWorkflowDefinitionIssue(blockers, "FORM_CODE_DUPLICATE", "template_code được dùng bởi nhiều form_key", "form_templates", 0, key)
			continue
		}
		seenCodes[codeKey] = key

		schema, err := workflowDefinitionJSONRecord(rawVersion["schema_json"])
		if err != nil {
			addWorkflowDefinitionIssue(blockers, "FORM_SCHEMA_INVALID", "schema_json phải là JSON object hợp lệ", "form_versions", 0, key)
			continue
		}
		validationJSON, err := workflowDefinitionJSONRecord(rawVersion["validation_json"])
		if err != nil {
			addWorkflowDefinitionIssue(blockers, "FORM_VALIDATION_INVALID", "validation_json phải là JSON object hợp lệ", "form_versions", 0, key)
			continue
		}
		fieldRequests := make([]WorkflowFormFieldRequest, 0, len(fieldRows[key]))
		for fieldIndex, rawField := range fieldRows[key] {
			options, optionsErr := workflowDefinitionJSONValue(rawField["options_json"], []any{})
			fieldValidation, validationErr := workflowDefinitionJSONRecord(rawField["validation_json"])
			visibility, visibilityErr := workflowDefinitionJSONRecord(rawField["visibility_condition_json"])
			if optionsErr != nil || validationErr != nil || visibilityErr != nil {
				addWorkflowDefinitionIssue(blockers, "FORM_FIELD_JSON_INVALID", "JSON của Field không hợp lệ", "form_fields", fieldIndex+2, key)
				continue
			}
			fieldRequests = append(fieldRequests, WorkflowFormFieldRequest{
				FieldKey: workflowDefinitionText(rawField["field_key"]), Label: workflowDefinitionText(rawField["label"]),
				FieldType: workflowDefinitionText(rawField["field_type"]), Required: workflowDefinitionBool(rawField["required"]),
				Options: options, Validation: fieldValidation, HelpText: workflowDefinitionText(rawField["help_text"]),
				OrderIndex: workflowDefinitionInt(rawField["order_index"], (fieldIndex+1)*10), VisibilityCondition: visibility,
			})
		}
		versionBody, err := normalizeWorkflowFormVersionRequest(WorkflowFormVersionRequest{Schema: schema, Validation: validationJSON, Fields: fieldRequests}, true)
		if err != nil {
			addWorkflowDefinitionIssue(blockers, "FORM_FIELDS_INVALID", err.Error(), "form_fields", 0, key)
			continue
		}
		definitionHash, err := workflowFormDefinitionHash(versionBody["schema_json"], versionBody["validation_json"], workflowFieldRecords(versionBody["fields"]))
		if err != nil {
			addWorkflowDefinitionIssue(blockers, "FORM_HASH_INVALID", "Không thể chuẩn hoá nội dung Form", "form_versions", 0, key)
			continue
		}
		sourceHash := workflowDefinitionText(rawVersion["source_definition_hash"])
		if sourceHash != "" && sourceHash != definitionHash {
			addWorkflowDefinitionIssue(warnings, "FORM_CONTENT_CHANGED", "Nội dung Form khác hash nguồn; bản nháp đích sẽ dùng nội dung hiện tại trong file", "form_versions", 0, key)
		}
		action := "create_template"
		targetTemplateID := ""
		if existing, exists := catalog.FormByCode[codeKey]; exists {
			targetTemplateID = cleanID(existing["template_id"])
			action = "create_version"
			if workflowDefinitionText(existing["template_status"]) != "active" {
				addWorkflowDefinitionIssue(blockers, "FORM_TEMPLATE_ARCHIVED", "Form đích đã lưu trữ; import không tự kích hoạt lại", "form_templates", 0, key)
			}
			if number(existing["draft_count"]) > 0 {
				addWorkflowDefinitionIssue(blockers, "FORM_DRAFT_CONFLICT", "Form đích đang có version nháp; import không ghi đè", "form_templates", 0, key)
			}
		}
		form := Record{
			"form_key": key, "template_code": templateBody["code"], "name": templateBody["name"], "description": templateBody["description"],
			"form_type": templateBody["form_type"], "schema_json": versionBody["schema_json"], "validation_json": versionBody["validation_json"],
			"fields": versionBody["fields"], "definition_hash": definitionHash, "source_version_no": workflowDefinitionInt(rawVersion["source_version_no"], 0),
			"source_template_id": workflowDefinitionText(rawTemplate["source_template_id"]), "action": action, "target_template_id": targetTemplateID,
		}
		forms = append(forms, form)
		formKeys[key] = form
	}
	for key := range versionRows {
		if _, exists := templateRows[key]; !exists {
			addWorkflowDefinitionIssue(blockers, "FORM_TEMPLATE_MISSING", "Form_Versions tham chiếu form_key không tồn tại", "form_versions", 0, key)
		}
	}
	for key := range fieldRows {
		if _, exists := templateRows[key]; !exists {
			addWorkflowDefinitionIssue(blockers, "FORM_TEMPLATE_MISSING", "Form_Fields tham chiếu form_key không tồn tại", "form_fields", 0, key)
		}
	}
	return forms, formKeys
}

func normalizeWorkflowDefinitionPipelines(req WorkflowDefinitionImportPreviewRequest, catalog workflowDefinitionCatalog, importedForms map[string]Record, blockers, warnings *[]Record) []Record {
	pipelineRows := map[string]Record{}
	for index, raw := range req.Pipelines {
		key := workflowDefinitionText(raw["pipeline_key"])
		if !validWorkflowDefinitionPackageKey(key) {
			addWorkflowDefinitionIssue(blockers, "PIPELINE_KEY_INVALID", "Pipeline cần pipeline_key ổn định và hợp lệ", "pipelines", index+2, key)
			continue
		}
		if _, exists := pipelineRows[key]; exists {
			addWorkflowDefinitionIssue(blockers, "PIPELINE_KEY_DUPLICATE", "pipeline_key bị trùng trong gói", "pipelines", index+2, key)
			continue
		}
		pipelineRows[key] = raw
	}
	nodesByPipeline := groupWorkflowDefinitionRows(req.PipelineNodes, "pipeline_key")
	stepsByNode := map[string][]Record{}
	for _, row := range req.PipelineSteps {
		compound := workflowDefinitionText(row["pipeline_key"]) + "\x00" + workflowDefinitionText(row["node_key"])
		stepsByNode[compound] = append(stepsByNode[compound], row)
	}
	edgesByPipeline := groupWorkflowDefinitionRows(req.PipelineEdges, "pipeline_key")
	keys := make([]string, 0, len(pipelineRows))
	for key := range pipelineRows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pipelines := make([]Record, 0, len(keys))
	for _, key := range keys {
		raw := pipelineRows[key]
		name := workflowDefinitionText(raw["name"])
		if name == "" || len([]rune(name)) > 255 {
			addWorkflowDefinitionIssue(blockers, "PIPELINE_NAME_INVALID", "Tên Pipeline là bắt buộc và không quá 255 ký tự", "pipelines", 0, key)
			continue
		}
		engineVersion := workflowDefinitionInt(raw["engine_version"], 2)
		if engineVersion < 1 || engineVersion > 4 {
			addWorkflowDefinitionIssue(blockers, "PIPELINE_ENGINE_INVALID", "engine_version phải trong khoảng 1 đến 4", "pipelines", 0, key)
			continue
		}
		reportConfig, err := workflowDefinitionJSONRecord(raw["report_config_json"])
		if err != nil {
			addWorkflowDefinitionIssue(blockers, "PIPELINE_REPORT_CONFIG_INVALID", "report_config_json phải là JSON object", "pipelines", 0, key)
			continue
		}
		targetFamilyID := cleanID(raw["target_family_id"])
		action := "create_family"
		if targetFamilyID != "" {
			if _, err := uuid.Parse(targetFamilyID); err != nil {
				addWorkflowDefinitionIssue(blockers, "PIPELINE_TARGET_INVALID", "target_family_id không hợp lệ", "pipelines", 0, key)
			} else if family, exists := catalog.PipelineFamilyByID[targetFamilyID]; !exists {
				addWorkflowDefinitionIssue(blockers, "PIPELINE_TARGET_MISSING", "Không tìm thấy family Pipeline đích", "pipelines", 0, key)
			} else {
				action = "create_revision"
				if number(family["draft_count"]) > 0 {
					addWorkflowDefinitionIssue(blockers, "PIPELINE_DRAFT_CONFLICT", "Family Pipeline đích đang có revision nháp; import không ghi đè", "pipelines", 0, key)
				}
			}
		}
		nodes := normalizeWorkflowDefinitionNodes(key, nodesByPipeline[key], stepsByNode, engineVersion, catalog, importedForms, blockers)
		edges := normalizeWorkflowDefinitionEdges(key, edgesByPipeline[key], nodes, catalog, importedForms, blockers)
		if len(nodes) == 0 {
			addWorkflowDefinitionIssue(blockers, "PIPELINE_NODES_MISSING", "Pipeline cần ít nhất một Node", "pipeline_nodes", 0, key)
		}
		stepCount := 0
		for _, node := range nodes {
			stepCount += len(workflowFieldRecords(node["steps"]))
		}
		if stepCount == 0 {
			addWorkflowDefinitionIssue(blockers, "PIPELINE_STEPS_MISSING", "Pipeline cần ít nhất một Step", "pipeline_steps", 0, key)
		}
		appendWorkflowDefinitionGraphIssues(key, nodes, edges, blockers)
		pipeline := Record{
			"pipeline_key": key, "name": name, "description": workflowDefinitionText(raw["description"]),
			"project_type": workflowDefinitionText(raw["project_type"]), "engine_version": engineVersion,
			"report_config_json": reportConfig, "nodes": nodes, "edges": edges, "target_family_id": targetFamilyID,
			"source_family_id": workflowDefinitionText(raw["source_family_id"]), "source_revision_no": workflowDefinitionInt(raw["source_revision_no"], 0),
			"source_definition_hash": workflowDefinitionText(raw["source_definition_hash"]), "action": action,
		}
		pipelines = append(pipelines, pipeline)
	}
	for key := range nodesByPipeline {
		if _, exists := pipelineRows[key]; !exists {
			addWorkflowDefinitionIssue(blockers, "PIPELINE_MISSING", "Pipeline_Nodes tham chiếu pipeline_key không tồn tại", "pipeline_nodes", 0, key)
		}
	}
	for key := range edgesByPipeline {
		if _, exists := pipelineRows[key]; !exists {
			addWorkflowDefinitionIssue(blockers, "PIPELINE_MISSING", "Pipeline_Edges tham chiếu pipeline_key không tồn tại", "pipeline_edges", 0, key)
		}
	}
	for compound := range stepsByNode {
		parts := strings.SplitN(compound, "\x00", 2)
		if len(parts) != 2 {
			continue
		}
		pipelineKey, nodeKey := parts[0], parts[1]
		if _, exists := pipelineRows[pipelineKey]; !exists {
			addWorkflowDefinitionIssue(blockers, "PIPELINE_MISSING", "Pipeline_Steps tham chiếu pipeline_key không tồn tại", "pipeline_steps", 0, pipelineKey+":"+nodeKey)
			continue
		}
		knownNode := false
		for _, node := range workflowFieldRecords(findWorkflowDefinitionPipeline(pipelines, pipelineKey)["nodes"]) {
			if workflowDefinitionText(node["node_key"]) == nodeKey {
				knownNode = true
				break
			}
		}
		if !knownNode {
			addWorkflowDefinitionIssue(blockers, "NODE_MISSING", "Pipeline_Steps tham chiếu node_key không tồn tại hoặc Node không hợp lệ", "pipeline_steps", 0, pipelineKey+":"+nodeKey)
		}
	}
	_ = warnings
	return pipelines
}

func normalizeWorkflowDefinitionNodes(pipelineKey string, rows []Record, stepsByNode map[string][]Record, engineVersion int, catalog workflowDefinitionCatalog, importedForms map[string]Record, blockers *[]Record) []Record {
	nodes := []Record{}
	seen := map[string]bool{}
	for index, raw := range rows {
		nodeKey := workflowDefinitionText(raw["node_key"])
		if !validWorkflowDefinitionPackageKey(nodeKey) || seen[nodeKey] {
			addWorkflowDefinitionIssue(blockers, "NODE_KEY_INVALID", "node_key thiếu, sai định dạng hoặc bị trùng", "pipeline_nodes", index+2, pipelineKey+":"+nodeKey)
			continue
		}
		seen[nodeKey] = true
		name := workflowDefinitionText(raw["name"])
		if name == "" {
			addWorkflowDefinitionIssue(blockers, "NODE_NAME_MISSING", "Tên Node là bắt buộc", "pipeline_nodes", index+2, pipelineKey+":"+nodeKey)
			continue
		}
		roles, ok := workflowDefinitionRoleLabels(raw["responsible_roles_json"], catalog, blockers, "pipeline_nodes", pipelineKey+":"+nodeKey)
		permissions, permissionsOK := workflowDefinitionPermissionKeys(raw["permissions_json"], catalog, blockers, "pipeline_nodes", pipelineKey+":"+nodeKey)
		requiredPermission := workflowDefinitionText(raw["required_permission"])
		if requiredPermission != "" {
			if _, exists := catalog.PermissionByKey[requiredPermission]; !exists {
				addWorkflowDefinitionIssue(blockers, "PERMISSION_NOT_FOUND", "Quyền bắt buộc của Node không tồn tại", "pipeline_nodes", index+2, pipelineKey+":"+nodeKey)
				permissionsOK = false
			}
		}
		entryConditions, entryErr := workflowDefinitionJSONRecord(raw["entry_conditions_json"])
		exitConditions, exitErr := workflowDefinitionJSONRecord(raw["exit_conditions_json"])
		approvalPolicy, policyErr := workflowDefinitionJSONRecord(raw["approval_policy_json"])
		if entryErr != nil || exitErr != nil || policyErr != nil {
			addWorkflowDefinitionIssue(blockers, "NODE_JSON_INVALID", "JSON điều kiện hoặc approval policy của Node không hợp lệ", "pipeline_nodes", index+2, pipelineKey+":"+nodeKey)
			continue
		}
		approvalPolicy = workflowDefinitionTranslateApprovalRoles(approvalPolicy, catalog, blockers, pipelineKey+":"+nodeKey)
		approvalPolicy, policyErr = normalizeApprovalPolicyValue(approvalPolicy)
		if policyErr != nil {
			addWorkflowDefinitionIssue(blockers, "APPROVAL_POLICY_INVALID", policyErr.Error(), "pipeline_nodes", index+2, pipelineKey+":"+nodeKey)
			continue
		}
		node := Record{
			"node_key": nodeKey, "business_code": workflowDefinitionText(raw["business_code"]), "name": name,
			"description": workflowDefinitionText(raw["description"]), "type": workflowDefinitionText(raw["type"]),
			"position_x": workflowDefinitionInt(raw["position_x"], index*280), "position_y": workflowDefinitionInt(raw["position_y"], 260),
			"order_index": workflowDefinitionInt(raw["order_index"], index+1), "responsible_role_ids": roles,
			"required_permission": requiredPermission, "permissions": permissions, "entry_conditions": entryConditions,
			"exit_conditions": exitConditions, "approval_rule": workflowDefinitionText(raw["approval_rule"]),
			"approval_policy_json": approvalPolicy, "rework_enabled": workflowDefinitionBool(raw["rework_enabled"]),
			"replanning_enabled": workflowDefinitionBool(raw["replanning_enabled"]), "fail_path": workflowDefinitionText(raw["fail_path"]),
			"status": defaultWorkflowDefinitionText(raw["status"], "active"),
		}
		if !ok || !permissionsOK {
			continue
		}
		if err := prepareBusinessCode(node, engineVersion >= 3); err != nil {
			addWorkflowDefinitionIssue(blockers, "NODE_BUSINESS_CODE_INVALID", err.Error(), "pipeline_nodes", index+2, pipelineKey+":"+nodeKey)
			continue
		}
		compound := pipelineKey + "\x00" + nodeKey
		node["steps"] = normalizeWorkflowDefinitionSteps(pipelineKey, nodeKey, stepsByNode[compound], engineVersion, catalog, importedForms, blockers)
		nodes = append(nodes, node)
	}
	return nodes
}

func normalizeWorkflowDefinitionSteps(pipelineKey, nodeKey string, rows []Record, engineVersion int, catalog workflowDefinitionCatalog, importedForms map[string]Record, blockers *[]Record) []Record {
	steps := []Record{}
	seen := map[string]bool{}
	for index, raw := range rows {
		stepKey := workflowDefinitionText(raw["step_key"])
		entityKey := pipelineKey + ":" + nodeKey + ":" + stepKey
		if !validWorkflowDefinitionPackageKey(stepKey) || seen[stepKey] {
			addWorkflowDefinitionIssue(blockers, "STEP_KEY_INVALID", "step_key thiếu, sai định dạng hoặc bị trùng", "pipeline_steps", index+2, entityKey)
			continue
		}
		seen[stepKey] = true
		name := workflowDefinitionText(raw["name"])
		if name == "" {
			addWorkflowDefinitionIssue(blockers, "STEP_NAME_MISSING", "Tên Step là bắt buộc", "pipeline_steps", index+2, entityKey)
			continue
		}
		roles, rolesOK := workflowDefinitionRoleLabels(raw["required_roles_json"], catalog, blockers, "pipeline_steps", entityKey)
		requiredPermission := workflowDefinitionText(raw["required_permission"])
		if requiredPermission != "" {
			if _, exists := catalog.PermissionByKey[requiredPermission]; !exists {
				addWorkflowDefinitionIssue(blockers, "PERMISSION_NOT_FOUND", "Quyền bắt buộc của Step không tồn tại", "pipeline_steps", index+2, entityKey)
				rolesOK = false
			}
		}
		formRef := workflowDefinitionText(raw["form_ref"])
		formRequired := workflowDefinitionBool(raw["form_required"])
		formVersionID := ""
		if formRef != "" {
			if _, exists := importedForms[formRef]; !exists {
				if existing, exists := catalog.PublishedFormByRef[strings.ToLower(formRef)]; exists {
					formVersionID = cleanID(existing["version_id"])
				} else {
					addWorkflowDefinitionIssue(blockers, "FORM_REFERENCE_UNRESOLVED", "Step tham chiếu Form không có trong bundle và không khớp version published hiện tại", "pipeline_steps", index+2, entityKey)
					rolesOK = false
				}
			}
		} else if formRequired {
			addWorkflowDefinitionIssue(blockers, "FORM_REFERENCE_REQUIRED", "Step bắt buộc Form cần form_ref", "pipeline_steps", index+2, entityKey)
			rolesOK = false
		}
		step := Record{
			"step_key": stepKey, "business_code": workflowDefinitionText(raw["business_code"]), "name": name,
			"description": workflowDefinitionText(raw["description"]), "step_type": defaultWorkflowDefinitionText(raw["step_type"], "task"),
			"required_role_ids": roles, "required_permission": requiredPermission,
			"checklist_required": workflowDefinitionBool(raw["checklist_required"]), "document_required": workflowDefinitionBool(raw["document_required"]),
			"photo_required": workflowDefinitionBool(raw["photo_required"]), "form_ref": formRef, "form_version_id": formVersionID,
			"form_required": formRequired, "surface_kind": workflowDefinitionText(raw["surface_kind"]),
			"domain_action_key": workflowDefinitionText(raw["domain_action_key"]), "order_index": workflowDefinitionInt(raw["order_index"], index+1),
		}
		if step["surface_kind"] == "" && engineVersion >= 2 {
			step["surface_kind"] = defaultStepSurfaceKind(fmt.Sprint(step["step_type"]), formRequired)
		}
		if surfaceKind := workflowDefinitionText(step["surface_kind"]); surfaceKind != "" && !workflowDefinitionSurfaceKinds[surfaceKind] {
			addWorkflowDefinitionIssue(blockers, "STEP_SURFACE_INVALID", "surface_kind của Step không được hỗ trợ", "pipeline_steps", index+2, entityKey)
			continue
		}
		if !rolesOK {
			continue
		}
		if err := prepareBusinessCode(step, engineVersion >= 3); err != nil {
			addWorkflowDefinitionIssue(blockers, "STEP_BUSINESS_CODE_INVALID", err.Error(), "pipeline_steps", index+2, entityKey)
			continue
		}
		steps = append(steps, step)
	}
	return steps
}

func normalizeWorkflowDefinitionEdges(pipelineKey string, rows, nodes []Record, catalog workflowDefinitionCatalog, importedForms map[string]Record, blockers *[]Record) []Record {
	knownNodes := map[string]bool{}
	nodesByKey := map[string]Record{}
	for _, node := range nodes {
		nodeKey := workflowDefinitionText(node["node_key"])
		knownNodes[nodeKey] = true
		nodesByKey[nodeKey] = node
	}
	edges := []Record{}
	seen := map[string]bool{}
	for index, raw := range rows {
		edgeKey := workflowDefinitionText(raw["edge_key"])
		entityKey := pipelineKey + ":" + edgeKey
		if !validWorkflowDefinitionPackageKey(edgeKey) || seen[edgeKey] {
			addWorkflowDefinitionIssue(blockers, "EDGE_KEY_INVALID", "edge_key thiếu, sai định dạng hoặc bị trùng", "pipeline_edges", index+2, entityKey)
			continue
		}
		seen[edgeKey] = true
		fromKey := workflowDefinitionText(raw["from_node_key"])
		toKey := workflowDefinitionText(raw["to_node_key"])
		if !knownNodes[fromKey] || !knownNodes[toKey] {
			addWorkflowDefinitionIssue(blockers, "EDGE_NODE_UNRESOLVED", "Connector tham chiếu Node không tồn tại", "pipeline_edges", index+2, entityKey)
			continue
		}
		config, err := workflowDefinitionJSONRecord(raw["condition_config_json"])
		if err != nil {
			addWorkflowDefinitionIssue(blockers, "EDGE_CONFIG_INVALID", "condition_config_json phải là JSON object", "pipeline_edges", index+2, entityKey)
			continue
		}
		edge := Record{
			"edge_key": edgeKey, "from_node_key": fromKey, "to_node_key": toKey,
			"from_node_id":   uuid.NewSHA1(uuid.NameSpaceOID, []byte(pipelineKey+":"+fromKey)).String(),
			"to_node_id":     uuid.NewSHA1(uuid.NameSpaceOID, []byte(pipelineKey+":"+toKey)).String(),
			"condition_type": defaultWorkflowDefinitionText(raw["condition_type"], "always"), "condition_config": config,
			"label": workflowDefinitionText(raw["label"]), "priority": workflowDefinitionInt(raw["priority"], 100),
		}
		if err := prepareTemplateEdgeBody(edge, false); err != nil {
			addWorkflowDefinitionIssue(blockers, "EDGE_INVALID", err.Error(), "pipeline_edges", index+2, entityKey)
			continue
		}
		if workflowDefinitionText(edge["condition_type"]) == "workflow_form_answer_equals" {
			fieldKey := workflowDefinitionText(config["field_key"])
			if !workflowDefinitionNodeHasFormField(nodesByKey[fromKey], fieldKey, catalog, importedForms) {
				addWorkflowDefinitionIssue(blockers, "EDGE_FORM_FIELD_UNRESOLVED", "field_key của Connector không thuộc Form được gắn vào Node nguồn", "pipeline_edges", index+2, entityKey)
				continue
			}
		}
		edges = append(edges, edge)
	}
	return edges
}

func workflowDefinitionNodeHasFormField(node Record, fieldKey string, catalog workflowDefinitionCatalog, importedForms map[string]Record) bool {
	if fieldKey == "" {
		return false
	}
	for _, step := range workflowFieldRecords(node["steps"]) {
		formRef := workflowDefinitionText(step["form_ref"])
		if imported, exists := importedForms[formRef]; exists {
			for _, field := range workflowFieldRecords(imported["fields"]) {
				if workflowDefinitionText(field["field_key"]) == fieldKey {
					return true
				}
			}
			continue
		}
		if existing, exists := catalog.PublishedFormByRef[strings.ToLower(formRef)]; exists {
			for _, existingFieldKey := range stringSlice(existing["field_keys"]) {
				if existingFieldKey == fieldKey {
					return true
				}
			}
		}
	}
	return false
}

func findWorkflowDefinitionPipeline(pipelines []Record, pipelineKey string) Record {
	for _, pipeline := range pipelines {
		if workflowDefinitionText(pipeline["pipeline_key"]) == pipelineKey {
			return pipeline
		}
	}
	return Record{}
}

func workflowFormDefinitionHash(schema, validation any, fields []Record) (string, error) {
	canonicalFields := make([]Record, 0, len(fields))
	for _, field := range fields {
		canonicalFields = append(canonicalFields, Record{
			"field_key": workflowDefinitionText(field["field_key"]), "label": workflowDefinitionText(field["label"]),
			"field_type": workflowDefinitionText(field["field_type"]), "required": boolValue(field["required"]),
			"options_json":    defaultWorkflowDefinitionJSON(field["options_json"], []any{}),
			"validation_json": defaultWorkflowDefinitionJSON(field["validation_json"], Record{}),
			"help_text":       workflowDefinitionText(field["help_text"]), "order_index": workflowDefinitionInt(field["order_index"], 0),
			"visibility_condition_json": defaultWorkflowDefinitionJSON(field["visibility_condition_json"], Record{}),
		})
	}
	sort.SliceStable(canonicalFields, func(i, j int) bool {
		leftOrder := workflowDefinitionInt(canonicalFields[i]["order_index"], 0)
		rightOrder := workflowDefinitionInt(canonicalFields[j]["order_index"], 0)
		if leftOrder == rightOrder {
			return workflowDefinitionText(canonicalFields[i]["field_key"]) < workflowDefinitionText(canonicalFields[j]["field_key"])
		}
		return leftOrder < rightOrder
	})
	raw, err := json.Marshal(Record{
		"schema_json":     defaultWorkflowDefinitionJSON(schema, Record{}),
		"validation_json": defaultWorkflowDefinitionJSON(validation, Record{}),
		"fields":          canonicalFields,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func workflowFormDefinitionHashTx(ctx context.Context, tx *sql.Tx, versionID string) (string, error) {
	versions, err := queryTx(ctx, tx, `SELECT schema_json,validation_json FROM workflow_form_versions WHERE id=$1`, versionID)
	if err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", sql.ErrNoRows
	}
	fields, err := queryTx(ctx, tx, `
		SELECT field_key,label,field_type,required,options_json,validation_json,COALESCE(help_text,'') AS help_text,order_index,visibility_condition_json
		FROM workflow_form_fields WHERE form_version_id=$1 ORDER BY order_index,created_at,id
	`, versionID)
	if err != nil {
		return "", err
	}
	return workflowFormDefinitionHash(versions[0]["schema_json"], versions[0]["validation_json"], fields)
}

func appendWorkflowDefinitionGraphIssues(pipelineKey string, nodes, edges []Record, blockers *[]Record) {
	graphNodes := make([]Record, 0, len(nodes))
	for _, node := range nodes {
		graphNodes = append(graphNodes, Record{"id": uuid.NewSHA1(uuid.NameSpaceOID, []byte(pipelineKey+":"+workflowDefinitionText(node["node_key"]))).String()})
	}
	for _, issue := range validateTemplateGraph(graphNodes, edges) {
		addWorkflowDefinitionIssue(blockers, workflowDefinitionText(issue["code"]), workflowDefinitionText(issue["message"]), "pipeline_edges", 0, pipelineKey)
	}
}

func workflowDefinitionRoleLabels(value any, catalog workflowDefinitionCatalog, blockers *[]Record, sheet, entityKey string) ([]string, bool) {
	items, err := workflowDefinitionJSONStringSlice(value)
	if err != nil {
		addWorkflowDefinitionIssue(blockers, "ROLE_REFERENCE_INVALID", "Danh sách role phải là JSON array các role name", sheet, 0, entityKey)
		return nil, false
	}
	labels := make([]string, 0, len(items))
	ok := true
	seen := map[string]bool{}
	for _, item := range items {
		role, exists := catalog.RoleByName[strings.ToLower(item)]
		if !exists {
			addWorkflowDefinitionIssue(blockers, "ROLE_NOT_FOUND", "Role name không tồn tại: "+item, sheet, 0, entityKey)
			ok = false
			continue
		}
		if boolValue(role["external_only"]) {
			addWorkflowDefinitionIssue(blockers, "EXTERNAL_ROLE_NOT_ASSIGNABLE", "Role đối tác ngoài App không thể gán vào workflow: "+item, sheet, 0, entityKey)
			ok = false
			continue
		}
		label := workflowDefinitionText(role["display_name"])
		if !seen[strings.ToLower(label)] {
			seen[strings.ToLower(label)] = true
			labels = append(labels, label)
		}
	}
	return labels, ok
}

func workflowDefinitionPermissionKeys(value any, catalog workflowDefinitionCatalog, blockers *[]Record, sheet, entityKey string) ([]string, bool) {
	items, err := workflowDefinitionJSONStringSlice(value)
	if err != nil {
		addWorkflowDefinitionIssue(blockers, "PERMISSION_REFERENCE_INVALID", "Danh sách permission phải là JSON array các permission key", sheet, 0, entityKey)
		return nil, false
	}
	ok := true
	for _, item := range items {
		if _, exists := catalog.PermissionByKey[item]; !exists {
			addWorkflowDefinitionIssue(blockers, "PERMISSION_NOT_FOUND", "Permission key không tồn tại: "+item, sheet, 0, entityKey)
			ok = false
		}
	}
	return items, ok
}

func workflowDefinitionTranslateApprovalRoles(value Record, catalog workflowDefinitionCatalog, blockers *[]Record, entityKey string) Record {
	raw, _ := json.Marshal(value)
	policy := ApprovalPolicy{}
	if err := json.Unmarshal(raw, &policy); err != nil {
		return value
	}
	translated, _ := workflowDefinitionRoleLabels(policy.Roles, catalog, blockers, "pipeline_nodes", entityKey)
	policy.Roles = translated
	for index := range policy.Stages {
		translated, _ := workflowDefinitionRoleLabels(policy.Stages[index].Roles, catalog, blockers, "pipeline_nodes", entityKey)
		policy.Stages[index].Roles = translated
	}
	raw, _ = json.Marshal(policy)
	out := Record{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func workflowDefinitionJSONStringSlice(value any) ([]string, error) {
	parsed, err := workflowDefinitionJSONValue(value, []any{})
	if err != nil {
		return nil, err
	}
	switch parsed.(type) {
	case []any, []string:
		return stringSlice(parsed), nil
	default:
		return nil, fmt.Errorf("expected JSON array")
	}
}

func workflowDefinitionJSONRecord(value any) (Record, error) {
	parsed, err := workflowDefinitionJSONValue(value, Record{})
	if err != nil {
		return nil, err
	}
	switch parsed.(type) {
	case Record, map[string]any:
		return recordFromAny(parsed), nil
	default:
		return nil, fmt.Errorf("expected JSON object")
	}
}

func workflowDefinitionJSONValue(value, empty any) (any, error) {
	if value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" || fmt.Sprint(value) == "<nil>" {
		return empty, nil
	}
	switch typed := value.(type) {
	case Record, map[string]any, []any, []string:
		return typed, nil
	case string:
		var parsed any
		if err := json.Unmarshal([]byte(strings.TrimSpace(typed)), &parsed); err != nil {
			return nil, err
		}
		return parsed, nil
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		var parsed any
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, err
		}
		return parsed, nil
	}
}

func defaultWorkflowDefinitionJSON(value, empty any) any {
	parsed, err := workflowDefinitionJSONValue(value, empty)
	if err != nil {
		return empty
	}
	return parsed
}

func workflowDefinitionText(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func defaultWorkflowDefinitionText(value any, fallback string) string {
	if text := workflowDefinitionText(value); text != "" {
		return text
	}
	return fallback
}

func workflowDefinitionInt(value any, fallback int) int {
	if value == nil || workflowDefinitionText(value) == "" {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	}
	parsed, err := strconv.Atoi(workflowDefinitionText(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func workflowDefinitionBool(value any) bool {
	if text := strings.ToLower(workflowDefinitionText(value)); text != "" {
		return statusIn(text, "true", "1", "yes", "y", "có", "x")
	}
	return boolValue(value)
}

func validWorkflowDefinitionPackageKey(key string) bool {
	return workflowDefinitionPackageKeyPattern.MatchString(strings.TrimSpace(key))
}

func groupWorkflowDefinitionRows(rows []Record, key string) map[string][]Record {
	result := map[string][]Record{}
	for _, row := range rows {
		value := workflowDefinitionText(row[key])
		result[value] = append(result[value], row)
	}
	return result
}

func normalizedChildRows(parents []Record, key string) []Record {
	rows := []Record{}
	for _, parent := range parents {
		for _, child := range workflowFieldRecords(parent[key]) {
			copy := Record{}
			for field, value := range child {
				if field != "steps" {
					copy[field] = value
				}
			}
			copy["pipeline_key"] = parent["pipeline_key"]
			rows = append(rows, copy)
		}
	}
	return rows
}

func normalizedNestedChildRows(parents []Record, parentKey, childKey string) []Record {
	rows := []Record{}
	for _, parent := range parents {
		for _, middle := range workflowFieldRecords(parent[parentKey]) {
			for _, child := range workflowFieldRecords(middle[childKey]) {
				copy := Record{}
				for field, value := range child {
					copy[field] = value
				}
				copy["pipeline_key"] = parent["pipeline_key"]
				copy["node_key"] = middle["node_key"]
				rows = append(rows, copy)
			}
		}
	}
	return rows
}

func addWorkflowDefinitionIssue(target *[]Record, code, message, sheet string, row int, entityKey string) {
	issue := Record{"code": code, "message": message, "sheet": sheet, "entity_key": entityKey}
	if row > 0 {
		issue["row_number"] = row
	}
	*target = append(*target, issue)
}
