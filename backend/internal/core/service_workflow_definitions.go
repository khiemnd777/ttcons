package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
)

func (s *Service) ListTemplates(ctx context.Context) ([]Record, error) {
	return s.repo.Query(ctx, `
		SELECT t.*, COUNT(n.id) AS node_count
		FROM pipeline_templates t
		LEFT JOIN pipeline_template_nodes n ON n.template_id=t.id
		WHERE t.status NOT IN ('deleted','archived')
		GROUP BY t.id
		ORDER BY t.updated_at DESC, t.created_at DESC
	`)
}

func (s *Service) TemplateDetail(ctx context.Context, id string) (Record, error) {
	return s.repo.GetTemplateDetail(ctx, id)
}

func (s *Service) CreateTemplate(ctx context.Context, body Record, actorID string) (Record, error) {
	body["status"] = "draft"
	for _, key := range []string{"id", "template_family_id", "revision_no", "source_template_id", "definition_hash", "published_at", "published_by"} {
		delete(body, key)
	}
	return s.repo.CreateTemplateWithNodes(ctx, body, actorID)
}

func (s *Service) UpdateTemplate(ctx context.Context, id string, body Record) (Record, error) {
	if _, err := s.ensureDraftTemplate(ctx, "template", id); err != nil {
		return nil, err
	}
	for _, key := range []string{"id", "status", "version", "created_by", "template_family_id", "revision_no", "source_template_id", "engine_version", "definition_hash", "published_at", "published_by"} {
		delete(body, key)
	}
	return s.repo.Update(ctx, "pipeline_templates", templateColumns(), id, body)
}

func (s *Service) DeleteTemplate(ctx context.Context, id string) error {
	template, err := s.repo.GetByID(ctx, "pipeline_templates", id)
	if err != nil {
		return err
	}
	targetStatus := "deleted"
	if strings.TrimSpace(fmt.Sprint(template["status"])) == "published" {
		targetStatus = "archived"
	}
	return s.repo.Exec(ctx, `UPDATE pipeline_templates SET status=$2, updated_at=now() WHERE id=$1`, id, targetStatus)
}

func (s *Service) PublishTemplate(ctx context.Context, id, actorID, actorName string) (Record, error) {
	template, err := s.repo.GetTemplateDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(fmt.Sprint(template["status"])) == "published" {
		return template, nil
	}
	if strings.TrimSpace(fmt.Sprint(template["status"])) != "draft" {
		return nil, validationError("Chỉ revision nháp mới có thể được phát hành", Record{"template_id": id, "status": template["status"]})
	}
	issues, err := s.repo.TemplatePublishIssues(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(issues) > 0 {
		return nil, validationError("Mẫu quy trình chưa đạt điều kiện phát hành", Record{"issues": issues})
	}
	definitionHash, err := templateDefinitionHash(template)
	if err != nil {
		return nil, err
	}
	template, err = s.repo.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		rows, queryErr := queryTx(ctx, tx, `
			UPDATE pipeline_templates
			SET status='published', definition_hash=$2, published_at=now(), published_by=NULLIF($3,'')::uuid, updated_at=now()
			WHERE id=$1 AND status='draft'
			RETURNING *
		`, id, definitionHash, actorID)
		if queryErr != nil {
			return nil, queryErr
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		return rows[0], nil
	}, func(row Record) auditEvent {
		return auditEvent{
			ActorID: actorID, ActorName: actorName, EventType: "pipeline_template_published",
			TargetType: "pipeline_template", TargetID: id,
			Description: fmt.Sprintf("Phát hành mẫu %s", row["name"]), Metadata: Record{},
		}
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetTemplateDetail(ctx, id)
}

func (s *Service) CloneTemplate(ctx context.Context, id, actorID string) (Record, error) {
	return s.repo.CloneTemplate(ctx, id, actorID)
}

func (s *Service) WorkflowFormCatalog(ctx context.Context) ([]Record, error) {
	return s.repo.WorkflowFormCatalog(ctx)
}

func (s *Service) WorkflowFormTemplateLibrary(ctx context.Context) ([]Record, error) {
	return s.repo.WorkflowFormTemplateLibrary(ctx)
}

func (s *Service) WorkflowFormTemplateDetail(ctx context.Context, templateID string) (Record, error) {
	templateID = cleanID(templateID)
	if templateID == "" {
		return nil, validationError("ID mẫu biểu mẫu quy trình không hợp lệ", Record{"template_id": templateID})
	}
	return s.repo.WorkflowFormTemplateDetail(ctx, templateID)
}

func (s *Service) CreateWorkflowFormTemplate(ctx context.Context, req WorkflowFormTemplateRequest, actorID, actorName string) (Record, error) {
	normalized, err := normalizeWorkflowFormTemplateRequest(req, true)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateWorkflowFormTemplate(ctx, normalized, actorID, actorName)
}

func (s *Service) UpdateWorkflowFormTemplate(ctx context.Context, templateID string, req WorkflowFormTemplateRequest, actorID, actorName string) (Record, error) {
	templateID = cleanID(templateID)
	if templateID == "" {
		return nil, validationError("ID mẫu biểu mẫu quy trình không hợp lệ", Record{"template_id": templateID})
	}
	normalized, err := normalizeWorkflowFormTemplateRequest(req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateWorkflowFormTemplate(ctx, templateID, normalized, actorID, actorName)
}

func (s *Service) ArchiveWorkflowFormTemplate(ctx context.Context, templateID, actorID, actorName string) (Record, error) {
	templateID = cleanID(templateID)
	if templateID == "" {
		return nil, validationError("ID mẫu biểu mẫu quy trình không hợp lệ", Record{"template_id": templateID})
	}
	return s.repo.ArchiveWorkflowFormTemplate(ctx, templateID, actorID, actorName)
}

func (s *Service) CreateWorkflowFormVersion(ctx context.Context, templateID string, req WorkflowFormVersionRequest, actorID, actorName string) (Record, error) {
	templateID = cleanID(templateID)
	if templateID == "" {
		return nil, validationError("ID mẫu biểu mẫu quy trình không hợp lệ", Record{"template_id": templateID})
	}
	normalized, err := normalizeWorkflowFormVersionRequest(req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateWorkflowFormVersion(ctx, templateID, normalized, actorID, actorName)
}

func (s *Service) UpdateWorkflowFormVersion(ctx context.Context, versionID string, req WorkflowFormVersionRequest, actorID, actorName string) (Record, error) {
	versionID = cleanID(versionID)
	if versionID == "" {
		return nil, validationError("ID phiên bản biểu mẫu quy trình không hợp lệ", Record{"version_id": versionID})
	}
	normalized, err := normalizeWorkflowFormVersionRequest(req, false)
	if err != nil {
		return nil, err
	}
	schemaProvided := req.SchemaProvided || req.Schema != nil
	validationProvided := req.ValidationProvided || req.Validation != nil
	fieldsProvided := req.FieldsProvided || req.Fields != nil
	if !schemaProvided || !validationProvided || !fieldsProvided {
		existing, err := s.repo.WorkflowFormVersionDetail(ctx, versionID)
		if err != nil {
			return nil, err
		}
		if !schemaProvided {
			normalized["schema_json"] = existing["schema_json"]
		}
		if !validationProvided {
			normalized["validation_json"] = existing["validation_json"]
		}
		if !fieldsProvided {
			normalized["fields"] = workflowFieldRecords(existing["fields"])
		}
	}
	return s.repo.UpdateWorkflowFormVersion(ctx, versionID, normalized, actorID, actorName)
}

func (s *Service) PublishWorkflowFormVersion(ctx context.Context, versionID, actorID, actorName string) (Record, error) {
	versionID = cleanID(versionID)
	if versionID == "" {
		return nil, validationError("ID phiên bản biểu mẫu quy trình không hợp lệ", Record{"version_id": versionID})
	}
	return s.repo.PublishWorkflowFormVersion(ctx, versionID, actorID, actorName)
}

func (s *Service) ArchiveWorkflowFormVersion(ctx context.Context, versionID, actorID, actorName string) (Record, error) {
	versionID = cleanID(versionID)
	if versionID == "" {
		return nil, validationError("ID phiên bản biểu mẫu quy trình không hợp lệ", Record{"version_id": versionID})
	}
	return s.repo.ArchiveWorkflowFormVersion(ctx, versionID, actorID, actorName)
}

func (s *Service) CreateTemplateStep(ctx context.Context, body Record) (Record, error) {
	template, err := s.ensureDraftTemplate(ctx, "node", cleanID(body["node_id"]))
	if err != nil {
		return nil, err
	}
	if err := prepareBusinessCode(body, int(number(template["engine_version"])) >= 3); err != nil {
		return nil, err
	}
	if err := s.applyTemplateStepFormDefaults(ctx, body, nil); err != nil {
		return nil, err
	}
	return s.repo.Insert(ctx, "pipeline_template_steps", templateStepColumns(), body)
}

func (s *Service) UpdateTemplateStep(ctx context.Context, id string, body Record) (Record, error) {
	template, err := s.ensureDraftTemplate(ctx, "step", id)
	if err != nil {
		return nil, err
	}
	if err := prepareBusinessCode(body, int(number(template["engine_version"])) >= 3); err != nil {
		return nil, err
	}
	current, err := s.repo.GetByID(ctx, "pipeline_template_steps", id)
	if err != nil {
		return nil, err
	}
	if err := s.applyTemplateStepFormDefaults(ctx, body, current); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, "pipeline_template_steps", templateStepColumns(), id, body)
}

func (s *Service) DeleteTemplateStep(ctx context.Context, id string) error {
	if _, err := s.ensureDraftTemplate(ctx, "step", id); err != nil {
		return err
	}
	return s.repo.Exec(ctx, `DELETE FROM pipeline_template_steps WHERE id=$1`, id)
}

func (s *Service) applyTemplateStepFormDefaults(ctx context.Context, body Record, current Record) error {
	if body == nil {
		return nil
	}
	stepType := "task"
	if current != nil {
		stepType = strings.TrimSpace(fmt.Sprint(current["step_type"]))
	}
	if raw, ok := body["step_type"]; ok {
		stepType = strings.TrimSpace(fmt.Sprint(raw))
	}
	if stepType == "" || stepType == "<nil>" {
		stepType = "task"
	}
	stepName := ""
	if current != nil {
		stepName = strings.TrimSpace(fmt.Sprint(current["name"]))
	}
	if raw, ok := body["name"]; ok {
		stepName = strings.TrimSpace(fmt.Sprint(raw))
	}
	checklistRequired := true
	if current != nil {
		checklistRequired = boolValue(current["checklist_required"])
	}
	if raw, ok := body["checklist_required"]; ok {
		checklistRequired = boolValue(raw)
	}
	formRequired := workflowFormRequiredForStep(stepType, checklistRequired)
	if raw, ok := body["form_required"]; ok {
		formRequired = boolValue(raw)
	} else {
		body["form_required"] = formRequired
	}
	if _, provided := body["surface_kind"]; !provided {
		if current != nil && strings.TrimSpace(fmt.Sprint(current["surface_kind"])) != "" && fmt.Sprint(current["surface_kind"]) != "<nil>" {
			body["surface_kind"] = current["surface_kind"]
		} else {
			body["surface_kind"] = defaultStepSurfaceKind(stepType, formRequired)
		}
	}
	_, formVersionProvided := body["form_version_id"]
	if !formVersionProvided && cleanID(current["form_version_id"]) != "" {
		return nil
	}
	if formVersionProvided && cleanID(body["form_version_id"]) != "" {
		return nil
	}
	if !formRequired {
		return nil
	}
	formVersionID, err := s.repo.DefaultWorkflowFormVersionID(ctx, stepType, checklistRequired, stepName)
	if err != nil {
		return err
	}
	if formVersionID != "" {
		body["form_version_id"] = formVersionID
	}
	return nil
}

func workflowFormRequiredForStep(stepType string, checklistRequired bool) bool {
	return checklistRequired || statusIn(stepType, "daily_report", "meeting_minutes", "checklist", "acceptance_check", "handover_check")
}

func defaultStepSurfaceKind(stepType string, formRequired bool) string {
	switch {
	case statusIn(stepType, "photo_upload", "evidence_upload"):
		return "evidence"
	case stepType == "approval":
		return "approval"
	case formRequired:
		return "workflow_form"
	default:
		return "generic"
	}
}

func normalizeWorkflowFormTemplateRequest(req WorkflowFormTemplateRequest, create bool) (Record, error) {
	code := strings.TrimSpace(req.Code)
	name := strings.TrimSpace(req.Name)
	description := strings.TrimSpace(req.Description)
	formType := strings.TrimSpace(req.FormType)
	status := strings.TrimSpace(req.Status)
	if create {
		if code == "" || !validWorkflowFormCode(code) {
			return nil, validationError("Mã biểu mẫu quy trình không hợp lệ", Record{"code": code})
		}
		if formType == "" {
			formType = "generic_submission"
		}
		if !validWorkflowFormType(formType) {
			return nil, validationError("Loại biểu mẫu quy trình không hợp lệ", Record{"form_type": formType})
		}
	}
	if name == "" {
		return nil, validationError("Tên biểu mẫu quy trình là bắt buộc")
	}
	if status == "" {
		status = "active"
	}
	if !statusIn(status, "active", "archived") {
		return nil, validationError("Trạng thái mẫu biểu mẫu quy trình không hợp lệ", Record{"status": status})
	}
	out := Record{"name": name, "description": description, "status": status}
	if create {
		out["code"] = code
		out["form_type"] = formType
	}
	return out, nil
}

func normalizeWorkflowFormVersionRequest(req WorkflowFormVersionRequest, requireFields bool) (Record, error) {
	fields, err := normalizeWorkflowFormFieldRequests(req.Fields, requireFields)
	if err != nil {
		return nil, err
	}
	schema := req.Schema
	if schema == nil {
		schema = Record{}
	}
	validation := req.Validation
	if validation == nil {
		validation = Record{}
	}
	return Record{
		"source_version_id": cleanID(req.SourceVersionID),
		"schema_json":       schema,
		"validation_json":   validation,
		"fields":            fields,
	}, nil
}

func normalizeWorkflowFormFieldRequests(fields []WorkflowFormFieldRequest, requireFields bool) ([]Record, error) {
	if requireFields && len(fields) == 0 {
		return nil, validationError("Phiên bản biểu mẫu quy trình cần ít nhất một trường")
	}
	seen := map[string]bool{}
	out := make([]Record, 0, len(fields))
	for index, field := range fields {
		fieldKey := strings.TrimSpace(field.FieldKey)
		if !validWorkflowFieldKey(fieldKey) {
			return nil, validationError("Mã trường biểu mẫu quy trình không hợp lệ", Record{"field_key": fieldKey})
		}
		if seen[fieldKey] {
			return nil, validationError("Mã trường biểu mẫu quy trình bị trùng", Record{"field_key": fieldKey})
		}
		seen[fieldKey] = true
		label := strings.TrimSpace(field.Label)
		if label == "" {
			return nil, validationError("Nhãn trường biểu mẫu quy trình là bắt buộc", Record{"field_key": fieldKey})
		}
		fieldType := strings.TrimSpace(field.FieldType)
		if !validWorkflowFieldType(fieldType) {
			return nil, validationError("Loại trường biểu mẫu quy trình không hợp lệ", Record{"field_key": fieldKey, "field_type": fieldType})
		}
		orderIndex := field.OrderIndex
		if orderIndex <= 0 {
			orderIndex = (index + 1) * 10
		}
		options := field.Options
		if options == nil {
			options = []any{}
		}
		validation := field.Validation
		if validation == nil {
			validation = Record{}
		}
		if err := validateWorkflowCurrencyDefinitions(fieldType, validation); err != nil {
			return nil, validationError(err.Error(), Record{"field_key": fieldKey})
		}
		visibility := field.VisibilityCondition
		if visibility == nil {
			visibility = Record{}
		}
		out = append(out, Record{
			"field_key":                 fieldKey,
			"label":                     label,
			"field_type":                fieldType,
			"required":                  field.Required,
			"options_json":              options,
			"validation_json":           validation,
			"help_text":                 strings.TrimSpace(field.HelpText),
			"order_index":               orderIndex,
			"visibility_condition_json": visibility,
		})
	}
	if err := validateWorkflowFormFieldRules(out); err != nil {
		return nil, err
	}
	return out, nil
}

func validWorkflowFormCode(value string) bool {
	if value == "" {
		return false
	}
	return validWorkflowIdentifier(value)
}

func validWorkflowFieldKey(value string) bool {
	return validWorkflowIdentifier(value)
}

func validWorkflowIdentifier(value string) bool {
	for index, char := range value {
		if index == 0 {
			if char < 'a' || char > 'z' {
				return false
			}
			continue
		}
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' {
			continue
		}
		return false
	}
	return value != ""
}

func validWorkflowFormType(value string) bool {
	return statusIn(value, "checklist", "daily_report", "meeting_minutes", "generic_submission")
}

func validWorkflowFieldType(value string) bool {
	return statusIn(value, "text", "number", "currency", "date", "textarea", "select", "multi_select", "checkbox", "evidence_reference", "structured_list")
}

func validateWorkflowCurrencyDefinitions(fieldType string, validation Record) error {
	if fieldType == "currency" {
		return validateWorkflowCurrencyConfiguration(validation)
	}
	if fieldType != "structured_list" {
		return nil
	}
	rawFields, _ := validation["item_fields"].([]any)
	for _, rawField := range rawFields {
		spec, ok := rawField.(map[string]any)
		if !ok || strings.TrimSpace(fmt.Sprint(spec["type"])) != "currency" {
			continue
		}
		if err := validateWorkflowCurrencyConfiguration(Record(spec)); err != nil {
			return fmt.Errorf("Cấu hình tiền tệ của %s không hợp lệ: %w", strings.TrimSpace(fmt.Sprint(spec["key"])), err)
		}
	}
	return nil
}

func validateWorkflowCurrencyConfiguration(configuration Record) error {
	currencyCode := "VND"
	if rawCode, exists := configuration["currency_code"]; exists {
		currencyCode = strings.TrimSpace(fmt.Sprint(rawCode))
		if !validWorkflowCurrencyCode(currencyCode) {
			return errors.New("mã tiền tệ phải gồm 3 chữ cái viết hoa")
		}
	}
	if rawDigits, exists := configuration["maximum_fraction_digits"]; exists {
		digits, err := workflowNumberValue(rawDigits)
		if err != nil || digits < 0 || digits > 6 || digits != math.Trunc(digits) {
			return errors.New("số lẻ tối đa phải là số nguyên từ 0 đến 6")
		}
		if currencyCode == "VND" && digits != 0 {
			return errors.New("VND phải dùng 0 chữ số thập phân")
		}
	}
	if rawAllowNegative, exists := configuration["allow_negative"]; exists {
		if _, ok := rawAllowNegative.(bool); !ok {
			return errors.New("allow_negative phải là boolean")
		}
	}
	var minimum, maximum float64
	var hasMinimum, hasMaximum bool
	if rawMinimum, exists := configuration["min"]; exists {
		value, err := workflowNumberValue(rawMinimum)
		if err != nil {
			return errors.New("giá trị nhỏ nhất phải là số")
		}
		minimum, hasMinimum = value, true
	}
	if rawMaximum, exists := configuration["max"]; exists {
		value, err := workflowNumberValue(rawMaximum)
		if err != nil {
			return errors.New("giá trị lớn nhất phải là số")
		}
		maximum, hasMaximum = value, true
	}
	if hasMinimum && hasMaximum && minimum > maximum {
		return errors.New("giá trị nhỏ nhất không được lớn hơn giá trị lớn nhất")
	}
	return nil
}

func validWorkflowCurrencyCode(code string) bool {
	if len(code) != 3 {
		return false
	}
	for _, char := range code {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
}
