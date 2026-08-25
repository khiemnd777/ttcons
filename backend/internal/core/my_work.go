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

var myWorkQueues = map[string]bool{
	"responsible": true, "all": true, "mine": true, "department": true, "pending_approval": true, "watching": true,
}

var myWorkDeadlines = map[string]bool{
	"": true, "overdue": true, "due_7_days": true, "no_due": true,
}

var myWorkSorts = map[string]bool{
	"priority": true, "due_asc": true, "newest": true,
}

var myWorkBuckets = map[string]bool{
	"": true, "all": true, "need_action": true, "in_progress": true, "overdue": true,
	"blocked": true, "waiting_decision": true, "upcoming": true, "completed": true, "done_today": true,
}

var myWorkStatuses = map[string]bool{
	"": true, "pending": true, "ready": true, "in_progress": true, "waiting_evidence": true,
	"waiting_review": true, "waiting_approval": true, "blocked": true, "needs_changes": true,
	"completed": true, "cancelled": true,
}

var myWorkPriorities = map[string]bool{
	"": true, "low": true, "normal": true, "high": true, "urgent": true,
}

func normalizeMyWorkItemsFilter(filter MyWorkItemsFilter) (MyWorkItemsFilter, int, error) {
	filter.Queue = strings.ToLower(strings.TrimSpace(filter.Queue))
	if filter.Queue == "" {
		filter.Queue = "responsible"
	}
	if !myWorkQueues[filter.Queue] {
		return filter, 0, validationError("Hàng đợi công việc không hợp lệ", Record{"queue": filter.Queue})
	}
	filter.ProjectID = cleanID(filter.ProjectID)
	if filter.ProjectID != "" && !validActivityUUID(filter.ProjectID) {
		return filter, 0, validationError("project_id không hợp lệ", Record{"project_id": filter.ProjectID})
	}
	filter.Status = strings.ToLower(strings.TrimSpace(filter.Status))
	if !myWorkStatuses[filter.Status] {
		return filter, 0, validationError("Trạng thái công việc không hợp lệ", Record{"status": filter.Status})
	}
	filter.Deadline = strings.ToLower(strings.TrimSpace(filter.Deadline))
	if !myWorkDeadlines[filter.Deadline] {
		return filter, 0, validationError("Bộ lọc hạn việc không hợp lệ", Record{"deadline": filter.Deadline})
	}
	filter.Priority = strings.ToLower(strings.TrimSpace(filter.Priority))
	if !myWorkPriorities[filter.Priority] {
		return filter, 0, validationError("Mức ưu tiên không hợp lệ", Record{"priority": filter.Priority})
	}
	filter.WorkType = strings.ToLower(strings.TrimSpace(filter.WorkType))
	if filter.WorkType != "" && !validMyWorkToken(filter.WorkType) {
		return filter, 0, validationError("Loại công việc không hợp lệ", Record{"work_type": filter.WorkType})
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if len([]rune(filter.Search)) > 200 {
		return filter, 0, validationError("Từ khóa tìm kiếm không được vượt quá 200 ký tự", nil)
	}
	filter.Sort = strings.ToLower(strings.TrimSpace(filter.Sort))
	if filter.Sort == "" {
		filter.Sort = "priority"
	}
	if !myWorkSorts[filter.Sort] {
		return filter, 0, validationError("Cách sắp xếp công việc không hợp lệ", Record{"sort": filter.Sort})
	}
	filter.Bucket = strings.ToLower(strings.TrimSpace(filter.Bucket))
	if !myWorkBuckets[filter.Bucket] {
		return filter, 0, validationError("Nhóm công việc không hợp lệ", Record{"bucket": filter.Bucket})
	}
	if filter.Limit == 0 {
		filter.Limit = 40
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		return filter, 0, validationError("limit phải nằm trong khoảng 1 đến 100", Record{"limit": filter.Limit})
	}
	offset := 0
	if filter.Cursor != "" {
		parsed, err := parseMyWorkItemsCursor(strings.TrimSpace(filter.Cursor))
		if err != nil {
			return filter, 0, validationError("cursor không hợp lệ", Record{"cursor": filter.Cursor})
		}
		offset = parsed
	}
	return filter, offset, nil
}

func normalizeMyWorkflowActionsFilter(filter MyWorkflowActionsFilter) (MyWorkflowActionsFilter, error) {
	filter.Queue = strings.ToLower(strings.TrimSpace(filter.Queue))
	if filter.Queue == "" {
		filter.Queue = "responsible"
	}
	if !myWorkQueues[filter.Queue] {
		return filter, validationError("Hàng đợi hành động quy trình không hợp lệ", Record{"queue": filter.Queue})
	}
	filter.ProjectID = cleanID(filter.ProjectID)
	if filter.ProjectID != "" && !validActivityUUID(filter.ProjectID) {
		return filter, validationError("project_id không hợp lệ", Record{"project_id": filter.ProjectID})
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if len([]rune(filter.Search)) > 200 {
		return filter, validationError("Từ khóa tìm kiếm không được vượt quá 200 ký tự", nil)
	}
	return filter, nil
}

func validMyWorkToken(value string) bool {
	if len(value) > 64 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func encodeMyWorkItemsCursor(offset int) string {
	if offset < 0 {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("offset:%d", offset)))
}

func parseMyWorkItemsCursor(cursor string) (int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, err
	}
	parts := strings.SplitN(string(raw), ":", 2)
	if len(parts) != 2 || parts[0] != "offset" {
		return 0, errors.New("invalid cursor")
	}
	offset, err := strconv.Atoi(parts[1])
	if err != nil || offset < 0 {
		return 0, errors.New("invalid cursor")
	}
	return offset, nil
}

func myWorkItemQueueScopes(item Record, canDecide bool) []string {
	scopes := make([]string, 0, 5)
	if boolValue(item["is_responsible"]) {
		scopes = append(scopes, "responsible")
	}
	if boolValue(item["is_mine"]) {
		scopes = append(scopes, "mine")
	}
	if boolValue(item["is_department"]) {
		scopes = append(scopes, "department")
	}
	if canDecide {
		scopes = append(scopes, "pending_approval")
	}
	if boolValue(item["is_watching"]) {
		scopes = append(scopes, "watching")
	}
	return scopes
}

func myWorkItemAvailableActions(item Record) []string {
	canAct := boolValue(item["can_act"])
	canDecide := boolValue(item["can_decide"])
	startReadiness := recordFromAny(item["start_readiness"])
	status := fmt.Sprint(item["status"])
	actions := make([]string, 0, 6)
	for _, action := range []string{"start", "complete", "block", "resolve-blocker", "request-review", "request-approval"} {
		plan, err := workItemTransitionForAction(action, item)
		canRun := canAct
		if action == "start" && len(startReadiness) > 0 {
			canRun = boolValue(startReadiness["can_start"])
		}
		if err == nil && !plan.Decision && canRun {
			actions = append(actions, action)
		}
	}
	if canDecide && statusIn(status, "waiting_review", "waiting_approval") {
		actions = append(actions, "approve", "reject", "request-change")
	}
	return actions
}

func myWorkItemReadiness(item Record) string {
	if statusIn(fmt.Sprint(item["status"]), "completed", "cancelled") {
		return "history"
	}
	if boolValue(item["can_decide"]) {
		return "decision"
	}
	if boolValue(item["can_act"]) {
		return "actionable"
	}
	if boolValue(item["is_future"]) {
		return "upcoming"
	}
	return "read_only"
}

func myWorkPlannedStartInFuture(value any, now time.Time) bool {
	var planned time.Time
	switch typed := value.(type) {
	case time.Time:
		planned = typed
	case *time.Time:
		if typed != nil {
			planned = *typed
		}
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05-07"} {
			if parsed, err := time.Parse(layout, text); err == nil {
				planned = parsed
				break
			}
		}
	}
	return !planned.IsZero() && planned.After(now)
}

func (s *Service) myWorkItemCanActNow(ctx context.Context, item Record, actorID string, scoped bool) (bool, error) {
	if !scoped || statusIn(fmt.Sprint(item["status"]), "completed", "cancelled") {
		return false, nil
	}
	started := workItemHasStarted(item)
	if !started && myWorkPlannedStartInFuture(item["planned_start_at"], time.Now()) {
		return false, nil
	}
	if !started && int(number(item["pipeline_engine_version"])) >= 2 && cleanID(item["pipeline_run_id"]) != "" {
		if cleanID(item["pipeline_current_node_id"]) != cleanID(item["run_node_id"]) ||
			!statusIn(fmt.Sprint(item["run_node_status"]), "in_progress", "waiting_approval", "blocked") {
			return false, nil
		}
	}
	requirement, _ := item["evidence_requirement_json"].(map[string]any)
	requiredPermission := strings.TrimSpace(fmt.Sprint(requirement["required_permission"]))
	if requiredPermission == "" || requiredPermission == "<nil>" {
		return true, nil
	}
	return s.repo.UserHasAnyProjectPermission(ctx, actorID, cleanID(item["project_id"]), []string{requiredPermission})
}
