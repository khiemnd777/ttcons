package core

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) Dashboard(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"report.view"}); err != nil {
		return err
	}
	req, err := dashboardPeriodRequest(c)
	if err != nil {
		return err
	}
	data, err := h.service.Dashboard(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func (h *Handler) DashboardProjects(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"report.view"}); err != nil {
		return err
	}
	req, err := dashboardProjectsRequest(c)
	if err != nil {
		return err
	}
	data, err := h.service.DashboardProjects(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func dashboardPeriodRequest(c *fiber.Ctx) (DashboardPeriodRequest, error) {
	year, err := optionalQueryInt(c, "year")
	if err != nil {
		return DashboardPeriodRequest{}, err
	}
	periodNo, err := optionalQueryInt(c, "period_no")
	if err != nil {
		return DashboardPeriodRequest{}, err
	}
	return DashboardPeriodRequest{
		PeriodType: c.Query("period_type"),
		Year:       year,
		PeriodNo:   periodNo,
	}, nil
}

func dashboardProjectsRequest(c *fiber.Ctx) (DashboardProjectsRequest, error) {
	period, err := dashboardPeriodRequest(c)
	if err != nil {
		return DashboardProjectsRequest{}, err
	}
	limit, err := optionalQueryInt(c, "limit")
	if err != nil {
		return DashboardProjectsRequest{}, err
	}
	return DashboardProjectsRequest{
		DashboardPeriodRequest: period,
		Search:                 c.Query("search"),
		Status:                 c.Query("status"),
		Sort:                   c.Query("sort"),
		Direction:              c.Query("direction"),
		Cursor:                 c.Query("cursor"),
		Limit:                  limit,
	}, nil
}

func optionalQueryInt(c *fiber.Ctx, key string) (int, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, validationError(fmt.Sprintf("%s không hợp lệ", key), Record{key: raw})
	}
	return parsed, nil
}

func (h *Handler) ReportOptions(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"report.view", "report.finance.view", "accounting.view"}); err != nil {
		return err
	}
	req, err := projectOptionsRequest(c)
	if err != nil {
		return err
	}
	data, err := h.service.ReportOptions(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func projectOptionsRequest(c *fiber.Ctx) (ReportOptionsRequest, error) {
	limit := 0
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			return ReportOptionsRequest{}, validationError("limit không hợp lệ", Record{"limit": rawLimit})
		}
		limit = parsed
	}
	return ReportOptionsRequest{
		Search:     c.Query("search"),
		Cursor:     c.Query("cursor"),
		Limit:      limit,
		SelectedID: c.Query("selected_id"),
	}, nil
}

func (h *Handler) CashflowReport(c *fiber.Ctx) error {
	permissions := []string{"report.finance.view", "accounting.view"}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	req := cashflowReportRequest(c)
	if req.ProjectID != "" {
		if !validActivityUUID(req.ProjectID) {
			return validationError("project_id không hợp lệ", Record{"project_id": req.ProjectID})
		}
	}
	data, err := h.service.CashflowReport(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}
func (h *Handler) CashflowReportCSV(c *fiber.Ctx) error {
	permissions := []string{"report.finance.view", "accounting.view"}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	req := cashflowReportRequest(c)
	if req.ProjectID != "" {
		if !validActivityUUID(req.ProjectID) {
			return validationError("project_id không hợp lệ", Record{"project_id": req.ProjectID})
		}
	}
	data, err := h.service.CashflowReport(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	payload, err := cashflowCSV(data)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="cashflow.csv"`)
	return c.Send(payload)
}
func (h *Handler) TaxReport(c *fiber.Ctx) error {
	permissions := []string{"report.finance.view", "accounting.view"}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	req := taxReportRequest(c)
	if req.ProjectID != "" {
		if !validActivityUUID(req.ProjectID) {
			return validationError("project_id không hợp lệ", Record{"project_id": req.ProjectID})
		}
	}
	data, err := h.service.TaxReport(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}
func (h *Handler) TaxReportCSV(c *fiber.Ctx) error {
	permissions := []string{"report.finance.view", "accounting.view"}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	req := taxReportRequest(c)
	if req.ProjectID != "" {
		if !validActivityUUID(req.ProjectID) {
			return validationError("project_id không hợp lệ", Record{"project_id": req.ProjectID})
		}
	}
	data, err := h.service.TaxReport(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	payload, err := taxCSV(data)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="tax-report.csv"`)
	return c.Send(payload)
}
func (h *Handler) ProfitReport(c *fiber.Ctx) error {
	permissions := []string{"report.finance.view", "accounting.view"}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	req := profitReportRequest(c)
	if req.ProjectID != "" {
		if !validActivityUUID(req.ProjectID) {
			return validationError("project_id không hợp lệ", Record{"project_id": req.ProjectID})
		}
	}
	data, err := h.service.ProfitReport(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}
func (h *Handler) ProfitReportCSV(c *fiber.Ctx) error {
	permissions := []string{"report.finance.view", "accounting.view"}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	req := profitReportRequest(c)
	if req.ProjectID != "" {
		if !validActivityUUID(req.ProjectID) {
			return validationError("project_id không hợp lệ", Record{"project_id": req.ProjectID})
		}
	}
	data, err := h.service.ProfitReport(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	payload, err := profitCSV(data)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="profit-report.csv"`)
	return c.Send(payload)
}
func (h *Handler) ProjectReport(c *fiber.Ctx) error     { return h.report(c, "projects") }
func (h *Handler) WarehouseReport(c *fiber.Ctx) error   { return h.report(c, "warehouse") }
func (h *Handler) ProcurementReport(c *fiber.Ctx) error { return h.report(c, "procurement") }
func (h *Handler) AccountingReport(c *fiber.Ctx) error  { return h.report(c, "accounting") }
func (h *Handler) StaffReport(c *fiber.Ctx) error       { return h.report(c, "staff") }
func (h *Handler) ProjectReportCSV(c *fiber.Ctx) error  { return h.reportCSV(c, "projects") }
func (h *Handler) WarehouseReportCSV(c *fiber.Ctx) error {
	return h.reportCSV(c, "warehouse")
}
func (h *Handler) ProcurementReportCSV(c *fiber.Ctx) error {
	return h.reportCSV(c, "procurement")
}
func (h *Handler) AccountingReportCSV(c *fiber.Ctx) error {
	return h.reportCSV(c, "accounting")
}
func (h *Handler) StaffReportCSV(c *fiber.Ctx) error { return h.reportCSV(c, "staff") }

func (h *Handler) report(c *fiber.Ctx, kind string) error {
	permissions := operationalReportPermissions(kind)
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	req, err := operationalReportRequest(c)
	if err != nil {
		return err
	}
	rows, err := h.service.Report(c.Context(), kind, req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) reportCSV(c *fiber.Ctx, kind string) error {
	permissions := operationalReportPermissions(kind)
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	req, err := operationalReportRequest(c)
	if err != nil {
		return err
	}
	rows, err := h.service.Report(c.Context(), kind, req, actorID(c))
	if err != nil {
		return err
	}
	payload, err := operationalReportCSV(kind, rows)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s-report.csv"`, kind))
	return c.Send(payload)
}

func operationalReportRequest(c *fiber.Ctx) (OperationalReportRequest, error) {
	projectID := cleanID(c.Query("project_id"))
	if projectID != "" && !validActivityUUID(projectID) {
		return OperationalReportRequest{}, validationError("project_id không hợp lệ", Record{"project_id": projectID})
	}
	return OperationalReportRequest{
		ProjectID: projectID,
		Search:    c.Query("search"),
		Sort:      c.Query("sort"),
		Direction: c.Query("direction"),
	}, nil
}

func cashflowReportRequest(c *fiber.Ctx) CashflowReportRequest {
	return CashflowReportRequest{
		ProjectID: cleanID(c.Query("project_id")),
		FromDate:  strings.TrimSpace(c.Query("from")),
		ToDate:    strings.TrimSpace(c.Query("to")),
		GroupBy:   strings.TrimSpace(c.Query("group_by")),
		Status:    strings.TrimSpace(c.Query("status")),
	}
}

func taxReportRequest(c *fiber.Ctx) TaxReportRequest {
	return TaxReportRequest{
		ProjectID: cleanID(c.Query("project_id")),
		FromDate:  strings.TrimSpace(c.Query("from")),
		ToDate:    strings.TrimSpace(c.Query("to")),
		GroupBy:   strings.TrimSpace(c.Query("group_by")),
		Source:    strings.TrimSpace(c.Query("source")),
		Status:    strings.TrimSpace(c.Query("status")),
	}
}

func profitReportRequest(c *fiber.Ctx) ProfitReportRequest {
	return ProfitReportRequest{
		ProjectID: cleanID(c.Query("project_id")),
		FromDate:  strings.TrimSpace(c.Query("from")),
		ToDate:    strings.TrimSpace(c.Query("to")),
		GroupBy:   strings.TrimSpace(c.Query("group_by")),
		Status:    strings.TrimSpace(c.Query("status")),
	}
}

func cashflowCSV(data Record) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{"occurred_at", "payment_code", "project_name", "category", "direction", "payment_type", "payment_method", "status", "inflow_amount", "outflow_amount", "net_amount", "target_type"}); err != nil {
		return nil, err
	}
	ledger, _ := data["ledger"].([]Record)
	for _, row := range ledger {
		if err := writer.Write([]string{
			fmt.Sprint(row["occurred_at"]),
			fmt.Sprint(row["payment_code"]),
			fmt.Sprint(row["project_name"]),
			fmt.Sprint(row["category"]),
			fmt.Sprint(row["direction"]),
			fmt.Sprint(row["payment_type"]),
			fmt.Sprint(row["payment_method"]),
			fmt.Sprint(row["status"]),
			fmt.Sprint(row["inflow_amount"]),
			fmt.Sprint(row["outflow_amount"]),
			fmt.Sprint(row["net_amount"]),
			fmt.Sprint(row["target_type"]),
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func operationalReportCSV(kind string, rows []Record) ([]byte, error) {
	columns := map[string][]string{
		"projects":    {"project_id", "name", "is_sample", "status", "progress_percent", "open_issues", "pending_approvals"},
		"warehouse":   {"material_id", "name", "category", "stock_quantity", "surplus_quantity"},
		"procurement": {"status", "budget_status", "total", "amount"},
		"accounting":  {"project_id", "name", "payable_remaining", "receivable_remaining", "used_budget"},
		"staff":       {"staff_id", "full_name", "department", "position", "project_count"},
	}
	keys, ok := columns[kind]
	if !ok || len(keys) == 0 {
		return nil, errors.New("Báo cáo không hợp lệ")
	}
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write(keys); err != nil {
		return nil, err
	}
	for _, row := range rows {
		values := make([]string, len(keys))
		for index, key := range keys {
			values[index] = fmt.Sprint(row[key])
		}
		if err := writer.Write(values); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func taxCSV(data Record) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{"invoice_date", "invoice_number", "source", "project_name", "counterparty_name", "tax_code", "amount_before_vat", "vat_amount", "total_amount", "status", "source_type", "source_code", "document_name", "warning_flags"}); err != nil {
		return nil, err
	}
	ledger, _ := data["ledger"].([]Record)
	for _, row := range ledger {
		if err := writer.Write([]string{
			fmt.Sprint(row["invoice_date"]),
			fmt.Sprint(row["invoice_number"]),
			fmt.Sprint(row["direction"]),
			fmt.Sprint(row["project_name"]),
			fmt.Sprint(row["counterparty_name"]),
			fmt.Sprint(row["tax_code"]),
			fmt.Sprint(row["amount_before_vat"]),
			fmt.Sprint(row["vat_amount"]),
			fmt.Sprint(row["total_amount"]),
			fmt.Sprint(row["status"]),
			fmt.Sprint(row["source_type"]),
			fmt.Sprint(row["source_code"]),
			fmt.Sprint(row["document_name"]),
			fmt.Sprint(row["warning_flags"]),
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func profitCSV(data Record) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{"invoice_date", "invoice_number", "direction", "cost_type", "project_name", "counterparty_name", "amount_before_vat", "revenue_amount", "direct_cost_amount", "indirect_cost_amount", "status", "source_type", "source_code", "classification_reason", "warning_flags"}); err != nil {
		return nil, err
	}
	ledger, _ := data["ledger"].([]Record)
	for _, row := range ledger {
		if err := writer.Write([]string{
			fmt.Sprint(row["invoice_date"]),
			fmt.Sprint(row["invoice_number"]),
			fmt.Sprint(row["direction"]),
			fmt.Sprint(row["cost_type"]),
			fmt.Sprint(row["project_name"]),
			fmt.Sprint(row["counterparty_name"]),
			fmt.Sprint(row["amount_before_vat"]),
			fmt.Sprint(row["revenue_amount"]),
			fmt.Sprint(row["direct_cost_amount"]),
			fmt.Sprint(row["indirect_cost_amount"]),
			fmt.Sprint(row["status"]),
			fmt.Sprint(row["source_type"]),
			fmt.Sprint(row["source_code"]),
			fmt.Sprint(row["classification_reason"]),
			fmt.Sprint(row["warning_flags"]),
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
