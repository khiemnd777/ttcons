package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type RateLimitPolicy struct {
	Limit  int
	Burst  int
	Window time.Duration
}

type RateLimitPolicies struct {
	Auth    RateLimitPolicy
	General RateLimitPolicy
	Upload  RateLimitPolicy
	Export  RateLimitPolicy
	SSE     RateLimitPolicy
}

type Config struct {
	AppEnv                    string
	Port                      string
	DatabaseURL               string
	RedisURL                  string
	JWTSecret                 string
	FrontendOrigin            string
	MigrationsDir             string
	AutoMigrate               bool
	UploadDir                 string
	MaxUploadBytes            int64
	AuthAccessTokenMinutes    int64
	AuthRefreshTokenHours     int64
	AuthRefreshCookieSecure   bool
	AuthRefreshCookieSameSite string
	TrustedProxies            []string
	HTTPReadTimeout           time.Duration
	HTTPWriteTimeout          time.Duration
	HTTPIdleTimeout           time.Duration
	ShutdownTimeout           time.Duration
	ReadinessTimeout          time.Duration
	DBStatementTimeout        time.Duration
	DBLockTimeout             time.Duration
	DBMaxOpenConns            int
	DBMaxIdleConns            int
	DBConnMaxLifetime         time.Duration
	DBConnMaxIdleTime         time.Duration
	RedisOperationTimeout     time.Duration
	RateLimitRedisMaxInFlight int
	RateLimitEnabled          bool
	RateLimitMode             string
	RateLimitLocalMaxKeys     int
	RateLimitIP               RateLimitPolicies
	RateLimitActor            RateLimitPolicies
	UploadConcurrency         int
	ExportConcurrency         int
	ReportConcurrency         int
	SSEMaxConnections         int
	SSEMaxConnectionsPerIP    int
	SSEMaxConnectionsPerActor int
	SSEVisibilityTimeout      time.Duration
	SSEMaxConnectionLifetime  time.Duration
	SSEBrokerChannel          string
	invalidEnvironment        []string
}

func Load() Config {
	validation := &envValidation{}
	cfg := Config{
		AppEnv:                    strings.ToLower(strings.TrimSpace(getenv("APP_ENV", "development"))),
		Port:                      getenv("PORT", "8080"),
		DatabaseURL:               getenv("DATABASE_URL", "postgres://consflow:consflow@127.0.0.1:5432/consflow?sslmode=disable"),
		RedisURL:                  getenv("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:                 getenv("JWT_SECRET", "consflow-local-dev-secret"),
		FrontendOrigin:            getenv("FRONTEND_ORIGIN", "http://localhost:5173"),
		MigrationsDir:             getenv("MIGRATIONS_DIR", "migrations"),
		AutoMigrate:               getenvBoolTracked("AUTO_MIGRATE", true, validation),
		UploadDir:                 getenv("UPLOAD_DIR", "uploads"),
		MaxUploadBytes:            getenvInt64Tracked("MAX_UPLOAD_BYTES", 10*1024*1024, validation),
		AuthAccessTokenMinutes:    getenvInt64Tracked("AUTH_ACCESS_TOKEN_MINUTES", 15, validation),
		AuthRefreshTokenHours:     getenvInt64Tracked("AUTH_REFRESH_TOKEN_HOURS", 24*7, validation),
		AuthRefreshCookieSecure:   getenvBoolTracked("AUTH_REFRESH_COOKIE_SECURE", false, validation),
		AuthRefreshCookieSameSite: getenv("AUTH_REFRESH_COOKIE_SAME_SITE", "Lax"),
		TrustedProxies:            getenvCSV("TRUSTED_PROXY_CIDRS"),
		HTTPReadTimeout:           getenvDurationTracked("HTTP_READ_TIMEOUT", 15*time.Second, false, validation),
		// SSE responses are intentionally long-lived, so the global write deadline is disabled by default.
		HTTPWriteTimeout:          getenvDurationTracked("HTTP_WRITE_TIMEOUT", 0, true, validation),
		HTTPIdleTimeout:           getenvDurationTracked("HTTP_IDLE_TIMEOUT", 60*time.Second, false, validation),
		ShutdownTimeout:           getenvDurationTracked("SHUTDOWN_TIMEOUT", 15*time.Second, false, validation),
		ReadinessTimeout:          getenvDurationTracked("READINESS_TIMEOUT", 2*time.Second, false, validation),
		DBStatementTimeout:        getenvDurationTracked("DB_STATEMENT_TIMEOUT", 15*time.Second, false, validation),
		DBLockTimeout:             getenvDurationTracked("DB_LOCK_TIMEOUT", 5*time.Second, false, validation),
		DBMaxOpenConns:            getenvIntTracked("DB_MAX_OPEN_CONNS", 20, validation),
		DBMaxIdleConns:            getenvIntTracked("DB_MAX_IDLE_CONNS", 10, validation),
		DBConnMaxLifetime:         getenvDurationTracked("DB_CONN_MAX_LIFETIME", 30*time.Minute, false, validation),
		DBConnMaxIdleTime:         getenvDurationTracked("DB_CONN_MAX_IDLE_TIME", 5*time.Minute, false, validation),
		RedisOperationTimeout:     getenvDurationTracked("REDIS_OPERATION_TIMEOUT", 250*time.Millisecond, false, validation),
		RateLimitRedisMaxInFlight: getenvIntTracked("RATE_LIMIT_REDIS_MAX_IN_FLIGHT", 32, validation),
		RateLimitEnabled:          getenvBoolTracked("RATE_LIMIT_ENABLED", true, validation),
		RateLimitMode:             strings.ToLower(strings.TrimSpace(getenv("RATE_LIMIT_MODE", "enforce"))),
		RateLimitLocalMaxKeys:     getenvIntTracked("RATE_LIMIT_LOCAL_MAX_KEYS", 10000, validation),
		RateLimitIP: RateLimitPolicies{
			Auth:    rateLimitPolicy("RATE_LIMIT_IP_AUTH", 10, 10, validation),
			General: rateLimitPolicy("RATE_LIMIT_IP_GENERAL", 300, 60, validation),
			Upload:  rateLimitPolicy("RATE_LIMIT_IP_UPLOAD", 20, 5, validation),
			Export:  rateLimitPolicy("RATE_LIMIT_IP_EXPORT", 30, 5, validation),
			SSE:     rateLimitPolicy("RATE_LIMIT_IP_SSE", 30, 5, validation),
		},
		RateLimitActor: RateLimitPolicies{
			Auth:    rateLimitPolicy("RATE_LIMIT_ACTOR_AUTH", 30, 10, validation),
			General: rateLimitPolicy("RATE_LIMIT_ACTOR_GENERAL", 600, 120, validation),
			Upload:  rateLimitPolicy("RATE_LIMIT_ACTOR_UPLOAD", 30, 5, validation),
			Export:  rateLimitPolicy("RATE_LIMIT_ACTOR_EXPORT", 60, 10, validation),
			SSE:     rateLimitPolicy("RATE_LIMIT_ACTOR_SSE", 20, 4, validation),
		},
		UploadConcurrency:         getenvIntTracked("UPLOAD_CONCURRENCY", 4, validation),
		ExportConcurrency:         getenvIntTracked("EXPORT_CONCURRENCY", 4, validation),
		ReportConcurrency:         getenvIntTracked("REPORT_CONCURRENCY", 6, validation),
		SSEMaxConnections:         getenvIntTracked("SSE_MAX_CONNECTIONS", 500, validation),
		SSEMaxConnectionsPerIP:    getenvIntTracked("SSE_MAX_CONNECTIONS_PER_IP", 20, validation),
		SSEMaxConnectionsPerActor: getenvIntTracked("SSE_MAX_CONNECTIONS_PER_ACTOR", 2, validation),
		SSEVisibilityTimeout:      getenvDurationTracked("SSE_VISIBILITY_TIMEOUT", 2*time.Second, false, validation),
		SSEMaxConnectionLifetime:  getenvDurationTracked("SSE_MAX_CONNECTION_LIFETIME", 5*time.Minute, false, validation),
		SSEBrokerChannel:          getenv("SSE_BROKER_CHANNEL", "consflow:activity:v1"),
	}
	cfg.invalidEnvironment = validation.invalid
	return cfg
}

func (c Config) Validate() error {
	appEnv := strings.ToLower(strings.TrimSpace(c.AppEnv))
	switch appEnv {
	case "development", "test", "staging", "production":
	default:
		return fmt.Errorf("APP_ENV must be one of development, test, staging, or production")
	}
	if c.RateLimitMode != "observe" && c.RateLimitMode != "enforce" {
		return fmt.Errorf("RATE_LIMIT_MODE must be observe or enforce")
	}
	if c.DBMaxIdleConns > c.DBMaxOpenConns {
		return fmt.Errorf("DB_MAX_IDLE_CONNS must not exceed DB_MAX_OPEN_CONNS")
	}
	for _, proxy := range c.TrustedProxies {
		if net.ParseIP(proxy) == nil {
			if _, _, err := net.ParseCIDR(proxy); err != nil {
				return fmt.Errorf("TRUSTED_PROXY_CIDRS contains invalid IP or CIDR %q", proxy)
			}
		}
	}
	if appEnv == "production" {
		if len(c.invalidEnvironment) > 0 {
			return fmt.Errorf("invalid production environment variables: %s", strings.Join(c.invalidEnvironment, ", "))
		}
		if strings.TrimSpace(c.DatabaseURL) == "" || containsPlaceholder(c.DatabaseURL) {
			return fmt.Errorf("DATABASE_URL must not contain CHANGE_ME in production")
		}
		if strings.TrimSpace(c.RedisURL) == "" || containsPlaceholder(c.RedisURL) {
			return fmt.Errorf("REDIS_URL must not contain CHANGE_ME in production")
		}
		redisURL, err := url.Parse(c.RedisURL)
		if err != nil || (redisURL.Scheme != "redis" && redisURL.Scheme != "rediss") || redisURL.Host == "" {
			return fmt.Errorf("REDIS_URL must be an absolute redis:// or rediss:// URL in production")
		}
		if containsPlaceholder(c.JWTSecret) {
			return fmt.Errorf("JWT_SECRET must not contain CHANGE_ME in production")
		}
		if len([]byte(c.JWTSecret)) < 32 {
			return fmt.Errorf("JWT_SECRET must be at least 32 bytes in production")
		}
		frontendURL, err := url.Parse(c.FrontendOrigin)
		if err != nil || frontendURL.Scheme != "https" || frontendURL.Host == "" {
			return fmt.Errorf("FRONTEND_ORIGIN must be an absolute HTTPS URL in production")
		}
		if len(c.TrustedProxies) == 0 {
			return fmt.Errorf("TRUSTED_PROXY_CIDRS must not be empty in production")
		}
		if !c.AuthRefreshCookieSecure {
			return fmt.Errorf("AUTH_REFRESH_COOKIE_SECURE must be true in production")
		}
		if !c.RateLimitEnabled {
			return fmt.Errorf("RATE_LIMIT_ENABLED must be true in production")
		}
	}
	return nil
}

func containsPlaceholder(value string) bool {
	return strings.Contains(strings.ToUpper(value), "CHANGE_ME")
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

type envValidation struct {
	invalid []string
}

func (v *envValidation) markInvalid(key string) {
	for _, existing := range v.invalid {
		if existing == key {
			return
		}
	}
	v.invalid = append(v.invalid, key)
}

func getenvBoolTracked(key string, fallback bool, validation *envValidation) bool {
	raw, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		validation.markInvalid(key)
		return fallback
	}
	return parsed
}

func getenvInt64Tracked(key string, fallback int64, validation *envValidation) int64 {
	raw, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || parsed <= 0 {
		validation.markInvalid(key)
		return fallback
	}
	return parsed
}

func getenvIntTracked(key string, fallback int, validation *envValidation) int {
	raw, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || parsed <= 0 {
		validation.markInvalid(key)
		return fallback
	}
	return parsed
}

func getenvDurationTracked(key string, fallback time.Duration, allowZero bool, validation *envValidation) time.Duration {
	raw, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || parsed < 0 || (!allowZero && parsed == 0) {
		validation.markInvalid(key)
		return fallback
	}
	return parsed
}

func getenvCSV(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func rateLimitPolicy(prefix string, defaultLimit, defaultBurst int, validation *envValidation) RateLimitPolicy {
	return RateLimitPolicy{
		Limit:  getenvIntTracked(prefix+"_LIMIT", defaultLimit, validation),
		Burst:  getenvIntTracked(prefix+"_BURST", defaultBurst, validation),
		Window: getenvDurationTracked(prefix+"_WINDOW", time.Minute, false, validation),
	}
}
