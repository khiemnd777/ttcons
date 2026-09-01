package core

import "github.com/gofiber/fiber/v2"

func registerQualityRoutes(protected fiber.Router, h *Handler) {
	protected.Get("/projects/:id/quality", h.ProjectQualityWorkspace)
	protected.Post("/projects/:id/quality/itp-plans", h.CreateQualityITP)
	protected.Post("/quality/itp-plans/:id/revise", h.ReviseQualityITP)
	protected.Post("/quality/itp-plans/:id/submit", h.SubmitQualityITP)
	protected.Post("/quality/itp-plans/:id/approve", h.ApproveQualityITP)
	protected.Post("/quality/itp-plans/:id/activate", h.ActivateQualityITP)
	protected.Post("/projects/:id/quality/rfis", h.CreateQualityInspectionRequest)
	protected.Post("/quality/rfis/:id/submit", h.SubmitQualityInspectionRequest)
	protected.Post("/quality/rfis/:id/schedule", h.ScheduleQualityInspectionRequest)
	protected.Post("/quality/rfis/:id/inspect", h.InspectQualityRequest)
	protected.Post("/projects/:id/quality/ncrs", h.CreateQualityNCR)
	protected.Post("/quality/ncrs/:id/root-cause", h.SubmitQualityNCRRootCause)
	protected.Post("/quality/ncrs/:id/corrective", h.SubmitQualityNCRCorrective)
	protected.Post("/quality/ncrs/:id/reinspection", h.LinkQualityNCRReinspection)
	protected.Post("/quality/ncrs/:id/close", h.CloseQualityNCR)
	protected.Post("/projects/:id/quality/recurring-plans", h.CreateRecurringPlan)
	protected.Post("/quality/recurring-plans/:id/generate", h.GenerateRecurringOccurrences)
	protected.Post("/quality/recurring-occurrences/:id/complete", h.CompleteRecurringOccurrence)
	protected.Post("/quality/recurring-occurrences/:id/skip", h.SkipRecurringOccurrence)
}
