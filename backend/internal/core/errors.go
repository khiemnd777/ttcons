package core

import (
	"database/sql"
	"errors"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"
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
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" && statusIn(pgErr.ConstraintName,
		"acceptance_pdf_export_available",
		"workflow_form_pdf_export_deletion_unreferenced",
	) {
		return response.Fail(c, fiber.StatusConflict, "CONFLICT", "Không thể xoá hoặc liên kết PDF khi hồ sơ nghiệm thu đầu ra đang tham chiếu tệp này")
	}
	if errors.Is(err, sql.ErrNoRows) {
		return response.Fail(c, fiber.StatusNotFound, "NOT_FOUND", "Không tìm thấy dữ liệu")
	}
	return response.Fail(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Đã xảy ra lỗi hệ thống. Vui lòng thử lại.")
}
