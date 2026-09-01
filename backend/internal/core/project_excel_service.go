package core

import (
	"context"
	"github.com/google/uuid"
	"strings"
	"time"
)

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
