package core

import (
	"strconv"
	"strings"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) WorkflowDefinitionExcelTemplate(c *fiber.Ctx) error {
	scope := strings.TrimSpace(c.Query("scope", "bundle"))
	if err := h.requireWorkflowDefinitionImportPermissions(c, scope); err != nil {
		return err
	}
	result, err := h.service.WorkflowDefinitionImportTemplate(c.Context(), scope, c.Query("format", dataPackageFormatXLSX))
	if err != nil {
		return err
	}
	return sendExport(c, result)
}

func (h *Handler) PreviewWorkflowDefinitionExcelImport(c *fiber.Ctx) error {
	var req WorkflowDefinitionImportPreviewRequest
	if err := decodeStrictJSON(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xem trước import định nghĩa workflow không hợp lệ")
	}
	if err := h.requireWorkflowDefinitionImportPermissions(c, req.Scope); err != nil {
		return err
	}
	result, err := h.service.PreviewWorkflowDefinitionExcelImport(c.Context(), req)
	if err != nil {
		return err
	}
	return response.OK(c, result, "Đã kiểm tra gói định nghĩa workflow")
}

func (h *Handler) ExecuteWorkflowDefinitionExcelImport(c *fiber.Ctx) error {
	var req WorkflowDefinitionImportExecuteRequest
	if err := decodeStrictJSON(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu thực thi import định nghĩa workflow không hợp lệ")
	}
	if err := h.requireWorkflowDefinitionImportPermissions(c, req.Scope); err != nil {
		return err
	}
	result, err := h.service.ExecuteWorkflowDefinitionExcelImport(c.Context(), actorID(c), actorName(c), req)
	if err != nil {
		return err
	}
	return response.Created(c, result)
}

func (h *Handler) ExportWorkflowDefinitionsExcel(c *fiber.Ctx) error {
	for _, permission := range []string{"workflow_definition.export", "pipeline.view"} {
		if err := h.requirePermission(c, []string{permission}); err != nil {
			return err
		}
	}
	includeDependencies := true
	if raw := strings.TrimSpace(c.Query("include_dependencies")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "include_dependencies không hợp lệ")
		}
		includeDependencies = parsed
	}
	result, err := h.service.ExportWorkflowDefinitionsExcel(c.Context(), actorID(c), actorName(c), WorkflowDefinitionExportRequest{
		Format:              strings.TrimSpace(c.Query("format", dataPackageFormatXLSX)),
		Scope:               strings.TrimSpace(c.Query("scope")),
		IDs:                 splitProjectExcelIDs(c.Query("ids")),
		IncludeDependencies: includeDependencies,
	})
	if err != nil {
		return err
	}
	return sendExport(c, result)
}

func (h *Handler) requireWorkflowDefinitionImportPermissions(c *fiber.Ctx, scope string) error {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if err := h.requirePermission(c, []string{"workflow_definition.import"}); err != nil {
		return err
	}
	if scope == "pipeline" || scope == "bundle" {
		if err := h.requirePermission(c, []string{"pipeline.create"}); err != nil {
			return err
		}
	}
	if scope == "form" || scope == "bundle" {
		if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
			return err
		}
	}
	return nil
}
