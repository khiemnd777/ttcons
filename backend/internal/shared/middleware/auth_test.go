package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type countingSessionValidator struct {
	calls  atomic.Int32
	active bool
}

func (v *countingSessionValidator) AuthSessionActive(context.Context, string, string, string) (bool, error) {
	v.calls.Add(1)
	return v.active, nil
}

func signedAuthToken(t *testing.T, secret string) string {
	t.Helper()
	claims := Claims{
		UserID:    "10000000-0000-0000-0000-000000000001",
		Email:     "user@example.com",
		SessionID: "20000000-0000-0000-0000-000000000001",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "token-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func authenticatedRequest(method, target, token string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	return request
}

func TestActorRateLimitRejectsBeforeSessionDatabaseValidation(t *testing.T) {
	const secret = "test-secret"
	validator := &countingSessionValidator{active: true}
	limiter := NewRateLimiter(nil, RateLimiterConfig{Enabled: true, Enforce: true})
	policies := RateLimitPolicies{RequestClassGeneral: {Limit: 1, Burst: 1, Window: time.Minute}}
	app := fiber.New()
	app.Use(AuthClaimsRequired(secret))
	app.Use(limiter.LimitActor(policies))
	app.Use(AuthSessionRequired(validator))
	app.Get("/protected", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	token := signedAuthToken(t, secret)

	first, err := app.Test(authenticatedRequest(fiber.MethodGet, "/protected", token))
	if err != nil || first.StatusCode != fiber.StatusOK {
		t.Fatalf("first request status=%v err=%v", first.StatusCode, err)
	}
	second, err := app.Test(authenticatedRequest(fiber.MethodGet, "/protected", token))
	if err != nil || second.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("second request status=%v err=%v, want 429", second.StatusCode, err)
	}
	if calls := validator.calls.Load(); calls != 1 {
		t.Fatalf("session validator calls=%d, want only the non-rate-limited request", calls)
	}
}

func TestInvalidJWTDoesNotReachActorOrSessionMiddleware(t *testing.T) {
	validator := &countingSessionValidator{active: true}
	var actorCalls atomic.Int32
	app := fiber.New()
	app.Use(AuthClaimsRequired("test-secret"))
	app.Use(func(c *fiber.Ctx) error {
		actorCalls.Add(1)
		return c.Next()
	})
	app.Use(AuthSessionRequired(validator))
	app.Get("/protected", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	response, err := app.Test(authenticatedRequest(fiber.MethodGet, "/protected", "not-a-valid-jwt"))
	if err != nil || response.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("invalid JWT status=%v err=%v, want 401", response.StatusCode, err)
	}
	if actorCalls.Load() != 0 || validator.calls.Load() != 0 {
		t.Fatalf("invalid JWT reached actor=%d session=%d", actorCalls.Load(), validator.calls.Load())
	}
}

func TestMissingAuthorizationDoesNotReachActorSessionOrHandler(t *testing.T) {
	validator := &countingSessionValidator{active: true}
	var actorCalls atomic.Int32
	var handlerCalls atomic.Int32
	app := fiber.New()
	app.Use(AuthClaimsRequired("test-secret"))
	app.Use(func(c *fiber.Ctx) error {
		actorCalls.Add(1)
		return c.Next()
	})
	app.Use(AuthSessionRequired(validator))
	app.Get("/protected", func(c *fiber.Ctx) error {
		handlerCalls.Add(1)
		return c.SendStatus(fiber.StatusOK)
	})

	response, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/protected", nil))
	if err != nil || response.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("missing authorization status=%v err=%v, want 401", response.StatusCode, err)
	}
	if actorCalls.Load() != 0 || validator.calls.Load() != 0 || handlerCalls.Load() != 0 {
		t.Fatalf("missing authorization reached actor=%d session=%d handler=%d", actorCalls.Load(), validator.calls.Load(), handlerCalls.Load())
	}
}

func TestInactiveSessionStopsProtectedHandler(t *testing.T) {
	const secret = "test-secret"
	validator := &countingSessionValidator{active: false}
	var handlerCalls atomic.Int32
	app := fiber.New()
	app.Use(AuthClaimsRequired(secret))
	app.Use(AuthSessionRequired(validator))
	app.Get("/protected", func(c *fiber.Ctx) error {
		handlerCalls.Add(1)
		return c.SendStatus(fiber.StatusOK)
	})

	response, err := app.Test(authenticatedRequest(fiber.MethodGet, "/protected", signedAuthToken(t, secret)))
	if err != nil || response.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("inactive session status=%v err=%v, want 401", response.StatusCode, err)
	}
	if validator.calls.Load() != 1 || handlerCalls.Load() != 0 {
		t.Fatalf("inactive session calls=%d handler=%d, want session=1 handler=0", validator.calls.Load(), handlerCalls.Load())
	}
}

func TestAuthRequiredCompatibilityStillValidatesSession(t *testing.T) {
	const secret = "test-secret"
	validator := &countingSessionValidator{active: true}
	app := fiber.New()
	app.Use(AuthRequired(secret, validator))
	app.Get("/protected", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	response, err := app.Test(authenticatedRequest(fiber.MethodGet, "/protected", signedAuthToken(t, secret)))
	if err != nil || response.StatusCode != fiber.StatusOK || validator.calls.Load() != 1 {
		t.Fatalf("compatibility auth status=%v err=%v session_calls=%d", response.StatusCode, err, validator.calls.Load())
	}
}
