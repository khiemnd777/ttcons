package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigValidateTrustedProxiesAndMode(t *testing.T) {
	cfg := Config{AppEnv: "development", RateLimitMode: "enforce", DBMaxOpenConns: 20, DBMaxIdleConns: 10, StoragePath: t.TempDir()}
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

func TestProjectExplorerCacheRequiresDedicatedRedisWhenEnabled(t *testing.T) {
	base := Config{
		AppEnv:                   "development",
		RateLimitMode:            "enforce",
		DBMaxOpenConns:           20,
		DBMaxIdleConns:           10,
		StoragePath:              t.TempDir(),
		ProjectExplorerCacheMode: "serve",
	}
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "REDIS_CACHE_URL") {
		t.Fatalf("enabled cache without dedicated URL error=%v", err)
	}
	base.RedisCacheURL = "redis://cache:6379/0"
	if err := base.Validate(); err != nil {
		t.Fatalf("valid dedicated cache rejected: %v", err)
	}
	base.RedisCacheURL = "redis://:CHANGE_ME@cache:6379/0"
	if err := base.Validate(); err == nil {
		t.Fatal("cache placeholder URL was accepted")
	}
}

func TestLoadOptionalRedisCacheURLFromFile(t *testing.T) {
	clearSecretFileEnvironment(t)
	unsetEnvironment(t, "REDIS_CACHE_URL")
	path := filepath.Join(t.TempDir(), "redis-cache-url")
	if err := os.WriteFile(path, []byte("redis://cache:6379/0\n"), 0o600); err != nil {
		t.Fatalf("write cache URL: %v", err)
	}
	t.Setenv("REDIS_CACHE_URL_FILE", path)
	t.Setenv("PROJECT_EXPLORER_CACHE_MODE", "shadow")
	cfg := Load()
	if cfg.RedisCacheURL != "redis://cache:6379/0" || cfg.ProjectExplorerCacheMode != "shadow" {
		t.Fatalf("cache config=(%q,%q)", cfg.RedisCacheURL, cfg.ProjectExplorerCacheMode)
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
		StorageProvider:         "s3",
		StorageOperationTimeout: 30 * time.Second,
		StorageS3Bucket:         "artifacts",
		StorageS3Region:         "ap-southeast-1",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid production config rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"database placeholder", func(cfg *Config) { cfg.DatabaseURL = "postgres://consflow:CHANGE_ME@postgres/consflow" }},
		{"malformed database url", func(cfg *Config) { cfg.DatabaseURL = "not-a-url" }},
		{"redis placeholder", func(cfg *Config) { cfg.RedisURL = "redis://:CHANGE_ME@redis:6379/0" }},
		{"malformed redis url", func(cfg *Config) { cfg.RedisURL = "not-a-url" }},
		{"jwt placeholder", func(cfg *Config) { cfg.JWTSecret = "CHANGE_ME_0123456789abcdef0123456789" }},
		{"short jwt", func(cfg *Config) { cfg.JWTSecret = "too-short" }},
		{"non https origin", func(cfg *Config) { cfg.FrontendOrigin = "http://workflow.example.com" }},
		{"empty trusted proxies", func(cfg *Config) { cfg.TrustedProxies = nil }},
		{"insecure refresh cookie", func(cfg *Config) { cfg.AuthRefreshCookieSecure = false }},
		{"disabled limiter", func(cfg *Config) { cfg.RateLimitEnabled = false }},
		{"backend auto migrate", func(cfg *Config) { cfg.AutoMigrate = true }},
		{"non-S3 production storage", func(cfg *Config) {
			cfg.StorageProvider = "default"
			cfg.StoragePath = t.TempDir()
			cfg.StorageS3Bucket = ""
			cfg.StorageS3Region = ""
		}},
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
		{key: "MIGRATION_LOCK_TIMEOUT", value: "0s"},
		{key: "MIGRATION_TIMEOUT", value: "not-a-duration"},
		{key: "AUTO_MIGRATE", value: "not-a-bool"},
		{key: "STORAGE_OPERATION_TIMEOUT", value: "0s"},
		{key: "STORAGE_S3_USE_PATH_STYLE", value: "not-a-bool"},
		{key: "STORAGE_S3_ALLOW_INSECURE_ENDPOINT", value: "not-a-bool"},
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

func TestStorageConfigRequiresCompleteS3Contract(t *testing.T) {
	base := Config{AppEnv: "development", RateLimitMode: "enforce", DBMaxOpenConns: 20, DBMaxIdleConns: 10, StoragePath: t.TempDir(), StorageOperationTimeout: 30 * time.Second}
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"unknown provider", func(cfg *Config) { cfg.StorageProvider = "filesystem" }},
		{"legacy local provider alias", func(cfg *Config) { cfg.StorageProvider = "local" }},
		{"missing bucket", func(cfg *Config) { cfg.StorageProvider = "s3"; cfg.StorageS3Region = "ap-southeast-1" }},
		{"missing region", func(cfg *Config) { cfg.StorageProvider = "s3"; cfg.StorageS3Bucket = "artifacts" }},
		{"endpoint credentials", func(cfg *Config) {
			cfg.StorageProvider = "s3"
			cfg.StorageS3Bucket = "artifacts"
			cfg.StorageS3Region = "ap-southeast-1"
			cfg.StorageS3Endpoint = "https://user:secret@storage.example.com"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := base
			test.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("incomplete or unsafe S3 configuration accepted")
			}
		})
	}

	valid := base
	valid.StorageProvider = "s3"
	valid.StorageS3Bucket = "artifacts"
	valid.StorageS3Region = "ap-southeast-1"
	valid.StorageS3Endpoint = "https://storage.example.com"
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid S3-compatible configuration rejected: %v", err)
	}
}

func TestDefaultStorageRequiresPathAndS3DoesNot(t *testing.T) {
	base := Config{AppEnv: "development", RateLimitMode: "enforce", DBMaxOpenConns: 20, DBMaxIdleConns: 10, StorageOperationTimeout: 30 * time.Second}
	for _, provider := range []string{"", "default"} {
		cfg := base
		cfg.StorageProvider = provider
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "STORAGE_PATH") {
			t.Fatalf("provider=%q missing path error=%v", provider, err)
		}
		cfg.StoragePath = t.TempDir()
		if err := cfg.Validate(); err != nil {
			t.Fatalf("provider=%q with path rejected: %v", provider, err)
		}
	}
	s3Config := base
	s3Config.StorageProvider = "s3"
	s3Config.StorageS3Bucket = "artifacts"
	s3Config.StorageS3Region = "ap-southeast-1"
	if err := s3Config.Validate(); err != nil {
		t.Fatalf("S3 config incorrectly required STORAGE_PATH: %v", err)
	}
}

func TestProductionS3EndpointRequiresHTTPSOrExplicitOverride(t *testing.T) {
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
		StorageProvider:         "s3",
		StorageOperationTimeout: 30 * time.Second,
		StorageS3Bucket:         "artifacts",
		StorageS3Region:         "ap-southeast-1",
		StorageS3Endpoint:       "http://minio.storage.svc:9000",
		StorageS3UsePathStyle:   true,
	}
	if err := valid.Validate(); err == nil || !strings.Contains(err.Error(), "STORAGE_S3_ALLOW_INSECURE_ENDPOINT") {
		t.Fatalf("production HTTP endpoint error=%v", err)
	}
	valid.StorageS3AllowInsecureEndpoint = true
	if err := valid.Validate(); err != nil {
		t.Fatalf("explicit private HTTP endpoint override rejected: %v", err)
	}
}

func TestStorageMigrationValidationRequiresOnlyDatabaseAndS3(t *testing.T) {
	clearSecretFileEnvironment(t)
	for _, key := range []string{"DATABASE_URL", "REDIS_URL", "JWT_SECRET"} {
		unsetEnvironment(t, key)
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://consflow:strong-password@postgres:5432/consflow?sslmode=disable")
	t.Setenv("STORAGE_PROVIDER", "s3")
	t.Setenv("STORAGE_PATH", "/external/legacy-storage")
	t.Setenv("STORAGE_S3_BUCKET", "artifacts")
	t.Setenv("STORAGE_S3_REGION", "ap-southeast-1")
	t.Setenv("STORAGE_S3_ENDPOINT", "https://storage.example.com")
	t.Setenv("STORAGE_MIGRATION_TIMEOUT", "2h")

	cfg := Load()
	if err := cfg.ValidateStorageMigration(); err != nil {
		t.Fatalf("storage migration config rejected without unrelated runtime secrets: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("full server validation accepted missing Redis/JWT secrets")
	}
}

func TestDevelopmentLoadFallsBackForMalformedRuntimeEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("STORAGE_PATH", t.TempDir())
	t.Setenv("HTTP_READ_TIMEOUT", "not-a-duration")
	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development config rejected fallback value: %v", err)
	}
	if cfg.HTTPReadTimeout <= 0 {
		t.Fatalf("development fallback HTTP read timeout=%v, want positive default", cfg.HTTPReadTimeout)
	}
}

func TestValidateMigrationUsesBoundedDefaults(t *testing.T) {
	clearSecretFileEnvironment(t)
	for _, key := range []string{"DATABASE_URL", "REDIS_URL", "JWT_SECRET"} {
		unsetEnvironment(t, key)
	}
	t.Setenv("APP_ENV", "development")

	cfg := Load()
	if err := cfg.ValidateMigration(); err != nil {
		t.Fatalf("development migration config rejected: %v", err)
	}
	if cfg.MigrationLockTimeout != 30*time.Second {
		t.Fatalf("MigrationLockTimeout=%v, want 30s", cfg.MigrationLockTimeout)
	}
	if cfg.MigrationTimeout != 5*time.Minute {
		t.Fatalf("MigrationTimeout=%v, want 5m", cfg.MigrationTimeout)
	}
}

func TestProductionMigrationValidationRequiresOnlyDatabaseSecret(t *testing.T) {
	clearSecretFileEnvironment(t)
	for _, key := range []string{"DATABASE_URL", "REDIS_URL", "JWT_SECRET"} {
		unsetEnvironment(t, key)
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://consflow:strong-password@postgres:5432/consflow?sslmode=disable")
	t.Setenv("MIGRATION_LOCK_TIMEOUT", "45s")
	t.Setenv("MIGRATION_TIMEOUT", "10m")

	cfg := Load()
	if err := cfg.ValidateMigration(); err != nil {
		t.Fatalf("migration-only production config rejected: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("full server validation unexpectedly accepted missing runtime secrets")
	}
}

func TestMigrationValidationRequiresLockTimeoutStrictlyBeforeTotalTimeout(t *testing.T) {
	clearSecretFileEnvironment(t)
	for _, key := range []string{"DATABASE_URL", "REDIS_URL", "JWT_SECRET"} {
		unsetEnvironment(t, key)
	}
	t.Setenv("APP_ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://consflow:consflow@127.0.0.1:5432/consflow?sslmode=disable")
	t.Setenv("MIGRATION_LOCK_TIMEOUT", "10m")
	t.Setenv("MIGRATION_TIMEOUT", "10m")

	err := Load().ValidateMigration()
	if err == nil || !strings.Contains(err.Error(), "MIGRATION_LOCK_TIMEOUT must be less than MIGRATION_TIMEOUT") {
		t.Fatalf("timeout ordering error=%v, want bounded ordering rejection", err)
	}
}

func TestLoadSecretValuesFromDirectEnvironment(t *testing.T) {
	clearSecretFileEnvironment(t)
	t.Setenv("DATABASE_URL", "postgres://direct:password@postgres:5432/consflow?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://:direct-password@redis:6379/0")
	t.Setenv("JWT_SECRET", "direct-secret-0123456789abcdef012345")

	cfg := Load()
	if cfg.DatabaseURL != "postgres://direct:password@postgres:5432/consflow?sslmode=disable" {
		t.Fatalf("DatabaseURL=%q, want direct environment value", cfg.DatabaseURL)
	}
	if cfg.RedisURL != "redis://:direct-password@redis:6379/0" {
		t.Fatalf("RedisURL=%q, want direct environment value", cfg.RedisURL)
	}
	if cfg.JWTSecret != "direct-secret-0123456789abcdef012345" {
		t.Fatalf("JWTSecret=%q, want direct environment value", cfg.JWTSecret)
	}
}

func TestLoadSecretValuesFromFiles(t *testing.T) {
	setValidProductionEnvironment(t)
	secretDir := t.TempDir()
	databaseURL := "postgres://consflow:file-password@postgres:5432/consflow?sslmode=disable"
	redisURL := "redis://:file-password@redis:6379/0"
	jwtSecret := "file-secret-0123456789abcdef01234567"
	files := map[string]string{
		"DATABASE_URL": databaseURL + "\n",
		"REDIS_URL":    redisURL + "\r\n",
		"JWT_SECRET":   jwtSecret + "\n",
	}
	for key, value := range files {
		unsetEnvironment(t, key)
		path := filepath.Join(secretDir, strings.ToLower(key))
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatalf("write %s secret: %v", key, err)
		}
		t.Setenv(key+"_FILE", path)
	}

	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid file-backed production config rejected: %v", err)
	}
	if cfg.DatabaseURL != databaseURL || cfg.RedisURL != redisURL || cfg.JWTSecret != jwtSecret {
		t.Fatalf("resolved secrets = (%q, %q, %q), want (%q, %q, %q)", cfg.DatabaseURL, cfg.RedisURL, cfg.JWTSecret, databaseURL, redisURL, jwtSecret)
	}
}

func TestLoadSecretEnvironmentConflictFailsClosed(t *testing.T) {
	clearSecretFileEnvironment(t)
	t.Setenv("DATABASE_URL", "postgres://direct:password@postgres:5432/consflow?sslmode=disable")
	secretPath := filepath.Join(t.TempDir(), "database-url")
	if err := os.WriteFile(secretPath, []byte("postgres://file:password@postgres:5432/consflow?sslmode=disable\n"), 0o600); err != nil {
		t.Fatalf("write database secret: %v", err)
	}
	t.Setenv("DATABASE_URL_FILE", secretPath)

	cfg := Load()
	if cfg.DatabaseURL != "" {
		t.Fatalf("conflicted DatabaseURL=%q, want unusable empty value", cfg.DatabaseURL)
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL and DATABASE_URL_FILE") {
		t.Fatalf("direct/file conflict error=%v, want both keys identified", err)
	}
}

func TestProductionLoadRejectsMissingSecretSources(t *testing.T) {
	for _, key := range []string{"DATABASE_URL", "REDIS_URL", "JWT_SECRET"} {
		t.Run(key, func(t *testing.T) {
			setValidProductionEnvironment(t)
			unsetEnvironment(t, key)

			err := Load().Validate()
			if err == nil || !strings.Contains(err.Error(), key+" or "+key+"_FILE") {
				t.Fatalf("missing source error=%v, want %s source identified", err, key)
			}
		})
	}
}

func TestLoadRejectsExplicitlyEmptyDirectSecretSources(t *testing.T) {
	for _, key := range []string{"DATABASE_URL", "REDIS_URL", "JWT_SECRET"} {
		t.Run(key, func(t *testing.T) {
			clearSecretFileEnvironment(t)
			t.Setenv(key, "")

			err := Load().Validate()
			if err == nil || !strings.Contains(err.Error(), key+" must not be empty") {
				t.Fatalf("empty direct source error=%v, want %s identified", err, key)
			}
		})
	}
}

func TestDevelopmentLoadAllowsMissingSecretSources(t *testing.T) {
	clearSecretFileEnvironment(t)
	for _, key := range []string{"DATABASE_URL", "REDIS_URL", "JWT_SECRET"} {
		unsetEnvironment(t, key)
	}
	t.Setenv("APP_ENV", "development")
	t.Setenv("STORAGE_PATH", t.TempDir())

	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development defaults rejected: %v", err)
	}
	if cfg.DatabaseURL == "" || cfg.RedisURL == "" || cfg.JWTSecret == "" {
		t.Fatal("development secret defaults were not applied")
	}
}

func TestReadSecretFileRejectsInvalidInput(t *testing.T) {
	secretDir := t.TempDir()
	tests := []struct {
		name    string
		path    string
		content []byte
	}{
		{name: "missing", path: filepath.Join(secretDir, "missing")},
		{name: "unreadable directory", path: secretDir},
		{name: "empty", path: filepath.Join(secretDir, "empty"), content: []byte{}},
		{name: "newline only", path: filepath.Join(secretDir, "newline"), content: []byte("\n")},
		{name: "nul", path: filepath.Join(secretDir, "nul"), content: []byte("secret\x00value")},
		{name: "oversized", path: filepath.Join(secretDir, "oversized"), content: []byte(strings.Repeat("x", maxSecretFileBytes+1))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.content != nil || test.name == "empty" {
				if err := os.WriteFile(test.path, test.content, 0o600); err != nil {
					t.Fatalf("write secret fixture: %v", err)
				}
			}
			if _, err := readSecretFile(test.path); err == nil {
				t.Fatal("invalid secret file was accepted")
			}
		})
	}
}

func TestReadSecretFileStripsExactlyOneTrailingNewline(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "line feed", content: "secret\n", want: "secret"},
		{name: "carriage return line feed", content: "secret\r\n", want: "secret"},
		{name: "only one line ending", content: "secret\n\n", want: "secret\n"},
		{name: "no line ending", content: "secret", want: "secret"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "secret")
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatalf("write secret fixture: %v", err)
			}
			got, err := readSecretFile(path)
			if err != nil {
				t.Fatalf("read secret: %v", err)
			}
			if got != test.want {
				t.Fatalf("resolved secret=%q, want %q", got, test.want)
			}
		})
	}
}

func TestFileBackedSecretStillUsesProductionPlaceholderValidation(t *testing.T) {
	setValidProductionEnvironment(t)
	unsetEnvironment(t, "JWT_SECRET")
	path := filepath.Join(t.TempDir(), "jwt-secret")
	if err := os.WriteFile(path, []byte("CHANGE_ME_0123456789abcdef0123456789\n"), 0o600); err != nil {
		t.Fatalf("write jwt secret: %v", err)
	}
	t.Setenv("JWT_SECRET_FILE", path)

	err := Load().Validate()
	if err == nil || !strings.Contains(err.Error(), "JWT_SECRET must not contain CHANGE_ME") {
		t.Fatalf("placeholder validation error=%v, want JWT_SECRET rejection", err)
	}
}

func setValidProductionEnvironment(t *testing.T) {
	t.Helper()
	clearSecretFileEnvironment(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://consflow:strong-password@postgres:5432/consflow?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://:strong-password@redis:6379/0")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("FRONTEND_ORIGIN", "https://workflow.example.com")
	t.Setenv("TRUSTED_PROXY_CIDRS", "172.30.0.0/24")
	t.Setenv("AUTH_REFRESH_COOKIE_SECURE", "true")
	t.Setenv("RATE_LIMIT_ENABLED", "true")
	t.Setenv("AUTO_MIGRATE", "false")
	t.Setenv("STORAGE_PROVIDER", "s3")
	t.Setenv("STORAGE_S3_BUCKET", "artifacts")
	t.Setenv("STORAGE_S3_REGION", "ap-southeast-1")
}

func clearSecretFileEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"DATABASE_URL_FILE", "REDIS_URL_FILE", "REDIS_CACHE_URL_FILE", "JWT_SECRET_FILE"} {
		unsetEnvironment(t, key)
	}
}

func unsetEnvironment(t *testing.T, key string) {
	t.Helper()
	value, exists := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if exists {
			_ = os.Setenv(key, value)
			return
		}
		_ = os.Unsetenv(key)
	})
}
