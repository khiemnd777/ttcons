package core

import "github.com/gofiber/fiber/v2"

func registerProcurementRoutes(protected fiber.Router, h *Handler) {
	protected.Get("/procurement-requests", h.ListProcurement)
	protected.Post("/procurement-requests", h.CreateProcurement)
	protected.Get("/procurement-requests/:id", h.GetProcurement)
	protected.Put("/procurement-requests/:id", h.UpdateProcurement)
	protected.Post("/procurement-requests/:id/submit", h.SubmitProcurement)
	protected.Post("/procurement-requests/:id/approve", h.ApproveProcurement)
	protected.Post("/procurement-requests/:id/order", h.OrderProcurement)
	protected.Post("/procurement-requests/:id/receive", h.ReceiveProcurement)
}
