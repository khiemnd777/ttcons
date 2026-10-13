package core

import (
	"consflow/backend/internal/shared/response"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) UpdateHandoverFieldAssessment(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "handover_records", c.Params("id"), []string{"handover.verify"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.UpdateHandoverFieldAssessment(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) UpdateHandoverItemFieldAssessment(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "handover_checklist_items", c.Params("id"), []string{"handover.verify"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.UpdateHandoverItemFieldAssessment(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CreateHandoverPhysicalItem(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "handover_records", c.Params("id"), []string{"handover.verify"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.CreateHandoverPhysicalItem(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}
