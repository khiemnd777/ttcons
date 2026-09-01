package core

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *Service) ProjectReworks(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.ListProjectReworks(ctx, cleanID(projectID))
}

func (s *Service) CreateRework(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	body["project_id"] = cleanID(projectID)
	if cleanID(body["created_by"]) == "" {
		body["created_by"] = actorID
	}
	row, err := s.repo.CreateRework(ctx, body)
	if err != nil {
		return nil, err
	}
	s.repo.AddAuditBestEffort(ctx, cleanID(row["project_id"]), actorID, actorName, "rework_created", "rework", cleanID(row["id"]), fmt.Sprintf("Tạo yêu cầu sửa lỗi %s", row["title"]), Record{"status": row["status"], "issue_id": cleanID(row["issue_id"]), "acceptance_id": cleanID(row["acceptance_id"])})
	return row, nil
}

func (s *Service) UpdateReworkStatus(ctx context.Context, id, status string, actorID, actorName string) (Record, error) {
	status = strings.TrimSpace(status)
	if !statusIn(status, "approved", "done") {
		return nil, validationError("Trạng thái sửa lỗi không hợp lệ", Record{"status": status})
	}
	row, err := s.repo.UpdateReworkStatus(ctx, cleanID(id), status, actorID, actorName)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Service) ProjectReplannings(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.ListProjectReplannings(ctx, cleanID(projectID))
}

func (s *Service) CreateReplanning(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	body["project_id"] = cleanID(projectID)
	if cleanID(body["created_by"]) == "" {
		body["created_by"] = actorID
	}
	row, err := s.repo.CreateReplanning(ctx, body)
	if err != nil {
		return nil, err
	}
	s.repo.AddAuditBestEffort(ctx, cleanID(row["project_id"]), actorID, actorName, "replanning_created", "replanning", cleanID(row["id"]), "Tạo yêu cầu lập kế hoạch lại", Record{"status": row["status"], "issue_id": cleanID(row["issue_id"]), "node_id": cleanID(row["node_id"])})
	return row, nil
}

func (s *Service) ApproveReplanning(ctx context.Context, id string, actorID, actorName string) (Record, error) {
	row, err := s.repo.ApproveReplanning(ctx, cleanID(id), actorID, actorName)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Service) ProjectQualityWorkspace(ctx context.Context, projectID string) (Record, error) {
	return s.repo.ProjectQualityWorkspace(ctx, cleanID(projectID))
}

func (s *Service) CreateQualityITP(ctx context.Context, projectID string, req QualityITPRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" || strings.TrimSpace(req.ITPCode) == "" || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Discipline) == "" || strings.TrimSpace(req.WorkPackage) == "" {
		return nil, validationError("Mã, tên, bộ môn và gói công việc ITP là bắt buộc")
	}
	if len(req.Items) == 0 {
		return nil, validationError("ITP cần ít nhất một inspection item")
	}
	items := make([]Record, 0, len(req.Items))
	seen := map[int]bool{}
	for index, item := range req.Items {
		if item.SequenceNo <= 0 || seen[item.SequenceNo] || strings.TrimSpace(item.ActivityName) == "" || strings.TrimSpace(item.AcceptanceCriteria) == "" || strings.TrimSpace(item.InspectionMethod) == "" || strings.TrimSpace(item.ResponsibleRole) == "" {
			return nil, validationError("Inspection item ITP không hợp lệ hoặc trùng thứ tự", Record{"item_index": index})
		}
		seen[item.SequenceNo] = true
		controlPoint := strings.TrimSpace(item.ControlPoint)
		if !statusIn(controlPoint, "review", "witness", "hold", "surveillance") {
			return nil, validationError("Control point ITP không hợp lệ", Record{"item_index": index})
		}
		frequencyType := strings.TrimSpace(item.FrequencyType)
		if !statusIn(frequencyType, "each_lot", "daily", "weekly", "quantity", "custom") || item.FrequencyValue <= 0 {
			return nil, validationError("Tần suất ITP không hợp lệ", Record{"item_index": index})
		}
		items = append(items, Record{"sequence_no": item.SequenceNo, "activity_name": strings.TrimSpace(item.ActivityName), "inspection_stage": strings.TrimSpace(item.InspectionStage), "acceptance_criteria": strings.TrimSpace(item.AcceptanceCriteria), "inspection_method": strings.TrimSpace(item.InspectionMethod), "control_point": controlPoint, "frequency_type": frequencyType, "frequency_value": item.FrequencyValue, "responsible_role": strings.TrimSpace(item.ResponsibleRole), "evidence_required": item.EvidenceRequired, "client_witness_required": item.ClientWitnessRequired, "notes": strings.TrimSpace(item.Notes)})
	}
	header := Record{"itp_code": strings.TrimSpace(req.ITPCode), "title": strings.TrimSpace(req.Title), "discipline": strings.TrimSpace(req.Discipline), "work_package": strings.TrimSpace(req.WorkPackage), "document_id": cleanID(req.DocumentID), "review_comment": strings.TrimSpace(req.ReviewComment)}
	return s.repo.CreateQualityITP(ctx, projectID, header, items, actorID, actorName)
}

func (s *Service) ReviseQualityITP(ctx context.Context, itpID, comment, actorID, actorName string) (Record, error) {
	return s.repo.ReviseQualityITP(ctx, cleanID(itpID), strings.TrimSpace(comment), actorID, actorName)
}

func (s *Service) QualityITPAction(ctx context.Context, itpID, action, comment, actorID, actorName string) (Record, error) {
	comment = strings.TrimSpace(comment)
	if statusIn(action, "approve", "activate") && comment == "" {
		return nil, validationError("Duyệt/kích hoạt ITP cần có nhận xét")
	}
	return s.repo.QualityITPAction(ctx, cleanID(itpID), action, comment, actorID, actorName)
}

func (s *Service) CreateQualityInspectionRequest(ctx context.Context, projectID string, req QualityInspectionRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" || cleanID(req.ITPItemID) == "" || strings.TrimSpace(req.RFICode) == "" || strings.TrimSpace(req.Location) == "" {
		return nil, validationError("Mã RFI, ITP item và vị trí kiểm tra là bắt buộc")
	}
	requestedAt := strings.TrimSpace(req.RequestedInspectionAt)
	if _, err := time.Parse(time.RFC3339, requestedAt); err != nil {
		return nil, validationError("Thời điểm yêu cầu kiểm tra phải theo RFC3339")
	}
	body := Record{"rfi_code": strings.TrimSpace(req.RFICode), "itp_item_id": cleanID(req.ITPItemID), "work_item_id": cleanID(req.WorkItemID), "location": strings.TrimSpace(req.Location), "lot_reference": strings.TrimSpace(req.LotReference), "requested_inspection_at": requestedAt, "request_comment": strings.TrimSpace(req.RequestComment), "request_evidence_id": cleanID(req.RequestEvidenceID)}
	return s.repo.CreateQualityInspectionRequest(ctx, projectID, body, actorID, actorName)
}

func (s *Service) QualityInspectionAction(ctx context.Context, rfiID, action string, req QualityInspectionDecisionRequest, actorID, actorName string) (Record, error) {
	if action == "inspect" {
		if !statusIn(req.Result, "pass", "conditional_pass", "fail") || strings.TrimSpace(req.Comment) == "" {
			return nil, validationError("Inspection cần kết quả và nhận xét")
		}
	}
	return s.repo.QualityInspectionAction(ctx, cleanID(rfiID), action, strings.TrimSpace(req.Result), strings.TrimSpace(req.Comment), cleanID(req.InspectionEvidenceID), actorID, actorName)
}

func (s *Service) CreateQualityNCR(ctx context.Context, projectID string, req QualityNCRRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" || strings.TrimSpace(req.NCRCode) == "" || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Nonconformity) == "" || !statusIn(req.SourceType, "rfi", "issue", "work_item", "manual") || !statusIn(req.Severity, "low", "medium", "high", "critical") {
		return nil, validationError("Dữ liệu NCR không hợp lệ")
	}
	dueDate := strings.TrimSpace(req.DueDate)
	if dueDate != "" {
		if _, err := time.Parse("2006-01-02", dueDate); err != nil {
			return nil, validationError("Hạn NCR không hợp lệ")
		}
	}
	sourceID := cleanID(req.SourceID)
	if sourceID == "" {
		sourceID = map[string]string{"rfi": cleanID(req.RFIID), "issue": cleanID(req.IssueID), "work_item": cleanID(req.WorkItemID), "manual": projectID}[req.SourceType]
	}
	if sourceID == "" {
		return nil, validationError("NCR cần hồ sơ nguồn phù hợp")
	}
	body := Record{"ncr_code": strings.TrimSpace(req.NCRCode), "source_type": req.SourceType, "source_id": sourceID, "rfi_id": cleanID(req.RFIID), "work_item_id": cleanID(req.WorkItemID), "issue_id": cleanID(req.IssueID), "title": strings.TrimSpace(req.Title), "severity": req.Severity, "nonconformity": strings.TrimSpace(req.Nonconformity), "immediate_action": strings.TrimSpace(req.ImmediateAction), "due_date": dueDate, "evidence_id": cleanID(req.EvidenceID), "owner_id": cleanID(req.OwnerID)}
	return s.repo.CreateQualityNCR(ctx, projectID, body, actorID, actorName)
}

func (s *Service) QualityNCRAction(ctx context.Context, ncrID, action string, req QualityNCRActionRequest, actorID, actorName string) (Record, error) {
	if action == "root-cause" && strings.TrimSpace(req.RootCause) == "" {
		return nil, validationError("NCR cần phân tích nguyên nhân gốc")
	}
	if action == "corrective" && (strings.TrimSpace(req.CorrectiveAction) == "" || cleanID(req.CorrectiveEvidenceID) == "") {
		return nil, validationError("Corrective action cần nội dung và minh chứng")
	}
	if action == "reinspection" && cleanID(req.ReinspectionRFIID) == "" {
		return nil, validationError("Cần liên kết RFI tái kiểm tra")
	}
	if action == "close" && strings.TrimSpace(req.Comment) == "" {
		return nil, validationError("Đóng NCR cần nhận xét")
	}
	return s.repo.QualityNCRAction(ctx, cleanID(ncrID), action, req, actorID, actorName)
}

func (s *Service) CreateRecurringPlan(ctx context.Context, projectID string, req ConstructionRecurringPlanRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" || strings.TrimSpace(req.PlanCode) == "" || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.ResponsibleRole) == "" || !statusIn(req.RecurrenceType, "daily", "weekly", "monthly", "custom_days") || req.IntervalValue <= 0 {
		return nil, validationError("Dữ liệu lịch QA/QC lặp không hợp lệ")
	}
	startDate := strings.TrimSpace(req.StartDate)
	endDate := strings.TrimSpace(req.EndDate)
	if _, err := time.Parse("2006-01-02", startDate); err != nil {
		return nil, validationError("Ngày bắt đầu lịch lặp không hợp lệ")
	}
	if endDate != "" {
		if _, err := time.Parse("2006-01-02", endDate); err != nil || endDate < startDate {
			return nil, validationError("Ngày kết thúc lịch lặp không hợp lệ")
		}
	}
	body := Record{"plan_code": strings.TrimSpace(req.PlanCode), "title": strings.TrimSpace(req.Title), "description": strings.TrimSpace(req.Description), "recurrence_type": req.RecurrenceType, "interval_value": req.IntervalValue, "start_date": startDate, "end_date": endDate, "due_time": strings.TrimSpace(req.DueTime), "responsible_role": strings.TrimSpace(req.ResponsibleRole), "assigned_user_id": cleanID(req.AssignedUserID), "evidence_required": req.EvidenceRequired, "source_work_item_id": cleanID(req.SourceWorkItemID)}
	return s.repo.CreateRecurringPlan(ctx, projectID, body, actorID, actorName)
}

func (s *Service) GenerateRecurringOccurrences(ctx context.Context, planID string, throughDate string, actorID, actorName string) (Record, error) {
	throughDate = strings.TrimSpace(throughDate)
	if throughDate == "" {
		throughDate = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", throughDate); err != nil {
		return nil, validationError("Ngày sinh occurrence không hợp lệ")
	}
	return s.repo.GenerateRecurringOccurrences(ctx, cleanID(planID), throughDate, actorID, actorName)
}

func (s *Service) RecurringOccurrenceAction(ctx context.Context, occurrenceID, action string, req ConstructionRecurringOccurrenceRequest, actorID, actorName string) (Record, error) {
	if action == "complete" && strings.TrimSpace(req.Comment) == "" {
		return nil, validationError("Hoàn tất occurrence cần ghi chú")
	}
	if action == "skip" && strings.TrimSpace(req.Reason) == "" {
		return nil, validationError("Bỏ qua occurrence cần lý do")
	}
	return s.repo.RecurringOccurrenceAction(ctx, cleanID(occurrenceID), action, strings.TrimSpace(req.Comment), cleanID(req.EvidenceID), strings.TrimSpace(req.Reason), actorID, actorName)
}
