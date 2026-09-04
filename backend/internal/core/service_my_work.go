package core

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const accountingRoleName = "accounting"

func (s *Service) ProjectWorkItems(ctx context.Context, projectID, actorID string) ([]Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem công việc của công trình này")
	}
	items, err := s.repo.ListProjectWorkItems(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	attachExecutionTargets(items)
	return items, nil
}

func (s *Service) MyWorkItems(ctx context.Context, filter MyWorkItemsFilter, actorID string) (Record, error) {
	filter, offset, err := normalizeMyWorkItemsFilter(filter)
	if err != nil {
		return nil, err
	}
	// Authorization to observe/act on accounting handoffs does not make the actor
	// responsible for Accounting's inbox.
	filter.IncludeAccounting = false
	filter.AccountingResponsible = false
	if filter.Unified {
		permissions, err := s.repo.UserPermissions(ctx, actorID)
		if err != nil {
			return nil, err
		}
		filter.IncludeAccounting = canAccessAccountingWorkQueue(permissions)
		if filter.IncludeAccounting {
			filter.AccountingResponsible, err = s.repo.UserHasGlobalRole(ctx, actorID, accountingRoleName)
			if err != nil {
				return nil, err
			}
		}
	}
	page, err := s.repo.MyWorkItemsPage(ctx, actorID, filter, offset)
	if err != nil {
		return nil, err
	}
	for _, item := range page.Items {
		canAct := boolValue(item["can_act"])
		canDecide := boolValue(item["can_decide"])
		item["capabilities"] = Record{"can_act": canAct, "can_decide": canDecide}
		item["queue_scopes"] = myWorkItemQueueScopes(item, canDecide)
		item["available_actions"] = myWorkItemAvailableActions(item)
		item["readiness"] = myWorkItemReadiness(item)
		item["execution_target"] = workItemExecutionTarget(item, nil)
		for _, internalKey := range []string{
			"action_scope", "decision_required_roles", "decision_requested_by",
			"pipeline_current_node_id", "run_node_status", "is_responsible", "is_mine", "is_department", "is_watching",
			"role_responsibility", "department_scope", "unassigned_department_responsibility", "is_future",
		} {
			delete(item, internalKey)
		}
	}
	result := Record{
		"items":       page.Items,
		"summary":     page.Summary,
		"facets":      page.Facets,
		"total":       page.Total,
		"has_more":    page.HasMore,
		"next_cursor": page.NextCursor,
	}
	if filter.Unified {
		result["entries"] = page.Entries
	}
	return result, nil
}

func canAccessAccountingWorkQueue(permissions []string) bool {
	return hasAnyString(permissions, []string{"accounting_handoff.view"}) && hasAnyString(permissions, []string{"accounting_handoff.acknowledge"})
}

func (s *Service) MyAccountingWork(ctx context.Context, filter MyAccountingWorkFilter, actorID string) (Record, error) {
	filter, offset, err := normalizeMyAccountingWorkFilter(filter)
	if err != nil {
		return nil, err
	}
	allowed, err := s.repo.UserHasAnyPermission(ctx, actorID, []string{"accounting_handoff.view", "project_finance.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem công việc Kế toán")
	}
	return s.repo.MyAccountingWork(ctx, actorID, filter, offset)
}

func (s *Service) MyWorkNavigationSummary(ctx context.Context, actorID string) (Record, error) {
	workItemCounts, err := s.repo.MyWorkNavigationCounts(ctx, actorID)
	if err != nil {
		return nil, err
	}
	workflowActions, err := s.repo.MyWorkflowActions(ctx, actorID, MyWorkflowActionsFilter{Queue: "responsible"})
	if err != nil {
		return nil, err
	}
	workflowActionCount := len(workflowActions)
	startupActions, err := s.MyProjectStartupActions(ctx, actorID, "", "")
	if err != nil {
		return nil, err
	}
	startupActionCount := int(number(recordValue(startupActions["summary"])["need_action"]))

	permissions, err := s.repo.UserPermissions(ctx, actorID)
	if err != nil {
		return nil, err
	}
	nodeApprovalCount := 0
	if hasAnyString(permissions, []string{"project.view", "operation.view", "procurement.view", "accounting.view", "report.view"}) {
		pendingApprovals, err := s.PendingApprovals(ctx, PendingApprovalFilter{DecidableOnly: true}, actorID)
		if err != nil {
			return nil, err
		}
		for _, approval := range pendingApprovals {
			if fmt.Sprint(approval["target_type"]) == "pipeline_run_node" {
				nodeApprovalCount++
			}
		}
	}

	workReportReviewCount := 0
	canViewWorkReports := hasAnyString(permissions, []string{"work_report.view"})
	canReviewWorkReports := hasAnyString(permissions, []string{"work_report.review", "work_report.manage"})
	if canViewWorkReports && canReviewWorkReports {
		workReportSummary, err := s.WorkReportSummary(ctx, WorkReportFilter{Scope: "review"}, actorID)
		if err != nil {
			return nil, err
		}
		workReportReviewCount = workReportSummary.Scopes["review"]
	}

	accountingVisibleCount := 0
	accountingResponsibleCount := 0
	if canAccessAccountingWorkQueue(permissions) {
		accountingPage, err := s.MyAccountingWork(ctx, MyAccountingWorkFilter{Status: "handed_off", Limit: 1}, actorID)
		if err != nil {
			return nil, err
		}
		accountingVisibleCount = int(number(accountingPage["total"]))
		isAccounting, err := s.repo.UserHasGlobalRole(ctx, actorID, accountingRoleName)
		if err != nil {
			return nil, err
		}
		if isAccounting {
			accountingResponsibleCount = accountingVisibleCount
		}
	}

	return myWorkNavigationSummaryRecord(workItemCounts, workflowActionCount, startupActionCount, nodeApprovalCount, workReportReviewCount, accountingVisibleCount, accountingResponsibleCount), nil
}

func (s *Service) MyWorkScopeSummary(ctx context.Context, actorID, projectID string) (Record, error) {
	projectID, err := normalizeMyWorkScopeSummaryProjectID(projectID)
	if err != nil {
		return nil, err
	}
	workItemCounts, err := s.repo.MyWorkScopeCounts(ctx, actorID, projectID)
	if err != nil {
		return nil, err
	}
	workflowActions, err := s.repo.MyWorkflowActions(ctx, actorID, MyWorkflowActionsFilter{
		Queue: "all", ProjectID: projectID,
	})
	if err != nil {
		return nil, err
	}
	workflowResponsible := 0
	for _, action := range workflowActions {
		if boolValue(action["is_responsible"]) {
			workflowResponsible++
		}
	}
	startupActions, err := s.MyProjectStartupActions(ctx, actorID, projectID, "")
	if err != nil {
		return nil, err
	}
	startupNeedAction := int(number(recordValue(startupActions["summary"])["need_action"]))
	result := myWorkScopeSummaryRecord(workItemCounts, workflowResponsible, len(workflowActions), startupNeedAction)
	permissions, err := s.repo.UserPermissions(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if canAccessAccountingWorkQueue(permissions) {
		page, err := s.MyAccountingWork(ctx, MyAccountingWorkFilter{ProjectID: projectID, Status: "handed_off", Limit: 1}, actorID)
		if err != nil {
			return nil, err
		}
		allRow := recordValue(result["all"])
		allRow["accounting_handoffs"] = page["total"]
		allRow["total"] = number(allRow["total"]) + number(page["total"])

		isAccounting, err := s.repo.UserHasGlobalRole(ctx, actorID, accountingRoleName)
		if err != nil {
			return nil, err
		}
		if isAccounting {
			responsibleRow := recordValue(result["responsible"])
			responsibleRow["accounting_handoffs"] = page["total"]
			responsibleRow["total"] = number(responsibleRow["total"]) + number(page["total"])
		}
	}
	return result, nil
}

func myWorkScopeSummaryRecord(workItemCounts myWorkScopeCounts, workflowResponsible, workflowAll, startupNeedAction int) Record {
	row := func(workItems, workflowActions, projectStartups int) Record {
		return Record{
			"work_items": workItems, "workflow_actions": workflowActions, "project_startups": projectStartups,
			"total": workItems + workflowActions + projectStartups,
		}
	}
	return Record{
		"responsible": row(workItemCounts.Responsible, workflowResponsible, startupNeedAction),
		"mine":        row(workItemCounts.Mine, 0, 0),
		"department":  row(workItemCounts.Department, 0, 0),
		"watching":    row(workItemCounts.Watching, 0, 0),
		"all":         row(workItemCounts.All, workflowAll, startupNeedAction),
	}
}

func myWorkNavigationSummaryRecord(workItemCounts myWorkNavigationCounts, workflowActionCount, startupActionCount, nodeApprovalCount, workReportReviewCount, accountingVisibleCount, accountingResponsibleCount int) Record {
	workTotal := workItemCounts.NeedAction + workflowActionCount + startupActionCount + accountingResponsibleCount
	approvalTotal := workItemCounts.WaitingDecision + nodeApprovalCount + workReportReviewCount
	return Record{
		"work_items_need_action":      workItemCounts.NeedAction,
		"workflow_actions_ready":      workflowActionCount,
		"project_startups_pending":    startupActionCount,
		"work_total":                  workTotal,
		"work_items_waiting_decision": workItemCounts.WaitingDecision,
		"node_approvals_pending":      nodeApprovalCount,
		"work_reports_review_pending": workReportReviewCount,
		"approval_total":              approvalTotal,
		"accounting_handoffs_pending": accountingVisibleCount,
		"accounting_total":            accountingVisibleCount,
		// Accounting visibility remains available as a breakdown; only an Accounting
		// actor's responsible count contributes to the personal work total.
		"total": workTotal + approvalTotal,
	}
}

func (s *Service) MyWorkflowActions(ctx context.Context, filter MyWorkflowActionsFilter, actorID string) (Record, error) {
	filter, err := normalizeMyWorkflowActionsFilter(filter)
	if err != nil {
		return nil, err
	}
	actions, err := s.repo.MyWorkflowActions(ctx, actorID, filter)
	if err != nil {
		return nil, err
	}
	filtered := actions
	if filter.Search != "" {
		needle := strings.ToLower(filter.Search)
		filtered = make([]Record, 0, len(actions))
		for _, action := range actions {
			haystack := strings.ToLower(strings.Join([]string{
				fmt.Sprint(action["title"]), fmt.Sprint(action["project_name"]), fmt.Sprint(action["project_code"]),
				fmt.Sprint(action["run_node_name"]), fmt.Sprint(action["next_run_node_name"]),
			}, " "))
			if strings.Contains(haystack, needle) {
				filtered = append(filtered, action)
			}
		}
	}
	for _, action := range filtered {
		delete(action, "is_responsible")
	}
	return Record{"items": filtered, "total": len(filtered), "summary_total": len(actions)}, nil
}

func (s *Service) MyActivityAttention(ctx context.Context, filter ActivityAttentionFilter, actorID string) ([]Record, error) {
	filter.Queue = strings.TrimSpace(filter.Queue)
	if filter.Queue == "" {
		filter.Queue = "all"
	}
	if !activityAttentionQueues[filter.Queue] {
		return nil, validationError("Hàng đợi hoạt động không hợp lệ", Record{"queue": filter.Queue})
	}
	projectID, profile, err := s.myActivityAttentionScope(ctx, filter.ProjectID, actorID)
	if err != nil {
		return nil, err
	}
	filter.ProjectID = projectID
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 80
	}
	return s.repo.ListMyActivityAttention(ctx, actorID, profile, filter)
}

func (s *Service) MyActivityAttentionSummary(ctx context.Context, projectID, actorID string) (ActivityAttentionSummary, error) {
	projectID, profile, err := s.myActivityAttentionScope(ctx, projectID, actorID)
	if err != nil {
		return ActivityAttentionSummary{}, err
	}
	return s.repo.MyActivityAttentionSummary(ctx, actorID, profile, projectID)
}

func (s *Service) myActivityAttentionScope(ctx context.Context, projectID, actorID string) (string, workItemAccessProfile, error) {
	projectID = cleanID(projectID)
	if projectID != "" {
		if !validActivityUUID(projectID) {
			return "", workItemAccessProfile{}, validationError("project_id không hợp lệ", Record{"project_id": projectID})
		}
		allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
		if err != nil {
			return "", workItemAccessProfile{}, err
		}
		if !allowed {
			return "", workItemAccessProfile{}, forbiddenError("Bạn không có quyền xem hoạt động của công trình này")
		}
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return "", workItemAccessProfile{}, err
	}
	return projectID, profile, nil
}

const (
	projectOperationsViewFull            = ""
	projectOperationsViewWorkflowSidebar = "workflow-sidebar"
)

func (s *Service) ProjectOperations(ctx context.Context, projectID, actorID string, filter ProjectOperationsFilter) (Record, error) {
	filter.View = strings.ToLower(strings.TrimSpace(filter.View))
	if filter.View == "full" {
		filter.View = projectOperationsViewFull
	}
	if filter.View != projectOperationsViewFull && filter.View != projectOperationsViewWorkflowSidebar {
		return nil, validationError("Kiểu dữ liệu điều hành công trình không hợp lệ", Record{"view": filter.View})
	}
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem điều hành công trình này")
	}
	operations, err := s.repo.ProjectOperations(ctx, projectID, actorID, filter)
	if err != nil {
		return nil, err
	}
	if items, ok := operations["work_items"].([]Record); ok {
		attachExecutionTargets(items)
	}
	return operations, nil
}

func (s *Service) ProjectEvidencePage(ctx context.Context, projectID string, filter ProjectEvidenceFilter, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem hồ sơ của công trình này")
	}
	filter, err = normalizeProjectEvidenceFilter(filter)
	if err != nil {
		return nil, err
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	queryFilter := filter
	queryFilter.Limit = filter.Limit + 1
	rows, err := s.repo.ListProjectEvidence(ctx, projectID, profile, queryFilter)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	nextCursor := ""
	if hasMore && len(rows) > 0 {
		nextCursor = encodeEvidenceCursor(rows[len(rows)-1])
	}
	for _, row := range rows {
		row["can_review"] = false
		if statusIn(fmt.Sprint(row["status"]), "draft", "submitted") {
			canReview, err := s.repo.CanDecideWorkItem(ctx, Record{
				"id":                  row["work_item_id"],
				"project_id":          projectID,
				"status":              row["work_item_status"],
				"role_ids":            row["work_item_role_ids"],
				"approval_request_id": row["work_item_approval_request_id"],
			}, actorID)
			if err != nil {
				return nil, err
			}
			row["can_review"] = canReview
		}
		delete(row, "review_priority")
		delete(row, "work_item_role_ids")
		delete(row, "work_item_approval_request_id")
	}
	filteredTotal, err := s.repo.ProjectEvidenceCount(ctx, projectID, profile, filter)
	if err != nil {
		return nil, err
	}
	summary, err := s.repo.ProjectEvidenceSummary(ctx, projectID, profile)
	if err != nil {
		return nil, err
	}
	options, err := s.repo.ProjectEvidenceOptions(ctx, projectID, profile)
	if err != nil {
		return nil, err
	}
	return Record{
		"items":          rows,
		"summary":        summary,
		"options":        options,
		"filtered_total": filteredTotal,
		"has_more":       hasMore,
		"next_cursor":    nextCursor,
	}, nil
}

func (s *Service) ProjectActivity(ctx context.Context, projectID string, filter ProjectActivityFilter, actorID string) ([]Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem hoạt động của công trình này")
	}
	filter, err = normalizeProjectActivityFilter(filter)
	if err != nil {
		return nil, err
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return s.repo.ProjectActivityThread(ctx, projectID, profile, filter)
}

func (s *Service) ProjectActivityPage(ctx context.Context, projectID string, filter ProjectActivityFilter, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem hoạt động của công trình này")
	}
	filter, err = normalizeProjectActivityFilter(filter)
	if err != nil {
		return nil, err
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	queryFilter := filter
	queryFilter.Limit = filter.Limit + 1
	rows, err := s.repo.ProjectActivityThread(ctx, projectID, profile, queryFilter)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	nextCursor := ""
	if hasMore && len(rows) > 0 {
		nextCursor = encodeActivityCursor(rows[len(rows)-1])
	}
	summary, err := s.repo.ProjectActivitySummary(ctx, projectID, profile)
	if err != nil {
		return nil, err
	}
	return Record{
		"items":       rows,
		"summary":     summary,
		"has_more":    hasMore,
		"next_cursor": nextCursor,
	}, nil
}

func (s *Service) ProjectActivityOptions(ctx context.Context, projectID, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem tùy chọn trao đổi của công trình này")
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return s.repo.ProjectActivityOptions(ctx, projectID, profile)
}

func normalizeProjectEvidenceFilter(filter ProjectEvidenceFilter) (ProjectEvidenceFilter, error) {
	filter.EvidenceType = strings.ToLower(strings.TrimSpace(filter.EvidenceType))
	if filter.EvidenceType == "all" {
		filter.EvidenceType = ""
	}
	if filter.EvidenceType != "" && !statusIn(filter.EvidenceType, "document", "photo", "checklist", "link", "note", "other") {
		return filter, validationError("Loại minh chứng lọc không hợp lệ", Record{"evidence_type": filter.EvidenceType})
	}
	filter.Status = strings.ToLower(strings.TrimSpace(filter.Status))
	if filter.Status == "all" {
		filter.Status = ""
	}
	if filter.Status == "pending" || filter.Status == "submitted" || filter.Status == "draft" {
		filter.Status = "waiting"
	}
	if filter.Status != "" && !statusIn(filter.Status, "waiting", "accepted", "rejected") {
		return filter, validationError("Trạng thái hồ sơ lọc không hợp lệ", Record{"status": filter.Status})
	}
	filter.WorkItemID = cleanID(filter.WorkItemID)
	if filter.WorkItemID != "" && !validActivityUUID(filter.WorkItemID) {
		return filter, validationError("Đầu việc lọc không hợp lệ", Record{"work_item_id": filter.WorkItemID})
	}
	filter.SubmittedBy = cleanID(filter.SubmittedBy)
	if filter.SubmittedBy != "" && !validActivityUUID(filter.SubmittedBy) {
		return filter, validationError("Người gửi lọc không hợp lệ", Record{"submitted_by": filter.SubmittedBy})
	}
	filter.Visibility = strings.ToLower(strings.TrimSpace(filter.Visibility))
	if filter.Visibility != "" && !statusIn(filter.Visibility, "internal", "project", "client") {
		return filter, validationError("Phạm vi hiển thị lọc không hợp lệ", Record{"visibility": filter.Visibility})
	}
	filter.DateFrom = strings.TrimSpace(filter.DateFrom)
	filter.DateTo = strings.TrimSpace(filter.DateTo)
	var dateFrom, dateTo time.Time
	var err error
	if filter.DateFrom != "" {
		dateFrom, err = time.Parse("2006-01-02", filter.DateFrom)
		if err != nil {
			return filter, validationError("Ngày bắt đầu không hợp lệ", Record{"date_from": filter.DateFrom})
		}
	}
	if filter.DateTo != "" {
		dateTo, err = time.Parse("2006-01-02", filter.DateTo)
		if err != nil {
			return filter, validationError("Ngày kết thúc không hợp lệ", Record{"date_to": filter.DateTo})
		}
	}
	if !dateFrom.IsZero() && !dateTo.IsZero() && dateFrom.After(dateTo) {
		return filter, validationError("Khoảng thời gian hồ sơ không hợp lệ", Record{"date_from": filter.DateFrom, "date_to": filter.DateTo})
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if len([]rune(filter.Search)) > 200 {
		return filter, validationError("Từ khóa tìm kiếm quá dài", Record{"max_length": 200})
	}
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	if filter.Cursor != "" {
		priority, createdAt, evidenceID, err := parseEvidenceCursor(filter.Cursor)
		if err != nil {
			return filter, validationError("Con trỏ hồ sơ không hợp lệ")
		}
		filter.CursorPriority = priority
		filter.CursorCreatedAt = createdAt
		filter.CursorID = evidenceID
	}
	if filter.Limit <= 0 || filter.Limit > 50 {
		filter.Limit = 25
	}
	return filter, nil
}

func encodeEvidenceCursor(evidence Record) string {
	evidenceID := cleanID(evidence["id"])
	priority := int(number(evidence["review_priority"]))
	createdAt := ""
	switch value := evidence["created_at"].(type) {
	case time.Time:
		createdAt = value.UTC().Format(time.RFC3339Nano)
	case string:
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			createdAt = parsed.UTC().Format(time.RFC3339Nano)
		}
	}
	if createdAt == "" || evidenceID == "" || (priority != 0 && priority != 1) {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d|%s|%s", priority, createdAt, evidenceID)))
}

func parseEvidenceCursor(cursor string) (int, string, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, "", "", err
	}
	parts := strings.SplitN(string(raw), "|", 3)
	if len(parts) != 3 || !validActivityUUID(parts[2]) {
		return 0, "", "", errors.New("invalid evidence cursor")
	}
	priority, err := strconv.Atoi(parts[0])
	if err != nil || (priority != 0 && priority != 1) {
		return 0, "", "", errors.New("invalid evidence cursor priority")
	}
	parsed, err := time.Parse(time.RFC3339Nano, parts[1])
	if err != nil {
		return 0, "", "", err
	}
	return priority, parsed.UTC().Format(time.RFC3339Nano), parts[2], nil
}

func normalizeProjectActivityFilter(filter ProjectActivityFilter) (ProjectActivityFilter, error) {
	filter.ScopeType = strings.ToLower(strings.TrimSpace(filter.ScopeType))
	filter.ScopeID = cleanID(filter.ScopeID)
	if filter.ScopeType != "" && !activityTargetScopes[filter.ScopeType] {
		return filter, validationError("Phạm vi hoạt động không hợp lệ", Record{"scope_type": filter.ScopeType})
	}
	if filter.ScopeID != "" && filter.ScopeType == "" {
		return filter, validationError("Cần scope_type khi lọc theo scope_id")
	}
	if filter.ScopeID != "" && !validActivityUUID(filter.ScopeID) {
		return filter, validationError("scope_id không hợp lệ", Record{"scope_id": filter.ScopeID})
	}
	filter.ActivityType = strings.ToLower(strings.TrimSpace(filter.ActivityType))
	if filter.ActivityType != "" && !activityFilterTypes[filter.ActivityType] {
		return filter, validationError("Loại hoạt động lọc không hợp lệ", Record{"activity_type": filter.ActivityType})
	}
	filter.ActorID = cleanID(filter.ActorID)
	if filter.ActorID != "" && !validActivityUUID(filter.ActorID) {
		return filter, validationError("Người gửi lọc không hợp lệ", Record{"actor_id": filter.ActorID})
	}
	filter.Visibility = strings.ToLower(strings.TrimSpace(filter.Visibility))
	if filter.Visibility != "" && !activityVisibilities[filter.Visibility] {
		return filter, validationError("Phạm vi hiển thị lọc không hợp lệ", Record{"visibility": filter.Visibility})
	}
	filter.Kind = strings.ToLower(strings.TrimSpace(filter.Kind))
	if filter.Kind == "all" {
		filter.Kind = ""
	}
	if filter.Kind != "" && filter.Kind != "discussion" && filter.Kind != "system" && filter.Kind != "decision" {
		return filter, validationError("Nhóm hoạt động lọc không hợp lệ", Record{"kind": filter.Kind})
	}
	filter.Attention = strings.ToLower(strings.TrimSpace(filter.Attention))
	if filter.Attention == "all" {
		filter.Attention = ""
	}
	if filter.Attention != "" && !activityAttentionQueues[filter.Attention] {
		return filter, validationError("Hàng đợi chú ý không hợp lệ", Record{"attention": filter.Attention})
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if len([]rune(filter.Search)) > 200 {
		return filter, validationError("Từ khóa tìm kiếm quá dài", Record{"max_length": 200})
	}
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	if filter.Cursor != "" {
		createdAt, activityID, err := parseActivityCursor(filter.Cursor)
		if err != nil {
			return filter, validationError("Con trỏ hoạt động không hợp lệ")
		}
		filter.CursorCreatedAt = createdAt
		filter.CursorID = activityID
	}
	if filter.Limit <= 0 || filter.Limit > 50 {
		filter.Limit = 30
	}
	return filter, nil
}

func encodeActivityCursor(activity Record) string {
	activityID := cleanID(activity["id"])
	createdAt := ""
	if value := strings.TrimSpace(fmt.Sprint(activity["cursor_created_at"])); value != "" && value != "<nil>" {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			createdAt = parsed.UTC().Format(time.RFC3339Nano)
		}
	}
	if createdAt == "" {
		switch value := activity["created_at"].(type) {
		case time.Time:
			createdAt = value.UTC().Format(time.RFC3339Nano)
		case string:
			if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
				createdAt = parsed.UTC().Format(time.RFC3339Nano)
			}
		}
	}
	if createdAt == "" || activityID == "" {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(createdAt + "|" + activityID))
}

func parseActivityCursor(cursor string) (string, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 || !validActivityUUID(parts[1]) {
		return "", "", errors.New("invalid activity cursor")
	}
	parsed, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return "", "", err
	}
	return parsed.UTC().Format(time.RFC3339Nano), parts[1], nil
}

func (s *Service) WorkItemDetail(ctx context.Context, workItemID, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem đầu việc này")
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID, profileIsClient(profile) && !profile.IsManager)
	if err != nil {
		return nil, err
	}
	canActInScope, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	canAct, err := s.myWorkItemCanActNow(ctx, item, actorID, canActInScope)
	if err != nil {
		return nil, err
	}
	startReadiness, err := s.workItemStartReadiness(ctx, item, actorID, profile, canActInScope)
	if err != nil {
		return nil, err
	}
	canDecide := false
	if statusIn(fmt.Sprint(item["status"]), "waiting_review", "waiting_approval") {
		canDecide, err = s.repo.CanDecideWorkItem(ctx, item, actorID)
		if err != nil {
			return nil, err
		}
	}
	if boolValue(item["is_future"]) {
		canDecide = false
	}
	item["capabilities"] = Record{"can_act": canAct, "can_decide": canDecide}
	item["can_act"] = canAct
	item["can_decide"] = canDecide
	item["start_readiness"] = startReadiness
	item["available_actions"] = myWorkItemAvailableActions(item)
	item["readiness"] = myWorkItemReadiness(item)
	businessSource, err := s.repo.WorkflowBusinessSourceForWorkItem(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if len(businessSource) > 0 {
		item["business_source"] = businessSource
	}
	item["execution_target"] = workItemExecutionTarget(item, businessSource)
	completionRequirements, err := s.workItemCompletionRequirements(ctx, item, recordFromAny(item["completion_payload_json"]))
	if err != nil {
		return nil, err
	}
	item["completion_requirements"] = completionRequirements
	delete(item, "pipeline_current_node_id")
	delete(item, "run_node_status")
	delete(item, "is_future")
	return item, nil
}
