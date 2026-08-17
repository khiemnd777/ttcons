package core

import (
	"fmt"
	"strings"
	"time"
)

const (
	clientSiteRiskAssessmentFormCode = "client_site_risk_assessment"
	clientSiteRiskItemsFieldKey      = "risk_items"
	clientSiteRiskCategoryFieldKey   = "category"
	clientSiteRiskDefaultCategory    = "other"
)

func applyWorkflowFormAnswerDefaults(instance Record, answers Record) Record {
	if answers == nil || strings.TrimSpace(fmt.Sprint(instance["form_code"])) != clientSiteRiskAssessmentFormCode {
		return answers
	}
	rawItems, exists := answers[clientSiteRiskItemsFieldKey]
	if !exists {
		return answers
	}
	items, err := workflowRecordList(rawItems)
	if err != nil {
		return answers
	}

	defaultedItems := make([]Record, len(items))
	changed := false
	for index, item := range items {
		category := strings.TrimSpace(fmt.Sprint(item[clientSiteRiskCategoryFieldKey]))
		if category != "" && category != "<nil>" {
			defaultedItems[index] = item
			continue
		}
		defaultedItem := cloneWorkflowFormRecord(item)
		defaultedItem[clientSiteRiskCategoryFieldKey] = clientSiteRiskDefaultCategory
		defaultedItems[index] = defaultedItem
		changed = true
	}
	if !changed {
		return answers
	}
	defaultedAnswers := cloneWorkflowFormRecord(answers)
	defaultedAnswers[clientSiteRiskItemsFieldKey] = defaultedItems
	return defaultedAnswers
}

func workflowFormSuggestedAnswers(instance Record, fields []Record, actorName string, now time.Time) Record {
	suggested := Record{}
	for _, field := range fields {
		validation := recordFromAny(field["validation_json"])
		switch strings.TrimSpace(fmt.Sprint(validation["default_source"])) {
		case "current_date":
			suggested[fmt.Sprint(field["field_key"])] = now.Format("2006-01-02")
		case "actor_name":
			if value := strings.TrimSpace(actorName); value != "" {
				suggested[fmt.Sprint(field["field_key"])] = value
			}
		case "step_title":
			if value := strings.TrimSpace(fmt.Sprint(instance["work_item_title"])); value != "" && value != "<nil>" {
				suggested[fmt.Sprint(field["field_key"])] = value
			}
		}
	}
	return suggested
}

func applyWorkflowFormSuggestedAnswers(instance Record, fields []Record, answers Record, actorName string, now time.Time) Record {
	if answers == nil {
		answers = Record{}
	}
	result := cloneWorkflowFormRecord(answers)
	for key, value := range workflowFormSuggestedAnswers(instance, fields, actorName, now) {
		if current, exists := result[key]; exists && strings.TrimSpace(fmt.Sprint(current)) != "" {
			continue
		}
		result[key] = value
	}
	return result
}

func cloneWorkflowFormRecord(source Record) Record {
	cloned := make(Record, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
