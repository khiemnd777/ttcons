package core

import (
	"consflow/backend/internal/shared/response"
	"encoding/json"
	"fmt"
	fiber "github.com/gofiber/fiber/v2"
	"net/url"
	"strconv"
	"strings"
)

func (h *Handler) ProjectWorkItems(c *fiber.Ctx) error {
	items, err := h.service.ProjectWorkItems(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, items)
}

func (h *Handler) MyWorkItems(c *fiber.Ctx) error {
	return h.myWorkPage(c, false)
}

func (h *Handler) MyWorkQueue(c *fiber.Ctx) error {
	return h.myWorkPage(c, true)
}

func (h *Handler) myWorkPage(c *fiber.Ctx, unified bool) error {
	limit := 0
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			return validationError("limit không hợp lệ", Record{"limit": rawLimit})
		}
		limit = parsed
	}
	items, err := h.service.MyWorkItems(c.Context(), MyWorkItemsFilter{
		Unified:   unified,
		EntryKind: c.Query("entry_kind"),
		Queue:     c.Query("queue"),
		ProjectID: c.Query("project_id"),
		Status:    c.Query("status"),
		Deadline:  c.Query("deadline"),
		WorkType:  c.Query("work_type"),
		Priority:  c.Query("priority"),
		Search:    c.Query("search"),
		Sort:      c.Query("sort"),
		Bucket:    c.Query("bucket"),
		Cursor:    c.Query("cursor"),
		Limit:     limit,
	}, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, items)
}

func (h *Handler) MyWorkNavigationSummary(c *fiber.Ctx) error {
	summary, err := h.service.MyWorkNavigationSummary(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, summary)
}

func (h *Handler) MyAccountingWork(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting_handoff.view"}); err != nil {
		return err
	}
	page, err := h.service.MyAccountingWork(c.Context(), MyAccountingWorkFilter{
		ProjectID: c.Query("project_id"),
		Status:    c.Query("status"),
		Search:    c.Query("search"),
		Cursor:    c.Query("cursor"),
		Limit:     c.QueryInt("limit", 0),
	}, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, page)
}

func (h *Handler) MyWorkScopeSummary(c *fiber.Ctx) error {
	summary, err := h.service.MyWorkScopeSummary(c.Context(), actorID(c), c.Query("project_id"))
	if err != nil {
		return err
	}
	return response.OK(c, summary)
}

func (h *Handler) MyWorkflowActions(c *fiber.Ctx) error {
	actions, err := h.service.MyWorkflowActions(c.Context(), MyWorkflowActionsFilter{
		Queue:     c.Query("queue"),
		ProjectID: c.Query("project_id"),
		Search:    c.Query("search"),
	}, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, actions)
}

func (h *Handler) MyProjectDiscussions(c *fiber.Ctx) error {
	limit := 0
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			return validationError("limit không hợp lệ", Record{"limit": rawLimit})
		}
		limit = parsed
	}
	page, err := h.service.MyProjectDiscussions(c.Context(), ProjectDiscussionFilter{
		ProjectID: c.Query("project_id"),
		Search:    c.Query("search"),
		Cursor:    c.Query("cursor"),
		Limit:     limit,
	}, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, page)
}

func (h *Handler) MyActivityAttention(c *fiber.Ctx) error {
	items, err := h.service.MyActivityAttention(c.Context(), ActivityAttentionFilter{
		Queue:     c.Query("queue"),
		ProjectID: c.Query("project_id"),
		Limit:     c.QueryInt("limit", 80),
	}, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, items)
}

func (h *Handler) MyActivityAttentionSummary(c *fiber.Ctx) error {
	summary, err := h.service.MyActivityAttentionSummary(c.Context(), c.Query("project_id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, summary)
}

func (h *Handler) ProjectOperations(c *fiber.Ctx) error {
	operations, err := h.service.ProjectOperations(c.Context(), c.Params("id"), actorID(c), ProjectOperationsFilter{
		View: c.Query("view"),
	})
	if err != nil {
		return err
	}
	return response.OK(c, operations)
}

func (h *Handler) ProjectEvidence(c *fiber.Ctx) error {
	page, err := h.service.ProjectEvidencePage(c.Context(), c.Params("id"), ProjectEvidenceFilter{
		EvidenceType: c.Query("evidence_type"),
		Status:       c.Query("status"),
		WorkItemID:   c.Query("work_item_id"),
		SubmittedBy:  c.Query("submitted_by"),
		Visibility:   c.Query("visibility"),
		DateFrom:     c.Query("date_from"),
		DateTo:       c.Query("date_to"),
		Search:       c.Query("search"),
		Cursor:       c.Query("cursor"),
		Limit:        c.QueryInt("limit", 25),
	}, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, page)
}

func (h *Handler) ProjectActivity(c *fiber.Ctx) error {
	page, err := h.service.ProjectActivityPage(c.Context(), c.Params("id"), ProjectActivityFilter{
		TransitionsOnly: c.QueryBool("transitions_only", false),
		ScopeType:       c.Query("scope_type"),
		ScopeID:         c.Query("scope_id"),
		ActivityType:    c.Query("activity_type"),
		ActorID:         c.Query("actor_id"),
		Visibility:      c.Query("visibility"),
		Kind:            c.Query("kind"),
		Attention:       c.Query("attention"),
		Search:          c.Query("search"),
		Cursor:          c.Query("cursor"),
		Limit:           c.QueryInt("limit", 30),
	}, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, page)
}

func (h *Handler) ProjectActivityOptions(c *fiber.Ctx) error {
	options, err := h.service.ProjectActivityOptions(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, options)
}

func (h *Handler) AddProjectActivity(c *fiber.Ctx) error {
	req := WorkItemActivityRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu hoạt động không hợp lệ")
		}
	}
	activity, err := h.service.AddProjectActivity(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, activity)
}

func (h *Handler) MarkProjectActivityRead(c *fiber.Ctx) error {
	result, err := h.service.MarkProjectActivityRead(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) MarkProjectDiscussionReadThrough(c *fiber.Ctx) error {
	req := ProjectDiscussionReadRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đánh dấu đã đọc không hợp lệ")
	}
	result, err := h.service.MarkProjectDiscussionReadThrough(c.Context(), c.Params("id"), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) GetWorkItem(c *fiber.Ctx) error {
	item, err := h.service.WorkItemDetail(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) GetWorkItemForm(c *fiber.Ctx) error {
	form, err := h.service.WorkItemForm(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, form)
}

func (h *Handler) StartWorkItem(c *fiber.Ctx) error { return h.workItemAction(c, "start") }

func (h *Handler) CompleteWorkItem(c *fiber.Ctx) error { return h.workItemAction(c, "complete") }

func (h *Handler) BlockWorkItem(c *fiber.Ctx) error { return h.workItemAction(c, "block") }

func (h *Handler) ResolveWorkItemBlocker(c *fiber.Ctx) error {
	return h.workItemAction(c, "resolve-blocker")
}

func (h *Handler) RequestWorkItemReview(c *fiber.Ctx) error {
	return h.workItemAction(c, "request-review")
}

func (h *Handler) RequestWorkItemApproval(c *fiber.Ctx) error {
	return h.workItemAction(c, "request-approval")
}

func (h *Handler) ApproveWorkItem(c *fiber.Ctx) error { return h.workItemAction(c, "approve") }

func (h *Handler) RejectWorkItem(c *fiber.Ctx) error { return h.workItemAction(c, "reject") }

func (h *Handler) RequestWorkItemChange(c *fiber.Ctx) error {
	return h.workItemAction(c, "request-change")
}

func (h *Handler) workItemAction(c *fiber.Ctx, action string) error {
	req := WorkItemActionRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu gửi lên không hợp lệ")
		}
	}
	item, err := h.service.WorkItemAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) AddWorkItemEvidence(c *fiber.Ctx) error {
	req := WorkItemEvidenceRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu minh chứng không hợp lệ")
		}
	}
	evidence, err := h.service.AddWorkItemEvidence(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, evidence)
}

func (h *Handler) UploadWorkItemEvidence(c *fiber.Ctx) error {
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
	if rawPayload := c.FormValue("payload"); rawPayload != "" {
		payload := Record{}
		if err := json.Unmarshal([]byte(rawPayload), &payload); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Nội dung minh chứng không hợp lệ")
		}
		req.Payload = payload
	}
	evidence, err := h.service.AddWorkItemEvidenceUpload(c.Context(), c.Params("id"), req, fileHeader, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, evidence)
}

func (h *Handler) AddWorkItemActivity(c *fiber.Ctx) error {
	req := WorkItemActivityRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu hoạt động không hợp lệ")
		}
	}
	activity, err := h.service.AddWorkItemActivity(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, activity)
}

func (h *Handler) GetSubcontractorReconciliation(c *fiber.Ctx) error {
	var revision []int
	if value := c.Query("revision"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return validationError("Phiên bản đối soát không hợp lệ")
		}
		revision = []int{n}
	}
	reconciliation, err := h.service.SubcontractorReconciliation(c.Context(), c.Params("id"), actorID(c), revision...)
	if err != nil {
		return err
	}
	return response.OK(c, reconciliation)
}

func (h *Handler) SaveSubcontractorReconciliation(c *fiber.Ctx) error {
	req := SubcontractorReconciliationRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đối soát thầu phụ không hợp lệ")
		}
	}
	reconciliation, err := h.service.SaveSubcontractorReconciliation(c.Context(), c.Params("id"), req, actorID(c), actorName(c), c.Get("If-Match"))
	if err != nil {
		return err
	}
	return response.OK(c, reconciliation)
}

func (h *Handler) SubmitSubcontractorReconciliation(c *fiber.Ctx) error {
	req := SubcontractorReconciliationRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đối soát thầu phụ không hợp lệ")
		}
	}
	reconciliation, err := h.service.SubmitSubcontractorReconciliation(c.Context(), c.Params("id"), req, actorID(c), actorName(c), c.Get("If-Match"))
	if err != nil {
		return err
	}
	return response.OK(c, reconciliation)
}

func (h *Handler) GetOwnerClientReconciliation(c *fiber.Ctx) error {
	reconciliation, err := h.service.OwnerClientReconciliation(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, reconciliation)
}

func (h *Handler) SaveOwnerClientReconciliation(c *fiber.Ctx) error {
	req := OwnerClientReconciliationRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đối soát chủ đầu tư không hợp lệ")
		}
	}
	reconciliation, err := h.service.SaveOwnerClientReconciliation(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, reconciliation)
}

func (h *Handler) SubmitOwnerClientReconciliation(c *fiber.Ctx) error {
	req := OwnerClientReconciliationRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đối soát chủ đầu tư không hợp lệ")
		}
	}
	reconciliation, err := h.service.SubmitOwnerClientReconciliation(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, reconciliation)
}

func (h *Handler) GetMaterialSnapshot(c *fiber.Ctx) error {
	snapshot, err := h.service.MaterialSnapshot(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, snapshot)
}

func (h *Handler) SaveMaterialSnapshot(c *fiber.Ctx) error {
	req := MaterialSnapshotRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu ghi nhận vật tư không hợp lệ")
		}
	}
	snapshot, err := h.service.SaveMaterialSnapshot(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, snapshot)
}

func (h *Handler) SubmitMaterialSnapshot(c *fiber.Ctx) error {
	req := MaterialSnapshotRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu ghi nhận vật tư không hợp lệ")
		}
	}
	snapshot, err := h.service.SubmitMaterialSnapshot(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, snapshot)
}

func (h *Handler) GetSurplusHandlings(c *fiber.Ctx) error {
	handlings, err := h.service.SurplusHandlings(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, handlings)
}

func (h *Handler) CreateSurplusHandling(c *fiber.Ctx) error {
	req := SurplusHandlingRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xử lý vật tư dư không hợp lệ")
		}
	}
	handling, err := h.service.CreateSurplusHandling(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, handling)
}

func (h *Handler) UpdateSurplusHandling(c *fiber.Ctx) error {
	req := SurplusHandlingRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xử lý vật tư dư không hợp lệ")
		}
	}
	handling, err := h.service.UpdateSurplusHandling(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, handling)
}

func (h *Handler) SubmitSurplusHandling(c *fiber.Ctx) error {
	handling, err := h.service.SubmitSurplusHandling(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, handling)
}

func (h *Handler) WorkItemIncomingAcceptances(c *fiber.Ctx) error {
	rows, err := h.service.WorkItemIncomingAcceptances(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) WorkItemOutgoingAcceptance(c *fiber.Ctx) error {
	rows, err := h.service.WorkItemOutgoingAcceptance(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) SaveWorkflowFormDraft(c *fiber.Ctx) error {
	req := WorkflowFormSaveRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu workflow form không hợp lệ")
		}
	}
	form, err := h.service.SaveWorkflowFormDraft(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, form)
}

func (h *Handler) SubmitWorkflowForm(c *fiber.Ctx) error {
	req := WorkflowFormSaveRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu workflow form không hợp lệ")
		}
	}
	form, err := h.service.SubmitWorkflowForm(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, form)
}

func (h *Handler) AmendWorkflowForm(c *fiber.Ctx) error {
	req := WorkflowFormAmendRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu chỉnh sửa hồ sơ nghiệp vụ không hợp lệ")
		}
	}
	form, err := h.service.AmendWorkflowForm(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, form)
}

func (h *Handler) SignWorkflowForm(c *fiber.Ctx) error {
	req := WorkflowFormSignRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu chữ ký workflow form không hợp lệ")
		}
	}
	form, err := h.service.SignWorkflowForm(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, form)
}

func (h *Handler) SealAssets(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"seal.use", "seal.manage"}); err != nil {
		return err
	}
	assets, err := h.service.SealAssets(c.Context(), c.QueryBool("include_archived", false), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, assets)
}

func (h *Handler) UploadSealAsset(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"seal.manage"}); err != nil {
		return err
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tệp ảnh con dấu là bắt buộc")
	}
	asset, err := h.service.CreateSealAsset(c.Context(), c.FormValue("name"), fileHeader, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, asset)
}

func (h *Handler) ArchiveSealAsset(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"seal.manage"}); err != nil {
		return err
	}
	asset, err := h.service.ArchiveSealAsset(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, asset)
}

func (h *Handler) GetSealAssetFile(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"seal.use", "seal.manage"}); err != nil {
		return err
	}
	file, err := h.service.SealAssetFile(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return sendProtectedFile(c, file, "con-dau.png")
}

func (h *Handler) ApplyWorkflowFormSeal(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"seal.use"}); err != nil {
		return err
	}
	req := WorkflowFormSealApplyRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu con dấu workflow form không hợp lệ")
		}
	}
	form, err := h.service.ApplyWorkflowFormSeal(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, form)
}

func (h *Handler) GetWorkflowFormSealApplicationVisual(c *fiber.Ctx) error {
	file, err := h.service.WorkflowFormSealApplicationVisual(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return sendProtectedFile(c, file, "con-dau-ho-so.png")
}

func (h *Handler) SaveWorkflowFormPDFConfirmationPreference(c *fiber.Ctx) error {
	req := WorkflowFormPDFConfirmationPreferenceRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Lựa chọn xác nhận hiển thị trong PDF không hợp lệ")
		}
	}
	form, err := h.service.SaveWorkflowFormPDFConfirmationPreference(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, form)
}

func (h *Handler) GenerateWorkflowFormPDFExport(c *fiber.Ctx) error {
	form, err := h.service.GenerateWorkflowFormPDFExport(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, form)
}

func (h *Handler) MarkActivityRead(c *fiber.Ctx) error {
	item, err := h.service.MarkActivityRead(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) ResolveActivity(c *fiber.Ctx) error {
	req := ActivityResolutionRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xử lý hoạt động không hợp lệ")
		}
	}
	item, err := h.service.ResolveActivity(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) SetActivityReaction(c *fiber.Ctx) error {
	req := ActivityReactionRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu phản ứng không hợp lệ")
	}
	item, err := h.service.SetActivityReaction(c.Context(), c.Params("id"), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) UpdateActivityMessage(c *fiber.Ctx) error {
	req := ActivityMessageUpdateRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu chỉnh sửa tin nhắn không hợp lệ")
	}
	item, err := h.service.UpdateActivityMessage(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) WithdrawActivityMessage(c *fiber.Ctx) error {
	item, err := h.service.WithdrawActivityMessage(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) GetEvidenceDetail(c *fiber.Ctx) error {
	evidence, err := h.service.EvidenceDetail(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	return response.OK(c, evidence)
}

func (h *Handler) GetEvidenceFile(c *fiber.Ctx) error {
	file, err := h.service.EvidenceFile(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return sendProtectedFile(c, file, "minh-chung")
}

func (h *Handler) GetWorkflowFormSignatureVisual(c *fiber.Ctx) error {
	file, err := h.service.WorkflowFormSignatureVisual(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return sendProtectedFile(c, file, "signature.png")
}

func (h *Handler) GetWorkflowFormPDFExportFile(c *fiber.Ctx) error {
	file, err := h.service.WorkflowFormPDFExportFile(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return sendProtectedFile(c, file, "workflow-form.pdf")
}

func (h *Handler) DeleteWorkflowFormPDFExport(c *fiber.Ctx) error {
	form, err := h.service.DeleteWorkflowFormPDFExport(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, form)
}

func sendProtectedFile(c *fiber.Ctx, file EvidenceFileResult, fallbackName string) error {
	c.Set(fiber.HeaderContentType, file.MIMEType)
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("inline; filename=%q; filename*=UTF-8''%s", fallbackName, url.PathEscape(file.Filename)))
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	size := int(file.Size)
	if file.Size >= 0 && int64(size) == file.Size {
		if err := c.SendStream(file.Reader, size); err != nil {
			_ = file.Reader.Close()
			return err
		}
		return nil
	}
	if err := c.SendStream(file.Reader); err != nil {
		_ = file.Reader.Close()
		return err
	}
	return nil
}

func (h *Handler) AcceptEvidence(c *fiber.Ctx) error {
	return h.evidenceAction(c, "accept")
}

func (h *Handler) RejectEvidence(c *fiber.Ctx) error {
	return h.evidenceAction(c, "reject")
}

func (h *Handler) evidenceAction(c *fiber.Ctx, action string) error {
	req := ActionRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu thao tác minh chứng không hợp lệ")
		}
	}
	evidence, err := h.service.EvidenceAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, evidence)
}

func (h *Handler) StartRunNode(c *fiber.Ctx) error {
	if err := h.requireRunNodePermission(c, c.Params("id")); err != nil {
		return err
	}
	node, err := h.service.repo.QueryOne(c.Context(), `UPDATE pipeline_run_nodes SET status='in_progress', started_at=COALESCE(started_at, now()) WHERE id=$1 RETURNING *`, c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, node)
}

func (h *Handler) SubmitRunStep(c *fiber.Ctx) error {
	if err := h.requireRunStepPermission(c, c.Params("id")); err != nil {
		return err
	}
	req := WorkItemActionRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu gửi lên không hợp lệ")
		}
	}
	step, err := h.service.SubmitRunStep(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, step)
}

func (h *Handler) RequestNodeApproval(c *fiber.Ctx) error {
	if err := h.requireRunNodePermission(c, c.Params("id")); err != nil {
		return err
	}
	body, _ := bodyMap(c)
	req, err := h.service.RequestNodeApproval(c.Context(), c.Params("id"), actorID(c), actorName(c), stringValue(body["reason"]))
	if err != nil {
		return err
	}
	return response.Created(c, req)
}

func (h *Handler) AdvanceRunNode(c *fiber.Ctx) error {
	if err := h.requireRunNodePermission(c, c.Params("id")); err != nil {
		return err
	}
	run, err := h.service.AdvanceRunNode(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, run)
}

func (h *Handler) BlockRunNode(c *fiber.Ctx) error {
	if err := h.requireRunNodePermission(c, c.Params("id")); err != nil {
		return err
	}
	body, _ := bodyMap(c)
	node, err := h.service.BlockRunNode(c.Context(), c.Params("id"), stringValue(body["reason"]), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, node)
}

func (h *Handler) ListApprovals(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"project.view", "operation.view", "procurement.view", "accounting.view", "report.view"}); err != nil {
		return err
	}
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	rows, err := h.service.repo.Query(c.Context(), fmt.Sprintf(`
		SELECT ar.*, p.name AS project_name, u.full_name AS requested_by_name
		FROM approval_requests ar
		LEFT JOIN projects p ON p.id=ar.project_id
		LEFT JOIN users u ON u.id=ar.requested_by
		WHERE (p.id IS NULL OR p.status <> 'cancelled')
		  AND (p.id IS NOT NULL AND %s)
		ORDER BY ar.created_at DESC
	`, scopeSQL), scopeArgs...)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) PendingApprovals(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"project.view", "operation.view", "procurement.view", "accounting.view", "report.view"}); err != nil {
		return err
	}
	rows, err := h.service.PendingApprovals(c.Context(), PendingApprovalFilter{
		ProjectID:     c.Query("project_id"),
		DecidableOnly: c.QueryBool("decidable_only", false),
	}, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) Approve(c *fiber.Ctx) error { return h.approvalAction(c, "approve") }

func (h *Handler) Reject(c *fiber.Ctx) error { return h.approvalAction(c, "reject") }

func (h *Handler) RequestChange(c *fiber.Ctx) error { return h.approvalAction(c, "request-change") }

func (h *Handler) approvalAction(c *fiber.Ctx, action string) error {
	body, _ := bodyMap(c)
	req, err := h.service.ApprovalAction(c.Context(), c.Params("id"), action, stringValue(body["comment"]), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, req)
}
