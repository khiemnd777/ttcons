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

type ActionRequest struct {
	Comment string `json:"comment"`
	Reason  string `json:"reason"`
}

type InputInvoiceRequest struct {
	InvoiceNumber        string  `json:"invoice_number"`
	InvoiceSymbol        string  `json:"invoice_symbol"`
	InvoiceDate          string  `json:"invoice_date"`
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
	InvoiceNumber   string  `json:"invoice_number"`
	InvoiceSymbol   string  `json:"invoice_symbol"`
	InvoiceDate     string  `json:"invoice_date"`
	ClientName      string  `json:"client_name"`
	ClientTaxCode   string  `json:"client_tax_code"`
	AmountBeforeVAT float64 `json:"amount_before_vat"`
	VATAmount       float64 `json:"vat_amount"`
	TotalAmount     float64 `json:"total_amount"`
	PaymentID       string  `json:"payment_id"`
	DocumentID      string  `json:"document_id"`
	Notes           string  `json:"notes"`
}

type WorkItemActionRequest struct {
	Comment           string   `json:"comment"`
	Reason            string   `json:"reason"`
	CompletionPayload Record   `json:"completion_payload"`
	RequiredRoles     []string `json:"required_roles"`
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

type WorkflowFormSaveRequest struct {
	Answers Record `json:"answers"`
	Comment string `json:"comment"`
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
	WorkItemID           string                          `json:"work_item_id"`
	ProcurementRequestID string                          `json:"procurement_request_id"`
	WarehouseID          string                          `json:"warehouse_id"`
	SourceType           string                          `json:"source_type"`
	SupplierName         string                          `json:"supplier_name"`
	SubcontractorName    string                          `json:"subcontractor_name"`
	DeliveryReference    string                          `json:"delivery_reference"`
	ReceivedAt           string                          `json:"received_at"`
	Result               string                          `json:"result"`
	Comment              string                          `json:"comment"`
	Items                []IncomingAcceptanceItemRequest `json:"items"`
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
	ItemIndex      int    `json:"item_index"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Decision       string `json:"decision"`
	Severity       string `json:"severity"`
	RequiredAction string `json:"required_action"`
	DueDate        string `json:"due_date"`
	EvidenceID     string `json:"evidence_id"`
	Note           string `json:"note"`
	PhotoRequired  bool   `json:"photo_required"`
	PhotoURL       string `json:"photo_url"`
}

type WorkflowFormSignRequest struct {
	Purpose               string                        `json:"purpose"`
	Comment               string                        `json:"comment"`
	SignatureImageDataURL string                        `json:"signature_image_data_url"`
	SignatureStrokes      []WorkflowFormSignatureStroke `json:"signature_strokes"`
}

type BOQUnitPriceRequest struct {
	UnitPrice float64 `json:"unit_price"`
	Comment   string  `json:"comment"`
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
	ActivityType string                      `json:"activity_type"`
	Comment      string                      `json:"comment"`
	Visibility   string                      `json:"visibility"`
	ScopeType    string                      `json:"scope_type"`
	ScopeID      string                      `json:"scope_id"`
	WorkItemID   string                      `json:"work_item_id"`
	Mentions     []ActivityMentionRequest    `json:"mentions"`
	Attachments  []ActivityAttachmentRequest `json:"attachments"`
	Metadata     Record                      `json:"metadata"`
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
	ScopeType string
	ScopeID   string
	Limit     int
}

type ActivityAttentionFilter struct {
	Queue     string
	ProjectID string
	Limit     int
}

type StockActionRequest struct {
	ProjectID     string  `json:"project_id"`
	WarehouseID   string  `json:"warehouse_id"`
	MaterialID    string  `json:"material_id"`
	MovementType  string  `json:"movement_type"`
	Quantity      float64 `json:"quantity"`
	Unit          string  `json:"unit"`
	Reason        string  `json:"reason"`
	ReferenceType string  `json:"reference_type"`
	ReferenceID   string  `json:"reference_id"`
}
