package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Service) Dashboard(ctx context.Context, actorID string) (Record, error) {
	return s.repo.DashboardReport(ctx, actorID)
}

type reportProjectOptionsCursor struct {
	NameKey string `json:"name_key"`
	CodeKey string `json:"code_key"`
	ID      string `json:"id"`
}

func (s *Service) ReportOptions(ctx context.Context, req ReportOptionsRequest, actorID string) (ReportOptionsResponse, error) {
	return s.projectOptions(ctx, req, actorID, "")
}

func (s *Service) ProjectFinanceProjects(ctx context.Context, req ReportOptionsRequest, actorID string) (ReportOptionsResponse, error) {
	return s.projectOptions(ctx, req, actorID, "project_finance.view")
}

func (s *Service) projectOptions(ctx context.Context, req ReportOptionsRequest, actorID, requiredPermission string) (ReportOptionsResponse, error) {
	normalized, cursor, err := normalizeReportOptionsRequest(req)
	if err != nil {
		return ReportOptionsResponse{}, err
	}
	rows, selectedRow, err := s.repo.projectOptions(ctx, actorID, normalized, cursor, requiredPermission)
	if err != nil {
		return ReportOptionsResponse{}, err
	}
	hasMore := len(rows) > normalized.Limit
	if hasMore {
		rows = rows[:normalized.Limit]
	}
	items := make([]ReportProjectOption, 0, len(rows))
	for _, row := range rows {
		items = append(items, reportProjectOptionFromRecord(row))
	}
	nextCursor := ""
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		nextCursor = encodeReportProjectOptionsCursor(reportProjectOptionsCursor{
			NameKey: strings.TrimSpace(fmt.Sprint(last["name_key"])),
			CodeKey: strings.TrimSpace(fmt.Sprint(last["code_key"])),
			ID:      cleanID(last["id"]),
		})
	}
	var selected *ReportProjectOption
	if selectedRow != nil {
		option := reportProjectOptionFromRecord(selectedRow)
		selected = &option
	}
	return ReportOptionsResponse{Projects: ReportProjectOptionsPage{
		Items:      items,
		HasMore:    hasMore,
		NextCursor: nextCursor,
		Selected:   selected,
	}}, nil
}

func normalizeReportOptionsRequest(req ReportOptionsRequest) (ReportOptionsRequest, *reportProjectOptionsCursor, error) {
	req.Search = strings.TrimSpace(req.Search)
	if len([]rune(req.Search)) > 120 {
		return req, nil, validationError("Từ khóa tìm kiếm không được vượt quá 120 ký tự")
	}
	req.SelectedID = cleanID(req.SelectedID)
	if req.SelectedID != "" && !validActivityUUID(req.SelectedID) {
		return req, nil, validationError("selected_id không hợp lệ", Record{"selected_id": req.SelectedID})
	}
	if req.Limit == 0 {
		req.Limit = 20
	}
	if req.Limit < 1 || req.Limit > 50 {
		return req, nil, validationError("limit phải nằm trong khoảng 1 đến 50", Record{"limit": req.Limit})
	}
	req.Cursor = strings.TrimSpace(req.Cursor)
	if req.Cursor == "" {
		return req, nil, nil
	}
	cursor, err := parseReportProjectOptionsCursor(req.Cursor)
	if err != nil {
		return req, nil, validationError("cursor không hợp lệ", Record{"cursor": req.Cursor})
	}
	return req, &cursor, nil
}

func reportProjectOptionFromRecord(row Record) ReportProjectOption {
	return ReportProjectOption{
		ID:   cleanID(row["id"]),
		Code: strings.TrimSpace(fmt.Sprint(row["code"])),
		Name: strings.TrimSpace(fmt.Sprint(row["name"])),
	}
}

func encodeReportProjectOptionsCursor(cursor reportProjectOptionsCursor) string {
	raw, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func parseReportProjectOptionsCursor(value string) (reportProjectOptionsCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return reportProjectOptionsCursor{}, err
	}
	cursor := reportProjectOptionsCursor{}
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return reportProjectOptionsCursor{}, err
	}
	cursor.NameKey = strings.TrimSpace(cursor.NameKey)
	cursor.CodeKey = strings.TrimSpace(cursor.CodeKey)
	cursor.ID = cleanID(cursor.ID)
	if cursor.NameKey == "" || cursor.CodeKey == "" || !validActivityUUID(cursor.ID) {
		return reportProjectOptionsCursor{}, errors.New("invalid report project options cursor")
	}
	return cursor, nil
}

func (s *Service) Report(ctx context.Context, kind string, req OperationalReportRequest, actorID string) ([]Record, error) {
	req.ProjectID = cleanID(req.ProjectID)
	if req.ProjectID != "" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, req.ProjectID, operationalReportPermissions(kind))
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem báo cáo của công trình này", Record{"permissions": operationalReportPermissions(kind)})
		}
	}
	return s.repo.Report(ctx, kind, actorID, req.ProjectID)
}

func operationalReportPermissions(kind string) []string {
	if kind == "accounting" {
		return []string{"report.finance.view", "accounting.view"}
	}
	return []string{"report.view"}
}

func (s *Service) CashflowReport(ctx context.Context, req CashflowReportRequest, actorID string) (Record, error) {
	normalized, err := normalizeCashflowReportRequest(req)
	if err != nil {
		return nil, err
	}
	if normalized.ProjectID != "" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, normalized.ProjectID, []string{"report.finance.view", "accounting.view"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem dòng tiền của công trình này", Record{"permissions": []string{"report.finance.view", "accounting.view"}})
		}
	}
	return s.repo.CashflowReport(ctx, normalized, actorID)
}

func (s *Service) TaxReport(ctx context.Context, req TaxReportRequest, actorID string) (Record, error) {
	normalized, err := normalizeTaxReportRequest(req)
	if err != nil {
		return nil, err
	}
	if normalized.ProjectID != "" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, normalized.ProjectID, []string{"report.finance.view", "accounting.view"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem báo cáo thuế của công trình này", Record{"permissions": []string{"report.finance.view", "accounting.view"}})
		}
	}
	return s.repo.TaxReport(ctx, normalized, actorID)
}

func (s *Service) ProfitReport(ctx context.Context, req ProfitReportRequest, actorID string) (Record, error) {
	normalized, err := normalizeProfitReportRequest(req)
	if err != nil {
		return nil, err
	}
	if normalized.ProjectID != "" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, normalized.ProjectID, []string{"report.finance.view", "accounting.view"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem báo cáo lợi nhuận của công trình này", Record{"permissions": []string{"report.finance.view", "accounting.view"}})
		}
	}
	return s.repo.ProfitReport(ctx, normalized, actorID)
}

func normalizeCashflowReportRequest(req CashflowReportRequest) (CashflowReportRequest, error) {
	now := time.Now()
	from := strings.TrimSpace(req.FromDate)
	to := strings.TrimSpace(req.ToDate)
	if from == "" {
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	}
	if to == "" {
		to = now.Format("2006-01-02")
	}
	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return CashflowReportRequest{}, validationError("Ngày bắt đầu không hợp lệ", Record{"from": from})
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return CashflowReportRequest{}, validationError("Ngày kết thúc không hợp lệ", Record{"to": to})
	}
	if toDate.Before(fromDate) {
		return CashflowReportRequest{}, validationError("Ngày kết thúc phải sau ngày bắt đầu", Record{"from": from, "to": to})
	}
	groupBy := strings.TrimSpace(req.GroupBy)
	if groupBy == "" {
		groupBy = "day"
	}
	if !statusIn(groupBy, "day", "week", "month") {
		return CashflowReportRequest{}, validationError("Nhóm thời gian dòng tiền không hợp lệ", Record{"group_by": groupBy})
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "approved"
	}
	if !statusIn(status, "approved", "all") {
		return CashflowReportRequest{}, validationError("Bộ lọc trạng thái dòng tiền không hợp lệ", Record{"status": status})
	}
	return CashflowReportRequest{
		ProjectID: cleanID(req.ProjectID),
		FromDate:  from,
		ToDate:    to,
		GroupBy:   groupBy,
		Status:    status,
	}, nil
}

func normalizeProfitReportRequest(req ProfitReportRequest) (ProfitReportRequest, error) {
	now := time.Now()
	from := strings.TrimSpace(req.FromDate)
	to := strings.TrimSpace(req.ToDate)
	if from == "" {
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	}
	if to == "" {
		to = now.Format("2006-01-02")
	}
	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return ProfitReportRequest{}, validationError("Ngày bắt đầu không hợp lệ", Record{"from": from})
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return ProfitReportRequest{}, validationError("Ngày kết thúc không hợp lệ", Record{"to": to})
	}
	if toDate.Before(fromDate) {
		return ProfitReportRequest{}, validationError("Ngày kết thúc phải sau ngày bắt đầu", Record{"from": from, "to": to})
	}
	groupBy := strings.TrimSpace(req.GroupBy)
	if groupBy == "" {
		groupBy = "month"
	}
	if !statusIn(groupBy, "month", "quarter") {
		return ProfitReportRequest{}, validationError("Nhóm kỳ báo cáo lợi nhuận không hợp lệ", Record{"group_by": groupBy})
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "posted"
	}
	if !statusIn(status, "posted", "all") {
		return ProfitReportRequest{}, validationError("Bộ lọc trạng thái báo cáo lợi nhuận không hợp lệ", Record{"status": status})
	}
	return ProfitReportRequest{
		ProjectID: cleanID(req.ProjectID),
		FromDate:  from,
		ToDate:    to,
		GroupBy:   groupBy,
		Status:    status,
	}, nil
}

func normalizeTaxReportRequest(req TaxReportRequest) (TaxReportRequest, error) {
	now := time.Now()
	from := strings.TrimSpace(req.FromDate)
	to := strings.TrimSpace(req.ToDate)
	if from == "" {
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	}
	if to == "" {
		to = now.Format("2006-01-02")
	}
	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return TaxReportRequest{}, validationError("Ngày bắt đầu không hợp lệ", Record{"from": from})
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return TaxReportRequest{}, validationError("Ngày kết thúc không hợp lệ", Record{"to": to})
	}
	if toDate.Before(fromDate) {
		return TaxReportRequest{}, validationError("Ngày kết thúc phải sau ngày bắt đầu", Record{"from": from, "to": to})
	}
	groupBy := strings.TrimSpace(req.GroupBy)
	if groupBy == "" {
		groupBy = "month"
	}
	if !statusIn(groupBy, "month", "quarter") {
		return TaxReportRequest{}, validationError("Nhóm kỳ báo cáo thuế không hợp lệ", Record{"group_by": groupBy})
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "all"
	}
	if !statusIn(source, "all", "input", "output") {
		return TaxReportRequest{}, validationError("Nguồn hoá đơn báo cáo thuế không hợp lệ", Record{"source": source})
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "posted"
	}
	if !statusIn(status, "posted", "all") {
		return TaxReportRequest{}, validationError("Bộ lọc trạng thái báo cáo thuế không hợp lệ", Record{"status": status})
	}
	return TaxReportRequest{
		ProjectID: cleanID(req.ProjectID),
		FromDate:  from,
		ToDate:    to,
		GroupBy:   groupBy,
		Source:    source,
		Status:    status,
	}, nil
}
