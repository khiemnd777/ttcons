package core

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) exportNativeOwner(c *fiber.Ctx, export func(context.Context) (tabularExportResult, error)) error {
	ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
	defer cancel()
	result, err := export(ctx)
	if err != nil {
		return err
	}
	return sendExport(c, result)
}
func (h *Handler) ExportNativeWorkflowForm(c *fiber.Ctx) error {
	return h.exportNativeOwner(c, func(ctx context.Context) (tabularExportResult, error) {
		return h.service.ExportNativeWorkflowForm(ctx, c.Params("id"), "default", actorID(c), actorName(c))
	})
}
func (h *Handler) ExportNativeWorkflowRisks(c *fiber.Ctx) error {
	return h.exportNativeOwner(c, func(ctx context.Context) (tabularExportResult, error) {
		return h.service.ExportNativeWorkflowForm(ctx, c.Params("id"), "risks", actorID(c), actorName(c))
	})
}
func (h *Handler) ExportNativeWorkflowNextPlan(c *fiber.Ctx) error {
	return h.exportNativeOwner(c, func(ctx context.Context) (tabularExportResult, error) {
		return h.service.ExportNativeWorkflowForm(ctx, c.Params("id"), "next_plan", actorID(c), actorName(c))
	})
}
func (h *Handler) ExportNativeDailyRisks(c *fiber.Ctx) error {
	return h.exportNativeOwner(c, func(ctx context.Context) (tabularExportResult, error) {
		return h.service.ExportNativeDailyBusiness(ctx, c.Params("id"), "risks", actorID(c), actorName(c))
	})
}
func (h *Handler) ExportNativeDailyNextPlan(c *fiber.Ctx) error {
	return h.exportNativeOwner(c, func(ctx context.Context) (tabularExportResult, error) {
		return h.service.ExportNativeDailyBusiness(ctx, c.Params("id"), "next_plan", actorID(c), actorName(c))
	})
}
func (h *Handler) ExportNativeConstructionAction(c *fiber.Ctx) error {
	return h.exportNativeOwner(c, func(ctx context.Context) (tabularExportResult, error) {
		return h.service.ExportNativeConstructionAction(ctx, c.Params("id"), actorID(c), actorName(c))
	})
}

func (h *Handler) ExportNativeConstructionActions(c *fiber.Ctx) error {
	return h.exportNativeOwner(c, func(ctx context.Context) (tabularExportResult, error) {
		return h.service.ExportNativeConstructionActions(ctx, c.Params("id"), NativeConstructionActionFilter{Queue: c.Query("queue", "open"), Search: c.Query("search"), AssigneeID: c.Query("assignee_id"), Priority: c.Query("priority"), ImpactScope: c.Query("impact_scope")}, actorID(c), actorName(c))
	})
}
func (h *Handler) ExportNativeWorkItemPhotos(c *fiber.Ctx) error {
	return h.exportNativeOwner(c, func(ctx context.Context) (tabularExportResult, error) {
		return h.service.ExportNativeWorkItemPhotos(ctx, c.Params("id"), actorID(c), actorName(c))
	})
}
func (h *Handler) ExportNativeHandover(c *fiber.Ctx) error {
	return h.exportNativeOwner(c, func(ctx context.Context) (tabularExportResult, error) {
		return h.service.ExportNativeHandover(ctx, c.Params("id"), actorID(c), actorName(c))
	})
}
