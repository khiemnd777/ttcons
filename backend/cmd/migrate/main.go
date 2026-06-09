package main

import (
	"context"
	"log"
	"time"

	"consflow/backend/internal/shared/config"
	"consflow/backend/internal/shared/db"
	"consflow/backend/internal/shared/migrations"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	conn, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer conn.Close()

	result, err := migrations.Run(ctx, conn, cfg.MigrationsDir)
	if err != nil {
		log.Fatalf("run migrations: %v", err)
	}
	log.Printf("migrations applied=%d skipped=%d", len(result.Applied), len(result.Skipped))
	for _, item := range result.Applied {
		log.Printf("applied migration %s", item)
	}
}
