package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *Service) StartPipeline(ctx context.Context, projectID, templateID, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền chạy quy trình của công trình này")
	}
	canRun, err := s.repo.UserHasAnyProjectPermission(ctx, actorID, projectID, []string{"pipeline.run"})
	if err != nil {
		return nil, err
	}
	if !canRun {
		return nil, forbiddenError("Bạn không có quyền chạy quy trình", Record{"permissions": []string{"pipeline.run"}})
	}
	run, err := s.repo.StartPipeline(ctx, projectID, templateID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishProjectProgressChanged(cleanID(projectID))
	s.publishWorkQueueChanged(cleanID(projectID))
	return run, nil
}

func (s *Service) PipelineRun(ctx context.Context, projectID, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem quy trình của công trình này")
	}
	return s.repo.ProjectPipelineRun(ctx, projectID, actorID)
}

func (s *Service) PlanningDailyReports(ctx context.Context, filter PlanningDailyReportFilter, actorID string) ([]Record, error) {
	normalized, err := normalizePlanningDailyReportFilter(filter)
	if err != nil {
		return nil, err
	}
	if normalized.ProjectID != "" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, normalized.ProjectID, []string{"planning.view", "pipeline.view"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem nhật ký kế hoạch của công trình này", Record{"permissions": []string{"planning.view", "pipeline.view"}})
		}
	}
	return s.repo.PlanningDailyReports(ctx, normalized, actorID)
}

func normalizePlanningDailyReportFilter(filter PlanningDailyReportFilter) (PlanningDailyReportFilter, error) {
	normalized := PlanningDailyReportFilter{
		ProjectID:  cleanID(filter.ProjectID),
		DateFrom:   strings.TrimSpace(filter.DateFrom),
		DateTo:     strings.TrimSpace(filter.DateTo),
		ReportedBy: strings.TrimSpace(filter.ReportedBy),
		Owner:      strings.TrimSpace(filter.Owner),
		Status:     strings.TrimSpace(filter.Status),
		Search:     strings.TrimSpace(filter.Search),
	}
	if normalized.ProjectID != "" && !validActivityUUID(normalized.ProjectID) {
		return normalized, validationError("project_id không hợp lệ", Record{"project_id": normalized.ProjectID})
	}
	if normalized.DateFrom != "" {
		if _, err := time.Parse("2006-01-02", normalized.DateFrom); err != nil {
			return normalized, validationError("Ngày bắt đầu không hợp lệ", Record{"date_from": normalized.DateFrom})
		}
	}
	if normalized.DateTo != "" {
		if _, err := time.Parse("2006-01-02", normalized.DateTo); err != nil {
			return normalized, validationError("Ngày kết thúc không hợp lệ", Record{"date_to": normalized.DateTo})
		}
	}
	if normalized.DateFrom != "" && normalized.DateTo != "" && normalized.DateFrom > normalized.DateTo {
		return normalized, validationError("Khoảng ngày nhật ký không hợp lệ", Record{"date_from": normalized.DateFrom, "date_to": normalized.DateTo})
	}
	if normalized.Status == "all" {
		normalized.Status = ""
	}
	if normalized.Status != "" && !statusIn(normalized.Status, "draft", "submitted", "reopened") {
		return normalized, validationError("Trạng thái nhật ký không hợp lệ", Record{"status": normalized.Status})
	}
	return normalized, nil
}

func (s *Service) PlanningMeetingMinutes(ctx context.Context, filter PlanningMeetingMinutesFilter, actorID string) ([]Record, error) {
	normalized, err := normalizePlanningMeetingMinutesFilter(filter)
	if err != nil {
		return nil, err
	}
	if normalized.ProjectID != "" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, normalized.ProjectID, []string{"planning.view", "pipeline.view"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem biên bản họp của công trình này", Record{"permissions": []string{"planning.view", "pipeline.view"}})
		}
	}
	return s.repo.PlanningMeetingMinutes(ctx, normalized, actorID)
}

func normalizePlanningMeetingMinutesFilter(filter PlanningMeetingMinutesFilter) (PlanningMeetingMinutesFilter, error) {
	normalized := PlanningMeetingMinutesFilter{
		ProjectID: cleanID(filter.ProjectID),
		DateFrom:  strings.TrimSpace(filter.DateFrom),
		DateTo:    strings.TrimSpace(filter.DateTo),
		Chair:     strings.TrimSpace(filter.Chair),
		Owner:     strings.TrimSpace(filter.Owner),
		Status:    strings.TrimSpace(filter.Status),
		Search:    strings.TrimSpace(filter.Search),
	}
	if normalized.ProjectID != "" && !validActivityUUID(normalized.ProjectID) {
		return normalized, validationError("project_id không hợp lệ", Record{"project_id": normalized.ProjectID})
	}
	if normalized.DateFrom != "" {
		if _, err := time.Parse("2006-01-02", normalized.DateFrom); err != nil {
			return normalized, validationError("Ngày bắt đầu không hợp lệ", Record{"date_from": normalized.DateFrom})
		}
	}
	if normalized.DateTo != "" {
		if _, err := time.Parse("2006-01-02", normalized.DateTo); err != nil {
			return normalized, validationError("Ngày kết thúc không hợp lệ", Record{"date_to": normalized.DateTo})
		}
	}
	if normalized.DateFrom != "" && normalized.DateTo != "" && normalized.DateFrom > normalized.DateTo {
		return normalized, validationError("Khoảng ngày biên bản họp không hợp lệ", Record{"date_from": normalized.DateFrom, "date_to": normalized.DateTo})
	}
	if normalized.Status == "all" {
		normalized.Status = ""
	}
	if normalized.Status != "" && !statusIn(normalized.Status, "draft", "submitted", "reopened") {
		return normalized, validationError("Trạng thái biên bản họp không hợp lệ", Record{"status": normalized.Status})
	}
	return normalized, nil
}

func (s *Service) PlanningChecklists(ctx context.Context, filter PlanningChecklistFilter, actorID string) ([]Record, error) {
	normalized, err := normalizePlanningChecklistFilter(filter)
	if err != nil {
		return nil, err
	}
	if normalized.ProjectID != "" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, normalized.ProjectID, []string{"planning.view", "pipeline.view"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem checklist kế hoạch của công trình này", Record{"permissions": []string{"planning.view", "pipeline.view"}})
		}
	}
	return s.repo.PlanningChecklists(ctx, normalized, actorID)
}

func normalizePlanningChecklistFilter(filter PlanningChecklistFilter) (PlanningChecklistFilter, error) {
	normalized := PlanningChecklistFilter{
		ProjectID:  cleanID(filter.ProjectID),
		DateFrom:   strings.TrimSpace(filter.DateFrom),
		DateTo:     strings.TrimSpace(filter.DateTo),
		CheckedBy:  strings.TrimSpace(filter.CheckedBy),
		Assignee:   strings.TrimSpace(filter.Assignee),
		ItemStatus: strings.TrimSpace(filter.ItemStatus),
		Evidence:   strings.TrimSpace(filter.Evidence),
		Status:     strings.TrimSpace(filter.Status),
		Search:     strings.TrimSpace(filter.Search),
	}
	if normalized.ProjectID != "" && !validActivityUUID(normalized.ProjectID) {
		return normalized, validationError("project_id không hợp lệ", Record{"project_id": normalized.ProjectID})
	}
	if normalized.DateFrom != "" {
		if _, err := time.Parse("2006-01-02", normalized.DateFrom); err != nil {
			return normalized, validationError("Ngày bắt đầu không hợp lệ", Record{"date_from": normalized.DateFrom})
		}
	}
	if normalized.DateTo != "" {
		if _, err := time.Parse("2006-01-02", normalized.DateTo); err != nil {
			return normalized, validationError("Ngày kết thúc không hợp lệ", Record{"date_to": normalized.DateTo})
		}
	}
	if normalized.DateFrom != "" && normalized.DateTo != "" && normalized.DateFrom > normalized.DateTo {
		return normalized, validationError("Khoảng ngày checklist kế hoạch không hợp lệ", Record{"date_from": normalized.DateFrom, "date_to": normalized.DateTo})
	}
	if normalized.Status == "all" {
		normalized.Status = ""
	}
	if normalized.Status != "" && !statusIn(normalized.Status, "draft", "submitted", "reopened") {
		return normalized, validationError("Trạng thái checklist kế hoạch không hợp lệ", Record{"status": normalized.Status})
	}
	if normalized.ItemStatus == "all" {
		normalized.ItemStatus = ""
	}
	if normalized.ItemStatus != "" && !statusIn(normalized.ItemStatus, "done", "not_applicable", "needs_follow_up", "blocked") {
		return normalized, validationError("Trạng thái dòng checklist không hợp lệ", Record{"item_status": normalized.ItemStatus})
	}
	if normalized.Evidence == "all" {
		normalized.Evidence = ""
	}
	if normalized.Evidence != "" && !statusIn(normalized.Evidence, "linked", "missing") {
		return normalized, validationError("Bộ lọc minh chứng checklist không hợp lệ", Record{"evidence": normalized.Evidence})
	}
	return normalized, nil
}

func (s *Service) SubmitRunStep(ctx context.Context, stepID string, req WorkItemActionRequest, actorID, actorName string) (Record, error) {
	workItemID := ""
	item, err := s.repo.GetWorkItemByRunStep(ctx, stepID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		workItemID = cleanID(item["id"])
		if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
			return nil, err
		}
		fromStatus := fmt.Sprint(item["status"])
		if !canCompleteWorkItemStatus(fromStatus) {
			return nil, validationError("Trạng thái đầu việc không cho phép hoàn tất", Record{"status": fromStatus})
		}
		allowed, err := s.repo.CanActOnWorkItem(ctx, fmt.Sprint(item["id"]), actorID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền thao tác đầu việc này")
		}
		if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
			return nil, err
		}
		req = withLegacyChecklistCompletion(item, req)
		missing, err := s.missingRequiredEvidence(ctx, item, req)
		if err != nil {
			return nil, err
		}
		if len(missing) > 0 {
			return nil, validationError("Đầu việc cần đủ minh chứng trước khi hoàn tất", Record{"missing_evidence": missing})
		}
		if err := s.repo.RunRequiredWorkflowAutomationForWorkItem(ctx, workItemID, actorID, actorName); err != nil {
			return nil, err
		}
	}
	step, err := s.repo.SubmitRunStep(ctx, stepID, req, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if workItemID != "" {
		_ = s.repo.RunClientWorkflowAutomationForWorkItem(ctx, workItemID, actorID, actorName)
	}
	s.publishProjectProgressChanged(cleanID(step["project_id"]))
	s.publishWorkQueueChanged(cleanID(step["project_id"]))
	return step, nil
}

func canCompleteWorkItemStatus(status string) bool {
	return statusIn(status, "ready", "in_progress", "needs_changes", "waiting_evidence")
}

func (s *Service) AdvanceRunNode(ctx context.Context, nodeID, actorID, actorName string) (Record, error) {
	node, err := s.repo.AdvanceRunNode(ctx, nodeID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishProjectProgressChanged(cleanID(node["project_id"]))
	s.publishWorkQueueChanged(cleanID(node["project_id"]))
	return node, nil
}

func (s *Service) RequestNodeApproval(ctx context.Context, nodeID, actorID, actorName, reason string) (Record, error) {
	node, err := s.repo.GetByID(ctx, "pipeline_run_nodes", nodeID)
	if err != nil {
		return nil, err
	}
	issues, err := s.repo.NodeCompletionIssues(ctx, node)
	if err != nil {
		return nil, err
	}
	blocking := []Record{}
	for _, issue := range issues {
		if fmt.Sprint(issue["code"]) != "NODE_APPROVAL_REQUIRED" {
			blocking = append(blocking, issue)
		}
	}
	if len(blocking) > 0 {
		return nil, validationError("Node chưa đủ điều kiện gửi phê duyệt", Record{"issues": blocking})
	}
	request, err := s.repo.RequestNodeApproval(ctx, nodeID, actorID, actorName, reason)
	if err != nil {
		return nil, err
	}
	s.publishProjectProgressChanged(cleanID(request["project_id"]))
	s.publishWorkQueueChanged(cleanID(request["project_id"]))
	return request, nil
}

func (s *Service) BlockRunNode(ctx context.Context, nodeID, reason, actorID, actorName string) (Record, error) {
	node, err := s.repo.BlockRunNode(ctx, nodeID, reason, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishProjectProgressChanged(cleanID(node["project_id"]))
	s.publishWorkQueueChanged(cleanID(node["project_id"]))
	return node, nil
}

func (s *Service) ApprovalAction(ctx context.Context, id, action, comment, actorID, actorName string) (Record, error) {
	action = normalizeApprovalAction(action)
	if _, ok := approvalStatusForAction(action); !ok {
		return nil, validationError("Hành động phê duyệt không hợp lệ", Record{"action": action})
	}
	request, err := s.repo.GetByID(ctx, "approval_requests", id)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(request["target_type"]) == "work_item" {
		workItemID := cleanID(request["target_id"])
		if workItemID == "" {
			return nil, validationError("Yêu cầu phê duyệt chưa liên kết đầu việc", Record{"approval_request_id": id})
		}
		_, err := s.WorkItemAction(ctx, workItemID, action, WorkItemActionRequest{
			Comment: comment,
			Reason:  comment,
		}, actorID, actorName)
		if err != nil {
			return nil, err
		}
		return s.repo.GetByID(ctx, "approval_requests", id)
	}
	allowed, err := s.repo.CanDecideApprovalRequest(ctx, request, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền duyệt yêu cầu này")
	}
	updated, err := s.repo.ApplyApprovalPolicyAction(ctx, request, action, comment, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishProjectProgressChanged(cleanID(updated["project_id"]))
	s.publishWorkQueueChanged(cleanID(updated["project_id"]))
	return updated, nil
}

func (s *Service) PendingApprovals(ctx context.Context, filter PendingApprovalFilter, actorID string) ([]Record, error) {
	filter.ProjectID = strings.TrimSpace(filter.ProjectID)
	if filter.ProjectID != "" {
		if _, err := uuid.Parse(filter.ProjectID); err != nil {
			return nil, validationError("project_id không hợp lệ", Record{"project_id": filter.ProjectID})
		}
	}
	rows, err := s.repo.ListPendingApprovalRequests(ctx, actorID, filter.ProjectID)
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(rows))
	for _, row := range rows {
		canDecide, err := s.repo.CanDecideApprovalRequest(ctx, row, actorID)
		if err != nil {
			return nil, err
		}
		if filter.DecidableOnly && !canDecide {
			continue
		}
		policy := approvalPolicyFromValue(row["policy_json"])
		stage := int(number(row["current_stage"]))
		requiredRoles := stringSlice(row["required_roles"])
		quorum := 1
		commentRequired := false
		requesterCannotApprove := true
		if policy.enabled() {
			requiredRoles = policy.rolesForStage(stage)
			quorum = policy.quorumForStage(stage)
			commentRequired = policy.CommentRequired
			requesterCannotApprove = policy.RequesterCannotApprove
		}
		row["capabilities"] = Record{"can_decide": canDecide}
		row["available_actions"] = []string{}
		if canDecide {
			row["available_actions"] = []string{"approve", "request-change", "reject"}
		}
		row["decision_context"] = Record{
			"current_stage":            stage,
			"required_roles":           requiredRoles,
			"quorum":                   quorum,
			"comment_required":         commentRequired,
			"requester_cannot_approve": requesterCannotApprove,
		}
		out = append(out, row)
	}
	return out, nil
}

func normalizeApprovalAction(action string) string {
	action = strings.TrimSpace(action)
	if action == "request_change" {
		return "request-change"
	}
	return action
}

func approvalStatusForAction(action string) (string, bool) {
	switch action {
	case "approve":
		return "approved", true
	case "reject":
		return "rejected", true
	case "request-change":
		return "changes_requested", true
	default:
		return "", false
	}
}

func approvalAuditActionLabel(action string) string {
	switch action {
	case "approve":
		return "Duyệt"
	case "reject":
		return "Từ chối"
	case "request-change":
		return "Yêu cầu chỉnh sửa"
	default:
		return action
	}
}
