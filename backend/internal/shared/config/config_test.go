package config

import (
	"strings"
	"testing"
)

func TestConfigValidateTrustedProxiesAndMode(t *testing.T) {
	cfg := Config{AppEnv: "development", RateLimitMode: "enforce", DBMaxOpenConns: 20, DBMaxIdleConns: 10}
	cfg.TrustedProxies = []string{"10.0.0.0/8", "127.0.0.1"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	cfg.TrustedProxies = []string{"not-a-network"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("invalid trusted proxy accepted")
	}
	cfg.TrustedProxies = nil
	cfg.RateLimitMode = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Fatal("invalid rate limit mode accepted")
	}
}

func TestConfigValidateRejectsIdlePoolLargerThanOpenPool(t *testing.T) {
	cfg := Config{AppEnv: "development", RateLimitMode: "enforce", DBMaxOpenConns: 5, DBMaxIdleConns: 6}
	if err := cfg.Validate(); err == nil {
		t.Fatal("idle connection pool larger than open pool was accepted")
	}
}

func TestLoadDefaultsToDevelopment(t *testing.T) {
	t.Setenv("APP_ENV", "")
	if cfg := Load(); cfg.AppEnv != "development" {
		t.Fatalf("APP_ENV default=%q, want development", cfg.AppEnv)
	}
}

func TestLoadNormalizesAndValidateRejectsUnknownAppEnv(t *testing.T) {
	t.Setenv("APP_ENV", "  PRODUCTION  ")
	if cfg := Load(); cfg.AppEnv != "production" {
		t.Fatalf("normalized APP_ENV=%q, want production", cfg.AppEnv)
	}
	cfg := Config{AppEnv: "prod", RateLimitMode: "enforce", DBMaxOpenConns: 20, DBMaxIdleConns: 10}
	if err := cfg.Validate(); err == nil {
		t.Fatal("unknown APP_ENV was accepted")
	}
}

func TestProductionConfigValidationFailsClosed(t *testing.T) {
	valid := Config{
		AppEnv:                  "production",
		DatabaseURL:             "postgres://consflow:strong-password@postgres:5432/consflow?sslmode=disable",
		RedisURL:                "redis://:strong-password@redis:6379/0",
		JWTSecret:               "0123456789abcdef0123456789abcdef",
		FrontendOrigin:          "https://workflow.example.com",
		AuthRefreshCookieSecure: true,
		TrustedProxies:          []string{"172.30.0.0/24"},
		RateLimitEnabled:        true,
		RateLimitMode:           "enforce",
		DBMaxOpenConns:          20,
		DBMaxIdleConns:          10,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid production config rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"database placeholder", func(cfg *Config) { cfg.DatabaseURL = "postgres://consflow:CHANGE_ME@postgres/consflow" }},
		{"redis placeholder", func(cfg *Config) { cfg.RedisURL = "redis://:CHANGE_ME@redis:6379/0" }},
		{"malformed redis url", func(cfg *Config) { cfg.RedisURL = "not-a-url" }},
		{"jwt placeholder", func(cfg *Config) { cfg.JWTSecret = "CHANGE_ME_0123456789abcdef0123456789" }},
		{"short jwt", func(cfg *Config) { cfg.JWTSecret = "too-short" }},
		{"non https origin", func(cfg *Config) { cfg.FrontendOrigin = "http://workflow.example.com" }},
		{"empty trusted proxies", func(cfg *Config) { cfg.TrustedProxies = nil }},
		{"insecure refresh cookie", func(cfg *Config) { cfg.AuthRefreshCookieSecure = false }},
		{"disabled limiter", func(cfg *Config) { cfg.RateLimitEnabled = false }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := valid
			test.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("unsafe production config accepted")
			}
		})
	}
}

func TestProductionLoadRejectsMalformedOrNonPositiveRuntimeEnvironment(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{key: "HTTP_READ_TIMEOUT", value: "not-a-duration"},
		{key: "DB_MAX_OPEN_CONNS", value: "0"},
		{key: "REDIS_OPERATION_TIMEOUT", value: "-1s"},
		{key: "RATE_LIMIT_IP_AUTH_LIMIT", value: "0"},
		{key: "REPORT_CONCURRENCY", value: "not-a-number"},
		{key: "SSE_MAX_CONNECTION_LIFETIME", value: "0s"},
		{key: "RATE_LIMIT_ENABLED", value: "not-a-bool"},
	}
	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			setValidProductionEnvironment(t)
			t.Setenv(test.key, test.value)
			cfg := Load()
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("production accepted %s=%q", test.key, test.value)
			}
			if !strings.Contains(err.Error(), test.key) {
				t.Fatalf("validation error %q does not identify %s", err, test.key)
			}
		})
	}
}

func TestDevelopmentLoadFallsBackForMalformedRuntimeEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("HTTP_READ_TIMEOUT", "not-a-duration")
	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development config rejected fallback value: %v", err)
	}
	if cfg.HTTPReadTimeout <= 0 {
		t.Fatalf("development fallback HTTP read timeout=%v, want positive default", cfg.HTTPReadTimeout)
	}
}

func setValidProductionEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://consflow:strong-password@postgres:5432/consflow?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://:strong-password@redis:6379/0")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("FRONTEND_ORIGIN", "https://workflow.example.com")
	t.Setenv("TRUSTED_PROXY_CIDRS", "172.30.0.0/24")
	t.Setenv("AUTH_REFRESH_COOKIE_SECURE", "true")
	t.Setenv("RATE_LIMIT_ENABLED", "true")
}
