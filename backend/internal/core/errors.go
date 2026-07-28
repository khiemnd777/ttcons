package core

import (
	"database/sql"
	"errors"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

type StatusError struct {
	Status  int
	Code    string
	Message string
	Details any
}

func (e StatusError) Error() string {
	return e.Message
}

func validationError(message string, details ...any) StatusError {
	err := StatusError{Status: fiber.StatusBadRequest, Code: "VALIDATION_ERROR", Message: message}
	if len(details) > 0 {
		err.Details = details[0]
	}
	return err
}

func forbiddenError(message string, details ...any) StatusError {
	err := StatusError{Status: fiber.StatusForbidden, Code: "FORBIDDEN", Message: message}
	if len(details) > 0 {
		err.Details = details[0]
	}
	return err
}

func conflictError(message string, details ...any) StatusError {
	err := StatusError{Status: fiber.StatusConflict, Code: "CONFLICT", Message: message}
	if len(details) > 0 {
		err.Details = details[0]
	}
	return err
}

func ErrorHandler(c *fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}
	var statusErr StatusError
	if errors.As(err, &statusErr) {
		return response.Fail(c, statusErr.Status, statusErr.Code, statusErr.Message, statusErr.Details)
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
