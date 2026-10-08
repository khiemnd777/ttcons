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

func (h *Handler) DashboardActivities(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"report.view"}); err != nil {
		return err
	}
	limit, err := optionalQueryInt(c, "limit")
	if err != nil {
		return err
	}
	page, err := h.service.DashboardActivities(c.Context(), DashboardActivitiesRequest{
		Range: c.Query("range"), Search: c.Query("search"), ActivityType: c.Query("activity_type"),
		Cursor: c.Query("cursor"), Limit: limit,
	}, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, page)
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
	if err := h.requirePermission(c, []string{"report.view"}); err != nil {
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

func (h *Handler) ProjectReport(c *fiber.Ctx) error     { return h.report(c, "projects") }
func (h *Handler) WarehouseReport(c *fiber.Ctx) error   { return h.report(c, "warehouse") }
func (h *Handler) ProcurementReport(c *fiber.Ctx) error { return h.report(c, "procurement") }
func (h *Handler) StaffReport(c *fiber.Ctx) error       { return h.report(c, "staff") }
func (h *Handler) ProjectReportCSV(c *fiber.Ctx) error  { return h.reportCSV(c, "projects") }
func (h *Handler) WarehouseReportCSV(c *fiber.Ctx) error {
	return h.reportCSV(c, "warehouse")
}
func (h *Handler) ProcurementReportCSV(c *fiber.Ctx) error {
	return h.reportCSV(c, "procurement")
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

func operationalReportCSV(kind string, rows []Record) ([]byte, error) {
	columns := map[string][]string{
		"projects":    {"project_id", "name", "is_sample", "status", "progress_percent", "open_issues", "pending_approvals"},
		"warehouse":   {"material_id", "name", "category", "stock_quantity", "surplus_quantity"},
		"procurement": {"status", "budget_status", "total", "amount"},
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
