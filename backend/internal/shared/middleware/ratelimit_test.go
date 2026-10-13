package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

type recordingRedisCmdable struct {
	redis.Cmdable
	script string
	keys   []string
	args   []any
}

type outageRedisCmdable struct {
	redis.Cmdable
	failing  atomic.Bool
	calls    atomic.Int32
	inflight atomic.Int32
	max      atomic.Int32
	started  chan struct{}
	release  chan struct{}
}

func newOutageRedisCmdable(failing bool) *outageRedisCmdable {
	client := &outageRedisCmdable{started: make(chan struct{}, 64), release: make(chan struct{})}
	client.failing.Store(failing)
	return client
}

func (r *outageRedisCmdable) Eval(ctx context.Context, _ string, _ []string, _ ...interface{}) *redis.Cmd {
	r.calls.Add(1)
	cmd := redis.NewCmd(ctx)
	if !r.failing.Load() {
		cmd.SetVal([]any{int64(1), int64(4), int64(0)})
		return cmd
	}
	inflight := r.inflight.Add(1)
	for {
		maximum := r.max.Load()
		if inflight <= maximum || r.max.CompareAndSwap(maximum, inflight) {
			break
		}
	}
	r.started <- struct{}{}
	select {
	case <-r.release:
	case <-ctx.Done():
	}
	r.inflight.Add(-1)
	cmd.SetErr(errors.New("redis unavailable"))
	return cmd
}

func (r *recordingRedisCmdable) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	r.script = script
	r.keys = append([]string(nil), keys...)
	r.args = append([]any(nil), args...)
	cmd := redis.NewCmd(ctx)
	cmd.SetVal([]any{int64(1), int64(4), int64(0)})
	return cmd
}

func TestClassifyRequest(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   RequestClass
	}{
		{fiber.MethodPost, "/api/auth/login", RequestClassAuth},
		{fiber.MethodPost, "/api/work-items/1/evidence/upload", RequestClassUpload},
		{fiber.MethodGet, "/api/reports/projects/export", RequestClassExport},
		{fiber.MethodGet, "/api/projects/imports/template", RequestClassExport},
		{fiber.MethodGet, "/api/workflow-definitions/imports/template", RequestClassExport},
		{fiber.MethodPost, "/api/workflow-form-instances/1/pdf-exports", RequestClassExport},
		{fiber.MethodGet, "/api/projects/1/activity/stream", RequestClassSSE},
		{fiber.MethodGet, "/api/evidence/1/file", RequestClassExport},
		{fiber.MethodGet, "/api/input-invoices/1/file", RequestClassExport},
		{fiber.MethodGet, "/api/output-invoices/1/file", RequestClassExport},
		{fiber.MethodGet, "/api/workflow-form-pdf-exports/1/file", RequestClassExport},
		{fiber.MethodGet, "/api/workflow-form-signatures/1/visual", RequestClassGeneral},
		{fiber.MethodGet, "/api/projects", RequestClassGeneral},
		{fiber.MethodGet, "/api/daily-reports/1/export.xlsx", RequestClassExport},
		{fiber.MethodGet, "/api/work-reports/1/revisions/2/export.xlsx", RequestClassExport},
		{fiber.MethodGet, "/api/projects/1/daily-reports/export.xlsx", RequestClassExport},
		{fiber.MethodGet, "/api/work-reports/export.xlsx", RequestClassExport},
		{fiber.MethodPost, "/api/daily-reports/1/export.xlsx", RequestClassGeneral},
		{fiber.MethodPost, "/api/projects/1/bm-gs/preview", RequestClassGeneral},
		{fiber.MethodGet, "/api/projects/1/bm-gs/preview", RequestClassGeneral},
		{fiber.MethodPost, "/api/projects/1/preview", RequestClassGeneral},
		{fiber.MethodPost, "/api/other/1/bm-gs/preview", RequestClassGeneral},
	}
	for _, test := range tests {
		if got := ClassifyRequest(test.method, test.path); got != test.want {
			t.Fatalf("ClassifyRequest(%q, %q) = %q, want %q", test.method, test.path, got, test.want)
		}
	}
}

func TestRateLimiterUsesBoundedLocalFallback(t *testing.T) {
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	limiter := NewRateLimiter(nil, RateLimiterConfig{
		Enabled:      true,
		Enforce:      true,
		LocalMaxKeys: 2,
		Now:          func() time.Time { return now },
	})
	policy := RateLimitPolicy{Limit: 2, Burst: 2, Window: time.Minute}

	if result := limiter.Allow(context.Background(), "ip", "192.0.2.1", RequestClassGeneral, policy); !result.Allowed || !result.Fallback {
		t.Fatalf("first fallback request = %+v, want allowed local fallback", result)
	}
	if result := limiter.Allow(context.Background(), "ip", "192.0.2.1", RequestClassGeneral, policy); !result.Allowed {
		t.Fatalf("second fallback request = %+v, want allowed", result)
	}
	if result := limiter.Allow(context.Background(), "ip", "192.0.2.1", RequestClassGeneral, policy); result.Allowed || result.RetryAfter <= 0 {
		t.Fatalf("third fallback request = %+v, want denied with retry delay", result)
	}

	limiter.Allow(context.Background(), "ip", "192.0.2.2", RequestClassGeneral, policy)
	thirdIdentity := limiter.Allow(context.Background(), "ip", "192.0.2.3", RequestClassGeneral, policy)
	if got := len(limiter.buckets); got != 2 {
		t.Fatalf("local fallback bucket count = %d, want bounded count 2", got)
	}
	if !thirdIdentity.Allowed {
		t.Fatal("first overflow request unexpectedly rejected")
	}
	if result := limiter.Allow(context.Background(), "ip", "192.0.2.4", RequestClassGeneral, policy); !result.Allowed {
		t.Fatal("second overflow request unexpectedly rejected")
	}
	if result := limiter.Allow(context.Background(), "ip", "192.0.2.5", RequestClassGeneral, policy); result.Allowed {
		t.Fatal("rotating identity received a fresh burst instead of sharing overflow quota")
	}
	if got := len(limiter.buckets); got != 2 {
		t.Fatalf("local fallback bucket count after overflow = %d, want 2", got)
	}
}

func TestRedisRateLimiterUsesRedisServerTime(t *testing.T) {
	redisRecorder := &recordingRedisCmdable{}
	appClock := time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)
	limiter := NewRateLimiter(redisRecorder, RateLimiterConfig{
		Enabled: true,
		Enforce: true,
		Now:     func() time.Time { return appClock },
	})
	result := limiter.Allow(context.Background(), "ip", "192.0.2.1", RequestClassAuth, RateLimitPolicy{Limit: 10, Burst: 5, Window: time.Minute})
	if !result.Allowed || result.Fallback {
		t.Fatalf("redis result=%+v, want allowed without fallback", result)
	}
	if !strings.Contains(redisRecorder.script, "redis.call('TIME')") {
		t.Fatal("redis limiter script does not use Redis TIME")
	}
	if len(redisRecorder.keys) != 1 || len(redisRecorder.args) != 3 {
		t.Fatalf("redis Eval keys=%d args=%d, want one key and capacity/refill/ttl only", len(redisRecorder.keys), len(redisRecorder.args))
	}
	if redisRecorder.args[0] != 5 {
		t.Fatalf("first redis arg=%v, want burst capacity 5 rather than app timestamp", redisRecorder.args[0])
	}
}

func TestRedisCircuitAllowsSingleStartupProbeDuringOutage(t *testing.T) {
	redisClient := newOutageRedisCmdable(true)
	limiter := NewRateLimiter(redisClient, RateLimiterConfig{
		Enabled:          true,
		Enforce:          true,
		RedisTimeout:     time.Second,
		RedisMaxInFlight: 4,
	})
	policy := RateLimitPolicy{Limit: 1000, Burst: 1000, Window: time.Minute}
	var wait sync.WaitGroup
	for index := 0; index < 50; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			limiter.Allow(context.Background(), "ip", fmt.Sprintf("192.0.2.%d", index), RequestClassGeneral, policy)
		}(index)
	}
	<-redisClient.started
	close(redisClient.release)
	wait.Wait()
	if calls := redisClient.calls.Load(); calls != 1 {
		t.Fatalf("startup outage Redis calls=%d, want exactly one half-open probe", calls)
	}
	if maximum := redisClient.max.Load(); maximum != 1 {
		t.Fatalf("startup outage max in-flight=%d, want 1", maximum)
	}
}

func TestRedisCircuitBoundsClosedStateOutageWave(t *testing.T) {
	redisClient := newOutageRedisCmdable(false)
	limiter := NewRateLimiter(redisClient, RateLimiterConfig{
		Enabled:          true,
		Enforce:          true,
		RedisTimeout:     time.Second,
		RedisMaxInFlight: 3,
	})
	policy := RateLimitPolicy{Limit: 1000, Burst: 1000, Window: time.Minute}
	first := limiter.Allow(context.Background(), "ip", "192.0.2.1", RequestClassGeneral, policy)
	if !first.Allowed || first.Fallback {
		t.Fatalf("initial Redis probe=%+v, want healthy distributed result", first)
	}
	redisClient.failing.Store(true)
	var wait sync.WaitGroup
	for index := 0; index < 50; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			limiter.Allow(context.Background(), "ip", fmt.Sprintf("198.51.100.%d", index), RequestClassGeneral, policy)
		}(index)
	}
	<-redisClient.started
	close(redisClient.release)
	wait.Wait()
	if maximum := redisClient.max.Load(); maximum > 3 {
		t.Fatalf("closed-state outage max in-flight=%d, want <=3", maximum)
	}
	if calls := redisClient.calls.Load(); calls > 4 {
		t.Fatalf("closed-state outage total Redis calls=%d, want initial success plus <=3 admitted failures", calls)
	}
}

func TestLocalOverflowQuotasAreIsolatedByScopeAndClass(t *testing.T) {
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	limiter := NewRateLimiter(nil, RateLimiterConfig{
		Enabled:      true,
		Enforce:      true,
		LocalMaxKeys: 1,
		Now:          func() time.Time { return now },
	})
	authPolicy := RateLimitPolicy{Limit: 1, Burst: 1, Window: time.Minute}
	generalPolicy := RateLimitPolicy{Limit: 100, Burst: 10, Window: time.Minute}

	// Fill the primary identity map so every new identity below uses overflow.
	limiter.Allow(context.Background(), "ip", "192.0.2.1", RequestClassGeneral, generalPolicy)
	if result := limiter.Allow(context.Background(), "ip", "192.0.2.2", RequestClassAuth, authPolicy); !result.Allowed {
		t.Fatal("first IP/auth overflow request unexpectedly rejected")
	}
	if result := limiter.Allow(context.Background(), "ip", "192.0.2.3", RequestClassAuth, authPolicy); result.Allowed {
		t.Fatal("IP/auth overflow received a fresh token")
	}
	if result := limiter.Allow(context.Background(), "ip", "192.0.2.4", RequestClassGeneral, generalPolicy); !result.Allowed {
		t.Fatal("IP/general overflow incorrectly shared exhausted auth quota")
	}
	if result := limiter.Allow(context.Background(), "actor", "actor-1", RequestClassAuth, authPolicy); !result.Allowed {
		t.Fatal("actor/auth overflow incorrectly shared exhausted IP/auth quota")
	}
	if got := len(limiter.overflowBuckets); got != 3 {
		t.Fatalf("overflow bucket cardinality=%d, want one each for ip/auth, ip/general, actor/auth", got)
	}
}

func TestRateLimiterReturnsEnvelopeAndRetryAfter(t *testing.T) {
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	limiter := NewRateLimiter(nil, RateLimiterConfig{
		Enabled: true,
		Enforce: true,
		Now:     func() time.Time { return now },
	})
	policies := RateLimitPolicies{
		RequestClassGeneral: {Limit: 1, Burst: 1, Window: time.Minute},
	}
	app := fiber.New()
	app.Use(limiter.LimitIP(policies))
	app.Get("/api/items", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	first, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/api/items", nil))
	if err != nil || first.StatusCode != fiber.StatusOK {
		t.Fatalf("first request status=%v err=%v, want 200", first.StatusCode, err)
	}
	second, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/api/items", nil))
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	if second.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("second request status=%d, want 429", second.StatusCode)
	}
	if second.Header.Get("Retry-After") == "" {
		t.Fatal("second request missing Retry-After")
	}
	defer second.Body.Close()
	var envelope struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(second.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Success || envelope.Error.Code != "RATE_LIMITED" {
		t.Fatalf("response envelope = %+v, want RATE_LIMITED", envelope)
	}
}

func TestRateLimiterObserveModeDoesNotReject(t *testing.T) {
	limiter := NewRateLimiter(nil, RateLimiterConfig{Enabled: true, Enforce: false})
	policies := RateLimitPolicies{RequestClassGeneral: {Limit: 1, Burst: 1, Window: time.Minute}}
	app := fiber.New()
	app.Use(limiter.LimitIP(policies))
	app.Get("/api/items", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	_, _ = app.Test(httptest.NewRequest(fiber.MethodGet, "/api/items", nil))
	response, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/api/items", nil))
	if err != nil {
		t.Fatalf("observe request: %v", err)
	}
	if response.StatusCode != fiber.StatusOK || response.Header.Get("X-RateLimit-Observed") != "true" {
		t.Fatalf("observe response status=%d header=%q", response.StatusCode, response.Header.Get("X-RateLimit-Observed"))
	}
}

func TestConcurrencyGuardRejectsWhenHeavyClassIsFull(t *testing.T) {
	guard := NewConcurrencyGuards(1, 1)
	app := fiber.New()
	app.Use(guard.Middleware())
	started := make(chan struct{})
	release := make(chan struct{})
	app.Post("/api/files/upload", func(c *fiber.Ctx) error {
		close(started)
		<-release
		return c.SendStatus(fiber.StatusOK)
	})

	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		response, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/api/files/upload", nil), -1)
		if err != nil {
			t.Errorf("first upload: %v", err)
			return
		}
		if response.StatusCode != fiber.StatusOK {
			t.Errorf("first upload status=%d, want 200", response.StatusCode)
		}
	}()
	<-started
	response, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/api/files/upload", nil))
	if err != nil {
		t.Fatalf("second upload: %v", err)
	}
	if response.StatusCode != fiber.StatusServiceUnavailable || response.Header.Get("Retry-After") == "" {
		t.Fatalf("second upload status=%d Retry-After=%q, want 503 with retry", response.StatusCode, response.Header.Get("Retry-After"))
	}
	close(release)
	wait.Wait()
}

func TestHeavyReportRequestClassification(t *testing.T) {
	if !isHeavyReportRequest(fiber.MethodGet, "/api/reports/dashboard") {
		t.Fatal("dashboard was not classified as a heavy report read")
	}
	if !isHeavyReportRequest(fiber.MethodGet, "/api/reports/cashflow") {
		t.Fatal("finance report was not classified as a heavy report read")
	}
	if isHeavyReportRequest(fiber.MethodGet, "/api/reports/options") {
		t.Fatal("lightweight report options was classified as a heavy report read")
	}
}

func TestConcurrencyGuardRejectsHeavyReportWhenFull(t *testing.T) {
	guard := NewConcurrencyGuards(1, 1, 1)
	guard.report <- struct{}{}
	defer func() { <-guard.report }()
	app := fiber.New()
	app.Use(guard.Middleware())
	app.Get("/api/reports/dashboard", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	response, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/api/reports/dashboard", nil))
	if err != nil {
		t.Fatalf("dashboard request: %v", err)
	}
	if response.StatusCode != fiber.StatusServiceUnavailable || response.Header.Get("Retry-After") == "" {
		t.Fatalf("dashboard status=%d Retry-After=%q, want 503 with retry", response.StatusCode, response.Header.Get("Retry-After"))
	}
}
