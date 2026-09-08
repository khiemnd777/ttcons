package core

import (
	"context"
	"database/sql"
	"log"
	"sync"

	"consflow/backend/internal/shared/config"
	"consflow/backend/internal/shared/mailer"
	"consflow/backend/internal/shared/middleware"
	sharedstorage "consflow/backend/internal/shared/storage"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

func RegisterRoutes(app *fiber.App, db *sql.DB, redisClient, redisCacheClient *redis.Client, cfg config.Config, storageRegistry *sharedstorage.Registry, cacheObserver ProjectExplorerCacheObserver, settingsStore *SystemSettingsStore) (func(), error) {
	repo := NewRepository(db)
	repo.ConfigureSystemSettings(settingsStore)
	service := NewService(repo, cfg.JWTSecret)
	if err := service.ConfigureStorage(storageRegistry, cfg.StorageWriteProvider(), cfg.MaxUploadBytes); err != nil {
		return nil, err
	}
	service.ConfigureAuth(cfg.AuthAccessTokenMinutes, cfg.AuthRefreshTokenHours, cfg.AuthRefreshCookieSecure, cfg.AuthRefreshCookieSameSite)
	recoverySender, recoveryConfigErr := newAccountRecoverySender(cfg)
	if recoveryConfigErr != nil {
		log.Printf("level=warn component=account_recovery operation=configure status=unavailable error=%q", recoveryConfigErr.Error())
	} else if _, preview := recoverySender.(*mailer.PreviewSender); preview {
		log.Printf("level=warn component=account_recovery operation=configure status=ready provider=preview delivery=discarded")
	} else if recoverySender != nil {
		log.Printf("level=info component=account_recovery operation=configure status=ready provider=smtp")
	}
	service.ConfigureAccountRecovery(recoverySender != nil, recoverySender, cfg.FrontendOrigin, cfg.AuthRecoveryTokenTTL, cfg.AuthRecoveryVerifyTTL)
	service.ConfigureActivityStreaming(redisClient, cfg.SSEBrokerChannel, cfg.RedisOperationTimeout, cfg.SSEMaxConnections, cfg.SSEMaxConnectionsPerIP, cfg.SSEMaxConnectionsPerActor, cfg.SSEVisibilityTimeout, cfg.SSEMaxConnectionLifetime)
	service.ConfigureProjectExplorerCache(redisCacheClient, ProjectExplorerCacheConfig{
		Mode:             cfg.ProjectExplorerCacheMode,
		OperationTimeout: cfg.ProjectExplorerCacheTimeout,
		IndexTTL:         cfg.ProjectExplorerCacheIndexTTL,
		RowTTL:           cfg.ProjectExplorerCacheRowTTL,
		LockTTL:          cfg.ProjectExplorerCacheLockTTL,
		MaxValueBytes:    cfg.ProjectExplorerCacheMaxBytes,
	}, cacheObserver)
	stopProjectExplorerDispatcher := service.StartProjectExplorerInvalidationDispatcher(context.Background())
	stopNotificationDispatcher := service.StartNotificationDispatcher(context.Background())
	h := NewHandler(service)
	h.ConfigureReadiness(cfg.ReadinessTimeout, db.PingContext)

	rateLimiter := middleware.NewRateLimiter(redisClient, middleware.RateLimiterConfig{
		Enabled:          cfg.RateLimitEnabled,
		Enforce:          cfg.RateLimitMode == "enforce",
		RedisTimeout:     cfg.RedisOperationTimeout,
		RedisMaxInFlight: cfg.RateLimitRedisMaxInFlight,
		LocalMaxKeys:     cfg.RateLimitLocalMaxKeys,
	})
	concurrencyGuards := middleware.NewConcurrencyGuards(cfg.UploadConcurrency, cfg.ExportConcurrency, cfg.ReportConcurrency)

	api := app.Group("/api", rateLimiter.LimitIP(rateLimitPolicies(cfg.RateLimitIP)))
	api.Get("/health", h.Health)
	api.Get("/ready", h.Ready)
	api.Post("/auth/login", h.Login)
	api.Post("/auth/refresh", h.Refresh)
	api.Post("/auth/logout", h.Logout)
	api.Post("/auth/password/forgot", h.ForgotPassword)
	api.Post("/auth/password/reset", h.ResetPassword)
	api.Post("/auth/recovery-email/verify", h.VerifyRecoveryEmail)

	protected := api.Group("",
		middleware.AuthClaimsRequired(cfg.JWTSecret),
		rateLimiter.LimitActor(rateLimitPolicies(cfg.RateLimitActor)),
		middleware.AuthSessionRequired(repo),
		concurrencyGuards.Middleware(),
	)
	protected.Get("/auth/me", h.Me)
	protected.Put("/auth/password", h.ChangeOwnPassword)
	protected.Use(middleware.InitialPasswordChanged(repo))
	protected.Put("/auth/profile", h.UpdateOwnProfile)
	protected.Post("/auth/avatar/upload", h.UploadOwnAvatar)
	protected.Get("/auth/avatar", h.GetOwnAvatar)
	protected.Put("/auth/recovery-email", h.UpdateRecoveryEmail)
	protected.Delete("/auth/recovery-email", h.RemoveRecoveryEmail)

	registerProjectsRoutes(protected, h)
	registerWorkflowRoutes(protected, h)
	registerCommercialRoutes(protected, h)
	registerQualityRoutes(protected, h)
	registerProcurementRoutes(protected, h)
	registerMaterialsRoutes(protected, h)
	registerConstructionRoutes(protected, h)
	registerAcceptanceHandoverRoutes(protected, h)
	registerFinanceRoutes(protected, h)
	registerAdminRoutes(protected, h)
	registerReportingRoutes(protected, h)
	registerSettingsRoutes(protected, h)

	var closeOnce sync.Once
	return func() {
		closeOnce.Do(func() {
			stopNotificationDispatcher()
			stopProjectExplorerDispatcher()
			service.CloseActivityStreaming()
		})
	}, nil
}

func newAccountRecoverySender(cfg config.Config) (mailer.Sender, error) {
	if !cfg.AuthRecoveryEnabled {
		return nil, nil
	}
	if err := cfg.AccountRecoveryConfigurationError(); err != nil {
		if cfg.AppEnv == "development" || cfg.AppEnv == "test" {
			return mailer.NewPreview(), nil
		}
		return nil, err
	}
	return mailer.NewSMTP(mailer.SMTPConfig{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword,
		FromAddress: cfg.SMTPFromAddress, FromName: cfg.SMTPFromName, TLSMode: cfg.SMTPTLSMode, Timeout: cfg.SMTPTimeout,
	})
}

func rateLimitPolicies(values config.RateLimitPolicies) middleware.RateLimitPolicies {
	return middleware.RateLimitPolicies{
		middleware.RequestClassAuth:    rateLimitPolicy(values.Auth),
		middleware.RequestClassGeneral: rateLimitPolicy(values.General),
		middleware.RequestClassUpload:  rateLimitPolicy(values.Upload),
		middleware.RequestClassExport:  rateLimitPolicy(values.Export),
		middleware.RequestClassSSE:     rateLimitPolicy(values.SSE),
	}
}

func rateLimitPolicy(value config.RateLimitPolicy) middleware.RateLimitPolicy {
	return middleware.RateLimitPolicy{Limit: value.Limit, Burst: value.Burst, Window: value.Window}
}
