package core

type AccountingHandoffRequest struct {
	SourceType        string                  `json:"source_type"`
	SourceID          string                  `json:"source_id"`
	Note              string                  `json:"note"`
	ExternalReference string                  `json:"external_reference"`
	WorkItemContext   WorkItemMutationContext `json:"work_item_context"`
}

type AccountingHandoffActionRequest struct {
	Note              string                  `json:"note"`
	ReturnReason      string                  `json:"return_reason"`
	ExternalReference string                  `json:"external_reference"`
	WorkItemContext   WorkItemMutationContext `json:"work_item_context"`
}
