package core

import (
	"consflow/backend/internal/shared/response"
	"context"
	fiber "github.com/gofiber/fiber/v2"
	"time"
)

type Handler struct {
	service          *Service
	readinessTimeout time.Duration
	readinessCheck   func(context.Context) error
}

const refreshCookieName = "consflow_refresh_token"

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Health(c *fiber.Ctx) error {
	return response.OK(c, Record{"status": "ok", "service": "consflow"})
}

func (h *Handler) ConfigureReadiness(timeout time.Duration, check func(context.Context) error) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	h.readinessTimeout = timeout
	h.readinessCheck = check
}

func (h *Handler) Ready(c *fiber.Ctx) error {
	if h.readinessCheck == nil {
		return response.Fail(c, fiber.StatusServiceUnavailable, "NOT_READY", "Dịch vụ chưa sẵn sàng")
	}
	ctx, cancel := context.WithTimeout(context.Background(), h.readinessTimeout)
	defer cancel()
	if err := h.readinessCheck(ctx); err != nil {
		return response.Fail(c, fiber.StatusServiceUnavailable, "NOT_READY", "Cơ sở dữ liệu chưa sẵn sàng")
	}
	return response.OK(c, Record{"status": "ready", "service": "consflow", "database": "ok"})
}
