package core

import (
	"encoding/json"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) SampleDataOptions(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"sample_data.create"}); err != nil {
		return err
	}
	options, err := h.service.SampleDataOptions(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, options)
}

func (h *Handler) CreateSampleDataProject(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"sample_data.create"}); err != nil {
		return err
	}
	var request SampleDataCreateRequest
	if err := json.Unmarshal(c.Body(), &request); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu tạo công trình mẫu không hợp lệ")
	}
	result, err := h.service.CreateSampleDataProject(c.Context(), request, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, result)
}
