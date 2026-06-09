package core

import (
	"database/sql"
	"errors"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

func ErrorHandler(c *fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return response.Fail(c, fiberErr.Code, "HTTP_ERROR", fiberErr.Message)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return response.Fail(c, fiber.StatusNotFound, "NOT_FOUND", "Không tìm thấy dữ liệu")
	}
	return response.Fail(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
}
