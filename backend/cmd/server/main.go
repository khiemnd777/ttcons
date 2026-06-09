package main

import (
	"context"
	"log"
	"time"

	"consflow/backend/internal/core"
	"consflow/backend/internal/shared/cache"
	"consflow/backend/internal/shared/config"
	"consflow/backend/internal/shared/db"
	"consflow/backend/internal/shared/migrations"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer conn.Close()

	if cfg.AutoMigrate {
		result, err := migrations.Run(ctx, conn, cfg.MigrationsDir)
		if err != nil {
			log.Fatalf("run migrations: %v", err)
		}
		log.Printf("database migrations applied=%d skipped=%d", len(result.Applied), len(result.Skipped))
	}

	redisClient := cache.Connect(context.Background(), cfg.RedisURL)
	defer redisClient.Close()

	app := fiber.New(fiber.Config{
		AppName:      "consflow",
		ErrorHandler: core.ErrorHandler,
	})
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.FrontendOrigin + ",http://localhost:5173,http://127.0.0.1:5173",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowCredentials: true,
	}))
	app.Use(fiberlogger.New())

	core.RegisterRoutes(app, conn, redisClient, cfg)

	log.Printf("consflow backend listening on :%s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
