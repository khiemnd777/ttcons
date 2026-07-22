package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"

	"consflow/backend/internal/shared/config"
	"consflow/backend/internal/shared/db"
	sharedstorage "consflow/backend/internal/shared/storage"
)

func main() {
	apply := flag.Bool("apply", false, "copy verified local objects to S3 and atomically move database metadata; default is dry-run")
	flag.Parse()

	cfg := config.Load()
	if err := cfg.ValidateStorageMigration(); err != nil {
		log.Fatalf("invalid storage migration configuration: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.StorageMigrationTimeout)
	defer cancel()

	connection, err := db.Connect(ctx, cfg.DatabaseURL, db.Options{
		MaxOpenConns:     4,
		MaxIdleConns:     2,
		StatementTimeout: cfg.DBStatementTimeout,
		LockTimeout:      cfg.DBLockTimeout,
	})
	if err != nil {
		log.Fatalf("connect postgres for storage migration: %v", err)
	}
	defer connection.Close()

	runtime, err := sharedstorage.NewRuntime(ctx, sharedstorage.RuntimeConfig{
		WriteProvider: sharedstorage.S3Provider,
		LocalRoot:     cfg.UploadDir,
		S3: sharedstorage.S3Config{
			Bucket:           cfg.StorageS3Bucket,
			Region:           cfg.StorageS3Region,
			Endpoint:         cfg.StorageS3Endpoint,
			UsePathStyle:     cfg.StorageS3UsePathStyle,
			OperationTimeout: cfg.StorageOperationTimeout,
		},
	})
	if err != nil {
		log.Fatalf("configure storage migration providers: %v", err)
	}
	defer runtime.Close()

	repository, err := sharedstorage.NewSQLMigrationRepository(connection)
	if err != nil {
		log.Fatalf("configure storage migration repository: %v", err)
	}
	localStore, err := runtime.Registry.Store(sharedstorage.LocalProvider)
	if err != nil {
		log.Fatalf("resolve local storage provider: %v", err)
	}
	s3Store, err := runtime.Registry.Store(sharedstorage.S3Provider)
	if err != nil {
		log.Fatalf("resolve S3 storage provider: %v", err)
	}

	summary, migrationErr := sharedstorage.MigrateLocalToS3(ctx, repository, localStore, s3Store, sharedstorage.MigrationOptions{Apply: *apply})
	rawSummary, marshalErr := json.Marshal(summary)
	if marshalErr != nil {
		log.Fatalf("marshal storage migration summary: %v", marshalErr)
	}
	log.Printf("event=storage_migration_summary result=%s", rawSummary)
	if migrationErr != nil {
		log.Fatalf("storage migration failed: %v", migrationErr)
	}
}
