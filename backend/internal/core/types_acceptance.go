package core

type AcceptanceRequest struct {
	WorkItemID               string                           `json:"work_item_id"`
	FormInstanceID           string                           `json:"form_instance_id"`
	PDFExportID              string                           `json:"pdf_export_id"`
	Type                     string                           `json:"type"`
	Title                    string                           `json:"title"`
	AcceptanceDate           string                           `json:"acceptance_date"`
	Location                 string                           `json:"location"`
	ClientRepresentative     string                           `json:"client_representative"`
	ContractorRepresentative string                           `json:"contractor_representative"`
	ScopeSummary             string                           `json:"scope_summary"`
	SignatureStatus          string                           `json:"signature_status"`
	DocumentStatus           string                           `json:"document_status"`
	Notes                    string                           `json:"notes"`
	Checklist                []AcceptanceChecklistItemRequest `json:"checklist"`
	WorkItemContext          WorkItemMutationContext          `json:"work_item_context"`
}

type AcceptanceChecklistItemRequest struct {
	ItemIndex      int      `json:"item_index"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Decision       string   `json:"decision"`
	Severity       string   `json:"severity"`
	RequiredAction string   `json:"required_action"`
	DueDate        string   `json:"due_date"`
	EvidenceID     string   `json:"evidence_id"`
	EvidenceIDs    []string `json:"evidence_ids"`
	Note           string   `json:"note"`
	PhotoRequired  bool     `json:"photo_required"`
	PhotoURL       string   `json:"photo_url"`
}
