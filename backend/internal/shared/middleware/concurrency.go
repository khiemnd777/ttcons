package middleware

import (
	"strconv"
	"strings"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

type ConcurrencyGuards struct {
	upload chan struct{}
	export chan struct{}
	report chan struct{}
}

func NewConcurrencyGuards(uploadLimit, exportLimit int, reportLimits ...int) *ConcurrencyGuards {
	if uploadLimit < 1 {
		uploadLimit = 1
	}
	if exportLimit < 1 {
		exportLimit = 1
	}
	reportLimit := 6
	if len(reportLimits) > 0 && reportLimits[0] > 0 {
		reportLimit = reportLimits[0]
	}
	return &ConcurrencyGuards{
		upload: make(chan struct{}, uploadLimit),
		export: make(chan struct{}, exportLimit),
		report: make(chan struct{}, reportLimit),
	}
}

func (g *ConcurrencyGuards) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if g == nil {
			return c.Next()
		}
		var semaphore chan struct{}
		class := ClassifyRequest(c.Method(), c.Path())
		switch class {
		case RequestClassUpload:
			semaphore = g.upload
		case RequestClassExport:
			semaphore = g.export
		default:
			if isHeavyReportRequest(c.Method(), c.Path()) {
				semaphore = g.report
			} else {
				return c.Next()
			}
		}
		select {
		case semaphore <- struct{}{}:
			defer func() { <-semaphore }()
			return c.Next()
		default:
			const retrySeconds = 1
			c.Set("Retry-After", strconv.Itoa(retrySeconds))
			return response.Fail(c, fiber.StatusServiceUnavailable, "SERVER_BUSY", "Hệ thống đang xử lý nhiều tác vụ nặng, vui lòng thử lại", map[string]any{
				"retry_after_seconds": retrySeconds,
				"request_class":       concurrencyRequestClass(class, c.Method(), c.Path()),
			})
		}
	}
}

func isHeavyReportRequest(method, path string) bool {
	return method == fiber.MethodGet && strings.HasPrefix(strings.ToLower(path), "/api/reports/") && !strings.HasSuffix(strings.ToLower(path), "/options")
}

func concurrencyRequestClass(class RequestClass, method, path string) string {
	if isHeavyReportRequest(method, path) && class == RequestClassGeneral {
		return "report"
	}
	return string(class)
}
