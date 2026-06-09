package config

import "os"

type Config struct {
	Port           string
	DatabaseURL    string
	RedisURL       string
	JWTSecret      string
	FrontendOrigin string
	MigrationsDir  string
	AutoMigrate    bool
}

func Load() Config {
	return Config{
		Port:           getenv("PORT", "8080"),
		DatabaseURL:    getenv("DATABASE_URL", "postgres://consflow:consflow@127.0.0.1:5432/consflow?sslmode=disable"),
		RedisURL:       getenv("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:      getenv("JWT_SECRET", "consflow-local-dev-secret"),
		FrontendOrigin: getenv("FRONTEND_ORIGIN", "http://localhost:5173"),
		MigrationsDir:  getenv("MIGRATIONS_DIR", "migrations"),
		AutoMigrate:    getenv("AUTO_MIGRATE", "true") != "false",
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
