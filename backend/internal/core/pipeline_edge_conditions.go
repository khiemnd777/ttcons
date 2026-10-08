package core

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type EdgeConditionDefinition struct {
	Type         string                               `json:"type"`
	Label        string                               `json:"label"`
	Description  string                               `json:"description"`
	DefaultLabel string                               `json:"default_label"`
	Category     string                               `json:"category"`
	ConfigSchema []EdgeConditionConfigFieldDefinition `json:"config_schema,omitempty"`
}

type EdgeConditionConfigFieldDefinition struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Control   string `json:"control"`
	Required  bool   `json:"required"`
	DependsOn string `json:"depends_on,omitempty"`
	HelpText  string `json:"help_text,omitempty"`
}

var pipelineEdgeConditionCatalog = []EdgeConditionDefinition{
	{Type: "always", Label: "Luôn chuyển", Description: "Chuyển sang node đích khi node nguồn hoàn tất.", DefaultLabel: "", Category: "core"},
	{Type: "order_index", Label: "Theo thứ tự node", Description: "Chỉ chuyển nếu node đích đứng sau node nguồn trong thứ tự mẫu quy trình.", DefaultLabel: "", Category: "core"},
	{Type: "approval_approved", Label: "Phê duyệt đã duyệt", Description: "Chuyển khi yêu cầu phê duyệt của node nguồn đã được duyệt.", DefaultLabel: "", Category: "approval"},
	{Type: "approval_rejected", Label: "Phê duyệt bị từ chối", Description: "Chuyển khi yêu cầu phê duyệt của node nguồn bị từ chối.", DefaultLabel: "", Category: "approval"},
	{Type: "acceptance_passed", Label: "Nghiệm thu đạt", Description: "Chuyển khi công trình có nghiệm thu đã hoàn tất với kết quả đạt.", DefaultLabel: "", Category: "quality"},
	{Type: "acceptance_failed", Label: "Nghiệm thu không đạt", Description: "Chuyển khi công trình có nghiệm thu không đạt.", DefaultLabel: "", Category: "quality"},
	{Type: "rework_required", Label: "Cần làm lại", Description: "Chuyển khi vấn đề hoặc nghiệm thu đang yêu cầu làm lại.", DefaultLabel: "", Category: "rework"},
	{Type: "rework_completed", Label: "Làm lại đã hoàn tất", Description: "Chuyển khi việc làm lại đã hoàn tất và không còn yêu cầu làm lại đang mở.", DefaultLabel: "", Category: "rework"},
	{Type: "replanning_required", Label: "Cần lập lại kế hoạch", Description: "Chuyển khi vấn đề hoặc yêu cầu lập lại kế hoạch đang mở.", DefaultLabel: "", Category: "planning"},
	{Type: "materials_issued", Label: "Vật tư đã xuất đủ", Description: "Chuyển khi vật tư theo kế hoạch không còn thiếu tại công trình.", DefaultLabel: "", Category: "material"},
	{Type: "boq_approved", Label: "BOQ đã duyệt", Description: "Chuyển khi BOQ có dữ liệu và toàn bộ item đã được duyệt.", DefaultLabel: "", Category: "commercial"},
	{Type: "procurement_approved", Label: "Mua vật tư đã duyệt", Description: "Chuyển khi yêu cầu mua vật tư đã được duyệt hoặc đã vào các trạng thái sau duyệt.", DefaultLabel: "", Category: "material"},
	{
		Type: "workflow_form_answer_equals", Label: "Câu trả lời biểu mẫu bằng giá trị",
		Description:  "Chuyển khi biểu mẫu quy trình đã được gửi ở node nguồn và có trường khớp giá trị cấu hình.",
		DefaultLabel: "", Category: "workflow_form",
		ConfigSchema: []EdgeConditionConfigFieldDefinition{
			{Key: "field_key", Label: "Trường biểu mẫu nguồn", Control: "source_form_field", Required: true, HelpText: "Chọn trường thuộc biểu mẫu đã gắn vào node nguồn."},
			{Key: "value", Label: "Giá trị cần khớp", Control: "source_field_option", Required: true, DependsOn: "field_key", HelpText: "Chọn một giá trị được định nghĩa bởi trường nguồn."},
		},
	},
	{Type: "handover_ready", Label: "Sẵn sàng bàn giao", Description: "Chuyển khi nghiệm thu, danh sách lỗi, kho và hồ sơ bàn giao không còn điểm chặn.", DefaultLabel: "", Category: "handover"},
}

func PipelineEdgeConditionCatalog() []EdgeConditionDefinition {
	return pipelineEdgeConditionCatalog
}

func edgeConditionDefinition(conditionType string) (EdgeConditionDefinition, bool) {
	conditionType = strings.TrimSpace(conditionType)
	for _, definition := range pipelineEdgeConditionCatalog {
		if definition.Type == conditionType {
			return definition, true
		}
	}
	return EdgeConditionDefinition{}, false
}

type edgeConditionConfigValidator func(Record) error

var edgeConditionConfigValidators = map[string]edgeConditionConfigValidator{
	"workflow_form_answer_equals": validateWorkflowFormAnswerEqualsConfig,
}

func validateWorkflowFormAnswerEqualsConfig(config Record) error {
	if strings.TrimSpace(fmt.Sprint(config["field_key"])) == "" || fmt.Sprint(config["field_key"]) == "<nil>" {
		return errors.New("field_key là bắt buộc cho điều kiện câu trả lời biểu mẫu")
	}
	if _, exists := config["value"]; !exists {
		return errors.New("value là bắt buộc cho điều kiện câu trả lời biểu mẫu")
	}
	return nil
}

func prepareTemplateEdgeBody(body Record, partial bool) error {
	if !partial {
		if strings.TrimSpace(fmt.Sprint(body["from_node_id"])) == "" {
			return errors.New("from_node_id là bắt buộc")
		}
		if strings.TrimSpace(fmt.Sprint(body["to_node_id"])) == "" {
			return errors.New("to_node_id là bắt buộc")
		}
	}

	if value, ok := body["condition_type"]; ok {
		conditionType := strings.TrimSpace(fmt.Sprint(value))
		if conditionType == "" || conditionType == "<nil>" {
			return errors.New("condition_type là bắt buộc")
		}
		if _, exists := edgeConditionDefinition(conditionType); !exists {
			return fmt.Errorf("condition_type không được hỗ trợ: %q", conditionType)
		}
		body["condition_type"] = conditionType
	} else if !partial {
		body["condition_type"] = "always"
	}

	if value, ok := body["label"]; ok {
		label := strings.TrimSpace(fmt.Sprint(value))
		if label == "<nil>" {
			label = ""
		}
		if len([]rune(label)) > 80 {
			return errors.New("Nhãn điều kiện không được vượt quá 80 ký tự")
		}
		body["label"] = label
	}

	if value, ok := body["priority"]; ok || !partial {
		priority, err := parseEdgePriority(value)
		if err != nil {
			return err
		}
		body["priority"] = priority
	}

	if _, ok := body["condition_config"]; !ok && !partial {
		body["condition_config"] = Record{}
	}

	if config, ok := body["condition_config"]; ok && config == nil {
		body["condition_config"] = Record{}
	} else if ok {
		switch config.(type) {
		case Record, map[string]any:
		default:
			return errors.New("condition_config phải là object")
		}
	}

	conditionType := strings.TrimSpace(fmt.Sprint(body["condition_type"]))
	if validator, exists := edgeConditionConfigValidators[conditionType]; exists {
		if err := validator(recordFromAny(body["condition_config"])); err != nil {
			return err
		}
	}

	return nil
}

func parseEdgePriority(value any) (int, error) {
	if value == nil {
		return 100, nil
	}
	switch v := value.(type) {
	case int:
		return validateEdgePriority(v)
	case int32:
		return validateEdgePriority(int(v))
	case int64:
		return validateEdgePriority(int(v))
	case float64:
		if math.Trunc(v) != v {
			return 0, errors.New("priority phải là số nguyên")
		}
		return validateEdgePriority(int(v))
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, errors.New("priority là bắt buộc")
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, errors.New("priority phải là số nguyên")
		}
		return validateEdgePriority(parsed)
	default:
		return 0, errors.New("priority phải là số nguyên")
	}
}

func validateEdgePriority(priority int) (int, error) {
	if priority < 1 || priority > 999 {
		return 0, errors.New("priority phải nằm trong khoảng 1 đến 999")
	}
	return priority, nil
}
