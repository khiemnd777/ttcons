package core

import (
	"encoding/json"
	"fmt"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Health(c *fiber.Ctx) error {
	return response.OK(c, Record{"status": "ok", "service": "consflow"})
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đăng nhập không hợp lệ")
	}
	data, err := h.service.Login(c.Context(), req)
	if err != nil {
		return response.Fail(c, fiber.StatusUnauthorized, "INVALID_CREDENTIALS", "Email hoặc mật khẩu không đúng")
	}
	return response.OK(c, data, "Đăng nhập thành công")
}

func (h *Handler) Refresh(c *fiber.Ctx) error {
	user, err := h.service.Me(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	roles := []string{}
	if raw, ok := user["roles"].([]string); ok {
		roles = raw
	}
	token, err := h.service.signToken(fmt.Sprint(user["id"]), fmt.Sprint(user["email"]), roles)
	if err != nil {
		return err
	}
	return response.OK(c, Record{"token": token})
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	return response.OK(c, Record{"logged_out": true})
}

func (h *Handler) Me(c *fiber.Ctx) error {
	user, err := h.service.Me(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, user)
}

func (h *Handler) ListProjects(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"project.view"}); err != nil {
		return err
	}
	rows, err := h.service.ListProjects(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateProject(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"project.create"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	project, err := h.service.CreateProject(c.Context(), body)
	if err != nil {
		return err
	}
	return response.Created(c, project)
}

func (h *Handler) GetProject(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"project.view"}); err != nil {
		return err
	}
	project, err := h.service.ProjectDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, project)
}

func (h *Handler) UpdateProject(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"project.update"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	project, err := h.service.UpdateProject(c.Context(), c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.OK(c, project)
}

func (h *Handler) DeleteProject(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"project.delete"}); err != nil {
		return err
	}
	if err := h.service.DeleteProject(c.Context(), c.Params("id")); err != nil {
		return err
	}
	return response.OK(c, Record{"deleted": true})
}

func (h *Handler) ListTemplates(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.view"}); err != nil {
		return err
	}
	rows, err := h.service.ListTemplates(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateTemplate(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.create"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	template, err := h.service.CreateTemplate(c.Context(), body, actorID(c))
	if err != nil {
		return err
	}
	return response.Created(c, template)
}

func (h *Handler) GetTemplate(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.view"}); err != nil {
		return err
	}
	template, err := h.service.TemplateDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, template)
}

func (h *Handler) UpdateTemplate(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	template, err := h.service.UpdateTemplate(c.Context(), c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.OK(c, template)
}

func (h *Handler) DeleteTemplate(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	if err := h.service.DeleteTemplate(c.Context(), c.Params("id")); err != nil {
		return err
	}
	return response.OK(c, Record{"deleted": true})
}

func (h *Handler) PublishTemplate(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.publish"}); err != nil {
		return err
	}
	template, err := h.service.PublishTemplate(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, template)
}

func (h *Handler) CloneTemplate(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.create"}); err != nil {
		return err
	}
	template, err := h.service.CloneTemplate(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.Created(c, template)
}

func (h *Handler) AddTemplateNode(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	body["template_id"] = c.Params("id")
	node, err := h.service.repo.Insert(c.Context(), "pipeline_template_nodes", templateNodeColumns(), body)
	if err != nil {
		return err
	}
	return response.Created(c, node)
}

func (h *Handler) UpdateTemplateNode(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	node, err := h.service.repo.Update(c.Context(), "pipeline_template_nodes", templateNodeColumns(), c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.OK(c, node)
}

func (h *Handler) DeleteTemplateNode(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	if err := h.service.repo.Exec(c.Context(), `DELETE FROM pipeline_template_nodes WHERE id=$1`, c.Params("id")); err != nil {
		return err
	}
	return response.OK(c, Record{"deleted": true})
}

func (h *Handler) AddTemplateStep(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	body["node_id"] = c.Params("id")
	step, err := h.service.repo.Insert(c.Context(), "pipeline_template_steps", templateStepColumns(), body)
	if err != nil {
		return err
	}
	return response.Created(c, step)
}

func (h *Handler) UpdateTemplateStep(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	step, err := h.service.repo.Update(c.Context(), "pipeline_template_steps", templateStepColumns(), c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.OK(c, step)
}

func (h *Handler) DeleteTemplateStep(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	if err := h.service.repo.Exec(c.Context(), `DELETE FROM pipeline_template_steps WHERE id=$1`, c.Params("id")); err != nil {
		return err
	}
	return response.OK(c, Record{"deleted": true})
}

func (h *Handler) AddTemplateEdge(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	body["template_id"] = c.Params("id")
	if err := prepareTemplateEdgeBody(body, false); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	}
	edge, err := h.service.repo.Insert(c.Context(), "pipeline_template_edges", edgeColumns(), body)
	if err != nil {
		return err
	}
	return response.Created(c, edge)
}

func (h *Handler) PipelineEdgeConditions(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.view"}); err != nil {
		return err
	}
	return response.OK(c, PipelineEdgeConditionCatalog())
}

func (h *Handler) UpdateTemplateEdge(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	if err := prepareTemplateEdgeBody(body, true); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	}
	edge, err := h.service.repo.Update(c.Context(), "pipeline_template_edges", edgeColumns(), c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.OK(c, edge)
}

func (h *Handler) DeleteTemplateEdge(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	if err := h.service.repo.Exec(c.Context(), `DELETE FROM pipeline_template_edges WHERE id=$1`, c.Params("id")); err != nil {
		return err
	}
	return response.OK(c, Record{"deleted": true})
}

func (h *Handler) StartPipeline(c *fiber.Ctx) error {
	body, _ := bodyMap(c)
	run, err := h.service.StartPipeline(c.Context(), c.Params("id"), stringValue(body["template_id"]), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, run)
}

func (h *Handler) GetPipelineRun(c *fiber.Ctx) error {
	run, err := h.service.PipelineRun(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, run)
}

func (h *Handler) ProjectWorkItems(c *fiber.Ctx) error {
	items, err := h.service.ProjectWorkItems(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, items)
}

func (h *Handler) MyWorkItems(c *fiber.Ctx) error {
	items, err := h.service.MyWorkItems(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, items)
}

func (h *Handler) ProjectOperations(c *fiber.Ctx) error {
	operations, err := h.service.ProjectOperations(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, operations)
}

func (h *Handler) GetWorkItem(c *fiber.Ctx) error {
	item, err := h.service.WorkItemDetail(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) StartWorkItem(c *fiber.Ctx) error    { return h.workItemAction(c, "start") }
func (h *Handler) CompleteWorkItem(c *fiber.Ctx) error { return h.workItemAction(c, "complete") }
func (h *Handler) BlockWorkItem(c *fiber.Ctx) error    { return h.workItemAction(c, "block") }
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
func (h *Handler) RejectWorkItem(c *fiber.Ctx) error  { return h.workItemAction(c, "reject") }
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
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu evidence không hợp lệ")
		}
	}
	evidence, err := h.service.AddWorkItemEvidence(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, evidence)
}

func (h *Handler) AddWorkItemActivity(c *fiber.Ctx) error {
	req := WorkItemActivityRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu activity không hợp lệ")
		}
	}
	activity, err := h.service.AddWorkItemActivity(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, activity)
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
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu evidence action không hợp lệ")
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
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	rows, err := h.service.repo.Query(c.Context(), fmt.Sprintf(`
		SELECT ar.*, p.name AS project_name
		FROM approval_requests ar
		LEFT JOIN projects p ON p.id=ar.project_id
		WHERE ar.status='pending' AND (p.id IS NULL OR p.status <> 'cancelled')
		  AND (p.id IS NOT NULL AND %s)
		ORDER BY ar.created_at DESC
	`, scopeSQL), scopeArgs...)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) Approve(c *fiber.Ctx) error       { return h.approvalAction(c, "approve") }
func (h *Handler) Reject(c *fiber.Ctx) error        { return h.approvalAction(c, "reject") }
func (h *Handler) RequestChange(c *fiber.Ctx) error { return h.approvalAction(c, "request-change") }

func (h *Handler) approvalAction(c *fiber.Ctx, action string) error {
	body, _ := bodyMap(c)
	req, err := h.service.ApprovalAction(c.Context(), c.Params("id"), action, stringValue(body["comment"]), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, req)
}

func (h *Handler) ProjectBOQ(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"boq.view"}); err != nil {
		return err
	}
	rows, err := h.service.repo.Query(c.Context(), `
		SELECT b.*, m.name AS material_name
		FROM boq_items b
		LEFT JOIN materials m ON m.id=b.material_id
		WHERE b.project_id=$1 AND b.status <> 'deleted'
		ORDER BY b.category, b.created_at
	`, c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateBOQ(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"boq.create"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	body["project_id"] = c.Params("id")
	body["created_by"] = actorID(c)
	item, err := h.service.repo.Insert(c.Context(), "boq_items", boqColumns(), body)
	if err != nil {
		return err
	}
	return response.Created(c, item)
}

func (h *Handler) UpdateBOQ(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "boq_items", c.Params("id"), []string{"boq.update"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	item, err := h.service.repo.Update(c.Context(), "boq_items", boqColumns(), c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) SubmitBOQ(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"boq.update"}); err != nil {
		return err
	}
	if err := h.service.repo.Exec(c.Context(), `UPDATE boq_items SET status='submitted', updated_at=now() WHERE project_id=$1 AND status='draft'`, c.Params("id")); err != nil {
		return err
	}
	return response.OK(c, Record{"submitted": true})
}

func (h *Handler) ApproveBOQ(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"boq.approve"}); err != nil {
		return err
	}
	if err := h.service.repo.Exec(c.Context(), `UPDATE boq_items SET status='approved', approved_by=NULLIF($2,'')::uuid, updated_at=now() WHERE project_id=$1 AND status IN ('submitted','draft')`, c.Params("id"), actorID(c)); err != nil {
		return err
	}
	return response.OK(c, Record{"approved": true})
}

func (h *Handler) GenerateProcurementFromBOQ(c *fiber.Ctx) error {
	req, err := h.service.GenerateProcurementFromBOQ(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, req)
}

func (h *Handler) ListProcurement(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"procurement.view"}); err != nil {
		return err
	}
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	rows, err := h.service.repo.Query(c.Context(), fmt.Sprintf(`
		SELECT pr.*, p.name AS project_name
		FROM procurement_requests pr
		JOIN projects p ON p.id=pr.project_id
		WHERE pr.status <> 'cancelled' AND p.status <> 'cancelled' AND %s
		ORDER BY pr.created_at DESC
	`, scopeSQL), scopeArgs...)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateProcurement(c *fiber.Ctx) error {
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	if err := h.requireProjectPermission(c, cleanID(body["project_id"]), []string{"procurement.create"}); err != nil {
		return err
	}
	if body["request_code"] == nil {
		body["request_code"] = fmt.Sprintf("PR-%d", timeNowUnix())
	}
	body["requested_by"] = actorID(c)
	req, err := h.service.repo.Insert(c.Context(), "procurement_requests", procurementColumns(), body)
	if err != nil {
		return err
	}
	return response.Created(c, req)
}

func (h *Handler) GetProcurement(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "procurement_requests", c.Params("id"), []string{"procurement.view"}); err != nil {
		return err
	}
	req, err := h.service.repo.GetProcurementDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, req)
}

func (h *Handler) UpdateProcurement(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "procurement_requests", c.Params("id"), []string{"procurement.create"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	req, err := h.service.repo.Update(c.Context(), "procurement_requests", procurementColumns(), c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.OK(c, req)
}

func (h *Handler) SubmitProcurement(c *fiber.Ctx) error  { return h.procurementAction(c, "submit") }
func (h *Handler) ApproveProcurement(c *fiber.Ctx) error { return h.procurementAction(c, "approve") }
func (h *Handler) OrderProcurement(c *fiber.Ctx) error   { return h.procurementAction(c, "order") }
func (h *Handler) ReceiveProcurement(c *fiber.Ctx) error { return h.procurementAction(c, "receive") }

func (h *Handler) procurementAction(c *fiber.Ctx, action string) error {
	permissions := map[string][]string{
		"submit":  {"procurement.create"},
		"approve": {"procurement.approve"},
		"order":   {"procurement.order"},
		"receive": {"procurement.receive"},
	}[action]
	if err := h.requireRecordProjectPermission(c, "procurement_requests", c.Params("id"), permissions); err != nil {
		return err
	}
	actionReq, err := actionRequest(c)
	if err != nil {
		return err
	}
	req, err := h.service.ProcurementAction(c.Context(), c.Params("id"), action, actionReq, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, req)
}

func (h *Handler) ListMaterials(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"warehouse.view", "procurement.view"}); err != nil {
		return err
	}
	return h.list(c, `SELECT * FROM materials WHERE active ORDER BY category, name`)
}
func (h *Handler) CreateMaterial(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"warehouse.adjust"}); err != nil {
		return err
	}
	return h.create(c, "materials", materialColumns())
}
func (h *Handler) ListWarehouses(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"warehouse.view"}); err != nil {
		return err
	}
	return h.list(c, `SELECT * FROM warehouses WHERE active ORDER BY type, name`)
}
func (h *Handler) CreateWarehouse(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"warehouse.adjust"}); err != nil {
		return err
	}
	return h.create(c, "warehouses", warehouseColumns())
}

func (h *Handler) Stock(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"warehouse.view"}); err != nil {
		return err
	}
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	rows, err := h.service.repo.Query(c.Context(), fmt.Sprintf(`
		SELECT m.id AS material_id, m.code, m.name, m.unit, m.category,
			COALESCE(SUM(CASE
				WHEN sm.movement_type IN ('receive','return_to_warehouse') THEN sm.quantity
				WHEN sm.movement_type IN ('issue','transfer','loss') THEN -sm.quantity
				ELSE 0
			END),0) AS stock_quantity
		FROM materials m
		LEFT JOIN stock_movements sm ON sm.material_id=m.id
			AND (
				sm.project_id IS NULL
				OR EXISTS (
					SELECT 1
					FROM projects p
					WHERE p.id=sm.project_id AND p.status <> 'cancelled' AND %s
				)
			)
		WHERE m.active
		GROUP BY m.id
		ORDER BY m.category, m.name
	`, scopeSQL), scopeArgs...)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) StockReceive(c *fiber.Ctx) error  { return h.stockAction(c, "receive") }
func (h *Handler) StockIssue(c *fiber.Ctx) error    { return h.stockAction(c, "issue") }
func (h *Handler) StockTransfer(c *fiber.Ctx) error { return h.stockAction(c, "transfer") }
func (h *Handler) StockReturnWarehouse(c *fiber.Ctx) error {
	return h.stockAction(c, "return_to_warehouse")
}
func (h *Handler) StockReturnSubcontractor(c *fiber.Ctx) error {
	return h.stockAction(c, "return_to_subcontractor")
}

func (h *Handler) stockAction(c *fiber.Ctx, movementType string) error {
	var req StockActionRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu nhập xuất kho không hợp lệ")
	}
	req.MovementType = movementType
	permissions := map[string][]string{
		"receive":                 {"warehouse.receive"},
		"issue":                   {"warehouse.issue"},
		"transfer":                {"warehouse.transfer"},
		"return_to_warehouse":     {"warehouse.surplus_handle"},
		"return_to_subcontractor": {"warehouse.surplus_handle"},
	}[movementType]
	if cleanID(req.ProjectID) != "" {
		if err := h.requireProjectPermission(c, req.ProjectID, permissions); err != nil {
			return err
		}
	} else if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	movement, err := h.service.CreateStockMovement(c.Context(), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, movement)
}

func (h *Handler) MaterialBalances(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"warehouse.view"}); err != nil {
		return err
	}
	rows, err := h.service.repo.ProjectMaterialBalances(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) ProjectDailyReports(c *fiber.Ctx) error {
	return h.projectList(c, "operation_daily_reports", []string{"operation.view"})
}
func (h *Handler) CreateDailyReport(c *fiber.Ctx) error {
	return h.projectCreate(c, "operation_daily_reports", dailyReportColumns(), []string{"operation.report"})
}
func (h *Handler) ProjectIssues(c *fiber.Ctx) error {
	return h.projectList(c, "issues", []string{"operation.view"})
}
func (h *Handler) CreateIssue(c *fiber.Ctx) error {
	return h.projectCreate(c, "issues", issueColumns(), []string{"operation.issue_create"})
}
func (h *Handler) UpdateIssue(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "issues", c.Params("id"), []string{"operation.issue_update"}); err != nil {
		return err
	}
	return h.update(c, "issues", issueColumns())
}

func (h *Handler) ProjectAcceptances(c *fiber.Ctx) error {
	return h.projectList(c, "acceptance_records", []string{"acceptance.view"})
}
func (h *Handler) CreateAcceptance(c *fiber.Ctx) error {
	return h.projectCreate(c, "acceptance_records", acceptanceColumns(), []string{"acceptance.create"})
}
func (h *Handler) GetAcceptance(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "acceptance_records", c.Params("id"), []string{"acceptance.view"}); err != nil {
		return err
	}
	acc, err := h.service.repo.GetByID(c.Context(), "acceptance_records", c.Params("id"))
	if err != nil {
		return err
	}
	items, _ := h.service.repo.Query(c.Context(), `SELECT * FROM acceptance_checklist_items WHERE acceptance_id=$1 ORDER BY created_at`, c.Params("id"))
	acc["checklist"] = items
	return response.OK(c, acc)
}
func (h *Handler) PassAcceptance(c *fiber.Ctx) error { return h.acceptanceAction(c, "pass") }
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
	return h.projectList(c, "rework_requests", []string{"rework.view"})
}
func (h *Handler) CreateRework(c *fiber.Ctx) error {
	return h.projectCreate(c, "rework_requests", reworkColumns(), []string{"rework.create"})
}
func (h *Handler) ApproveRework(c *fiber.Ctx) error {
	return h.setStatus(c, "rework_requests", "approved", []string{"rework.approve"})
}
func (h *Handler) CompleteRework(c *fiber.Ctx) error {
	return h.setStatus(c, "rework_requests", "done", []string{"rework.approve"})
}
func (h *Handler) ProjectReplannings(c *fiber.Ctx) error {
	return h.projectList(c, "replanning_requests", []string{"replanning.view"})
}
func (h *Handler) CreateReplanning(c *fiber.Ctx) error {
	return h.projectCreate(c, "replanning_requests", replanningColumns(), []string{"replanning.create"})
}
func (h *Handler) ApproveReplanning(c *fiber.Ctx) error {
	return h.setStatus(c, "replanning_requests", "approved", []string{"replanning.approve"})
}

func (h *Handler) ProjectHandover(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"handover.view"}); err != nil {
		return err
	}
	rows, err := h.service.repo.Query(c.Context(), `SELECT * FROM handover_records WHERE project_id=$1 AND status <> 'deleted' ORDER BY created_at DESC`, c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}
func (h *Handler) CreateHandover(c *fiber.Ctx) error {
	return h.projectCreate(c, "handover_records", handoverColumns(), []string{"handover.create"})
}
func (h *Handler) CheckHandover(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "handover_records", c.Params("id"), []string{"handover.view"}); err != nil {
		return err
	}
	check, err := h.service.HandoverCheck(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, check)
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

func (h *Handler) ProjectBudget(c *fiber.Ctx) error {
	return h.projectList(c, "accounting_budgets", []string{"accounting.budget_view"})
}
func (h *Handler) CreateBudget(c *fiber.Ctx) error {
	return h.projectCreate(c, "accounting_budgets", budgetColumns(), []string{"accounting.budget_approve"})
}
func (h *Handler) ApproveBudget(c *fiber.Ctx) error {
	return h.setStatus(c, "accounting_budgets", "approved", []string{"accounting.budget_approve"})
}
func (h *Handler) ListPayables(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting.view", "accounting.payable_view"}); err != nil {
		return err
	}
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	return h.list(c, fmt.Sprintf(`SELECT pay.*, p.name AS project_name FROM payables pay JOIN projects p ON p.id=pay.project_id WHERE pay.status <> 'cancelled' AND p.status <> 'cancelled' AND %s ORDER BY pay.created_at DESC`, scopeSQL), scopeArgs...)
}
func (h *Handler) CreatePayable(c *fiber.Ctx) error {
	if err := h.requireBodyProjectPermission(c, []string{"accounting.payable_approve"}); err != nil {
		return err
	}
	return h.create(c, "payables", payableColumns())
}
func (h *Handler) ApprovePayable(c *fiber.Ctx) error {
	return h.payableAction(c, "approve")
}
func (h *Handler) PayPayable(c *fiber.Ctx) error {
	return h.payableAction(c, "pay")
}
func (h *Handler) ListReceivables(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting.view", "accounting.receivable_view"}); err != nil {
		return err
	}
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	return h.list(c, fmt.Sprintf(`SELECT rec.*, p.name AS project_name FROM receivables rec JOIN projects p ON p.id=rec.project_id WHERE rec.status <> 'cancelled' AND p.status <> 'cancelled' AND %s ORDER BY rec.created_at DESC`, scopeSQL), scopeArgs...)
}
func (h *Handler) CreateReceivable(c *fiber.Ctx) error {
	if err := h.requireBodyProjectPermission(c, []string{"accounting.receivable_approve"}); err != nil {
		return err
	}
	return h.create(c, "receivables", receivableColumns())
}
func (h *Handler) ApproveReceivable(c *fiber.Ctx) error {
	return h.receivableAction(c, "approve")
}
func (h *Handler) ReceiveReceivable(c *fiber.Ctx) error {
	return h.receivableAction(c, "receive")
}
func (h *Handler) ListPayments(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting.view", "accounting.payment_create", "accounting.payment_approve"}); err != nil {
		return err
	}
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	return h.list(c, fmt.Sprintf(`SELECT pay.*, p.name AS project_name FROM payments pay JOIN projects p ON p.id=pay.project_id WHERE pay.status <> 'cancelled' AND p.status <> 'cancelled' AND %s ORDER BY pay.created_at DESC`, scopeSQL), scopeArgs...)
}
func (h *Handler) CreatePayment(c *fiber.Ctx) error {
	if err := h.requireBodyProjectPermission(c, []string{"accounting.payment_create"}); err != nil {
		return err
	}
	return h.create(c, "payments", paymentColumns())
}
func (h *Handler) ApprovePayment(c *fiber.Ctx) error {
	return h.paymentAction(c, "approve")
}

func (h *Handler) payableAction(c *fiber.Ctx, action string) error {
	permissions := []string{"accounting.payable_approve"}
	if action == "pay" {
		permissions = []string{"accounting.payable_approve", "accounting.payment_approve"}
	}
	if err := h.requireRecordProjectPermission(c, "payables", c.Params("id"), permissions); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.PayableAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) receivableAction(c *fiber.Ctx, action string) error {
	permissions := []string{"accounting.receivable_approve"}
	if action == "receive" {
		permissions = []string{"accounting.receivable_approve", "accounting.payment_approve"}
	}
	if err := h.requireRecordProjectPermission(c, "receivables", c.Params("id"), permissions); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.ReceivableAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) paymentAction(c *fiber.Ctx, action string) error {
	if err := h.requireRecordProjectPermission(c, "payments", c.Params("id"), []string{"accounting.payment_approve"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.PaymentAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ListStaff(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"staff.view"}); err != nil {
		return err
	}
	return h.list(c, `SELECT * FROM staff WHERE status <> 'deleted' ORDER BY full_name`)
}
func (h *Handler) CreateStaff(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"staff.create"}); err != nil {
		return err
	}
	return h.create(c, "staff", staffColumns())
}
func (h *Handler) UpdateStaff(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"staff.update"}); err != nil {
		return err
	}
	return h.update(c, "staff", staffColumns())
}
func (h *Handler) ListRoles(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"role.view"}); err != nil {
		return err
	}
	return h.list(c, `SELECT * FROM roles ORDER BY display_name`)
}
func (h *Handler) CreateRole(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"role.create"}); err != nil {
		return err
	}
	return h.create(c, "roles", roleColumns())
}
func (h *Handler) UpdateRole(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"role.update"}); err != nil {
		return err
	}
	return h.update(c, "roles", roleColumns())
}
func (h *Handler) ListPermissions(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"permission.view"}); err != nil {
		return err
	}
	return h.list(c, `SELECT * FROM permissions ORDER BY module, key`)
}

func (h *Handler) AssignRolePermissions(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"permission.assign"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	permissionIDs, _ := body["permission_ids"].([]any)
	for _, permissionID := range permissionIDs {
		_ = h.service.repo.Exec(c.Context(), `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, c.Params("id"), permissionID)
	}
	return response.OK(c, Record{"assigned": len(permissionIDs)})
}

func (h *Handler) AssignUserRoles(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"permission.assign"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	roleIDs, _ := body["role_ids"].([]any)
	for _, roleID := range roleIDs {
		_ = h.service.repo.Exec(c.Context(), `INSERT INTO user_roles (user_id, role_id, project_id) VALUES ($1,$2,NULLIF($3,'')::uuid) ON CONFLICT DO NOTHING`, c.Params("id"), roleID, stringValue(body["project_id"]))
	}
	return response.OK(c, Record{"assigned": len(roleIDs)})
}

func (h *Handler) AuditLogs(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"audit.view"}); err != nil {
		return err
	}
	rows, err := h.service.AuditLogs(c.Context(), "", actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}
func (h *Handler) ProjectAuditLogs(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"audit.view"}); err != nil {
		return err
	}
	rows, err := h.service.AuditLogs(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) Dashboard(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"report.view"}); err != nil {
		return err
	}
	data, err := h.service.Dashboard(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}
func (h *Handler) ProjectReport(c *fiber.Ctx) error     { return h.report(c, "projects") }
func (h *Handler) WarehouseReport(c *fiber.Ctx) error   { return h.report(c, "warehouse") }
func (h *Handler) ProcurementReport(c *fiber.Ctx) error { return h.report(c, "procurement") }
func (h *Handler) AccountingReport(c *fiber.Ctx) error  { return h.report(c, "accounting") }
func (h *Handler) StaffReport(c *fiber.Ctx) error       { return h.report(c, "staff") }
func (h *Handler) report(c *fiber.Ctx, kind string) error {
	permissions := []string{"report.view"}
	if kind == "accounting" {
		permissions = []string{"report.finance.view", "accounting.view"}
	}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	rows, err := h.service.Report(c.Context(), kind, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}
func (h *Handler) Settings(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"settings.view"}); err != nil {
		return err
	}
	return h.list(c, `SELECT * FROM system_settings ORDER BY key`)
}

func (h *Handler) list(c *fiber.Ctx, query string, args ...any) error {
	rows, err := h.service.repo.Query(c.Context(), query, args...)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) create(c *fiber.Ctx, table string, specs []ColumnSpec) error {
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.repo.Insert(c.Context(), table, specs, body)
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) update(c *fiber.Ctx, table string, specs []ColumnSpec) error {
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.repo.Update(c.Context(), table, specs, c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) projectList(c *fiber.Ctx, table string, permissions []string) error {
	if err := h.requireProjectPermission(c, c.Params("id"), permissions); err != nil {
		return err
	}
	tableName := safeTable(table)
	rows, err := h.service.repo.Query(c.Context(), fmt.Sprintf("SELECT * FROM %s WHERE project_id=$1%s ORDER BY created_at DESC", tableName, projectListNotDeletedCondition(tableName)), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func projectListNotDeletedCondition(table string) string {
	switch table {
	case "issues", "acceptance_records", "rework_requests", "replanning_requests", "handover_records":
		return " AND status <> 'deleted'"
	case "accounting_budgets":
		return " AND status <> 'cancelled'"
	default:
		return ""
	}
}

func (h *Handler) projectCreate(c *fiber.Ctx, table string, specs []ColumnSpec, permissions []string) error {
	if err := h.requireProjectPermission(c, c.Params("id"), permissions); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	body["project_id"] = c.Params("id")
	if body["created_by"] == nil {
		body["created_by"] = actorID(c)
	}
	row, err := h.service.repo.Insert(c.Context(), table, specs, body)
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) setStatus(c *fiber.Ctx, table, status string, permissions []string) error {
	if err := h.requireRecordProjectPermission(c, table, c.Params("id"), permissions); err != nil {
		return err
	}
	row, err := h.service.repo.QueryOne(c.Context(), fmt.Sprintf("UPDATE %s SET status=$2%s WHERE id=$1 RETURNING *", safeTable(table), updatedAtSQL(table)), c.Params("id"), status)
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) requirePermission(c *fiber.Ctx, permissions []string) error {
	allowed, err := h.service.repo.CanUsePermission(c.Context(), actorID(c), permissions)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	return forbiddenError("Bạn không có quyền thao tác module này", Record{"permissions": permissions})
}

func (h *Handler) requireProjectPermission(c *fiber.Ctx, projectID string, permissions []string) error {
	allowed, err := h.service.repo.CanAccessProjectPermission(c.Context(), actorID(c), projectID, permissions)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	return forbiddenError("Bạn không có quyền thao tác dữ liệu của công trình này", Record{"permissions": permissions})
}

func (h *Handler) requireRecordProjectPermission(c *fiber.Ctx, table, id string, permissions []string) error {
	projectID, err := h.service.repo.RecordProjectID(c.Context(), table, id)
	if err != nil {
		return err
	}
	return h.requireProjectPermission(c, projectID, permissions)
}

func (h *Handler) requireBodyProjectPermission(c *fiber.Ctx, permissions []string) error {
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	return h.requireProjectPermission(c, cleanID(body["project_id"]), permissions)
}

func (h *Handler) projectAccessFilter(c *fiber.Ctx, projectAlias string, start int) (string, []any, error) {
	return h.service.repo.ProjectAccessFilter(c.Context(), actorID(c), projectAlias, start)
}

func bodyMap(c *fiber.Ctx) (Record, error) {
	if len(c.Body()) == 0 {
		return Record{}, nil
	}
	var body Record
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return nil, response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu gửi lên không hợp lệ")
	}
	return body, nil
}

func actionRequest(c *fiber.Ctx) (ActionRequest, error) {
	req := ActionRequest{}
	if len(c.Body()) == 0 {
		return req, nil
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return req, response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu thao tác không hợp lệ")
	}
	return req, nil
}

func (h *Handler) requireRunNodePermission(c *fiber.Ctx, nodeID string) error {
	allowed, permissions, err := h.service.repo.RunNodePermissionGate(c.Context(), actorID(c), nodeID)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	return forbiddenError("Bạn không có quyền thao tác node này", Record{"permissions": permissions})
}

func (h *Handler) requireRunStepPermission(c *fiber.Ctx, stepID string) error {
	allowed, permissions, err := h.service.repo.RunStepPermissionGate(c.Context(), actorID(c), stepID)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	return forbiddenError("Bạn không có quyền thao tác step này", Record{"permissions": permissions})
}

func actorID(c *fiber.Ctx) string {
	return fmt.Sprint(c.Locals("user_id"))
}

func actorName(c *fiber.Ctx) string {
	return fmt.Sprint(c.Locals("email"))
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func updatedAtSQL(table string) string {
	if hasUpdatedAt(table) {
		return ", updated_at=now()"
	}
	return ""
}
