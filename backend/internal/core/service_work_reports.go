package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *Service) Statistic(ctx context.Context, kind string, req OperationalReportRequest, actorID string) ([]Record, error) {
	req.ProjectID = cleanID(req.ProjectID)
	if req.ProjectID != "" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, req.ProjectID, []string{"statistics.view"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem thống kê của công trình này", Record{"permissions": []string{"statistics.view"}})
		}
	}
	rows, err := s.repo.StatisticsReport(ctx, kind, actorID, req.ProjectID)
	if err != nil {
		return nil, err
	}
	return filterAndSortStatisticRows(kind, rows, req)
}

func filterAndSortStatisticRows(kind string, rows []Record, req OperationalReportRequest) ([]Record, error) {
	definition, err := statisticDefinition(kind)
	if err != nil {
		return nil, err
	}
	search := strings.ToLower(strings.TrimSpace(req.Search))
	if len([]rune(search)) > 120 {
		return nil, validationError("Từ khóa tìm kiếm không được vượt quá 120 ký tự")
	}
	filtered := make([]Record, 0, len(rows))
	for _, row := range rows {
		if search == "" || statisticRowContains(row, definition.Columns, search) {
			filtered = append(filtered, row)
		}
	}

	sortKey := strings.TrimSpace(req.Sort)
	if sortKey == "" {
		return filtered, nil
	}
	direction := strings.ToLower(strings.TrimSpace(req.Direction))
	if direction == "" {
		direction = "asc"
	}
	if direction != "asc" && direction != "desc" {
		return nil, validationError("Chiều sắp xếp thống kê không hợp lệ", Record{"direction": direction})
	}
	var sortColumn *tabularColumn
	for index := range definition.Columns {
		if definition.Columns[index].Key == sortKey {
			sortColumn = &definition.Columns[index]
			break
		}
	}
	if sortColumn == nil {
		return nil, validationError("Cột sắp xếp thống kê không hợp lệ", Record{"sort": sortKey})
	}
	sort.SliceStable(filtered, func(leftIndex, rightIndex int) bool {
		leftValue := filtered[leftIndex][sortColumn.Key]
		rightValue := filtered[rightIndex][sortColumn.Key]
		leftEmpty := strings.TrimSpace(exportCellText(leftValue)) == ""
		rightEmpty := strings.TrimSpace(exportCellText(rightValue)) == ""
		if leftEmpty || rightEmpty {
			return !leftEmpty && rightEmpty
		}
		comparison := compareStatisticValues(leftValue, rightValue, sortColumn.Numeric)
		if direction == "desc" {
			return comparison > 0
		}
		return comparison < 0
	})
	return filtered, nil
}

func statisticRowContains(row Record, columns []tabularColumn, search string) bool {
	for _, column := range columns {
		if strings.Contains(strings.ToLower(exportCellText(row[column.Key])), search) {
			return true
		}
	}
	return false
}

func compareStatisticValues(left, right any, numeric bool) int {
	leftText := strings.TrimSpace(exportCellText(left))
	rightText := strings.TrimSpace(exportCellText(right))
	if leftText == "" && rightText == "" {
		return 0
	}
	if leftText == "" {
		return 1
	}
	if rightText == "" {
		return -1
	}
	if numeric {
		leftNumber, leftOK := exportNumber(left)
		rightNumber, rightOK := exportNumber(right)
		if leftOK && rightOK {
			switch {
			case leftNumber < rightNumber:
				return -1
			case leftNumber > rightNumber:
				return 1
			default:
				return 0
			}
		}
	}
	return strings.Compare(strings.ToLower(leftText), strings.ToLower(rightText))
}

func (s *Service) ExportStatistic(ctx context.Context, kind, format string, req OperationalReportRequest, actorID, actorName string) (tabularExportResult, error) {
	rows, err := s.Statistic(ctx, kind, req, actorID)
	if err != nil {
		return tabularExportResult{}, err
	}
	definition, err := statisticDefinition(kind)
	if err != nil {
		return tabularExportResult{}, err
	}
	result, err := generateTabularExport(definition, rows, format, time.Now())
	if err != nil {
		return tabularExportResult{}, err
	}
	s.repo.AddAuditBestEffort(ctx, cleanID(req.ProjectID), actorID, actorName, "statistics_exported", "statistics", "", "Xuất dữ liệu thống kê", Record{"kind": kind, "format": strings.ToLower(strings.TrimSpace(format)), "project_id": cleanID(req.ProjectID), "row_count": len(rows)})
	return result, nil
}

func (s *Service) WorkReports(ctx context.Context, filter WorkReportFilter, actorID string) ([]Record, error) {
	normalized, err := normalizeWorkReportFilter(filter)
	if err != nil {
		return nil, err
	}
	if normalized.Scope == "review" {
		allowed, err := s.repo.UserHasAnyPermission(ctx, actorID, []string{"work_report.review", "work_report.manage"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem hộp báo cáo chờ phản hồi")
		}
	}
	if normalized.Scope == "all" {
		allowed, err := s.repo.UserHasAnyPermission(ctx, actorID, []string{"work_report.manage"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem toàn bộ báo cáo")
		}
	}
	if normalized.ProjectID != "" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, normalized.ProjectID, []string{"work_report.view"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem báo cáo của công trình này", Record{"permissions": []string{"work_report.view"}})
		}
	}
	return s.repo.WorkReports(ctx, actorID, normalized)
}

func normalizeWorkReportFilter(filter WorkReportFilter) (WorkReportFilter, error) {
	filter.Scope = strings.TrimSpace(filter.Scope)
	if filter.Scope == "" {
		filter.Scope = "mine"
	}
	if filter.Scope != "mine" && filter.Scope != "review" && filter.Scope != "all" {
		return filter, validationError("Phạm vi báo cáo không hợp lệ", Record{"scope": filter.Scope})
	}
	filter.ProjectID = cleanID(filter.ProjectID)
	if filter.ProjectID != "" && !validActivityUUID(filter.ProjectID) {
		return filter, validationError("project_id không hợp lệ", Record{"project_id": filter.ProjectID})
	}
	filter.Status = strings.TrimSpace(filter.Status)
	if filter.Status != "" && !workReportContains([]string{"draft", "recorded", "submitted", "needs_changes", "acknowledged"}, filter.Status) {
		return filter, validationError("Trạng thái báo cáo không hợp lệ", Record{"status": filter.Status})
	}
	filter.DateFrom = strings.TrimSpace(filter.DateFrom)
	filter.DateTo = strings.TrimSpace(filter.DateTo)
	for key, value := range map[string]string{"date_from": filter.DateFrom, "date_to": filter.DateTo} {
		if value == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return filter, validationError(key+" không hợp lệ", Record{key: value})
		}
	}
	if filter.DateFrom != "" && filter.DateTo != "" && filter.DateFrom > filter.DateTo {
		return filter, validationError("Khoảng ngày báo cáo không hợp lệ", Record{"date_from": filter.DateFrom, "date_to": filter.DateTo})
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if len([]rune(filter.Search)) > 120 {
		return filter, validationError("Từ khóa tìm kiếm không được vượt quá 120 ký tự")
	}
	return filter, nil
}

func (s *Service) WorkReportDetail(ctx context.Context, sourceID, actorID string) (Record, error) {
	sourceID = cleanID(sourceID)
	if !validActivityUUID(sourceID) {
		return nil, validationError("Báo cáo không hợp lệ", Record{"source_id": sourceID})
	}
	detail, err := s.repo.WorkReportSource(ctx, sourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, validationError("Không tìm thấy báo cáo", Record{"source_id": sourceID})
	}
	if err != nil {
		return nil, err
	}
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, cleanID(detail["project_id"]), []string{"work_report.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem báo cáo này")
	}
	canManage, err := s.repo.UserHasAnyProjectPermission(ctx, actorID, cleanID(detail["project_id"]), []string{"work_report.manage"})
	if err != nil {
		return nil, err
	}
	if !canManage && actorID != cleanID(detail["author_id"]) && actorID != cleanID(detail["recipient_id"]) {
		return nil, forbiddenError("Báo cáo này không thuộc phạm vi cá nhân của bạn")
	}
	if cleanID(detail["work_report_id"]) == "" {
		if boolValue(detail["report_delivery_eligible"]) {
			detail["status"] = "draft"
		} else {
			detail["status"] = "recorded"
		}
		detail["revisions"] = []Record{}
		detail["events"] = []Record{}
		return detail, nil
	}
	revisions, events, err := s.repo.WorkReportHistory(ctx, cleanID(detail["work_report_id"]))
	if err != nil {
		return nil, err
	}
	detail["revisions"] = revisions
	detail["events"] = events
	return detail, nil
}

func (s *Service) WorkReportRecipients(ctx context.Context, projectID, actorID string) ([]Record, error) {
	projectID = cleanID(projectID)
	if !validActivityUUID(projectID) {
		return nil, validationError("project_id không hợp lệ", Record{"project_id": projectID})
	}
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"work_report.submit"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền gửi báo cáo của công trình này")
	}
	return s.repo.WorkReportRecipients(ctx, projectID, actorID)
}

func (s *Service) WorkReportProjects(ctx context.Context, req ReportOptionsRequest, actorID string) (ReportOptionsResponse, error) {
	return s.projectOptions(ctx, req, actorID, "work_report.view")
}

func (s *Service) StatisticsProjects(ctx context.Context, req ReportOptionsRequest, actorID string) (ReportOptionsResponse, error) {
	return s.projectOptions(ctx, req, actorID, "statistics.view")
}

func (s *Service) SubmitWorkReport(ctx context.Context, sourceID string, req WorkReportSubmitRequest, actorID, actorName string) (Record, error) {
	sourceID = cleanID(sourceID)
	req.RecipientID = cleanID(req.RecipientID)
	req.Comment = strings.TrimSpace(req.Comment)
	if !validActivityUUID(sourceID) {
		return nil, validationError("Báo cáo không hợp lệ", Record{"source_id": sourceID})
	}
	if !validActivityUUID(req.RecipientID) {
		return nil, validationError("Người nhận báo cáo là bắt buộc", Record{"recipient_id": "required"})
	}
	if len([]rune(req.Comment)) > 2000 {
		return nil, validationError("Ghi chú gửi báo cáo không được vượt quá 2000 ký tự")
	}
	source, err := s.repo.WorkReportSource(ctx, sourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, validationError("Không tìm thấy báo cáo", Record{"source_id": sourceID})
	}
	if err != nil {
		return nil, err
	}
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, cleanID(source["project_id"]), []string{"work_report.submit"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền gửi báo cáo này")
	}
	return s.repo.SubmitWorkReport(ctx, sourceID, req.RecipientID, req.Comment, actorID, actorName)
}

func (s *Service) ReviewWorkReport(ctx context.Context, reportID, action string, req WorkReportReviewRequest, actorID, actorName string) (Record, error) {
	reportID = cleanID(reportID)
	req.Comment = strings.TrimSpace(req.Comment)
	if !validActivityUUID(reportID) {
		return nil, validationError("Báo cáo không hợp lệ", Record{"report_id": reportID})
	}
	if len([]rune(req.Comment)) > 2000 {
		return nil, validationError("Ý kiến xem xét không được vượt quá 2000 ký tự")
	}
	header, err := s.repo.WorkReportHeader(ctx, reportID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, validationError("Không tìm thấy báo cáo", Record{"report_id": reportID})
	}
	if err != nil {
		return nil, err
	}
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, cleanID(header["project_id"]), []string{"work_report.review", "work_report.manage"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem xét báo cáo này")
	}
	canManage, err := s.repo.UserHasAnyProjectPermission(ctx, actorID, cleanID(header["project_id"]), []string{"work_report.manage"})
	if err != nil {
		return nil, err
	}
	return s.repo.ReviewWorkReport(ctx, reportID, action, req.Comment, actorID, actorName, canManage)
}

func (s *Service) ExportWorkReports(ctx context.Context, filter WorkReportFilter, format, actorID, actorName string) (tabularExportResult, error) {
	rows, err := s.WorkReports(ctx, filter, actorID)
	if err != nil {
		return tabularExportResult{}, err
	}
	definition := tabularDefinition{
		Title: "Sổ đăng ký báo cáo công việc", FileSlug: "so-dang-ky-bao-cao",
		Columns: []tabularColumn{{"report_date", "Ngày", false}, {"project_name", "Công trình", false}, {"report_type_label", "Loại báo cáo", false}, {"author_name", "Người lập", false}, {"recipient_name", "Gửi đến", false}, {"status_label", "Trạng thái", false}, {"current_revision_no", "Lần gửi", true}, {"submitted_at", "Gửi lúc", false}},
	}
	for _, row := range rows {
		row["report_type_label"] = "Báo cáo thi công ngày"
		row["status_label"] = workReportStatusLabel(fmt.Sprint(row["status"]))
	}
	result, err := generateTabularExport(definition, rows, format, time.Now())
	if err != nil {
		return tabularExportResult{}, err
	}
	s.repo.AddAuditBestEffort(ctx, cleanID(filter.ProjectID), actorID, actorName, "work_reports_exported", "work_report", "", "Xuất sổ đăng ký báo cáo", Record{"format": strings.ToLower(strings.TrimSpace(format)), "scope": filter.Scope, "project_id": cleanID(filter.ProjectID), "row_count": len(rows)})
	return result, nil
}

func (s *Service) ExportWorkReportPDF(ctx context.Context, reportID string, revisionNo int, actorID, actorName string) (tabularExportResult, error) {
	reportID = cleanID(reportID)
	if !validActivityUUID(reportID) || revisionNo < 1 {
		return tabularExportResult{}, validationError("Bản báo cáo không hợp lệ", Record{"report_id": reportID, "revision_no": revisionNo})
	}
	header, err := s.repo.WorkReportHeader(ctx, reportID)
	if errors.Is(err, sql.ErrNoRows) {
		return tabularExportResult{}, validationError("Không tìm thấy báo cáo", Record{"report_id": reportID})
	}
	if err != nil {
		return tabularExportResult{}, err
	}
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, cleanID(header["project_id"]), []string{"work_report.export"})
	if err != nil {
		return tabularExportResult{}, err
	}
	if !allowed {
		return tabularExportResult{}, forbiddenError("Bạn không có quyền xuất báo cáo này")
	}
	canManage, err := s.repo.UserHasAnyProjectPermission(ctx, actorID, cleanID(header["project_id"]), []string{"work_report.manage"})
	if err != nil {
		return tabularExportResult{}, err
	}
	if !canManage && actorID != cleanID(header["author_id"]) && actorID != cleanID(header["recipient_id"]) {
		return tabularExportResult{}, forbiddenError("Báo cáo này không thuộc phạm vi cá nhân của bạn")
	}
	revision, err := s.repo.WorkReportRevision(ctx, reportID, revisionNo)
	if errors.Is(err, sql.ErrNoRows) {
		return tabularExportResult{}, validationError("Không tìm thấy lần gửi báo cáo", Record{"revision_no": revisionNo})
	}
	if err != nil {
		return tabularExportResult{}, err
	}
	result, err := generateWorkReportPDF(revision)
	if err != nil {
		return tabularExportResult{}, err
	}
	s.repo.AddAuditBestEffort(ctx, cleanID(header["project_id"]), actorID, actorName, "work_report_exported", "work_report", reportID, "Xuất PDF báo cáo công việc", Record{"revision_no": revisionNo, "format": "pdf"})
	return result, nil
}

func workReportStatusLabel(status string) string {
	return map[string]string{"draft": "Bản nháp", "recorded": "Đã ghi nhận", "submitted": "Chờ xác nhận", "needs_changes": "Yêu cầu bổ sung", "acknowledged": "Đã xác nhận"}[status]
}

func workReportContains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
