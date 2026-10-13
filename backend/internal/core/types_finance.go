package core

type HandoverCreateRequest struct {
	HandoverCode           string                  `json:"handover_code"`
	FinalAcceptanceID      string                  `json:"final_acceptance_id"`
	WorkflowFormInstanceID string                  `json:"workflow_form_instance_id"`
	HandoverDate           string                  `json:"handover_date"`
	HandoverStage          string                  `json:"handover_stage"`
	ClientRepresentative   string                  `json:"client_representative"`
	CompanyRepresentative  string                  `json:"company_representative"`
	Notes                  string                  `json:"notes"`
	WorkItemContext        WorkItemMutationContext `json:"work_item_context"`
}

type HandoverChecklistItemRequest struct {
	Requirement    *string `json:"requirement"`
	PhysicalResult *string `json:"physical_result"`
	Defect         *string `json:"defect"`
	DueDate        *string `json:"due_date"`
	Status         string  `json:"status"`
	EvidenceID     string  `json:"evidence_id"`
	DocumentID     string  `json:"document_id"`
	Notes          string  `json:"notes"`
}

type PartnerFilter struct {
	Query  string
	Type   string
	Group  string
	Status string
}

type ProjectMemberMutationRequest struct {
	StaffID        string   `json:"staff_id"`
	RoleIDs        []string `json:"role_ids"`
	ProjectRoleIDs []string `json:"project_role_ids"`
	Notes          string   `json:"notes"`
}

type ProjectMemberBulkMutationRequest struct {
	Members []ProjectMemberMutationRequest `json:"members"`
}
