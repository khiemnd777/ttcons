package core

import (
	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) EventLogs(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"event_log.view"}); err != nil {
		return err
	}
	status, err := optionalQueryInt(c, "http_status")
	if err != nil {
		return err
	}
	page, err := optionalQueryInt(c, "page")
	if err != nil {
		return err
	}
	pageSize, err := optionalQueryInt(c, "page_size")
	if err != nil {
		return err
	}
	data, err := h.service.EventLogs(c.Context(), EventLogFilter{
		Category: c.Query("category"), HTTPStatus: status, Method: c.Query("method"),
		Route: c.Query("route"), ActorEmail: c.Query("actor_email"), ProjectID: c.Query("project_id"),
		DateFrom: c.Query("date_from"), DateTo: c.Query("date_to"), Search: c.Query("search"),
		Page: page, PageSize: pageSize,
	})
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func (h *Handler) EventLogDetail(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"event_log.view"}); err != nil {
		return err
	}
	data, err := h.service.EventLogDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}
