package core

import "time"

// BMGSWorkbookData is a source-authorized, immutable-in-memory export projection.
// The renderer must not retrieve live records or storage objects itself.
type BMGSWorkbookData struct {
	TemplateCode string
	ProjectCode  string
	ProjectName  string
	Address      string
	PreparedBy   string
	PeriodLabel  string
	GeneratedAt  time.Time
	Sheets       []BMGSSheet
	Images       []BMGSImage
}

type BMGSSheet struct {
	Name           string
	DateLabel      string
	Title          string
	Metadata       Record
	Rows           []Record
	CrewRows       []Record
	MaterialRows   []Record
	CriteriaRows   []Record
	SourceSections []BMGSSourceSection
}

// SourceSections retain native report facts whose meaning differs from a client
// column (for example, material usage is not a delivery and confirmed is not measured).
type BMGSSourceSection struct {
	Title string
	Lines []string
}

type BMGSImage struct {
	EvidenceID        string
	Filename          string
	Caption           string
	Location          string
	CapturedAt        string
	Phase             string
	SourceLabel       string
	SourceRevisionKey string
	DerivativeNote    string
	MIMEType          string
	Bytes             []byte
	Width             int
	Height            int
}
