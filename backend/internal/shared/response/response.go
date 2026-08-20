package response

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type Envelope struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

type ErrorEnvelope struct {
	Success bool        `json:"success"`
	Error   ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func OK(c *fiber.Ctx, data any, message ...string) error {
	msg := "OK"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	return c.JSON(Envelope{Success: true, Data: data, Message: msg})
}

func Created(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusCreated).JSON(Envelope{Success: true, Data: data, Message: "Created"})
}

func Fail(c *fiber.Ctx, status int, code, message string, details ...any) error {
	var d any
	if len(details) > 0 {
		d = details[0]
	}
	if status >= fiber.StatusInternalServerError && status <= 599 {
		d = withRequestID(d, strings.TrimSpace(fmt.Sprint(c.Locals("requestid"))))
	}
	return c.Status(status).JSON(ErrorEnvelope{
		Success: false,
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: d,
		},
	})
}

func withRequestID(details any, requestID string) any {
	if requestID == "" || requestID == "<nil>" {
		return details
	}
	value := reflect.ValueOf(details)
	if value.IsValid() && value.Kind() == reflect.Map && value.Type().Key().Kind() == reflect.String {
		copy := make(map[string]any, value.Len()+1)
		iterator := value.MapRange()
		for iterator.Next() {
			copy[iterator.Key().String()] = iterator.Value().Interface()
		}
		copy["request_id"] = requestID
		return copy
	}
	if details == nil {
		return map[string]any{"request_id": requestID}
	}
	return map[string]any{"request_id": requestID, "context": details}
}
