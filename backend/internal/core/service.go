package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo      *Repository
	jwtSecret string
	streamHub *ActivityStreamHub
}

func NewService(repo *Repository, jwtSecret string) *Service {
	return &Service{repo: repo, jwtSecret: jwtSecret, streamHub: NewActivityStreamHub()}
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (Record, error) {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		return nil, errors.New("email and password are required")
	}
	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	hash := fmt.Sprint(user["password_hash"])
	ok := false
	if strings.HasPrefix(hash, "demo:") {
		ok = strings.TrimPrefix(hash, "demo:") == req.Password
	} else {
		ok = bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) == nil
	}
	if !ok {
		return nil, sql.ErrNoRows
	}
	roles, err := s.repo.UserRoles(ctx, fmt.Sprint(user["id"]))
	if err != nil {
		return nil, err
	}
	permissions, err := s.repo.UserPermissions(ctx, fmt.Sprint(user["id"]))
	if err != nil {
		return nil, err
	}
	token, err := s.signToken(fmt.Sprint(user["id"]), fmt.Sprint(user["email"]), roles)
	if err != nil {
		return nil, err
	}
	delete(user, "password_hash")
	user["roles"] = roles
	user["permissions"] = permissions
	return Record{"token": token, "user": user}, nil
}

func (s *Service) Me(ctx context.Context, userID string) (Record, error) {
	user, err := s.repo.GetByID(ctx, "users", userID)
	if err != nil {
		return nil, err
	}
	roles, _ := s.repo.UserRoles(ctx, userID)
	permissions, _ := s.repo.UserPermissions(ctx, userID)
	delete(user, "password_hash")
	user["roles"] = roles
	user["permissions"] = permissions
	return user, nil
}

func (s *Service) signToken(userID, email string, roles []string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"roles":   roles,
		"exp":     time.Now().Add(12 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *Service) ListProjects(ctx context.Context, actorID string) ([]Record, error) {
	return s.repo.ListProjects(ctx, actorID)
}

func (s *Service) CreateProject(ctx context.Context, body Record) (Record, error) {
	if body["code"] == nil || body["code"] == "" {
		body["code"] = fmt.Sprintf("PRJ-%d", time.Now().Unix())
	}
	if body["status"] == nil {
		body["status"] = "draft"
	}
	return s.repo.Insert(ctx, "projects", projectColumns(), body)
}

func (s *Service) UpdateProject(ctx context.Context, id string, body Record) (Record, error) {
	return s.repo.Update(ctx, "projects", projectColumns(), id, body)
}

func (s *Service) ProjectDetail(ctx context.Context, id string) (Record, error) {
	return s.repo.GetProjectDetail(ctx, id)
}

func (s *Service) DeleteProject(ctx context.Context, id string) error {
	return s.repo.Exec(ctx, `UPDATE projects SET status='cancelled', updated_at=now() WHERE id=$1`, id)
}

func (s *Service) ListTemplates(ctx context.Context) ([]Record, error) {
	return s.repo.Query(ctx, `
		SELECT t.*, COUNT(n.id) AS node_count
		FROM pipeline_templates t
		LEFT JOIN pipeline_template_nodes n ON n.template_id=t.id
		WHERE t.status <> 'deleted'
		GROUP BY t.id
		ORDER BY t.updated_at DESC, t.created_at DESC
	`)
}

func (s *Service) TemplateDetail(ctx context.Context, id string) (Record, error) {
	return s.repo.GetTemplateDetail(ctx, id)
}

func (s *Service) CreateTemplate(ctx context.Context, body Record, actorID string) (Record, error) {
	if body["status"] == nil {
		body["status"] = "draft"
	}
	return s.repo.CreateTemplateWithNodes(ctx, body, actorID)
}

func (s *Service) UpdateTemplate(ctx context.Context, id string, body Record) (Record, error) {
	return s.repo.Update(ctx, "pipeline_templates", templateColumns(), id, body)
}

func (s *Service) DeleteTemplate(ctx context.Context, id string) error {
	return s.repo.Exec(ctx, `UPDATE pipeline_templates SET status='deleted', updated_at=now() WHERE id=$1`, id)
}

func (s *Service) PublishTemplate(ctx context.Context, id, actorID, actorName string) (Record, error) {
	template, err := s.repo.QueryOne(ctx, `UPDATE pipeline_templates SET status='published', updated_at=now() WHERE id=$1 RETURNING *`, id)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, "", actorID, actorName, "pipeline_template_published", "pipeline_template", id, fmt.Sprintf("Publish template %s", template["name"]), Record{})
	return s.repo.GetTemplateDetail(ctx, id)
}

func (s *Service) CloneTemplate(ctx context.Context, id, actorID string) (Record, error) {
	return s.repo.CloneTemplate(ctx, id, actorID)
}

func (s *Service) StartPipeline(ctx context.Context, projectID, templateID, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền chạy pipeline của công trình này")
	}
	canRun, err := s.repo.UserHasAnyProjectPermission(ctx, actorID, projectID, []string{"pipeline.run"})
	if err != nil {
		return nil, err
	}
	if !canRun {
		return nil, forbiddenError("Bạn không có quyền chạy pipeline", Record{"permissions": []string{"pipeline.run"}})
	}
	return s.repo.StartPipeline(ctx, projectID, templateID, actorID, actorName)
}

func (s *Service) ProjectWorkItems(ctx context.Context, projectID, actorID string) ([]Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem công việc của công trình này")
	}
	return s.repo.ListProjectWorkItems(ctx, projectID, actorID)
}

func (s *Service) MyWorkItems(ctx context.Context, actorID string) ([]Record, error) {
	return s.repo.ListMyWorkItems(ctx, actorID)
}

func (s *Service) MyActivityAttention(ctx context.Context, filter ActivityAttentionFilter, actorID string) ([]Record, error) {
	filter.Queue = strings.TrimSpace(filter.Queue)
	if filter.Queue == "" {
		filter.Queue = "all"
	}
	if !activityAttentionQueues[filter.Queue] {
		return nil, validationError("Queue activity không hợp lệ", Record{"queue": filter.Queue})
	}
	filter.ProjectID = cleanID(filter.ProjectID)
	if filter.ProjectID != "" {
		if !validActivityUUID(filter.ProjectID) {
			return nil, validationError("project_id không hợp lệ", Record{"project_id": filter.ProjectID})
		}
		allowed, err := s.repo.CanViewProjectOperations(ctx, filter.ProjectID, actorID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem activity của công trình này")
		}
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 80
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListMyActivityAttention(ctx, actorID, profile, filter)
}

func (s *Service) ProjectOperations(ctx context.Context, projectID, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem điều hành công trình này")
	}
	return s.repo.ProjectOperations(ctx, projectID, actorID)
}

func (s *Service) ProjectActivity(ctx context.Context, projectID string, filter ProjectActivityFilter, actorID string) ([]Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem activity của công trình này")
	}
	filter.ScopeType = strings.TrimSpace(filter.ScopeType)
	filter.ScopeID = cleanID(filter.ScopeID)
	if filter.ScopeType != "" && !activityTargetScopes[filter.ScopeType] {
		return nil, validationError("Scope activity không hợp lệ", Record{"scope_type": filter.ScopeType})
	}
	if filter.ScopeID != "" && filter.ScopeType == "" {
		return nil, validationError("Cần scope_type khi lọc theo scope_id")
	}
	if filter.ScopeID != "" && !validActivityUUID(filter.ScopeID) {
		return nil, validationError("scope_id không hợp lệ", Record{"scope_id": filter.ScopeID})
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 80
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return s.repo.ProjectActivityThread(ctx, projectID, profile, filter)
}

func (s *Service) WorkItemDetail(ctx context.Context, workItemID, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem work item này")
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetWorkItemDetail(ctx, workItemID, profileIsClient(profile) && !profile.IsManager)
}

func (s *Service) WorkItemAction(ctx context.Context, workItemID, action string, req WorkItemActionRequest, actorID, actorName string) (Record, error) {
	canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, forbiddenError("Bạn không có quyền xem work item này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	plan, err := workItemTransitionForAction(action, item)
	if err != nil {
		return nil, err
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
			return nil, forbiddenError("Bạn không có quyền duyệt work item này")
		}
	} else {
		allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền thao tác work item này")
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
			return nil, validationError("Work item cần đủ evidence trước khi gửi review, gửi duyệt hoặc hoàn tất", Record{"missing_evidence": missing})
		}
	}
	if plan.Decision {
		return s.repo.ApplyWorkItemDecision(ctx, item, action, plan.ToStatus, plan.ActivityType, req, actorID, actorName)
	}
	if plan.CreatesApproval {
		return s.repo.RequestWorkItemApproval(ctx, item, req, actorID, actorName)
	}
	return s.repo.ApplyWorkItemTransition(ctx, item, plan.ToStatus, plan.ActivityType, req, actorID, actorName)
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
			return workItemTransitionPlan{}, validationError("Trạng thái work item không cho phép bắt đầu", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "in_progress", ActivityType: "started"}, nil
	case "complete":
		if !statusIn(fromStatus, "ready", "in_progress", "needs_changes", "waiting_evidence") {
			return workItemTransitionPlan{}, validationError("Trạng thái work item không cho phép hoàn tất", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "completed", ActivityType: "completed", RequiresEvidence: true}, nil
	case "block":
		if !statusIn(fromStatus, "pending", "ready", "in_progress", "waiting_evidence", "waiting_review", "waiting_approval", "needs_changes") {
			return workItemTransitionPlan{}, validationError("Trạng thái work item không cho phép báo blocker", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "blocked", ActivityType: "blocked", RequiresReason: true}, nil
	case "resolve-blocker":
		if fromStatus != "blocked" {
			return workItemTransitionPlan{}, validationError("Chỉ work item đang blocked mới có thể resolve blocker", Record{"status": fromStatus})
		}
		toStatus := "ready"
		if cleanID(item["started_at"]) != "" {
			toStatus = "in_progress"
		}
		return workItemTransitionPlan{ToStatus: toStatus, ActivityType: "blocker_resolved"}, nil
	case "request-review":
		if !statusIn(fromStatus, "ready", "in_progress", "needs_changes", "waiting_evidence") {
			return workItemTransitionPlan{}, validationError("Trạng thái work item không cho phép gửi review", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "waiting_review", ActivityType: "review_requested", RequiresEvidence: true}, nil
	case "request-approval":
		if !statusIn(fromStatus, "ready", "in_progress", "waiting_review", "needs_changes", "waiting_evidence") {
			return workItemTransitionPlan{}, validationError("Trạng thái work item không cho phép gửi duyệt", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "waiting_approval", ActivityType: "approval_requested", RequiresEvidence: true, CreatesApproval: true}, nil
	case "approve":
		if !statusIn(fromStatus, "waiting_review", "waiting_approval") {
			return workItemTransitionPlan{}, validationError("Trạng thái work item không cho phép approve", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "completed", ActivityType: "approved", RequiresEvidence: true, Decision: true}, nil
	case "reject":
		if !statusIn(fromStatus, "waiting_review", "waiting_approval") {
			return workItemTransitionPlan{}, validationError("Trạng thái work item không cho phép reject", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "needs_changes", ActivityType: "rejected", RequiresReason: true, Decision: true}, nil
	case "request-change":
		if !statusIn(fromStatus, "waiting_review", "waiting_approval") {
			return workItemTransitionPlan{}, validationError("Trạng thái work item không cho phép request changes", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "needs_changes", ActivityType: "changes_requested", RequiresReason: true, Decision: true}, nil
	default:
		return workItemTransitionPlan{}, validationError("Action work item không hợp lệ", Record{"action": action})
	}
}

func (s *Service) AddWorkItemEvidence(ctx context.Context, workItemID string, req WorkItemEvidenceRequest, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thêm evidence cho work item này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	normalized, err := validateWorkItemEvidenceRequest(req)
	if err != nil {
		return nil, err
	}
	return s.repo.AddWorkItemEvidence(ctx, item, normalized, actorID, actorName)
}

func (s *Service) AddWorkItemActivity(ctx context.Context, workItemID string, req WorkItemActivityRequest, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thêm activity cho work item này")
	}
	normalized, err := validateActivityRequest(req)
	if err != nil {
		return nil, err
	}
	if err := s.requireActivityVisibility(ctx, normalized, actorID); err != nil {
		return nil, err
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	normalized, err = s.validateActivityAttachments(ctx, cleanID(item["project_id"]), workItemID, normalized)
	if err != nil {
		return nil, err
	}
	activity, err := s.repo.AddWorkItemActivity(ctx, item, normalized, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(activityEventFromRecord("activity_changed", activity, actorID))
	return activity, nil
}

func (s *Service) MarkActivityRead(ctx context.Context, activityID, actorID string) (Record, error) {
	activityID = cleanID(activityID)
	if !validActivityUUID(activityID) {
		return nil, validationError("activity_id không hợp lệ", Record{"activity_id": activityID})
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	visible, err := s.repo.ActivityVisibleToUser(ctx, activityID, profile)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, forbiddenError("Bạn không có quyền xem activity này")
	}
	readState, err := s.repo.MarkActivityRead(ctx, activityID, actorID)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(ActivityStreamEvent{
		Type:       "activity_read",
		ProjectID:  cleanID(readState["project_id"]),
		ActivityID: activityID,
		ActorID:    actorID,
	})
	return readState, nil
}

func (s *Service) AddProjectActivity(ctx context.Context, projectID string, req WorkItemActivityRequest, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thêm activity cho công trình này")
	}
	normalized, err := validateActivityRequest(req)
	if err != nil {
		return nil, err
	}
	if err := s.requireActivityVisibility(ctx, normalized, actorID); err != nil {
		return nil, err
	}
	normalized, err = s.normalizeProjectActivityTarget(ctx, projectID, normalized, actorID)
	if err != nil {
		return nil, err
	}
	attachmentWorkItemID := ""
	if normalized.ScopeType == "work_item" {
		attachmentWorkItemID = normalized.ScopeID
	} else if normalized.WorkItemID != "" {
		attachmentWorkItemID = normalized.WorkItemID
	}
	normalized, err = s.validateActivityAttachments(ctx, projectID, attachmentWorkItemID, normalized)
	if err != nil {
		return nil, err
	}
	activity, err := s.repo.AddProjectActivity(ctx, projectID, normalized, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(activityEventFromRecord("activity_changed", activity, actorID))
	return activity, nil
}

var userActivityTypes = map[string]bool{
	"comment":   true,
	"status":    true,
	"blocker":   true,
	"question":  true,
	"decision":  true,
	"reminder":  true,
	"file_note": true,
}

var activityVisibilities = map[string]bool{
	"internal": true,
	"project":  true,
	"client":   true,
}

var activityTargetScopes = map[string]bool{
	"project":             true,
	"work_item":           true,
	"boq_item":            true,
	"procurement_request": true,
	"stock_movement":      true,
	"acceptance":          true,
	"rework":              true,
	"handover":            true,
	"payable":             true,
	"receivable":          true,
	"payment":             true,
}

var activityAttentionQueues = map[string]bool{
	"all":       true,
	"mentions":  true,
	"questions": true,
	"blockers":  true,
	"reminders": true,
	"unread":    true,
}

var activityAttachmentTypes = map[string]bool{
	"evidence": true,
	"document": true,
}

func validateActivityRequest(req WorkItemActivityRequest) (WorkItemActivityRequest, error) {
	activityType := strings.TrimSpace(req.ActivityType)
	if activityType == "" {
		activityType = "comment"
	}
	if activityType == "system_event" {
		return req, validationError("System event chỉ được tạo bởi workflow")
	}
	if !userActivityTypes[activityType] {
		return req, validationError("Loại activity không hợp lệ", Record{"activity_type": activityType})
	}

	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		return req, validationError("Nội dung activity là bắt buộc")
	}

	visibility := strings.TrimSpace(req.Visibility)
	if visibility == "" {
		visibility = "project"
	}
	if !activityVisibilities[visibility] {
		return req, validationError("Visibility activity không hợp lệ", Record{"visibility": visibility})
	}

	metadata := req.Metadata
	if metadata == nil {
		metadata = Record{}
	}

	if activityType == "reminder" {
		dueAt := activityMetadataText(metadata, "due_at")
		if dueAt == "" {
			return req, validationError("Reminder cần ngày nhắc lại", Record{"field": "metadata.due_at"})
		}
		if !validActivityDueDate(dueAt) {
			return req, validationError("Ngày nhắc lại không hợp lệ", Record{"field": "metadata.due_at"})
		}
		metadata["due_at"] = dueAt
	}
	if activityType == "blocker" {
		if dueAt := activityMetadataText(metadata, "due_at"); dueAt != "" && !validActivityDueDate(dueAt) {
			return req, validationError("Hạn xử lý blocker không hợp lệ", Record{"field": "metadata.due_at"})
		}
	}
	req.ActivityType = activityType
	req.Comment = comment
	req.Visibility = visibility
	req.ScopeType = strings.TrimSpace(req.ScopeType)
	req.ScopeID = cleanID(req.ScopeID)
	req.WorkItemID = cleanID(req.WorkItemID)
	attachments, err := normalizeActivityAttachments(req.Attachments, metadata)
	if err != nil {
		return req, err
	}
	req.Attachments = attachments
	if len(attachments) > 0 {
		metadata["attachments"] = attachments
		for _, attachment := range attachments {
			if attachment.Type == "evidence" && activityMetadataText(metadata, "evidence_id") == "" {
				metadata["evidence_id"] = attachment.ID
			}
			if attachment.Type == "document" && activityMetadataText(metadata, "document_id") == "" {
				metadata["document_id"] = attachment.ID
			}
		}
	}
	if activityType == "file_note" && len(attachments) == 0 {
		return req, validationError("File note cần evidence hoặc document", Record{"fields": []string{"attachments", "metadata.evidence_id", "metadata.document_id"}})
	}
	mentions, err := normalizeActivityMentions(req.Mentions)
	if err != nil {
		return req, err
	}
	req.Mentions = mentions
	if len(mentions) > 0 {
		metadata["mentions"] = mentions
	}
	req.Metadata = metadata
	return req, nil
}

func normalizeActivityAttachments(input []ActivityAttachmentRequest, metadata Record) ([]ActivityAttachmentRequest, error) {
	out := make([]ActivityAttachmentRequest, 0, len(input))
	seen := map[string]bool{}
	add := func(attachmentType, attachmentID, label string) error {
		attachmentType = strings.TrimSpace(strings.ToLower(attachmentType))
		attachmentID = cleanID(attachmentID)
		label = strings.TrimSpace(label)
		if attachmentType == "" && attachmentID == "" {
			return nil
		}
		if !activityAttachmentTypes[attachmentType] {
			return validationError("Loại attachment không hợp lệ", Record{"attachment_type": attachmentType})
		}
		if attachmentID == "" || !validActivityUUID(attachmentID) {
			return validationError("Attachment id không hợp lệ", Record{"attachment_type": attachmentType, "id": attachmentID})
		}
		key := attachmentType + ":" + strings.ToLower(attachmentID)
		if seen[key] {
			return nil
		}
		seen[key] = true
		out = append(out, ActivityAttachmentRequest{Type: attachmentType, ID: attachmentID, Label: label})
		return nil
	}
	for _, attachment := range input {
		if err := add(attachment.Type, attachment.ID, attachment.Label); err != nil {
			return nil, err
		}
	}
	if rawAttachments, ok := metadata["attachments"]; ok {
		for _, rawAttachment := range anySlice(rawAttachments) {
			if record, ok := rawAttachment.(map[string]any); ok {
				if err := add(fmt.Sprint(record["type"]), fmt.Sprint(record["id"]), fmt.Sprint(record["label"])); err != nil {
					return nil, err
				}
				continue
			}
			if record, ok := rawAttachment.(Record); ok {
				if err := add(fmt.Sprint(record["type"]), fmt.Sprint(record["id"]), fmt.Sprint(record["label"])); err != nil {
					return nil, err
				}
			}
		}
	}
	if evidenceID := activityMetadataText(metadata, "evidence_id"); evidenceID != "" {
		if err := add("evidence", evidenceID, ""); err != nil {
			return nil, err
		}
	}
	if documentID := activityMetadataText(metadata, "document_id"); documentID != "" {
		if err := add("document", documentID, ""); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func anySlice(value any) []any {
	switch v := value.(type) {
	case []any:
		return v
	case []Record:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	case []ActivityAttachmentRequest:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, Record{"type": item.Type, "id": item.ID, "label": item.Label})
		}
		return out
	default:
		return nil
	}
}

func (s *Service) validateActivityAttachments(ctx context.Context, projectID, workItemID string, req WorkItemActivityRequest) (WorkItemActivityRequest, error) {
	if len(req.Attachments) == 0 {
		return req, nil
	}
	for _, attachment := range req.Attachments {
		switch attachment.Type {
		case "evidence":
			evidence, err := s.repo.GetEvidenceDetail(ctx, attachment.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return req, validationError("Evidence attachment không tồn tại", Record{"evidence_id": attachment.ID})
				}
				return req, err
			}
			if cleanID(evidence["project_id"]) != projectID {
				return req, validationError("Evidence attachment không thuộc công trình này", Record{"evidence_id": attachment.ID})
			}
			if workItemID != "" && cleanID(evidence["work_item_id"]) != workItemID {
				return req, validationError("Evidence attachment không thuộc work item này", Record{"evidence_id": attachment.ID})
			}
			if req.Visibility == "client" && fmt.Sprint(evidence["visibility"]) != "client" {
				return req, validationError("Client-visible activity chỉ được gắn evidence client-visible", Record{"evidence_id": attachment.ID})
			}
		case "document":
			document, err := s.repo.GetDocumentAttachment(ctx, attachment.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return req, validationError("Document attachment không tồn tại", Record{"document_id": attachment.ID})
				}
				return req, err
			}
			if cleanID(document["project_id"]) != projectID {
				return req, validationError("Document attachment không thuộc công trình này", Record{"document_id": attachment.ID})
			}
			if req.Visibility == "client" {
				return req, validationError("Document attachment chưa có visibility; hãy gắn evidence client-visible thay thế", Record{"document_id": attachment.ID})
			}
		}
	}
	return req, nil
}

func normalizeActivityMentions(input []ActivityMentionRequest) ([]ActivityMentionRequest, error) {
	out := make([]ActivityMentionRequest, 0, len(input))
	seen := map[string]bool{}
	for _, mention := range input {
		mentionType := strings.TrimSpace(strings.ToLower(mention.Type))
		mentionID := cleanID(mention.ID)
		label := strings.TrimSpace(mention.Label)
		if mentionType == "" && mentionID == "" {
			continue
		}
		if mentionType != "user" && mentionType != "staff" && mentionType != "role" {
			return nil, validationError("Loại mention không hợp lệ", Record{"mention_type": mention.Type})
		}
		if mentionID == "" {
			return nil, validationError("Mention cần id hoặc role label", Record{"mention_type": mentionType})
		}
		if mentionType != "role" && !validActivityUUID(mentionID) {
			return nil, validationError("Mention user/staff không hợp lệ", Record{"mention_type": mentionType, "id": mentionID})
		}
		key := mentionType + ":" + strings.ToLower(mentionID)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ActivityMentionRequest{Type: mentionType, ID: mentionID, Label: label})
	}
	return out, nil
}

func (s *Service) normalizeProjectActivityTarget(ctx context.Context, projectID string, req WorkItemActivityRequest, actorID string) (WorkItemActivityRequest, error) {
	scopeType := strings.TrimSpace(req.ScopeType)
	if scopeType == "" {
		scopeType = "project"
	}
	if !activityTargetScopes[scopeType] {
		return req, validationError("Scope activity không hợp lệ", Record{"scope_type": scopeType})
	}
	if scopeType == "project" {
		req.ScopeType = "project"
		req.ScopeID = projectID
		req.WorkItemID = ""
		return req, nil
	}

	scopeID := cleanID(req.ScopeID)
	if scopeID == "" {
		return req, validationError("Scope activity cần record đích", Record{"scope_type": scopeType})
	}
	if !validActivityUUID(scopeID) {
		return req, validationError("scope_id không hợp lệ", Record{"scope_id": scopeID})
	}
	if scopeType == "work_item" {
		allowed, err := s.repo.CanActOnWorkItem(ctx, scopeID, actorID)
		if err != nil {
			return req, err
		}
		if !allowed {
			return req, forbiddenError("Bạn không có quyền thêm activity cho work item này")
		}
		item, err := s.repo.GetWorkItemDetail(ctx, scopeID)
		if err != nil {
			return req, err
		}
		if cleanID(item["project_id"]) != projectID {
			return req, validationError("Work item không thuộc công trình này", Record{"work_item_id": scopeID})
		}
		req.ScopeType = "work_item"
		req.ScopeID = scopeID
		req.WorkItemID = scopeID
		return req, nil
	}

	exists, err := s.repo.ProjectActivityTargetExists(ctx, projectID, scopeType, scopeID)
	if err != nil {
		return req, err
	}
	if !exists {
		return req, validationError("Record activity không thuộc công trình này", Record{"scope_type": scopeType, "scope_id": scopeID})
	}
	if req.WorkItemID != "" {
		if !validActivityUUID(req.WorkItemID) {
			return req, validationError("work_item_id không hợp lệ", Record{"work_item_id": req.WorkItemID})
		}
		allowed, err := s.repo.CanActOnWorkItem(ctx, req.WorkItemID, actorID)
		if err != nil {
			return req, err
		}
		if !allowed {
			return req, forbiddenError("Bạn không có quyền liên kết activity với work item này")
		}
		item, err := s.repo.GetWorkItemDetail(ctx, req.WorkItemID)
		if err != nil {
			return req, err
		}
		if cleanID(item["project_id"]) != projectID {
			return req, validationError("Work item liên kết không thuộc công trình này", Record{"work_item_id": req.WorkItemID})
		}
	}
	req.ScopeType = scopeType
	req.ScopeID = scopeID
	return req, nil
}

func validActivityUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func activityMetadataText(metadata Record, keys ...string) string {
	for _, key := range keys {
		text := strings.TrimSpace(fmt.Sprint(metadata[key]))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func validActivityDueDate(value string) bool {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if _, err := time.Parse(layout, value); err == nil {
			return true
		}
	}
	return false
}

func (s *Service) requireActivityVisibility(ctx context.Context, req WorkItemActivityRequest, actorID string) error {
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return err
	}
	if profileIsClient(profile) && !profile.IsManager && req.Visibility != "client" {
		return validationError("Client chỉ được tạo activity client-visible", Record{"visibility": req.Visibility})
	}
	return nil
}

func (s *Service) EvidenceAction(ctx context.Context, evidenceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	if action != "accept" && action != "reject" {
		return nil, validationError("Action evidence không hợp lệ", Record{"action": action})
	}
	evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
	if err != nil {
		return nil, err
	}
	item, err := s.repo.GetWorkItemDetail(ctx, fmt.Sprint(evidence["work_item_id"]))
	if err != nil {
		return nil, err
	}
	allowed, err := s.repo.CanDecideWorkItem(ctx, item, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền review evidence này")
	}
	return s.repo.EvidenceAction(ctx, evidence, item, action, req, actorID, actorName)
}

func validateWorkItemEvidenceRequest(req WorkItemEvidenceRequest) (WorkItemEvidenceRequest, error) {
	req.EvidenceType = strings.TrimSpace(req.EvidenceType)
	req.DocumentType = strings.TrimSpace(req.DocumentType)
	req.Name = strings.TrimSpace(req.Name)
	req.DocumentID = cleanID(req.DocumentID)
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.FileURL = strings.TrimSpace(req.FileURL)
	req.MIMEType = strings.TrimSpace(req.MIMEType)
	req.Caption = strings.TrimSpace(req.Caption)
	req.RevisionNo = strings.TrimSpace(req.RevisionNo)
	req.Visibility = strings.TrimSpace(req.Visibility)
	if req.Visibility == "" {
		req.Visibility = "internal"
	}
	if req.Payload == nil {
		req.Payload = Record{}
	}
	validType := map[string]bool{"document": true, "photo": true, "checklist": true, "link": true, "note": true, "other": true}
	if !validType[req.EvidenceType] {
		return req, validationError("Loại evidence không hợp lệ", Record{"evidence_type": req.EvidenceType})
	}
	validVisibility := map[string]bool{"internal": true, "project": true, "client": true}
	if !validVisibility[req.Visibility] {
		return req, validationError("Visibility evidence không hợp lệ", Record{"visibility": req.Visibility})
	}
	if req.Name == "" {
		req.Name = req.Title
	}
	if req.Name == "" && req.Caption != "" {
		req.Name = req.Caption
	}
	if req.Name == "" {
		return req, validationError("Tên evidence là bắt buộc")
	}
	if req.FileSize < 0 {
		return req, validationError("Kích thước file không hợp lệ")
	}
	if statusIn(req.EvidenceType, "document", "photo", "link") && req.FileURL == "" && req.DocumentID == "" {
		return req, validationError("Evidence dạng document/photo/link cần file_url hoặc document_id", Record{"evidence_type": req.EvidenceType})
	}
	return req, nil
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
		return forbiddenError("Bạn không có quyền bắt buộc để thao tác work item này", Record{"permission": requiredPermission})
	}
	return nil
}

func (s *Service) missingRequiredEvidence(ctx context.Context, item Record, req WorkItemActionRequest) ([]string, error) {
	requirement, _ := item["evidence_requirement_json"].(map[string]any)
	if len(requirement) == 0 {
		return nil, nil
	}
	counts, err := s.repo.WorkItemEvidenceCounts(ctx, fmt.Sprint(item["id"]))
	if err != nil {
		return nil, err
	}
	missing := []string{}
	if boolValue(requirement["document_required"]) && counts["document"] == 0 {
		missing = append(missing, "document")
	}
	if boolValue(requirement["photo_required"]) && counts["photo"] == 0 {
		missing = append(missing, "photo")
	}
	if boolValue(requirement["checklist_required"]) && counts["checklist"] == 0 && !hasChecklistPayload(req.CompletionPayload) {
		missing = append(missing, "checklist")
	}
	return missing, nil
}

func withLegacyChecklistCompletion(item Record, req WorkItemActionRequest) WorkItemActionRequest {
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
			"label":   "Submitted from pipeline step",
			"checked": true,
			"source":  "pipeline_run_step_submit",
		},
	}
	req.CompletionPayload = payload
	return req
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

func statusIn(status string, allowed ...string) bool {
	for _, candidate := range allowed {
		if status == candidate {
			return true
		}
	}
	return false
}

func (s *Service) PipelineRun(ctx context.Context, projectID, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem pipeline của công trình này")
	}
	return s.repo.ProjectPipelineRun(ctx, projectID, actorID)
}

func (s *Service) SubmitRunStep(ctx context.Context, stepID string, req WorkItemActionRequest, actorID, actorName string) (Record, error) {
	item, err := s.repo.GetWorkItemByRunStep(ctx, stepID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		fromStatus := fmt.Sprint(item["status"])
		if !canCompleteWorkItemStatus(fromStatus) {
			return nil, validationError("Trạng thái work item không cho phép hoàn tất", Record{"status": fromStatus})
		}
		allowed, err := s.repo.CanActOnWorkItem(ctx, fmt.Sprint(item["id"]), actorID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền thao tác work item này")
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
			return nil, validationError("Work item cần đủ evidence trước khi hoàn tất", Record{"missing_evidence": missing})
		}
	}
	return s.repo.SubmitRunStep(ctx, stepID, req, actorID, actorName)
}

func canCompleteWorkItemStatus(status string) bool {
	return statusIn(status, "ready", "in_progress", "needs_changes", "waiting_evidence")
}

func (s *Service) AdvanceRunNode(ctx context.Context, nodeID, actorID, actorName string) (Record, error) {
	return s.repo.AdvanceRunNode(ctx, nodeID, actorID, actorName)
}

func (s *Service) RequestNodeApproval(ctx context.Context, nodeID, actorID, actorName, reason string) (Record, error) {
	return s.repo.RequestNodeApproval(ctx, nodeID, actorID, actorName, reason)
}

func (s *Service) BlockRunNode(ctx context.Context, nodeID, reason, actorID, actorName string) (Record, error) {
	return s.repo.BlockRunNode(ctx, nodeID, reason, actorID, actorName)
}

func (s *Service) ApprovalAction(ctx context.Context, id, action, comment, actorID, actorName string) (Record, error) {
	action = normalizeApprovalAction(action)
	if _, ok := approvalStatusForAction(action); !ok {
		return nil, validationError("Action approval không hợp lệ", Record{"action": action})
	}
	request, err := s.repo.GetByID(ctx, "approval_requests", id)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(request["target_type"]) == "work_item" {
		workItemID := cleanID(request["target_id"])
		if workItemID == "" {
			return nil, validationError("Approval request chưa liên kết work item", Record{"approval_request_id": id})
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
	return s.repo.ApprovalAction(ctx, id, action, comment, actorID, actorName)
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

func (s *Service) GenerateProcurementFromBOQ(ctx context.Context, projectID, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thao tác mua sắm của công trình này")
	}
	canGenerate, err := s.repo.UserHasAnyProjectPermission(ctx, actorID, projectID, []string{"procurement.create", "procurement.approve", "boq.approve"})
	if err != nil {
		return nil, err
	}
	if !canGenerate {
		return nil, forbiddenError("Bạn không có quyền sinh yêu cầu mua từ BOQ", Record{"permissions": []string{"procurement.create", "procurement.approve", "boq.approve"}})
	}
	return s.repo.GenerateProcurementFromBOQ(ctx, projectID, actorID, actorName)
}

func (s *Service) ProcurementAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.ProcurementAction(ctx, id, action, req, actorID, actorName)
}

func (s *Service) CreateStockMovement(ctx context.Context, req StockActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.CreateStockMovement(ctx, req, actorID, actorName)
}

func (s *Service) AcceptanceAction(ctx context.Context, id, result string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.AcceptanceAction(ctx, id, result, req, actorID, actorName)
}

func (s *Service) CreateReworkFromAcceptance(ctx context.Context, acceptanceID, actorID, actorName string) (Record, error) {
	return s.repo.CreateReworkFromAcceptance(ctx, acceptanceID, actorID, actorName)
}

func (s *Service) HandoverCheck(ctx context.Context, id string) (Record, error) {
	return s.repo.HandoverCheck(ctx, id)
}

func (s *Service) CompleteHandover(ctx context.Context, id string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.CompleteHandover(ctx, id, req, actorID, actorName)
}

func (s *Service) PayableAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.PayableAction(ctx, id, action, req, actorID, actorName)
}

func (s *Service) ReceivableAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.ReceivableAction(ctx, id, action, req, actorID, actorName)
}

func (s *Service) PaymentAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.PaymentAction(ctx, id, action, req, actorID, actorName)
}

func (s *Service) Dashboard(ctx context.Context, actorID string) (Record, error) {
	return s.repo.DashboardReport(ctx, actorID)
}

func (s *Service) Report(ctx context.Context, kind, actorID string) ([]Record, error) {
	return s.repo.Report(ctx, kind, actorID)
}

func (s *Service) AuditLogs(ctx context.Context, projectID, actorID string) ([]Record, error) {
	if projectID != "" {
		return s.repo.Query(ctx, `
			SELECT * FROM audit_logs
			WHERE project_id=$1
			ORDER BY created_at DESC
			LIMIT 200
		`, projectID)
	}
	scopeSQL, scopeArgs, err := s.repo.ProjectAccessFilter(ctx, actorID, "p", 1)
	if err != nil {
		return nil, err
	}
	return s.repo.Query(ctx, fmt.Sprintf(`
		SELECT al.*
		FROM audit_logs al
		JOIN projects p ON p.id=al.project_id
		WHERE p.status <> 'cancelled' AND %s
		ORDER BY al.created_at DESC
		LIMIT 300
	`, scopeSQL), scopeArgs...)
}
