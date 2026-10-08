package core

import "github.com/gofiber/fiber/v2"

func registerFinanceRoutes(protected fiber.Router, h *Handler) {
	protected.Get("/projects/:id/accounting-operations", h.AccountingOperationsWorkspace)
	protected.Post("/projects/:id/accounting-bases", h.CreateAccountingBasis)
	protected.Post("/projects/:id/misa-imports/preview", h.PreviewAccountingMISA)
	protected.Post("/projects/:id/misa-imports/:batch/confirm", h.ConfirmAccountingMISA)
	protected.Get("/projects/:id/misa-imports/:batch/file", h.AccountingMISAFile)
	protected.Post("/projects/:id/misa-allocations", h.AllocateAccountingMISA)
	protected.Post("/projects/:id/misa-allocations/:allocation/revoke", h.RevokeAccountingAllocation)
	protected.Post("/projects/:id/misa-reviews", h.ReviewAccountingMISA)
	protected.Get("/projects/:id/project-finance", h.ProjectFinanceWorkspace)
	protected.Get("/project-finance/projects", h.ProjectFinanceProjects)
	protected.Post("/projects/:id/accounting-handoffs", h.CreateAccountingHandoff)
	protected.Get("/projects/:id/accounting-handoff-detail", h.AccountingHandoffDetail)
	protected.Post("/accounting-handoffs/:id/:action", h.AccountingHandoffAction)
}
