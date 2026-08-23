package core

const sampleDataBlueprintVersion = 2

var sampleDataPlanningProgressPresets = []int{5, 10, 15, 25, 40, 65, 90}

type SampleDataCreateRequest struct {
	Profile            string `json:"profile"`
	ProgressPercent    int    `json:"progress_percent"`
	PipelineTemplateID string `json:"pipeline_template_id"`
	AccountingBookID   string `json:"accounting_book_id"`
	IdempotencyKey     string `json:"idempotency_key"`
}

type SampleDataBlocker struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type SampleDataRoleCoverage struct {
	RoleName   string `json:"role_name"`
	StaffCount int    `json:"staff_count"`
	Ready      bool   `json:"ready"`
}

type SampleDataPipelineOption struct {
	ID                string                   `json:"id"`
	Name              string                   `json:"name"`
	Revision          int                      `json:"revision"`
	DefinitionHash    string                   `json:"definition_hash"`
	EngineVersion     int                      `json:"engine_version"`
	NodeCount         int                      `json:"node_count"`
	SelectedStepCount int                      `json:"selected_step_count"`
	Compatible        bool                     `json:"compatible"`
	Blockers          []SampleDataBlocker      `json:"blockers"`
	RoleCoverage      []SampleDataRoleCoverage `json:"role_coverage"`
	MissingRoles      []string                 `json:"missing_roles"`
}

type SampleDataAccountingBookOption struct {
	ID           string              `json:"id"`
	Code         string              `json:"code"`
	Name         string              `json:"name"`
	BaseCurrency string              `json:"base_currency"`
	Compatible   bool                `json:"compatible"`
	Blockers     []SampleDataBlocker `json:"blockers"`
}

type SampleDataOptions struct {
	Ready             bool                             `json:"ready"`
	BlueprintVersion  int                              `json:"blueprint_version"`
	ProgressPresets   []int                            `json:"progress_presets"`
	Pipelines         []SampleDataPipelineOption       `json:"pipelines"`
	AccountingBooks   []SampleDataAccountingBookOption `json:"accounting_books"`
	EnvironmentPolicy string                           `json:"environment_policy"`
}

type sampleDataStaffActor struct {
	StaffID    string
	UserID     string
	FullName   string
	RoleID     string
	RoleName   string
	RoleKey    string
	Department string
}

type sampleDataStepDefinition struct {
	Record
	Fields []Record
}

type sampleDataNodeDefinition struct {
	Record
	Steps []sampleDataStepDefinition
}

type sampleDataBlueprint struct {
	Template Record
	Nodes    []sampleDataNodeDefinition
	Edges    []Record
	Actors   map[string][]sampleDataStaffActor
}

type sampleDataAccountingContext struct {
	ID               string
	Code             string
	Name             string
	BaseCurrency     string
	BankAccountID    string
	CostAccountID    string
	RevenueAccountID string
	HasOpenPeriod    bool
}

type sampleDataArtifact struct {
	ID       string
	Provider string
	Key      string
	Name     string
	MIMEType string
	Size     int64
	Checksum string
}

type sampleDataPlannedStep struct {
	Definition     sampleDataStepDefinition
	RunNodeID      string
	RunStepID      string
	WorkItemID     string
	FormInstanceID string
	Status         string
	WorkStatus     string
	Selected       bool
	Completed      bool
	Actor          sampleDataStaffActor
	Evidence       []sampleDataArtifact
	FormPDF        *sampleDataArtifact
	Answers        map[string]any
}

type sampleDataPlannedNode struct {
	Definition sampleDataNodeDefinition
	RunNodeID  string
	Status     string
	Selected   bool
	Steps      []sampleDataPlannedStep
}

type sampleDataPlan struct {
	GenerationID      string
	ProjectID         string
	RunID             string
	Profile           string
	ProgressPercent   int
	ProjectCode       string
	ProjectName       string
	ProjectStatus     string
	CurrentNodeID     string
	CompletedAt       string
	TemplateID        string
	TemplateHash      string
	TemplateRevision  int
	SelectedStepCount int
	AccountingBookID  string
	BankAccountID     string
	CostAccountID     string
	RevenueAccountID  string
	Nodes             []sampleDataPlannedNode
	Edges             []Record
	Actors            map[string][]sampleDataStaffActor
	AllActors         []sampleDataStaffActor
	ActorID           string
	ActorName         string
	IdempotencyKey    string
}
