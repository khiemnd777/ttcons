package core

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) ProjectFinanceProjects(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting_handoff.view", "project_finance.view"}); err != nil {
		return err
	}
	req, err := projectOptionsRequest(c)
	if err != nil {
		return err
	}
	data, err := h.service.ProjectFinanceProjects(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func (h *Handler) ProjectFinanceWorkspace(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"accounting_handoff.view", "project_finance.view"}); err != nil {
		return err
	}
	canViewReport, err := h.service.repo.CanAccessProjectPermission(c.Context(), actorID(c), c.Params("id"), []string{"project_finance.report_view"})
	if err != nil {
		return err
	}
	data, err := h.service.ProjectFinanceWorkspace(c.Context(), c.Params("id"), canViewReport)
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func (h *Handler) CreateAccountingHandoff(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"accounting_handoff.submit"}); err != nil {
		return err
	}
	var req AccountingHandoffRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return validationError("Dữ liệu bàn giao kế toán không hợp lệ")
	}
	row, err := h.service.CreateAccountingHandoff(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) AccountingHandoffDetail(c *fiber.Ctx) error {
	data, err := h.service.AccountingHandoffDetail(c.Context(), c.Params("id"), c.Query("source_type"), c.Query("source_id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func (h *Handler) AccountingHandoffAction(c *fiber.Ctx) error {
	action := c.Params("action")
	permissions := map[string][]string{
		"handoff":     {"accounting_handoff.submit"},
		"acknowledge": {"accounting_handoff.acknowledge"},
		"return":      {"accounting_handoff.acknowledge"},
	}[action]
	if len(permissions) == 0 {
		return validationError("Hành động bàn giao kế toán không hợp lệ")
	}
	if err := h.requireRecordProjectPermission(c, "accounting_handoffs", c.Params("id"), permissions); err != nil {
		return err
	}
	var req AccountingHandoffActionRequest
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return validationError("Dữ liệu thao tác không hợp lệ")
		}
	}
	row, err := h.service.AccountingHandoffAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ProjectFinanceOptions(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"project_finance.view", "project_finance.cost_create", "project_finance.request_create"}); err != nil {
		return err
	}
	data, err := h.service.ProjectFinanceOptions(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func (h *Handler) CreateProjectMaterialCost(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"project_finance.cost_create"}); err != nil {
		return err
	}
	var req ProjectMaterialCostRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return validationError("Dữ liệu chi phí vật tư không hợp lệ")
	}
	row, err := h.service.CreateProjectMaterialCost(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) CreateProjectCashRequest(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"project_finance.request_create"}); err != nil {
		return err
	}
	var req ProjectCashRequestRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return validationError("Dữ liệu đề nghị thu/chi không hợp lệ")
	}
	row, err := h.service.CreateProjectCashRequest(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ProjectMaterialCostAction(c *fiber.Ctx) error {
	action := c.Params("action")
	permissions := map[string][]string{"submit": {"project_finance.cost_create"}, "request-changes": {"project_finance.cost_review"}, "confirm": {"project_finance.cost_review"}}[action]
	if len(permissions) == 0 {
		return validationError("Hành động chi phí vật tư không hợp lệ")
	}
	if err := h.requireRecordProjectPermission(c, "project_material_costs", c.Params("id"), permissions); err != nil {
		return err
	}
	var req ProjectFinanceActionRequest
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return validationError("Dữ liệu thao tác không hợp lệ")
		}
	}
	row, err := h.service.ProjectMaterialCostAction(c.Context(), c.Params("id"), normalizeProjectFinanceAction(action), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ProjectCashRequestAction(c *fiber.Ctx) error {
	action := c.Params("action")
	permissions := map[string][]string{
		"submit": {"project_finance.request_create"}, "approve": {"project_finance.request_approve"},
		"request-changes": {"project_finance.request_approve"}, "handoff": {"project_finance.process"}, "process": {"project_finance.process"},
	}[action]
	if len(permissions) == 0 {
		return validationError("Hành động đề nghị thu/chi không hợp lệ")
	}
	if err := h.requireRecordProjectPermission(c, "project_cash_requests", c.Params("id"), permissions); err != nil {
		return err
	}
	var req ProjectFinanceActionRequest
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return validationError("Dữ liệu thao tác không hợp lệ")
		}
	}
	row, err := h.service.ProjectCashRequestAction(c.Context(), c.Params("id"), normalizeProjectFinanceAction(action), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func normalizeProjectFinanceAction(action string) string {
	if action == "request-changes" {
		return "request_changes"
	}
	return action
}

func (h *Handler) ProjectFinanceCSV(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"project_finance.report_view"}); err != nil {
		return err
	}
	data, err := h.service.ProjectFinanceWorkspace(c.Context(), c.Params("id"), true)
	if err != nil {
		return err
	}
	rows, _ := data["material_costs"].([]Record)
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buf)
	_ = writer.Write([]string{"Nhà cung cấp", "Số hóa đơn", "Ngày hóa đơn", "Nội dung", "Trước VAT", "VAT", "Tổng thanh toán", "Chi phí đã xác nhận", "Trạng thái"})
	for _, row := range rows {
		_ = writer.Write([]string{fmt.Sprint(row["vendor_name"]), fmt.Sprint(row["invoice_number"]), fmt.Sprint(row["invoice_date"]), fmt.Sprint(row["description"]), csvNumber(row["amount_before_vat"]), csvNumber(row["vat_amount"]), csvNumber(row["total_amount"]), csvNumber(row["recognized_cost_amount"]), fmt.Sprint(row["status"])})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="project-finance-material-costs.csv"`)
	return c.Send(buf.Bytes())
}

func csvNumber(value any) string { return strconv.FormatFloat(number(value), 'f', 2, 64) }
