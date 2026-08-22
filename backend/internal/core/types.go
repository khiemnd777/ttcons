package core

type Record map[string]any

type ColumnSpec struct {
	Name string
	Cast string
}

func col(name string) ColumnSpec {
	return ColumnSpec{Name: name}
}

func jsonCol(name string) ColumnSpec {
	return ColumnSpec{Name: name, Cast: "jsonb"}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UpdateOwnProfileRequest struct {
	FullName        string `json:"full_name"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
	CurrentPassword string `json:"current_password"`
}

type ChangeOwnPasswordRequest struct {
	CurrentPassword         string `json:"current_password"`
	NewPassword             string `json:"new_password"`
	NewPasswordConfirmation string `json:"new_password_confirmation"`
}

type CreateStaffRequest struct {
	FullName      string `json:"full_name"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	Department    string `json:"department"`
	Position      string `json:"position"`
	Status        string `json:"status"`
	Password      string `json:"password"`
	RecoveryEmail string `json:"recovery_email"`
}

type StaffAccountImportRowRequest struct {
	RowNumber     int    `json:"row_number"`
	FullName      string `json:"full_name"`
	LoginEmail    string `json:"login_email"`
	Password      string `json:"password"`
	RecoveryEmail string `json:"recovery_email"`
	Phone         string `json:"phone"`
	Department    string `json:"department"`
	Position      string `json:"position"`
	RoleCode      string `json:"role_code"`
}

type StaffAccountImportPreviewRequest struct {
	SourceFilename  string                         `json:"source_filename"`
	SourceSizeBytes int64                          `json:"source_size_bytes"`
	Rows            []StaffAccountImportRowRequest `json:"rows"`
}

type StaffAccountImportExecuteRequest struct {
	StaffAccountImportPreviewRequest
	PreviewHash    string `json:"preview_hash"`
	IdempotencyKey string `json:"idempotency_key"`
}

type StaffDeletionPreviewRequest struct {
	StaffIDs []string `json:"staff_ids"`
}

type StaffDeletionExecuteRequest struct {
	StaffDeletionPreviewRequest
	PreviewHash    string `json:"preview_hash"`
	IdempotencyKey string `json:"idempotency_key"`
}

type UpdateStaffRequest struct {
	UserID                  *string `json:"user_id"`
	FullName                *string `json:"full_name"`
	Email                   *string `json:"email"`
	Phone                   *string `json:"phone"`
	Department              *string `json:"department"`
	Position                *string `json:"position"`
	Status                  *string `json:"status"`
	NewPassword             string  `json:"new_password"`
	NewPasswordConfirmation string  `json:"new_password_confirmation"`
	RecoveryEmail           string  `json:"recovery_email"`
}

type UpdateRecoveryEmailRequest struct {
	RecoveryEmail   string `json:"recovery_email"`
	CurrentPassword string `json:"current_password"`
}

type RemoveRecoveryEmailRequest struct {
	CurrentPassword string `json:"current_password"`
}

type VerifyRecoveryEmailRequest struct {
	Token string `json:"token"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token                   string `json:"token"`
	NewPassword             string `json:"new_password"`
	NewPasswordConfirmation string `json:"new_password_confirmation"`
}

type ActionRequest struct {
	Comment string `json:"comment"`
	Reason  string `json:"reason"`
}

type PipelineTemplateLayoutRequest struct {
	Nodes []PipelineTemplateNodeLayoutRequest `json:"nodes"`
	Edges []PipelineTemplateEdgeLayoutRequest `json:"edges"`
}

type PipelineTemplateNodeLayoutRequest struct {
	ID        string `json:"id"`
	PositionX int    `json:"position_x"`
	PositionY int    `json:"position_y"`
}

type PipelineTemplateEdgeLayoutRequest struct {
	ID         string `json:"id"`
	SourceSide string `json:"source_side"`
	TargetSide string `json:"target_side"`
}

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
	HandoverCode           string `json:"handover_code"`
	FinalAcceptanceID      string `json:"final_acceptance_id"`
	WorkflowFormInstanceID string `json:"workflow_form_instance_id"`
	HandoverDate           string `json:"handover_date"`
	ClientRepresentative   string `json:"client_representative"`
	CompanyRepresentative  string `json:"company_representative"`
	Notes                  string `json:"notes"`
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

type WorkItemActionRequest struct {
	Comment                 string   `json:"comment"`
	Reason                  string   `json:"reason"`
	CompletionPayload       Record   `json:"completion_payload"`
	RequiredRoles           []string `json:"required_roles"`
	BypassStartRequirements bool     `json:"bypass_start_requirements"`
	StartRequirementsToken  string   `json:"start_requirements_token"`
	StartOverrideMetadata   Record   `json:"-"`
}

type MyWorkItemsFilter struct {
	Queue     string
	ProjectID string
	Status    string
	Deadline  string
	WorkType  string
	Priority  string
	Search    string
	Sort      string
	Bucket    string
	Cursor    string
	Limit     int
}

type PendingApprovalFilter struct {
	ProjectID     string
	DecidableOnly bool
}

type WorkItemEvidenceRequest struct {
	EvidenceType    string `json:"evidence_type"`
	DocumentType    string `json:"document_type"`
	Name            string `json:"name"`
	DocumentID      string `json:"document_id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	FileURL         string `json:"file_url"`
	MIMEType        string `json:"mime_type"`
	FileSize        int64  `json:"file_size"`
	StorageProvider string `json:"storage_provider"`
	StorageKey      string `json:"storage_key"`
	ChecksumSHA256  string `json:"checksum_sha256"`
	Caption         string `json:"caption"`
	RevisionNo      string `json:"revision_no"`
	Visibility      string `json:"visibility"`
	Payload         Record `json:"payload"`
}

type ConstructionDailyReportCreateRequest struct {
	ReportDate        string                                   `json:"report_date"`
	Weather           string                                   `json:"weather"`
	ProgressPercent   int                                      `json:"progress_percent"`
	WorkItemID        string                                   `json:"work_item_id"`
	DoneItems         []ConstructionDailyReportWorkItemRequest `json:"done_items"`
	CurrentItems      []ConstructionDailyReportWorkItemRequest `json:"current_items"`
	NextItems         []ConstructionDailyReportWorkItemRequest `json:"next_items"`
	Blockers          []ConstructionDailyReportBlockerRequest  `json:"blockers"`
	FieldNotes        string                                   `json:"field_notes"`
	CrewItems         []ConstructionDailyReportCrewRequest     `json:"crew_items"`
	MaterialItems     []ConstructionDailyReportMaterialRequest `json:"material_items"`
	EvidenceIDs       []string                                 `json:"evidence_ids"`
	PrimaryEvidenceID string                                   `json:"primary_evidence_id"`
	ManpowerSummary   string                                   `json:"manpower_summary"`
	MaterialSummary   string                                   `json:"material_usage_summary"`
}

type ConstructionDailyReportWorkItemRequest struct {
	Title string `json:"title"`
}

type ConstructionDailyReportBlockerRequest struct {
	Content string `json:"content"`
}

type ConstructionDailyReportCrewRequest struct {
	CrewName    string  `json:"crew_name"`
	WorkerCount int     `json:"worker_count"`
	WorkHours   float64 `json:"work_hours"`
	Note        string  `json:"note"`
}

type ConstructionDailyReportMaterialRequest struct {
	MaterialName string  `json:"material_name"`
	Quantity     float64 `json:"quantity"`
	Unit         string  `json:"unit"`
	Note         string  `json:"note"`
}

type WorkflowFormSaveRequest struct {
	Answers Record `json:"answers"`
	Comment string `json:"comment"`
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
	SourceVersionID string                     `json:"source_version_id"`
	Schema          Record                     `json:"schema_json"`
	Validation      Record                     `json:"validation_json"`
	Fields          []WorkflowFormFieldRequest `json:"fields"`
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

type SubcontractorReconciliationRequest struct {
	Comment string                                   `json:"comment"`
	Items   []SubcontractorReconciliationItemRequest `json:"items"`
}

type SubcontractorReconciliationItemRequest struct {
	ItemIndex         int      `json:"item_index"`
	BOQItemID         string   `json:"boq_item_id"`
	ConfirmedQuantity *float64 `json:"confirmed_quantity"`
	Decision          string   `json:"decision"`
	Comment           string   `json:"comment"`
}

type OwnerClientReconciliationRequest struct {
	Comment string                                 `json:"comment"`
	Items   []OwnerClientReconciliationItemRequest `json:"items"`
}

type OwnerClientReconciliationItemRequest struct {
	ItemIndex    int    `json:"item_index"`
	AcceptanceID string `json:"acceptance_id"`
	Decision     string `json:"decision"`
	Comment      string `json:"comment"`
}

type MaterialSnapshotRequest struct {
	SnapshotDate     string                        `json:"snapshot_date"`
	ConstructionArea string                        `json:"construction_area"`
	Comment          string                        `json:"comment"`
	Items            []MaterialSnapshotItemRequest `json:"items"`
}

type MaterialSnapshotItemRequest struct {
	ItemIndex          int      `json:"item_index"`
	MaterialID         string   `json:"material_id"`
	MaterialName       string   `json:"material_name"`
	Unit               string   `json:"unit"`
	Source             string   `json:"source"`
	BeforeQuantity     *float64 `json:"before_quantity"`
	AddedQuantity      *float64 `json:"added_quantity"`
	AfterQuantity      *float64 `json:"after_quantity"`
	WasteQuantity      *float64 `json:"waste_quantity"`
	AffectsSiteBalance *bool    `json:"affects_site_balance"`
	BeforeEvidenceID   string   `json:"before_evidence_id"`
	AfterEvidenceID    string   `json:"after_evidence_id"`
	ConstructionArea   string   `json:"construction_area"`
	Condition          string   `json:"condition"`
	Comment            string   `json:"comment"`
}

type SurplusHandlingRequest struct {
	SiteBalanceID          string   `json:"site_balance_id"`
	MaterialSnapshotID     string   `json:"material_snapshot_id"`
	MaterialSnapshotItemID string   `json:"material_snapshot_item_id"`
	HandlingType           string   `json:"handling_type"`
	Quantity               *float64 `json:"quantity"`
	Unit                   string   `json:"unit"`
	WarehouseID            string   `json:"warehouse_id"`
	CustodyLocation        string   `json:"custody_location"`
	Condition              string   `json:"condition"`
	EvidenceID             string   `json:"evidence_id"`
	Comment                string   `json:"comment"`
}

type IncomingAcceptanceRequest struct {
	WorkItemID             string                          `json:"work_item_id"`
	ProcurementRequestID   string                          `json:"procurement_request_id"`
	WarehouseID            string                          `json:"warehouse_id"`
	SourceType             string                          `json:"source_type"`
	SupplierPartnerID      string                          `json:"supplier_partner_id"`
	SupplierName           string                          `json:"supplier_name"`
	SubcontractorPartnerID string                          `json:"subcontractor_partner_id"`
	SubcontractorName      string                          `json:"subcontractor_name"`
	DeliveryReference      string                          `json:"delivery_reference"`
	ReceivedAt             string                          `json:"received_at"`
	Result                 string                          `json:"result"`
	Comment                string                          `json:"comment"`
	Items                  []IncomingAcceptanceItemRequest `json:"items"`
}

type IncomingAcceptanceItemRequest struct {
	ItemIndex                int      `json:"item_index"`
	MaterialID               string   `json:"material_id"`
	MaterialName             string   `json:"material_name"`
	ProcurementRequestItemID string   `json:"procurement_request_item_id"`
	DeliveredQuantity        *float64 `json:"delivered_quantity"`
	AcceptedQuantity         *float64 `json:"accepted_quantity"`
	RejectedQuantity         *float64 `json:"rejected_quantity"`
	Unit                     string   `json:"unit"`
	Condition                string   `json:"condition"`
	DefectNote               string   `json:"defect_note"`
	EvidenceID               string   `json:"evidence_id"`
	Decision                 string   `json:"decision"`
	Comment                  string   `json:"comment"`
}

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
	TechnicalStatus  string `json:"technical_status"`
	CommercialStatus string `json:"commercial_status"`
	Comment          string `json:"comment"`
}

type SupplierQuoteSelectionRequest struct {
	DecisionReason string `json:"decision_reason"`
}

type ContractRequest struct {
	ContractCode          string  `json:"contract_code"`
	Title                 string  `json:"title"`
	ContractType          string  `json:"contract_type"`
	Direction             string  `json:"direction"`
	CounterpartyPartnerID string  `json:"counterparty_partner_id"`
	CounterpartyName      string  `json:"counterparty_name"`
	Currency              string  `json:"currency"`
	OriginalAmount        float64 `json:"original_amount"`
	AdvancePercent        float64 `json:"advance_percent"`
	RetentionPercent      float64 `json:"retention_percent"`
	WarrantyDays          int     `json:"warranty_days"`
	SignedDate            string  `json:"signed_date"`
	EffectiveDate         string  `json:"effective_date"`
	ExpiryDate            string  `json:"expiry_date"`
}

type ContractVersionRequest struct {
	VersionLabel     string  `json:"version_label"`
	Amount           float64 `json:"amount"`
	ScopeSummary     string  `json:"scope_summary"`
	CommercialTerms  Record  `json:"commercial_terms"`
	PaymentTerms     Record  `json:"payment_terms"`
	PlannedStartDate string  `json:"planned_start_date"`
	PlannedEndDate   string  `json:"planned_end_date"`
	DocumentID       string  `json:"document_id"`
	Comment          string  `json:"comment"`
}

type ContractVersionSignRequest struct {
	WorkflowFormSignatureID string `json:"workflow_form_signature_id"`
	Comment                 string `json:"comment"`
}

type ContractDispatchRequest struct {
	ContractVersionID string `json:"contract_version_id"`
	RecipientName     string `json:"recipient_name"`
	DeliveryMethod    string `json:"delivery_method"`
	TrackingReference string `json:"tracking_reference"`
	SentDocumentID    string `json:"sent_document_id"`
	Note              string `json:"note"`
}

type ContractDispatchReceiveRequest struct {
	ReturnedScanDocumentID string `json:"returned_scan_document_id"`
	Note                   string `json:"note"`
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

type ContractRetentionReleaseRequest struct {
	Amount  float64 `json:"amount"`
	DueDate string  `json:"due_date"`
	Comment string  `json:"comment"`
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

type ConstructionRecurringPlanRequest struct {
	PlanCode         string `json:"plan_code"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	RecurrenceType   string `json:"recurrence_type"`
	IntervalValue    int    `json:"interval_value"`
	StartDate        string `json:"start_date"`
	EndDate          string `json:"end_date"`
	DueTime          string `json:"due_time"`
	ResponsibleRole  string `json:"responsible_role"`
	AssignedUserID   string `json:"assigned_user_id"`
	EvidenceRequired bool   `json:"evidence_required"`
	SourceWorkItemID string `json:"source_work_item_id"`
}

type ConstructionRecurringOccurrenceRequest struct {
	Comment    string `json:"comment"`
	EvidenceID string `json:"evidence_id"`
	Reason     string `json:"reason"`
}

type ConstructionCheckCriterion struct {
	Title            string `json:"title"`
	Criteria         string `json:"criteria"`
	EvidenceRequired bool   `json:"evidence_required"`
}

type ConstructionHandoffItemRequest struct {
	SourceFieldKey   string                       `json:"source_field_key"`
	SourceItemIndex  int                          `json:"source_item_index"`
	Title            string                       `json:"title"`
	Location         string                       `json:"location"`
	ScheduledAt      string                       `json:"scheduled_at"`
	ResponsibleRole  string                       `json:"responsible_role"`
	AssignedUserID   string                       `json:"assigned_user_id"`
	EvidenceRequired bool                         `json:"evidence_required"`
	Criteria         []ConstructionCheckCriterion `json:"criteria"`
}

type ConstructionHandoffActivateRequest struct {
	Title                string                           `json:"title"`
	Notes                string                           `json:"notes"`
	SourceRunNodeID      string                           `json:"source_run_node_id"`
	SourceFormInstanceID string                           `json:"source_form_instance_id"`
	SourceFormRevisionNo int                              `json:"source_form_revision_no"`
	Items                []ConstructionHandoffItemRequest `json:"items"`
}

type ConstructionCheckTemplateRequest struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Criteria    []ConstructionCheckCriterion `json:"criteria"`
}

type ConstructionCheckRequest struct {
	WorkItemID          string                       `json:"work_item_id"`
	SourceDailyReportID string                       `json:"source_daily_report_id"`
	TemplateID          string                       `json:"template_id"`
	Title               string                       `json:"title"`
	Location            string                       `json:"location"`
	ScheduledAt         string                       `json:"scheduled_at"`
	Criteria            []ConstructionCheckCriterion `json:"criteria"`
	ContextEvidenceIDs  []string                     `json:"context_evidence_ids"`
}

type ConstructionCheckUpdateRequest struct {
	WorkItemID         string                       `json:"work_item_id"`
	TemplateID         string                       `json:"template_id"`
	Title              string                       `json:"title"`
	Location           string                       `json:"location"`
	ScheduledAt        string                       `json:"scheduled_at"`
	Criteria           []ConstructionCheckCriterion `json:"criteria"`
	ContextEvidenceIDs []string                     `json:"context_evidence_ids"`
}

type ConstructionCheckCompleteRequest struct {
	Result         string   `json:"result"`
	Note           string   `json:"note"`
	EvidenceID     string   `json:"evidence_id"`
	EvidenceIDs    []string `json:"evidence_ids"`
	ActionTitle    string   `json:"action_title"`
	AssignedUserID string   `json:"assigned_user_id"`
	DueAt          string   `json:"due_at"`
	ImpactScope    string   `json:"impact_scope"`
	Priority       string   `json:"priority"`
}

type ConstructionActionRequest struct {
	WorkItemID          string `json:"work_item_id"`
	SourceDailyReportID string `json:"source_daily_report_id"`
	Title               string `json:"title"`
	Description         string `json:"description"`
	Priority            string `json:"priority"`
	AssignedUserID      string `json:"assigned_user_id"`
	DueAt               string `json:"due_at"`
	ImpactScope         string `json:"impact_scope"`
	EvidenceID          string `json:"evidence_id"`
}

type ConstructionActionUpdateRequest struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	Priority       string `json:"priority"`
	AssignedUserID string `json:"assigned_user_id"`
	DueAt          string `json:"due_at"`
}

type ConstructionActionResolutionRequest struct {
	EvidenceID string `json:"evidence_id"`
	Note       string `json:"note"`
}

type ConstructionActionVerifyRequest struct {
	Result  string `json:"result"`
	Comment string `json:"comment"`
}

type WorkflowFormSignatureStroke struct {
	Points []WorkflowFormSignaturePoint `json:"points"`
}

type WorkflowFormSignaturePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type StoredFileMetadata struct {
	StorageProvider string `json:"storage_provider"`
	StorageKey      string `json:"storage_key"`
	MIMEType        string `json:"mime_type"`
	FileSize        int64  `json:"file_size"`
	ChecksumSHA256  string `json:"checksum_sha256"`
}

type WorkItemActivityRequest struct {
	ActivityType     string                      `json:"activity_type"`
	Comment          string                      `json:"comment"`
	Visibility       string                      `json:"visibility"`
	ScopeType        string                      `json:"scope_type"`
	ScopeID          string                      `json:"scope_id"`
	WorkItemID       string                      `json:"work_item_id"`
	ParentActivityID string                      `json:"parent_activity_id"`
	Mentions         []ActivityMentionRequest    `json:"mentions"`
	Attachments      []ActivityAttachmentRequest `json:"attachments"`
	Metadata         Record                      `json:"metadata"`
}

type ActivityReactionRequest struct {
	Reaction string `json:"reaction"`
	Active   bool   `json:"active"`
}

type ActivityMessageUpdateRequest struct {
	Comment string `json:"comment"`
}

type ActivityMentionRequest struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

type ActivityAttachmentRequest struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

type ProjectActivityFilter struct {
	ScopeType       string
	ScopeID         string
	ActivityType    string
	ActorID         string
	Visibility      string
	Kind            string
	Attention       string
	Search          string
	Cursor          string
	CursorCreatedAt string
	CursorID        string
	Limit           int
}

type ProjectEvidenceFilter struct {
	EvidenceType    string
	Status          string
	WorkItemID      string
	SubmittedBy     string
	Visibility      string
	DateFrom        string
	DateTo          string
	Search          string
	Cursor          string
	CursorPriority  int
	CursorCreatedAt string
	CursorID        string
	Limit           int
}

type AuditLogFilter struct {
	ProjectID  string
	EventType  string
	TargetType string
	ActorName  string
	DateFrom   string
	DateTo     string
	Search     string
	Page       int
	PageSize   int
}

type ActivityResolutionRequest struct {
	Comment string `json:"comment"`
}

type ProjectOperationsFilter struct {
	View string
}

type ActivityAttentionFilter struct {
	Queue     string
	ProjectID string
	Limit     int
}

type StockActionRequest struct {
	ProjectID         string  `json:"project_id"`
	WarehouseID       string  `json:"warehouse_id"`
	TargetWarehouseID string  `json:"target_warehouse_id"`
	MaterialID        string  `json:"material_id"`
	MovementType      string  `json:"movement_type"`
	Quantity          float64 `json:"quantity"`
	Unit              string  `json:"unit"`
	Reason            string  `json:"reason"`
	ReferenceType     string  `json:"reference_type"`
	ReferenceID       string  `json:"reference_id"`
	StockDocumentID   string  `json:"-"`
}

type StockDocumentItemRequest struct {
	MaterialID     string  `json:"material_id"`
	Quantity       float64 `json:"quantity"`
	Unit           string  `json:"unit"`
	Reason         string  `json:"reason"`
	SourceLineType string  `json:"source_line_type"`
	SourceLineID   string  `json:"source_line_id"`
}

type StockDocumentRequest struct {
	ProjectID         string                     `json:"project_id"`
	MovementType      string                     `json:"movement_type"`
	SourceWarehouseID string                     `json:"source_warehouse_id"`
	TargetWarehouseID string                     `json:"target_warehouse_id"`
	ReferenceType     string                     `json:"reference_type"`
	ReferenceID       string                     `json:"reference_id"`
	DocumentDate      string                     `json:"document_date"`
	Note              string                     `json:"note"`
	Items             []StockDocumentItemRequest `json:"items"`
}
