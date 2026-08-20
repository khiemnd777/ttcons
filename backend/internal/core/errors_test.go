package core

import (
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

func TestUnexpectedErrorResponseIsGenericAndReturnsRequestID(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	app.Use(requestid.New())
	app.Get("/api/failure", func(c *fiber.Ctx) error {
		return errors.New("password=internal-secret postgres://user:pass@db/app")
	})
	request := httptest.NewRequest(fiber.MethodGet, "/api/failure", nil)
	request.Header.Set("X-Request-ID", "event-log-test-request")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	text := string(body)
	if response.StatusCode != fiber.StatusInternalServerError || !strings.Contains(text, "Đã xảy ra lỗi hệ thống") {
		t.Fatalf("unexpected response status=%d body=%s", response.StatusCode, text)
	}
	for _, forbidden := range []string{"internal-secret", "user:pass"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"request_id":"event-log-test-request"`) {
		t.Fatalf("response missing request ID: %s", text)
	}
}

func TestServerErrorPreservesNamedDetailsAndAddsRequestID(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	app.Use(requestid.New())
	app.Get("/api/busy", func(c *fiber.Ctx) error {
		return StatusError{
			Status:  fiber.StatusServiceUnavailable,
			Code:    "SERVER_BUSY",
			Message: "Hệ thống đang bận",
			Details: Record{"retryable": true},
		}
	})
	request := httptest.NewRequest(fiber.MethodGet, "/api/busy", nil)
	request.Header.Set("X-Request-ID", "controlled-503-request")
	result, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if result.StatusCode != fiber.StatusServiceUnavailable || !strings.Contains(text, `"retryable":true`) || !strings.Contains(text, `"request_id":"controlled-503-request"`) {
		t.Fatalf("named details were not preserved: status=%d body=%s", result.StatusCode, text)
	}
}
