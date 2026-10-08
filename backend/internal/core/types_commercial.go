package core

type BOQUnitPriceRequest struct {
	UnitPrice float64 `json:"unit_price"`
	Comment   string  `json:"comment"`
}

type BOQReviseRequest struct {
	Reason string `json:"reason"`
}

type SupplierQuotationRequest struct {
	QuotationCode     string                         `json:"quotation_code"`
	SupplierPartnerID string                         `json:"supplier_partner_id"`
	QuotationDate     string                         `json:"quotation_date"`
	ValidUntil        string                         `json:"valid_until"`
	Currency          string                         `json:"currency"`
	PaymentTermsDays  int                            `json:"payment_terms_days"`
	LeadTimeDays      int                            `json:"lead_time_days"`
	DeliveryTerms     string                         `json:"delivery_terms"`
	WarrantyTerms     string                         `json:"warranty_terms"`
	Notes             string                         `json:"notes"`
	Items             []SupplierQuotationItemRequest `json:"items"`
	WorkItemContext   WorkItemMutationContext        `json:"work_item_context"`
}

type SupplierQuotationItemRequest struct {
	BOQItemID        string  `json:"boq_item_id"`
	Description      string  `json:"description"`
	Unit             string  `json:"unit"`
	Quantity         float64 `json:"quantity"`
	UnitPrice        float64 `json:"unit_price"`
	DiscountAmount   float64 `json:"discount_amount"`
	TaxRate          float64 `json:"tax_rate"`
	LeadTimeDays     int     `json:"lead_time_days"`
	ComplianceStatus string  `json:"compliance_status"`
	DeviationNote    string  `json:"deviation_note"`
}

type SupplierQuotationQualificationRequest struct {
	TechnicalStatus  string                  `json:"technical_status"`
	CommercialStatus string                  `json:"commercial_status"`
	Comment          string                  `json:"comment"`
	WorkItemContext  WorkItemMutationContext `json:"work_item_context"`
}

type SupplierQuoteSelectionRequest struct {
	DecisionReason  string                  `json:"decision_reason"`
	WorkItemContext WorkItemMutationContext `json:"work_item_context"`
}

type ContractRequest struct {
	ContractCode          string                  `json:"contract_code"`
	Title                 string                  `json:"title"`
	ContractType          string                  `json:"contract_type"`
	Direction             string                  `json:"direction"`
	CounterpartyPartnerID string                  `json:"counterparty_partner_id"`
	CounterpartyName      string                  `json:"counterparty_name"`
	Currency              string                  `json:"currency"`
	OriginalAmount        float64                 `json:"original_amount"`
	AdvancePercent        float64                 `json:"advance_percent"`
	RetentionPercent      float64                 `json:"retention_percent"`
	WarrantyDays          int                     `json:"warranty_days"`
	SignedDate            string                  `json:"signed_date"`
	EffectiveDate         string                  `json:"effective_date"`
	ExpiryDate            string                  `json:"expiry_date"`
	WorkItemContext       WorkItemMutationContext `json:"work_item_context"`
}

type ContractVersionRequest struct {
	VersionLabel     string                  `json:"version_label"`
	Amount           float64                 `json:"amount"`
	ScopeSummary     string                  `json:"scope_summary"`
	CommercialTerms  Record                  `json:"commercial_terms"`
	PaymentTerms     Record                  `json:"payment_terms"`
	PlannedStartDate string                  `json:"planned_start_date"`
	PlannedEndDate   string                  `json:"planned_end_date"`
	DocumentID       string                  `json:"document_id"`
	Comment          string                  `json:"comment"`
	WorkItemContext  WorkItemMutationContext `json:"work_item_context"`
}

type ContractVersionSignRequest struct {
	WorkflowFormSignatureID string                  `json:"workflow_form_signature_id"`
	Comment                 string                  `json:"comment"`
	WorkItemContext         WorkItemMutationContext `json:"work_item_context"`
}

type ContractDispatchRequest struct {
	ContractVersionID string                  `json:"contract_version_id"`
	RecipientName     string                  `json:"recipient_name"`
	DeliveryMethod    string                  `json:"delivery_method"`
	TrackingReference string                  `json:"tracking_reference"`
	SentDocumentID    string                  `json:"sent_document_id"`
	Note              string                  `json:"note"`
	WorkItemContext   WorkItemMutationContext `json:"work_item_context"`
}

type ContractDispatchReceiveRequest struct {
	ReturnedScanDocumentID string                  `json:"returned_scan_document_id"`
	Note                   string                  `json:"note"`
	WorkItemContext        WorkItemMutationContext `json:"work_item_context"`
}

type ContractMilestoneRequest struct {
	MilestoneCode      string  `json:"milestone_code"`
	MilestoneName      string  `json:"milestone_name"`
	SequenceNo         int     `json:"sequence_no"`
	MilestoneType      string  `json:"milestone_type"`
	AmountBasis        string  `json:"amount_basis"`
	Percentage         float64 `json:"percentage"`
	GrossAmount        float64 `json:"gross_amount"`
	RetentionPercent   float64 `json:"retention_percent"`
	PlannedDate        string  `json:"planned_date"`
	DueDays            int     `json:"due_days"`
	AcceptanceRequired bool    `json:"acceptance_required"`
	AcceptanceID       string  `json:"acceptance_id"`
}

type ContractVariationRequest struct {
	VariationCode              string  `json:"variation_code"`
	Title                      string  `json:"title"`
	VariationType              string  `json:"variation_type"`
	Reason                     string  `json:"reason"`
	AmountDelta                float64 `json:"amount_delta"`
	DaysDelta                  int     `json:"days_delta"`
	ScopeDelta                 string  `json:"scope_delta"`
	DocumentID                 string  `json:"document_id"`
	Comment                    string  `json:"comment"`
	Priority                   string  `json:"priority"`
	SourceWorkItemID           string  `json:"source_work_item_id"`
	SourceConstructionActionID string  `json:"source_construction_action_id"`
}

type QualityITPRequest struct {
	ITPCode       string                  `json:"itp_code"`
	Title         string                  `json:"title"`
	Discipline    string                  `json:"discipline"`
	WorkPackage   string                  `json:"work_package"`
	DocumentID    string                  `json:"document_id"`
	ReviewComment string                  `json:"review_comment"`
	Items         []QualityITPItemRequest `json:"items"`
}

type QualityITPItemRequest struct {
	SequenceNo            int    `json:"sequence_no"`
	ActivityName          string `json:"activity_name"`
	InspectionStage       string `json:"inspection_stage"`
	AcceptanceCriteria    string `json:"acceptance_criteria"`
	InspectionMethod      string `json:"inspection_method"`
	ControlPoint          string `json:"control_point"`
	FrequencyType         string `json:"frequency_type"`
	FrequencyValue        int    `json:"frequency_value"`
	ResponsibleRole       string `json:"responsible_role"`
	EvidenceRequired      bool   `json:"evidence_required"`
	ClientWitnessRequired bool   `json:"client_witness_required"`
	Notes                 string `json:"notes"`
}

type QualityInspectionRequest struct {
	RFICode               string `json:"rfi_code"`
	ITPItemID             string `json:"itp_item_id"`
	WorkItemID            string `json:"work_item_id"`
	Location              string `json:"location"`
	LotReference          string `json:"lot_reference"`
	RequestedInspectionAt string `json:"requested_inspection_at"`
	RequestComment        string `json:"request_comment"`
	RequestEvidenceID     string `json:"request_evidence_id"`
}

type QualityInspectionDecisionRequest struct {
	Result               string `json:"result"`
	Comment              string `json:"comment"`
	InspectionEvidenceID string `json:"inspection_evidence_id"`
}

type QualityNCRRequest struct {
	NCRCode         string `json:"ncr_code"`
	SourceType      string `json:"source_type"`
	SourceID        string `json:"source_id"`
	RFIID           string `json:"rfi_id"`
	WorkItemID      string `json:"work_item_id"`
	IssueID         string `json:"issue_id"`
	Title           string `json:"title"`
	Severity        string `json:"severity"`
	Nonconformity   string `json:"nonconformity"`
	ImmediateAction string `json:"immediate_action"`
	DueDate         string `json:"due_date"`
	EvidenceID      string `json:"evidence_id"`
	OwnerID         string `json:"owner_id"`
}

type QualityNCRActionRequest struct {
	RootCause            string `json:"root_cause"`
	CorrectiveAction     string `json:"corrective_action"`
	PreventiveAction     string `json:"preventive_action"`
	CorrectiveEvidenceID string `json:"corrective_evidence_id"`
	ReinspectionRFIID    string `json:"reinspection_rfi_id"`
	Comment              string `json:"comment"`
}
