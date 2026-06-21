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
