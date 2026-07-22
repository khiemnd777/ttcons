package observability

import (
	"database/sql"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestMetricsUseRegisteredRouteTemplateWithoutSensitiveValues(t *testing.T) {
	database, err := sql.Open("pgx", "postgres://metrics-user:metrics-password@invalid.local/metrics-db")
	if err != nil {
		t.Fatalf("open test database handle: %v", err)
	}
	defer database.Close()
	database.SetMaxOpenConns(17)

	metrics := New(database)
	app := fiber.New()
	app.Use(metrics.Middleware())
	app.Get("/metrics", metrics.Handler())
	app.Get("/api/projects/:id", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	projectIDs := []string{
		"30000000-0000-0000-0000-000000000020",
		"30000000-0000-0000-0000-000000000021",
	}
	for _, projectID := range projectIDs {
		request := httptest.NewRequest(fiber.MethodGet, "/api/projects/"+projectID, nil)
		request.Header.Set("Authorization", "Bearer header-secret-value")
		request.Header.Set("X-Internal-Secret", "private-header-value")
		response, requestErr := app.Test(request)
		if requestErr != nil {
			t.Fatalf("request project %s: %v", projectID, requestErr)
		}
		if response.StatusCode != fiber.StatusNoContent {
			t.Fatalf("project status=%d, want %d", response.StatusCode, fiber.StatusNoContent)
		}
	}

	body := scrapeMetrics(t, app)
	assertContains(t, body, `consflow_http_requests_total{class="general",method="GET",route="/api/projects/:id",status="204"} 2`)
	assertContains(t, body, "consflow_http_request_duration_seconds_bucket")
	assertContains(t, body, `consflow_http_requests_in_flight{class="general",method="GET"} 0`)
	assertContains(t, body, "go_goroutines")
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		assertContains(t, body, "process_cpu_seconds_total")
	}
	assertContains(t, body, `go_sql_max_open_connections{db_name="primary"} 17`)
	assertContains(t, body, `go_sql_open_connections{db_name="primary"} 0`)
	assertContains(t, body, `go_sql_in_use_connections{db_name="primary"} 0`)
	assertContains(t, body, `go_sql_idle_connections{db_name="primary"} 0`)
	assertContains(t, body, `go_sql_wait_count_total{db_name="primary"} 0`)
	assertContains(t, body, `go_sql_wait_duration_seconds_total{db_name="primary"} 0`)

	for _, forbidden := range []string{
		projectIDs[0],
		projectIDs[1],
		"header-secret-value",
		"private-header-value",
		"metrics-password",
		"invalid.local",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("metrics exposition contains forbidden value %q", forbidden)
		}
	}
}

func TestMetricsErrorHandlerRunsOnceAndRecordsHandledStatus(t *testing.T) {
	handled := 0
	metrics := New(nil)
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(func(c *fiber.Ctx, err error) error {
		handled++
		return c.Status(fiber.StatusTeapot).SendString("handled")
	})})
	app.Use(metrics.Middleware())
	app.Get("/metrics", metrics.Handler())
	app.Get("/api/failure/:id", func(c *fiber.Ctx) error {
		return fiber.ErrBadRequest
	})

	response, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/api/failure/30000000-0000-0000-0000-000000000020", nil))
	if err != nil {
		t.Fatalf("request failure route: %v", err)
	}
	if response.StatusCode != fiber.StatusTeapot {
		t.Fatalf("failure status=%d, want %d", response.StatusCode, fiber.StatusTeapot)
	}
	if handled != 1 {
		t.Fatalf("error handler calls=%d, want 1", handled)
	}

	body := scrapeMetrics(t, app)
	assertContains(t, body, `consflow_http_requests_total{class="general",method="GET",route="/api/failure/:id",status="418"} 1`)
	if strings.Contains(body, "30000000-0000-0000-0000-000000000020") {
		t.Fatal("metrics exposition contains raw failure-route UUID")
	}
}

func TestMetricsEndpointIsNotRecorded(t *testing.T) {
	metrics := New(nil)
	app := fiber.New()
	app.Use(metrics.Middleware())
	app.Get("/metrics", metrics.Handler())

	first := scrapeMetrics(t, app)
	second := scrapeMetrics(t, app)
	if strings.Contains(first, `route="/metrics"`) || strings.Contains(second, `route="/metrics"`) {
		t.Fatal("metrics endpoint must not record itself")
	}
}

func TestProductionNginxExplicitlyDeniesMetricsProxy(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "frontend", "nginx.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read production nginx config: %v", err)
	}
	blockPattern := regexp.MustCompile(`(?s)location\s*=\s*/metrics\s*\{([^}]*)\}`)
	match := blockPattern.FindSubmatch(content)
	if len(match) != 2 {
		t.Fatal("production Nginx must contain an explicit exact-match /metrics deny block")
	}
	block := string(match[1])
	if strings.Contains(block, "proxy_pass") {
		t.Fatal("production Nginx /metrics block must not proxy to backend")
	}
	if !strings.Contains(block, "return 404") {
		t.Fatal("production Nginx /metrics block must return 404")
	}
}

func scrapeMetrics(t *testing.T, app *fiber.App) string {
	t.Helper()
	response, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/metrics", nil))
	if err != nil {
		t.Fatalf("scrape metrics: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("metrics status=%d, want %d", response.StatusCode, fiber.StatusOK)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read metrics response: %v", err)
	}
	return string(body)
}

func assertContains(t *testing.T, body, expected string) {
	t.Helper()
	if !strings.Contains(body, expected) {
		t.Fatalf("metrics response missing %q", expected)
	}
}
