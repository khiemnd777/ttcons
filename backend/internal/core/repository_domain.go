package core

const clientOfficialWorkflowTemplateID = "40000000-0000-0000-0000-000000000020"

type workflowAutomationOutcome struct {
	ActionKey     string
	Status        string
	Message       string
	TargetType    string
	TargetID      string
	BlockerReason string
	ErrorMessage  string
	Metadata      Record
}
