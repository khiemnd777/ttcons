package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (s *Service) validateWorkflowFormAnswers(ctx context.Context, instance Record, fields []Record, answers Record, submit bool) (Record, error) {
	if answers == nil {
		answers = Record{}
	}
	answers = applyWorkflowFormAnswerDefaults(instance, answers)
	fieldByKey := map[string]Record{}
	for _, field := range fields {
		fieldByKey[fmt.Sprint(field["field_key"])] = field
	}
	for key := range answers {
		if _, ok := fieldByKey[key]; !ok {
			return nil, validationError("Biểu mẫu quy trình có trường không thuộc schema", Record{"field": key})
		}
	}
	normalized := Record{}
	missing := []string{}
	fieldErrors := Record{}
	for _, field := range fields {
		key := fmt.Sprint(field["field_key"])
		value, hasValue := answers[key]
		visible, err := workflowFormFieldVisible(field, answers)
		if err != nil {
			return nil, validationError("Quy tắc hiển thị biểu mẫu không hợp lệ", Record{"field": key, "error": err.Error()})
		}
		if !visible {
			continue
		}
		required, err := workflowFormFieldRequired(field, answers)
		if err != nil {
			return nil, validationError("Quy tắc bắt buộc biểu mẫu không hợp lệ", Record{"field": key, "error": err.Error()})
		}
		if submit && required && workflowFormValueEmpty(field, value) {
			missing = append(missing, key)
			fieldErrors[key] = fmt.Sprintf("%s là bắt buộc", field["label"])
			continue
		}
		if submit {
			if issue := workflowSelectOtherTextSubmitIssue(instance, field, value); issue != "" {
				fieldErrors[key] = issue
				continue
			}
			if issue := workflowFormOtherDetailSubmitIssue(instance, field, value, answers); issue != "" {
				fieldErrors[key] = issue
				continue
			}
		}
		if !hasValue || workflowFormValueEmpty(field, value) {
			continue
		}
		normalizedValue, err := s.normalizeWorkflowFormValue(ctx, instance, field, value, submit)
		if err != nil {
			fieldErrors[key] = err.Error()
			continue
		}
		normalized[key] = normalizedValue
	}
	if len(fieldErrors) > 0 {
		return nil, validationError("Biểu mẫu quy trình có dữ liệu không hợp lệ", Record{"fields": fieldErrors, "missing": missing})
	}
	if submit && fmt.Sprint(instance["form_type"]) == "checklist" {
		if err := validateChecklistSubmitReady(normalized); err != nil {
			return nil, err
		}
	}
	return normalized, nil
}

func workflowFormValueEmpty(field Record, value any) bool {
	fieldType := fmt.Sprint(field["field_type"])
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case []any:
		return len(v) == 0
	case []string:
		return len(v) == 0
	case []Record:
		return len(v) == 0
	case bool:
		return fieldType == "checkbox" && !v
	default:
		text := strings.TrimSpace(fmt.Sprint(v))
		return text == "" || text == "<nil>"
	}
}

func (s *Service) normalizeWorkflowFormValue(ctx context.Context, instance Record, field Record, value any, submit bool) (any, error) {
	fieldType := fmt.Sprint(field["field_type"])
	switch fieldType {
	case "text", "textarea":
		text := strings.TrimSpace(fmt.Sprint(value))
		if err := validateWorkflowTextLength(field, text, submit); err != nil {
			return nil, err
		}
		return text, nil
	case "number", "currency":
		numeric, err := workflowNumberValue(value)
		if err != nil {
			return nil, err
		}
		return numeric, validateWorkflowNumberRange(field, numeric)
	case "date":
		text := strings.TrimSpace(fmt.Sprint(value))
		if _, err := time.Parse("2006-01-02", text); err != nil {
			return nil, errors.New("Ngày không hợp lệ")
		}
		return text, nil
	case "select":
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" {
			return nil, errors.New("Lựa chọn không hợp lệ")
		}
		return normalizeWorkflowSelectAnswer(instance, field, text)
	case "multi_select":
		values, err := workflowStringList(value)
		if err != nil {
			return nil, err
		}
		for _, selected := range values {
			if !workflowOptionAllowed(field, selected) {
				return nil, errors.New("Lựa chọn không nằm trong schema")
			}
		}
		return values, nil
	case "checkbox":
		checked, ok := value.(bool)
		if !ok {
			return nil, errors.New("Giá trị checkbox phải là boolean")
		}
		return checked, nil
	case "evidence_reference":
		evidenceID := cleanID(value)
		if evidenceID == "" {
			return nil, errors.New("Minh chứng không hợp lệ")
		}
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, errors.New("Minh chứng không tồn tại")
			}
			return nil, err
		}
		if cleanID(evidence["project_id"]) != cleanID(instance["project_id"]) {
			return nil, errors.New("Minh chứng không thuộc công trình này")
		}
		if workItemID := cleanID(instance["work_item_id"]); workItemID != "" && cleanID(evidence["work_item_id"]) != workItemID {
			return nil, errors.New("Minh chứng không thuộc đầu việc này")
		}
		if fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, errors.New("Minh chứng đã bị từ chối")
		}
		return evidenceID, nil
	case "structured_list":
		return s.normalizeWorkflowStructuredList(ctx, instance, field, value, submit)
	default:
		return nil, errors.New("Loại trường không được hỗ trợ")
	}
}

func validateChecklistSubmitReady(answers Record) error {
	rawItems, ok := answers["checklist_items"]
	if !ok {
		return nil
	}
	items, err := workflowRecordList(rawItems)
	if err != nil {
		return err
	}
	blocking := []Record{}
	for index, item := range items {
		status := strings.TrimSpace(fmt.Sprint(item["status"]))
		if !statusIn(status, "done", "not_applicable") {
			blocking = append(blocking, Record{
				"row":    index + 1,
				"title":  item["title"],
				"status": status,
			})
		}
	}
	if len(blocking) == 0 {
		return nil
	}
	return validationError("Checklist còn hạng mục chưa đạt, chỉ được gửi khi tất cả dòng là Hoàn tất hoặc Không áp dụng", Record{
		"fields":         Record{"checklist_items": "Tất cả dòng checklist phải là Hoàn tất hoặc Không áp dụng trước khi gửi"},
		"blocking_items": blocking,
	})
}

func (s *Service) normalizeWorkflowStructuredList(ctx context.Context, instance Record, field Record, value any, submit bool) ([]Record, error) {
	items, err := workflowRecordList(value)
	if err != nil {
		return nil, err
	}
	validation, _ := field["validation_json"].(map[string]any)
	minItems := int(number(validation["min_items"]))
	maxItems := int(number(validation["max_items"]))
	if minItems > 0 && len(items) < minItems {
		return nil, fmt.Errorf("%s cần ít nhất %d dòng", field["label"], minItems)
	}
	if maxItems > 0 && len(items) > maxItems {
		return nil, fmt.Errorf("%s không được vượt quá %d dòng", field["label"], maxItems)
	}
	rawFields, _ := validation["item_fields"].([]any)
	if len(rawFields) == 0 {
		return items, nil
	}
	itemFields := make([]Record, 0, len(rawFields))
	fieldByKey := map[string]Record{}
	for _, rawField := range rawFields {
		spec, ok := rawField.(map[string]any)
		if !ok {
			continue
		}
		key := strings.TrimSpace(fmt.Sprint(spec["key"]))
		if key == "" || key == "<nil>" {
			continue
		}
		record := effectiveWorkflowStructuredItemField(instance, field, Record(spec))
		itemFields = append(itemFields, record)
		fieldByKey[key] = record
	}
	out := make([]Record, 0, len(items))
	for index, item := range items {
		for key := range item {
			if _, ok := fieldByKey[key]; !ok {
				return nil, fmt.Errorf("%s dòng %d có field không thuộc schema: %s", field["label"], index+1, key)
			}
		}
		normalized := Record{}
		for _, spec := range itemFields {
			key := fmt.Sprint(spec["key"])
			label := fmt.Sprint(spec["label"])
			if label == "" || label == "<nil>" {
				label = key
			}
			value, hasValue := item[key]
			required := boolValue(spec["required"])
			if required && structuredValueEmpty(spec, value) {
				return nil, fmt.Errorf("%s dòng %d: %s là bắt buộc", field["label"], index+1, label)
			}
			if !hasValue || structuredValueEmpty(spec, value) {
				continue
			}
			normalizedValue, err := s.normalizeStructuredFieldValue(ctx, instance, spec, value, submit)
			if err != nil {
				return nil, fmt.Errorf("%s dòng %d: %s", field["label"], index+1, err.Error())
			}
			normalized[key] = normalizedValue
		}
		if len(normalized) > 0 {
			out = append(out, normalized)
		}
	}
	if minItems > 0 && len(out) < minItems {
		return nil, fmt.Errorf("%s cần ít nhất %d dòng hợp lệ", field["label"], minItems)
	}
	return out, nil
}

func workflowRecordList(value any) ([]Record, error) {
	rawItems, ok := value.([]any)
	if !ok {
		if records, ok := value.([]Record); ok {
			return records, nil
		}
		return nil, errors.New("Giá trị danh sách phải là array")
	}
	items := make([]Record, 0, len(rawItems))
	for _, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, errors.New("Mỗi dòng trong danh sách phải là object")
		}
		items = append(items, Record(item))
	}
	return items, nil
}

func structuredValueEmpty(field Record, value any) bool {
	if value == nil {
		return true
	}
	fieldType := fmt.Sprint(field["type"])
	if fieldType == "checkbox" {
		return value != true
	}
	if values, ok := value.([]any); ok {
		return len(values) == 0
	}
	return strings.TrimSpace(fmt.Sprint(value)) == "" || strings.TrimSpace(fmt.Sprint(value)) == "<nil>"
}

func (s *Service) normalizeStructuredFieldValue(ctx context.Context, instance Record, field Record, value any, submit bool) (any, error) {
	fieldType := fmt.Sprint(field["type"])
	label := fmt.Sprint(field["label"])
	if label == "" || label == "<nil>" {
		label = fmt.Sprint(field["key"])
	}
	switch fieldType {
	case "text", "textarea":
		text := strings.TrimSpace(fmt.Sprint(value))
		if min := int(number(field["min_length"])); submit && min > 0 && len([]rune(text)) < min {
			return nil, fmt.Errorf("%s phải có ít nhất %d ký tự", label, min)
		}
		if max := int(number(field["max_length"])); max > 0 && len([]rune(text)) > max {
			return nil, fmt.Errorf("%s không được vượt quá %d ký tự", label, max)
		}
		return text, nil
	case "number", "currency":
		numeric, err := workflowNumberValue(value)
		if err != nil {
			return nil, fmt.Errorf("%s phải là số", label)
		}
		if minValue, ok := field["min"]; ok && numeric < number(minValue) {
			return nil, fmt.Errorf("%s phải lớn hơn hoặc bằng %v", label, minValue)
		}
		if maxValue, ok := field["max"]; ok && numeric > number(maxValue) {
			return nil, fmt.Errorf("%s phải nhỏ hơn hoặc bằng %v", label, maxValue)
		}
		return numeric, nil
	case "date":
		text := strings.TrimSpace(fmt.Sprint(value))
		if _, err := time.Parse("2006-01-02", text); err != nil {
			return nil, fmt.Errorf("%s không hợp lệ", label)
		}
		return text, nil
	case "select":
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" {
			return nil, fmt.Errorf("%s không hợp lệ", label)
		}
		if !structuredOptionAllowed(field, text) {
			return nil, fmt.Errorf("%s không nằm trong schema", label)
		}
		return text, nil
	case "evidence_reference":
		evidenceID := cleanID(value)
		if evidenceID == "" {
			return nil, fmt.Errorf("%s không hợp lệ", label)
		}
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("%s không tồn tại", label)
			}
			return nil, err
		}
		if cleanID(evidence["project_id"]) != cleanID(instance["project_id"]) {
			return nil, fmt.Errorf("%s không thuộc công trình này", label)
		}
		if workItemID := cleanID(instance["work_item_id"]); workItemID != "" && cleanID(evidence["work_item_id"]) != workItemID {
			return nil, fmt.Errorf("%s không thuộc đầu việc này", label)
		}
		if fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, fmt.Errorf("%s đã bị từ chối", label)
		}
		return evidenceID, nil
	default:
		return nil, fmt.Errorf("Loại trường %s không được hỗ trợ", fieldType)
	}
}

func structuredOptionAllowed(field Record, selected string) bool {
	rawOptions, _ := field["options"].([]any)
	if len(rawOptions) == 0 {
		return true
	}
	for _, raw := range rawOptions {
		switch option := raw.(type) {
		case string:
			if option == selected {
				return true
			}
		case map[string]any:
			if strings.TrimSpace(fmt.Sprint(option["value"])) == selected {
				return true
			}
		}
	}
	return false
}

func validateWorkflowTextLength(field Record, text string, enforceMin bool) error {
	validation, _ := field["validation_json"].(map[string]any)
	if min := int(number(validation["min_length"])); enforceMin && min > 0 && len([]rune(text)) < min {
		return fmt.Errorf("%s phải có ít nhất %d ký tự", field["label"], min)
	}
	if max := int(number(validation["max_length"])); max > 0 && len([]rune(text)) > max {
		return fmt.Errorf("%s không được vượt quá %d ký tự", field["label"], max)
	}
	return nil
}

func workflowNumberValue(value any) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case string:
		out, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, errors.New("Giá trị phải là số")
		}
		return out, nil
	default:
		return 0, errors.New("Giá trị phải là số")
	}
}

func validateWorkflowNumberRange(field Record, numeric float64) error {
	validation, _ := field["validation_json"].(map[string]any)
	if minValue, ok := validation["min"]; ok && numeric < number(minValue) {
		return fmt.Errorf("%s phải lớn hơn hoặc bằng %v", field["label"], minValue)
	}
	if maxValue, ok := validation["max"]; ok && numeric > number(maxValue) {
		return fmt.Errorf("%s phải nhỏ hơn hoặc bằng %v", field["label"], maxValue)
	}
	return nil
}

func workflowStringList(value any) ([]string, error) {
	switch v := value.(type) {
	case []string:
		out := make([]string, 0, len(v))
		seen := map[string]bool{}
		for _, item := range v {
			text := strings.TrimSpace(item)
			if text == "" || seen[text] {
				continue
			}
			seen[text] = true
			out = append(out, text)
		}
		return out, nil
	case []any:
		out := make([]string, 0, len(v))
		seen := map[string]bool{}
		for _, item := range v {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text == "" || text == "<nil>" || seen[text] {
				continue
			}
			seen[text] = true
			out = append(out, text)
		}
		return out, nil
	default:
		return nil, errors.New("Giá trị phải là danh sách")
	}
}

func workflowOptionAllowed(field Record, value string) bool {
	options, _ := field["options_json"].([]any)
	if len(options) == 0 {
		return true
	}
	for _, rawOption := range options {
		option, _ := rawOption.(map[string]any)
		if strings.TrimSpace(fmt.Sprint(option["value"])) == value {
			return true
		}
		if option == nil && strings.TrimSpace(fmt.Sprint(rawOption)) == value {
			return true
		}
	}
	return false
}
