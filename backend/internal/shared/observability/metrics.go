package observability

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	consflowmiddleware "consflow/backend/internal/shared/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const unmatchedRoute = "__unmatched__"

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

func ErrorHandler(next fiber.ErrorHandler) fiber.ErrorHandler {
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
		return handledErr
	}
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
