package main

import (
	"context"
	"flag"
	"log"
	"os"

	"consflow/backend/internal/shared/config"
	"consflow/backend/internal/shared/db"
	"consflow/backend/internal/shared/migrations"
)

func main() {
	checkOnly := flag.Bool("check", os.Getenv("MIGRATE_CHECK") == "true", "validate migration files without connecting to the database")
	flag.Parse()

	cfg := config.Load()
	if *checkOnly {
		result, err := migrations.CheckDir(cfg.MigrationsDir)
		if err != nil {
			log.Fatalf("check migrations: %v", err)
		}
		log.Printf("migrations checked=%d", result.Count)
		for _, item := range result.Migrations {
			log.Printf("checked migration %s", item)
		}
		return
	}
	if err := cfg.ValidateMigration(); err != nil {
		log.Fatalf("invalid migration configuration: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.MigrationTimeout)
	defer cancel()

	conn, err := db.Connect(ctx, cfg.DatabaseURL, db.Options{
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	})
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer conn.Close()

	result, err := migrations.RunWithOptions(ctx, conn, cfg.MigrationsDir, migrations.Options{
		LockTimeout: cfg.MigrationLockTimeout,
	})
	if err != nil {
		log.Fatalf("run migrations: %v", err)
	}
	log.Printf("migrations applied=%d skipped=%d", len(result.Applied), len(result.Skipped))
	for _, item := range result.Applied {
		log.Printf("applied migration %s", item)
	}
}
