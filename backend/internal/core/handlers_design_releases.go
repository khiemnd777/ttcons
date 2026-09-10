package core

import (
	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) DesignReleaseWorkspace(c *fiber.Ctx) error {
	workspace, err := h.service.DesignReleaseWorkspace(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, workspace)
}

func (h *Handler) CreateDesignRelease(c *fiber.Ctx) error {
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	release, err := h.service.CreateDesignRelease(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, release)
}

func (h *Handler) UpdateDesignReleaseDraft(c *fiber.Ctx) error {
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	release, err := h.service.UpdateDesignReleaseDraft(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, release)
}

func (h *Handler) UploadDesignReleaseFile(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tệp hồ sơ thiết kế là bắt buộc")
	}
	document, err := h.service.uploadDesignReleaseFile(
		c.Context(), c.Params("id"), c.FormValue("document_type"), c.FormValue("name"), c.FormValue("discussion_batch_id"),
		fileHeader, actorID(c), actorName(c),
	)
	if err != nil {
		return err
	}
	return response.Created(c, document)
}

func (h *Handler) CompleteDesignReleaseUploadBatch(c *fiber.Ctx) error {
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	evidenceIDs, err := idListFromBody(body, "evidence_ids")
	if err != nil {
		return err
	}
	result, err := h.service.CompleteDesignReleaseUploadBatch(c.Context(), c.Params("id"), c.Params("batchId"), evidenceIDs, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) ReleaseDesignRelease(c *fiber.Ctx) error {
	release, err := h.service.ReleaseDesignRelease(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, release)
}

func (h *Handler) HandoffDesignRelease(c *fiber.Ctx) error {
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	handoff, err := h.service.HandoffDesignRelease(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, handoff)
}
