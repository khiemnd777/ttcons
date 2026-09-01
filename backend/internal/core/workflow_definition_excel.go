package core

import (
	"regexp"
)

const (
	workflowDefinitionWorkbookVersion  = "1"
	workflowDefinitionMaximumFileBytes = int64(10 * 1024 * 1024)
	workflowDefinitionMaximumPipelines = 25
	workflowDefinitionMaximumForms     = 100
	workflowDefinitionMaximumSheetRows = 10000
)

var workflowDefinitionPackageKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:@-]{0,119}$`)

var workflowDefinitionSurfaceKinds = map[string]bool{
	"workflow_form": true, "evidence": true, "approval": true, "incoming_acceptance": true,
	"material_snapshot": true, "surplus_handling": true, "outgoing_acceptance": true,
	"owner_client_reconciliation": true, "subcontractor_reconciliation": true, "contract": true,
	"quality": true, "finance": true, "handover": true, "generic": true,
}

type WorkflowDefinitionImportPreviewRequest struct {
	Scope           string   `json:"scope"`
	WorkbookVersion string   `json:"workbook_version"`
	PackageVersion  string   `json:"package_version"`
	SourceFormat    string   `json:"source_format"`
	SourceFilename  string   `json:"source_filename"`
	SourceSizeBytes int64    `json:"source_size_bytes"`
	Pipelines       []Record `json:"pipelines"`
	PipelineNodes   []Record `json:"pipeline_nodes"`
	PipelineSteps   []Record `json:"pipeline_steps"`
	PipelineEdges   []Record `json:"pipeline_edges"`
	FormTemplates   []Record `json:"form_templates"`
	FormVersions    []Record `json:"form_versions"`
	FormFields      []Record `json:"form_fields"`
}

type WorkflowDefinitionImportExecuteRequest struct {
	WorkflowDefinitionImportPreviewRequest
	PreviewHash    string `json:"preview_hash"`
	IdempotencyKey string `json:"idempotency_key"`
}

type WorkflowDefinitionExportRequest struct {
	Format              string
	Scope               string
	IDs                 []string
	IncludeDependencies bool
}

type workflowDefinitionPackage struct {
	Scope         string   `json:"scope"`
	Pipelines     []Record `json:"pipelines"`
	PipelineNodes []Record `json:"pipeline_nodes"`
	PipelineSteps []Record `json:"pipeline_steps"`
	PipelineEdges []Record `json:"pipeline_edges"`
	Forms         []Record `json:"forms"`
}

type workflowDefinitionCatalog struct {
	Permissions           []Record
	Roles                 []Record
	Forms                 []Record
	PipelineFamilies      []Record
	PermissionByKey       map[string]Record
	RoleByName            map[string]Record
	RoleNameByDisplayName map[string]string
	FormByCode            map[string]Record
	PublishedFormByRef    map[string]Record
	PipelineFamilyByID    map[string]Record
	Fingerprint           string
}

type workflowDefinitionValidation struct {
	Package      workflowDefinitionPackage
	Preview      Record
	Hash         string
	Catalog      workflowDefinitionCatalog
	SourceFormat string
}
