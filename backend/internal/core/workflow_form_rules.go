package core

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type workflowFormRuleEvaluator func(actual any, actualExists bool, expected any) bool

var workflowFormRuleEvaluators = map[string]workflowFormRuleEvaluator{
	"equals": func(actual any, actualExists bool, expected any) bool {
		return actualExists && workflowFormAnswerValueMatches(actual, expected)
	},
	"not_equals": func(actual any, actualExists bool, expected any) bool {
		return !actualExists || !workflowFormAnswerValueMatches(actual, expected)
	},
	"in": func(actual any, actualExists bool, expected any) bool {
		if !actualExists {
			return false
		}
		for _, candidate := range anySlice(expected) {
			if workflowFormAnswerValueMatches(actual, candidate) {
				return true
			}
		}
		return false
	},
	"exists": func(_ any, actualExists bool, _ any) bool {
		return actualExists
	},
	"not_exists": func(_ any, actualExists bool, _ any) bool {
		return !actualExists
	},
}

func evaluateWorkflowFormRule(raw any, answers Record, emptyResult bool) (bool, error) {
	rule := recordFromAny(raw)
	if len(rule) == 0 {
		return emptyResult, nil
	}
	operator := strings.TrimSpace(fmt.Sprint(rule["operator"]))
	fieldKey := strings.TrimSpace(fmt.Sprint(rule["field_key"]))
	evaluator, exists := workflowFormRuleEvaluators[operator]
	if !exists {
		return false, fmt.Errorf("toán tử quy tắc biểu mẫu không được hỗ trợ: %s", operator)
	}
	if fieldKey == "" || fieldKey == "<nil>" {
		return false, errors.New("quy tắc biểu mẫu thiếu field_key")
	}
	actual, actualExists := answers[fieldKey]
	if actualExists && workflowRuleValueEmpty(actual) {
		actualExists = false
	}
	return evaluator(actual, actualExists, rule["value"]), nil
}

func workflowFormFieldVisible(field Record, answers Record) (bool, error) {
	return evaluateWorkflowFormRule(field["visibility_condition_json"], answers, true)
}

func workflowFormFieldRequired(field Record, answers Record) (bool, error) {
	if boolValue(field["required"]) {
		return true, nil
	}
	validation := recordFromAny(field["validation_json"])
	return evaluateWorkflowFormRule(validation["required_when"], answers, false)
}

func validateWorkflowFormFieldRules(fields []Record) error {
	fieldKeys := map[string]bool{}
	for _, field := range fields {
		fieldKeys[fmt.Sprint(field["field_key"])] = true
	}
	for _, field := range fields {
		fieldKey := fmt.Sprint(field["field_key"])
		if err := validateWorkflowFormRuleDefinition(field["visibility_condition_json"], fieldKey, fieldKeys); err != nil {
			return validationError(err.Error(), Record{"field_key": fieldKey, "rule": "visibility_condition_json"})
		}
		validation := recordFromAny(field["validation_json"])
		if err := validateWorkflowFormRuleDefinition(validation["required_when"], fieldKey, fieldKeys); err != nil {
			return validationError(err.Error(), Record{"field_key": fieldKey, "rule": "validation_json.required_when"})
		}
	}
	return nil
}

func validateWorkflowFormRuleDefinition(raw any, ownerFieldKey string, fieldKeys map[string]bool) error {
	rule := recordFromAny(raw)
	if len(rule) == 0 {
		return nil
	}
	operator := strings.TrimSpace(fmt.Sprint(rule["operator"]))
	if _, exists := workflowFormRuleEvaluators[operator]; !exists {
		return fmt.Errorf("toán tử quy tắc biểu mẫu không được hỗ trợ: %s", operator)
	}
	fieldKey := strings.TrimSpace(fmt.Sprint(rule["field_key"]))
	if !fieldKeys[fieldKey] {
		return fmt.Errorf("quy tắc biểu mẫu tham chiếu field_key không tồn tại: %s", fieldKey)
	}
	if fieldKey == ownerFieldKey {
		return errors.New("quy tắc biểu mẫu không được tự tham chiếu chính trường của nó")
	}
	if statusIn(operator, "equals", "not_equals", "in") {
		if _, exists := rule["value"]; !exists {
			return errors.New("quy tắc biểu mẫu thiếu value")
		}
	}
	if operator == "in" && reflect.ValueOf(rule["value"]).Kind() != reflect.Slice {
		return errors.New("value của toán tử in phải là danh sách")
	}
	return nil
}

func workflowRuleValueEmpty(value any) bool {
	if value == nil {
		return true
	}
	text, isText := value.(string)
	return isText && strings.TrimSpace(text) == ""
}
