package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"sort"
	"strconv"
	"strings"
	"time"
)

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
