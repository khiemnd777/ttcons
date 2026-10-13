package core

import "github.com/gofiber/fiber/v2"

func registerAcceptanceHandoverRoutes(protected fiber.Router, h *Handler) {
	protected.Get("/projects/:id/acceptances", h.ProjectAcceptances)
	protected.Post("/projects/:id/acceptances", h.CreateAcceptance)
	protected.Get("/acceptances/:id", h.GetAcceptance)
	protected.Put("/acceptances/:id", h.UpdateAcceptance)
	protected.Post("/acceptances/:id/submit", h.SubmitAcceptance)
	protected.Post("/acceptances/:id/pass", h.PassAcceptance)
	protected.Post("/acceptances/:id/conditional-pass", h.ConditionalPassAcceptance)
	protected.Post("/acceptances/:id/fail", h.FailAcceptance)
	protected.Post("/acceptances/:id/create-rework", h.CreateReworkFromAcceptance)

	protected.Get("/projects/:id/reworks", h.ProjectReworks)
	protected.Post("/projects/:id/reworks", h.CreateRework)
	protected.Post("/reworks/:id/approve", h.ApproveRework)
	protected.Post("/reworks/:id/complete", h.CompleteRework)
	protected.Get("/projects/:id/replannings", h.ProjectReplannings)
	protected.Post("/projects/:id/replannings", h.CreateReplanning)
	protected.Post("/replannings/:id/approve", h.ApproveReplanning)

	protected.Get("/projects/:id/handover", h.ProjectHandover)
	protected.Get("/projects/:id/handover/options", h.HandoverOptions)
	protected.Post("/projects/:id/handover", h.CreateHandover)
	protected.Get("/handover/:id", h.GetHandover)
	protected.Put("/handover/:id/field-assessment", h.UpdateHandoverFieldAssessment)
	protected.Post("/handover/:id/checklist-items", h.CreateHandoverPhysicalItem)
	protected.Post("/handover/:id/submit", h.SubmitHandover)
	protected.Post("/handover/:id/check", h.CheckHandover)
	protected.Post("/handover/:id/complete", h.CompleteHandover)
	protected.Put("/handover-checklist-items/:id", h.UpdateHandoverChecklistItem)
	protected.Put("/handover-checklist-items/:id/field-assessment", h.UpdateHandoverItemFieldAssessment)
}
