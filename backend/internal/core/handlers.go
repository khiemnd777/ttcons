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
	rows, err := h.service.ListProjects(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateProject(c *fiber.Ctx) error {
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
	project, err := h.service.ProjectDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, project)
}

func (h *Handler) UpdateProject(c *fiber.Ctx) error {
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
	if err := h.service.DeleteProject(c.Context(), c.Params("id")); err != nil {
		return err
	}
	return response.OK(c, Record{"deleted": true})
}

func (h *Handler) ListTemplates(c *fiber.Ctx) error {
	rows, err := h.service.ListTemplates(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateTemplate(c *fiber.Ctx) error {
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
	template, err := h.service.TemplateDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, template)
}

func (h *Handler) UpdateTemplate(c *fiber.Ctx) error {
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
	if err := h.service.DeleteTemplate(c.Context(), c.Params("id")); err != nil {
		return err
	}
	return response.OK(c, Record{"deleted": true})
}

func (h *Handler) PublishTemplate(c *fiber.Ctx) error {
	template, err := h.service.PublishTemplate(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, template)
}

func (h *Handler) CloneTemplate(c *fiber.Ctx) error {
	template, err := h.service.CloneTemplate(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.Created(c, template)
}

func (h *Handler) AddTemplateNode(c *fiber.Ctx) error {
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
	if err := h.service.repo.Exec(c.Context(), `DELETE FROM pipeline_template_nodes WHERE id=$1`, c.Params("id")); err != nil {
		return err
	}
	return response.OK(c, Record{"deleted": true})
}

func (h *Handler) AddTemplateStep(c *fiber.Ctx) error {
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
	if err := h.service.repo.Exec(c.Context(), `DELETE FROM pipeline_template_steps WHERE id=$1`, c.Params("id")); err != nil {
		return err
	}
	return response.OK(c, Record{"deleted": true})
}

func (h *Handler) AddTemplateEdge(c *fiber.Ctx) error {
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
	return response.OK(c, PipelineEdgeConditionCatalog())
}

func (h *Handler) UpdateTemplateEdge(c *fiber.Ctx) error {
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
	run, err := h.service.PipelineRun(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, run)
}

func (h *Handler) StartRunNode(c *fiber.Ctx) error {
	node, err := h.service.repo.QueryOne(c.Context(), `UPDATE pipeline_run_nodes SET status='in_progress', started_at=COALESCE(started_at, now()) WHERE id=$1 RETURNING *`, c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, node)
}

func (h *Handler) SubmitRunStep(c *fiber.Ctx) error {
	step, err := h.service.SubmitRunStep(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, step)
}

func (h *Handler) RequestNodeApproval(c *fiber.Ctx) error {
	body, _ := bodyMap(c)
	req, err := h.service.RequestNodeApproval(c.Context(), c.Params("id"), actorID(c), actorName(c), stringValue(body["reason"]))
	if err != nil {
		return err
	}
	return response.Created(c, req)
}

func (h *Handler) AdvanceRunNode(c *fiber.Ctx) error {
	run, err := h.service.AdvanceRunNode(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, run)
}

func (h *Handler) BlockRunNode(c *fiber.Ctx) error {
	body, _ := bodyMap(c)
	node, err := h.service.BlockRunNode(c.Context(), c.Params("id"), stringValue(body["reason"]), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, node)
}

func (h *Handler) ListApprovals(c *fiber.Ctx) error {
	rows, err := h.service.repo.Query(c.Context(), `
		SELECT ar.*, p.name AS project_name, u.full_name AS requested_by_name
		FROM approval_requests ar
		LEFT JOIN projects p ON p.id=ar.project_id
		LEFT JOIN users u ON u.id=ar.requested_by
		WHERE p.id IS NULL OR p.status <> 'cancelled'
		ORDER BY ar.created_at DESC
	`)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) PendingApprovals(c *fiber.Ctx) error {
	rows, err := h.service.repo.Query(c.Context(), `
		SELECT ar.*, p.name AS project_name
		FROM approval_requests ar
		LEFT JOIN projects p ON p.id=ar.project_id
		WHERE ar.status='pending' AND (p.id IS NULL OR p.status <> 'cancelled')
		ORDER BY ar.created_at DESC
	`)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) Approve(c *fiber.Ctx) error       { return h.approvalAction(c, "approve") }
func (h *Handler) Reject(c *fiber.Ctx) error        { return h.approvalAction(c, "reject") }
func (h *Handler) RequestChange(c *fiber.Ctx) error { return h.approvalAction(c, "request_change") }

func (h *Handler) approvalAction(c *fiber.Ctx, action string) error {
	body, _ := bodyMap(c)
	req, err := h.service.ApprovalAction(c.Context(), c.Params("id"), action, stringValue(body["comment"]), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, req)
}

func (h *Handler) ProjectBOQ(c *fiber.Ctx) error {
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
	if err := h.service.repo.Exec(c.Context(), `UPDATE boq_items SET status='submitted', updated_at=now() WHERE project_id=$1 AND status='draft'`, c.Params("id")); err != nil {
		return err
	}
	return response.OK(c, Record{"submitted": true})
}

func (h *Handler) ApproveBOQ(c *fiber.Ctx) error {
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
	rows, err := h.service.repo.Query(c.Context(), `
		SELECT pr.*, p.name AS project_name
		FROM procurement_requests pr
		JOIN projects p ON p.id=pr.project_id
		WHERE pr.status <> 'cancelled' AND p.status <> 'cancelled'
		ORDER BY pr.created_at DESC
	`)
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
	req, err := h.service.repo.GetProcurementDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, req)
}

func (h *Handler) UpdateProcurement(c *fiber.Ctx) error {
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
	req, err := h.service.ProcurementAction(c.Context(), c.Params("id"), action, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, req)
}

func (h *Handler) ListMaterials(c *fiber.Ctx) error {
	return h.list(c, `SELECT * FROM materials WHERE active ORDER BY category, name`)
}
func (h *Handler) CreateMaterial(c *fiber.Ctx) error {
	return h.create(c, "materials", materialColumns())
}
func (h *Handler) ListWarehouses(c *fiber.Ctx) error {
	return h.list(c, `SELECT * FROM warehouses WHERE active ORDER BY type, name`)
}
func (h *Handler) CreateWarehouse(c *fiber.Ctx) error {
	return h.create(c, "warehouses", warehouseColumns())
}

func (h *Handler) Stock(c *fiber.Ctx) error {
	rows, err := h.service.repo.Query(c.Context(), `
		SELECT m.id AS material_id, m.code, m.name, m.unit, m.category,
			COALESCE(SUM(CASE WHEN sm.movement_type='receive' THEN sm.quantity WHEN sm.movement_type IN ('issue','loss') THEN -sm.quantity ELSE 0 END),0) AS stock_quantity
		FROM materials m
		LEFT JOIN stock_movements sm ON sm.material_id=m.id
		WHERE m.active
		GROUP BY m.id
		ORDER BY m.category, m.name
	`)
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
	movement, err := h.service.CreateStockMovement(c.Context(), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, movement)
}

func (h *Handler) MaterialBalances(c *fiber.Ctx) error {
	rows, err := h.service.repo.ProjectMaterialBalances(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) ProjectDailyReports(c *fiber.Ctx) error {
	return h.projectList(c, "operation_daily_reports")
}
func (h *Handler) CreateDailyReport(c *fiber.Ctx) error {
	return h.projectCreate(c, "operation_daily_reports", dailyReportColumns())
}
func (h *Handler) ProjectIssues(c *fiber.Ctx) error { return h.projectList(c, "issues") }
func (h *Handler) CreateIssue(c *fiber.Ctx) error {
	return h.projectCreate(c, "issues", issueColumns())
}
func (h *Handler) UpdateIssue(c *fiber.Ctx) error {
	return h.update(c, "issues", issueColumns())
}

func (h *Handler) ProjectAcceptances(c *fiber.Ctx) error {
	return h.projectList(c, "acceptance_records")
}
func (h *Handler) CreateAcceptance(c *fiber.Ctx) error {
	return h.projectCreate(c, "acceptance_records", acceptanceColumns())
}
func (h *Handler) GetAcceptance(c *fiber.Ctx) error {
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
	acc, err := h.service.AcceptanceAction(c.Context(), c.Params("id"), result, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, acc)
}
func (h *Handler) CreateReworkFromAcceptance(c *fiber.Ctx) error {
	rework, err := h.service.CreateReworkFromAcceptance(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, rework)
}

func (h *Handler) ProjectReworks(c *fiber.Ctx) error { return h.projectList(c, "rework_requests") }
func (h *Handler) CreateRework(c *fiber.Ctx) error {
	return h.projectCreate(c, "rework_requests", reworkColumns())
}
func (h *Handler) ApproveRework(c *fiber.Ctx) error {
	return h.setStatus(c, "rework_requests", "approved")
}
func (h *Handler) CompleteRework(c *fiber.Ctx) error {
	return h.setStatus(c, "rework_requests", "done")
}
func (h *Handler) ProjectReplannings(c *fiber.Ctx) error {
	return h.projectList(c, "replanning_requests")
}
func (h *Handler) CreateReplanning(c *fiber.Ctx) error {
	return h.projectCreate(c, "replanning_requests", replanningColumns())
}
func (h *Handler) ApproveReplanning(c *fiber.Ctx) error {
	return h.setStatus(c, "replanning_requests", "approved")
}

func (h *Handler) ProjectHandover(c *fiber.Ctx) error {
	rows, err := h.service.repo.Query(c.Context(), `SELECT * FROM handover_records WHERE project_id=$1 AND status <> 'deleted' ORDER BY created_at DESC`, c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}
func (h *Handler) CreateHandover(c *fiber.Ctx) error {
	return h.projectCreate(c, "handover_records", handoverColumns())
}
func (h *Handler) CheckHandover(c *fiber.Ctx) error {
	check, err := h.service.HandoverCheck(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, check)
}
func (h *Handler) CompleteHandover(c *fiber.Ctx) error {
	done, err := h.service.CompleteHandover(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, done)
}

func (h *Handler) ProjectBudget(c *fiber.Ctx) error { return h.projectList(c, "accounting_budgets") }
func (h *Handler) CreateBudget(c *fiber.Ctx) error {
	return h.projectCreate(c, "accounting_budgets", budgetColumns())
}
func (h *Handler) ApproveBudget(c *fiber.Ctx) error {
	return h.setStatus(c, "accounting_budgets", "approved")
}
func (h *Handler) ListPayables(c *fiber.Ctx) error {
	return h.list(c, `SELECT pay.*, p.name AS project_name FROM payables pay JOIN projects p ON p.id=pay.project_id WHERE pay.status <> 'cancelled' AND p.status <> 'cancelled' ORDER BY pay.created_at DESC`)
}
func (h *Handler) CreatePayable(c *fiber.Ctx) error  { return h.create(c, "payables", payableColumns()) }
func (h *Handler) ApprovePayable(c *fiber.Ctx) error { return h.setStatus(c, "payables", "approved") }
func (h *Handler) PayPayable(c *fiber.Ctx) error     { return h.setStatus(c, "payables", "paid") }
func (h *Handler) ListReceivables(c *fiber.Ctx) error {
	return h.list(c, `SELECT rec.*, p.name AS project_name FROM receivables rec JOIN projects p ON p.id=rec.project_id WHERE rec.status <> 'cancelled' AND p.status <> 'cancelled' ORDER BY rec.created_at DESC`)
}
func (h *Handler) CreateReceivable(c *fiber.Ctx) error {
	return h.create(c, "receivables", receivableColumns())
}
func (h *Handler) ApproveReceivable(c *fiber.Ctx) error {
	return h.setStatus(c, "receivables", "approved")
}
func (h *Handler) ReceiveReceivable(c *fiber.Ctx) error {
	return h.setStatus(c, "receivables", "received")
}
func (h *Handler) ListPayments(c *fiber.Ctx) error {
	return h.list(c, `SELECT pay.*, p.name AS project_name FROM payments pay JOIN projects p ON p.id=pay.project_id WHERE pay.status <> 'cancelled' AND p.status <> 'cancelled' ORDER BY pay.created_at DESC`)
}
func (h *Handler) CreatePayment(c *fiber.Ctx) error  { return h.create(c, "payments", paymentColumns()) }
func (h *Handler) ApprovePayment(c *fiber.Ctx) error { return h.setStatus(c, "payments", "approved") }

func (h *Handler) ListStaff(c *fiber.Ctx) error {
	return h.list(c, `SELECT * FROM staff WHERE status <> 'deleted' ORDER BY full_name`)
}
func (h *Handler) CreateStaff(c *fiber.Ctx) error { return h.create(c, "staff", staffColumns()) }
func (h *Handler) UpdateStaff(c *fiber.Ctx) error { return h.update(c, "staff", staffColumns()) }
func (h *Handler) ListRoles(c *fiber.Ctx) error {
	return h.list(c, `SELECT * FROM roles ORDER BY display_name`)
}
func (h *Handler) CreateRole(c *fiber.Ctx) error { return h.create(c, "roles", roleColumns()) }
func (h *Handler) UpdateRole(c *fiber.Ctx) error { return h.update(c, "roles", roleColumns()) }
func (h *Handler) ListPermissions(c *fiber.Ctx) error {
	return h.list(c, `SELECT * FROM permissions ORDER BY module, key`)
}

func (h *Handler) AssignRolePermissions(c *fiber.Ctx) error {
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
	rows, err := h.service.AuditLogs(c.Context(), "")
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}
func (h *Handler) ProjectAuditLogs(c *fiber.Ctx) error {
	rows, err := h.service.AuditLogs(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) Dashboard(c *fiber.Ctx) error {
	data, err := h.service.Dashboard(c.Context())
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
	rows, err := h.service.Report(c.Context(), kind)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}
func (h *Handler) Settings(c *fiber.Ctx) error {
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

func (h *Handler) projectList(c *fiber.Ctx, table string) error {
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

func (h *Handler) projectCreate(c *fiber.Ctx, table string, specs []ColumnSpec) error {
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

func (h *Handler) setStatus(c *fiber.Ctx, table, status string) error {
	row, err := h.service.repo.QueryOne(c.Context(), fmt.Sprintf("UPDATE %s SET status=$2%s WHERE id=$1 RETURNING *", safeTable(table), updatedAtSQL(table)), c.Params("id"), status)
	if err != nil {
		return err
	}
	return response.OK(c, row)
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
