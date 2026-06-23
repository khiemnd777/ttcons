package config

import (
	"os"
	"strconv"
)

type Config struct {
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
}

func Load() Config {
	return Config{
		Port:                      getenv("PORT", "8080"),
		DatabaseURL:               getenv("DATABASE_URL", "postgres://consflow:consflow@127.0.0.1:5432/consflow?sslmode=disable"),
		RedisURL:                  getenv("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:                 getenv("JWT_SECRET", "consflow-local-dev-secret"),
		FrontendOrigin:            getenv("FRONTEND_ORIGIN", "http://localhost:5173"),
		MigrationsDir:             getenv("MIGRATIONS_DIR", "migrations"),
		AutoMigrate:               getenv("AUTO_MIGRATE", "true") != "false",
		UploadDir:                 getenv("UPLOAD_DIR", "uploads"),
		MaxUploadBytes:            getenvInt64("MAX_UPLOAD_BYTES", 10*1024*1024),
		AuthAccessTokenMinutes:    getenvInt64("AUTH_ACCESS_TOKEN_MINUTES", 15),
		AuthRefreshTokenHours:     getenvInt64("AUTH_REFRESH_TOKEN_HOURS", 24*7),
		AuthRefreshCookieSecure:   getenv("AUTH_REFRESH_COOKIE_SECURE", "false") == "true",
		AuthRefreshCookieSameSite: getenv("AUTH_REFRESH_COOKIE_SAME_SITE", "Lax"),
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvInt64(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
