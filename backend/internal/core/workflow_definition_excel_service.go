package core

import (
	"context"
	"github.com/google/uuid"
	"sort"
	"strings"
	"time"
)

func (s *Service) WorkflowDefinitionExcelTemplate(ctx context.Context, scope string) (tabularExportResult, error) {
	return s.WorkflowDefinitionImportTemplate(ctx, scope, dataPackageFormatXLSX)
}

func (s *Service) WorkflowDefinitionImportTemplate(ctx context.Context, scope, format string) (tabularExportResult, error) {
	scope, err := normalizeWorkflowDefinitionScope(scope)
	if err != nil {
		return tabularExportResult{}, err
	}
	catalog, err := s.repo.WorkflowDefinitionCatalog(ctx)
	if err != nil {
		return tabularExportResult{}, err
	}
	packageData := workflowDefinitionPackage{Scope: scope}
	if format, err = normalizeDataPackageFormat(format); err != nil {
		return tabularExportResult{}, err
	}
	if format == dataPackageFormatJSON {
		return generateWorkflowDefinitionJSONPackage(packageData, catalog, time.Now(), "mau")
	}
	return generateWorkflowDefinitionWorkbook(packageData, catalog, time.Now(), "mau")
}

func (s *Service) PreviewWorkflowDefinitionExcelImport(ctx context.Context, req WorkflowDefinitionImportPreviewRequest) (Record, error) {
	validation, err := s.validateWorkflowDefinitionImport(ctx, req)
	if err != nil {
		return nil, err
	}
	return validation.Preview, nil
}

func (s *Service) ExecuteWorkflowDefinitionExcelImport(ctx context.Context, actorID, actorName string, req WorkflowDefinitionImportExecuteRequest) (Record, error) {
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if _, err := uuid.Parse(idempotencyKey); err != nil {
		return nil, validationError("Mã chống tạo trùng không hợp lệ", Record{"field": "idempotency_key"})
	}
	previewHash := strings.TrimSpace(req.PreviewHash)
	if stored, found, err := s.repo.WorkflowDefinitionImportResult(ctx, actorID, idempotencyKey, previewHash); err != nil {
		return nil, err
	} else if found {
		stored["idempotent_replay"] = true
		return stored, nil
	}
	validation, err := s.validateWorkflowDefinitionImport(ctx, req.WorkflowDefinitionImportPreviewRequest)
	if err != nil {
		return nil, err
	}
	if number(validation.Preview["blocker_count"]) > 0 {
		return nil, validationError("Gói định nghĩa workflow còn điểm chặn", validation.Preview)
	}
	if previewHash == "" || previewHash != validation.Hash {
		return nil, conflictError("Dữ liệu hoặc danh mục tham chiếu đã thay đổi sau bước xem trước. Vui lòng xem trước lại", Record{"field": "preview_hash"})
	}
	return s.repo.ExecuteWorkflowDefinitionImport(ctx, actorID, actorName, idempotencyKey, validation.SourceFormat, strings.TrimSpace(req.SourceFilename), req.SourceSizeBytes, validation.Hash, validation.Catalog.Fingerprint, validation.Package)
}

func (s *Service) ExportWorkflowDefinitionsExcel(ctx context.Context, actorID, actorName string, req WorkflowDefinitionExportRequest) (tabularExportResult, error) {
	format, err := normalizeDataPackageFormat(req.Format)
	if err != nil {
		return tabularExportResult{}, err
	}
	scope, err := normalizeWorkflowDefinitionExportScope(req.Scope)
	if err != nil {
		return tabularExportResult{}, err
	}
	if len(req.IDs) == 0 || len(req.IDs) > workflowDefinitionMaximumPipelines {
		return tabularExportResult{}, validationError("Cần chọn từ 1 đến 25 định nghĩa để xuất", Record{"field": "ids", "maximum": workflowDefinitionMaximumPipelines})
	}
	seen := map[string]bool{}
	for index, id := range req.IDs {
		id = strings.TrimSpace(id)
		if _, err := uuid.Parse(id); err != nil {
			return tabularExportResult{}, validationError("ID định nghĩa xuất không hợp lệ", Record{"id": id})
		}
		if seen[id] {
			return tabularExportResult{}, validationError("Danh sách định nghĩa xuất bị trùng", Record{"id": id})
		}
		seen[id] = true
		req.IDs[index] = id
	}
	catalog, err := s.repo.WorkflowDefinitionCatalog(ctx)
	if err != nil {
		return tabularExportResult{}, err
	}
	packageData := workflowDefinitionPackage{Scope: scope}
	if scope == "form" {
		for _, versionID := range req.IDs {
			detail, err := s.repo.WorkflowFormVersionDetail(ctx, versionID)
			if err != nil {
				return tabularExportResult{}, err
			}
			appendWorkflowDefinitionForm(&packageData, detail)
		}
	} else {
		formIDs := map[string]bool{}
		for _, templateID := range req.IDs {
			detail, err := s.repo.GetTemplateDetail(ctx, templateID)
			if err != nil {
				return tabularExportResult{}, err
			}
			appendWorkflowDefinitionPipeline(&packageData, detail, catalog, req.IncludeDependencies, formIDs)
		}
		if req.IncludeDependencies {
			packageData.Scope = "bundle"
			ids := make([]string, 0, len(formIDs))
			for id := range formIDs {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			for _, id := range ids {
				detail, err := s.repo.WorkflowFormVersionDetail(ctx, id)
				if err != nil {
					return tabularExportResult{}, err
				}
				appendWorkflowDefinitionForm(&packageData, detail)
			}
		}
	}
	var result tabularExportResult
	if format == dataPackageFormatJSON {
		result, err = generateWorkflowDefinitionJSONPackage(packageData, catalog, time.Now(), "xuat")
	} else {
		result, err = generateWorkflowDefinitionWorkbook(packageData, catalog, time.Now(), "xuat")
	}
	if err != nil {
		return tabularExportResult{}, err
	}
	s.repo.AddAuditBestEffort(ctx, "", actorID, actorName, "workflow_definitions_exported", "workflow_definition_export", "", "Xuất định nghĩa workflow", Record{
		"source_format": format,
		"scope":         scope, "selected_count": len(req.IDs), "include_dependencies": req.IncludeDependencies,
		"pipeline_count": len(packageData.Pipelines), "form_count": len(packageData.Forms),
	})
	return result, nil
}
