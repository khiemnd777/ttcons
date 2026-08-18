package core

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) StatisticsOptions(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"statistics.view"}); err != nil {
		return err
	}
	req, err := projectOptionsRequest(c)
	if err != nil {
		return err
	}
	data, err := h.service.StatisticsProjects(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func (h *Handler) ProjectStatistic(c *fiber.Ctx) error     { return h.statistic(c, "projects") }
func (h *Handler) WarehouseStatistic(c *fiber.Ctx) error   { return h.statistic(c, "warehouse") }
func (h *Handler) ProcurementStatistic(c *fiber.Ctx) error { return h.statistic(c, "procurement") }
func (h *Handler) StaffStatistic(c *fiber.Ctx) error       { return h.statistic(c, "staff") }

func (h *Handler) ProjectStatisticExport(c *fiber.Ctx) error {
	return h.statisticExport(c, "projects")
}
func (h *Handler) WarehouseStatisticExport(c *fiber.Ctx) error {
	return h.statisticExport(c, "warehouse")
}
func (h *Handler) ProcurementStatisticExport(c *fiber.Ctx) error {
	return h.statisticExport(c, "procurement")
}
func (h *Handler) StaffStatisticExport(c *fiber.Ctx) error {
	return h.statisticExport(c, "staff")
}

func (h *Handler) statistic(c *fiber.Ctx, kind string) error {
	if err := h.requirePermission(c, []string{"statistics.view"}); err != nil {
		return err
	}
	req, err := operationalReportRequest(c)
	if err != nil {
		return err
	}
	rows, err := h.service.Statistic(c.Context(), kind, req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) statisticExport(c *fiber.Ctx, kind string) error {
	if err := h.requirePermission(c, []string{"statistics.export"}); err != nil {
		return err
	}
	req, err := operationalReportRequest(c)
	if err != nil {
		return err
	}
	result, err := h.service.ExportStatistic(c.Context(), kind, c.Query("format", "csv"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return sendExport(c, result)
}

func (h *Handler) WorkReports(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"work_report.view"}); err != nil {
		return err
	}
	filter := workReportFilter(c)
	rows, err := h.service.WorkReports(c.Context(), filter, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) WorkReportProjects(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"work_report.view"}); err != nil {
		return err
	}
	req, err := projectOptionsRequest(c)
	if err != nil {
		return err
	}
	data, err := h.service.WorkReportProjects(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func (h *Handler) WorkReportDetail(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"work_report.view"}); err != nil {
		return err
	}
	detail, err := h.service.WorkReportDetail(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, detail)
}

func (h *Handler) WorkReportRecipients(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"work_report.submit"}); err != nil {
		return err
	}
	rows, err := h.service.WorkReportRecipients(c.Context(), c.Query("project_id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) SubmitWorkReport(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"work_report.submit"}); err != nil {
		return err
	}
	req := WorkReportSubmitRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu gửi báo cáo không hợp lệ")
	}
	detail, err := h.service.SubmitWorkReport(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, detail)
}

func (h *Handler) ReviewWorkReport(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"work_report.review", "work_report.manage"}); err != nil {
		return err
	}
	req := WorkReportReviewRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xem xét báo cáo không hợp lệ")
	}
	detail, err := h.service.ReviewWorkReport(c.Context(), c.Params("id"), c.Params("action"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, detail)
}

func (h *Handler) ExportWorkReports(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"work_report.export"}); err != nil {
		return err
	}
	result, err := h.service.ExportWorkReports(c.Context(), workReportFilter(c), c.Query("format", "csv"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return sendExport(c, result)
}

func (h *Handler) ExportWorkReportPDF(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"work_report.export"}); err != nil {
		return err
	}
	revisionNo, err := strconv.Atoi(strings.TrimSpace(c.Query("revision")))
	if err != nil || revisionNo < 1 {
		return validationError("revision không hợp lệ", Record{"revision": c.Query("revision")})
	}
	result, err := h.service.ExportWorkReportPDF(c.Context(), c.Params("id"), revisionNo, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return sendExport(c, result)
}

func workReportFilter(c *fiber.Ctx) WorkReportFilter {
	return WorkReportFilter{
		Scope:     c.Query("scope", "mine"),
		ProjectID: c.Query("project_id"),
		Status:    c.Query("status"),
		DateFrom:  c.Query("date_from"),
		DateTo:    c.Query("date_to"),
		Search:    c.Query("search"),
	}
}

func sendExport(c *fiber.Ctx, result tabularExportResult) error {
	c.Set(fiber.HeaderContentType, result.ContentType)
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, strings.ReplaceAll(result.Filename, `"`, "")))
	return c.Send(result.Data)
}
