package core

import (
	"consflow/backend/internal/shared/response"
	"encoding/json"
	fiber "github.com/gofiber/fiber/v2"
)

func (h *Handler) ProjectDailyReports(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"operation.view"}); err != nil {
		return err
	}
	rows, err := h.service.ListNativeDailyReports(c.Context(), c.Params("id"), actorID(c), NativeDailyReportFilter{DateFrom: c.Query("date_from"), DateTo: c.Query("date_to")})
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateDailyReport(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"operation.report"}); err != nil {
		return err
	}
	req := ConstructionDailyReportCreateRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu báo cáo thi công không hợp lệ")
	}
	row, err := h.service.CreateDailyReport(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) UpdateDailyReport(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "operation_daily_reports", c.Params("id"), []string{"operation.report"}); err != nil {
		return err
	}
	req := ConstructionDailyReportCreateRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu báo cáo thi công không hợp lệ")
	}
	row, err := h.service.UpdateDailyReport(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) UploadDailyReportEvidence(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "operation_daily_reports", c.Params("id"), []string{"operation.report"}); err != nil {
		return err
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tệp minh chứng là bắt buộc")
	}
	req := WorkItemEvidenceRequest{
		EvidenceType: c.FormValue("evidence_type"),
		DocumentType: c.FormValue("document_type"),
		Name:         c.FormValue("name"),
		Title:        c.FormValue("title"),
		Description:  c.FormValue("description"),
		Caption:      c.FormValue("caption"),
		RevisionNo:   c.FormValue("revision_no"),
		Visibility:   c.FormValue("visibility"),
	}
	row, err := h.service.AddDailyReportEvidenceUpload(c.Context(), c.Params("id"), c.FormValue("work_item_id"), req, fileHeader, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ProjectConstructionWorkspace(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"operation.view", "quality.view", "rework.view"}); err != nil {
		return err
	}
	workspace, err := h.service.ProjectConstructionWorkspace(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, workspace)
}

func (h *Handler) ProjectConstructionVariationContext(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if err := h.requireProjectPermission(c, projectID, []string{"operation.view", "contract.view", "contract.variation.propose"}); err != nil {
		return err
	}
	context, err := h.service.ProjectConstructionVariationContext(c.Context(), projectID)
	if err != nil {
		return err
	}
	canPropose, err := h.service.repo.UserHasAnyProjectPermission(c.Context(), actorID(c), projectID, []string{"contract.variation.propose"})
	if err != nil {
		return err
	}
	context["can_propose"] = canPropose
	return response.OK(c, context)
}

func (h *Handler) ProposeConstructionVariation(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"contract.variation.propose"}); err != nil {
		return err
	}
	req := ContractVariationRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đề xuất phát sinh không hợp lệ")
	}
	row, err := h.service.ProposeConstructionVariation(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ProjectConstructionHandoff(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"operation.view", "planning.view", "construction.handoff.manage"}); err != nil {
		return err
	}
	result, err := h.service.ProjectConstructionHandoff(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) ActivateConstructionHandoff(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"construction.handoff.manage"}); err != nil {
		return err
	}
	req := ConstructionHandoffActivateRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu bàn giao thi công không hợp lệ")
	}
	result, err := h.service.ActivateConstructionHandoff(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, result)
}

func (h *Handler) CreateConstructionCheckTemplate(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"construction.template.manage"}); err != nil {
		return err
	}
	req := ConstructionCheckTemplateRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu mẫu kiểm tra không hợp lệ")
	}
	row, err := h.service.CreateConstructionCheckTemplate(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) CreateConstructionCheck(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"construction.check.request"}); err != nil {
		return err
	}
	req := ConstructionCheckRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu kiểm tra không hợp lệ")
	}
	row, err := h.service.CreateConstructionCheck(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) UpdateConstructionCheck(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "construction_checks", c.Params("id"), []string{"construction.check.edit"}); err != nil {
		return err
	}
	req := ConstructionCheckUpdateRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu cập nhật kiểm tra không hợp lệ")
	}
	row, err := h.service.UpdateConstructionCheck(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CompleteConstructionCheck(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "construction_checks", c.Params("id"), []string{"construction.check.perform"}); err != nil {
		return err
	}
	req := ConstructionCheckCompleteRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Kết quả kiểm tra không hợp lệ")
	}
	row, err := h.service.CompleteConstructionCheck(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CreateConstructionAction(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"construction.action.create"}); err != nil {
		return err
	}
	req := ConstructionActionRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu việc cần xử lý không hợp lệ")
	}
	row, err := h.service.CreateConstructionAction(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) UpdateConstructionAction(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "construction_action_items", c.Params("id"), []string{"construction.action.edit"}); err != nil {
		return err
	}
	req := ConstructionActionUpdateRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu cập nhật việc cần xử lý không hợp lệ")
	}
	row, err := h.service.UpdateConstructionAction(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) SubmitConstructionActionResolution(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "construction_action_items", c.Params("id"), []string{"construction.action.resolve"}); err != nil {
		return err
	}
	req := ConstructionActionResolutionRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Kết quả khắc phục không hợp lệ")
	}
	row, err := h.service.SubmitConstructionActionResolution(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) VerifyConstructionAction(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "construction_action_items", c.Params("id"), []string{"construction.action.verify"}); err != nil {
		return err
	}
	req := ConstructionActionVerifyRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Kết quả xác nhận không hợp lệ")
	}
	row, err := h.service.VerifyConstructionAction(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ApproveConstructionActionImpact(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "construction_action_items", c.Params("id"), []string{"construction.action.verify", "replanning.approve"}); err != nil {
		return err
	}
	row, err := h.service.ApproveConstructionActionImpact(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ProjectIssues(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"operation.view"}); err != nil {
		return err
	}
	rows, err := h.service.ProjectIssues(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateIssue(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"operation.issue_create"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.CreateIssue(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) UpdateIssue(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "issues", c.Params("id"), []string{"operation.issue_update"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.UpdateIssue(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}
