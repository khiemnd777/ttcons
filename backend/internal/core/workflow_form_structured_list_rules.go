package core

import "fmt"

const (
	clientSurveyWorkPackageFormCode       = "client_survey_work_package"
	clientSurveyDimensionItemsFieldKey    = "dimension_items"
	clientSurveyDimensionAreaItemFieldKey = "area"
)

func effectiveWorkflowStructuredItemField(instance, field, spec Record) Record {
	if fmt.Sprint(instance["form_code"]) != clientSurveyWorkPackageFormCode ||
		fmt.Sprint(field["field_key"]) != clientSurveyDimensionItemsFieldKey ||
		fmt.Sprint(spec["key"]) != clientSurveyDimensionAreaItemFieldKey {
		return spec
	}

	effective := Record{}
	for key, value := range spec {
		effective[key] = value
	}
	effective["required"] = false
	delete(effective, "min_length")
	return effective
}
