package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"strconv"
	"strings"
	"time"
)

func (s *Service) ProjectDailyReports(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.ListProjectDailyReports(ctx, cleanID(projectID))
}

func (s *Service) CreateDailyReport(ctx context.Context, projectID string, req ConstructionDailyReportCreateRequest, actorID, actorName string) (Record, error) {
	body, err := s.normalizeConstructionDailyReport(ctx, projectID, req, actorID, nil)
	if err != nil {
		return nil, err
	}
	delivery, err := s.normalizeConstructionReportDelivery(ctx, projectID, req.Delivery, actorID)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.CreateDailyReport(ctx, body, delivery, actorID, actorName)
	if err != nil {
		return nil, err
	}
	transition := recordFromAny(row["work_item_transition"])
	if boolValue(transition["auto_started"]) {
		s.publishWorkItemChanged(Record{"id": transition["work_item_id"], "project_id": row["project_id"]}, actorID)
	}
	s.publishDailyReportDeliveryChanged(row, actorID)
	return row, nil
}

func (s *Service) normalizeConstructionDailyReport(ctx context.Context, projectID string, req ConstructionDailyReportCreateRequest, actorID string, existingEvidenceIDs map[string]bool) (Record, error) {
	projectID = cleanID(projectID)
	reportDate := strings.TrimSpace(req.ReportDate)
	if _, err := time.Parse("2006-01-02", reportDate); err != nil {
		return nil, validationError("Ngày báo cáo không hợp lệ", Record{"report_date": req.ReportDate})
	}
	weather := strings.TrimSpace(req.Weather)
	if weather == "" {
		return nil, validationError("Thời tiết là bắt buộc", Record{"weather": "required"})
	}
	if req.ProgressPercent < 0 || req.ProgressPercent > 100 {
		return nil, validationError("Tiến độ báo cáo phải từ 0 đến 100", Record{"progress_percent": req.ProgressPercent})
	}

	doneItems, doneTitles, err := normalizeConstructionReportWorkItems(req.DoneItems, "done_items", "done", 250)
	if err != nil {
		return nil, err
	}
	if len(doneItems) == 0 {
		return nil, validationError("Báo cáo cần ít nhất một hạng mục đã làm", Record{"done_items": "required"})
	}
	currentItems, currentTitles, err := normalizeConstructionReportWorkItems(req.CurrentItems, "current_items", "partial", 250)
	if err != nil {
		return nil, err
	}
	nextItems, nextTitles, err := normalizeConstructionReportNextItems(req.NextItems)
	if err != nil {
		return nil, err
	}
	blockers, err := normalizeConstructionReportBlockers(req.Blockers)
	if err != nil {
		return nil, err
	}
	crewItems, manpowerSummary, err := normalizeConstructionReportCrew(req.CrewItems)
	if err != nil {
		return nil, err
	}
	if manpowerSummary == "" {
		manpowerSummary, err = normalizeConstructionLegacySummary(req.ManpowerSummary, "manpower_summary")
		if err != nil {
			return nil, err
		}
	}
	materialItems, materialSummary, err := normalizeConstructionReportMaterials(req.MaterialItems)
	if err != nil {
		return nil, err
	}
	if materialSummary == "" {
		materialSummary, err = normalizeConstructionLegacySummary(req.MaterialSummary, "material_usage_summary")
		if err != nil {
			return nil, err
		}
	}

	workItemID := cleanID(req.WorkItemID)
	if workItemID == "" {
		return nil, validationError("Báo cáo thi công cần chọn hạng mục đang thực hiện", Record{"work_item_id": "required"})
	}
	workItem, err := s.requireActiveConstructionWorkItem(ctx, projectID, workItemID)
	if err != nil {
		return nil, err
	}
	canAct, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canAct {
		return nil, forbiddenError("Bạn không có quyền thao tác hạng mục thi công này")
	}
	if err := s.requireWorkItemPermission(ctx, workItem, actorID); err != nil {
		return nil, err
	}

	evidenceIDs := normalizeConstructionReportEvidenceIDs(req.EvidenceIDs, req.PrimaryEvidenceID)
	if len(evidenceIDs) == 0 {
		return nil, validationError("Báo cáo thi công cần ít nhất một ảnh hiện trường", Record{"evidence_ids": "photo_required"})
	}
	if len(evidenceIDs) > 50 {
		return nil, validationError("Báo cáo chỉ được liên kết tối đa 50 minh chứng", Record{"evidence_ids": len(evidenceIDs), "max": 50})
	}
	if len(evidenceIDs) > 0 && workItemID == "" {
		return nil, validationError("Chọn công việc liên quan trước khi liên kết minh chứng", Record{"work_item_id": "required"})
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	photoCount := 0
	for _, evidenceID := range evidenceIDs {
		if !validActivityUUID(evidenceID) {
			return nil, validationError("Minh chứng không hợp lệ", Record{"evidence_id": evidenceID})
		}
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, validationError("Không tìm thấy minh chứng", Record{"evidence_id": evidenceID})
		}
		if err != nil {
			return nil, err
		}
		if cleanID(evidence["project_id"]) != projectID || cleanID(evidence["work_item_id"]) != workItemID {
			return nil, validationError("Minh chứng không thuộc công việc liên quan của báo cáo", Record{"evidence_id": evidenceID, "work_item_id": workItemID})
		}
		if fmt.Sprint(evidence["status"]) == "rejected" && !existingEvidenceIDs[evidenceID] {
			return nil, validationError("Không thể liên kết minh chứng đã bị từ chối", Record{"evidence_id": evidenceID})
		}
		if profileIsClient(profile) && !profile.IsManager && fmt.Sprint(evidence["visibility"]) != "client" {
			return nil, forbiddenError("Bạn không có quyền liên kết minh chứng này")
		}
		if fmt.Sprint(evidence["evidence_type"]) == "photo" && fmt.Sprint(evidence["status"]) != "rejected" {
			photoCount++
		}
	}
	if photoCount == 0 {
		return nil, validationError("Báo cáo thi công cần ít nhất một minh chứng loại ảnh", Record{"evidence_ids": "photo_required"})
	}
	primaryEvidenceID := cleanID(req.PrimaryEvidenceID)
	if primaryEvidenceID == "" && len(evidenceIDs) > 0 {
		primaryEvidenceID = evidenceIDs[0]
	}

	completedItems := append(doneItems, currentItems...)
	assessments, err := normalizeConstructionSupervisionAssessments(req.SupervisionAssessments)
	if err != nil {
		return nil, err
	}
	arrivals, err := normalizeConstructionMaterialArrivals(req.MaterialArrivalItems)
	if err != nil {
		return nil, err
	}
	structured := Record{
		"completed_items":         completedItems,
		"next_plan_items":         nextItems,
		"blockers":                blockers,
		"crew_items":              crewItems,
		"material_usage_items":    materialItems,
		"supervision_assessments": assessments,
		"material_arrival_items":  arrivals,
	}
	if primaryEvidenceID != "" {
		structured["site_photo_evidence_id"] = primaryEvidenceID
	}
	body := Record{
		"project_id":             projectID,
		"report_date":            reportDate,
		"reported_by":            actorID,
		"weather":                weather,
		"work_summary":           constructionReportWorkSummary(doneTitles, currentTitles, nextTitles),
		"manpower_summary":       manpowerSummary,
		"material_usage_summary": materialSummary,
		"issues_summary":         strings.TrimSpace(req.FieldNotes),
		"progress_percent":       req.ProgressPercent,
		"photos":                 evidenceIDs,
		"work_item_id":           workItemID,
		"structured_json":        structured,
		"primary_evidence_id":    primaryEvidenceID,
	}
	return body, nil
}

func (s *Service) UpdateDailyReport(ctx context.Context, reportID string, req ConstructionDailyReportCreateRequest, actorID, actorName string) (Record, error) {
	reportID = cleanID(reportID)
	if !validActivityUUID(reportID) {
		return nil, validationError("Báo cáo thi công không hợp lệ", Record{"report_id": reportID})
	}
	current, err := s.repo.GetByID(ctx, "operation_daily_reports", reportID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, validationError("Không tìm thấy báo cáo thi công", Record{"report_id": reportID})
	}
	if err != nil {
		return nil, err
	}
	if cleanID(current["workflow_form_instance_id"]) != "" {
		return nil, validationError("Báo cáo từ quy trình phải được cập nhật tại công việc liên quan", Record{"workflow_form_instance_id": current["workflow_form_instance_id"]})
	}
	if !boolValue(current["report_delivery_eligible"]) {
		return nil, validationError("Nhật ký lịch sử này chỉ dùng để tra cứu và không thể chỉnh sửa", Record{"report_id": reportID})
	}
	workReportStatus, err := s.repo.WorkReportStatusForSource(ctx, reportID)
	if err != nil {
		return nil, err
	}
	if workReportStatus != "" && workReportStatus != "needs_changes" {
		return nil, validationError("Báo cáo chỉ được chỉnh sửa khi chưa gửi hoặc đang được yêu cầu bổ sung", Record{"report_id": reportID, "status": workReportStatus})
	}
	existingEvidenceIDs := map[string]bool{}
	for _, evidenceID := range constructionReportStringList(current["photos"]) {
		existingEvidenceIDs[evidenceID] = true
	}
	body, err := s.normalizeConstructionDailyReport(ctx, cleanID(current["project_id"]), req, actorID, existingEvidenceIDs)
	if err != nil {
		return nil, err
	}
	// Resolve omitted sections from the current source under its transaction lock.
	// An explicit empty list still means the user removed the section.
	omittedSections := []string{}
	if req.SupervisionAssessments == nil {
		omittedSections = append(omittedSections, "supervision_assessments")
	}
	if req.MaterialArrivalItems == nil {
		omittedSections = append(omittedSections, "material_arrival_items")
	}
	delivery, err := s.normalizeConstructionReportDelivery(ctx, cleanID(current["project_id"]), req.Delivery, actorID)
	if err != nil {
		return nil, err
	}
	delete(body, "project_id")
	delete(body, "reported_by")
	row, err := s.repo.UpdateDailyReportWithDelivery(ctx, reportID, body, delivery, actorID, actorName, omittedSections...)
	if err != nil {
		return nil, err
	}
	s.publishDailyReportDeliveryChanged(row, actorID)
	return row, nil
}

func (s *Service) publishDailyReportDeliveryChanged(row Record, actorID string) {
	projectID := cleanID(row["project_id"])
	transition := recordFromAny(row["delivery_transition"])
	recipientID := cleanID(transition["recipient_id"])
	s.publishWorkReportChanged(projectID, actorID, recipientID)
}

func (s *Service) normalizeConstructionReportDelivery(ctx context.Context, projectID string, req WorkReportDeliveryRequest, actorID string) (WorkReportDeliveryRequest, error) {
	req.Mode = strings.ToLower(strings.TrimSpace(req.Mode))
	if req.Mode == "" {
		req.Mode = "draft"
	}
	if req.Mode != "draft" && req.Mode != "submit" {
		return req, validationError("Cách lưu báo cáo không hợp lệ", Record{"delivery.mode": req.Mode})
	}
	req.RecipientID = cleanID(req.RecipientID)
	req.Comment = strings.TrimSpace(req.Comment)
	if len([]rune(req.Comment)) > 2000 {
		return req, validationError("Ghi chú gửi báo cáo không được vượt quá 2000 ký tự", Record{"delivery.comment": "too_long"})
	}
	if req.Mode == "draft" {
		req.RecipientID = ""
		return req, nil
	}
	if !validActivityUUID(req.RecipientID) {
		return req, validationError("Người nhận báo cáo là bắt buộc", Record{"delivery.recipient_id": "required"})
	}
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, cleanID(projectID), []string{"work_report.submit"})
	if err != nil {
		return req, err
	}
	if !allowed {
		return req, forbiddenError("Bạn không có quyền gửi báo cáo của công trình này")
	}
	return req, nil
}

func (s *Service) AddDailyReportEvidenceUpload(ctx context.Context, reportID, requestedWorkItemID string, req WorkItemEvidenceRequest, fileHeader *multipart.FileHeader, actorID, actorName string) (Record, error) {
	reportID = cleanID(reportID)
	if !validActivityUUID(reportID) {
		return nil, validationError("Báo cáo thi công không hợp lệ", Record{"report_id": reportID})
	}
	report, err := s.repo.GetByID(ctx, "operation_daily_reports", reportID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, validationError("Không tìm thấy báo cáo thi công", Record{"report_id": reportID})
	}
	if err != nil {
		return nil, err
	}
	if cleanID(report["workflow_form_instance_id"]) != "" {
		return nil, validationError("Báo cáo từ quy trình phải bổ sung minh chứng tại công việc liên quan", Record{"workflow_form_instance_id": report["workflow_form_instance_id"]})
	}
	if !boolValue(report["report_delivery_eligible"]) {
		return nil, validationError("Nhật ký lịch sử này chỉ dùng để tra cứu và không thể bổ sung minh chứng", Record{"report_id": reportID})
	}
	workReportStatus, err := s.repo.WorkReportStatusForSource(ctx, reportID)
	if err != nil {
		return nil, err
	}
	if workReportStatus != "" && workReportStatus != "needs_changes" {
		return nil, validationError("Báo cáo chỉ được bổ sung minh chứng khi chưa gửi hoặc đang được yêu cầu bổ sung", Record{"report_id": reportID, "status": workReportStatus})
	}

	workItemID := cleanID(report["work_item_id"])
	requestedWorkItemID = cleanID(requestedWorkItemID)
	if workItemID != "" && requestedWorkItemID != "" && requestedWorkItemID != workItemID {
		return nil, validationError("Công việc liên quan không khớp với báo cáo", Record{"work_item_id": requestedWorkItemID, "expected_work_item_id": workItemID})
	}
	if workItemID == "" {
		workItemID = requestedWorkItemID
	}
	if workItemID == "" || !validActivityUUID(workItemID) {
		return nil, validationError("Chọn công việc liên quan trước khi tải minh chứng", Record{"work_item_id": "required"})
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, validationError("Không tìm thấy công việc liên quan", Record{"work_item_id": workItemID})
	}
	if err != nil {
		return nil, err
	}
	projectID := cleanID(report["project_id"])
	if cleanID(item["project_id"]) != projectID || fmt.Sprint(item["status"]) == "cancelled" {
		return nil, validationError("Công việc liên quan không thuộc công trình hoặc đã bị hủy", Record{"work_item_id": workItemID})
	}
	photos := constructionReportStringList(report["photos"])
	if len(photos) >= 50 {
		return nil, validationError("Báo cáo chỉ được liên kết tối đa 50 minh chứng", Record{"evidence_ids": len(photos), "max": 50})
	}
	if strings.TrimSpace(req.Visibility) == "" {
		req.Visibility = "project"
	}
	evidence, err := s.AddWorkItemEvidenceUpload(ctx, workItemID, req, fileHeader, actorID, actorName)
	if err != nil {
		return nil, err
	}
	evidenceID := cleanID(evidence["id"])
	photos = normalizeConstructionReportEvidenceIDs(append(photos, evidenceID), "")
	primaryEvidenceID := cleanID(report["primary_evidence_id"])
	if primaryEvidenceID == "" {
		primaryEvidenceID = evidenceID
	}
	structured := constructionReportRecord(report["structured_json"])
	if cleanID(structured["site_photo_evidence_id"]) == "" {
		structured["site_photo_evidence_id"] = primaryEvidenceID
	}
	updated, err := s.repo.UpdateDailyReport(ctx, reportID, Record{
		"photos":              photos,
		"work_item_id":        workItemID,
		"structured_json":     structured,
		"primary_evidence_id": primaryEvidenceID,
	})
	if err != nil {
		return nil, err
	}
	s.repo.AddAuditBestEffort(ctx, projectID, actorID, actorName, "daily_report_evidence_added", "daily_report", reportID, fmt.Sprintf("Thêm minh chứng %s vào nhật ký ngày %s", evidence["name"], report["report_date"]), Record{
		"work_item_id": workItemID,
		"evidence_id":  evidenceID,
	})
	return updated, nil
}

func normalizeConstructionLegacySummary(value, field string) (string, error) {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > 4000 {
		return "", validationError("Nội dung tóm tắt tối đa 4000 ký tự", Record{field: "too_long"})
	}
	return value, nil
}

func constructionReportRecord(value any) Record {
	switch typed := value.(type) {
	case Record:
		return typed
	case map[string]any:
		return Record(typed)
	case string:
		row := Record{}
		if json.Unmarshal([]byte(typed), &row) == nil {
			return row
		}
	}
	return Record{}
}

func constructionReportStringList(value any) []string {
	values := []string{}
	switch typed := value.(type) {
	case []string:
		values = append(values, typed...)
	case []any:
		for _, item := range typed {
			values = append(values, fmt.Sprint(item))
		}
	case string:
		_ = json.Unmarshal([]byte(typed), &values)
	}
	return normalizeConstructionReportEvidenceIDs(values, "")
}

func normalizeConstructionReportWorkItems(items []ConstructionDailyReportWorkItemRequest, field, result string, maxItems int) ([]Record, []string, error) {
	rows := make([]Record, 0, len(items))
	titles := make([]string, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}
		if len([]rune(title)) > 180 {
			return nil, nil, validationError("Tên hạng mục tối đa 180 ký tự", Record{field: title})
		}
		rows = append(rows, Record{"title": title, "result": result})
		titles = append(titles, title)
	}
	if len(rows) > maxItems {
		return nil, nil, validationError("Báo cáo có quá nhiều hạng mục", Record{field: len(rows), "max": maxItems})
	}
	return rows, titles, nil
}

func normalizeConstructionReportNextItems(items []ConstructionDailyReportWorkItemRequest) ([]Record, []string, error) {
	rows := make([]Record, 0, len(items))
	titles := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		title, err := nativeCaptureText(item.Title, "next_items.title", 180)
		if err != nil {
			return nil, nil, err
		}
		row := Record{"title": title}
		if err := normalizeConstructionNextCapture(row, item); err != nil {
			return nil, nil, err
		}
		if title == "" {
			if constructionRowHasCaptureFacts(row, constructionNextCaptureKeys) {
				return nil, nil, validationError("Dòng kế hoạch cần tên công việc", Record{"field": "next_items.title"})
			}
			continue
		}
		rowID, err := constructionCaptureRowID(item.RowID, seen)
		if err != nil {
			return nil, nil, err
		}
		row["row_id"] = rowID
		rows = append(rows, row)
		titles = append(titles, title)
	}
	if len(rows) > 120 {
		return nil, nil, validationError("Báo cáo có quá nhiều hạng mục", Record{"field": "next_items", "max": 120})
	}
	return rows, titles, nil
}

func normalizeConstructionReportBlockers(items []ConstructionDailyReportBlockerRequest) ([]Record, error) {
	rows := make([]Record, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		content := strings.TrimSpace(item.Content)
		if len([]rune(content)) > 1000 {
			return nil, validationError("Nội dung blocker tối đa 1000 ký tự", Record{"blockers": content})
		}
		row := Record{"content": content}
		if err := normalizeConstructionBlockerCapture(row, item); err != nil {
			return nil, err
		}
		if content == "" {
			if constructionRowHasCaptureFacts(row, constructionBlockerCaptureKeys) {
				return nil, validationError("Dòng rủi ro cần nội dung", Record{"field": "blockers.content"})
			}
			continue
		}
		rowID, err := constructionCaptureRowID(item.RowID, seen)
		if err != nil {
			return nil, err
		}
		row["row_id"] = rowID
		rows = append(rows, row)
	}
	if len(rows) > 120 {
		return nil, validationError("Báo cáo có quá nhiều blocker", Record{"blockers": len(rows), "max": 120})
	}
	return rows, nil
}

func normalizeConstructionReportCrew(items []ConstructionDailyReportCrewRequest) ([]Record, string, error) {
	rows := make([]Record, 0, len(items))
	seenIDs := map[string]bool{}
	summaries := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.CrewName)
		if name == "" && item.WorkerCount == 0 && item.WorkHours == 0 && strings.TrimSpace(item.Note) == "" && item.Representative == nil && item.Contact == nil {
			continue
		}
		if name == "" || item.WorkerCount < 1 || item.WorkHours < 0 {
			return nil, "", validationError("Nhân công cần tên tổ đội, số người lớn hơn 0 và giờ công không âm", Record{"crew_name": name, "worker_count": item.WorkerCount, "work_hours": item.WorkHours})
		}
		rowID := cleanID(item.RowID)
		if rowID == "" {
			rowID = newID()
		}
		if !validActivityUUID(rowID) || seenIDs[rowID] {
			return nil, "", validationError("Mã tổ đội không hợp lệ hoặc trùng", nil)
		}
		seenIDs[rowID] = true
		row := Record{"row_id": rowID, "crew_name": name, "worker_count": item.WorkerCount, "work_hours": item.WorkHours, "note": strings.TrimSpace(item.Note)}
		if err := normalizeConstructionCrewContact(row, item.Representative, item.Contact); err != nil {
			return nil, "", err
		}
		rows = append(rows, row)
		summary := fmt.Sprintf("%s · %d người", name, item.WorkerCount)
		if item.WorkHours > 0 {
			summary += fmt.Sprintf(" · %s giờ", strconv.FormatFloat(item.WorkHours, 'f', -1, 64))
		}
		summaries = append(summaries, summary)
	}
	if len(rows) > 120 {
		return nil, "", validationError("Báo cáo có quá nhiều tổ đội", Record{"crew_items": len(rows), "max": 120})
	}
	return rows, strings.Join(summaries, "; "), nil
}

func normalizeConstructionReportMaterials(items []ConstructionDailyReportMaterialRequest) ([]Record, string, error) {
	rows := make([]Record, 0, len(items))
	summaries := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.MaterialName)
		unit := strings.TrimSpace(item.Unit)
		if name == "" && item.Quantity == 0 && unit == "" && strings.TrimSpace(item.Note) == "" {
			continue
		}
		if name == "" || item.Quantity < 0 || unit == "" {
			return nil, "", validationError("Vật tư cần tên, số lượng không âm và đơn vị tính", Record{"material_name": name, "quantity": item.Quantity, "unit": unit})
		}
		row := Record{"material_name": name, "quantity": item.Quantity, "unit": unit, "source": "other", "note": strings.TrimSpace(item.Note)}
		rows = append(rows, row)
		summaries = append(summaries, fmt.Sprintf("%s · %s %s", name, strconv.FormatFloat(item.Quantity, 'f', -1, 64), unit))
	}
	if len(rows) > 200 {
		return nil, "", validationError("Báo cáo có quá nhiều dòng vật tư", Record{"material_items": len(rows), "max": 200})
	}
	return rows, strings.Join(summaries, "; "), nil
}

func normalizeConstructionReportEvidenceIDs(ids []string, primaryID string) []string {
	seen := map[string]bool{}
	values := make([]string, 0, len(ids)+1)
	add := func(value string) {
		value = cleanID(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		values = append(values, value)
	}
	add(primaryID)
	for _, id := range ids {
		add(id)
	}
	return values
}

func normalizeConstructionCheckCriteria(items []ConstructionCheckCriterion) ([]ConstructionCheckCriterion, error) {
	criteria := make([]ConstructionCheckCriterion, 0, len(items))
	for _, item := range items {
		item.Title = strings.TrimSpace(item.Title)
		item.Criteria = strings.TrimSpace(item.Criteria)
		if item.Title == "" && item.Criteria == "" {
			continue
		}
		if item.Title == "" {
			item.Title = item.Criteria
		}
		if item.Criteria == "" {
			item.Criteria = item.Title
		}
		criteria = append(criteria, item)
	}
	if len(criteria) > 30 {
		return nil, validationError("Mỗi kiểm tra có tối đa 30 tiêu chí")
	}
	return criteria, nil
}

func normalizeConstructionCheckEvidenceIDs(ids []string, fallbackID string, required bool) ([]string, error) {
	values := normalizeConstructionReportEvidenceIDs(ids, fallbackID)
	if len(values) > 10 {
		return nil, validationError("Mỗi nhóm minh chứng có tối đa 10 ảnh hoặc tệp")
	}
	for _, id := range values {
		if !validActivityUUID(id) {
			return nil, validationError("Minh chứng kiểm tra không hợp lệ")
		}
	}
	if required && len(values) == 0 {
		return nil, validationError("Kết quả kiểm tra cần ít nhất một minh chứng")
	}
	return values, nil
}

func constructionReportWorkSummary(done, current, next []string) string {
	return strings.Join([]string{
		"Đã làm:\n" + strings.Join(done, "\n"),
		"Đang làm:\n" + strings.Join(current, "\n"),
		"Sẽ làm:\n" + strings.Join(next, "\n"),
	}, "\n\n")
}

func (s *Service) ProjectConstructionWorkspace(ctx context.Context, projectID string) (Record, error) {
	projectID = cleanID(projectID)
	if !validActivityUUID(projectID) {
		return nil, validationError("Mã công trình không hợp lệ")
	}
	return s.repo.ProjectConstructionWorkspace(ctx, projectID)
}

func (s *Service) CreateConstructionCheckTemplate(ctx context.Context, projectID string, req ConstructionCheckTemplateRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	name := strings.TrimSpace(req.Name)
	if !validActivityUUID(projectID) || name == "" {
		return nil, validationError("Mẫu kiểm tra cần tên")
	}
	criteria := make([]ConstructionCheckCriterion, 0, len(req.Criteria))
	for _, item := range req.Criteria {
		item.Title = strings.TrimSpace(item.Title)
		item.Criteria = strings.TrimSpace(item.Criteria)
		if item.Title == "" && item.Criteria == "" {
			continue
		}
		if item.Title == "" {
			item.Title = item.Criteria
		}
		criteria = append(criteria, item)
	}
	if len(criteria) == 0 || len(criteria) > 30 {
		return nil, validationError("Mẫu kiểm tra cần từ 1 đến 30 tiêu chí")
	}
	return s.repo.CreateConstructionCheckTemplate(ctx, projectID, name, strings.TrimSpace(req.Description), criteria, actorID, actorName)
}

func (s *Service) CreateConstructionCheck(ctx context.Context, projectID string, req ConstructionCheckRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	workItemID := cleanID(req.WorkItemID)
	sourceDailyReportID := cleanID(req.SourceDailyReportID)
	templateID := cleanID(req.TemplateID)
	title := strings.TrimSpace(req.Title)
	if !validActivityUUID(projectID) || !validActivityUUID(workItemID) || title == "" {
		return nil, validationError("Kiểm tra cần công việc liên quan và nội dung ngắn gọn")
	}
	if templateID != "" && !validActivityUUID(templateID) {
		return nil, validationError("Mẫu kiểm tra không hợp lệ")
	}
	if sourceDailyReportID != "" && !validActivityUUID(sourceDailyReportID) {
		return nil, validationError("Báo cáo nguồn không hợp lệ")
	}
	if _, err := s.requireActiveConstructionWorkItem(ctx, projectID, workItemID); err != nil {
		return nil, err
	}
	scheduledAt, err := constructionOptionalTime(req.ScheduledAt, "Thời điểm kiểm tra")
	if err != nil {
		return nil, err
	}
	criteria, err := normalizeConstructionCheckCriteria(req.Criteria)
	if err != nil {
		return nil, err
	}
	contextEvidenceIDs, err := normalizeConstructionCheckEvidenceIDs(req.ContextEvidenceIDs, "", false)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateConstructionCheck(ctx, projectID, workItemID, sourceDailyReportID, templateID, title, strings.TrimSpace(req.Location), scheduledAt, criteria, req.Criteria != nil, contextEvidenceIDs, actorID, actorName)
}

func (s *Service) UpdateConstructionCheck(ctx context.Context, checkID string, req ConstructionCheckUpdateRequest, actorID, actorName string) (Record, error) {
	checkID = cleanID(checkID)
	workItemID := cleanID(req.WorkItemID)
	templateID := cleanID(req.TemplateID)
	title := strings.TrimSpace(req.Title)
	if !validActivityUUID(checkID) || !validActivityUUID(workItemID) || title == "" {
		return nil, validationError("Kiểm tra cần công việc liên quan và nội dung ngắn gọn")
	}
	if templateID != "" && !validActivityUUID(templateID) {
		return nil, validationError("Mẫu kiểm tra không hợp lệ")
	}
	scheduledAt, err := constructionOptionalTime(req.ScheduledAt, "Thời điểm kiểm tra")
	if err != nil {
		return nil, err
	}
	criteria, err := normalizeConstructionCheckCriteria(req.Criteria)
	if err != nil {
		return nil, err
	}
	contextEvidenceIDs, err := normalizeConstructionCheckEvidenceIDs(req.ContextEvidenceIDs, "", false)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateConstructionCheck(ctx, checkID, workItemID, templateID, title, strings.TrimSpace(req.Location), scheduledAt, criteria, req.Criteria != nil, contextEvidenceIDs, req.ContextEvidenceIDs != nil, actorID, actorName)
}

func (s *Service) CompleteConstructionCheck(ctx context.Context, checkID string, req ConstructionCheckCompleteRequest, actorID, actorName string) (Record, error) {
	checkID = cleanID(checkID)
	result := strings.TrimSpace(req.Result)
	if !validActivityUUID(checkID) || !statusIn(result, "pass", "fail") {
		return nil, validationError("Kết quả kiểm tra cần kết luận Đạt hoặc Không đạt")
	}
	evidenceIDs, err := normalizeConstructionCheckEvidenceIDs(req.EvidenceIDs, req.EvidenceID, true)
	if err != nil {
		return nil, err
	}
	priority := strings.TrimSpace(req.Priority)
	if priority == "" {
		priority = "normal"
	}
	impactScope := strings.TrimSpace(req.ImpactScope)
	if impactScope == "" {
		impactScope = "none"
	}
	dueAt := ""
	if result == "fail" {
		if !validActivityUUID(cleanID(req.AssignedUserID)) || !statusIn(priority, "normal", "urgent") || !statusIn(impactScope, "none", "timeline", "budget", "design_scope") {
			return nil, validationError("Việc không đạt cần người phụ trách, mức ưu tiên và ảnh hưởng hợp lệ")
		}
		var err error
		dueAt, err = constructionRequiredTime(req.DueAt, "Hạn xử lý")
		if err != nil {
			return nil, err
		}
	}
	return s.repo.CompleteConstructionCheck(ctx, checkID, result, strings.TrimSpace(req.Note), evidenceIDs, strings.TrimSpace(req.ActionTitle), cleanID(req.AssignedUserID), dueAt, impactScope, priority, actorID, actorName)
}

func (s *Service) CreateConstructionAction(ctx context.Context, projectID string, req ConstructionActionRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	sourceDailyReportID := cleanID(req.SourceDailyReportID)
	priority := strings.TrimSpace(req.Priority)
	if priority == "" {
		priority = "normal"
	}
	impactScope := strings.TrimSpace(req.ImpactScope)
	if impactScope == "" {
		impactScope = "none"
	}
	if !validActivityUUID(projectID) || !validActivityUUID(cleanID(req.WorkItemID)) || strings.TrimSpace(req.Title) == "" ||
		!validActivityUUID(cleanID(req.AssignedUserID)) || !validActivityUUID(cleanID(req.EvidenceID)) ||
		!statusIn(priority, "normal", "urgent") || !statusIn(impactScope, "none", "timeline", "budget", "design_scope") {
		return nil, validationError("Việc cần xử lý cần nội dung, công việc, người phụ trách, ảnh và ảnh hưởng hợp lệ")
	}
	if sourceDailyReportID != "" && !validActivityUUID(sourceDailyReportID) {
		return nil, validationError("Báo cáo nguồn không hợp lệ")
	}
	if _, err := s.requireActiveConstructionWorkItem(ctx, projectID, cleanID(req.WorkItemID)); err != nil {
		return nil, err
	}
	dueAt, err := constructionRequiredTime(req.DueAt, "Hạn xử lý")
	if err != nil {
		return nil, err
	}
	fieldContext, err := normalizeConstructionActionFieldContext(req.FaultCode, req.Location, req.Severity, req.ResponsibleUnit, req.Representative, req.Contact)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateConstructionAction(ctx, projectID, cleanID(req.WorkItemID), sourceDailyReportID, strings.TrimSpace(req.Title), strings.TrimSpace(req.Description), priority, cleanID(req.AssignedUserID), dueAt, impactScope, cleanID(req.EvidenceID), actorID, actorName, fieldContext)
}

func (s *Service) UpdateConstructionAction(ctx context.Context, actionID string, req ConstructionActionUpdateRequest, actorID, actorName string) (Record, error) {
	actionID = cleanID(actionID)
	priority := strings.TrimSpace(req.Priority)
	if priority == "" {
		priority = "normal"
	}
	if !validActivityUUID(actionID) || strings.TrimSpace(req.Title) == "" || !validActivityUUID(cleanID(req.AssignedUserID)) || !statusIn(priority, "normal", "urgent") {
		return nil, validationError("Việc cần xử lý cần nội dung, người phụ trách và mức ưu tiên hợp lệ")
	}
	dueAt, err := constructionRequiredTime(req.DueAt, "Hạn xử lý")
	if err != nil {
		return nil, err
	}
	fieldContext, err := normalizeConstructionActionFieldContext(req.FaultCode, req.Location, req.Severity, req.ResponsibleUnit, req.Representative, req.Contact)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateConstructionAction(ctx, actionID, strings.TrimSpace(req.Title), strings.TrimSpace(req.Description), priority, cleanID(req.AssignedUserID), dueAt, actorID, actorName, fieldContext)
}

func (s *Service) SubmitConstructionActionResolution(ctx context.Context, actionID string, req ConstructionActionResolutionRequest, actorID, actorName string) (Record, error) {
	actionID = cleanID(actionID)
	evidenceID := cleanID(req.EvidenceID)
	if !validActivityUUID(actionID) || !validActivityUUID(evidenceID) {
		return nil, validationError("Kết quả khắc phục cần một minh chứng")
	}
	return s.repo.SubmitConstructionActionResolution(ctx, actionID, evidenceID, strings.TrimSpace(req.Note), actorID, actorName)
}

func (s *Service) VerifyConstructionAction(ctx context.Context, actionID string, req ConstructionActionVerifyRequest, actorID, actorName string) (Record, error) {
	actionID = cleanID(actionID)
	result := strings.TrimSpace(req.Result)
	comment := strings.TrimSpace(req.Comment)
	if !validActivityUUID(actionID) || !statusIn(result, "pass", "fail") {
		return nil, validationError("Kết quả xác nhận phải là Đạt hoặc Chưa đạt")
	}
	if result == "fail" && comment == "" {
		return nil, validationError("Trả lại xử lý cần ghi rõ lý do")
	}
	return s.repo.VerifyConstructionAction(ctx, actionID, result, comment, actorID, actorName)
}

func (s *Service) ApproveConstructionActionImpact(ctx context.Context, actionID, actorID, actorName string) (Record, error) {
	actionID = cleanID(actionID)
	if !validActivityUUID(actionID) {
		return nil, validationError("Việc cần xử lý không hợp lệ")
	}
	return s.repo.ApproveConstructionActionImpact(ctx, actionID, actorID, actorName)
}

func constructionOptionalTime(raw, label string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		return "", validationError(label + " không hợp lệ")
	}
	return value, nil
}

func constructionRequiredTime(raw, label string) (string, error) {
	value, err := constructionOptionalTime(raw, label)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", validationError(label + " là bắt buộc")
	}
	return value, nil
}

func (s *Service) ProjectIssues(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.ListProjectIssues(ctx, cleanID(projectID))
}

func (s *Service) CreateIssue(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	body["project_id"] = cleanID(projectID)
	if cleanID(body["reported_by"]) == "" {
		body["reported_by"] = actorID
	}
	row, err := s.repo.CreateIssue(ctx, body)
	if err != nil {
		return nil, err
	}
	s.repo.AddAuditBestEffort(ctx, cleanID(row["project_id"]), actorID, actorName, "issue_created", "issue", cleanID(row["id"]), fmt.Sprintf("Tạo vấn đề %s", row["title"]), Record{"severity": row["severity"], "status": row["status"]})
	s.publishProjectProgressChanged(cleanID(row["project_id"]))
	return row, nil
}

func (s *Service) UpdateIssue(ctx context.Context, id string, body Record, actorID, actorName string) (Record, error) {
	delete(body, "project_id")
	row, err := s.repo.UpdateIssue(ctx, cleanID(id), body, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishProjectProgressChanged(cleanID(row["project_id"]))
	return row, nil
}
