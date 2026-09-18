package core

import (
	"consflow/backend/internal/shared/response"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) AccountingOperationsWorkspace(c *fiber.Ctx) error {
	v, e := h.service.AccountingOperationsWorkspace(c.Context(), c.Params("id"), actorID(c))
	if e != nil {
		return e
	}
	return response.OK(c, v)
}
func (h *Handler) AccountingRequestQueue(c *fiber.Ctx) error {
	v, e := h.service.AccountingRequestQueue(c.Context(), actorID(c))
	if e != nil {
		return e
	}
	return response.OK(c, v)
}
func accountingBody(c *fiber.Ctx, v any) error {
	if e := json.Unmarshal(c.Body(), v); e != nil {
		return validationError("Dữ liệu hồ sơ không hợp lệ")
	}
	return nil
}
func (h *Handler) CreateAccountingBasis(c *fiber.Ctx) error {
	var in AccountingBasisInput
	if e := accountingBody(c, &in); e != nil {
		return e
	}
	v, e := h.service.CreateAccountingBasis(c.Context(), c.Params("id"), in, actorID(c), actorName(c))
	if e != nil {
		return e
	}
	return response.Created(c, v)
}
func (h *Handler) SaveAccountingRequest(c *fiber.Ctx) error {
	var in AccountingRequestInput
	if e := accountingBody(c, &in); e != nil {
		return e
	}
	v, e := h.service.SaveAccountingRequest(c.Context(), c.Params("id"), in, actorID(c), actorName(c))
	if e != nil {
		return e
	}
	return response.OK(c, v)
}
func (h *Handler) AccountingRequestAction(c *fiber.Ctx) error {
	var in AccountingRequestActionInput
	if e := accountingBody(c, &in); e != nil {
		return e
	}
	v, e := h.service.AccountingRequestAction(c.Context(), c.Params("id"), c.Params("request"), c.Params("action"), in, actorID(c), actorName(c))
	if e != nil {
		return e
	}
	return response.OK(c, v)
}
func (h *Handler) PreviewAccountingMISA(c *fiber.Ctx) error {
	var in AccountingMISAImportInput
	if e := accountingBody(c, &in); e != nil {
		return e
	}
	v, e := h.service.PreviewAccountingMISA(c.Context(), c.Params("id"), in, actorID(c))
	if e != nil {
		return e
	}
	return response.OK(c, v)
}
func (h *Handler) ConfirmAccountingMISA(c *fiber.Ctx) error {
	v, e := h.service.ConfirmAccountingMISA(c.Context(), c.Params("id"), c.Params("batch"), actorID(c), actorName(c))
	if e != nil {
		return e
	}
	return response.OK(c, v)
}
func (h *Handler) AllocateAccountingMISA(c *fiber.Ctx) error {
	var in AccountingMISAAllocationInput
	if e := accountingBody(c, &in); e != nil {
		return e
	}
	v, e := h.service.AllocateAccountingMISA(c.Context(), c.Params("id"), in, actorID(c), actorName(c))
	if e != nil {
		return e
	}
	return response.Created(c, v)
}
func (h *Handler) RevokeAccountingAllocation(c *fiber.Ctx) error {
	var in AccountingRequestActionInput
	if e := accountingBody(c, &in); e != nil {
		return e
	}
	v, e := h.service.RevokeAccountingAllocation(c.Context(), c.Params("id"), c.Params("allocation"), in.Reason, actorID(c), actorName(c))
	if e != nil {
		return e
	}
	return response.OK(c, v)
}
func (h *Handler) ReviewAccountingMISA(c *fiber.Ctx) error {
	var in AccountingMISAAllocationInput
	if e := accountingBody(c, &in); e != nil {
		return e
	}
	v, e := h.service.ReviewAccountingMISA(c.Context(), c.Params("id"), in, actorID(c), actorName(c))
	if e != nil {
		return e
	}
	return response.OK(c, v)
}
func (h *Handler) AccountingMISAFile(c *fiber.Ctx) error {
	v, e := h.service.AccountingMISAFile(c.Context(), c.Params("id"), c.Params("batch"), actorID(c))
	if e != nil {
		return e
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="misa-source.csv"`)
	c.Set("X-Content-Type-Options", "nosniff")
	return c.SendString(v)
}
