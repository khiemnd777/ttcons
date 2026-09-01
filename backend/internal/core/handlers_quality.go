package core

import (
	"consflow/backend/internal/shared/response"
	"encoding/json"
	fiber "github.com/gofiber/fiber/v2"
)

func (h *Handler) CreateQualityITP(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"quality.create"}); err != nil {
		return err
	}
	var req QualityITPRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu ITP không hợp lệ")
	}
	row, err := h.service.CreateQualityITP(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ReviseQualityITP(c *fiber.Ctx) error {
	if err := h.requireQualityChildPermission(c, "itp", c.Params("id"), []string{"quality.create"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.ReviseQualityITP(c.Context(), c.Params("id"), actionComment(req), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) SubmitQualityITP(c *fiber.Ctx) error {
	return h.qualityITPAction(c, "submit", []string{"quality.review"})
}

func (h *Handler) ApproveQualityITP(c *fiber.Ctx) error {
	return h.qualityITPAction(c, "approve", []string{"quality.approve"})
}

func (h *Handler) ActivateQualityITP(c *fiber.Ctx) error {
	return h.qualityITPAction(c, "activate", []string{"quality.approve"})
}

func (h *Handler) qualityITPAction(c *fiber.Ctx, action string, permissions []string) error {
	if err := h.requireQualityChildPermission(c, "itp", c.Params("id"), permissions); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.QualityITPAction(c.Context(), c.Params("id"), action, actionComment(req), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CreateQualityInspectionRequest(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"quality.create"}); err != nil {
		return err
	}
	var req QualityInspectionRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu RFI không hợp lệ")
	}
	row, err := h.service.CreateQualityInspectionRequest(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) SubmitQualityInspectionRequest(c *fiber.Ctx) error {
	return h.qualityInspectionAction(c, "submit", []string{"quality.create"})
}

func (h *Handler) ScheduleQualityInspectionRequest(c *fiber.Ctx) error {
	return h.qualityInspectionAction(c, "schedule", []string{"quality.inspect"})
}

func (h *Handler) InspectQualityRequest(c *fiber.Ctx) error {
	return h.qualityInspectionAction(c, "inspect", []string{"quality.inspect"})
}

func (h *Handler) qualityInspectionAction(c *fiber.Ctx, action string, permissions []string) error {
	if err := h.requireQualityChildPermission(c, "rfi", c.Params("id"), permissions); err != nil {
		return err
	}
	var req QualityInspectionDecisionRequest
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu inspection không hợp lệ")
		}
	}
	row, err := h.service.QualityInspectionAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CreateQualityNCR(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"quality.ncr_manage"}); err != nil {
		return err
	}
	var req QualityNCRRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu NCR không hợp lệ")
	}
	row, err := h.service.CreateQualityNCR(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) SubmitQualityNCRRootCause(c *fiber.Ctx) error {
	return h.qualityNCRAction(c, "root-cause")
}

func (h *Handler) SubmitQualityNCRCorrective(c *fiber.Ctx) error {
	return h.qualityNCRAction(c, "corrective")
}

func (h *Handler) LinkQualityNCRReinspection(c *fiber.Ctx) error {
	return h.qualityNCRAction(c, "reinspection")
}

func (h *Handler) CloseQualityNCR(c *fiber.Ctx) error { return h.qualityNCRAction(c, "close") }

func (h *Handler) qualityNCRAction(c *fiber.Ctx, action string) error {
	if err := h.requireQualityChildPermission(c, "ncr", c.Params("id"), []string{"quality.ncr_manage"}); err != nil {
		return err
	}
	var req QualityNCRActionRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xử lý NCR không hợp lệ")
	}
	row, err := h.service.QualityNCRAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CreateRecurringPlan(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"quality.recurring_manage"}); err != nil {
		return err
	}
	var req ConstructionRecurringPlanRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu lịch lặp không hợp lệ")
	}
	row, err := h.service.CreateRecurringPlan(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) GenerateRecurringOccurrences(c *fiber.Ctx) error {
	if err := h.requireQualityChildPermission(c, "recurring_plan", c.Params("id"), []string{"quality.recurring_manage"}); err != nil {
		return err
	}
	body, _ := bodyMap(c)
	row, err := h.service.GenerateRecurringOccurrences(c.Context(), c.Params("id"), stringValue(body["through_date"]), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CompleteRecurringOccurrence(c *fiber.Ctx) error {
	return h.recurringOccurrenceAction(c, "complete")
}

func (h *Handler) SkipRecurringOccurrence(c *fiber.Ctx) error {
	return h.recurringOccurrenceAction(c, "skip")
}

func (h *Handler) recurringOccurrenceAction(c *fiber.Ctx, action string) error {
	if err := h.requireQualityChildPermission(c, "occurrence", c.Params("id"), []string{"quality.recurring_manage"}); err != nil {
		return err
	}
	var req ConstructionRecurringOccurrenceRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu occurrence không hợp lệ")
	}
	row, err := h.service.RecurringOccurrenceAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) requireQualityChildPermission(c *fiber.Ctx, kind, id string, permissions []string) error {
	queries := map[string]string{
		"itp":            `SELECT project_id FROM quality_itp_plans WHERE id=$1`,
		"rfi":            `SELECT project_id FROM quality_inspection_requests WHERE id=$1`,
		"ncr":            `SELECT project_id FROM quality_ncrs WHERE id=$1`,
		"recurring_plan": `SELECT project_id FROM construction_recurring_plans WHERE id=$1`,
		"occurrence":     `SELECT project_id FROM construction_recurring_occurrences WHERE id=$1`,
	}
	query := queries[kind]
	if query == "" {
		return validationError("Loại hồ sơ QA/QC không hợp lệ")
	}
	row, err := h.service.repo.QueryOne(c.Context(), query, id)
	if err != nil {
		return err
	}
	return h.requireProjectPermission(c, cleanID(row["project_id"]), permissions)
}

func (h *Handler) ProjectAcceptances(c *fiber.Ctx) error {
	rows, err := h.service.ProjectAcceptances(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateAcceptance(c *fiber.Ctx) error {
	req := AcceptanceRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu nghiệm thu không hợp lệ")
		}
	}
	record, err := h.service.CreateAcceptance(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, record)
}

func (h *Handler) GetAcceptance(c *fiber.Ctx) error {
	acc, err := h.service.GetAcceptance(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, acc)
}

func (h *Handler) UpdateAcceptance(c *fiber.Ctx) error {
	req := AcceptanceRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu nghiệm thu không hợp lệ")
		}
	}
	record, err := h.service.UpdateAcceptance(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, record)
}

func (h *Handler) SubmitAcceptance(c *fiber.Ctx) error {
	record, err := h.service.SubmitAcceptance(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, record)
}

func (h *Handler) PassAcceptance(c *fiber.Ctx) error { return h.acceptanceAction(c, "pass") }

func (h *Handler) ConditionalPassAcceptance(c *fiber.Ctx) error {
	return h.acceptanceAction(c, "conditional_pass")
}

func (h *Handler) FailAcceptance(c *fiber.Ctx) error { return h.acceptanceAction(c, "fail") }

func (h *Handler) acceptanceAction(c *fiber.Ctx, result string) error {
	permissions := []string{"acceptance.approve"}
	if result == "fail" {
		permissions = []string{"acceptance.reject"}
	}
	if err := h.requireRecordProjectPermission(c, "acceptance_records", c.Params("id"), permissions); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	acc, err := h.service.AcceptanceAction(c.Context(), c.Params("id"), result, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, acc)
}

func (h *Handler) CreateReworkFromAcceptance(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "acceptance_records", c.Params("id"), []string{"rework.create", "acceptance.reject"}); err != nil {
		return err
	}
	rework, err := h.service.CreateReworkFromAcceptance(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, rework)
}

func (h *Handler) ProjectReworks(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"rework.view"}); err != nil {
		return err
	}
	rows, err := h.service.ProjectReworks(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateRework(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"rework.create"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.CreateRework(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ApproveRework(c *fiber.Ctx) error {
	return h.reworkStatusAction(c, "approved")
}

func (h *Handler) CompleteRework(c *fiber.Ctx) error {
	return h.reworkStatusAction(c, "done")
}

func (h *Handler) ProjectReplannings(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"replanning.view"}); err != nil {
		return err
	}
	rows, err := h.service.ProjectReplannings(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateReplanning(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"replanning.create"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.CreateReplanning(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ApproveReplanning(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "replanning_requests", c.Params("id"), []string{"replanning.approve"}); err != nil {
		return err
	}
	row, err := h.service.ApproveReplanning(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) reworkStatusAction(c *fiber.Ctx, status string) error {
	if err := h.requireRecordProjectPermission(c, "rework_requests", c.Params("id"), []string{"rework.approve"}); err != nil {
		return err
	}
	row, err := h.service.UpdateReworkStatus(c.Context(), c.Params("id"), status, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ProjectHandover(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"handover.view"}); err != nil {
		return err
	}
	rows, err := h.service.ProjectHandover(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) HandoverOptions(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"handover.view", "handover.create"}); err != nil {
		return err
	}
	options, err := h.service.HandoverOptions(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, options)
}

func (h *Handler) CreateHandover(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"handover.create"}); err != nil {
		return err
	}
	var req HandoverCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return err
	}
	row, err := h.service.CreateHandover(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) GetHandover(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "handover_records", c.Params("id"), []string{"handover.view"}); err != nil {
		return err
	}
	row, err := h.service.HandoverDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) SubmitHandover(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "handover_records", c.Params("id"), []string{"handover.create", "handover.verify"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.SubmitHandover(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CheckHandover(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "handover_records", c.Params("id"), []string{"handover.verify", "handover.approve"}); err != nil {
		return err
	}
	check, err := h.service.HandoverCheck(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, check)
}

func (h *Handler) UpdateHandoverChecklistItem(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "handover_checklist_items", c.Params("id"), []string{"handover.verify"}); err != nil {
		return err
	}
	var req HandoverChecklistItemRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return err
	}
	row, err := h.service.UpdateHandoverChecklistItem(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CompleteHandover(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "handover_records", c.Params("id"), []string{"handover.approve"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	done, err := h.service.CompleteHandover(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, done)
}
