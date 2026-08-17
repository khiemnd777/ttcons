package core

import "testing"

func TestApplyOperationalWorkflowFormFieldPresentationUsesClearLabels(t *testing.T) {
	tests := []struct {
		formCode string
		fieldKey string
		label    string
		helpText string
	}{
		{
			formCode: "client_survey_requirement_intake",
			fieldKey: "constraints",
			label:    "Ghi chú khác",
			helpText: "Các lưu ý khác có thể ảnh hưởng đến khảo sát hoặc thi công, nếu có.",
		},
		{
			formCode: "client_design_style_brief",
			fieldKey: "design_exclusions",
			label:    "Phong cách / chi tiết cần tránh",
			helpText: "Ví dụ: màu quá tối, nhiều phào chỉ, bề mặt bóng hoặc vật liệu không muốn sử dụng.",
		},
		{
			formCode: "client_design_layout_submission",
			fieldKey: "owner_feedback",
			label:    "Nội dung cần chỉnh trên layout",
			helpText: "Ghi những điểm cần sửa hoặc làm rõ trước khi thống nhất layout.",
		},
	}

	for _, test := range tests {
		t.Run(test.fieldKey, func(t *testing.T) {
			original := Record{"field_key": test.fieldKey, "label": "Legacy label", "help_text": nil}
			presented := applyOperationalWorkflowFormFieldPresentation(
				operationalWorkflowFormVariant,
				test.formCode,
				[]Record{original},
			)
			if presented[0]["label"] != test.label || presented[0]["help_text"] != test.helpText {
				t.Fatalf("presentation=%#v, want label=%q help=%q", presented[0], test.label, test.helpText)
			}
			if original["label"] != "Legacy label" || original["help_text"] != nil {
				t.Fatalf("published field definition was mutated: %#v", original)
			}
		})
	}
}

func TestApplyOperationalWorkflowFormFieldPresentationDoesNotRenameHistoricalVariants(t *testing.T) {
	fields := []Record{{"field_key": "constraints", "label": "Ràng buộc đặc biệt"}}
	presented := applyOperationalWorkflowFormFieldPresentation(
		"lean_capture_v2",
		"client_survey_requirement_intake",
		fields,
	)
	if presented[0]["label"] != "Ràng buộc đặc biệt" {
		t.Fatalf("historical label changed: %#v", presented[0]["label"])
	}
}
