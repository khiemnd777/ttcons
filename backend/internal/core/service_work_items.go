package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Service) WorkItemAction(ctx context.Context, workItemID, action string, req WorkItemActionRequest, actorID, actorName string) (Record, error) {
	canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, forbiddenError("Bạn không có quyền xem đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	plan, err := workItemTransitionForAction(action, item)
	if err != nil {
		return nil, err
	}
	actionScopeChecked := false
	if activeErr := s.requireActivePipelineWorkItem(ctx, item); activeErr != nil {
		if action != "start" {
			return nil, activeErr
		}
		allowed, scopeErr := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
		if scopeErr != nil {
			return nil, scopeErr
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền thao tác đầu việc này")
		}
		if permissionErr := s.requireWorkItemPermission(ctx, item, actorID); permissionErr != nil {
			return nil, permissionErr
		}
		profile, profileErr := s.repo.WorkItemAccessProfile(ctx, actorID)
		if profileErr != nil {
			return nil, profileErr
		}
		startReadiness, readinessErr := s.workItemStartReadiness(ctx, item, actorID, profile, allowed)
		if readinessErr != nil {
			return nil, readinessErr
		}
		if !req.BypassStartRequirements {
			return nil, validationError("Cần xác nhận các điều kiện bắt đầu trước khi thực hiện công việc sớm", Record{
				"start_readiness": startReadiness,
			})
		}
		if !boolValue(startReadiness["can_bypass"]) {
			return nil, validationError("Công việc chưa thể bắt đầu sớm do nhánh workflow hoặc điều kiện chặn chưa được giải quyết", Record{
				"start_readiness": startReadiness,
			})
		}
		reason := workItemActionComment(req)
		if reason == "" {
			return nil, validationError("Lý do bắt đầu sớm là bắt buộc", Record{"action": action})
		}
		expectedToken := strings.TrimSpace(fmt.Sprint(startReadiness["snapshot_token"]))
		if strings.TrimSpace(req.StartRequirementsToken) == "" || req.StartRequirementsToken != expectedToken {
			return nil, conflictError("Điều kiện bắt đầu đã thay đổi, vui lòng tải lại và xác nhận lại", Record{
				"start_readiness": startReadiness,
			})
		}
		overrideMode := "out_of_sequence"
		if cleanID(startReadiness["current_node_id"]) == cleanID(startReadiness["target_node_id"]) {
			overrideMode = "schedule_override"
		}
		req.StartOverrideMetadata = Record{
			"mode":                     overrideMode,
			"actor_id":                 actorID,
			"actor_name":               actorName,
			"started_at":               time.Now().UTC().Format(time.RFC3339Nano),
			"reason":                   reason,
			"current_node_id":          startReadiness["current_node_id"],
			"current_node_name":        startReadiness["current_node_name"],
			"target_node_id":           startReadiness["target_node_id"],
			"target_node_name":         startReadiness["target_node_name"],
			"start_requirements_token": expectedToken,
			"unmet_requirements":       startReadiness["requirements"],
		}
		actionScopeChecked = true
	}
	if plan.RequiresReason && workItemActionComment(req) == "" {
		return nil, validationError("Lý do thao tác là bắt buộc", Record{"action": action})
	}
	if plan.Decision {
		allowed, err := s.repo.CanDecideWorkItem(ctx, item, actorID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền duyệt đầu việc này")
		}
	} else {
		if !actionScopeChecked {
			allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
			if err != nil {
				return nil, err
			}
			if !allowed {
				return nil, forbiddenError("Bạn không có quyền thao tác đầu việc này")
			}
		}
		if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
			return nil, err
		}
	}
	if plan.RequiresEvidence {
		missing, err := s.missingRequiredEvidence(ctx, item, req)
		if err != nil {
			return nil, err
		}
		if len(missing) > 0 {
			return nil, validationError("Đầu việc cần đủ minh chứng trước khi gửi rà soát, gửi duyệt hoặc hoàn tất", Record{"missing_evidence": missing})
		}
	}
	if plan.ToStatus == "completed" {
		if err := s.repo.RunRequiredWorkflowAutomationForWorkItem(ctx, workItemID, actorID, actorName); err != nil {
			return nil, err
		}
	}
	if plan.Decision {
		updated, err := s.repo.ApplyWorkItemDecision(ctx, item, action, plan.ToStatus, plan.ActivityType, req, actorID, actorName)
		if err != nil {
			return nil, err
		}
		s.publishWorkItemChanged(updated, actorID)
		return updated, nil
	}
	if plan.CreatesApproval {
		updated, err := s.repo.RequestWorkItemApproval(ctx, item, req, actorID, actorName)
		if err != nil {
			return nil, err
		}
		s.publishWorkItemChanged(updated, actorID)
		return updated, nil
	}
	updated, err := s.repo.ApplyWorkItemTransition(ctx, item, plan.ToStatus, plan.ActivityType, req, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if plan.ToStatus == "completed" {
		_ = s.repo.RunClientWorkflowAutomationForWorkItem(ctx, cleanID(updated["id"]), actorID, actorName)
	}
	s.publishWorkItemChanged(updated, actorID)
	return updated, nil
}

func (s *Service) publishWorkItemChanged(item Record, actorID string) {
	s.publishActivityEvent(ActivityStreamEvent{
		Type:       "work_item_changed",
		ProjectID:  cleanID(item["project_id"]),
		WorkItemID: cleanID(item["id"]),
		ActorID:    actorID,
	})
}

func (s *Service) requireActivePipelineWorkItem(ctx context.Context, item Record) error {
	started := workItemHasStarted(item)
	if !started && myWorkPlannedStartInFuture(item["planned_start_at"], time.Now()) {
		return validationError("Đầu việc chưa đến thời điểm được phép bắt đầu", Record{
			"work_item_id":     item["id"],
			"planned_start_at": item["planned_start_at"],
		})
	}
	if int(number(item["pipeline_engine_version"])) < 2 || cleanID(item["pipeline_run_id"]) == "" {
		return nil
	}
	rows, err := s.repo.Query(ctx, `
		SELECT pr.current_node_id, pr.status AS run_status, rn.status AS node_status
		FROM pipeline_runs pr
		LEFT JOIN pipeline_run_nodes rn ON rn.id=pr.current_node_id
		WHERE pr.id=$1
	`, item["pipeline_run_id"])
	if err != nil {
		return err
	}
	currentNodeID := ""
	if len(rows) > 0 {
		currentNodeID = cleanID(rows[0]["current_node_id"])
	}
	if len(rows) == 0 {
		return validationError("Không tìm thấy pipeline run của đầu việc")
	}
	if statusIn(fmt.Sprint(rows[0]["run_status"]), "completed", "cancelled") {
		return validationError("Pipeline run không còn cho phép thao tác đầu việc", Record{"run_status": rows[0]["run_status"]})
	}
	if started && currentNodeID != "" && currentNodeID != cleanID(item["run_node_id"]) {
		if !statusIn(fmt.Sprint(rows[0]["node_status"]), "in_progress", "waiting_approval", "blocked") ||
			!statusIn(fmt.Sprint(item["run_node_status"]), "pending", "in_progress", "waiting_approval", "blocked") {
			return validationError("Pipeline run hoặc node bắt đầu sớm không còn cho phép thao tác", Record{
				"current_node_status": rows[0]["node_status"],
				"target_node_status":  item["run_node_status"],
			})
		}
		return nil
	}
	if currentNodeID == "" || currentNodeID != cleanID(item["run_node_id"]) {
		return validationError("Đầu việc chưa thuộc node đang thực hiện của quy trình", Record{
			"work_item_id":    item["id"],
			"run_node_id":     item["run_node_id"],
			"current_node_id": currentNodeID,
		})
	}
	if !statusIn(fmt.Sprint(rows[0]["node_status"]), "in_progress", "waiting_approval", "blocked") {
		return validationError("Node của đầu việc chưa sẵn sàng thao tác", Record{"node_status": rows[0]["node_status"]})
	}
	return nil
}

func (s *Service) requireActiveConstructionWorkItem(ctx context.Context, projectID, workItemID string) (Record, error) {
	projectID = cleanID(projectID)
	workItemID = cleanID(workItemID)
	if !validActivityUUID(projectID) || !validActivityUUID(workItemID) {
		return nil, validationError("Hạng mục thi công không hợp lệ", Record{"work_item_id": workItemID})
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, validationError("Không tìm thấy hạng mục thi công", Record{"work_item_id": workItemID})
	}
	if err != nil {
		return nil, err
	}
	if cleanID(item["project_id"]) != projectID || fmt.Sprint(item["status"]) == "cancelled" {
		return nil, validationError("Hạng mục không thuộc công trình hoặc đã bị hủy", Record{"work_item_id": workItemID})
	}
	if statusIn(fmt.Sprint(item["status"]), "completed", "done") {
		return nil, validationError("Hạng mục thi công đã hoàn tất, không thể ghi nhận phát sinh mới", Record{"work_item_id": workItemID, "status": item["status"]})
	}
	if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
		return nil, err
	}
	if int(number(item["pipeline_engine_version"])) >= 2 && cleanID(item["pipeline_run_id"]) != "" &&
		fmt.Sprint(item["run_step_business_code"]) != "construction.item_reporting" &&
		fmt.Sprint(item["run_node_business_code"]) != "construction_execution" &&
		fmt.Sprint(item["run_node_type"]) != "construction" {
		return nil, validationError("Hạng mục không thuộc bước Thi công của quy trình", Record{
			"work_item_id":           workItemID,
			"run_node_business_code": item["run_node_business_code"],
			"run_step_business_code": item["run_step_business_code"],
		})
	}
	return item, nil
}

type workItemTransitionPlan struct {
	ToStatus         string
	ActivityType     string
	RequiresEvidence bool
	RequiresReason   bool
	CreatesApproval  bool
	Decision         bool
}

func workItemTransitionForAction(action string, item Record) (workItemTransitionPlan, error) {
	fromStatus := fmt.Sprint(item["status"])
	switch action {
	case "start":
		if !statusIn(fromStatus, "pending", "ready", "needs_changes") {
			return workItemTransitionPlan{}, validationError("Trạng thái đầu việc không cho phép bắt đầu", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "in_progress", ActivityType: "started"}, nil
	case "complete":
		if !statusIn(fromStatus, "ready", "in_progress", "needs_changes", "waiting_evidence") {
			return workItemTransitionPlan{}, validationError("Trạng thái đầu việc không cho phép hoàn tất", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "completed", ActivityType: "completed", RequiresEvidence: true}, nil
	case "block":
		if !statusIn(fromStatus, "pending", "ready", "in_progress", "waiting_evidence", "waiting_review", "waiting_approval", "needs_changes") {
			return workItemTransitionPlan{}, validationError("Trạng thái đầu việc không cho phép báo điểm chặn", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "blocked", ActivityType: "blocked", RequiresReason: true}, nil
	case "resolve-blocker":
		if fromStatus != "blocked" {
			return workItemTransitionPlan{}, validationError("Chỉ đầu việc đang bị chặn mới có thể gỡ điểm chặn", Record{"status": fromStatus})
		}
		toStatus := "ready"
		if cleanID(item["started_at"]) != "" {
			toStatus = "in_progress"
		}
		return workItemTransitionPlan{ToStatus: toStatus, ActivityType: "blocker_resolved"}, nil
	case "request-review":
		if !statusIn(fromStatus, "ready", "in_progress", "needs_changes", "waiting_evidence") {
			return workItemTransitionPlan{}, validationError("Trạng thái đầu việc không cho phép gửi rà soát", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "waiting_review", ActivityType: "review_requested", RequiresEvidence: true}, nil
	case "request-approval":
		if !statusIn(fromStatus, "ready", "in_progress", "waiting_review", "needs_changes", "waiting_evidence") {
			return workItemTransitionPlan{}, validationError("Trạng thái đầu việc không cho phép gửi duyệt", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "waiting_approval", ActivityType: "approval_requested", RequiresEvidence: true, CreatesApproval: true}, nil
	case "approve":
		if !statusIn(fromStatus, "waiting_review", "waiting_approval") {
			return workItemTransitionPlan{}, validationError("Trạng thái đầu việc không cho phép duyệt", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "completed", ActivityType: "approved", RequiresEvidence: true, Decision: true}, nil
	case "reject":
		if !statusIn(fromStatus, "waiting_review", "waiting_approval") {
			return workItemTransitionPlan{}, validationError("Trạng thái đầu việc không cho phép từ chối", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "needs_changes", ActivityType: "rejected", RequiresReason: true, Decision: true}, nil
	case "request-change":
		if !statusIn(fromStatus, "waiting_review", "waiting_approval") {
			return workItemTransitionPlan{}, validationError("Trạng thái đầu việc không cho phép yêu cầu chỉnh sửa", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "needs_changes", ActivityType: "changes_requested", RequiresReason: true, Decision: true}, nil
	default:
		return workItemTransitionPlan{}, validationError("Hành động đầu việc không hợp lệ", Record{"action": action})
	}
}

func workItemActionComment(req WorkItemActionRequest) string {
	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		comment = strings.TrimSpace(req.Reason)
	}
	return comment
}

func (s *Service) requireWorkItemPermission(ctx context.Context, item Record, actorID string) error {
	requirement, _ := item["evidence_requirement_json"].(map[string]any)
	requiredPermission := strings.TrimSpace(fmt.Sprint(requirement["required_permission"]))
	if requiredPermission == "" || requiredPermission == "<nil>" {
		return nil
	}
	allowed, err := s.repo.UserHasAnyProjectPermission(ctx, actorID, cleanID(item["project_id"]), []string{requiredPermission})
	if err != nil {
		return err
	}
	if !allowed {
		return forbiddenError("Bạn không có quyền bắt buộc để thao tác đầu việc này", Record{"permission": requiredPermission})
	}
	return nil
}

func (s *Service) requireWorkflowFormArtifactScope(ctx context.Context, item, instance Record, actorID string) error {
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return err
	}
	// The first submitted revision keeps the original current-node gate. A later
	// revision can only exist through an audited amendment, so its confirmations
	// and PDF may be completed without reopening or rewinding the pipeline.
	if int(number(instance["current_revision"])) > 1 {
		return nil
	}
	if statusIn(fmt.Sprint(item["status"]), "completed", "done") {
		allowed, err := s.autoAdvancedWorkflowFormArtifactScope(ctx, item, instance)
		if err != nil {
			return err
		}
		if allowed {
			return nil
		}
		return validationError("Đầu việc đã hoàn tất, hồ sơ không còn cho phép thêm xác nhận hoặc tạo PDF", Record{
			"work_item_id": item["id"],
			"status":       item["status"],
		})
	}
	return s.requireActivePipelineWorkItem(ctx, item)
}

func (s *Service) autoAdvancedWorkflowFormArtifactScope(ctx context.Context, item, instance Record) (bool, error) {
	if fmt.Sprint(instance["status"]) != "submitted" || !statusIn(fmt.Sprint(item["status"]), "completed", "done") {
		return false, nil
	}
	if int(number(item["pipeline_engine_version"])) < 2 {
		return false, nil
	}
	runID := cleanID(item["pipeline_run_id"])
	runNodeID := cleanID(item["run_node_id"])
	if runID == "" || runNodeID == "" || runID != cleanID(instance["pipeline_run_id"]) || runNodeID != cleanID(instance["run_node_id"]) {
		return false, nil
	}
	node, err := s.repo.GetByID(ctx, "pipeline_run_nodes", runNodeID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	exitConditions := recordFromAny(node["exit_conditions"])
	return fmt.Sprint(node["status"]) == "completed" && boolValue(exitConditions["auto_advance_when_ready"]), nil
}

func (s *Service) requireWorkflowFormPDFDeletionScope(ctx context.Context, item Record, actorID string) error {
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return err
	}
	if statusIn(fmt.Sprint(item["status"]), "completed", "done") {
		return validationError("Đầu việc đã hoàn tất, PDF hồ sơ chỉ còn được phép xem", Record{
			"work_item_id": item["id"],
			"status":       item["status"],
		})
	}
	return s.requireActivePipelineWorkItem(ctx, item)
}

func (s *Service) missingRequiredEvidence(ctx context.Context, item Record, req WorkItemActionRequest) ([]string, error) {
	requirements, err := s.workItemCompletionRequirements(ctx, item, req.CompletionPayload)
	if err != nil {
		return nil, err
	}
	missing := []string{}
	for _, requirement := range requirements {
		if boolValue(requirement["required"]) && fmt.Sprint(requirement["status"]) == "missing" {
			missing = append(missing, fmt.Sprint(requirement["key"]))
		}
	}
	return missing, nil
}

func (s *Service) workItemCompletionRequirements(ctx context.Context, item Record, completionPayload Record) ([]Record, error) {
	requirement, _ := item["evidence_requirement_json"].(map[string]any)
	if len(requirement) == 0 {
		return []Record{}, nil
	}
	counts, err := s.repo.WorkItemEvidenceCounts(ctx, fmt.Sprint(item["id"]))
	if err != nil {
		return nil, err
	}
	rows := []Record{}
	engineVersion := int(number(item["pipeline_engine_version"]))
	formRequired := boolValue(item["workflow_form_required"]) || boolValue(requirement["form_required"])
	requiredFormVersionID := cleanID(item["required_workflow_form_version_id"])
	if requiredFormVersionID == "" {
		requiredFormVersionID = cleanID(requirement["form_version_id"])
	}
	exactFormSubmitted := fmt.Sprint(item["workflow_form_status"]) == "submitted" &&
		(requiredFormVersionID == "" || cleanID(item["workflow_form_version_id"]) == requiredFormVersionID)
	if engineVersion >= 2 && formRequired {
		rows = append(rows, completionRequirement("workflow_form", "Hồ sơ nghiệp vụ bắt buộc", exactFormSubmitted, "workflow_form"))
	}
	if boolValue(requirement["document_required"]) {
		rows = append(rows, completionRequirement("document", "Tài liệu", counts["document"] > 0, "evidence"))
	}
	if boolValue(requirement["photo_required"]) {
		rows = append(rows, completionRequirement("photo", "Hình ảnh", counts["photo"] > 0, "evidence"))
	}
	if boolValue(requirement["checklist_required"]) {
		satisfied := counts["checklist"] > 0
		satisfiedBy := "evidence"
		if !satisfied && hasChecklistPayload(completionPayload) {
			satisfied = true
			satisfiedBy = "completion_payload"
		}
		if !satisfied && engineVersion >= 2 && formRequired && exactFormSubmitted {
			satisfied = true
			satisfiedBy = "workflow_form"
		}
		if !satisfied && engineVersion < 2 {
			submittedForm, err := s.repo.HasSubmittedWorkflowFormForWorkItem(ctx, fmt.Sprint(item["id"]))
			if err != nil {
				return nil, err
			}
			if submittedForm {
				satisfied = true
				satisfiedBy = "workflow_form"
			}
		}
		rows = append(rows, completionRequirement("checklist", "Checklist của bước", satisfied, satisfiedBy))
	}
	return rows, nil
}

func completionRequirement(key, label string, satisfied bool, satisfiedBy string) Record {
	status := "missing"
	if satisfied {
		status = "satisfied"
	}
	row := Record{"key": key, "label": label, "required": true, "status": status}
	if satisfied {
		row["satisfied_by"] = satisfiedBy
	}
	return row
}

func withLegacyChecklistCompletion(item Record, req WorkItemActionRequest) WorkItemActionRequest {
	if int(number(item["pipeline_engine_version"])) >= 2 {
		return req
	}
	requirement, _ := item["evidence_requirement_json"].(map[string]any)
	if !boolValue(requirement["checklist_required"]) || hasChecklistPayload(req.CompletionPayload) {
		return req
	}
	payload := Record{}
	for key, value := range req.CompletionPayload {
		payload[key] = value
	}
	payload["checklist"] = []any{
		Record{
			"label":   "Đã gửi từ bước quy trình",
			"checked": true,
			"source":  "pipeline_run_step_submit",
		},
	}
	req.CompletionPayload = payload
	return req
}

func stringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func hasChecklistPayload(payload Record) bool {
	if len(payload) == 0 {
		return false
	}
	for _, key := range []string{"checklist", "checklist_items"} {
		if hasNonEmptyPayloadValue(payload[key]) {
			return true
		}
	}
	return false
}

func hasNonEmptyPayloadValue(value any) bool {
	switch v := value.(type) {
	case []any:
		return len(v) > 0
	case []string:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	case Record:
		return len(v) > 0
	case bool:
		return v
	default:
		text := strings.TrimSpace(fmt.Sprint(v))
		return text != "" && text != "<nil>"
	}
}
