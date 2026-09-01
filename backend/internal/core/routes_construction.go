package core

import "github.com/gofiber/fiber/v2"

func registerConstructionRoutes(protected fiber.Router, h *Handler) {
	protected.Get("/projects/:id/daily-reports", h.ProjectDailyReports)
	protected.Post("/projects/:id/daily-reports", h.CreateDailyReport)
	protected.Put("/daily-reports/:id", h.UpdateDailyReport)
	protected.Post("/daily-reports/:id/evidence/upload", h.UploadDailyReportEvidence)
	protected.Get("/projects/:id/construction-workspace", h.ProjectConstructionWorkspace)
	protected.Get("/projects/:id/construction-variation-context", h.ProjectConstructionVariationContext)
	protected.Post("/projects/:id/construction-variation-proposals", h.ProposeConstructionVariation)
	protected.Get("/projects/:id/construction-handoff", h.ProjectConstructionHandoff)
	protected.Post("/projects/:id/construction-handoff/activate", h.ActivateConstructionHandoff)
	protected.Post("/projects/:id/construction-check-templates", h.CreateConstructionCheckTemplate)
	protected.Post("/projects/:id/construction-checks", h.CreateConstructionCheck)
	protected.Put("/construction-checks/:id", h.UpdateConstructionCheck)
	protected.Post("/construction-checks/:id/complete", h.CompleteConstructionCheck)
	protected.Post("/projects/:id/construction-actions", h.CreateConstructionAction)
	protected.Put("/construction-actions/:id", h.UpdateConstructionAction)
	protected.Post("/construction-actions/:id/submit-resolution", h.SubmitConstructionActionResolution)
	protected.Post("/construction-actions/:id/verify", h.VerifyConstructionAction)
	protected.Post("/construction-actions/:id/approve-impact", h.ApproveConstructionActionImpact)
	protected.Get("/projects/:id/issues", h.ProjectIssues)
	protected.Post("/projects/:id/issues", h.CreateIssue)
	protected.Put("/issues/:id", h.UpdateIssue)
}
