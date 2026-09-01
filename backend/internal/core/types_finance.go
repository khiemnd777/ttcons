package core

type PayableCreateRequest struct {
	ProjectID       string  `json:"project_id"`
	BudgetID        string  `json:"budget_id"`
	CostAccountID   string  `json:"cost_account_id"`
	PayableCode     string  `json:"payable_code"`
	VendorType      string  `json:"vendor_type"`
	VendorPartnerID string  `json:"vendor_partner_id"`
	VendorName      string  `json:"vendor_name"`
	GrossAmount     float64 `json:"gross_amount"`
	AdvanceAmount   float64 `json:"advance_amount"`
	RetentionAmount float64 `json:"retention_amount"`
	DeductionAmount float64 `json:"deduction_amount"`
	DueDate         string  `json:"due_date"`
}

type ReceivableCreateRequest struct {
	ProjectID        string  `json:"project_id"`
	RevenueAccountID string  `json:"revenue_account_id"`
	ReceivableCode   string  `json:"receivable_code"`
	ClientPartnerID  string  `json:"client_partner_id"`
	ClientName       string  `json:"client_name"`
	ContractAmount   float64 `json:"contract_amount"`
	MilestoneName    string  `json:"milestone_name"`
	GrossAmount      float64 `json:"gross_amount"`
	DueDate          string  `json:"due_date"`
}

type HandoverCreateRequest struct {
	HandoverCode           string                  `json:"handover_code"`
	FinalAcceptanceID      string                  `json:"final_acceptance_id"`
	WorkflowFormInstanceID string                  `json:"workflow_form_instance_id"`
	HandoverDate           string                  `json:"handover_date"`
	ClientRepresentative   string                  `json:"client_representative"`
	CompanyRepresentative  string                  `json:"company_representative"`
	Notes                  string                  `json:"notes"`
	WorkItemContext        WorkItemMutationContext `json:"work_item_context"`
}

type HandoverChecklistItemRequest struct {
	Status     string `json:"status"`
	EvidenceID string `json:"evidence_id"`
	DocumentID string `json:"document_id"`
	Notes      string `json:"notes"`
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

type InputInvoiceRequest struct {
	TaxCodeID            string  `json:"tax_code_id"`
	ExpenseAccountID     string  `json:"expense_account_id"`
	InvoiceNumber        string  `json:"invoice_number"`
	InvoiceSymbol        string  `json:"invoice_symbol"`
	InvoiceDate          string  `json:"invoice_date"`
	VendorPartnerID      string  `json:"vendor_partner_id"`
	VendorName           string  `json:"vendor_name"`
	VendorTaxCode        string  `json:"vendor_tax_code"`
	AmountBeforeVAT      float64 `json:"amount_before_vat"`
	VATAmount            float64 `json:"vat_amount"`
	TotalAmount          float64 `json:"total_amount"`
	ProcurementRequestID string  `json:"procurement_request_id"`
	DocumentID           string  `json:"document_id"`
	Notes                string  `json:"notes"`
}

type OutputInvoiceRequest struct {
	TaxCodeID        string  `json:"tax_code_id"`
	RevenueAccountID string  `json:"revenue_account_id"`
	InvoiceNumber    string  `json:"invoice_number"`
	InvoiceSymbol    string  `json:"invoice_symbol"`
	InvoiceDate      string  `json:"invoice_date"`
	ClientPartnerID  string  `json:"client_partner_id"`
	ClientName       string  `json:"client_name"`
	ClientTaxCode    string  `json:"client_tax_code"`
	AmountBeforeVAT  float64 `json:"amount_before_vat"`
	VATAmount        float64 `json:"vat_amount"`
	TotalAmount      float64 `json:"total_amount"`
	PaymentID        string  `json:"payment_id"`
	DocumentID       string  `json:"document_id"`
	Notes            string  `json:"notes"`
}

type BudgetCreateRequest struct {
	BudgetCode    string  `json:"budget_code"`
	Name          string  `json:"name"`
	PlannedAmount float64 `json:"planned_amount"`
}

type PaymentCreateRequest struct {
	ProjectID     string  `json:"project_id"`
	BudgetID      string  `json:"budget_id"`
	CashAccountID string  `json:"cash_account_id"`
	PaymentCode   string  `json:"payment_code"`
	PaymentType   string  `json:"payment_type"`
	TargetType    string  `json:"target_type"`
	TargetID      string  `json:"target_id"`
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"payment_method"`
}

type PayablePaymentScheduleRequest struct {
	ScheduleCode  string  `json:"schedule_code"`
	MilestoneName string  `json:"milestone_name"`
	ScheduleType  string  `json:"schedule_type"`
	PlannedAmount float64 `json:"planned_amount"`
	DueDate       string  `json:"due_date"`
	Notes         string  `json:"notes"`
}

type AdvanceRequestCreateRequest struct {
	RequestCode string  `json:"request_code"`
	Amount      float64 `json:"amount"`
	DueDate     string  `json:"due_date"`
	Reason      string  `json:"reason"`
	Notes       string  `json:"notes"`
}

type PayablePaymentCreateRequest struct {
	CashAccountID    string  `json:"cash_account_id"`
	PaymentCode      string  `json:"payment_code"`
	PaymentType      string  `json:"payment_type"`
	PaymentMethod    string  `json:"payment_method"`
	Amount           float64 `json:"amount"`
	ScheduleID       string  `json:"schedule_id"`
	AdvanceRequestID string  `json:"advance_request_id"`
	InputInvoiceID   string  `json:"input_invoice_id"`
	PaidAt           string  `json:"paid_at"`
	Notes            string  `json:"notes"`
}

type AdvanceSettlementRequest struct {
	Amount         float64 `json:"amount"`
	InputInvoiceID string  `json:"input_invoice_id"`
	ScheduleID     string  `json:"schedule_id"`
	Comment        string  `json:"comment"`
}

type ReceivableAdvanceReceiptCreateRequest struct {
	ReceiptCode string  `json:"receipt_code"`
	Amount      float64 `json:"amount"`
	DueDate     string  `json:"due_date"`
	Reason      string  `json:"reason"`
	Notes       string  `json:"notes"`
}

type ReceivableReceiptCreateRequest struct {
	CashAccountID    string  `json:"cash_account_id"`
	PaymentCode      string  `json:"payment_code"`
	PaymentType      string  `json:"payment_type"`
	PaymentMethod    string  `json:"payment_method"`
	Amount           float64 `json:"amount"`
	ScheduleID       string  `json:"schedule_id"`
	AdvanceReceiptID string  `json:"advance_receipt_id"`
	OutputInvoiceID  string  `json:"output_invoice_id"`
	PaidAt           string  `json:"paid_at"`
	Notes            string  `json:"notes"`
}

type ReceivableAdvanceSettlementRequest struct {
	Amount          float64 `json:"amount"`
	OutputInvoiceID string  `json:"output_invoice_id"`
	ScheduleID      string  `json:"schedule_id"`
	Comment         string  `json:"comment"`
}
