package core

import (
	"strconv"

	"consflow/backend/internal/shared/response"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) requireDeveloperRecovery(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"account_recovery.manage"}); err != nil {
		return err
	}
	allowed, err := h.service.repo.DeveloperRecoveryAllowed(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	if !allowed {
		return forbiddenError("Chỉ Developer hệ thống được xử lý khôi phục mật khẩu")
	}
	return nil
}

func (h *Handler) ListDeveloperRecoveryRequests(c *fiber.Ctx) error {
	if err := h.requireDeveloperRecovery(c); err != nil {
		return err
	}
	page, err := strconv.Atoi(c.Query("page", "0"))
	if err != nil {
		return validationError("Số trang không hợp lệ")
	}
	result, err := h.service.ListDeveloperRecoveryRequests(c.Context(), actorID(c), c.Query("status", "pending"), page)
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) GetDeveloperRecoveryRequest(c *fiber.Ctx) error {
	if err := h.requireDeveloperRecovery(c); err != nil {
		return err
	}
	result, err := h.service.GetDeveloperRecoveryRequest(c.Context(), actorID(c), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) IssueDeveloperRecoveryLink(c *fiber.Ctx) error {
	if err := h.requireDeveloperRecovery(c); err != nil {
		return err
	}
	result, err := h.service.IssueDeveloperRecoveryLink(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	c.Set("Referrer-Policy", "no-referrer")
	return response.OK(c, result, "Đã cấp liên kết tạm")
}

func (h *Handler) RejectDeveloperRecoveryRequest(c *fiber.Ctx) error {
	if err := h.requireDeveloperRecovery(c); err != nil {
		return err
	}
	result, err := h.service.RejectDeveloperRecoveryRequest(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, result, "Đã từ chối yêu cầu")
}
