package core

import "github.com/gofiber/fiber/v2"

func registerAdminRoutes(protected fiber.Router, h *Handler) {
	protected.Get("/staff", h.ListStaff)
	protected.Post("/staff", h.CreateStaff)
	protected.Post("/staff/imports/preview", h.PreviewStaffAccountImport)
	protected.Post("/staff/imports/execute", h.ExecuteStaffAccountImport)
	protected.Post("/staff/deletions/preview", h.PreviewStaffDeletion)
	protected.Post("/staff/deletions/execute", h.ExecuteStaffDeletion)
	protected.Put("/staff/:id", h.UpdateStaff)
	protected.Get("/roles", h.ListRoles)
	protected.Post("/roles", h.CreateRole)
	protected.Put("/roles/:id", h.UpdateRole)
	protected.Get("/permissions", h.ListPermissions)
	protected.Post("/roles/:id/permissions", h.AssignRolePermissions)
	protected.Post("/users/:id/roles", h.AssignUserRoles)

	protected.Get("/audit-logs", h.AuditLogs)
	protected.Get("/projects/:id/audit-logs", h.ProjectAuditLogs)
	protected.Get("/event-logs", h.EventLogs)
	protected.Get("/event-logs/:id", h.EventLogDetail)
}
