package core

import (
	"fmt"
	"strings"
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

func cloneWorkflowFormRecord(source Record) Record {
	cloned := make(Record, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
