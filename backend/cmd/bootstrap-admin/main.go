package main

import (
	"context"
	"log"

	"consflow/backend/internal/shared/bootstrapadmin"
	"consflow/backend/internal/shared/config"
	"consflow/backend/internal/shared/db"
)

func main() {
	runtimeConfig := config.Load()
	if err := runtimeConfig.ValidateMigration(); err != nil {
		log.Fatalf("invalid Admin bootstrap database configuration: %v", err)
	}
	bootstrapConfig, err := bootstrapadmin.ConfigFromEnv()
	if err != nil {
		log.Fatalf("invalid Admin bootstrap identity configuration: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), runtimeConfig.MigrationTimeout)
	defer cancel()
	database, err := db.Connect(ctx, runtimeConfig.DatabaseURL, db.Options{
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	})
	if err != nil {
		log.Fatalf("connect postgres for Admin bootstrap: %v", err)
	}
	defer database.Close()

	result, err := bootstrapadmin.Run(ctx, database, bootstrapConfig)
	if err != nil {
		log.Fatalf("bootstrap Admin: %v", err)
	}
	status := "already_exists"
	if result.Created {
		status = "created"
	}
	log.Printf("event=admin_bootstrap status=%s user_id=%s", status, result.UserID)
}
