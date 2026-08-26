package core

import (
	"strconv"
	"strings"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) UserNotifications(c *fiber.Ctx) error {
	limit := 0
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			return validationError("limit thông báo không hợp lệ", Record{"limit": rawLimit})
		}
		limit = parsed
	}
	page, err := h.service.UserNotifications(c.Context(), NotificationFilter{
		State: c.Query("state"), EventKey: c.Query("event_key"), Category: c.Query("category"),
		ProjectID: c.Query("project_id"), Search: c.Query("search"), FromDate: c.Query("from_date"), ToDate: c.Query("to_date"),
		Cursor: c.Query("cursor"), Limit: limit,
	}, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, page)
}

func (h *Handler) UserNotificationSummary(c *fiber.Ctx) error {
	summary, err := h.service.UserNotificationSummary(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, summary)
}

func (h *Handler) MarkUserNotificationRead(c *fiber.Ctx) error {
	row, err := h.service.MarkUserNotificationRead(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) MarkAllUserNotificationsRead(c *fiber.Ctx) error {
	row, err := h.service.MarkAllUserNotificationsRead(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ArchiveUserNotification(c *fiber.Ctx) error {
	row, err := h.service.ArchiveUserNotification(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) NotificationCatalog(c *fiber.Ctx) error {
	rows, err := h.service.NotificationCatalog(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) NotificationProjects(c *fiber.Ctx) error {
	rows, err := h.service.NotificationProjects(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) NotificationPreferences(c *fiber.Ctx) error {
	rows, err := h.service.NotificationPreferences(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) SaveNotificationPreference(c *fiber.Ctx) error {
	input := NotificationPreference{}
	if err := c.BodyParser(&input); err != nil {
		return validationError("Dữ liệu cấu hình thông báo không hợp lệ")
	}
	row, err := h.service.SaveNotificationPreference(c.Context(), actorID(c), input)
	if err != nil {
		return err
	}
	return response.OK(c, row)
}
