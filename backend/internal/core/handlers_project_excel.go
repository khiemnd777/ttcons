package core

import (
	"strings"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) ProjectExcelReadiness(c *fiber.Ctx) error {
	if err := h.requireProjectExcelImportPermissions(c); err != nil {
		return err
	}
	result, err := h.service.ProjectExcelReadiness(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) ProjectExcelTemplate(c *fiber.Ctx) error {
	if err := h.requireProjectExcelImportPermissions(c); err != nil {
		return err
	}
	result, err := h.service.ProjectImportTemplate(c.Context(), c.Query("format", dataPackageFormatXLSX))
	if err != nil {
		return err
	}
	return sendExport(c, result)
}

func (h *Handler) PreviewProjectExcelImport(c *fiber.Ctx) error {
	if err := h.requireProjectExcelImportPermissions(c); err != nil {
		return err
	}
	var req ProjectExcelImportPreviewRequest
	if err := decodeStrictJSON(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xem trước import công trình không hợp lệ")
	}
	result, err := h.service.PreviewProjectExcelImport(c.Context(), req)
	if err != nil {
		return err
	}
	return response.OK(c, result, "Đã kiểm tra danh sách công trình")
}

func (h *Handler) ExecuteProjectExcelImport(c *fiber.Ctx) error {
	if err := h.requireProjectExcelImportPermissions(c); err != nil {
		return err
	}
	var req ProjectExcelImportExecuteRequest
	if err := decodeStrictJSON(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu thực thi import công trình không hợp lệ")
	}
	result, err := h.service.ExecuteProjectExcelImport(c.Context(), actorID(c), actorName(c), req)
	if err != nil {
		return err
	}
	return response.Created(c, result)
}

func (h *Handler) ExportProjectsExcel(c *fiber.Ctx) error {
	for _, permission := range []string{"project.export", "project.view"} {
		if err := h.requirePermission(c, []string{permission}); err != nil {
			return err
		}
	}
	request := ProjectExcelExportRequest{
		Format:     c.Query("format", dataPackageFormatXLSX),
		Scope:      c.Query("scope", "filtered"),
		ProjectIDs: splitProjectExcelIDs(c.Query("ids")),
		Search:     c.Query("search"),
		Status:     c.Query("status"),
		Sort:       c.Query("sort", "created_at"),
		Direction:  c.Query("direction", "desc"),
	}
	result, err := h.service.ExportProjectsExcel(c.Context(), actorID(c), actorName(c), request)
	if err != nil {
		return err
	}
	return sendExport(c, result)
}

func (h *Handler) requireProjectExcelImportPermissions(c *fiber.Ctx) error {
	for _, permission := range []string{"project.import", "project.create", "pipeline.view", "catalog.view"} {
		if err := h.requirePermission(c, []string{permission}); err != nil {
			return err
		}
	}
	return nil
}

func splitProjectExcelIDs(value string) []string {
	result := []string{}
	for _, raw := range strings.Split(value, ",") {
		if id := strings.TrimSpace(raw); id != "" {
			result = append(result, id)
		}
	}
	return result
}
