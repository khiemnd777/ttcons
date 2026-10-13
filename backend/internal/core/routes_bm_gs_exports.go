package core

import "github.com/gofiber/fiber/v2"

func registerBMGSExportRoutes(protected fiber.Router, h *Handler) {
	protected.Get("/daily-reports/:id/export.xlsx", h.ExportNativeDailyReport)
	protected.Get("/work-reports/:id/revisions/:revision/export.xlsx", h.ExportNativeWorkReport)
	protected.Get("/projects/:id/daily-reports/export.xlsx", h.ExportNativeDailyReports)
	protected.Get("/work-reports/export.xlsx", h.ExportNativeWorkReports)
	protected.Get("/workflow-form-instances/:id/export.xlsx", h.ExportNativeWorkflowForm)
	protected.Get("/workflow-form-instances/:id/risks/export.xlsx", h.ExportNativeWorkflowRisks)
	protected.Get("/workflow-form-instances/:id/next-plan/export.xlsx", h.ExportNativeWorkflowNextPlan)
	protected.Get("/daily-reports/:id/risks/export.xlsx", h.ExportNativeDailyRisks)
	protected.Get("/daily-reports/:id/next-plan/export.xlsx", h.ExportNativeDailyNextPlan)
	protected.Get("/construction-actions/:id/export.xlsx", h.ExportNativeConstructionAction)
	protected.Get("/projects/:id/construction-actions/export.xlsx", h.ExportNativeConstructionActions)
	protected.Get("/work-items/:id/photos/export.xlsx", h.ExportNativeWorkItemPhotos)
	protected.Get("/handover/:id/export.xlsx", h.ExportNativeHandover)
}
