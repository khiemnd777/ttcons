package core

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Service) Dashboard(ctx context.Context, req DashboardPeriodRequest, actorID string) (DashboardReportResponse, error) {
	_, meta, err := normalizeDashboardPeriodRequest(req)
	if err != nil {
		return DashboardReportResponse{}, err
	}
	snapshot, err := s.repo.DashboardReport(ctx, actorID)
	if err != nil {
		return DashboardReportResponse{}, err
	}
	return DashboardReportResponse{Meta: meta, ProjectSnapshot: snapshot}, nil
}

func (s *Service) DashboardProjects(ctx context.Context, req DashboardProjectsRequest, actorID string) (DashboardProjectsPage, error) {
	period, meta, err := normalizeDashboardPeriodRequest(req.DashboardPeriodRequest)
	if err != nil {
		return DashboardProjectsPage{}, err
	}
	req.DashboardPeriodRequest = period
	normalized, offset, filterHash, err := normalizeDashboardProjectsRequest(req, meta)
	if err != nil {
		return DashboardProjectsPage{}, err
	}
	page, err := s.repo.DashboardProjects(ctx, actorID, normalized, offset)
	if err != nil {
		return DashboardProjectsPage{}, err
	}
	if page.HasMore {
		page.NextCursor = encodeDashboardProjectsCursor(dashboardProjectsCursor{
			Version:    1,
			Offset:     offset + normalized.Limit,
			FilterHash: filterHash,
		})
	}
	return page, nil
}

var dashboardLocation = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)

func normalizeDashboardPeriodRequest(req DashboardPeriodRequest) (DashboardPeriodRequest, DashboardPeriodMeta, error) {
	return normalizeDashboardPeriodRequestAt(req, time.Now())
}

func normalizeDashboardPeriodRequestAt(req DashboardPeriodRequest, now time.Time) (DashboardPeriodRequest, DashboardPeriodMeta, error) {
	today := now.In(dashboardLocation)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, dashboardLocation)
	req.PeriodType = strings.TrimSpace(req.PeriodType)
	if req.PeriodType == "" {
		req.PeriodType = "month"
	}
	if !statusIn(req.PeriodType, "month", "quarter", "year") {
		return DashboardPeriodRequest{}, DashboardPeriodMeta{}, validationError("Loại kỳ Dashboard không hợp lệ", Record{"period_type": req.PeriodType})
	}
	if req.Year == 0 {
		req.Year = today.Year()
	}
	if req.Year < 1900 || req.Year > today.Year() {
		return DashboardPeriodRequest{}, DashboardPeriodMeta{}, validationError("Năm Dashboard không hợp lệ", Record{"year": req.Year})
	}

	var start, end time.Time
	switch req.PeriodType {
	case "month":
		if req.PeriodNo == 0 {
			req.PeriodNo = int(today.Month())
		}
		if req.PeriodNo < 1 || req.PeriodNo > 12 {
			return DashboardPeriodRequest{}, DashboardPeriodMeta{}, validationError("Tháng Dashboard không hợp lệ", Record{"period_no": req.PeriodNo})
		}
		start = time.Date(req.Year, time.Month(req.PeriodNo), 1, 0, 0, 0, 0, dashboardLocation)
		end = start.AddDate(0, 1, -1)
	case "quarter":
		if req.PeriodNo == 0 {
			req.PeriodNo = (int(today.Month())-1)/3 + 1
		}
		if req.PeriodNo < 1 || req.PeriodNo > 4 {
			return DashboardPeriodRequest{}, DashboardPeriodMeta{}, validationError("Quý Dashboard không hợp lệ", Record{"period_no": req.PeriodNo})
		}
		startMonth := time.Month((req.PeriodNo-1)*3 + 1)
		start = time.Date(req.Year, startMonth, 1, 0, 0, 0, 0, dashboardLocation)
		end = start.AddDate(0, 3, -1)
	case "year":
		req.PeriodNo = 0
		start = time.Date(req.Year, time.January, 1, 0, 0, 0, 0, dashboardLocation)
		end = time.Date(req.Year, time.December, 31, 0, 0, 0, 0, dashboardLocation)
	}
	if start.After(today) {
		return DashboardPeriodRequest{}, DashboardPeriodMeta{}, validationError("Không thể xem kỳ Dashboard trong tương lai", Record{"period_type": req.PeriodType, "year": req.Year, "period_no": req.PeriodNo})
	}
	isCurrent := !today.Before(start) && !today.After(end)
	to := end
	if isCurrent {
		to = today
	}
	groupBy := "month"
	if req.PeriodType == "month" {
		groupBy = "day"
	}
	return req, DashboardPeriodMeta{
		PeriodType:     req.PeriodType,
		Year:           req.Year,
		PeriodNo:       req.PeriodNo,
		FromDate:       start.Format("2006-01-02"),
		ToDate:         to.Format("2006-01-02"),
		AsOfDate:       today.Format("2006-01-02"),
		GroupBy:        groupBy,
		IsCurrent:      isCurrent,
		CanMoveForward: end.Before(today),
	}, nil
}

type dashboardProjectsCursor struct {
	Version    int    `json:"version"`
	Offset     int    `json:"offset"`
	FilterHash string `json:"filter_hash"`
}

func normalizeDashboardProjectsRequest(req DashboardProjectsRequest, meta DashboardPeriodMeta) (DashboardProjectsRequest, int, string, error) {
	req.Search = strings.TrimSpace(req.Search)
	if len([]rune(req.Search)) > 120 {
		return req, 0, "", validationError("Từ khóa tìm kiếm không được vượt quá 120 ký tự")
	}
	req.Status = strings.TrimSpace(req.Status)
	if len([]rune(req.Status)) > 60 {
		return req, 0, "", validationError("Trạng thái công trình không hợp lệ", Record{"status": req.Status})
	}
	req.Sort = strings.TrimSpace(req.Sort)
	if req.Sort == "" {
		req.Sort = "name"
	}
	if !statusIn(req.Sort, "name", "status", "progress") {
		return req, 0, "", validationError("Sắp xếp Dashboard không hợp lệ", Record{"sort": req.Sort})
	}
	req.Direction = strings.ToLower(strings.TrimSpace(req.Direction))
	if req.Direction == "" {
		req.Direction = "asc"
	}
	if !statusIn(req.Direction, "asc", "desc") {
		return req, 0, "", validationError("Chiều sắp xếp Dashboard không hợp lệ", Record{"direction": req.Direction})
	}
	if req.Limit == 0 {
		req.Limit = 25
	}
	if req.Limit < 1 || req.Limit > 50 {
		return req, 0, "", validationError("limit phải nằm trong khoảng 1 đến 50", Record{"limit": req.Limit})
	}
	filterHash := dashboardProjectsFilterHash(req, meta)
	req.Cursor = strings.TrimSpace(req.Cursor)
	if req.Cursor == "" {
		return req, 0, filterHash, nil
	}
	cursor, err := parseDashboardProjectsCursor(req.Cursor)
	if err != nil || cursor.Version != 1 || cursor.Offset < 0 || cursor.FilterHash != filterHash {
		return req, 0, "", validationError("cursor không hợp lệ hoặc không còn khớp bộ lọc", Record{"cursor": req.Cursor})
	}
	return req, cursor.Offset, filterHash, nil
}

func dashboardProjectsFilterHash(req DashboardProjectsRequest, meta DashboardPeriodMeta) string {
	raw := strings.Join([]string{
		meta.PeriodType,
		fmt.Sprint(meta.Year),
		fmt.Sprint(meta.PeriodNo),
		meta.FromDate,
		meta.ToDate,
		strings.ToLower(req.Search),
		req.Status,
		req.Sort,
		req.Direction,
		fmt.Sprint(req.Limit),
	}, "\x1f")
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum[:])
}

func encodeDashboardProjectsCursor(cursor dashboardProjectsCursor) string {
	raw, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func parseDashboardProjectsCursor(value string) (dashboardProjectsCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return dashboardProjectsCursor{}, err
	}
	cursor := dashboardProjectsCursor{}
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return dashboardProjectsCursor{}, err
	}
	if len(cursor.FilterHash) != sha256.Size*2 {
		return dashboardProjectsCursor{}, errors.New("invalid dashboard cursor")
	}
	return cursor, nil
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
	return s.projectOptions(ctx, req, actorID, "accounting_handoff.view")
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
	return []string{"report.view"}
}
