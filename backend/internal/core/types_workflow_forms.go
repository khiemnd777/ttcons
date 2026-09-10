package core

type WorkflowFormSaveRequest struct {
	Answers         Record `json:"answers"`
	Comment         string `json:"comment"`
	ExpectedAnswers Record `json:"expected_answers"`
}

type WorkflowFormAmendRequest struct {
	Answers          Record `json:"answers"`
	Reason           string `json:"reason"`
	ExpectedRevision int    `json:"expected_revision"`
}

type PlanningDailyReportFilter struct {
	ProjectID  string
	DateFrom   string
	DateTo     string
	ReportedBy string
	Owner      string
	Status     string
	Search     string
}

type PlanningMeetingMinutesFilter struct {
	ProjectID string
	DateFrom  string
	DateTo    string
	Chair     string
	Owner     string
	Status    string
	Search    string
}

type PlanningChecklistFilter struct {
	ProjectID  string
	DateFrom   string
	DateTo     string
	CheckedBy  string
	Assignee   string
	ItemStatus string
	Evidence   string
	Status     string
	Search     string
}

type WorkflowFormTemplateRequest struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	FormType    string `json:"form_type"`
	Status      string `json:"status"`
}

type WorkflowFormVersionRequest struct {
	SourceVersionID    string                     `json:"source_version_id"`
	Schema             Record                     `json:"schema_json"`
	Validation         Record                     `json:"validation_json"`
	Fields             []WorkflowFormFieldRequest `json:"fields"`
	SchemaProvided     bool                       `json:"-"`
	ValidationProvided bool                       `json:"-"`
	FieldsProvided     bool                       `json:"-"`
}

type WorkflowFormFieldRequest struct {
	FieldKey            string `json:"field_key"`
	Label               string `json:"label"`
	FieldType           string `json:"field_type"`
	Required            bool   `json:"required"`
	Options             any    `json:"options_json"`
	Validation          Record `json:"validation_json"`
	HelpText            string `json:"help_text"`
	OrderIndex          int    `json:"order_index"`
	VisibilityCondition Record `json:"visibility_condition_json"`
}

type WorkflowFormSignRequest struct {
	Purpose               string                        `json:"purpose"`
	Comment               string                        `json:"comment"`
	SignatureImageDataURL string                        `json:"signature_image_data_url"`
	SignatureStrokes      []WorkflowFormSignatureStroke `json:"signature_strokes"`
}

type WorkflowFormSealApplyRequest struct {
	SealAssetID string `json:"seal_asset_id"`
	Purpose     string `json:"purpose"`
	Comment     string `json:"comment"`
}

type WorkflowFormPDFConfirmationPreferenceRequest struct {
	Purpose           string `json:"purpose"`
	IncludeSignatures bool   `json:"include_signatures"`
	IncludeSeals      bool   `json:"include_seals"`
}
