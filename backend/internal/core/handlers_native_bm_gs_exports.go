package core

import (
	"context"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) ExportNativeDailyReport(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
	defer cancel()
	result, err := h.service.ExportNativeDailyReport(ctx, c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return sendExport(c, result)
}

func (h *Handler) ExportNativeWorkReport(c *fiber.Ctx) error {
	revision, err := strconv.Atoi(c.Params("revision"))
	if err != nil || revision < 1 {
		return validationError("Lần gửi báo cáo không hợp lệ")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
	defer cancel()
	result, err := h.service.ExportNativeWorkReport(ctx, c.Params("id"), revision, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return sendExport(c, result)
}

func (h *Handler) ExportNativeDailyReports(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
	defer cancel()
	result, err := h.service.ExportNativeDailyReports(ctx, c.Params("id"), NativeDailyReportFilter{DateFrom: c.Query("date_from"), DateTo: c.Query("date_to")}, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return sendExport(c, result)
}

func (h *Handler) ExportNativeWorkReports(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
	defer cancel()
	result, err := h.service.ExportNativeWorkReports(ctx, workReportFilter(c), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return sendExport(c, result)
}
