package core

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

type MyWorkflowActionsFilter struct {
	Queue     string
	ProjectID string
	Search    string
}

type MyAccountingWorkFilter struct {
	ProjectID string
	Status    string
	Search    string
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
