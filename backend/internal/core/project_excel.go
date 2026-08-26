package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

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

func (s *Service) ProjectExcelReadiness(ctx context.Context) (Record, error) {
	catalog, err := s.repo.ProjectExcelCatalog(ctx)
	if err != nil {
		return nil, err
	}
	return catalog.Readiness, nil
}

func (s *Service) ProjectExcelTemplate(ctx context.Context) (tabularExportResult, error) {
	return s.ProjectImportTemplate(ctx, dataPackageFormatXLSX)
}

func (s *Service) ProjectImportTemplate(ctx context.Context, format string) (tabularExportResult, error) {
	format, err := normalizeDataPackageFormat(format)
	if err != nil {
		return tabularExportResult{}, err
	}
	catalog, err := s.repo.ProjectExcelCatalog(ctx)
	if err != nil {
		return tabularExportResult{}, err
	}
	if !boolValue(catalog.Readiness["ready"]) {
		return tabularExportResult{}, projectExcelReadinessError(catalog.Readiness)
	}
	if format == dataPackageFormatJSON {
		return generateProjectJSONPackage(catalog, nil, time.Now(), false)
	}
	return generateProjectExcelWorkbook(catalog, nil, time.Now(), false)
}

func (s *Service) PreviewProjectExcelImport(ctx context.Context, req ProjectExcelImportPreviewRequest) (Record, error) {
	validation, err := s.validateProjectExcelImport(ctx, req)
	if err != nil {
		return nil, err
	}
	return validation.Preview, nil
}

func (s *Service) ExecuteProjectExcelImport(ctx context.Context, actorID, actorName string, req ProjectExcelImportExecuteRequest) (Record, error) {
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if _, err := uuid.Parse(idempotencyKey); err != nil {
		return nil, validationError("Mã chống tạo trùng không hợp lệ", Record{"field": "idempotency_key"})
	}
	previewHash := strings.TrimSpace(req.PreviewHash)
	if stored, found, err := s.repo.ProjectExcelImportResult(ctx, actorID, idempotencyKey, previewHash); err != nil {
		return nil, err
	} else if found {
		stored["idempotent_replay"] = true
		return stored, nil
	}
	validation, err := s.validateProjectExcelImport(ctx, req.ProjectExcelImportPreviewRequest)
	if err != nil {
		return nil, err
	}
	if number(validation.Preview["invalid_count"]) > 0 {
		return nil, validationError("Danh sách import còn dòng không hợp lệ", validation.Preview)
	}
	if previewHash == "" || previewHash != validation.Hash {
		return nil, conflictError("Dữ liệu hoặc danh mục tham chiếu đã thay đổi sau bước xem trước. Vui lòng xem trước lại", Record{"field": "preview_hash"})
	}
	result, err := s.repo.ExecuteProjectExcelImport(ctx, actorID, actorName, idempotencyKey, validation.SourceFormat, req.SourceFilename, req.SourceSizeBytes, validation.Hash, validation.Catalog.ReferenceFingerprint, validation.Rows)
	if err != nil {
		return nil, err
	}
	s.publishProjectProgressChanged("")
	s.publishWorkQueueChanged("")
	return result, nil
}

func (s *Service) ExportProjectsExcel(ctx context.Context, actorID, actorName string, req ProjectExcelExportRequest) (tabularExportResult, error) {
	normalized, err := normalizeProjectExcelExportRequest(req)
	if err != nil {
		return tabularExportResult{}, err
	}
	projects, err := s.repo.ProjectExcelExportRows(ctx, actorID, normalized)
	if err != nil {
		return tabularExportResult{}, err
	}
	if normalized.Scope == "selected" && len(projects) != len(normalized.ProjectIDs) {
		return tabularExportResult{}, forbiddenError("Một số công trình không tồn tại, đã bị huỷ hoặc nằm ngoài phạm vi được phép", Record{"requested_count": len(normalized.ProjectIDs), "available_count": len(projects)})
	}
	if len(projects) > projectExcelMaximumExportRows {
		return tabularExportResult{}, validationError("Kết quả xuất vượt quá 10.000 công trình. Vui lòng thu hẹp bộ lọc", Record{"maximum": projectExcelMaximumExportRows})
	}
	catalog, err := s.repo.ProjectExcelCatalog(ctx)
	if err != nil {
		return tabularExportResult{}, err
	}
	var result tabularExportResult
	if normalized.Format == dataPackageFormatJSON {
		result, err = generateProjectJSONPackage(catalog, projects, time.Now(), true)
	} else {
		result, err = generateProjectExcelWorkbook(catalog, projects, time.Now(), true)
	}
	if err != nil {
		return tabularExportResult{}, err
	}
	s.repo.AddAuditBestEffort(ctx, "", actorID, actorName, "projects_data_exported", "project_export", "", "Xuất danh sách công trình", Record{
		"source_format": normalized.Format,
		"scope":         normalized.Scope, "row_count": len(projects), "search": normalized.Search, "status": normalized.Status,
		"sort": normalized.Sort, "direction": normalized.Direction,
	})
	return result, nil
}

func (s *Service) validateProjectExcelImport(ctx context.Context, req ProjectExcelImportPreviewRequest) (projectExcelImportValidation, error) {
	filename := strings.TrimSpace(req.SourceFilename)
	sourceFormat, packageVersion, err := normalizeDataPackageImportSource(req.SourceFormat, req.PackageVersion, req.WorkbookVersion, filename)
	if err != nil {
		return projectExcelImportValidation{}, err
	}
	if req.SourceSizeBytes <= 0 || req.SourceSizeBytes > projectExcelMaximumFileBytes {
		return projectExcelImportValidation{}, validationError("Tệp import phải có dung lượng không quá 5 MB", Record{"field": "source_size_bytes", "maximum": projectExcelMaximumFileBytes})
	}
	if len(req.Rows) == 0 || len(req.Rows) > projectExcelMaximumRows {
		return projectExcelImportValidation{}, validationError("Mỗi lần import phải có từ 1 đến 100 công trình", Record{"field": "rows", "maximum": projectExcelMaximumRows})
	}
	catalog, err := s.repo.ProjectExcelCatalog(ctx)
	if err != nil {
		return projectExcelImportValidation{}, err
	}
	if !boolValue(catalog.Readiness["ready"]) {
		return projectExcelImportValidation{}, projectExcelReadinessError(catalog.Readiness)
	}

	rows := make([]normalizedProjectExcelImportRow, len(req.Rows))
	issues := make([][]Record, len(req.Rows))
	warnings := make([][]Record, len(req.Rows))
	codeRows := map[string][]int{}
	rowNumbers := map[int]bool{}
	for index, raw := range req.Rows {
		issues[index] = []Record{}
		warnings[index] = []Record{}
		rowNumber := raw.RowNumber
		if rowNumber < 2 || rowNumbers[rowNumber] {
			rowNumber = index + 2
		}
		rowNumbers[rowNumber] = true
		row := normalizedProjectExcelImportRow{RowNumber: rowNumber}
		row.ProjectCode = normalizeProjectExcelText(raw.ProjectCode, "project_code", "Mã công trình", 120, true, &issues[index])
		row.ProjectName = normalizeProjectExcelText(raw.ProjectName, "project_name", "Tên công trình", 240, true, &issues[index])
		row.Address = normalizeProjectExcelText(raw.Address, "address", "Địa chỉ", 1000, false, &issues[index])
		row.PlannedStartDate = normalizeProjectExcelDate(raw.PlannedStartDate, "planned_start_date", &issues[index])
		row.PlannedEndDate = normalizeProjectExcelDate(raw.PlannedEndDate, "planned_end_date", &issues[index])
		if row.PlannedStartDate != "" && row.PlannedEndDate != "" && row.PlannedEndDate < row.PlannedStartDate {
			issues[index] = append(issues[index], Record{"field": "planned_end_date", "message": "Ngày kết thúc dự kiến phải từ ngày bắt đầu trở đi"})
		}
		row.EstimatedBudgetVND = normalizeProjectExcelBudget(raw.EstimatedBudgetVND, &issues[index])

		partnerCode := strings.TrimSpace(raw.ClientPartnerCode)
		unregisteredName := normalizeProjectExcelText(raw.ClientNameUnregistered, "client_name_unregistered", "Tên chủ đầu tư chưa có trong danh mục", 240, false, &issues[index])
		if partnerCode != "" {
			partner, ok := catalog.PartnerByCode[strings.ToLower(partnerCode)]
			if !ok {
				issues[index] = append(issues[index], Record{"field": "client_partner_code", "message": "Mã chủ đầu tư không tồn tại, không active hoặc không thuộc loại client"})
			} else {
				row.PartnerID = cleanID(partner["id"])
				row.PartnerCode = strings.TrimSpace(fmt.Sprint(partner["partner_code"]))
				row.ClientName = strings.TrimSpace(fmt.Sprint(partner["name"]))
				if unregisteredName != "" {
					warnings[index] = append(warnings[index], Record{"field": "client_name_unregistered", "message": "Tên nhập tay được bỏ qua vì đã chọn chủ đầu tư trong danh mục"})
				}
			}
		} else if unregisteredName == "" {
			issues[index] = append(issues[index], Record{"field": "client_name_unregistered", "message": "Cần nhập tên chủ đầu tư khi không có client_partner_code"})
		} else {
			row.ClientName = unregisteredName
			warnings[index] = append(warnings[index], Record{"field": "client_name_unregistered", "message": "Chủ đầu tư chưa liên kết với danh mục Đầu vào"})
		}

		row.PipelineRef = strings.TrimSpace(raw.PipelineRef)
		if row.PipelineRef == "" {
			issues[index] = append(issues[index], Record{"field": "pipeline_ref", "message": "Pipeline reference là bắt buộc"})
		} else if pipeline, ok := catalog.PipelineByReference[row.PipelineRef]; !ok {
			issues[index] = append(issues[index], Record{"field": "pipeline_ref", "message": "Pipeline reference không tồn tại hoặc chưa sẵn sàng"})
		} else {
			row.PipelineID = cleanID(pipeline["template_id"])
			row.PipelineName = strings.TrimSpace(fmt.Sprint(pipeline["name"]))
			row.PipelineRevision = int(number(pipeline["revision_no"]))
			row.PipelineDefinitionHash = strings.TrimSpace(fmt.Sprint(pipeline["definition_hash"]))
		}
		if row.ProjectCode != "" {
			codeRows[strings.ToLower(row.ProjectCode)] = append(codeRows[strings.ToLower(row.ProjectCode)], index)
		}
		rows[index] = row
	}
	for code, indexes := range codeRows {
		if len(indexes) > 1 {
			for _, index := range indexes {
				issues[index] = append(issues[index], Record{"field": "project_code", "message": "Mã công trình bị trùng trong tệp", "value": code})
			}
		}
	}
	existingCodes, err := s.repo.ExistingProjectExcelCodes(ctx, sortedProjectExcelKeys(codeRows))
	if err != nil {
		return projectExcelImportValidation{}, err
	}
	for code := range existingCodes {
		for _, index := range codeRows[code] {
			issues[index] = append(issues[index], Record{"field": "project_code", "message": "Mã công trình đã tồn tại trong hệ thống, kể cả công trình đã huỷ"})
		}
	}

	previewRows := make([]Record, 0, len(rows))
	validCount := 0
	warningCount := 0
	for index, row := range rows {
		valid := len(issues[index]) == 0
		if valid {
			validCount++
		}
		warningCount += len(warnings[index])
		previewRows = append(previewRows, Record{
			"row_number": row.RowNumber, "project_code": row.ProjectCode, "project_name": row.ProjectName,
			"client_name": row.ClientName, "client_partner_code": row.PartnerCode, "pipeline_ref": row.PipelineRef,
			"pipeline_name": row.PipelineName, "pipeline_revision": row.PipelineRevision,
			"estimated_budget_vnd": row.EstimatedBudgetVND, "valid": valid, "errors": issues[index], "warnings": warnings[index],
		})
	}
	hash := ""
	if validCount == len(rows) {
		payload, err := json.Marshal(Record{
			"package_version": packageVersion, "source_format": sourceFormat, "source_filename": filename, "source_size_bytes": req.SourceSizeBytes,
			"reference_fingerprint": catalog.ReferenceFingerprint, "rows": rows,
		})
		if err != nil {
			return projectExcelImportValidation{}, err
		}
		mac := hmac.New(sha256.New, []byte(s.jwtSecret))
		if _, err := mac.Write(payload); err != nil {
			return projectExcelImportValidation{}, err
		}
		hash = hex.EncodeToString(mac.Sum(nil))
	}
	preview := Record{
		"workbook_version": projectExcelWorkbookVersion, "package_version": packageVersion, "source_format": sourceFormat, "source_filename": filename,
		"total_count": len(rows), "valid_count": validCount, "invalid_count": len(rows) - validCount,
		"warning_count": warningCount, "preview_hash": hash, "reference_fingerprint": catalog.ReferenceFingerprint,
		"readiness": catalog.Readiness, "rows": previewRows,
	}
	return projectExcelImportValidation{Preview: preview, Rows: rows, Hash: hash, Catalog: catalog, SourceFormat: sourceFormat}, nil
}

func normalizeProjectExcelText(value, field, label string, maximum int, required bool, issues *[]Record) string {
	value = strings.TrimSpace(value)
	if required && value == "" {
		*issues = append(*issues, Record{"field": field, "message": label + " là bắt buộc"})
		return ""
	}
	if len([]rune(value)) > maximum {
		*issues = append(*issues, Record{"field": field, "message": fmt.Sprintf("%s không được vượt quá %d ký tự", label, maximum)})
		return ""
	}
	return value
}

func normalizeProjectExcelDate(value, field string, issues *[]Record) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		*issues = append(*issues, Record{"field": field, "message": "Ngày phải có định dạng YYYY-MM-DD"})
		return ""
	}
	return value
}

func normalizeProjectExcelBudget(value string, issues *[]Record) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			*issues = append(*issues, Record{"field": "estimated_budget_vnd", "message": "Ngân sách VND phải là số nguyên không âm, không chứa dấu phân cách"})
			return 0
		}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 || parsed > projectExcelMaximumVND {
		*issues = append(*issues, Record{"field": "estimated_budget_vnd", "message": "Ngân sách VND vượt phạm vi cho phép"})
		return 0
	}
	return parsed
}

func normalizeProjectExcelExportRequest(req ProjectExcelExportRequest) (ProjectExcelExportRequest, error) {
	format, err := normalizeDataPackageFormat(req.Format)
	if err != nil {
		return req, err
	}
	req.Format = format
	req.Scope = strings.ToLower(strings.TrimSpace(req.Scope))
	if req.Scope == "" {
		req.Scope = "filtered"
	}
	if req.Scope != "filtered" && req.Scope != "selected" {
		return req, validationError("Phạm vi xuất không hợp lệ", Record{"scope": req.Scope})
	}
	explorer, err := normalizeProjectExplorerRequest(ProjectExplorerRequest{
		Search: req.Search, Status: req.Status, Sort: req.Sort, Direction: req.Direction, Page: 1, PageSize: 25,
	})
	if err != nil {
		return req, err
	}
	req.Search, req.Status, req.Sort, req.Direction = explorer.Search, explorer.Status, explorer.Sort, explorer.Direction
	seen := map[string]bool{}
	normalizedIDs := []string{}
	for _, projectID := range req.ProjectIDs {
		projectID = strings.TrimSpace(projectID)
		if _, err := uuid.Parse(projectID); err != nil {
			return req, validationError("Danh sách công trình xuất có ID không hợp lệ", Record{"project_id": projectID})
		}
		if !seen[projectID] {
			seen[projectID] = true
			normalizedIDs = append(normalizedIDs, projectID)
		}
	}
	req.ProjectIDs = normalizedIDs
	if req.Scope == "selected" && (len(req.ProjectIDs) == 0 || len(req.ProjectIDs) > projectExcelMaximumSelectedIDs) {
		return req, validationError("Phạm vi đã chọn phải có từ 1 đến 100 công trình", Record{"maximum": projectExcelMaximumSelectedIDs})
	}
	return req, nil
}

func projectExcelReadinessError(readiness Record) StatusError {
	code := "PIPELINE_FORM_REFERENCE_INVALID"
	message := "Dữ liệu Pipeline hoặc Biểu mẫu quy trình chưa sẵn sàng. Vui lòng cập nhật trước"
	if number(readiness["ready_pipeline_count"]) == 0 {
		code = "PIPELINE_DATA_MISSING"
		message = "Hệ thống chưa có Pipeline đã phát hành và sẵn sàng. Vui lòng cập nhật Trình thiết kế quy trình trước"
	} else if number(readiness["published_form_version_count"]) == 0 || number(readiness["field_count"]) == 0 {
		code = "WORKFLOW_FORM_DATA_MISSING"
		message = "Hệ thống chưa có Biểu mẫu quy trình và Fields hợp lệ. Vui lòng cập nhật trước"
	}
	return StatusError{Status: fiber.StatusConflict, Code: code, Message: message, Details: Record{"readiness": readiness}}
}

func sortedProjectExcelKeys(values map[string][]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (r *Repository) ProjectExcelCatalog(ctx context.Context) (projectExcelCatalog, error) {
	pipelines, err := r.Query(ctx, `
		WITH field_counts AS (
			SELECT form_version_id,COUNT(*)::int AS field_count
			FROM workflow_form_fields
			GROUP BY form_version_id
		)
		SELECT
			pt.id::text AS template_id,
			COALESCE(pt.template_family_id::text,'') AS template_family_id,
			pt.name,
			COALESCE(pt.project_type,'') AS project_type,
			pt.revision_no,
			pt.engine_version,
			COALESCE(pt.definition_hash,'') AS definition_hash,
			COUNT(DISTINCT node.id)::int AS node_count,
			COUNT(DISTINCT step.id)::int AS step_count,
			COUNT(DISTINCT step.id) FILTER (WHERE step.form_required)::int AS required_form_count,
			COUNT(DISTINCT step.id) FILTER (
				WHERE step.form_required AND (
					version.id IS NULL OR version.status <> 'published' OR template.status <> 'active'
				)
			)::int AS invalid_form_reference_count,
			COUNT(DISTINCT step.id) FILTER (
				WHERE step.form_required AND COALESCE(field_counts.field_count,0)=0
			)::int AS zero_field_reference_count
		FROM pipeline_templates pt
		LEFT JOIN pipeline_template_nodes node ON node.template_id=pt.id
		LEFT JOIN pipeline_template_steps step ON step.node_id=node.id
		LEFT JOIN workflow_form_versions version ON version.id=step.form_version_id
		LEFT JOIN workflow_form_templates template ON template.id=version.template_id
		LEFT JOIN field_counts ON field_counts.form_version_id=version.id
		WHERE pt.status='published'
		GROUP BY pt.id
		ORDER BY pt.name,pt.revision_no,pt.id
	`)
	if err != nil {
		return projectExcelCatalog{}, err
	}
	forms, err := r.Query(ctx, `
		SELECT
			template.id::text AS form_template_id,
			template.code AS form_template_code,
			template.name AS form_template_name,
			template.form_type,
			version.id::text AS form_version_id,
			version.version_no,
			version.status,
			COUNT(field.id)::int AS field_count
		FROM workflow_form_templates template
		JOIN workflow_form_versions version ON version.template_id=template.id AND version.status='published'
		LEFT JOIN workflow_form_fields field ON field.form_version_id=version.id
		WHERE template.status='active'
		GROUP BY template.id,version.id
		ORDER BY template.code,version.version_no,version.id
	`)
	if err != nil {
		return projectExcelCatalog{}, err
	}
	fields, err := r.Query(ctx, `
		SELECT
			template.code AS form_template_code,
			version.id::text AS form_version_id,
			version.version_no,
			field.field_key,
			field.label,
			field.field_type,
			field.required,
			field.options_json,
			field.validation_json,
			field.visibility_condition_json,
			field.order_index
		FROM workflow_form_templates template
		JOIN workflow_form_versions version ON version.template_id=template.id AND version.status='published'
		JOIN workflow_form_fields field ON field.form_version_id=version.id
		WHERE template.status='active'
		ORDER BY template.code,version.version_no,field.order_index,field.id
	`)
	if err != nil {
		return projectExcelCatalog{}, err
	}
	partners, err := r.Query(ctx, `
		SELECT id::text,partner_code,name,COALESCE(tax_code,'') AS tax_code,updated_at
		FROM partners
		WHERE active=true AND partner_types ? 'client'
		ORDER BY partner_code,id
	`)
	if err != nil {
		return projectExcelCatalog{}, err
	}

	pipelineByReference := map[string]Record{}
	readyPipelineCount := 0
	invalidPipelineCount := 0
	for _, pipeline := range pipelines {
		ready := number(pipeline["node_count"]) > 0 &&
			number(pipeline["step_count"]) > 0 &&
			number(pipeline["invalid_form_reference_count"]) == 0 &&
			number(pipeline["zero_field_reference_count"]) == 0 &&
			strings.TrimSpace(fmt.Sprint(pipeline["definition_hash"])) != ""
		pipeline["ready"] = ready
		pipeline["pipeline_ref"] = projectExcelPipelineReference(pipeline)
		if ready {
			readyPipelineCount++
			pipelineByReference[fmt.Sprint(pipeline["pipeline_ref"])] = pipeline
		} else {
			invalidPipelineCount++
		}
	}
	formTemplateIDs := map[string]bool{}
	validFormVersionCount := 0
	for _, form := range forms {
		form["form_ref"] = projectExcelFormReference(form)
		formTemplateIDs[cleanID(form["form_template_id"])] = true
		if number(form["field_count"]) > 0 {
			validFormVersionCount++
		}
	}
	for _, field := range fields {
		field["form_ref"] = projectExcelFormReference(field)
	}
	partnerByCode := map[string]Record{}
	for _, partner := range partners {
		partnerByCode[strings.ToLower(strings.TrimSpace(fmt.Sprint(partner["partner_code"])))] = partner
	}
	blockers := []Record{}
	warnings := []Record{}
	if readyPipelineCount == 0 {
		blockers = append(blockers, Record{"code": "PIPELINE_DATA_MISSING", "message": "Chưa có Pipeline published và sẵn sàng", "route": "/pipeline-builder"})
	}
	if len(forms) == 0 || validFormVersionCount == 0 || len(fields) == 0 {
		blockers = append(blockers, Record{"code": "WORKFLOW_FORM_DATA_MISSING", "message": "Chưa có Form version published kèm Fields", "route": "/workflow-forms"})
	}
	if invalidPipelineCount > 0 {
		warnings = append(warnings, Record{"code": "PIPELINE_FORM_REFERENCE_INVALID", "message": "Có Pipeline published không đủ điều kiện và đã bị loại khỏi danh mục import", "count": invalidPipelineCount, "route": "/pipeline-builder"})
	}
	readiness := Record{
		"ready":                        len(blockers) == 0,
		"published_pipeline_count":     len(pipelines),
		"ready_pipeline_count":         readyPipelineCount,
		"invalid_pipeline_count":       invalidPipelineCount,
		"active_form_template_count":   len(formTemplateIDs),
		"published_form_version_count": len(forms),
		"valid_form_version_count":     validFormVersionCount,
		"field_count":                  len(fields),
		"active_client_partner_count":  len(partners),
		"blockers":                     blockers,
		"warnings":                     warnings,
		"pipeline_builder_route":       "/pipeline-builder",
		"workflow_forms_route":         "/workflow-forms",
	}
	fingerprintPayload, err := json.Marshal(Record{"pipelines": pipelines, "forms": forms, "fields": fields, "partners": partners})
	if err != nil {
		return projectExcelCatalog{}, err
	}
	fingerprint := sha256.Sum256(fingerprintPayload)
	readiness["reference_fingerprint"] = hex.EncodeToString(fingerprint[:])
	return projectExcelCatalog{
		Pipelines: pipelines, Forms: forms, Fields: fields, Partners: partners,
		Readiness: readiness, ReferenceFingerprint: fmt.Sprint(readiness["reference_fingerprint"]),
		PipelineByReference: pipelineByReference, PartnerByCode: partnerByCode,
	}, nil
}

func (r *Repository) ExistingProjectExcelCodes(ctx context.Context, normalizedCodes []string) (map[string]bool, error) {
	result := map[string]bool{}
	if len(normalizedCodes) == 0 {
		return result, nil
	}
	placeholders := make([]string, len(normalizedCodes))
	args := make([]any, len(normalizedCodes))
	for index, code := range normalizedCodes {
		placeholders[index] = fmt.Sprintf("$%d", index+1)
		args[index] = code
	}
	rows, err := r.db.QueryContext(ctx, `SELECT lower(code) FROM projects WHERE lower(code) IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		result[code] = true
	}
	return result, rows.Err()
}

func (r *Repository) ProjectExcelImportResult(ctx context.Context, actorID, idempotencyKey, previewHash string) (Record, bool, error) {
	var storedActorID sql.NullString
	var storedHash string
	var sourceFormat string
	var storedJSON []byte
	err := r.db.QueryRowContext(ctx, `SELECT actor_id::text,preview_hash,source_format,result_json FROM project_excel_imports WHERE idempotency_key=$1`, idempotencyKey).Scan(&storedActorID, &storedHash, &sourceFormat, &storedJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !storedActorID.Valid || storedActorID.String != actorID || previewHash == "" || storedHash != previewHash {
		return nil, false, conflictError("Mã chống tạo trùng đã được dùng cho một danh sách import khác", Record{"field": "idempotency_key"})
	}
	var result Record
	if err := json.Unmarshal(storedJSON, &result); err != nil {
		return nil, false, err
	}
	result["source_format"] = sourceFormat
	return result, true, nil
}

func (r *Repository) ExecuteProjectExcelImport(ctx context.Context, actorID, actorName, idempotencyKey, sourceFormat, sourceFilename string, sourceSizeBytes int64, previewHash, referenceFingerprint string, rows []normalizedProjectExcelImportRow) (result Record, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, idempotencyKey); err != nil {
		return nil, err
	}
	var storedHash string
	var storedActorID sql.NullString
	var storedJSON []byte
	if err := tx.QueryRowContext(ctx, `SELECT actor_id::text,preview_hash,result_json FROM project_excel_imports WHERE idempotency_key=$1`, idempotencyKey).Scan(&storedActorID, &storedHash, &storedJSON); err == nil {
		if !storedActorID.Valid || storedActorID.String != actorID || storedHash != previewHash {
			return nil, conflictError("Mã chống tạo trùng đã được dùng cho một danh sách import khác", Record{"field": "idempotency_key"})
		}
		if err := json.Unmarshal(storedJSON, &result); err != nil {
			return nil, err
		}
		result["idempotent_replay"] = true
		return result, tx.Commit()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	importID := uuid.NewString()
	created := make([]Record, 0, len(rows))
	for _, row := range rows {
		var revision int
		var definitionHash string
		var pipelineReady bool
		if err := tx.QueryRowContext(ctx, `
			SELECT template.revision_no,COALESCE(template.definition_hash,''),
				EXISTS (SELECT 1 FROM pipeline_template_nodes node WHERE node.template_id=template.id)
				AND EXISTS (
					SELECT 1 FROM pipeline_template_steps step
					JOIN pipeline_template_nodes node ON node.id=step.node_id
					WHERE node.template_id=template.id
				)
				AND NOT EXISTS (
					SELECT 1
					FROM pipeline_template_steps step
					JOIN pipeline_template_nodes node ON node.id=step.node_id
					LEFT JOIN workflow_form_versions version ON version.id=step.form_version_id
					LEFT JOIN workflow_form_templates form_template ON form_template.id=version.template_id
					WHERE node.template_id=template.id AND step.form_required AND (
						version.id IS NULL OR version.status <> 'published' OR form_template.status <> 'active'
						OR NOT EXISTS (SELECT 1 FROM workflow_form_fields field WHERE field.form_version_id=version.id)
					)
				)
			FROM pipeline_templates template
			WHERE template.id=$1 AND template.status='published'
			FOR SHARE
		`, row.PipelineID).Scan(&revision, &definitionHash, &pipelineReady); err != nil {
			return nil, conflictError("Pipeline reference đã thay đổi hoặc không còn được phát hành", Record{"row_number": row.RowNumber, "field": "pipeline_ref"})
		}
		if !pipelineReady || projectExcelPipelineReferenceFromValues(row.PipelineID, revision, definitionHash) != row.PipelineRef {
			return nil, conflictError("Pipeline reference đã thay đổi sau bước xem trước", Record{"row_number": row.RowNumber, "field": "pipeline_ref"})
		}
		if row.PartnerID != "" {
			var partnerCode, partnerName string
			if err := tx.QueryRowContext(ctx, `SELECT partner_code,name FROM partners WHERE id=$1 AND active=true AND partner_types ? 'client' FOR SHARE`, row.PartnerID).Scan(&partnerCode, &partnerName); err != nil || !strings.EqualFold(partnerCode, row.PartnerCode) || partnerName != row.ClientName {
				return nil, conflictError("Chủ đầu tư đã thay đổi sau bước xem trước", Record{"row_number": row.RowNumber, "field": "client_partner_code"})
			}
		}
		var projectID string
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO projects (
				name,code,partner_id,client_name,address,planned_start_date,planned_end_date,
				estimated_budget,contract_amount,status,pipeline_template_id,progress_percent,created_by
			)
			VALUES ($1,$2,NULLIF($3,'')::uuid,$4,NULLIF($5,''),NULLIF($6,'')::date,NULLIF($7,'')::date,$8,0,'draft',$9,0,NULLIF($10,'')::uuid)
			RETURNING id::text
		`, row.ProjectName, row.ProjectCode, row.PartnerID, row.ClientName, row.Address, row.PlannedStartDate, row.PlannedEndDate, row.EstimatedBudgetVND, row.PipelineID, actorID).Scan(&projectID); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return nil, conflictError("Mã công trình đã được sử dụng trong lúc import", Record{"row_number": row.RowNumber, "field": "project_code"})
			}
			return nil, err
		}
		if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "project_imported", "project", projectID, "Tạo công trình từ gói dữ liệu", Record{
			"import_id": importID, "source_filename": strings.TrimSpace(sourceFilename), "row_number": row.RowNumber,
			"project_code": row.ProjectCode, "pipeline_ref": row.PipelineRef, "partner_code": row.PartnerCode,
			"pipeline_started": false, "source_format": sourceFormat,
		}); err != nil {
			return nil, err
		}
		created = append(created, Record{"row_number": row.RowNumber, "project_id": projectID, "project_code": row.ProjectCode, "pipeline_ref": row.PipelineRef})
	}
	result = Record{
		"import_id": importID, "source_format": sourceFormat, "source_filename": strings.TrimSpace(sourceFilename), "created_count": len(rows),
		"pipeline_started_count": 0, "items": created, "idempotent_replay": false,
	}
	storedResult := Record{
		"import_id": importID, "source_format": sourceFormat, "source_filename": strings.TrimSpace(sourceFilename), "created_count": len(rows),
		"pipeline_started_count": 0, "items": created,
	}
	resultJSON, err := json.Marshal(storedResult)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_excel_imports (
			id,idempotency_key,actor_id,source_filename,source_size_bytes,workbook_version,source_format,
			preview_hash,reference_fingerprint,row_count,created_count,result_json
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10,$11::jsonb)
	`, importID, idempotencyKey, actorID, strings.TrimSpace(sourceFilename), sourceSizeBytes, projectExcelWorkbookVersion, sourceFormat, previewHash, referenceFingerprint, len(rows), resultJSON); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "project_data_import_completed", "project_excel_import", importID, "Hoàn tất import công trình từ gói dữ liệu", Record{
		"source_filename": strings.TrimSpace(sourceFilename), "source_format": sourceFormat, "row_count": len(rows), "created_count": len(rows), "pipeline_started_count": 0,
	}); err != nil {
		return nil, err
	}
	if _, err := emitNotificationEventTx(ctx, tx, NotificationDomainEvent{
		Key: "project.setup_batch_required", AggregateType: "project_excel_import", AggregateID: importID, ActorUserID: actorID,
		Payload:  Record{"project_count": len(rows), "import_id": importID},
		Audience: Record{"user_ids": []string{actorID}}, CorrelationKey: "project_setup_batch:" + importID,
		IdempotencyKey: "project_setup_batch:" + importID,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *Repository) ProjectExcelExportRows(ctx context.Context, actorID string, req ProjectExcelExportRequest) ([]Record, error) {
	scopeSQL, args, err := r.ProjectAccessFilter(ctx, actorID, "project", 1)
	if err != nil {
		return nil, err
	}
	where := []string{"project.status <> 'cancelled'", scopeSQL}
	if req.Search != "" {
		args = append(args, "%"+escapeLikePattern(req.Search)+"%")
		placeholder := len(args)
		where = append(where, fmt.Sprintf(`(
			LOWER(project.name) LIKE LOWER($%[1]d) ESCAPE E'\\' OR
			LOWER(project.code) LIKE LOWER($%[1]d) ESCAPE E'\\' OR
			LOWER(project.client_name) LIKE LOWER($%[1]d) ESCAPE E'\\' OR
			LOWER(COALESCE(s.full_name,'')) LIKE LOWER($%[1]d) ESCAPE E'\\'
		)`, placeholder))
	}
	if req.Status != "" {
		args = append(args, req.Status)
		where = append(where, fmt.Sprintf("project.status=$%d", len(args)))
	}
	if req.Scope == "selected" {
		placeholders := make([]string, 0, len(req.ProjectIDs))
		for _, projectID := range req.ProjectIDs {
			args = append(args, projectID)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		where = append(where, "project.id::text IN ("+strings.Join(placeholders, ",")+")")
	}
	explorerRequest := ProjectExplorerRequest{Sort: req.Sort, Direction: req.Direction}
	orderSQL := projectExplorerOrderSQLForAlias(explorerRequest, "project", "s")
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT
			project.id::text AS project_id,
			project.code AS project_code,
			project.name AS project_name,
			COALESCE(partner.partner_code,'') AS client_partner_code,
			CASE WHEN partner.id IS NULL THEN project.client_name ELSE '' END AS client_name_unregistered,
			project.client_name,
			COALESCE(project.address,'') AS address,
			project.planned_start_date,
			project.planned_end_date,
			project.estimated_budget AS estimated_budget_vnd,
			project.contract_amount,
			project.status,
			project.progress_percent,
			COALESCE(project.pm_staff_id::text,'') AS pm_staff_id,
			COALESCE(s.full_name,'') AS pm_name,
			project.created_at,
			project.updated_at,
			COALESCE(selected_template.id::text,'') AS selected_template_id,
			COALESCE(selected_template.revision_no,0) AS selected_template_revision,
			COALESCE(selected_template.definition_hash,'') AS selected_template_definition_hash,
			COALESCE(selected_template.name,'') AS selected_template_name,
			COALESCE(run.id::text,'') AS runtime_id,
			COALESCE(run_template.id::text,'') AS runtime_template_id,
			COALESCE(run_template.revision_no,0) AS runtime_template_revision,
			COALESCE(run.template_definition_hash,run_template.definition_hash,'') AS runtime_template_definition_hash,
			COALESCE(run_template.name,'') AS runtime_template_name,
			COALESCE(run.engine_version,0) AS runtime_engine_version
		FROM projects project
		LEFT JOIN staff s ON s.id=project.pm_staff_id
		LEFT JOIN partners partner ON partner.id=project.partner_id
		LEFT JOIN pipeline_templates selected_template ON selected_template.id=project.pipeline_template_id
		LEFT JOIN pipeline_runs run ON run.id=project.current_pipeline_run_id
		LEFT JOIN pipeline_templates run_template ON run_template.id=run.template_id
		WHERE %s
		ORDER BY %s
		LIMIT %d
	`, strings.Join(where, " AND "), orderSQL, projectExcelMaximumExportRows+1), args...)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		selectedRef := projectExcelPipelineReferenceFromValues(cleanID(row["selected_template_id"]), int(number(row["selected_template_revision"])), strings.TrimSpace(fmt.Sprint(row["selected_template_definition_hash"])))
		runtimeRef := projectExcelPipelineReferenceFromValues(cleanID(row["runtime_template_id"]), int(number(row["runtime_template_revision"])), strings.TrimSpace(fmt.Sprint(row["runtime_template_definition_hash"])))
		row["selected_pipeline_ref"] = selectedRef
		row["runtime_pipeline_ref"] = runtimeRef
		row["pipeline_ref"] = selectedRef
		if selectedRef == "" {
			row["pipeline_ref"] = runtimeRef
		}
		switch {
		case selectedRef == "" && runtimeRef == "":
			row["reference_state"] = "missing"
		case selectedRef != "" && runtimeRef != "" && selectedRef != runtimeRef:
			row["reference_state"] = "runtime_mismatch"
		case selectedRef == "" && runtimeRef != "":
			row["reference_state"] = "runtime_only"
		default:
			row["reference_state"] = "ready"
		}
	}
	return rows, nil
}

func projectExplorerOrderSQLForAlias(request ProjectExplorerRequest, projectAlias, staffAlias string) string {
	direction := "ASC"
	if request.Direction == "desc" {
		direction = "DESC"
	}
	tieBreak := fmt.Sprintf("LOWER(%s.code) ASC,LOWER(%s.name) ASC,%s.id ASC", projectAlias, projectAlias, projectAlias)
	switch request.Sort {
	case "project":
		return fmt.Sprintf("LOWER(%s.code) %s,LOWER(%s.name) %s,%s.id ASC", projectAlias, direction, projectAlias, direction, projectAlias)
	case "status":
		return fmt.Sprintf("LOWER(%s) %s,%s", projectStatusLabelSQL(projectAlias+".status"), direction, tieBreak)
	case "owner":
		return fmt.Sprintf("(NULLIF(BTRIM(%s.full_name),'') IS NULL) ASC,LOWER(%s.full_name) %s,%s", staffAlias, staffAlias, direction, tieBreak)
	default:
		return fmt.Sprintf("%s.created_at %s,%s", projectAlias, direction, tieBreak)
	}
}

func projectExcelPipelineReference(row Record) string {
	return projectExcelPipelineReferenceFromValues(cleanID(row["template_id"]), int(number(row["revision_no"])), strings.TrimSpace(fmt.Sprint(row["definition_hash"])))
}

func projectExcelPipelineReferenceFromValues(templateID string, revision int, definitionHash string) string {
	if templateID == "" || revision < 1 || definitionHash == "" {
		return ""
	}
	return fmt.Sprintf("%s@r%d#%s", templateID, revision, definitionHash)
}

func projectExcelFormReference(row Record) string {
	code := strings.TrimSpace(fmt.Sprint(row["form_template_code"]))
	versionID := cleanID(row["form_version_id"])
	version := int(number(row["version_no"]))
	if code == "" || versionID == "" || version < 1 {
		return ""
	}
	return fmt.Sprintf("%s@v%d#%s", code, version, versionID)
}

func generateProjectExcelWorkbook(catalog projectExcelCatalog, projects []Record, generatedAt time.Time, exported bool) (tabularExportResult, error) {
	missingReferences := 0
	runtimeMismatches := 0
	projectRows := make([]Record, 0, len(projects))
	metadataRows := make([]Record, 0, len(projects))
	for _, project := range projects {
		state := strings.TrimSpace(fmt.Sprint(project["reference_state"]))
		if state == "missing" {
			missingReferences++
		}
		if state == "runtime_mismatch" {
			runtimeMismatches++
		}
		projectRows = append(projectRows, Record{
			"project_code":             exportCellText(project["project_code"]),
			"project_name":             exportCellText(project["project_name"]),
			"client_partner_code":      exportCellText(project["client_partner_code"]),
			"client_name_unregistered": exportCellText(project["client_name_unregistered"]),
			"address":                  exportCellText(project["address"]),
			"planned_start_date":       projectExcelDateCell(project["planned_start_date"]),
			"planned_end_date":         projectExcelDateCell(project["planned_end_date"]),
			"estimated_budget_vnd":     projectExcelVNDCell(project["estimated_budget_vnd"]),
			"pipeline_ref":             exportCellText(project["pipeline_ref"]),
		})
		metadataRows = append(metadataRows, Record{
			"project_id": project["project_id"], "project_code": project["project_code"], "status": project["status"],
			"progress_percent": project["progress_percent"], "contract_amount_snapshot": projectExcelVNDCell(project["contract_amount"]),
			"selected_pipeline_ref": project["selected_pipeline_ref"], "selected_pipeline_name": project["selected_template_name"],
			"runtime_pipeline_ref": project["runtime_pipeline_ref"], "runtime_pipeline_name": project["runtime_template_name"],
			"runtime_id": project["runtime_id"], "runtime_engine_version": project["runtime_engine_version"],
			"runtime_definition_hash": project["runtime_template_definition_hash"], "reference_state": state,
			"pm_staff_id": project["pm_staff_id"], "pm_name": project["pm_name"],
			"created_at": project["created_at"], "updated_at": project["updated_at"],
		})
	}

	guideRows := []Record{
		{"muc": "workbook_version", "noi_dung": projectExcelWorkbookVersion},
		{"muc": "generated_at", "noi_dung": generatedAt.UTC().Format(time.RFC3339)},
		{"muc": "pham_vi", "noi_dung": map[bool]string{true: "File xuất công trình", false: "File mẫu import công trình"}[exported]},
		{"muc": "quy_tac_1", "noi_dung": "Chỉ chỉnh sửa và import sheet Cong_trinh; giữ nguyên đúng chín tiêu đề cột."},
		{"muc": "quy_tac_2", "noi_dung": "project_code, project_name và pipeline_ref là bắt buộc. Mỗi lần tối đa 100 công trình, file tối đa 5 MB."},
		{"muc": "quy_tac_3", "noi_dung": "pipeline_ref phải lấy nguyên văn từ Ref_Pipeline và chỉ dòng ready=true mới được dùng."},
		{"muc": "quy_tac_4", "noi_dung": "client_partner_code lấy từ Ref_Chu_dau_tu; nếu bỏ trống thì phải nhập client_name_unregistered."},
		{"muc": "quy_tac_5", "noi_dung": "Import chỉ tạo master Công trình ở trạng thái draft; không khởi chạy Pipeline, không tạo runtime, Form answers, thành viên hoặc dữ liệu nghiệp vụ."},
		{"muc": "quy_tac_6", "noi_dung": "contract_amount_snapshot chỉ đọc. Nguồn chính thức là hợp đồng client đã ký; không nhập giá trị này từ Excel."},
		{"muc": "readiness", "noi_dung": fmt.Sprintf("ready=%t; pipelines=%d; forms=%d; fields=%d", boolValue(catalog.Readiness["ready"]), int(number(catalog.Readiness["ready_pipeline_count"])), int(number(catalog.Readiness["published_form_version_count"])), int(number(catalog.Readiness["field_count"])))},
		{"muc": "reference_fingerprint", "noi_dung": catalog.ReferenceFingerprint},
	}
	if exported {
		guideRows = append(guideRows,
			Record{"muc": "exported_project_count", "noi_dung": strconv.Itoa(len(projects))},
			Record{"muc": "missing_reference_count", "noi_dung": strconv.Itoa(missingReferences)},
			Record{"muc": "runtime_mismatch_count", "noi_dung": strconv.Itoa(runtimeMismatches)},
			Record{"muc": "can_import_again", "noi_dung": "File xuất là dữ liệu tham khảo. Import create-only sẽ từ chối mã công trình đã tồn tại; dòng thiếu pipeline_ref phải được cập nhật trước."},
		)
	}

	pipelineRows := make([]Record, 0, len(catalog.Pipelines))
	for _, source := range catalog.Pipelines {
		pipelineRows = append(pipelineRows, Record{
			"pipeline_ref": source["pipeline_ref"], "template_id": source["template_id"], "template_family_id": source["template_family_id"],
			"name": source["name"], "project_type": source["project_type"], "revision_no": source["revision_no"],
			"engine_version": source["engine_version"], "definition_hash": source["definition_hash"], "node_count": source["node_count"],
			"step_count": source["step_count"], "required_form_count": source["required_form_count"],
			"invalid_form_reference_count": source["invalid_form_reference_count"], "zero_field_reference_count": source["zero_field_reference_count"],
			"ready": source["ready"],
		})
	}
	formRows := make([]Record, 0, len(catalog.Forms))
	for _, source := range catalog.Forms {
		formRows = append(formRows, Record{
			"form_ref": source["form_ref"], "form_template_id": source["form_template_id"], "form_template_code": source["form_template_code"],
			"form_template_name": source["form_template_name"], "form_type": source["form_type"], "form_version_id": source["form_version_id"],
			"version_no": source["version_no"], "status": source["status"], "field_count": source["field_count"],
		})
	}
	fieldRows := make([]Record, 0, len(catalog.Fields))
	for _, source := range catalog.Fields {
		fieldRows = append(fieldRows, Record{
			"form_ref": source["form_ref"], "form_template_code": source["form_template_code"], "form_version_id": source["form_version_id"],
			"version_no": source["version_no"], "field_key": source["field_key"], "label": source["label"], "field_type": source["field_type"],
			"required": source["required"], "options_json": projectExcelJSONText(source["options_json"]),
			"validation_json": projectExcelJSONText(source["validation_json"]), "visibility_condition_json": projectExcelJSONText(source["visibility_condition_json"]),
			"order_index": source["order_index"],
		})
	}
	partnerRows := make([]Record, 0, len(catalog.Partners))
	for _, source := range catalog.Partners {
		partnerRows = append(partnerRows, Record{
			"partner_code": source["partner_code"], "name": source["name"], "tax_code": source["tax_code"], "updated_at": source["updated_at"],
		})
	}

	sheets := []xlsxWorkbookSheet{
		{Name: "Huong_dan", Columns: []tabularColumn{{"muc", "muc", false}, {"noi_dung", "noi_dung", false}}, Rows: guideRows},
		{Name: "Cong_trinh", Columns: projectExcelImportColumns(), Rows: projectRows},
		{Name: "Thong_tin_xuat", Columns: []tabularColumn{
			{"project_id", "project_id", false}, {"project_code", "project_code", false}, {"status", "status", false}, {"progress_percent", "progress_percent", true},
			{"contract_amount_snapshot", "contract_amount_snapshot", false}, {"selected_pipeline_ref", "selected_pipeline_ref", false},
			{"selected_pipeline_name", "selected_pipeline_name", false}, {"runtime_pipeline_ref", "runtime_pipeline_ref", false},
			{"runtime_pipeline_name", "runtime_pipeline_name", false}, {"runtime_id", "runtime_id", false}, {"runtime_engine_version", "runtime_engine_version", true},
			{"runtime_definition_hash", "runtime_definition_hash", false}, {"reference_state", "reference_state", false},
			{"pm_staff_id", "pm_staff_id", false}, {"pm_name", "pm_name", false}, {"created_at", "created_at", false}, {"updated_at", "updated_at", false},
		}, Rows: metadataRows},
		{Name: "Ref_Pipeline", Columns: []tabularColumn{
			{"pipeline_ref", "pipeline_ref", false}, {"template_id", "template_id", false}, {"template_family_id", "template_family_id", false},
			{"name", "name", false}, {"project_type", "project_type", false}, {"revision_no", "revision_no", true}, {"engine_version", "engine_version", true},
			{"definition_hash", "definition_hash", false}, {"node_count", "node_count", true}, {"step_count", "step_count", true},
			{"required_form_count", "required_form_count", true}, {"invalid_form_reference_count", "invalid_form_reference_count", true},
			{"zero_field_reference_count", "zero_field_reference_count", true}, {"ready", "ready", false},
		}, Rows: pipelineRows},
		{Name: "Ref_Bieu_mau", Columns: []tabularColumn{
			{"form_ref", "form_ref", false}, {"form_template_id", "form_template_id", false}, {"form_template_code", "form_template_code", false},
			{"form_template_name", "form_template_name", false}, {"form_type", "form_type", false}, {"form_version_id", "form_version_id", false},
			{"version_no", "version_no", true}, {"status", "status", false}, {"field_count", "field_count", true},
		}, Rows: formRows},
		{Name: "Ref_Fields", Columns: []tabularColumn{
			{"form_ref", "form_ref", false}, {"form_template_code", "form_template_code", false}, {"form_version_id", "form_version_id", false},
			{"version_no", "version_no", true}, {"field_key", "field_key", false}, {"label", "label", false}, {"field_type", "field_type", false},
			{"required", "required", false}, {"options_json", "options_json", false}, {"validation_json", "validation_json", false},
			{"visibility_condition_json", "visibility_condition_json", false}, {"order_index", "order_index", true},
		}, Rows: fieldRows},
		{Name: "Ref_Chu_dau_tu", Columns: []tabularColumn{
			{"partner_code", "partner_code", false}, {"name", "name", false}, {"tax_code", "tax_code", false}, {"updated_at", "updated_at", false},
		}, Rows: partnerRows},
	}
	data, err := generateWorkbookXLSX("Import/Export Công trình", sheets, generatedAt)
	if err != nil {
		return tabularExportResult{}, err
	}
	filename := "mau-import-cong-trinh.xlsx"
	if exported {
		filename = "cong-trinh-" + generatedAt.In(time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)).Format("20060102-150405") + ".xlsx"
	}
	return tabularExportResult{Data: data, Filename: filename, ContentType: exportXLSXContentType}, nil
}

func generateProjectJSONPackage(catalog projectExcelCatalog, projects []Record, generatedAt time.Time, exported bool) (tabularExportResult, error) {
	projectRows := make([]Record, 0, len(projects))
	snapshots := make([]Record, 0, len(projects))
	for _, project := range projects {
		projectRows = append(projectRows, Record{
			"project_code":             fmt.Sprint(project["project_code"]),
			"project_name":             fmt.Sprint(project["project_name"]),
			"client_partner_code":      fmt.Sprint(project["client_partner_code"]),
			"client_name_unregistered": fmt.Sprint(project["client_name_unregistered"]),
			"address":                  fmt.Sprint(project["address"]),
			"planned_start_date":       projectExcelDateCell(project["planned_start_date"]),
			"planned_end_date":         projectExcelDateCell(project["planned_end_date"]),
			"estimated_budget_vnd":     projectExcelVNDCell(project["estimated_budget_vnd"]),
			"pipeline_ref":             fmt.Sprint(project["pipeline_ref"]),
		})
		snapshots = append(snapshots, Record{
			"project_id": project["project_id"], "project_code": project["project_code"], "status": project["status"],
			"progress_percent": project["progress_percent"], "contract_amount_snapshot": projectExcelVNDCell(project["contract_amount"]),
			"selected_pipeline_ref": project["selected_pipeline_ref"], "selected_pipeline_name": project["selected_template_name"],
			"runtime_pipeline_ref": project["runtime_pipeline_ref"], "runtime_pipeline_name": project["runtime_template_name"],
			"runtime_id": project["runtime_id"], "runtime_engine_version": project["runtime_engine_version"],
			"runtime_definition_hash": project["runtime_template_definition_hash"], "reference_state": project["reference_state"],
			"pm_staff_id": project["pm_staff_id"], "pm_name": project["pm_name"],
			"created_at": project["created_at"], "updated_at": project["updated_at"],
		})
	}

	pipelines := make([]Record, 0, len(catalog.Pipelines))
	for _, source := range catalog.Pipelines {
		pipelines = append(pipelines, Record{
			"pipeline_ref": source["pipeline_ref"], "template_id": source["template_id"], "template_family_id": source["template_family_id"],
			"name": source["name"], "project_type": source["project_type"], "revision_no": source["revision_no"],
			"engine_version": source["engine_version"], "definition_hash": source["definition_hash"], "node_count": source["node_count"],
			"step_count": source["step_count"], "required_form_count": source["required_form_count"],
			"invalid_form_reference_count": source["invalid_form_reference_count"], "zero_field_reference_count": source["zero_field_reference_count"],
			"ready": source["ready"],
		})
	}
	forms := make([]Record, 0, len(catalog.Forms))
	for _, source := range catalog.Forms {
		forms = append(forms, Record{
			"form_ref": source["form_ref"], "form_template_id": source["form_template_id"], "form_template_code": source["form_template_code"],
			"form_template_name": source["form_template_name"], "form_type": source["form_type"], "form_version_id": source["form_version_id"],
			"version_no": source["version_no"], "status": source["status"], "field_count": source["field_count"],
		})
	}
	fields := make([]Record, 0, len(catalog.Fields))
	for _, source := range catalog.Fields {
		fields = append(fields, Record{
			"form_ref": source["form_ref"], "form_template_code": source["form_template_code"], "form_version_id": source["form_version_id"],
			"version_no": source["version_no"], "field_key": source["field_key"], "label": source["label"], "field_type": source["field_type"],
			"required": source["required"], "options_json": defaultWorkflowDefinitionJSON(source["options_json"], []any{}),
			"validation_json":           defaultWorkflowDefinitionJSON(source["validation_json"], Record{}),
			"visibility_condition_json": defaultWorkflowDefinitionJSON(source["visibility_condition_json"], Record{}), "order_index": source["order_index"],
		})
	}
	clients := make([]Record, 0, len(catalog.Partners))
	for _, source := range catalog.Partners {
		clients = append(clients, Record{
			"partner_code": source["partner_code"], "name": source["name"], "tax_code": source["tax_code"], "updated_at": source["updated_at"],
		})
	}
	mode := "template"
	if exported {
		mode = "export"
	}
	data, err := marshalDataPackageJSON(struct {
		Schema               string   `json:"schema"`
		SchemaVersion        int      `json:"schema_version"`
		Mode                 string   `json:"mode"`
		GeneratedAt          string   `json:"generated_at"`
		ReferenceFingerprint string   `json:"reference_fingerprint"`
		Instructions         []string `json:"instructions"`
		Readiness            Record   `json:"readiness"`
		Projects             []Record `json:"projects"`
		ProjectSnapshots     []Record `json:"project_snapshots"`
		References           struct {
			Pipelines []Record `json:"pipelines"`
			Forms     []Record `json:"forms"`
			Fields    []Record `json:"fields"`
			Clients   []Record `json:"clients"`
		} `json:"references"`
	}{
		Schema: "consflow.project-package", SchemaVersion: 1, Mode: mode, GeneratedAt: generatedAt.UTC().Format(time.RFC3339),
		ReferenceFingerprint: catalog.ReferenceFingerprint,
		Instructions: []string{
			"Chỉ chỉnh mảng projects; project_snapshots và references là dữ liệu chỉ đọc.",
			"Import chỉ tạo master Công trình ở trạng thái draft; không khởi chạy Pipeline hoặc tạo dữ liệu runtime.",
		},
		Readiness: catalog.Readiness, Projects: projectRows, ProjectSnapshots: snapshots,
		References: struct {
			Pipelines []Record `json:"pipelines"`
			Forms     []Record `json:"forms"`
			Fields    []Record `json:"fields"`
			Clients   []Record `json:"clients"`
		}{Pipelines: pipelines, Forms: forms, Fields: fields, Clients: clients},
	})
	if err != nil {
		return tabularExportResult{}, err
	}
	filename := "mau-import-cong-trinh.json"
	if exported {
		filename = "cong-trinh-" + generatedAt.In(time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)).Format("20060102-150405") + ".json"
	}
	return tabularExportResult{Data: data, Filename: filename, ContentType: exportJSONContentType}, nil
}

func projectExcelImportColumns() []tabularColumn {
	return []tabularColumn{
		{"project_code", "project_code", false}, {"project_name", "project_name", false},
		{"client_partner_code", "client_partner_code", false}, {"client_name_unregistered", "client_name_unregistered", false},
		{"address", "address", false}, {"planned_start_date", "planned_start_date", false}, {"planned_end_date", "planned_end_date", false},
		{"estimated_budget_vnd", "estimated_budget_vnd", false}, {"pipeline_ref", "pipeline_ref", false},
	}
}

func projectExcelDateCell(value any) string {
	text := exportCellText(value)
	if len(text) >= 10 {
		return text[:10]
	}
	return text
}

func projectExcelVNDCell(value any) string {
	text := exportCellText(value)
	separator := strings.IndexByte(text, '.')
	if separator < 0 {
		return text
	}
	for _, digit := range text[separator+1:] {
		if digit != '0' {
			return text
		}
	}
	return text[:separator]
}

func projectExcelJSONText(value any) string {
	if value == nil {
		return ""
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return string(encoded)
}
