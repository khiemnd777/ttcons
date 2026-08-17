package core

import "strings"

const operationalWorkflowFormVariant = "operational_capture_v3"

type workflowFormFieldPresentationOverride struct {
	Label    string
	HelpText string
}

var operationalWorkflowFormFieldPresentationOverrides = map[string]map[string]workflowFormFieldPresentationOverride{
	"client_survey_requirement_intake": {
		"constraints": {
			Label:    "Ghi chú khác",
			HelpText: "Các lưu ý khác có thể ảnh hưởng đến khảo sát hoặc thi công, nếu có.",
		},
	},
	"client_design_style_brief": {
		"design_exclusions": {
			Label:    "Phong cách / chi tiết cần tránh",
			HelpText: "Ví dụ: màu quá tối, nhiều phào chỉ, bề mặt bóng hoặc vật liệu không muốn sử dụng.",
		},
	},
	"client_design_layout_submission": {
		"owner_feedback": {
			Label:    "Nội dung cần chỉnh trên layout",
			HelpText: "Ghi những điểm cần sửa hoặc làm rõ trước khi thống nhất layout.",
		},
	},
}

func applyOperationalWorkflowFormFieldPresentation(variant, formCode string, fields []Record) []Record {
	if strings.TrimSpace(variant) != operationalWorkflowFormVariant {
		return fields
	}
	overrides := operationalWorkflowFormFieldPresentationOverrides[strings.TrimSpace(formCode)]
	if len(overrides) == 0 {
		return fields
	}

	presented := make([]Record, len(fields))
	for index, field := range fields {
		override, exists := overrides[strings.TrimSpace(cleanID(field["field_key"]))]
		if !exists {
			presented[index] = field
			continue
		}
		clone := make(Record, len(field))
		for key, value := range field {
			clone[key] = value
		}
		clone["label"] = override.Label
		clone["help_text"] = override.HelpText
		presented[index] = clone
	}
	return presented
}
