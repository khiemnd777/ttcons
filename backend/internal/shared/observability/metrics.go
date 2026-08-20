package observability

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"consflow/backend/internal/shared/eventlog"
	consflowmiddleware "consflow/backend/internal/shared/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	unmatchedRoute         = "__unmatched__"
	eventCaptureStartedKey = "event_log_started_at"
	panicValueKey          = "event_log_panic_value"
	panicStackKey          = "event_log_panic_stack"
)

type Metrics struct {
	registry                     *prometheus.Registry
	requestsTotal                *prometheus.CounterVec
	requestDuration              *prometheus.HistogramVec
	requestsInFlight             *prometheus.GaugeVec
	projectExplorerCacheRequests *prometheus.CounterVec
	projectExplorerCacheDuration *prometheus.HistogramVec
	projectExplorerCacheFallback *prometheus.CounterVec
	projectExplorerCacheMismatch prometheus.Counter
}

func New(database *sql.DB) *Metrics {
	registry := prometheus.NewRegistry()
	metrics := &Metrics{
		registry: registry,
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "consflow",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total HTTP requests handled by the backend.",
		}, []string{"method", "route", "status", "class"}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "consflow",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "route", "status", "class"}),
		requestsInFlight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "consflow",
			Subsystem: "http",
			Name:      "requests_in_flight",
			Help:      "HTTP requests currently being handled.",
		}, []string{"method", "class"}),
		projectExplorerCacheRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "consflow",
			Subsystem: "project_explorer_cache",
			Name:      "requests_total",
			Help:      "Project explorer cache item outcomes by bounded layer and outcome.",
		}, []string{"layer", "outcome"}),
		projectExplorerCacheDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "consflow",
			Subsystem: "project_explorer_cache",
			Name:      "operation_duration_seconds",
			Help:      "Project explorer cache operation duration.",
			Buckets:   []float64{0.001, 0.003, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
		}, []string{"operation"}),
		projectExplorerCacheFallback: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "consflow",
			Subsystem: "project_explorer_cache",
			Name:      "fallback_total",
			Help:      "Project explorer database fallbacks by bounded reason.",
		}, []string{"reason"}),
		projectExplorerCacheMismatch: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "consflow",
			Subsystem: "project_explorer_cache",
			Name:      "shadow_mismatch_total",
			Help:      "Stable-revision semantic mismatches observed in shadow mode.",
		}),
	}

	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		metrics.requestsTotal,
		metrics.requestDuration,
		metrics.requestsInFlight,
		metrics.projectExplorerCacheRequests,
		metrics.projectExplorerCacheDuration,
		metrics.projectExplorerCacheFallback,
		metrics.projectExplorerCacheMismatch,
	)
	if database != nil {
		registry.MustRegister(collectors.NewDBStatsCollector(database, "primary"))
	}
	return metrics
}

func (m *Metrics) ObserveProjectExplorerCache(layer, outcome string, count int) {
	if m == nil || count <= 0 {
		return
	}
	m.projectExplorerCacheRequests.WithLabelValues(layer, outcome).Add(float64(count))
}

func (m *Metrics) ObserveProjectExplorerCacheOperation(operation string, elapsed time.Duration) {
	if m == nil {
		return
	}
	m.projectExplorerCacheDuration.WithLabelValues(operation).Observe(elapsed.Seconds())
}

func (m *Metrics) ObserveProjectExplorerCacheFallback(reason string) {
	if m == nil {
		return
	}
	m.projectExplorerCacheFallback.WithLabelValues(reason).Inc()
}

func (m *Metrics) ObserveProjectExplorerCacheMismatch() {
	if m == nil {
		return
	}
	m.projectExplorerCacheMismatch.Inc()
}

func (m *Metrics) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Path() == "/metrics" {
			return c.Next()
		}

		method := normalizedMethod(c.Method())
		requestClass := string(consflowmiddleware.ClassifyRequest(c.Method(), c.Path()))
		inFlight := m.requestsInFlight.WithLabelValues(method, requestClass)
		inFlight.Inc()
		defer inFlight.Dec()

		startedAt := time.Now()
		err := c.Next()
		if err != nil {
			if handlerErr := c.App().ErrorHandler(c, err); handlerErr != nil {
				err = handlerErr
			} else {
				err = nil
			}
		}

		route := routeTemplate(c)
		status := strconv.Itoa(c.Response().StatusCode())
		m.requestsTotal.WithLabelValues(method, route, status, requestClass).Inc()
		m.requestDuration.WithLabelValues(method, route, status, requestClass).Observe(time.Since(startedAt).Seconds())
		return err
	}
}

func (m *Metrics) Handler() fiber.Handler {
	handler := promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
	return adaptor.HTTPHandler(handler)
}

func EventCaptureMiddleware(recorder eventlog.Recorder) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals(eventCaptureStartedKey, time.Now())
		err := c.Next()
		if err == nil && shouldCaptureEvent(c) {
			recordEvent(c, nil, recorder)
		}
		return err
	}
}

func PanicStackTraceHandler(c *fiber.Ctx, recovered any) {
	c.Locals(panicValueKey, fmt.Sprint(recovered))
	c.Locals(panicStackKey, string(debug.Stack()))
}

func ErrorHandler(next fiber.ErrorHandler, recorders ...eventlog.Recorder) fiber.ErrorHandler {
	var recorder eventlog.Recorder
	if len(recorders) > 0 {
		recorder = recorders[0]
	}
	return func(c *fiber.Ctx, err error) error {
		handledErr := next(c, err)
		requestID := "-"
		if value := c.Locals("requestid"); value != nil {
			requestID = safeLogValue(fmt.Sprint(value))
		}
		log.Printf(
			"event=http_error status=%d method=%s route=%s request_id=%s",
			c.Response().StatusCode(),
			normalizedMethod(c.Method()),
			routeTemplate(c),
			requestID,
		)
		if shouldCaptureEvent(c) {
			recordEvent(c, err, recorder)
		}
		return handledErr
	}
}

type responseErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func shouldCaptureEvent(c *fiber.Ctx) bool {
	if c == nil {
		return false
	}
	switch c.Path() {
	case "/metrics", "/api/health", "/api/ready":
		return false
	}
	status := c.Response().StatusCode()
	return status >= fiber.StatusInternalServerError && status <= 599
}

func recordEvent(c *fiber.Ctx, requestErr error, recorder eventlog.Recorder) {
	if recorder == nil {
		return
	}
	status := c.Response().StatusCode()
	category := eventlog.CategoryUnexpected5xx
	if status == fiber.StatusServiceUnavailable {
		category = eventlog.CategoryControlled503
	}
	stackTrace := fmt.Sprint(c.Locals(panicStackKey))
	if stackTrace != "" && stackTrace != "<nil>" {
		category = eventlog.CategoryPanic
	}

	errorCode, publicMessage := responseError(c)
	errorType, internalMessage := eventlog.Diagnostic(requestErr)
	if requestErr == nil {
		internalMessage = publicMessage
	}
	if category == eventlog.CategoryPanic {
		errorType = "panic"
		if panicValue := fmt.Sprint(c.Locals(panicValueKey)); panicValue != "" && panicValue != "<nil>" {
			internalMessage = panicValue
		}
	}

	requestID := localString(c, "requestid")
	entry := eventlog.Entry{
		Category:        category,
		HTTPStatus:      status,
		ErrorCode:       errorCode,
		Method:          normalizedMethod(c.Method()),
		RouteTemplate:   routeTemplate(c),
		RequestClass:    string(consflowmiddleware.ClassifyRequest(c.Method(), c.Path())),
		RequestID:       requestID,
		ActorID:         localString(c, "user_id"),
		ActorEmail:      localString(c, "email"),
		ProjectID:       projectIDFromRoute(c),
		DurationMS:      eventDuration(c),
		ErrorType:       errorType,
		InternalMessage: eventlog.SanitizeMessage(internalMessage),
		StackTrace:      eventlog.SanitizeStack(stackTrace),
	}
	if err := recorder.Record(context.Background(), entry); err != nil {
		log.Printf(
			"event=event_log_fallback category=%s status=%d error_code=%s method=%s route=%s request_id=%s fingerprint=%s persist_error=%q",
			entry.Category,
			status,
			safeLogValue(entry.ErrorCode),
			entry.Method,
			entry.RouteTemplate,
			safeLogValue(requestID),
			eventlog.Fingerprint(entry),
			eventlog.SanitizeMessage(err.Error()),
		)
	}
}

func localString(c *fiber.Ctx, key string) string {
	value := c.Locals(key)
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func responseError(c *fiber.Ctx) (code, message string) {
	var envelope responseErrorEnvelope
	if err := json.Unmarshal(c.Response().Body(), &envelope); err == nil {
		code = strings.TrimSpace(envelope.Error.Code)
		message = strings.TrimSpace(envelope.Error.Message)
	}
	if code == "" {
		code = "INTERNAL_ERROR"
	}
	return code, message
}

func projectIDFromRoute(c *fiber.Ctx) string {
	route := routeTemplate(c)
	const projectPrefix = "/api/projects/:id"
	if route != projectPrefix && !strings.HasPrefix(route, projectPrefix+"/") {
		return ""
	}
	projectID := strings.TrimSpace(c.Params("id"))
	if _, err := uuid.Parse(projectID); err != nil {
		return ""
	}
	return projectID
}

func eventDuration(c *fiber.Ctx) int64 {
	startedAt, ok := c.Locals(eventCaptureStartedKey).(time.Time)
	if !ok || startedAt.IsZero() {
		return 0
	}
	duration := time.Since(startedAt).Milliseconds()
	if duration < 0 {
		return 0
	}
	return duration
}

func routeTemplate(c *fiber.Ctx) string {
	route := c.Route()
	if route == nil || len(route.Handlers) == 0 || route.Path == "" {
		return unmatchedRoute
	}
	return route.Path
}

func normalizedMethod(method string) string {
	switch strings.ToUpper(method) {
	case fiber.MethodGet, fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch,
		fiber.MethodDelete, fiber.MethodHead, fiber.MethodOptions:
		return strings.ToUpper(method)
	default:
		return "OTHER"
	}
}

func safeLogValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	if len(value) > 128 {
		value = value[:128]
	}
	return strings.Map(func(character rune) rune {
		switch {
		case character >= 'a' && character <= 'z':
			return character
		case character >= 'A' && character <= 'Z':
			return character
		case character >= '0' && character <= '9':
			return character
		case strings.ContainsRune("._:-", character):
			return character
		default:
			return '_'
		}
	}, value)
}
