package core

import (
	"errors"
	"fmt"
	"strings"
)

const (
	workflowOtherOptionValue   = "other"
	workflowOtherTextMaxLength = 300
)

type workflowSelectOtherTextRule struct {
	errorLabel string
}

var workflowSelectOtherTextRules = map[string]workflowSelectOtherTextRule{
	"client_design_style_brief.style_direction":            {errorLabel: "Phong cách khác"},
	"client_survey_work_package.style_direction":           {errorLabel: "Phong cách khác"},
	"client_boq_owner_submission.sent_channel":             {errorLabel: "Kênh gửi khác"},
	"client_hardcopy_dispatch.delivery_method":             {errorLabel: "Cách gửi khác"},
	"builtin_construction_daily_report.construction_scope": {errorLabel: "Phạm vi thi công khác"},
	"builtin_construction_daily_report.weather":            {errorLabel: "Điều kiện thi công khác"},
	"builtin_subcontractor_checklist.work_package":         {errorLabel: "Gói / phạm vi thi công khác"},
	"client_subcontractor_acceptance_package.work_package": {errorLabel: "Gói / phạm vi thi công khác"},
	"builtin_daily_report.planning_scope":                  {errorLabel: "Phạm vi kế hoạch khác"},
	"builtin_checklist.planning_scope":                     {errorLabel: "Phạm vi kế hoạch khác"},
	"client_owner_soft_file_submission.delivery_channel":   {errorLabel: "Kênh gửi khác"},
	"client_procurement_takeoff_review.takeoff_source":     {errorLabel: "Nguồn khối lượng khác"},
}

func workflowSelectSupportsOtherText(instance Record, field Record) (workflowSelectOtherTextRule, bool) {
	if strings.TrimSpace(fmt.Sprint(field["field_type"])) != "select" || !workflowOptionAllowed(field, workflowOtherOptionValue) {
		return workflowSelectOtherTextRule{}, false
	}
	rule, ok := workflowSelectOtherTextRules[workflowSelectOtherTextRuleKey(instance, field)]
	return rule, ok
}

func workflowSelectOtherTextSubmitIssue(instance Record, field Record, value any) string {
	rule, ok := workflowSelectSupportsOtherText(instance, field)
	if !ok {
		return ""
	}
	if strings.TrimSpace(fmt.Sprint(value)) == workflowOtherOptionValue {
		return rule.errorLabel + " là bắt buộc"
	}
	return ""
}

func normalizeWorkflowSelectAnswer(instance Record, field Record, value string) (string, error) {
	if workflowOptionAllowed(field, value) {
		return value, nil
	}
	rule, ok := workflowSelectSupportsOtherText(instance, field)
	if !ok {
		return "", errors.New("Lựa chọn không nằm trong schema")
	}
	if len([]rune(value)) > workflowOtherTextMaxLength {
		return "", fmt.Errorf("%s không được vượt quá %d ký tự", rule.errorLabel, workflowOtherTextMaxLength)
	}
	return value, nil
}

func workflowFormOtherDetailSubmitIssue(instance Record, field Record, value any, answers Record) string {
	formCode := strings.TrimSpace(fmt.Sprint(instance["form_code"]))
	fieldKey := strings.TrimSpace(fmt.Sprint(field["field_key"]))
	if formCode == "builtin_owner_client_checklist" && fieldKey == "scope_detail" {
		if strings.TrimSpace(fmt.Sprint(answers["handover_scope"])) == workflowOtherOptionValue && workflowFormValueEmpty(field, value) {
			return "Chi tiết phạm vi là bắt buộc khi Phạm vi kiểm tra là Khác"
		}
	}
	if formCode == "builtin_construction_daily_report" && fieldKey == "material_usage_items" {
		rows, err := workflowRecordList(value)
		if err != nil {
			return ""
		}
		for index, row := range rows {
			if strings.TrimSpace(fmt.Sprint(row["source"])) == workflowOtherOptionValue && strings.TrimSpace(fmt.Sprint(row["note"])) == "" {
				return fmt.Sprintf("Vật tư sử dụng dòng %d: Ghi chú là bắt buộc khi Nguồn là Khác", index+1)
			}
		}
	}
	return ""
}

func workflowSelectOtherTextRuleKey(instance Record, field Record) string {
	return strings.TrimSpace(fmt.Sprint(instance["form_code"])) + "." + strings.TrimSpace(fmt.Sprint(field["field_key"]))
}
