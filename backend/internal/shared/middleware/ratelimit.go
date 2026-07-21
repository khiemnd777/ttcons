package middleware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

type RequestClass string

const (
	RequestClassAuth    RequestClass = "auth"
	RequestClassGeneral RequestClass = "general"
	RequestClassUpload  RequestClass = "upload"
	RequestClassExport  RequestClass = "export"
	RequestClassSSE     RequestClass = "sse"
)

type RateLimitPolicy struct {
	Limit  int
	Burst  int
	Window time.Duration
}

type RateLimitPolicies map[RequestClass]RateLimitPolicy

type RateLimiterConfig struct {
	Enabled          bool
	Enforce          bool
	RedisTimeout     time.Duration
	RedisMaxInFlight int
	LocalMaxKeys     int
	Now              func() time.Time
}

type RateLimitResult struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
	Fallback   bool
}

type localBucket struct {
	tokens  float64
	updated time.Time
}

type RateLimiter struct {
	redis  redis.Cmdable
	config RateLimiterConfig

	mu               sync.Mutex
	buckets          map[string]localBucket
	overflowBuckets  map[string]localBucket
	redisStateMu     sync.Mutex
	redisCircuit     atomic.Int32
	redisRetryAt     atomic.Int64
	redisUnavailable atomic.Bool
	redisAdmission   chan struct{}
}

const (
	redisCircuitClosed int32 = iota
	redisCircuitOpen
	redisCircuitHalfOpen
)

const redisTokenBucketScript = `
local values = redis.call('HMGET', KEYS[1], 'tokens', 'updated')
local tokens = tonumber(values[1])
local updated = tonumber(values[2])
local redis_time = redis.call('TIME')
local now = tonumber(redis_time[1]) * 1000 + math.floor(tonumber(redis_time[2]) / 1000)
local capacity = tonumber(ARGV[1])
local refill = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
if tokens == nil then tokens = capacity end
if updated == nil then updated = now end
local elapsed = math.max(0, now - updated)
tokens = math.min(capacity, tokens + elapsed * refill)
local allowed = 0
local retry = 0
if tokens >= 1 then
  allowed = 1
  tokens = tokens - 1
else
  retry = math.ceil((1 - tokens) / refill)
end
redis.call('HSET', KEYS[1], 'tokens', tokens, 'updated', now)
redis.call('PEXPIRE', KEYS[1], ttl)
return {allowed, math.floor(tokens), retry}
`

func NewRateLimiter(redisClient redis.Cmdable, cfg RateLimiterConfig) *RateLimiter {
	if cfg.RedisTimeout <= 0 {
		cfg.RedisTimeout = 250 * time.Millisecond
	}
	if cfg.LocalMaxKeys <= 0 {
		cfg.LocalMaxKeys = 10000
	}
	if cfg.RedisMaxInFlight <= 0 {
		cfg.RedisMaxInFlight = 32
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	limiter := &RateLimiter{
		redis:           redisClient,
		config:          cfg,
		buckets:         make(map[string]localBucket),
		overflowBuckets: make(map[string]localBucket, 10),
		redisAdmission:  make(chan struct{}, cfg.RedisMaxInFlight),
	}
	if redisClient != nil {
		// Startup is half-open: one request probes Redis while concurrent requests use local fallback.
		limiter.redisCircuit.Store(redisCircuitOpen)
	}
	return limiter
}

func ClassifyRequest(method, path string) RequestClass {
	path = strings.ToLower(path)
	if strings.HasSuffix(path, "/stream") {
		return RequestClassSSE
	}
	if strings.HasPrefix(path, "/api/auth/") {
		return RequestClassAuth
	}
	if method == fiber.MethodPost && strings.Contains(path, "/upload") {
		return RequestClassUpload
	}
	if isHeavyFileDownloadRequest(method, path) {
		return RequestClassExport
	}
	if strings.HasSuffix(path, "/export") || strings.HasSuffix(path, "/pdf") || strings.Contains(path, "/pdf-exports") {
		return RequestClassExport
	}
	return RequestClassGeneral
}

func isHeavyFileDownloadRequest(method, path string) bool {
	if method != fiber.MethodGet || !strings.HasSuffix(path, "/file") {
		return false
	}
	for _, prefix := range []string{
		"/api/evidence/",
		"/api/input-invoices/",
		"/api/output-invoices/",
		"/api/workflow-form-pdf-exports/",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func (l *RateLimiter) LimitIP(policies RateLimitPolicies) fiber.Handler {
	return l.limit("ip", policies, func(c *fiber.Ctx) string { return c.IP() })
}

func (l *RateLimiter) LimitActor(policies RateLimitPolicies) fiber.Handler {
	return l.limit("actor", policies, func(c *fiber.Ctx) string { return fmt.Sprint(c.Locals("user_id")) })
}

func (l *RateLimiter) limit(scope string, policies RateLimitPolicies, identity func(*fiber.Ctx) string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if l == nil || !l.config.Enabled {
			return c.Next()
		}
		class := ClassifyRequest(c.Method(), c.Path())
		policy, ok := policies[class]
		if !ok {
			policy = policies[RequestClassGeneral]
		}
		result := l.Allow(c.Context(), scope, identity(c), class, policy)
		c.Set("X-RateLimit-Limit", strconv.Itoa(policy.Limit))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
		if result.Fallback {
			c.Set("X-RateLimit-Source", "local-fallback")
		}
		if result.Allowed {
			return c.Next()
		}
		retrySeconds := retryAfterSeconds(result.RetryAfter)
		c.Set("Retry-After", strconv.Itoa(retrySeconds))
		if !l.config.Enforce {
			c.Set("X-RateLimit-Observed", "true")
			return c.Next()
		}
		return response.Fail(c, fiber.StatusTooManyRequests, "RATE_LIMITED", "Quá nhiều yêu cầu, vui lòng thử lại sau", map[string]any{
			"retry_after_seconds": retrySeconds,
			"request_class":       class,
		})
	}
}

func (l *RateLimiter) Allow(ctx context.Context, scope, identity string, class RequestClass, policy RateLimitPolicy) RateLimitResult {
	policy = normalizeRateLimitPolicy(policy)
	key := rateLimitKey(scope, identity, class)
	overflowKey := localOverflowKey(scope, class)
	now := l.config.Now()
	if admitted, probe := l.admitRedis(now); admitted {
		defer func() { <-l.redisAdmission }()
		redisCtx, cancel := context.WithTimeout(ctx, l.config.RedisTimeout)
		result, err := l.allowRedis(redisCtx, key, policy)
		cancel()
		if err == nil {
			l.redisSucceeded(probe)
			return result
		}
		l.redisFailed()
	}
	result := l.allowLocal(key, overflowKey, policy, now)
	result.Fallback = true
	return result
}

func (l *RateLimiter) admitRedis(now time.Time) (bool, bool) {
	if l.redis == nil {
		return false, false
	}
	l.redisStateMu.Lock()
	defer l.redisStateMu.Unlock()
	state := l.redisCircuit.Load()
	switch state {
	case redisCircuitOpen:
		if now.UnixNano() < l.redisRetryAt.Load() || !l.redisCircuit.CompareAndSwap(redisCircuitOpen, redisCircuitHalfOpen) {
			return false, false
		}
		select {
		case l.redisAdmission <- struct{}{}:
			return true, true
		default:
			l.extendRedisRetryAt(now.Add(100 * time.Millisecond).UnixNano())
			l.redisCircuit.Store(redisCircuitOpen)
			return false, false
		}
	case redisCircuitHalfOpen:
		return false, false
	default:
		select {
		case l.redisAdmission <- struct{}{}:
			return true, false
		default:
			return false, false
		}
	}
}

func (l *RateLimiter) redisFailed() {
	l.redisStateMu.Lock()
	defer l.redisStateMu.Unlock()
	l.extendRedisRetryAt(l.config.Now().Add(5 * time.Second).UnixNano())
	// Publish Open only after its retry deadline so readers can never observe an
	// open circuit with the zero/expired deadline from the previous state.
	previous := l.redisCircuit.Swap(redisCircuitOpen)
	if previous != redisCircuitOpen && !l.redisUnavailable.Swap(true) {
		log.Printf("rate limiter redis unavailable; using bounded local fallback")
	}
}

func (l *RateLimiter) extendRedisRetryAt(deadline int64) {
	for {
		current := l.redisRetryAt.Load()
		if current >= deadline || l.redisRetryAt.CompareAndSwap(current, deadline) {
			return
		}
	}
}

func (l *RateLimiter) redisSucceeded(probe bool) {
	if !probe {
		return
	}
	l.redisStateMu.Lock()
	defer l.redisStateMu.Unlock()
	if !l.redisCircuit.CompareAndSwap(redisCircuitHalfOpen, redisCircuitClosed) {
		return
	}
	// The deadline is ignored while closed. Keeping the last value avoids
	// clearing a deadline concurrently published by a failing in-flight call.
	if l.redisUnavailable.Swap(false) {
		log.Printf("rate limiter redis recovered; distributed limiting restored")
	}
}

func (l *RateLimiter) allowRedis(ctx context.Context, key string, policy RateLimitPolicy) (RateLimitResult, error) {
	windowMilliseconds := policy.Window.Milliseconds()
	refillPerMillisecond := float64(policy.Limit) / float64(windowMilliseconds)
	ttlMilliseconds := int64(math.Ceil(float64(policy.Burst)/refillPerMillisecond)) + windowMilliseconds
	value, err := l.redis.Eval(ctx, redisTokenBucketScript, []string{key},
		policy.Burst, strconv.FormatFloat(refillPerMillisecond, 'g', -1, 64), ttlMilliseconds,
	).Result()
	if err != nil {
		return RateLimitResult{}, err
	}
	parts, ok := value.([]any)
	if !ok || len(parts) != 3 {
		return RateLimitResult{}, fmt.Errorf("unexpected redis limiter response")
	}
	allowed, err := limiterInt64(parts[0])
	if err != nil {
		return RateLimitResult{}, err
	}
	remaining, err := limiterInt64(parts[1])
	if err != nil {
		return RateLimitResult{}, err
	}
	retryMilliseconds, err := limiterInt64(parts[2])
	if err != nil {
		return RateLimitResult{}, err
	}
	return RateLimitResult{
		Allowed:    allowed == 1,
		Remaining:  int(maxInt64(0, remaining)),
		RetryAfter: time.Duration(retryMilliseconds) * time.Millisecond,
	}, nil
}

func (l *RateLimiter) allowLocal(key, overflowKey string, policy RateLimitPolicy, now time.Time) RateLimitResult {
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, exists := l.buckets[key]
	useOverflow := false
	if !exists {
		if len(l.buckets) >= l.config.LocalMaxKeys {
			useOverflow = true
			bucket, exists = l.overflowBuckets[overflowKey]
		}
		if !exists {
			bucket = localBucket{tokens: float64(policy.Burst), updated: now}
		}
	}
	elapsed := now.Sub(bucket.updated)
	if elapsed < 0 {
		elapsed = 0
	}
	refillPerNanosecond := float64(policy.Limit) / float64(policy.Window)
	bucket.tokens = math.Min(float64(policy.Burst), bucket.tokens+float64(elapsed)*refillPerNanosecond)
	bucket.updated = now
	result := RateLimitResult{Allowed: bucket.tokens >= 1}
	if result.Allowed {
		bucket.tokens--
	} else {
		result.RetryAfter = time.Duration(math.Ceil((1 - bucket.tokens) / refillPerNanosecond))
	}
	result.Remaining = int(math.Floor(math.Max(0, bucket.tokens)))
	if useOverflow {
		l.overflowBuckets[overflowKey] = bucket
	} else {
		l.buckets[key] = bucket
	}
	return result
}

func localOverflowKey(scope string, class RequestClass) string {
	if scope != "actor" {
		scope = "ip"
	}
	switch class {
	case RequestClassAuth, RequestClassGeneral, RequestClassUpload, RequestClassExport, RequestClassSSE:
	default:
		class = RequestClassGeneral
	}
	return scope + ":" + string(class)
}

func normalizeRateLimitPolicy(policy RateLimitPolicy) RateLimitPolicy {
	if policy.Limit <= 0 {
		policy.Limit = 1
	}
	if policy.Burst <= 0 {
		policy.Burst = policy.Limit
	}
	if policy.Window <= 0 {
		policy.Window = time.Minute
	}
	return policy
}

func rateLimitKey(scope, identity string, class RequestClass) string {
	digest := sha256.Sum256([]byte(identity))
	return fmt.Sprintf("consflow:ratelimit:%s:%s:%x", scope, class, digest[:16])
}

func retryAfterSeconds(value time.Duration) int {
	seconds := int(math.Ceil(value.Seconds()))
	if seconds < 1 {
		return 1
	}
	return seconds
}

func limiterInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected limiter value %T", value)
	}
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
