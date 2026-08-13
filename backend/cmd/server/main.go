package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"consflow/backend/internal/core"
	"consflow/backend/internal/shared/cache"
	"consflow/backend/internal/shared/config"
	"consflow/backend/internal/shared/db"
	"consflow/backend/internal/shared/migrations"
	"consflow/backend/internal/shared/observability"
	sharedstorage "consflow/backend/internal/shared/storage"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	storageRuntime, err := sharedstorage.NewRuntime(context.Background(), sharedstorage.RuntimeConfig{
		WriteProvider: cfg.StorageWriteProvider(),
		LocalRoot:     cfg.StoragePath,
		S3: sharedstorage.S3Config{
			Bucket:           cfg.StorageS3Bucket,
			Region:           cfg.StorageS3Region,
			Endpoint:         cfg.StorageS3Endpoint,
			UsePathStyle:     cfg.StorageS3UsePathStyle,
			OperationTimeout: cfg.StorageOperationTimeout,
		},
	})
	if err != nil {
		log.Fatalf("configure storage: %v", err)
	}
	defer func() {
		if err := storageRuntime.Close(); err != nil {
			log.Printf("event=storage_close error=%v", err)
		}
	}()
	preflightContext, cancelPreflight := context.WithTimeout(context.Background(), cfg.StorageOperationTimeout)
	writeStore, storeErr := storageRuntime.Registry.Store(storageRuntime.WriteProvider)
	if storeErr == nil {
		storeErr = sharedstorage.VerifyWriteAccess(preflightContext, writeStore)
	}
	cancelPreflight()
	if storeErr != nil {
		log.Fatalf("verify active storage provider: %v", storeErr)
	}
	log.Printf("event=storage_preflight provider=%s status=ready", storageRuntime.WriteProvider)
	if cfg.AutoMigrate {
		migrationContext, cancelMigration := context.WithTimeout(context.Background(), cfg.MigrationTimeout)
		migrationConn, err := db.Connect(migrationContext, cfg.DatabaseURL, db.Options{
			MaxOpenConns:    1,
			MaxIdleConns:    1,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: time.Minute,
		})
		if err != nil {
			cancelMigration()
			log.Fatalf("connect postgres for migrations: %v", err)
		}
		result, migrationErr := migrations.RunWithOptions(migrationContext, migrationConn, cfg.MigrationsDir, migrations.Options{
			LockTimeout: cfg.MigrationLockTimeout,
		})
		closeErr := migrationConn.Close()
		cancelMigration()
		if migrationErr != nil {
			log.Fatalf("run migrations: %v", migrationErr)
		}
		if closeErr != nil {
			log.Printf("close migration database connection: %v", closeErr)
		}
		log.Printf("event=database_migrations applied=%d skipped=%d", len(result.Applied), len(result.Skipped))
	}

	connectContext, cancelConnect := context.WithTimeout(context.Background(), 10*time.Second)
	conn, err := db.Connect(connectContext, cfg.DatabaseURL, db.Options{
		MaxOpenConns:     cfg.DBMaxOpenConns,
		MaxIdleConns:     cfg.DBMaxIdleConns,
		ConnMaxLifetime:  cfg.DBConnMaxLifetime,
		ConnMaxIdleTime:  cfg.DBConnMaxIdleTime,
		StatementTimeout: cfg.DBStatementTimeout,
		LockTimeout:      cfg.DBLockTimeout,
	})
	cancelConnect()
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer conn.Close()

	redisClient, err := cache.Connect(context.Background(), cfg.RedisURL, cfg.RedisOperationTimeout)
	if err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer redisClient.Close()

	runtimeMetrics := observability.New(conn)
	app := fiber.New(serverConfig(cfg))
	app.Use(runtimeMetrics.Middleware())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.FrontendOrigin + ",http://localhost:5173,http://127.0.0.1:5173",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowCredentials: true,
		ExposeHeaders:    "Retry-After, X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Source, X-RateLimit-Observed",
	}))
	app.Use(requestid.New())
	app.Use(fiberlogger.New(fiberlogger.Config{
		Format: "event=http_request time=${time} status=${status} method=${method} path=${path} req_id=${locals:requestid} latency=${latency}\n",
	}))
	app.Get("/metrics", runtimeMetrics.Handler())

	closeRoutes, err := core.RegisterRoutes(app, conn, redisClient, cfg, storageRuntime.Registry)
	if err != nil {
		log.Fatalf("register routes: %v", err)
	}
	defer closeRoutes()

	log.Printf("event=backend_listen port=%s", cfg.Port)
	listenErr := make(chan error, 1)
	go func() {
		listenErr <- app.Listen(":" + cfg.Port)
	}()

	signalContext, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	select {
	case err := <-listenErr:
		if err != nil {
			log.Printf("event=backend_listener_stopped error=%v", err)
		}
		return
	case <-signalContext.Done():
		log.Printf("event=backend_shutdown_requested")
	}

	// Close the activity hub first so long-lived SSE writers exit before Fiber drains connections.
	closeRoutes()
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()
	if err := app.ShutdownWithContext(shutdownContext); err != nil {
		log.Printf("event=backend_forced_shutdown error=%v", err)
	}
	select {
	case err := <-listenErr:
		if err != nil {
			log.Printf("event=backend_listener_shutdown error=%v", err)
		}
	case <-shutdownContext.Done():
		log.Printf("event=backend_shutdown_deadline_exceeded")
	}
}

func serverConfig(cfg config.Config) fiber.Config {
	return fiber.Config{
		AppName:                      "consflow",
		BodyLimit:                    int(cfg.MaxUploadBytes + 1024*1024),
		DisablePreParseMultipartForm: true,
		ReadTimeout:                  cfg.HTTPReadTimeout,
		WriteTimeout:                 cfg.HTTPWriteTimeout,
		IdleTimeout:                  cfg.HTTPIdleTimeout,
		ProxyHeader:                  fiber.HeaderXForwardedFor,
		EnableTrustedProxyCheck:      true,
		TrustedProxies:               cfg.TrustedProxies,
		ErrorHandler:                 observability.ErrorHandler(core.ErrorHandler),
	}
}
