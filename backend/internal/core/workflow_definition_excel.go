package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	workflowDefinitionWorkbookVersion  = "1"
	workflowDefinitionMaximumFileBytes = int64(10 * 1024 * 1024)
	workflowDefinitionMaximumPipelines = 25
	workflowDefinitionMaximumForms     = 100
	workflowDefinitionMaximumSheetRows = 10000
)

var workflowDefinitionPackageKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:@-]{0,119}$`)

var workflowDefinitionSurfaceKinds = map[string]bool{
	"workflow_form": true, "evidence": true, "approval": true, "incoming_acceptance": true,
	"material_snapshot": true, "surplus_handling": true, "outgoing_acceptance": true,
	"owner_client_reconciliation": true, "subcontractor_reconciliation": true, "contract": true,
	"quality": true, "finance": true, "handover": true, "generic": true,
}

type WorkflowDefinitionImportPreviewRequest struct {
	Scope           string   `json:"scope"`
	WorkbookVersion string   `json:"workbook_version"`
	PackageVersion  string   `json:"package_version"`
	SourceFormat    string   `json:"source_format"`
	SourceFilename  string   `json:"source_filename"`
	SourceSizeBytes int64    `json:"source_size_bytes"`
	Pipelines       []Record `json:"pipelines"`
	PipelineNodes   []Record `json:"pipeline_nodes"`
	PipelineSteps   []Record `json:"pipeline_steps"`
	PipelineEdges   []Record `json:"pipeline_edges"`
	FormTemplates   []Record `json:"form_templates"`
	FormVersions    []Record `json:"form_versions"`
	FormFields      []Record `json:"form_fields"`
}

type WorkflowDefinitionImportExecuteRequest struct {
	WorkflowDefinitionImportPreviewRequest
	PreviewHash    string `json:"preview_hash"`
	IdempotencyKey string `json:"idempotency_key"`
}

type WorkflowDefinitionExportRequest struct {
	Format              string
	Scope               string
	IDs                 []string
	IncludeDependencies bool
}

type workflowDefinitionPackage struct {
	Scope         string   `json:"scope"`
	Pipelines     []Record `json:"pipelines"`
	PipelineNodes []Record `json:"pipeline_nodes"`
	PipelineSteps []Record `json:"pipeline_steps"`
	PipelineEdges []Record `json:"pipeline_edges"`
	Forms         []Record `json:"forms"`
}

type workflowDefinitionCatalog struct {
	Permissions           []Record
	Roles                 []Record
	Forms                 []Record
	PipelineFamilies      []Record
	PermissionByKey       map[string]Record
	RoleByName            map[string]Record
	RoleNameByDisplayName map[string]string
	FormByCode            map[string]Record
	PublishedFormByRef    map[string]Record
	PipelineFamilyByID    map[string]Record
	Fingerprint           string
}

type workflowDefinitionValidation struct {
	Package      workflowDefinitionPackage
	Preview      Record
	Hash         string
	Catalog      workflowDefinitionCatalog
	SourceFormat string
}

func (s *Service) WorkflowDefinitionExcelTemplate(ctx context.Context, scope string) (tabularExportResult, error) {
	return s.WorkflowDefinitionImportTemplate(ctx, scope, dataPackageFormatXLSX)
}

func (s *Service) WorkflowDefinitionImportTemplate(ctx context.Context, scope, format string) (tabularExportResult, error) {
	scope, err := normalizeWorkflowDefinitionScope(scope)
	if err != nil {
		return tabularExportResult{}, err
	}
	catalog, err := s.repo.WorkflowDefinitionCatalog(ctx)
	if err != nil {
		return tabularExportResult{}, err
	}
	packageData := workflowDefinitionPackage{Scope: scope}
	if format, err = normalizeDataPackageFormat(format); err != nil {
		return tabularExportResult{}, err
	}
	if format == dataPackageFormatJSON {
		return generateWorkflowDefinitionJSONPackage(packageData, catalog, time.Now(), "mau")
	}
	return generateWorkflowDefinitionWorkbook(packageData, catalog, time.Now(), "mau")
}

func (s *Service) PreviewWorkflowDefinitionExcelImport(ctx context.Context, req WorkflowDefinitionImportPreviewRequest) (Record, error) {
	validation, err := s.validateWorkflowDefinitionImport(ctx, req)
	if err != nil {
		return nil, err
	}
	return validation.Preview, nil
}

func (s *Service) ExecuteWorkflowDefinitionExcelImport(ctx context.Context, actorID, actorName string, req WorkflowDefinitionImportExecuteRequest) (Record, error) {
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if _, err := uuid.Parse(idempotencyKey); err != nil {
		return nil, validationError("Mã chống tạo trùng không hợp lệ", Record{"field": "idempotency_key"})
	}
	previewHash := strings.TrimSpace(req.PreviewHash)
	if stored, found, err := s.repo.WorkflowDefinitionImportResult(ctx, actorID, idempotencyKey, previewHash); err != nil {
		return nil, err
	} else if found {
		stored["idempotent_replay"] = true
		return stored, nil
	}
	validation, err := s.validateWorkflowDefinitionImport(ctx, req.WorkflowDefinitionImportPreviewRequest)
	if err != nil {
		return nil, err
	}
	if number(validation.Preview["blocker_count"]) > 0 {
		return nil, validationError("Gói định nghĩa workflow còn điểm chặn", validation.Preview)
	}
	if previewHash == "" || previewHash != validation.Hash {
		return nil, conflictError("Dữ liệu hoặc danh mục tham chiếu đã thay đổi sau bước xem trước. Vui lòng xem trước lại", Record{"field": "preview_hash"})
	}
	return s.repo.ExecuteWorkflowDefinitionImport(ctx, actorID, actorName, idempotencyKey, validation.SourceFormat, strings.TrimSpace(req.SourceFilename), req.SourceSizeBytes, validation.Hash, validation.Catalog.Fingerprint, validation.Package)
}

func (s *Service) ExportWorkflowDefinitionsExcel(ctx context.Context, actorID, actorName string, req WorkflowDefinitionExportRequest) (tabularExportResult, error) {
	format, err := normalizeDataPackageFormat(req.Format)
	if err != nil {
		return tabularExportResult{}, err
	}
	scope, err := normalizeWorkflowDefinitionExportScope(req.Scope)
	if err != nil {
		return tabularExportResult{}, err
	}
	if len(req.IDs) == 0 || len(req.IDs) > workflowDefinitionMaximumPipelines {
		return tabularExportResult{}, validationError("Cần chọn từ 1 đến 25 định nghĩa để xuất", Record{"field": "ids", "maximum": workflowDefinitionMaximumPipelines})
	}
	seen := map[string]bool{}
	for index, id := range req.IDs {
		id = strings.TrimSpace(id)
		if _, err := uuid.Parse(id); err != nil {
			return tabularExportResult{}, validationError("ID định nghĩa xuất không hợp lệ", Record{"id": id})
		}
		if seen[id] {
			return tabularExportResult{}, validationError("Danh sách định nghĩa xuất bị trùng", Record{"id": id})
		}
		seen[id] = true
		req.IDs[index] = id
	}
	catalog, err := s.repo.WorkflowDefinitionCatalog(ctx)
	if err != nil {
		return tabularExportResult{}, err
	}
	packageData := workflowDefinitionPackage{Scope: scope}
	if scope == "form" {
		for _, versionID := range req.IDs {
			detail, err := s.repo.WorkflowFormVersionDetail(ctx, versionID)
			if err != nil {
				return tabularExportResult{}, err
			}
			appendWorkflowDefinitionForm(&packageData, detail)
		}
	} else {
		formIDs := map[string]bool{}
		for _, templateID := range req.IDs {
			detail, err := s.repo.GetTemplateDetail(ctx, templateID)
			if err != nil {
				return tabularExportResult{}, err
			}
			appendWorkflowDefinitionPipeline(&packageData, detail, catalog, req.IncludeDependencies, formIDs)
		}
		if req.IncludeDependencies {
			packageData.Scope = "bundle"
			ids := make([]string, 0, len(formIDs))
			for id := range formIDs {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			for _, id := range ids {
				detail, err := s.repo.WorkflowFormVersionDetail(ctx, id)
				if err != nil {
					return tabularExportResult{}, err
				}
				appendWorkflowDefinitionForm(&packageData, detail)
			}
		}
	}
	var result tabularExportResult
	if format == dataPackageFormatJSON {
		result, err = generateWorkflowDefinitionJSONPackage(packageData, catalog, time.Now(), "xuat")
	} else {
		result, err = generateWorkflowDefinitionWorkbook(packageData, catalog, time.Now(), "xuat")
	}
	if err != nil {
		return tabularExportResult{}, err
	}
	s.repo.AddAuditBestEffort(ctx, "", actorID, actorName, "workflow_definitions_exported", "workflow_definition_export", "", "Xuất định nghĩa workflow", Record{
		"source_format": format,
		"scope":         scope, "selected_count": len(req.IDs), "include_dependencies": req.IncludeDependencies,
		"pipeline_count": len(packageData.Pipelines), "form_count": len(packageData.Forms),
	})
	return result, nil
}

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

func (r *Repository) WorkflowDefinitionCatalog(ctx context.Context) (workflowDefinitionCatalog, error) {
	permissions, err := r.Query(ctx, `SELECT key,name,module FROM permissions ORDER BY key`)
	if err != nil {
		return workflowDefinitionCatalog{}, err
	}
	roles, err := r.Query(ctx, `SELECT name,display_name,external_only FROM roles ORDER BY name`)
	if err != nil {
		return workflowDefinitionCatalog{}, err
	}
	forms, err := r.Query(ctx, `
		SELECT template.id::text AS template_id,template.code,template.name,template.status AS template_status,
			COUNT(version.id) FILTER (WHERE version.status='draft')::int AS draft_count,
			COALESCE(MAX(version.version_no),0)::int AS max_version_no
		FROM workflow_form_templates template
		LEFT JOIN workflow_form_versions version ON version.template_id=template.id
		GROUP BY template.id
		ORDER BY template.code
	`)
	if err != nil {
		return workflowDefinitionCatalog{}, err
	}
	formVersions, err := r.Query(ctx, `
		SELECT version.id::text AS version_id,template.id::text AS template_id,template.code,template.name,
			template.status AS template_status,version.version_no,version.status AS version_status,
			COALESCE(version.definition_hash,'') AS definition_hash,COUNT(field.id)::int AS field_count,
			COALESCE(jsonb_agg(field.field_key ORDER BY field.order_index,field.created_at,field.id)
				FILTER (WHERE field.id IS NOT NULL),'[]'::jsonb) AS field_keys
		FROM workflow_form_versions version
		JOIN workflow_form_templates template ON template.id=version.template_id
		LEFT JOIN workflow_form_fields field ON field.form_version_id=version.id
		GROUP BY version.id,template.id
		HAVING COUNT(field.id)>0
		ORDER BY template.code,version.version_no
	`)
	if err != nil {
		return workflowDefinitionCatalog{}, err
	}
	families, err := r.Query(ctx, `
		SELECT COALESCE(template_family_id,id)::text AS family_id,
			(MAX(name) FILTER (WHERE revision_no=(SELECT MAX(inner_template.revision_no) FROM pipeline_templates inner_template WHERE COALESCE(inner_template.template_family_id,inner_template.id)=COALESCE(template.template_family_id,template.id) AND inner_template.status <> 'deleted'))) AS name,
			COUNT(*) FILTER (WHERE status='draft')::int AS draft_count,
			MAX(revision_no)::int AS max_revision_no
		FROM pipeline_templates template
		WHERE status <> 'deleted'
		GROUP BY COALESCE(template_family_id,id)
		ORDER BY family_id
	`)
	if err != nil {
		return workflowDefinitionCatalog{}, err
	}
	catalog := workflowDefinitionCatalog{
		Permissions: permissions, Roles: roles, Forms: formVersions, PipelineFamilies: families,
		PermissionByKey: map[string]Record{}, RoleByName: map[string]Record{}, RoleNameByDisplayName: map[string]string{},
		FormByCode: map[string]Record{}, PublishedFormByRef: map[string]Record{}, PipelineFamilyByID: map[string]Record{},
	}
	for _, row := range permissions {
		catalog.PermissionByKey[workflowDefinitionText(row["key"])] = row
	}
	for _, row := range roles {
		name := strings.ToLower(workflowDefinitionText(row["name"]))
		catalog.RoleByName[name] = row
		catalog.RoleNameByDisplayName[strings.ToLower(workflowDefinitionText(row["display_name"]))] = workflowDefinitionText(row["name"])
	}
	for _, row := range forms {
		catalog.FormByCode[strings.ToLower(workflowDefinitionText(row["code"]))] = row
	}
	for _, row := range formVersions {
		if workflowDefinitionText(row["template_status"]) != "active" || workflowDefinitionText(row["version_status"]) != "published" {
			continue
		}
		ref := fmt.Sprintf("%s@v%d", workflowDefinitionText(row["code"]), workflowDefinitionInt(row["version_no"], 0))
		row["form_ref"] = ref
		catalog.PublishedFormByRef[strings.ToLower(ref)] = row
	}
	for _, row := range families {
		catalog.PipelineFamilyByID[cleanID(row["family_id"])] = row
	}
	raw, err := json.Marshal(Record{"permissions": permissions, "roles": roles, "forms": forms, "form_versions": formVersions, "pipeline_families": families})
	if err != nil {
		return workflowDefinitionCatalog{}, err
	}
	sum := sha256.Sum256(raw)
	catalog.Fingerprint = hex.EncodeToString(sum[:])
	return catalog, nil
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

func (r *Repository) WorkflowDefinitionImportResult(ctx context.Context, actorID, idempotencyKey, previewHash string) (Record, bool, error) {
	var storedActorID sql.NullString
	var storedHash string
	var sourceFormat string
	var storedJSON []byte
	err := r.db.QueryRowContext(ctx, `SELECT actor_id::text,preview_hash,source_format,result_json FROM workflow_definition_imports WHERE idempotency_key=$1`, idempotencyKey).Scan(&storedActorID, &storedHash, &sourceFormat, &storedJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !storedActorID.Valid || storedActorID.String != actorID || previewHash == "" || storedHash != previewHash {
		return nil, false, conflictError("Mã chống tạo trùng đã được dùng cho một gói workflow khác", Record{"field": "idempotency_key"})
	}
	result := Record{}
	if err := json.Unmarshal(storedJSON, &result); err != nil {
		return nil, false, err
	}
	result["source_format"] = sourceFormat
	return result, true, nil
}

func (r *Repository) ExecuteWorkflowDefinitionImport(ctx context.Context, actorID, actorName, idempotencyKey, sourceFormat, sourceFilename string, sourceSizeBytes int64, previewHash, referenceFingerprint string, packageData workflowDefinitionPackage) (result Record, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, idempotencyKey); err != nil {
		return nil, err
	}
	var storedActorID sql.NullString
	var storedHash string
	var storedJSON []byte
	if err := tx.QueryRowContext(ctx, `SELECT actor_id::text,preview_hash,result_json FROM workflow_definition_imports WHERE idempotency_key=$1`, idempotencyKey).Scan(&storedActorID, &storedHash, &storedJSON); err == nil {
		if !storedActorID.Valid || storedActorID.String != actorID || storedHash != previewHash {
			return nil, conflictError("Mã chống tạo trùng đã được dùng cho một gói workflow khác", Record{"field": "idempotency_key"})
		}
		if err := json.Unmarshal(storedJSON, &result); err != nil {
			return nil, err
		}
		result["idempotent_replay"] = true
		return result, tx.Commit()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	createdForms := []Record{}
	formVersionIDs := map[string]string{}
	for _, form := range packageData.Forms {
		code := workflowDefinitionText(form["template_code"])
		templateID := cleanID(form["target_template_id"])
		if templateID == "" {
			if err := tx.QueryRowContext(ctx, `
				INSERT INTO workflow_form_templates (code,name,description,form_type,status,created_by)
				VALUES ($1,$2,$3,$4,'active',NULLIF($5,'')::uuid)
				RETURNING id::text
			`, code, form["name"], form["description"], form["form_type"], actorID).Scan(&templateID); err != nil {
				return nil, err
			}
		} else {
			var status string
			if err := tx.QueryRowContext(ctx, `SELECT status FROM workflow_form_templates WHERE id=$1 AND lower(code)=lower($2) FOR UPDATE`, templateID, code).Scan(&status); err != nil {
				return nil, conflictError("Form đích đã thay đổi sau bước xem trước", Record{"form_key": form["form_key"]})
			}
			if status != "active" {
				return nil, conflictError("Form đích không còn active", Record{"form_key": form["form_key"]})
			}
		}
		var draftCount, nextVersionNo int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FILTER (WHERE status='draft')::int,COALESCE(MAX(version_no),0)::int+1 FROM workflow_form_versions WHERE template_id=$1`, templateID).Scan(&draftCount, &nextVersionNo); err != nil {
			return nil, err
		}
		if draftCount > 0 {
			return nil, conflictError("Form đích đã có version nháp; import không ghi đè", Record{"form_key": form["form_key"]})
		}
		schemaRaw, _ := json.Marshal(defaultWorkflowDefinitionJSON(form["schema_json"], Record{}))
		validationRaw, _ := json.Marshal(defaultWorkflowDefinitionJSON(form["validation_json"], Record{}))
		versionID := newID()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workflow_form_versions (id,template_id,version_no,status,schema_json,validation_json,definition_hash,created_by)
			VALUES ($1,$2,$3,'draft',$4::jsonb,$5::jsonb,$6,NULLIF($7,'')::uuid)
		`, versionID, templateID, nextVersionNo, string(schemaRaw), string(validationRaw), form["definition_hash"], actorID); err != nil {
			return nil, err
		}
		if err := insertWorkflowFormFieldsTx(ctx, tx, versionID, workflowFieldRecords(form["fields"])); err != nil {
			return nil, err
		}
		if err := addAuditTx(ctx, tx, "", actorID, actorName, "workflow_form_definition_imported", "workflow_form_version", versionID, fmt.Sprintf("Import nháp biểu mẫu quy trình %s v%d", form["name"], nextVersionNo), Record{
			"import_idempotency_key": idempotencyKey, "template_id": templateID, "form_key": form["form_key"], "definition_hash": form["definition_hash"],
		}); err != nil {
			return nil, err
		}
		formKey := workflowDefinitionText(form["form_key"])
		formVersionIDs[formKey] = versionID
		createdForms = append(createdForms, Record{"form_key": formKey, "template_id": templateID, "version_id": versionID, "version_no": nextVersionNo, "status": "draft", "definition_hash": form["definition_hash"]})
	}

	createdPipelines := []Record{}
	for _, pipeline := range packageData.Pipelines {
		targetFamilyID := cleanID(pipeline["target_family_id"])
		templateID := newID()
		revisionNo := 1
		if targetFamilyID == "" {
			targetFamilyID = templateID
		} else {
			familyRows, err := queryTx(ctx, tx, `SELECT id FROM pipeline_templates WHERE COALESCE(template_family_id,id)=$1 AND status <> 'deleted' ORDER BY revision_no LIMIT 1 FOR UPDATE`, targetFamilyID)
			if err != nil {
				return nil, err
			}
			if len(familyRows) == 0 {
				return nil, conflictError("Family Pipeline đích đã thay đổi sau bước xem trước", Record{"pipeline_key": pipeline["pipeline_key"]})
			}
			var draftCount int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FILTER (WHERE status='draft')::int,COALESCE(MAX(revision_no),0)::int+1 FROM pipeline_templates WHERE COALESCE(template_family_id,id)=$1 AND status <> 'deleted'`, targetFamilyID).Scan(&draftCount, &revisionNo); err != nil {
				return nil, err
			}
			if draftCount > 0 {
				return nil, conflictError("Family Pipeline đích đã có revision nháp; import không ghi đè", Record{"pipeline_key": pipeline["pipeline_key"]})
			}
		}
		body := Record{
			"id": templateID, "name": pipeline["name"], "description": pipeline["description"], "project_type": pipeline["project_type"],
			"version": revisionNo, "status": "draft", "created_by": actorID, "template_family_id": targetFamilyID,
			"revision_no": revisionNo, "engine_version": pipeline["engine_version"], "report_config_json": pipeline["report_config_json"],
		}
		if _, err := insertRecord(ctx, tx, "pipeline_templates", templateColumns(), body); err != nil {
			return nil, err
		}
		nodeIDs := map[string]string{}
		for _, node := range workflowFieldRecords(pipeline["nodes"]) {
			nodeBody := Record{}
			for key, value := range node {
				if key != "node_key" && key != "steps" {
					nodeBody[key] = value
				}
			}
			nodeBody["template_id"] = templateID
			createdNode, err := insertRecord(ctx, tx, "pipeline_template_nodes", templateNodeColumns(), nodeBody)
			if err != nil {
				return nil, err
			}
			nodeID := cleanID(createdNode["id"])
			nodeIDs[workflowDefinitionText(node["node_key"])] = nodeID
			for _, step := range workflowFieldRecords(node["steps"]) {
				stepBody := Record{}
				for key, value := range step {
					if key != "step_key" && key != "form_ref" {
						stepBody[key] = value
					}
				}
				stepBody["node_id"] = nodeID
				if importedVersionID := formVersionIDs[workflowDefinitionText(step["form_ref"])]; importedVersionID != "" {
					stepBody["form_version_id"] = importedVersionID
				}
				if _, err := insertRecord(ctx, tx, "pipeline_template_steps", templateStepColumns(), stepBody); err != nil {
					return nil, err
				}
			}
		}
		for _, edge := range workflowFieldRecords(pipeline["edges"]) {
			fromID := nodeIDs[workflowDefinitionText(edge["from_node_key"])]
			toID := nodeIDs[workflowDefinitionText(edge["to_node_key"])]
			if fromID == "" || toID == "" {
				return nil, conflictError("Connector không còn khớp Node sau bước xem trước", Record{"pipeline_key": pipeline["pipeline_key"], "edge_key": edge["edge_key"]})
			}
			if _, err := insertRecord(ctx, tx, "pipeline_template_edges", edgeColumns(), Record{
				"template_id": templateID, "from_node_id": fromID, "to_node_id": toID, "condition_type": edge["condition_type"],
				"condition_config": edge["condition_config"], "label": edge["label"], "priority": edge["priority"],
			}); err != nil {
				return nil, err
			}
		}
		if err := addAuditTx(ctx, tx, "", actorID, actorName, "pipeline_definition_imported", "pipeline_template", templateID, fmt.Sprintf("Import revision nháp Pipeline %s", pipeline["name"]), Record{
			"import_idempotency_key": idempotencyKey, "pipeline_key": pipeline["pipeline_key"], "template_family_id": targetFamilyID, "revision_no": revisionNo, "engine_version": pipeline["engine_version"],
		}); err != nil {
			return nil, err
		}
		createdPipelines = append(createdPipelines, Record{"pipeline_key": pipeline["pipeline_key"], "template_id": templateID, "template_family_id": targetFamilyID, "revision_no": revisionNo, "status": "draft", "engine_version": pipeline["engine_version"]})
	}

	importID := newID()
	result = Record{
		"import_id": importID, "source_format": sourceFormat, "scope": packageData.Scope, "pipeline_count": len(createdPipelines), "form_count": len(createdForms),
		"pipelines": createdPipelines, "forms": createdForms, "status": "draft", "idempotent_replay": false,
	}
	resultRaw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_definition_imports (
			id,idempotency_key,actor_id,scope,source_filename,source_size_bytes,workbook_version,source_format,
			preview_hash,reference_fingerprint,pipeline_count,form_count,result_json
		) VALUES ($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb)
	`, importID, idempotencyKey, actorID, packageData.Scope, sourceFilename, sourceSizeBytes, workflowDefinitionWorkbookVersion, sourceFormat,
		previewHash, referenceFingerprint, len(createdPipelines), len(createdForms), string(resultRaw)); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "workflow_definitions_imported", "workflow_definition_import", importID, "Import gói định nghĩa workflow thành bản nháp", Record{
		"scope": packageData.Scope, "source_format": sourceFormat, "pipeline_count": len(createdPipelines), "form_count": len(createdForms), "preview_hash": previewHash,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

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
		} else {
			result = append(result, label)
		}
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
