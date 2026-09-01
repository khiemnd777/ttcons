package core

const (
	projectExcelWorkbookVersion    = "1"
	projectExcelMaximumRows        = 100
	projectExcelMaximumFileBytes   = int64(5 * 1024 * 1024)
	projectExcelMaximumExportRows  = 10000
	projectExcelMaximumSelectedIDs = 100
	projectExcelMaximumVND         = int64(9_000_000_000_000_000)
)

type ProjectExcelImportRowRequest struct {
	RowNumber              int    `json:"row_number"`
	ProjectCode            string `json:"project_code"`
	ProjectName            string `json:"project_name"`
	ClientPartnerCode      string `json:"client_partner_code"`
	ClientNameUnregistered string `json:"client_name_unregistered"`
	Address                string `json:"address"`
	PlannedStartDate       string `json:"planned_start_date"`
	PlannedEndDate         string `json:"planned_end_date"`
	EstimatedBudgetVND     string `json:"estimated_budget_vnd"`
	PipelineRef            string `json:"pipeline_ref"`
}

type ProjectExcelImportPreviewRequest struct {
	WorkbookVersion string                         `json:"workbook_version"`
	PackageVersion  string                         `json:"package_version"`
	SourceFormat    string                         `json:"source_format"`
	SourceFilename  string                         `json:"source_filename"`
	SourceSizeBytes int64                          `json:"source_size_bytes"`
	Rows            []ProjectExcelImportRowRequest `json:"rows"`
}

type ProjectExcelImportExecuteRequest struct {
	ProjectExcelImportPreviewRequest
	PreviewHash    string `json:"preview_hash"`
	IdempotencyKey string `json:"idempotency_key"`
}

type ProjectExcelExportRequest struct {
	Format     string
	Scope      string
	ProjectIDs []string
	Search     string
	Status     string
	Sort       string
	Direction  string
}

type normalizedProjectExcelImportRow struct {
	RowNumber              int    `json:"row_number"`
	ProjectCode            string `json:"project_code"`
	ProjectName            string `json:"project_name"`
	PartnerID              string `json:"partner_id"`
	PartnerCode            string `json:"partner_code"`
	ClientName             string `json:"client_name"`
	Address                string `json:"address"`
	PlannedStartDate       string `json:"planned_start_date"`
	PlannedEndDate         string `json:"planned_end_date"`
	EstimatedBudgetVND     int64  `json:"estimated_budget_vnd"`
	PipelineID             string `json:"pipeline_id"`
	PipelineRef            string `json:"pipeline_ref"`
	PipelineName           string `json:"pipeline_name"`
	PipelineRevision       int    `json:"pipeline_revision"`
	PipelineDefinitionHash string `json:"pipeline_definition_hash"`
}

type projectExcelCatalog struct {
	Pipelines            []Record
	Forms                []Record
	Fields               []Record
	Partners             []Record
	Readiness            Record
	ReferenceFingerprint string
	PipelineByReference  map[string]Record
	PartnerByCode        map[string]Record
}

type projectExcelImportValidation struct {
	Preview      Record
	Rows         []normalizedProjectExcelImportRow
	Hash         string
	Catalog      projectExcelCatalog
	SourceFormat string
}
