package core

import (
	"encoding/json"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) ProjectFinanceProjects(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting_handoff.view"}); err != nil {
		return err
	}
	req, err := projectOptionsRequest(c)
	if err != nil {
		return err
	}
	data, err := h.service.ProjectFinanceProjects(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func (h *Handler) ProjectFinanceWorkspace(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"accounting_handoff.view"}); err != nil {
		return err
	}
	data, err := h.service.ProjectFinanceWorkspace(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func (h *Handler) CreateAccountingHandoff(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"accounting_handoff.submit"}); err != nil {
		return err
	}
	var req AccountingHandoffRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return validationError("Dữ liệu bàn giao kế toán không hợp lệ")
	}
	row, err := h.service.CreateAccountingHandoff(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) AccountingHandoffDetail(c *fiber.Ctx) error {
	data, err := h.service.AccountingHandoffDetail(c.Context(), c.Params("id"), c.Query("source_type"), c.Query("source_id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func (h *Handler) AccountingHandoffAction(c *fiber.Ctx) error {
	action := c.Params("action")
	permissions := map[string][]string{
		"handoff":     {"accounting_handoff.submit"},
		"acknowledge": {"accounting_handoff.acknowledge"},
		"return":      {"accounting_handoff.acknowledge"},
	}[action]
	if len(permissions) == 0 {
		return validationError("Hành động bàn giao kế toán không hợp lệ")
	}
	if err := h.requireRecordProjectPermission(c, "accounting_handoffs", c.Params("id"), permissions); err != nil {
		return err
	}
	var req AccountingHandoffActionRequest
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return validationError("Dữ liệu thao tác không hợp lệ")
		}
	}
	row, err := h.service.AccountingHandoffAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}
