package config

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/mail"
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
	AppEnv                         string
	Port                           string
	DatabaseURL                    string
	RedisURL                       string
	RedisCacheURL                  string
	JWTSecret                      string
	SettingsEncryptionKey          string
	FrontendOrigin                 string
	MigrationsDir                  string
	AutoMigrate                    bool
	MigrationLockTimeout           time.Duration
	MigrationTimeout               time.Duration
	StoragePath                    string
	StorageProvider                string
	StorageOperationTimeout        time.Duration
	StorageMigrationTimeout        time.Duration
	StorageS3Bucket                string
	StorageS3Region                string
	StorageS3Endpoint              string
	StorageS3UsePathStyle          bool
	StorageS3AllowInsecureEndpoint bool
	MaxUploadBytes                 int64
	AuthAccessTokenMinutes         int64
	AuthRefreshTokenHours          int64
	AuthRefreshCookieSecure        bool
	AuthRefreshCookieSameSite      string
	AuthRecoveryEnabled            bool
	AuthRecoveryTokenTTL           time.Duration
	AuthRecoveryVerifyTTL          time.Duration
	SMTPHost                       string
	SMTPPort                       int
	SMTPUsername                   string
	SMTPPassword                   string
	SMTPFromAddress                string
	SMTPFromName                   string
	SMTPTLSMode                    string
	SMTPTimeout                    time.Duration
	TrustedProxies                 []string
	HTTPReadTimeout                time.Duration
	HTTPWriteTimeout               time.Duration
	HTTPIdleTimeout                time.Duration
	ShutdownTimeout                time.Duration
	ReadinessTimeout               time.Duration
	DBStatementTimeout             time.Duration
	DBLockTimeout                  time.Duration
	DBMaxOpenConns                 int
	DBMaxIdleConns                 int
	DBConnMaxLifetime              time.Duration
	DBConnMaxIdleTime              time.Duration
	RedisOperationTimeout          time.Duration
	ProjectExplorerCacheMode       string
	ProjectExplorerCacheTimeout    time.Duration
	ProjectExplorerCacheIndexTTL   time.Duration
	ProjectExplorerCacheRowTTL     time.Duration
	ProjectExplorerCacheLockTTL    time.Duration
	ProjectExplorerCacheMaxBytes   int
	RateLimitRedisMaxInFlight      int
	RateLimitEnabled               bool
	RateLimitMode                  string
	RateLimitLocalMaxKeys          int
	RateLimitIP                    RateLimitPolicies
	RateLimitActor                 RateLimitPolicies
	UploadConcurrency              int
	ExportConcurrency              int
	ReportConcurrency              int
	SSEMaxConnections              int
	SSEMaxConnectionsPerIP         int
	SSEMaxConnectionsPerActor      int
	SSEVisibilityTimeout           time.Duration
	SSEMaxConnectionLifetime       time.Duration
	SSEBrokerChannel               string
	invalidEnvironment             []string
	secretEnvironmentErrors        []string
	defaultSecretEnvironment       []string
	accountRecoveryInvalidEnv      []string
	accountRecoverySecretErrors    []string
}

const maxSecretFileBytes = 64 * 1024

func Load() Config {
	validation := &envValidation{}
	databaseURL := getenvSecretTracked("DATABASE_URL", "postgres://consflow:consflow@127.0.0.1:5432/consflow?sslmode=disable", validation)
	redisURL := getenvSecretTracked("REDIS_URL", "redis://localhost:6379/0", validation)
	redisCacheURL := getenvOptionalSecretTracked("REDIS_CACHE_URL", validation)
	jwtSecret := getenvSecretTracked("JWT_SECRET", "consflow-local-dev-secret", validation)
	settingsEncryptionKey := getenvOptionalSecretTracked("SETTINGS_ENCRYPTION_KEY", validation)
	authRecoveryEnabled := getenvBoolTracked("AUTH_RECOVERY_ENABLED", false, validation)
	recoveryValidation := &envValidation{}
	smtpPassword := getenvOptionalSecretAllowEmptyTracked("SMTP_PASSWORD", recoveryValidation)
	cfg := Config{
		AppEnv:                         strings.ToLower(strings.TrimSpace(getenv("APP_ENV", "development"))),
		Port:                           getenv("PORT", "8080"),
		DatabaseURL:                    databaseURL,
		RedisURL:                       redisURL,
		RedisCacheURL:                  redisCacheURL,
		JWTSecret:                      jwtSecret,
		SettingsEncryptionKey:          settingsEncryptionKey,
		FrontendOrigin:                 getenv("FRONTEND_ORIGIN", "http://localhost:5173"),
		MigrationsDir:                  getenv("MIGRATIONS_DIR", "migrations"),
		AutoMigrate:                    getenvBoolTracked("AUTO_MIGRATE", true, validation),
		MigrationLockTimeout:           getenvDurationTracked("MIGRATION_LOCK_TIMEOUT", 30*time.Second, false, validation),
		MigrationTimeout:               getenvDurationTracked("MIGRATION_TIMEOUT", 5*time.Minute, false, validation),
		StoragePath:                    strings.TrimSpace(os.Getenv("STORAGE_PATH")),
		StorageProvider:                strings.ToLower(strings.TrimSpace(os.Getenv("STORAGE_PROVIDER"))),
		StorageOperationTimeout:        getenvDurationTracked("STORAGE_OPERATION_TIMEOUT", 30*time.Second, false, validation),
		StorageMigrationTimeout:        getenvDurationTracked("STORAGE_MIGRATION_TIMEOUT", 2*time.Hour, false, validation),
		StorageS3Bucket:                strings.TrimSpace(os.Getenv("STORAGE_S3_BUCKET")),
		StorageS3Region:                strings.TrimSpace(os.Getenv("STORAGE_S3_REGION")),
		StorageS3Endpoint:              strings.TrimSpace(os.Getenv("STORAGE_S3_ENDPOINT")),
		StorageS3UsePathStyle:          getenvBoolTracked("STORAGE_S3_USE_PATH_STYLE", false, validation),
		StorageS3AllowInsecureEndpoint: getenvBoolTracked("STORAGE_S3_ALLOW_INSECURE_ENDPOINT", false, validation),
		MaxUploadBytes:                 getenvInt64Tracked("MAX_UPLOAD_BYTES", 10*1024*1024, validation),
		AuthAccessTokenMinutes:         getenvInt64Tracked("AUTH_ACCESS_TOKEN_MINUTES", 15, validation),
		AuthRefreshTokenHours:          getenvInt64Tracked("AUTH_REFRESH_TOKEN_HOURS", 24*7, validation),
		AuthRefreshCookieSecure:        getenvBoolTracked("AUTH_REFRESH_COOKIE_SECURE", false, validation),
		AuthRefreshCookieSameSite:      getenv("AUTH_REFRESH_COOKIE_SAME_SITE", "Lax"),
		AuthRecoveryEnabled:            authRecoveryEnabled,
		AuthRecoveryTokenTTL:           getenvDurationTracked("AUTH_RECOVERY_TOKEN_TTL", 15*time.Minute, false, recoveryValidation),
		AuthRecoveryVerifyTTL:          getenvDurationTracked("AUTH_RECOVERY_VERIFY_TTL", 24*time.Hour, false, recoveryValidation),
		SMTPHost:                       strings.TrimSpace(os.Getenv("SMTP_HOST")),
		SMTPPort:                       getenvIntTracked("SMTP_PORT", 587, recoveryValidation),
		SMTPUsername:                   strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		SMTPPassword:                   smtpPassword,
		SMTPFromAddress:                strings.ToLower(strings.TrimSpace(os.Getenv("SMTP_FROM_ADDRESS"))),
		SMTPFromName:                   strings.TrimSpace(getenv("SMTP_FROM_NAME", "Consflow")),
		SMTPTLSMode:                    strings.ToLower(strings.TrimSpace(getenv("SMTP_TLS_MODE", "starttls"))),
		SMTPTimeout:                    getenvDurationTracked("SMTP_TIMEOUT", 10*time.Second, false, recoveryValidation),
		TrustedProxies:                 getenvCSV("TRUSTED_PROXY_CIDRS"),
		HTTPReadTimeout:                getenvDurationTracked("HTTP_READ_TIMEOUT", 15*time.Second, false, validation),
		// SSE responses are intentionally long-lived, so the global write deadline is disabled by default.
		HTTPWriteTimeout:             getenvDurationTracked("HTTP_WRITE_TIMEOUT", 0, true, validation),
		HTTPIdleTimeout:              getenvDurationTracked("HTTP_IDLE_TIMEOUT", 60*time.Second, false, validation),
		ShutdownTimeout:              getenvDurationTracked("SHUTDOWN_TIMEOUT", 15*time.Second, false, validation),
		ReadinessTimeout:             getenvDurationTracked("READINESS_TIMEOUT", 2*time.Second, false, validation),
		DBStatementTimeout:           getenvDurationTracked("DB_STATEMENT_TIMEOUT", 15*time.Second, false, validation),
		DBLockTimeout:                getenvDurationTracked("DB_LOCK_TIMEOUT", 5*time.Second, false, validation),
		DBMaxOpenConns:               getenvIntTracked("DB_MAX_OPEN_CONNS", 20, validation),
		DBMaxIdleConns:               getenvIntTracked("DB_MAX_IDLE_CONNS", 10, validation),
		DBConnMaxLifetime:            getenvDurationTracked("DB_CONN_MAX_LIFETIME", 30*time.Minute, false, validation),
		DBConnMaxIdleTime:            getenvDurationTracked("DB_CONN_MAX_IDLE_TIME", 5*time.Minute, false, validation),
		RedisOperationTimeout:        getenvDurationTracked("REDIS_OPERATION_TIMEOUT", 250*time.Millisecond, false, validation),
		ProjectExplorerCacheMode:     strings.ToLower(strings.TrimSpace(getenv("PROJECT_EXPLORER_CACHE_MODE", "off"))),
		ProjectExplorerCacheTimeout:  getenvDurationTracked("PROJECT_EXPLORER_CACHE_TIMEOUT", 200*time.Millisecond, false, validation),
		ProjectExplorerCacheIndexTTL: getenvDurationTracked("PROJECT_EXPLORER_CACHE_INDEX_TTL", 5*time.Minute, false, validation),
		ProjectExplorerCacheRowTTL:   getenvDurationTracked("PROJECT_EXPLORER_CACHE_ROW_TTL", 10*time.Minute, false, validation),
		ProjectExplorerCacheLockTTL:  getenvDurationTracked("PROJECT_EXPLORER_CACHE_LOCK_TTL", 2*time.Second, false, validation),
		ProjectExplorerCacheMaxBytes: getenvIntTracked("PROJECT_EXPLORER_CACHE_MAX_VALUE_BYTES", 256*1024, validation),
		RateLimitRedisMaxInFlight:    getenvIntTracked("RATE_LIMIT_REDIS_MAX_IN_FLIGHT", 32, validation),
		RateLimitEnabled:             getenvBoolTracked("RATE_LIMIT_ENABLED", true, validation),
		RateLimitMode:                strings.ToLower(strings.TrimSpace(getenv("RATE_LIMIT_MODE", "enforce"))),
		RateLimitLocalMaxKeys:        getenvIntTracked("RATE_LIMIT_LOCAL_MAX_KEYS", 10000, validation),
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
	cfg.secretEnvironmentErrors = validation.secretErrors
	cfg.defaultSecretEnvironment = validation.secretDefaults
	cfg.accountRecoveryInvalidEnv = recoveryValidation.invalid
	cfg.accountRecoverySecretErrors = recoveryValidation.secretErrors
	return cfg
}

// ValidateBootstrap validates only configuration that must be trustworthy
// before the application can connect to PostgreSQL and load database-backed
// runtime settings. Runtime policy validation remains in Validate.
func (c Config) ValidateBootstrap() error {
	appEnv := strings.ToLower(strings.TrimSpace(c.AppEnv))
	switch appEnv {
	case "development", "test", "staging", "production":
	default:
		return fmt.Errorf("APP_ENV must be one of development, test, staging, or production")
	}
	if len(c.secretEnvironmentErrors) > 0 {
		return fmt.Errorf("invalid secret configuration: %s", strings.Join(c.secretEnvironmentErrors, "; "))
	}
	if c.SettingsEncryptionKey != "" && len([]byte(c.SettingsEncryptionKey)) < 32 {
		return fmt.Errorf("SETTINGS_ENCRYPTION_KEY must be at least 32 bytes when configured")
	}
	for _, key := range c.invalidEnvironment {
		if bootstrapEnvironmentKey(key) {
			return fmt.Errorf("invalid bootstrap environment variable: %s", key)
		}
	}
	if c.AutoMigrate && c.MigrationLockTimeout > 0 && c.MigrationTimeout > 0 && c.MigrationLockTimeout >= c.MigrationTimeout {
		return fmt.Errorf("MIGRATION_LOCK_TIMEOUT must be less than MIGRATION_TIMEOUT when AUTO_MIGRATE is true")
	}
	if err := c.validateStorage(); err != nil {
		return err
	}
	if appEnv == "production" && c.StorageWriteProvider() != "s3" {
		return fmt.Errorf("STORAGE_PROVIDER must be s3 in production")
	}
	for _, proxy := range c.TrustedProxies {
		if net.ParseIP(proxy) == nil {
			if _, _, err := net.ParseCIDR(proxy); err != nil {
				return fmt.Errorf("TRUSTED_PROXY_CIDRS contains invalid IP or CIDR %q", proxy)
			}
		}
	}
	if appEnv != "production" {
		return nil
	}
	if len(c.defaultSecretEnvironment) > 0 {
		return fmt.Errorf("missing production secret configuration: %s", strings.Join(c.defaultSecretEnvironment, ", "))
	}
	if strings.TrimSpace(c.DatabaseURL) == "" || containsPlaceholder(c.DatabaseURL) {
		return fmt.Errorf("DATABASE_URL must not contain CHANGE_ME in production")
	}
	databaseURL, err := url.Parse(c.DatabaseURL)
	if err != nil || (databaseURL.Scheme != "postgres" && databaseURL.Scheme != "postgresql") || databaseURL.Host == "" {
		return fmt.Errorf("DATABASE_URL must be an absolute postgres:// or postgresql:// URL in production")
	}
	if strings.TrimSpace(c.RedisURL) == "" || containsPlaceholder(c.RedisURL) {
		return fmt.Errorf("REDIS_URL must not contain CHANGE_ME in production")
	}
	redisURL, err := url.Parse(c.RedisURL)
	if err != nil || (redisURL.Scheme != "redis" && redisURL.Scheme != "rediss") || redisURL.Host == "" {
		return fmt.Errorf("REDIS_URL must be an absolute redis:// or rediss:// URL in production")
	}
	if strings.TrimSpace(c.JWTSecret) == "" || containsPlaceholder(c.JWTSecret) {
		return fmt.Errorf("JWT_SECRET must not contain CHANGE_ME in production")
	}
	if len([]byte(c.JWTSecret)) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 bytes in production")
	}
	if strings.TrimSpace(c.SettingsEncryptionKey) == "" || containsPlaceholder(c.SettingsEncryptionKey) {
		return fmt.Errorf("SETTINGS_ENCRYPTION_KEY must be configured without CHANGE_ME in production")
	}
	if len([]byte(c.SettingsEncryptionKey)) < 32 {
		return fmt.Errorf("SETTINGS_ENCRYPTION_KEY must be at least 32 bytes in production")
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
	if c.AutoMigrate {
		return fmt.Errorf("AUTO_MIGRATE must be false in production; run the dedicated migration job before backend startup")
	}
	return nil
}

// MarkRuntimeSettingsDatabaseBacked discards parse errors from legacy runtime
// environment variables after authoritative values have been loaded from the
// database. Bootstrap, storage and deployment errors remain fail-closed.
func (c *Config) MarkRuntimeSettingsDatabaseBacked() {
	filtered := make([]string, 0, len(c.invalidEnvironment))
	for _, key := range c.invalidEnvironment {
		if bootstrapEnvironmentKey(key) {
			filtered = append(filtered, key)
		}
	}
	c.invalidEnvironment = filtered
	c.accountRecoveryInvalidEnv = nil
	c.accountRecoverySecretErrors = nil
}

func bootstrapEnvironmentKey(key string) bool {
	switch key {
	case "AUTO_MIGRATE", "MIGRATION_LOCK_TIMEOUT", "MIGRATION_TIMEOUT",
		"STORAGE_OPERATION_TIMEOUT", "STORAGE_MIGRATION_TIMEOUT",
		"STORAGE_S3_USE_PATH_STYLE", "STORAGE_S3_ALLOW_INSECURE_ENDPOINT",
		"AUTH_REFRESH_COOKIE_SECURE":
		return true
	default:
		return false
	}
}

func (c Config) Validate() error {
	if err := c.ValidateBootstrap(); err != nil {
		return err
	}
	appEnv := strings.ToLower(strings.TrimSpace(c.AppEnv))
	switch appEnv {
	case "development", "test", "staging", "production":
	default:
		return fmt.Errorf("APP_ENV must be one of development, test, staging, or production")
	}
	if len(c.secretEnvironmentErrors) > 0 {
		return fmt.Errorf("invalid secret configuration: %s", strings.Join(c.secretEnvironmentErrors, "; "))
	}
	if c.RateLimitMode != "observe" && c.RateLimitMode != "enforce" {
		return fmt.Errorf("RATE_LIMIT_MODE must be observe or enforce")
	}
	projectExplorerCacheMode := strings.ToLower(strings.TrimSpace(c.ProjectExplorerCacheMode))
	if projectExplorerCacheMode == "" {
		projectExplorerCacheMode = "off"
	}
	if projectExplorerCacheMode != "off" && projectExplorerCacheMode != "shadow" && projectExplorerCacheMode != "serve" {
		return fmt.Errorf("PROJECT_EXPLORER_CACHE_MODE must be off, shadow, or serve")
	}
	if projectExplorerCacheMode != "off" {
		if strings.TrimSpace(c.RedisCacheURL) == "" || containsPlaceholder(c.RedisCacheURL) {
			return fmt.Errorf("REDIS_CACHE_URL must be configured without CHANGE_ME when PROJECT_EXPLORER_CACHE_MODE is enabled")
		}
		redisCacheURL, err := url.Parse(c.RedisCacheURL)
		if err != nil || (redisCacheURL.Scheme != "redis" && redisCacheURL.Scheme != "rediss") || redisCacheURL.Host == "" {
			return fmt.Errorf("REDIS_CACHE_URL must be an absolute redis:// or rediss:// URL when PROJECT_EXPLORER_CACHE_MODE is enabled")
		}
	}
	if c.DBMaxIdleConns > c.DBMaxOpenConns {
		return fmt.Errorf("DB_MAX_IDLE_CONNS must not exceed DB_MAX_OPEN_CONNS")
	}
	if c.AutoMigrate && c.MigrationLockTimeout > 0 && c.MigrationTimeout > 0 && c.MigrationLockTimeout >= c.MigrationTimeout {
		return fmt.Errorf("MIGRATION_LOCK_TIMEOUT must be less than MIGRATION_TIMEOUT when AUTO_MIGRATE is true")
	}
	if err := c.validateStorage(); err != nil {
		return err
	}
	if appEnv == "production" && c.StorageWriteProvider() != "s3" {
		return fmt.Errorf("STORAGE_PROVIDER must be s3 in production")
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
		if len(c.defaultSecretEnvironment) > 0 {
			return fmt.Errorf("missing production secret configuration: %s", strings.Join(c.defaultSecretEnvironment, ", "))
		}
		if strings.TrimSpace(c.DatabaseURL) == "" || containsPlaceholder(c.DatabaseURL) {
			return fmt.Errorf("DATABASE_URL must not contain CHANGE_ME in production")
		}
		databaseURL, err := url.Parse(c.DatabaseURL)
		if err != nil || (databaseURL.Scheme != "postgres" && databaseURL.Scheme != "postgresql") || databaseURL.Host == "" {
			return fmt.Errorf("DATABASE_URL must be an absolute postgres:// or postgresql:// URL in production")
		}
		if strings.TrimSpace(c.RedisURL) == "" || containsPlaceholder(c.RedisURL) {
			return fmt.Errorf("REDIS_URL must not contain CHANGE_ME in production")
		}
		redisURL, err := url.Parse(c.RedisURL)
		if err != nil || (redisURL.Scheme != "redis" && redisURL.Scheme != "rediss") || redisURL.Host == "" {
			return fmt.Errorf("REDIS_URL must be an absolute redis:// or rediss:// URL in production")
		}
		if strings.TrimSpace(c.JWTSecret) == "" || containsPlaceholder(c.JWTSecret) {
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
		if c.AutoMigrate {
			return fmt.Errorf("AUTO_MIGRATE must be false in production; run the dedicated migration job before backend startup")
		}
	}
	return nil
}

// AccountRecoveryConfigurationError validates the optional mail subsystem
// independently from startup-critical configuration. Callers should disable
// only account recovery when this returns an error; the rest of the backend
// remains available.
func (c Config) AccountRecoveryConfigurationError() error {
	if !c.AuthRecoveryEnabled {
		return nil
	}
	if len(c.accountRecoverySecretErrors) > 0 {
		return fmt.Errorf("invalid account recovery secret configuration: %s", strings.Join(c.accountRecoverySecretErrors, "; "))
	}
	if len(c.accountRecoveryInvalidEnv) > 0 {
		return fmt.Errorf("invalid account recovery environment variables: %s", strings.Join(c.accountRecoveryInvalidEnv, ", "))
	}
	return c.validateAccountRecovery(strings.ToLower(strings.TrimSpace(c.AppEnv)))
}

func (c Config) validateAccountRecovery(appEnv string) error {
	if !c.AuthRecoveryEnabled {
		return nil
	}
	if strings.TrimSpace(c.SMTPHost) == "" || containsPlaceholder(c.SMTPHost) {
		return fmt.Errorf("SMTP_HOST must be configured without CHANGE_ME when account recovery is enabled")
	}
	if c.SMTPPort < 1 || c.SMTPPort > 65535 {
		return fmt.Errorf("SMTP_PORT must be between 1 and 65535")
	}
	if c.AuthRecoveryTokenTTL <= 0 || c.AuthRecoveryVerifyTTL <= 0 || c.SMTPTimeout <= 0 {
		return fmt.Errorf("account recovery and SMTP timeouts must be positive")
	}
	if c.SMTPTLSMode != "starttls" && c.SMTPTLSMode != "tls" && c.SMTPTLSMode != "none" {
		return fmt.Errorf("SMTP_TLS_MODE must be starttls, tls, or none")
	}
	if appEnv == "production" && c.SMTPTLSMode == "none" {
		return fmt.Errorf("SMTP_TLS_MODE must use starttls or tls in production")
	}
	from, err := mail.ParseAddress(strings.ToLower(strings.TrimSpace(c.SMTPFromAddress)))
	if err != nil || from.Address != strings.ToLower(strings.TrimSpace(c.SMTPFromAddress)) {
		return fmt.Errorf("SMTP_FROM_ADDRESS must be a valid bare email address")
	}
	if strings.TrimSpace(c.SMTPUsername) == "" && c.SMTPPassword != "" || strings.TrimSpace(c.SMTPUsername) != "" && c.SMTPPassword == "" {
		return fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD must be configured together")
	}
	if containsPlaceholder(c.SMTPUsername) || containsPlaceholder(c.SMTPPassword) || containsPlaceholder(c.SMTPFromAddress) {
		return fmt.Errorf("SMTP credentials and sender must not contain CHANGE_ME")
	}
	if strings.ContainsAny(c.SMTPFromName, "\r\n") {
		return fmt.Errorf("SMTP_FROM_NAME must not contain line breaks")
	}
	return nil
}

func (c Config) validateStorage() error {
	provider := c.StorageWriteProvider()
	rawProvider := strings.ToLower(strings.TrimSpace(c.StorageProvider))
	if rawProvider != "" && rawProvider != "default" && rawProvider != "s3" {
		return fmt.Errorf("STORAGE_PROVIDER must be empty, default, or s3")
	}
	if provider == "local" && strings.TrimSpace(c.StoragePath) == "" {
		return fmt.Errorf("STORAGE_PATH must not be empty when STORAGE_PROVIDER is empty or default")
	}
	if c.StorageOperationTimeout < 0 {
		return fmt.Errorf("STORAGE_OPERATION_TIMEOUT must not be negative")
	}

	s3Configured := strings.TrimSpace(c.StorageS3Bucket) != "" ||
		strings.TrimSpace(c.StorageS3Region) != "" ||
		strings.TrimSpace(c.StorageS3Endpoint) != "" ||
		c.StorageS3UsePathStyle
	if provider == "s3" || s3Configured {
		if strings.TrimSpace(c.StorageS3Bucket) == "" {
			return fmt.Errorf("STORAGE_S3_BUCKET must not be empty when S3 storage is configured")
		}
		if strings.TrimSpace(c.StorageS3Region) == "" {
			return fmt.Errorf("STORAGE_S3_REGION must not be empty when S3 storage is configured")
		}
	}
	if endpoint := strings.TrimSpace(c.StorageS3Endpoint); endpoint != "" {
		parsed, err := url.Parse(endpoint)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("STORAGE_S3_ENDPOINT must be an absolute http:// or https:// URL without credentials, query, or fragment")
		}
		if strings.EqualFold(strings.TrimSpace(c.AppEnv), "production") && parsed.Scheme != "https" && !c.StorageS3AllowInsecureEndpoint {
			return fmt.Errorf("STORAGE_S3_ENDPOINT must use HTTPS in production unless STORAGE_S3_ALLOW_INSECURE_ENDPOINT=true")
		}
	} else if c.StorageS3AllowInsecureEndpoint {
		return fmt.Errorf("STORAGE_S3_ALLOW_INSECURE_ENDPOINT requires STORAGE_S3_ENDPOINT")
	}
	return nil
}

// StorageWriteProvider resolves the environment-facing default provider to the
// persisted provider name used by the storage registry and database metadata.
func (c Config) StorageWriteProvider() string {
	provider := strings.ToLower(strings.TrimSpace(c.StorageProvider))
	if provider == "" || provider == "default" {
		return "local"
	}
	return provider
}

// ValidateStorageMigration validates the one-shot local-to-S3 operator job.
// It intentionally requires only the database secret and S3 runtime contract;
// Redis, JWT, frontend, and HTTP settings are irrelevant to this offline job.
func (c Config) ValidateStorageMigration() error {
	appEnv := strings.ToLower(strings.TrimSpace(c.AppEnv))
	switch appEnv {
	case "development", "test", "staging", "production":
	default:
		return fmt.Errorf("APP_ENV must be one of development, test, staging, or production")
	}
	for _, message := range c.secretEnvironmentErrors {
		if strings.HasPrefix(message, "DATABASE_URL") {
			return fmt.Errorf("invalid storage migration secret configuration: %s", message)
		}
	}
	if appEnv == "production" {
		for _, source := range c.defaultSecretEnvironment {
			if strings.HasPrefix(source, "DATABASE_URL") {
				return fmt.Errorf("missing production storage migration secret configuration: %s", source)
			}
		}
	}
	for _, key := range c.invalidEnvironment {
		if key == "STORAGE_OPERATION_TIMEOUT" || key == "STORAGE_MIGRATION_TIMEOUT" || key == "STORAGE_S3_USE_PATH_STYLE" || key == "STORAGE_S3_ALLOW_INSECURE_ENDPOINT" {
			return fmt.Errorf("invalid storage migration environment variable: %s", key)
		}
	}
	if err := c.validateStorage(); err != nil {
		return err
	}
	if strings.TrimSpace(c.StoragePath) == "" {
		return fmt.Errorf("STORAGE_PATH is required for the external legacy storage migration source")
	}
	if strings.TrimSpace(c.StorageS3Bucket) == "" || strings.TrimSpace(c.StorageS3Region) == "" {
		return fmt.Errorf("STORAGE_S3_BUCKET and STORAGE_S3_REGION are required for storage migration")
	}
	if c.StorageMigrationTimeout <= 0 {
		return fmt.Errorf("STORAGE_MIGRATION_TIMEOUT must be positive")
	}
	databaseURL, err := url.Parse(c.DatabaseURL)
	if err != nil || (databaseURL.Scheme != "postgres" && databaseURL.Scheme != "postgresql") || databaseURL.Host == "" {
		return fmt.Errorf("DATABASE_URL must be an absolute postgres:// or postgresql:// URL")
	}
	if appEnv == "production" && containsPlaceholder(c.DatabaseURL) {
		return fmt.Errorf("DATABASE_URL must not contain CHANGE_ME in production")
	}
	return nil
}

func (c Config) ValidateMigration() error {
	appEnv := strings.ToLower(strings.TrimSpace(c.AppEnv))
	switch appEnv {
	case "development", "test", "staging", "production":
	default:
		return fmt.Errorf("APP_ENV must be one of development, test, staging, or production")
	}
	for _, message := range c.secretEnvironmentErrors {
		if strings.HasPrefix(message, "DATABASE_URL") {
			return fmt.Errorf("invalid migration secret configuration: %s", message)
		}
	}
	for _, key := range c.invalidEnvironment {
		if key == "MIGRATION_LOCK_TIMEOUT" || key == "MIGRATION_TIMEOUT" {
			return fmt.Errorf("invalid migration environment variable: %s", key)
		}
	}
	if appEnv == "production" {
		for _, source := range c.defaultSecretEnvironment {
			if strings.HasPrefix(source, "DATABASE_URL") {
				return fmt.Errorf("missing production migration secret configuration: %s", source)
			}
		}
	}
	if strings.TrimSpace(c.MigrationsDir) == "" {
		return fmt.Errorf("MIGRATIONS_DIR must not be empty")
	}
	if c.MigrationLockTimeout <= 0 {
		return fmt.Errorf("MIGRATION_LOCK_TIMEOUT must be positive")
	}
	if c.MigrationTimeout <= 0 {
		return fmt.Errorf("MIGRATION_TIMEOUT must be positive")
	}
	if c.MigrationLockTimeout >= c.MigrationTimeout {
		return fmt.Errorf("MIGRATION_LOCK_TIMEOUT must be less than MIGRATION_TIMEOUT")
	}
	databaseURL, err := url.Parse(c.DatabaseURL)
	if err != nil || (databaseURL.Scheme != "postgres" && databaseURL.Scheme != "postgresql") || databaseURL.Host == "" {
		return fmt.Errorf("DATABASE_URL must be an absolute postgres:// or postgresql:// URL")
	}
	if appEnv == "production" && containsPlaceholder(c.DatabaseURL) {
		return fmt.Errorf("DATABASE_URL must not contain CHANGE_ME in production")
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
	invalid        []string
	secretErrors   []string
	secretDefaults []string
}

func (v *envValidation) markInvalid(key string) {
	for _, existing := range v.invalid {
		if existing == key {
			return
		}
	}
	v.invalid = append(v.invalid, key)
}

func (v *envValidation) markSecretError(message string) {
	v.secretErrors = append(v.secretErrors, message)
}

func (v *envValidation) markSecretDefault(key string) {
	v.secretDefaults = append(v.secretDefaults, key)
}

func getenvSecretTracked(key, fallback string, validation *envValidation) string {
	directValue, directSet := os.LookupEnv(key)
	fileKey := key + "_FILE"
	filePath, fileSet := os.LookupEnv(fileKey)
	if directSet && fileSet {
		validation.markSecretError(fmt.Sprintf("%s and %s must not both be set", key, fileKey))
		return ""
	}
	if directSet {
		if directValue == "" {
			validation.markSecretError(fmt.Sprintf("%s must not be empty", key))
			return ""
		}
		return directValue
	}
	if !fileSet {
		validation.markSecretDefault(fmt.Sprintf("%s or %s", key, fileKey))
		return fallback
	}
	if strings.TrimSpace(filePath) == "" {
		validation.markSecretError(fmt.Sprintf("%s must name a readable secret file", fileKey))
		return ""
	}
	value, err := readSecretFile(filePath)
	if err != nil {
		validation.markSecretError(fmt.Sprintf("%s: %v", fileKey, err))
		return ""
	}
	return value
}

func getenvOptionalSecretTracked(key string, validation *envValidation) string {
	directValue, directSet := os.LookupEnv(key)
	fileKey := key + "_FILE"
	filePath, fileSet := os.LookupEnv(fileKey)
	if directSet && fileSet {
		validation.markSecretError(fmt.Sprintf("%s and %s must not both be set", key, fileKey))
		return ""
	}
	if directSet {
		if strings.TrimSpace(directValue) == "" {
			validation.markSecretError(fmt.Sprintf("%s must not be empty", key))
			return ""
		}
		return directValue
	}
	if !fileSet {
		return ""
	}
	if strings.TrimSpace(filePath) == "" {
		validation.markSecretError(fmt.Sprintf("%s must name a readable secret file", fileKey))
		return ""
	}
	value, err := readSecretFile(filePath)
	if err != nil {
		validation.markSecretError(fmt.Sprintf("%s: %v", fileKey, err))
		return ""
	}
	return value
}

// getenvOptionalSecretAllowEmptyTracked treats empty direct or file values as
// absent so Compose can mount /dev/null while the feature is disabled. A
// non-empty configured file remains fail-closed and conflicts with a non-empty
// direct value.
func getenvOptionalSecretAllowEmptyTracked(key string, validation *envValidation) string {
	directValue, directSet := os.LookupEnv(key)
	if directSet && strings.TrimSpace(directValue) == "" {
		directSet = false
	}
	fileKey := key + "_FILE"
	filePath, fileSet := os.LookupEnv(fileKey)
	if directSet && fileSet {
		validation.markSecretError(fmt.Sprintf("%s and %s must not both be set", key, fileKey))
		return ""
	}
	if directSet {
		return directValue
	}
	if !fileSet {
		return ""
	}
	if strings.TrimSpace(filePath) == "" {
		validation.markSecretError(fmt.Sprintf("%s must name a readable secret file", fileKey))
		return ""
	}
	value, err := readSecretFile(filePath)
	if err != nil {
		if strings.Contains(err.Error(), "secret file is empty") {
			return ""
		}
		validation.markSecretError(fmt.Sprintf("%s: %v", fileKey, err))
		return ""
	}
	return value
}

func readSecretFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open secret file: %w", err)
	}
	defer file.Close()

	value, err := io.ReadAll(io.LimitReader(file, maxSecretFileBytes+1))
	if err != nil {
		return "", fmt.Errorf("read secret file: %w", err)
	}
	if len(value) > maxSecretFileBytes {
		return "", fmt.Errorf("secret file exceeds %d bytes", maxSecretFileBytes)
	}
	if bytes.IndexByte(value, 0) >= 0 {
		return "", fmt.Errorf("secret file contains a NUL byte")
	}
	if bytes.HasSuffix(value, []byte("\r\n")) {
		value = value[:len(value)-2]
	} else if bytes.HasSuffix(value, []byte("\n")) {
		value = value[:len(value)-1]
	}
	if len(value) == 0 {
		return "", fmt.Errorf("secret file is empty")
	}
	return string(value), nil
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
