package core

import "github.com/gofiber/fiber/v2"

func registerSettingsRoutes(protected fiber.Router, h *Handler) {
	protected.Get("/settings", h.Settings)
	protected.Put("/settings", h.UpdateSettings)
	protected.Get("/settings/sample-data/options", h.SampleDataOptions)
	protected.Post("/settings/sample-data/users", h.CreateSampleDataUsers)
	protected.Post("/settings/sample-data/projects", h.CreateSampleDataProject)
	protected.Get("/settings/backup", h.DownloadSettingsBackup)
	protected.Post("/settings/backup/preview", h.PreviewSettingsBackupRestore)
	protected.Post("/settings/backup/restore", h.RestoreSettingsBackup)
}
