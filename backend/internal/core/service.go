package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo           *Repository
	jwtSecret      string
	streamHub      *ActivityStreamHub
	uploadDir      string
	maxUploadBytes int64
}

func NewService(repo *Repository, jwtSecret string) *Service {
	return &Service{
		repo:           repo,
		jwtSecret:      jwtSecret,
		streamHub:      NewActivityStreamHub(),
		uploadDir:      defaultUploadDir,
		maxUploadBytes: defaultMaxUploadBytes,
	}
}

const (
	defaultUploadDir      = "uploads"
	defaultMaxUploadBytes = int64(10 * 1024 * 1024)
)

type evidenceUploadMimeSpec struct {
	Extension    string
	EvidenceType string
}

var allowedEvidenceUploadMIMEs = map[string]evidenceUploadMimeSpec{
	"image/jpeg":      {Extension: ".jpg", EvidenceType: "photo"},
	"image/png":       {Extension: ".png", EvidenceType: "photo"},
	"image/webp":      {Extension: ".webp", EvidenceType: "photo"},
	"application/pdf": {Extension: ".pdf", EvidenceType: "document"},
}

func (s *Service) ConfigureUploads(uploadDir string, maxUploadBytes int64) {
	uploadDir = strings.TrimSpace(uploadDir)
	if uploadDir == "" {
		uploadDir = defaultUploadDir
	}
	if maxUploadBytes <= 0 {
		maxUploadBytes = defaultMaxUploadBytes
	}
	s.uploadDir = uploadDir
	s.maxUploadBytes = maxUploadBytes
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
	issues, err := s.repo.TemplatePublishFormIssues(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(issues) > 0 {
		return nil, validationError("Template có step bắt buộc workflow form nhưng chưa map form published", Record{"issues": issues})
	}
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

func (s *Service) WorkflowFormCatalog(ctx context.Context) ([]Record, error) {
	return s.repo.WorkflowFormCatalog(ctx)
}

func (s *Service) CreateTemplateStep(ctx context.Context, body Record) (Record, error) {
	if err := s.applyTemplateStepFormDefaults(ctx, body, nil); err != nil {
		return nil, err
	}
	return s.repo.Insert(ctx, "pipeline_template_steps", templateStepColumns(), body)
}

func (s *Service) UpdateTemplateStep(ctx context.Context, id string, body Record) (Record, error) {
	current, err := s.repo.GetByID(ctx, "pipeline_template_steps", id)
	if err != nil {
		return nil, err
	}
	if err := s.applyTemplateStepFormDefaults(ctx, body, current); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, "pipeline_template_steps", templateStepColumns(), id, body)
}

func (s *Service) applyTemplateStepFormDefaults(ctx context.Context, body Record, current Record) error {
	if body == nil {
		return nil
	}
	stepType := "task"
	if current != nil {
		stepType = strings.TrimSpace(fmt.Sprint(current["step_type"]))
	}
	if raw, ok := body["step_type"]; ok {
		stepType = strings.TrimSpace(fmt.Sprint(raw))
	}
	if stepType == "" || stepType == "<nil>" {
		stepType = "task"
	}
	stepName := ""
	if current != nil {
		stepName = strings.TrimSpace(fmt.Sprint(current["name"]))
	}
	if raw, ok := body["name"]; ok {
		stepName = strings.TrimSpace(fmt.Sprint(raw))
	}
	checklistRequired := true
	if current != nil {
		checklistRequired = boolValue(current["checklist_required"])
	}
	if raw, ok := body["checklist_required"]; ok {
		checklistRequired = boolValue(raw)
	}
	formRequired := workflowFormRequiredForStep(stepType, checklistRequired)
	if raw, ok := body["form_required"]; ok {
		formRequired = boolValue(raw)
	} else {
		body["form_required"] = formRequired
	}
	_, formVersionProvided := body["form_version_id"]
	if !formVersionProvided && cleanID(current["form_version_id"]) != "" {
		return nil
	}
	if formVersionProvided && cleanID(body["form_version_id"]) != "" {
		return nil
	}
	if !formRequired {
		return nil
	}
	formVersionID, err := s.repo.DefaultWorkflowFormVersionID(ctx, stepType, checklistRequired, stepName)
	if err != nil {
		return err
	}
	if formVersionID != "" {
		body["form_version_id"] = formVersionID
	}
	return nil
}

func workflowFormRequiredForStep(stepType string, checklistRequired bool) bool {
	return checklistRequired || statusIn(stepType, "daily_report", "meeting_minutes", "checklist", "acceptance_check", "handover_check")
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

func (s *Service) WorkItemForm(ctx context.Context, workItemID, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem workflow form này")
	}
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{"available": false}, nil
	}
	if err != nil {
		return nil, err
	}
	form["available"] = true
	return form, nil
}

func (s *Service) SaveWorkflowFormDraft(ctx context.Context, instanceID string, req WorkflowFormSaveRequest, actorID, actorName string) (Record, error) {
	return s.saveWorkflowForm(ctx, instanceID, req, false, actorID, actorName)
}

func (s *Service) SubmitWorkflowForm(ctx context.Context, instanceID string, req WorkflowFormSaveRequest, actorID, actorName string) (Record, error) {
	return s.saveWorkflowForm(ctx, instanceID, req, true, actorID, actorName)
}

func (s *Service) SubcontractorReconciliation(ctx context.Context, workItemID, actorID string) (Record, error) {
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
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{"available": false, "reason": "Work item chưa có workflow form"}, nil
	}
	if err != nil {
		return nil, err
	}
	if !isSubcontractorChecklistForm(form) {
		return Record{"available": false, "reason": "Workflow form không phải checklist thầu phụ", "form": form}, nil
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return Record{"available": false, "reason": "Submit checklist thầu phụ trước khi đối soát", "form": form}, nil
	}
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"boq.view", "boq.update", "operation.report"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem đối soát thầu phụ", Record{"permissions": []string{"boq.view", "operation.report"}})
	}
	return s.subcontractorReconciliationResponse(ctx, item, form)
}

func (s *Service) SaveSubcontractorReconciliation(ctx context.Context, workItemID string, req SubcontractorReconciliationRequest, actorID, actorName string) (Record, error) {
	return s.saveSubcontractorReconciliation(ctx, workItemID, req, false, actorID, actorName)
}

func (s *Service) SubmitSubcontractorReconciliation(ctx context.Context, workItemID string, req SubcontractorReconciliationRequest, actorID, actorName string) (Record, error) {
	return s.saveSubcontractorReconciliation(ctx, workItemID, req, true, actorID, actorName)
}

func (s *Service) saveSubcontractorReconciliation(ctx context.Context, workItemID string, req SubcontractorReconciliationRequest, submit bool, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thao tác work item này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	canReconcile, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"boq.update", "operation.report"})
	if err != nil {
		return nil, err
	}
	if !canReconcile {
		return nil, forbiddenError("Bạn không có quyền đối soát checklist thầu phụ", Record{"permissions": []string{"boq.update", "operation.report"}})
	}
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if !isSubcontractorChecklistForm(form) {
		return nil, validationError("Workflow form không phải checklist thầu phụ")
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Submit checklist thầu phụ trước khi đối soát", Record{"status": instance["status"]})
	}
	boqItems, err := s.repo.ListProjectBOQ(ctx, projectID)
	if err != nil {
		return nil, err
	}
	normalizedItems, summary, err := s.normalizeSubcontractorReconciliationRequest(ctx, item, form, boqItems, req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveSubcontractorReconciliation(ctx, item, form, strings.TrimSpace(req.Comment), normalizedItems, summary, submit, actorID, actorName); err != nil {
		return nil, err
	}
	return s.subcontractorReconciliationResponse(ctx, item, form)
}

func (s *Service) subcontractorReconciliationResponse(ctx context.Context, item Record, form Record) (Record, error) {
	projectID := cleanID(item["project_id"])
	boqItems, err := s.repo.ListProjectBOQ(ctx, projectID)
	if err != nil {
		return nil, err
	}
	reconciliation, err := s.repo.GetSubcontractorReconciliation(ctx, cleanID(item["id"]))
	if errors.Is(err, sql.ErrNoRows) {
		sourceItems, err := subcontractorChecklistSourceItems(form)
		if err != nil {
			return nil, err
		}
		items := make([]Record, 0, len(sourceItems))
		for index, source := range sourceItems {
			items = append(items, defaultSubcontractorReconciliationItem(index, source))
		}
		instance, _ := form["instance"].(Record)
		answers, _ := form["answers"].(Record)
		reconciliation = Record{
			"id":                 "",
			"project_id":         projectID,
			"work_item_id":       item["id"],
			"form_instance_id":   instance["id"],
			"form_revision_no":   instance["current_revision"],
			"status":             "draft",
			"subcontractor_name": answers["subcontractor_name"],
			"work_package":       answers["work_package"],
			"construction_area":  answers["construction_area"],
			"summary_json":       subcontractorReconciliationSummary(items),
			"comment":            "",
			"items":              items,
		}
	} else if err != nil {
		return nil, err
	}
	return Record{
		"available":      true,
		"work_item_id":   item["id"],
		"project_id":     projectID,
		"form":           form,
		"reconciliation": reconciliation,
		"boq_items":      boqItems,
	}, nil
}

func (s *Service) normalizeSubcontractorReconciliationRequest(ctx context.Context, item Record, form Record, boqItems []Record, req SubcontractorReconciliationRequest) ([]Record, Record, error) {
	if len([]rune(strings.TrimSpace(req.Comment))) > 2000 {
		return nil, nil, validationError("Ghi chú đối soát không được vượt quá 2000 ký tự")
	}
	sourceItems, err := subcontractorChecklistSourceItems(form)
	if err != nil {
		return nil, nil, err
	}
	requestByIndex := map[int]SubcontractorReconciliationItemRequest{}
	for _, itemReq := range req.Items {
		if itemReq.ItemIndex < 0 || itemReq.ItemIndex >= len(sourceItems) {
			return nil, nil, validationError("Dòng đối soát không thuộc checklist", Record{"item_index": itemReq.ItemIndex})
		}
		if _, exists := requestByIndex[itemReq.ItemIndex]; exists {
			return nil, nil, validationError("Dòng đối soát bị trùng", Record{"item_index": itemReq.ItemIndex})
		}
		requestByIndex[itemReq.ItemIndex] = itemReq
	}
	boqByID := map[string]Record{}
	for _, row := range boqItems {
		boqByID[cleanID(row["id"])] = row
	}
	projectID := cleanID(item["project_id"])
	workItemID := cleanID(item["id"])
	normalizedItems := make([]Record, 0, len(sourceItems))
	for index, source := range sourceItems {
		itemReq, hasRequest := requestByIndex[index]
		if !hasRequest {
			itemReq = SubcontractorReconciliationItemRequest{ItemIndex: index}
		}
		normalized, err := s.normalizeSubcontractorReconciliationItem(ctx, projectID, workItemID, index, source, itemReq, boqByID)
		if err != nil {
			return nil, nil, err
		}
		normalizedItems = append(normalizedItems, normalized)
	}
	return normalizedItems, subcontractorReconciliationSummary(normalizedItems), nil
}

func (s *Service) normalizeSubcontractorReconciliationItem(ctx context.Context, projectID, workItemID string, itemIndex int, source Record, req SubcontractorReconciliationItemRequest, boqByID map[string]Record) (Record, error) {
	title := strings.TrimSpace(fmt.Sprint(source["title"]))
	if title == "" || title == "<nil>" {
		return nil, validationError("Checklist item thiếu hạng mục", Record{"item_index": itemIndex})
	}
	claimed, err := optionalWorkflowNumber(source["claimed_quantity"])
	if err != nil {
		return nil, validationError("KL claim không hợp lệ", Record{"item_index": itemIndex})
	}
	confirmed, err := optionalWorkflowNumber(source["confirmed_quantity"])
	if err != nil {
		return nil, validationError("KL xác nhận không hợp lệ", Record{"item_index": itemIndex})
	}
	if req.ConfirmedQuantity != nil {
		if *req.ConfirmedQuantity < 0 {
			return nil, validationError("KL xác nhận không được âm", Record{"item_index": itemIndex})
		}
		value := *req.ConfirmedQuantity
		confirmed = &value
	}
	decision := strings.TrimSpace(req.Decision)
	if decision == "" {
		decision = defaultSubcontractorDecision(source)
	}
	if !statusIn(decision, "accepted", "adjust_quantity", "rework_required", "waiting_evidence", "rejected") {
		return nil, validationError("Quyết định đối soát không hợp lệ", Record{"item_index": itemIndex, "decision": decision})
	}
	if decision == "accepted" && confirmed == nil && claimed != nil {
		value := *claimed
		confirmed = &value
	}
	if decision == "adjust_quantity" && confirmed == nil {
		return nil, validationError("Điều chỉnh KL cần KL xác nhận", Record{"item_index": itemIndex})
	}
	boqItemID := cleanID(req.BOQItemID)
	var boqItem Record
	if boqItemID != "" {
		var ok bool
		boqItem, ok = boqByID[boqItemID]
		if !ok {
			return nil, validationError("Dòng BOQ không thuộc công trình", Record{"item_index": itemIndex, "boq_item_id": boqItemID})
		}
	}
	evidenceID := cleanID(source["evidence_id"])
	if evidenceID != "" {
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			return nil, validationError("Evidence đối soát không hợp lệ", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if cleanID(evidence["project_id"]) != projectID || cleanID(evidence["work_item_id"]) != workItemID || fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, validationError("Evidence đối soát không thuộc work item", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú dòng đối soát không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	var variance any
	if claimed != nil && confirmed != nil {
		variance = *confirmed - *claimed
	}
	row := Record{
		"item_index":         itemIndex,
		"checklist_title":    title,
		"boq_item_id":        boqItemID,
		"boq_item_ref":       cleanOptionalText(source["boq_item_ref"]),
		"claimed_quantity":   numberOrNil(claimed),
		"confirmed_quantity": numberOrNil(confirmed),
		"variance_quantity":  variance,
		"unit":               cleanOptionalText(source["unit"]),
		"status":             cleanOptionalText(source["status"]),
		"evidence_id":        evidenceID,
		"owner_response":     cleanOptionalText(source["owner_response"]),
		"decision":           decision,
		"due_date":           cleanOptionalText(source["due_date"]),
		"comment":            comment,
		"payload_json":       source,
	}
	if boqItem != nil {
		row["boq_work_item_name"] = boqItem["work_item_name"]
		row["boq_quantity"] = boqItem["quantity"]
		row["boq_unit"] = boqItem["unit"]
	}
	return row, nil
}

func isSubcontractorChecklistForm(form Record) bool {
	template, _ := form["template"].(Record)
	return fmt.Sprint(template["code"]) == "builtin_subcontractor_checklist"
}

func subcontractorChecklistSourceItems(form Record) ([]Record, error) {
	answers, _ := form["answers"].(Record)
	rawItems, ok := answers["checklist_items"]
	if !ok || workflowFormValueEmpty(Record{"field_type": "structured_list"}, rawItems) {
		return nil, validationError("Checklist thầu phụ chưa có item")
	}
	return workflowRecordList(rawItems)
}

func defaultSubcontractorReconciliationItem(index int, source Record) Record {
	decision := defaultSubcontractorDecision(source)
	claimed, _ := optionalWorkflowNumber(source["claimed_quantity"])
	confirmed, _ := optionalWorkflowNumber(source["confirmed_quantity"])
	if decision == "accepted" && confirmed == nil && claimed != nil {
		value := *claimed
		confirmed = &value
	}
	var variance any
	if claimed != nil && confirmed != nil {
		variance = *confirmed - *claimed
	}
	return Record{
		"item_index":         index,
		"checklist_title":    cleanOptionalText(source["title"]),
		"boq_item_id":        "",
		"boq_item_ref":       cleanOptionalText(source["boq_item_ref"]),
		"claimed_quantity":   numberOrNil(claimed),
		"confirmed_quantity": numberOrNil(confirmed),
		"variance_quantity":  variance,
		"unit":               cleanOptionalText(source["unit"]),
		"status":             cleanOptionalText(source["status"]),
		"evidence_id":        cleanID(source["evidence_id"]),
		"owner_response":     cleanOptionalText(source["owner_response"]),
		"decision":           decision,
		"due_date":           cleanOptionalText(source["due_date"]),
		"comment":            cleanOptionalText(source["comment"]),
		"payload_json":       source,
	}
}

func defaultSubcontractorDecision(source Record) string {
	ownerResponse := cleanOptionalText(source["owner_response"])
	if statusIn(ownerResponse, "accepted", "adjust_quantity", "rework_required", "waiting_evidence") {
		return ownerResponse
	}
	status := cleanOptionalText(source["status"])
	if statusIn(status, "done", "not_applicable") {
		return "accepted"
	}
	return "waiting_evidence"
}

func subcontractorReconciliationSummary(items []Record) Record {
	summary := Record{
		"total":            len(items),
		"accepted":         0,
		"adjust_quantity":  0,
		"rework_required":  0,
		"waiting_evidence": 0,
		"rejected":         0,
	}
	for _, item := range items {
		decision := cleanOptionalText(item["decision"])
		if _, ok := summary[decision]; ok {
			summary[decision] = int(number(summary[decision])) + 1
		}
	}
	return summary
}

func optionalWorkflowNumber(value any) (*float64, error) {
	if workflowFormValueEmpty(Record{"field_type": "number"}, value) {
		return nil, nil
	}
	numeric, err := workflowNumberValue(value)
	if err != nil {
		return nil, err
	}
	if numeric < 0 {
		return nil, errors.New("Số lượng không được âm")
	}
	return &numeric, nil
}

func numberOrNil(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func cleanOptionalText(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func (s *Service) OwnerClientReconciliation(ctx context.Context, workItemID, actorID string) (Record, error) {
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
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{"available": false, "reason": "Work item chưa có workflow form"}, nil
	}
	if err != nil {
		return nil, err
	}
	if !isOwnerClientChecklistForm(form) {
		return Record{"available": false, "reason": "Workflow form không phải checklist chủ đầu tư", "form": form}, nil
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return Record{"available": false, "reason": "Submit checklist chủ đầu tư trước khi đối soát", "form": form}, nil
	}
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "acceptance.approve", "acceptance.reject", "operation.report"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem đối soát chủ đầu tư", Record{"permissions": []string{"acceptance.view", "operation.report"}})
	}
	return s.ownerClientReconciliationResponse(ctx, item, form)
}

func (s *Service) SaveOwnerClientReconciliation(ctx context.Context, workItemID string, req OwnerClientReconciliationRequest, actorID, actorName string) (Record, error) {
	return s.saveOwnerClientReconciliation(ctx, workItemID, req, false, actorID, actorName)
}

func (s *Service) SubmitOwnerClientReconciliation(ctx context.Context, workItemID string, req OwnerClientReconciliationRequest, actorID, actorName string) (Record, error) {
	return s.saveOwnerClientReconciliation(ctx, workItemID, req, true, actorID, actorName)
}

func (s *Service) saveOwnerClientReconciliation(ctx context.Context, workItemID string, req OwnerClientReconciliationRequest, submit bool, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thao tác work item này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	canReconcile, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.approve", "acceptance.reject", "operation.report"})
	if err != nil {
		return nil, err
	}
	if !canReconcile {
		return nil, forbiddenError("Bạn không có quyền đối soát checklist chủ đầu tư", Record{"permissions": []string{"acceptance.approve", "acceptance.reject", "operation.report"}})
	}
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if !isOwnerClientChecklistForm(form) {
		return nil, validationError("Workflow form không phải checklist chủ đầu tư")
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Submit checklist chủ đầu tư trước khi đối soát", Record{"status": instance["status"]})
	}
	acceptances, err := s.repo.ListProjectAcceptanceRecords(ctx, projectID)
	if err != nil {
		return nil, err
	}
	normalizedItems, summary, err := s.normalizeOwnerClientReconciliationRequest(ctx, item, form, acceptances, req, submit)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveOwnerClientReconciliation(ctx, item, form, strings.TrimSpace(req.Comment), normalizedItems, summary, submit, actorID, actorName); err != nil {
		return nil, err
	}
	return s.ownerClientReconciliationResponse(ctx, item, form)
}

func (s *Service) ownerClientReconciliationResponse(ctx context.Context, item Record, form Record) (Record, error) {
	projectID := cleanID(item["project_id"])
	acceptances, err := s.repo.ListProjectAcceptanceRecords(ctx, projectID)
	if err != nil {
		return nil, err
	}
	reconciliation, err := s.repo.GetOwnerClientReconciliation(ctx, cleanID(item["id"]))
	if errors.Is(err, sql.ErrNoRows) {
		sourceItems, err := ownerClientChecklistSourceItems(form)
		if err != nil {
			return nil, err
		}
		items := make([]Record, 0, len(sourceItems))
		for index, source := range sourceItems {
			items = append(items, defaultOwnerClientReconciliationItem(index, source))
		}
		instance, _ := form["instance"].(Record)
		answers, _ := form["answers"].(Record)
		reconciliation = Record{
			"id":                     "",
			"project_id":             projectID,
			"work_item_id":           item["id"],
			"form_instance_id":       instance["id"],
			"form_revision_no":       instance["current_revision"],
			"status":                 "draft",
			"client_representative":  answers["client_representative"],
			"company_representative": answers["company_representative"],
			"handover_scope":         answers["handover_scope"],
			"scope_detail":           answers["scope_detail"],
			"summary_json":           ownerClientReconciliationSummary(items),
			"comment":                "",
			"items":                  items,
		}
	} else if err != nil {
		return nil, err
	}
	return Record{
		"available":      true,
		"work_item_id":   item["id"],
		"project_id":     projectID,
		"form":           form,
		"reconciliation": reconciliation,
		"acceptances":    acceptances,
	}, nil
}

func (s *Service) normalizeOwnerClientReconciliationRequest(ctx context.Context, item Record, form Record, acceptances []Record, req OwnerClientReconciliationRequest, submit bool) ([]Record, Record, error) {
	if len([]rune(strings.TrimSpace(req.Comment))) > 2000 {
		return nil, nil, validationError("Ghi chú đối soát không được vượt quá 2000 ký tự")
	}
	sourceItems, err := ownerClientChecklistSourceItems(form)
	if err != nil {
		return nil, nil, err
	}
	requestByIndex := map[int]OwnerClientReconciliationItemRequest{}
	for _, itemReq := range req.Items {
		if itemReq.ItemIndex < 0 || itemReq.ItemIndex >= len(sourceItems) {
			return nil, nil, validationError("Dòng đối soát không thuộc checklist", Record{"item_index": itemReq.ItemIndex})
		}
		if _, exists := requestByIndex[itemReq.ItemIndex]; exists {
			return nil, nil, validationError("Dòng đối soát bị trùng", Record{"item_index": itemReq.ItemIndex})
		}
		requestByIndex[itemReq.ItemIndex] = itemReq
	}
	acceptanceByID := map[string]Record{}
	for _, row := range acceptances {
		acceptanceByID[cleanID(row["id"])] = row
	}
	projectID := cleanID(item["project_id"])
	workItemID := cleanID(item["id"])
	normalizedItems := make([]Record, 0, len(sourceItems))
	for index, source := range sourceItems {
		itemReq, hasRequest := requestByIndex[index]
		if !hasRequest {
			itemReq = OwnerClientReconciliationItemRequest{ItemIndex: index}
		}
		normalized, err := s.normalizeOwnerClientReconciliationItem(ctx, projectID, workItemID, index, source, itemReq, acceptanceByID, submit)
		if err != nil {
			return nil, nil, err
		}
		normalizedItems = append(normalizedItems, normalized)
	}
	return normalizedItems, ownerClientReconciliationSummary(normalizedItems), nil
}

func (s *Service) normalizeOwnerClientReconciliationItem(ctx context.Context, projectID, workItemID string, itemIndex int, source Record, req OwnerClientReconciliationItemRequest, acceptanceByID map[string]Record, submit bool) (Record, error) {
	title := strings.TrimSpace(fmt.Sprint(source["title"]))
	if title == "" || title == "<nil>" {
		return nil, validationError("Checklist item thiếu hạng mục", Record{"item_index": itemIndex})
	}
	clientDecision := cleanOptionalText(source["client_decision"])
	if clientDecision != "" && !statusIn(clientDecision, "pass", "conditional_pass", "fail", "not_applicable") {
		return nil, validationError("Quyết định chủ đầu tư không hợp lệ", Record{"item_index": itemIndex, "client_decision": clientDecision})
	}
	severity := cleanOptionalText(source["severity"])
	if severity != "" && !statusIn(severity, "low", "medium", "high", "critical") {
		return nil, validationError("Severity checklist chủ đầu tư không hợp lệ", Record{"item_index": itemIndex, "severity": severity})
	}
	decision := strings.TrimSpace(req.Decision)
	if decision == "" {
		decision = defaultOwnerClientDecision(source)
	}
	if !statusIn(decision, "accepted", "conditional_acceptance", "rework_required", "waiting_internal_acceptance", "rejected", "not_applicable") {
		return nil, validationError("Quyết định đối soát không hợp lệ", Record{"item_index": itemIndex, "decision": decision})
	}
	if clientDecision == "fail" && decision == "accepted" {
		return nil, validationError("Không thể accepted khi chủ đầu tư đánh fail", Record{"item_index": itemIndex})
	}
	acceptanceID := cleanID(req.AcceptanceID)
	var acceptance Record
	if acceptanceID != "" {
		var ok bool
		acceptance, ok = acceptanceByID[acceptanceID]
		if !ok {
			return nil, validationError("Nghiệm thu không thuộc công trình", Record{"item_index": itemIndex, "acceptance_id": acceptanceID})
		}
	}
	if submit && statusIn(decision, "accepted", "conditional_acceptance") {
		if acceptance == nil {
			return nil, validationError("Cần link nghiệm thu nội bộ cho dòng đã chấp nhận", Record{"item_index": itemIndex})
		}
		if fmt.Sprint(acceptance["status"]) != "completed" {
			return nil, validationError("Nghiệm thu nội bộ phải completed", Record{"item_index": itemIndex, "acceptance_id": acceptanceID})
		}
		result := cleanOptionalText(acceptance["result"])
		if decision == "accepted" && result != "pass" {
			return nil, validationError("Accepted cần nghiệm thu nội bộ pass", Record{"item_index": itemIndex, "acceptance_id": acceptanceID, "result": result})
		}
		if decision == "conditional_acceptance" && !statusIn(result, "pass", "conditional_pass") {
			return nil, validationError("Conditional acceptance cần nghiệm thu nội bộ pass hoặc conditional pass", Record{"item_index": itemIndex, "acceptance_id": acceptanceID, "result": result})
		}
	}
	evidenceID := cleanID(source["evidence_id"])
	if evidenceID != "" {
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			return nil, validationError("Evidence đối soát không hợp lệ", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if cleanID(evidence["project_id"]) != projectID || cleanID(evidence["work_item_id"]) != workItemID || fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, validationError("Evidence đối soát không thuộc work item", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú dòng đối soát không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	row := Record{
		"item_index":      itemIndex,
		"checklist_title": title,
		"output_ref":      cleanOptionalText(source["output_ref"]),
		"status":          cleanOptionalText(source["status"]),
		"client_decision": clientDecision,
		"severity":        severity,
		"required_action": cleanOptionalText(source["required_action"]),
		"due_date":        cleanOptionalText(source["due_date"]),
		"evidence_id":     evidenceID,
		"acceptance_id":   acceptanceID,
		"decision":        decision,
		"comment":         comment,
		"payload_json":    source,
	}
	if acceptance != nil {
		row["acceptance_title"] = acceptance["title"]
		row["acceptance_type"] = acceptance["type"]
		row["acceptance_status"] = acceptance["status"]
		row["acceptance_result"] = acceptance["result"]
	}
	return row, nil
}

func isOwnerClientChecklistForm(form Record) bool {
	template, _ := form["template"].(Record)
	return fmt.Sprint(template["code"]) == "builtin_owner_client_checklist"
}

func ownerClientChecklistSourceItems(form Record) ([]Record, error) {
	answers, _ := form["answers"].(Record)
	rawItems, ok := answers["checklist_items"]
	if !ok || workflowFormValueEmpty(Record{"field_type": "structured_list"}, rawItems) {
		return nil, validationError("Checklist chủ đầu tư chưa có item")
	}
	return workflowRecordList(rawItems)
}

func defaultOwnerClientReconciliationItem(index int, source Record) Record {
	return Record{
		"item_index":      index,
		"checklist_title": cleanOptionalText(source["title"]),
		"output_ref":      cleanOptionalText(source["output_ref"]),
		"status":          cleanOptionalText(source["status"]),
		"client_decision": cleanOptionalText(source["client_decision"]),
		"severity":        cleanOptionalText(source["severity"]),
		"required_action": cleanOptionalText(source["required_action"]),
		"due_date":        cleanOptionalText(source["due_date"]),
		"evidence_id":     cleanID(source["evidence_id"]),
		"acceptance_id":   "",
		"decision":        defaultOwnerClientDecision(source),
		"comment":         cleanOptionalText(source["comment"]),
		"payload_json":    source,
	}
}

func defaultOwnerClientDecision(source Record) string {
	clientDecision := cleanOptionalText(source["client_decision"])
	if clientDecision == "not_applicable" || cleanOptionalText(source["status"]) == "not_applicable" {
		return "not_applicable"
	}
	switch clientDecision {
	case "pass":
		return "accepted"
	case "conditional_pass":
		return "conditional_acceptance"
	case "fail":
		return "rework_required"
	default:
		return "waiting_internal_acceptance"
	}
}

func ownerClientReconciliationSummary(items []Record) Record {
	summary := Record{
		"total":                       len(items),
		"accepted":                    0,
		"conditional_acceptance":      0,
		"rework_required":             0,
		"waiting_internal_acceptance": 0,
		"rejected":                    0,
		"not_applicable":              0,
	}
	for _, item := range items {
		decision := cleanOptionalText(item["decision"])
		if _, ok := summary[decision]; ok {
			summary[decision] = int(number(summary[decision])) + 1
		}
	}
	return summary
}

func (s *Service) MaterialSnapshot(ctx context.Context, workItemID, actorID string) (Record, error) {
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
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"warehouse.view", "operation.view", "operation.report", "warehouse.issue"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem snapshot vật tư", Record{"permissions": []string{"warehouse.view", "operation.view"}})
	}
	return s.materialSnapshotResponse(ctx, item)
}

func (s *Service) SaveMaterialSnapshot(ctx context.Context, workItemID string, req MaterialSnapshotRequest, actorID, actorName string) (Record, error) {
	return s.saveMaterialSnapshot(ctx, workItemID, req, false, actorID, actorName)
}

func (s *Service) SubmitMaterialSnapshot(ctx context.Context, workItemID string, req MaterialSnapshotRequest, actorID, actorName string) (Record, error) {
	return s.saveMaterialSnapshot(ctx, workItemID, req, true, actorID, actorName)
}

func (s *Service) saveMaterialSnapshot(ctx context.Context, workItemID string, req MaterialSnapshotRequest, submit bool, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thao tác work item này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	canUpdate, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"operation.report", "warehouse.issue"})
	if err != nil {
		return nil, err
	}
	if !canUpdate {
		return nil, forbiddenError("Bạn không có quyền ghi snapshot vật tư", Record{"permissions": []string{"operation.report", "warehouse.issue"}})
	}
	materials, err := s.repo.ListActiveMaterials(ctx)
	if err != nil {
		return nil, err
	}
	balances, err := s.repo.ProjectMaterialBalances(ctx, projectID)
	if err != nil {
		return nil, err
	}
	normalizedItems, summary, header, err := s.normalizeMaterialSnapshotRequest(ctx, item, materials, balances, req, submit)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveMaterialSnapshot(ctx, item, header, normalizedItems, summary, submit, actorID, actorName); err != nil {
		return nil, err
	}
	return s.materialSnapshotResponse(ctx, item)
}

func (s *Service) materialSnapshotResponse(ctx context.Context, item Record) (Record, error) {
	projectID := cleanID(item["project_id"])
	materials, err := s.repo.ListActiveMaterials(ctx)
	if err != nil {
		return nil, err
	}
	balances, err := s.repo.ProjectMaterialBalances(ctx, projectID)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.repo.GetMaterialSnapshot(ctx, cleanID(item["id"]))
	if errors.Is(err, sql.ErrNoRows) {
		snapshot = Record{
			"id":                      "",
			"project_id":              projectID,
			"work_item_id":            item["id"],
			"status":                  "draft",
			"snapshot_date":           time.Now().Format("2006-01-02"),
			"construction_area":       "",
			"summary_json":            materialSnapshotSummary(nil),
			"comment":                 "",
			"applied_to_site_balance": false,
			"items":                   []Record{},
		}
	} else if err != nil {
		return nil, err
	}
	return Record{
		"available":         true,
		"work_item_id":      item["id"],
		"project_id":        projectID,
		"snapshot":          snapshot,
		"materials":         materials,
		"material_balances": balances,
	}, nil
}

func (s *Service) normalizeMaterialSnapshotRequest(ctx context.Context, item Record, materials []Record, balances []Record, req MaterialSnapshotRequest, submit bool) ([]Record, Record, Record, error) {
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 2000 {
		return nil, nil, nil, validationError("Ghi chú snapshot vật tư không được vượt quá 2000 ký tự")
	}
	constructionArea := strings.TrimSpace(req.ConstructionArea)
	if len([]rune(constructionArea)) > 240 {
		return nil, nil, nil, validationError("Khu vực thi công không được vượt quá 240 ký tự")
	}
	snapshotDate := strings.TrimSpace(req.SnapshotDate)
	if snapshotDate != "" {
		if _, err := time.Parse("2006-01-02", snapshotDate); err != nil {
			return nil, nil, nil, validationError("Ngày snapshot vật tư không hợp lệ", Record{"snapshot_date": snapshotDate})
		}
	} else if submit {
		return nil, nil, nil, validationError("Ngày snapshot vật tư là bắt buộc khi submit")
	}
	if submit && len(req.Items) == 0 {
		return nil, nil, nil, validationError("Cần ít nhất một dòng vật tư để submit snapshot")
	}

	materialByID := map[string]Record{}
	for _, material := range materials {
		materialByID[cleanID(material["id"])] = material
	}
	balanceByMaterialID := map[string]Record{}
	for _, balance := range balances {
		balanceByMaterialID[cleanID(balance["material_id"])] = balance
	}
	seen := map[int]bool{}
	projectID := cleanID(item["project_id"])
	workItemID := cleanID(item["id"])
	normalized := make([]Record, 0, len(req.Items))
	for index, itemReq := range req.Items {
		itemIndex := itemReq.ItemIndex
		if itemIndex < 0 {
			itemIndex = index
		}
		if itemIndex < 0 {
			return nil, nil, nil, validationError("Dòng snapshot vật tư không hợp lệ", Record{"item_index": itemReq.ItemIndex})
		}
		if seen[itemIndex] {
			return nil, nil, nil, validationError("Dòng snapshot vật tư bị trùng", Record{"item_index": itemIndex})
		}
		seen[itemIndex] = true
		row, err := s.normalizeMaterialSnapshotItem(ctx, projectID, workItemID, itemIndex, materialByID, balanceByMaterialID, itemReq, submit)
		if err != nil {
			return nil, nil, nil, err
		}
		normalized = append(normalized, row)
	}
	summary := materialSnapshotSummary(normalized)
	header := Record{
		"snapshot_date":     snapshotDate,
		"construction_area": constructionArea,
		"comment":           comment,
	}
	return normalized, summary, header, nil
}

func (s *Service) normalizeMaterialSnapshotItem(ctx context.Context, projectID, workItemID string, itemIndex int, materialByID map[string]Record, balanceByMaterialID map[string]Record, req MaterialSnapshotItemRequest, submit bool) (Record, error) {
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "site_balance"
	}
	if !statusIn(source, "site_balance", "warehouse_issue", "subcontractor", "other") {
		return nil, validationError("Nguồn vật tư không hợp lệ", Record{"item_index": itemIndex, "source": source})
	}
	affectsSiteBalance := source == "site_balance" || source == "warehouse_issue"
	if req.AffectsSiteBalance != nil {
		affectsSiteBalance = *req.AffectsSiteBalance
	}

	materialID := cleanID(req.MaterialID)
	var material Record
	if materialID != "" {
		var ok bool
		material, ok = materialByID[materialID]
		if !ok {
			return nil, validationError("Vật tư không tồn tại hoặc đã ngưng hoạt động", Record{"item_index": itemIndex, "material_id": materialID})
		}
	}
	if affectsSiteBalance && materialID == "" {
		return nil, validationError("Dòng ảnh hưởng site balance cần chọn vật tư", Record{"item_index": itemIndex})
	}

	materialName := strings.TrimSpace(req.MaterialName)
	materialCode := ""
	if material != nil {
		materialName = strings.TrimSpace(fmt.Sprint(material["name"]))
		materialCode = strings.TrimSpace(fmt.Sprint(material["code"]))
	}
	if materialName == "" {
		return nil, validationError("Tên vật tư là bắt buộc", Record{"item_index": itemIndex})
	}
	if len([]rune(materialName)) > 240 {
		return nil, validationError("Tên vật tư không được vượt quá 240 ký tự", Record{"item_index": itemIndex})
	}

	unit := strings.TrimSpace(req.Unit)
	if unit == "" && material != nil {
		unit = strings.TrimSpace(fmt.Sprint(material["unit"]))
	}
	if unit == "" {
		return nil, validationError("Đơn vị vật tư là bắt buộc", Record{"item_index": itemIndex})
	}
	if len([]rune(unit)) > 40 {
		return nil, validationError("Đơn vị vật tư không được vượt quá 40 ký tự", Record{"item_index": itemIndex})
	}

	beforeQuantity, err := materialSnapshotQuantity(req.BeforeQuantity, "Số lượng trước không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	addedQuantity, err := materialSnapshotQuantity(req.AddedQuantity, "Số lượng bổ sung không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	afterQuantity, err := materialSnapshotQuantity(req.AfterQuantity, "Số lượng sau không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	wasteQuantity, err := materialSnapshotQuantity(req.WasteQuantity, "Số lượng hao hụt/hư hỏng không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	usedQuantity := materialSnapshotRound(beforeQuantity + addedQuantity - afterQuantity - wasteQuantity)
	if usedQuantity < -0.000001 {
		return nil, validationError("Số lượng sử dụng tính ra không được âm", Record{
			"item_index":      itemIndex,
			"before_quantity": beforeQuantity,
			"added_quantity":  addedQuantity,
			"after_quantity":  afterQuantity,
			"waste_quantity":  wasteQuantity,
		})
	}
	if usedQuantity < 0 {
		usedQuantity = 0
	}

	var balance Record
	siteBalanceID := ""
	if affectsSiteBalance {
		balance = balanceByMaterialID[materialID]
		if balance == nil {
			return nil, validationError("Chưa có số dư vật tư tại công trình cho dòng snapshot", Record{"item_index": itemIndex, "material_id": materialID})
		}
		siteBalanceID = cleanID(balance["id"])
		if submit && materialSnapshotQuantityExceeds(usedQuantity, materialBalanceAvailable(balance)) {
			return nil, validationError("Số lượng sử dụng vượt vật tư còn tại công trình", Record{
				"item_index":  itemIndex,
				"material_id": materialID,
				"available":   materialBalanceAvailable(balance),
				"quantity":    usedQuantity,
			})
		}
	}

	beforeEvidenceID, err := s.validateMaterialSnapshotEvidence(ctx, projectID, workItemID, req.BeforeEvidenceID, "before_evidence_id", itemIndex)
	if err != nil {
		return nil, err
	}
	afterEvidenceID, err := s.validateMaterialSnapshotEvidence(ctx, projectID, workItemID, req.AfterEvidenceID, "after_evidence_id", itemIndex)
	if err != nil {
		return nil, err
	}
	area := strings.TrimSpace(req.ConstructionArea)
	if len([]rune(area)) > 240 {
		return nil, validationError("Khu vực dòng snapshot không được vượt quá 240 ký tự", Record{"item_index": itemIndex})
	}
	condition := strings.TrimSpace(req.Condition)
	if condition == "" {
		condition = "usable"
	}
	if !statusIn(condition, "usable", "damaged", "mixed", "unknown") {
		return nil, validationError("Tình trạng vật tư không hợp lệ", Record{"item_index": itemIndex, "condition": condition})
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú dòng snapshot không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}

	return Record{
		"item_index":           itemIndex,
		"material_id":          materialID,
		"material_code":        materialCode,
		"material_name":        materialName,
		"unit":                 unit,
		"source":               source,
		"before_quantity":      beforeQuantity,
		"added_quantity":       addedQuantity,
		"after_quantity":       afterQuantity,
		"waste_quantity":       wasteQuantity,
		"used_quantity":        usedQuantity,
		"affects_site_balance": affectsSiteBalance,
		"site_balance_id":      siteBalanceID,
		"before_evidence_id":   beforeEvidenceID,
		"after_evidence_id":    afterEvidenceID,
		"construction_area":    area,
		"condition":            condition,
		"comment":              comment,
		"payload_json": Record{
			"source":                source,
			"affects_site_balance":  affectsSiteBalance,
			"computed_used_formula": "before_quantity + added_quantity - after_quantity - waste_quantity",
		},
	}, nil
}

func (s *Service) validateMaterialSnapshotEvidence(ctx context.Context, projectID, workItemID, evidenceID, field string, itemIndex int) (string, error) {
	evidenceID = cleanID(evidenceID)
	if evidenceID == "" {
		return "", nil
	}
	evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
	if err != nil {
		return "", validationError("Evidence snapshot vật tư không hợp lệ", Record{"item_index": itemIndex, field: evidenceID})
	}
	if cleanID(evidence["project_id"]) != projectID || cleanID(evidence["work_item_id"]) != workItemID || fmt.Sprint(evidence["status"]) == "rejected" {
		return "", validationError("Evidence snapshot vật tư không thuộc work item", Record{"item_index": itemIndex, field: evidenceID})
	}
	return evidenceID, nil
}

func materialSnapshotQuantity(value *float64, message string, itemIndex int) (float64, error) {
	if value == nil {
		return 0, nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 {
		return 0, validationError(message, Record{"item_index": itemIndex, "quantity": *value})
	}
	return materialSnapshotRound(*value), nil
}

func materialSnapshotRound(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func materialSnapshotQuantityExceeds(quantity, available float64) bool {
	return quantity > available+0.000001
}

func materialBalanceAvailable(balance Record) float64 {
	issuedMinusUsed := number(balance["issued_quantity"]) - number(balance["used_quantity"])
	surplus := number(balance["surplus_quantity"])
	available := math.Max(issuedMinusUsed, surplus)
	if available < 0 {
		return 0
	}
	return available
}

func materialSnapshotSummary(items []Record) Record {
	summary := Record{
		"total":                   len(items),
		"affects_site_balance":    0,
		"complete_evidence":       0,
		"with_waste_or_damage":    0,
		"site_balance_used_total": 0.0,
		"non_balance_item_count":  0,
		"source_site_balance":     0,
		"source_warehouse_issue":  0,
		"source_subcontractor":    0,
		"source_other":            0,
	}
	for _, item := range items {
		source := cleanOptionalText(item["source"])
		sourceKey := "source_" + source
		if _, ok := summary[sourceKey]; ok {
			summary[sourceKey] = int(number(summary[sourceKey])) + 1
		}
		if boolValue(item["affects_site_balance"]) {
			summary["affects_site_balance"] = int(number(summary["affects_site_balance"])) + 1
			summary["site_balance_used_total"] = materialSnapshotRound(number(summary["site_balance_used_total"]) + number(item["used_quantity"]))
		} else {
			summary["non_balance_item_count"] = int(number(summary["non_balance_item_count"])) + 1
		}
		if cleanID(item["before_evidence_id"]) != "" && cleanID(item["after_evidence_id"]) != "" {
			summary["complete_evidence"] = int(number(summary["complete_evidence"])) + 1
		}
		if number(item["waste_quantity"]) > 0 || cleanOptionalText(item["condition"]) == "damaged" {
			summary["with_waste_or_damage"] = int(number(summary["with_waste_or_damage"])) + 1
		}
	}
	return summary
}

func (s *Service) SurplusHandlings(ctx context.Context, workItemID, actorID string) (Record, error) {
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
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"warehouse.view", "operation.view", "operation.report", "warehouse.surplus_handle"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem xử lý vật tư dư", Record{"permissions": []string{"warehouse.view", "operation.view"}})
	}
	return s.surplusHandlingResponse(ctx, item)
}

func (s *Service) CreateSurplusHandling(ctx context.Context, workItemID string, req SurplusHandlingRequest, actorID, actorName string) (Record, error) {
	item, err := s.requireSurplusHandlingWorkItemAccess(ctx, workItemID, actorID, []string{"operation.report", "warehouse.surplus_handle"}, "Bạn không có quyền lưu xử lý vật tư dư")
	if err != nil {
		return nil, err
	}
	normalized, err := s.normalizeSurplusHandlingRequest(ctx, item, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveSurplusHandling(ctx, item, "", normalized, actorID, actorName)
}

func (s *Service) UpdateSurplusHandling(ctx context.Context, handlingID string, req SurplusHandlingRequest, actorID, actorName string) (Record, error) {
	handling, err := s.repo.GetSurplusHandling(ctx, handlingID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(handling["status"]) == "submitted" {
		return nil, validationError("Hồ sơ xử lý vật tư dư đã submit, không thể sửa", Record{"surplus_handling_id": handlingID})
	}
	item, err := s.requireSurplusHandlingWorkItemAccess(ctx, cleanID(handling["work_item_id"]), actorID, []string{"operation.report", "warehouse.surplus_handle"}, "Bạn không có quyền lưu xử lý vật tư dư")
	if err != nil {
		return nil, err
	}
	normalized, err := s.normalizeSurplusHandlingRequest(ctx, item, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveSurplusHandling(ctx, item, cleanID(handling["id"]), normalized, actorID, actorName)
}

func (s *Service) SubmitSurplusHandling(ctx context.Context, handlingID string, actorID, actorName string) (Record, error) {
	handling, err := s.repo.GetSurplusHandling(ctx, handlingID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(handling["status"]) == "submitted" {
		return nil, validationError("Hồ sơ xử lý vật tư dư đã submit", Record{"surplus_handling_id": handlingID})
	}
	item, err := s.requireSurplusHandlingWorkItemAccess(ctx, cleanID(handling["work_item_id"]), actorID, []string{"warehouse.surplus_handle"}, "Bạn không có quyền submit xử lý vật tư dư")
	if err != nil {
		return nil, err
	}
	req := SurplusHandlingRequest{
		SiteBalanceID:          cleanID(handling["site_balance_id"]),
		MaterialSnapshotID:     cleanID(handling["material_snapshot_id"]),
		MaterialSnapshotItemID: cleanID(handling["material_snapshot_item_id"]),
		HandlingType:           cleanOptionalText(handling["handling_type"]),
		Unit:                   cleanOptionalText(handling["unit"]),
		WarehouseID:            cleanID(handling["warehouse_id"]),
		CustodyLocation:        cleanOptionalText(handling["custody_location"]),
		Condition:              cleanOptionalText(handling["condition"]),
		EvidenceID:             cleanID(handling["evidence_id"]),
		Comment:                cleanOptionalText(handling["comment"]),
	}
	quantity := number(handling["quantity"])
	req.Quantity = &quantity
	if _, err := s.normalizeSurplusHandlingRequest(ctx, item, req, true); err != nil {
		return nil, err
	}
	return s.repo.SubmitSurplusHandling(ctx, cleanID(handling["id"]), actorID, actorName)
}

func (s *Service) requireSurplusHandlingWorkItemAccess(ctx context.Context, workItemID, actorID string, permissions []string, message string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thao tác work item này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	canUpdate, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, permissions)
	if err != nil {
		return nil, err
	}
	if !canUpdate {
		return nil, forbiddenError(message, Record{"permissions": permissions})
	}
	return item, nil
}

func (s *Service) surplusHandlingResponse(ctx context.Context, item Record) (Record, error) {
	projectID := cleanID(item["project_id"])
	balances, err := s.repo.ProjectMaterialBalances(ctx, projectID)
	if err != nil {
		return nil, err
	}
	warehouses, err := s.repo.ListActiveWarehouses(ctx)
	if err != nil {
		return nil, err
	}
	handlings, err := s.repo.ListSurplusHandlingsForWorkItem(ctx, cleanID(item["id"]))
	if err != nil {
		return nil, err
	}
	return Record{
		"available":         true,
		"work_item_id":      item["id"],
		"project_id":        projectID,
		"material_balances": balances,
		"warehouses":        warehouses,
		"handlings":         handlings,
	}, nil
}

func (s *Service) normalizeSurplusHandlingRequest(ctx context.Context, item Record, req SurplusHandlingRequest, submit bool) (Record, error) {
	projectID := cleanID(item["project_id"])
	workItemID := cleanID(item["id"])
	balances, err := s.repo.ProjectMaterialBalances(ctx, projectID)
	if err != nil {
		return nil, err
	}
	balanceByID := map[string]Record{}
	for _, balance := range balances {
		balanceByID[cleanID(balance["id"])] = balance
	}
	siteBalanceID := cleanID(req.SiteBalanceID)
	if siteBalanceID == "" {
		return nil, validationError("Số dư vật tư công trình là bắt buộc")
	}
	balance := balanceByID[siteBalanceID]
	if balance == nil {
		return nil, validationError("Số dư vật tư không thuộc công trình", Record{"site_balance_id": siteBalanceID})
	}
	surplusQuantity := number(balance["surplus_quantity"])
	if surplusQuantity <= 0 {
		return nil, validationError("Vật tư này không còn dư để xử lý", Record{"site_balance_id": siteBalanceID})
	}
	quantity, err := surplusHandlingQuantity(req.Quantity)
	if err != nil {
		return nil, err
	}
	if quantityExceedsAvailable(quantity, surplusQuantity) {
		return nil, validationError("Số lượng xử lý vượt vật tư dư", Record{"available": surplusQuantity, "quantity": quantity})
	}
	handlingType := strings.TrimSpace(req.HandlingType)
	if handlingType == "" {
		handlingType = "return_to_warehouse"
	}
	if !statusIn(handlingType, "return_to_warehouse", "return_to_subcontractor") {
		return nil, validationError("Loại xử lý vật tư dư không hợp lệ", Record{"handling_type": handlingType})
	}
	unit := strings.TrimSpace(req.Unit)
	if unit == "" {
		unit = cleanOptionalText(balance["unit"])
	}
	if unit == "" {
		return nil, validationError("Đơn vị xử lý vật tư dư là bắt buộc")
	}
	if len([]rune(unit)) > 40 {
		return nil, validationError("Đơn vị xử lý vật tư dư không được vượt quá 40 ký tự")
	}
	warehouseID := cleanID(req.WarehouseID)
	if warehouseID == "" {
		return nil, validationError("Kho/staging warehouse là bắt buộc")
	}
	if err := s.requireActiveWarehouse(ctx, warehouseID); err != nil {
		return nil, err
	}
	custodyLocation := strings.TrimSpace(req.CustodyLocation)
	if len([]rune(custodyLocation)) > 240 {
		return nil, validationError("Vị trí lưu/trả vật tư không được vượt quá 240 ký tự")
	}
	if submit && custodyLocation == "" {
		return nil, validationError("Vị trí lưu/trả vật tư là bắt buộc khi submit")
	}
	condition := strings.TrimSpace(req.Condition)
	if condition == "" {
		condition = "usable"
	}
	if !statusIn(condition, "usable", "damaged", "mixed", "unknown") {
		return nil, validationError("Tình trạng vật tư dư không hợp lệ", Record{"condition": condition})
	}
	evidenceID := cleanID(req.EvidenceID)
	if submit && evidenceID == "" {
		return nil, validationError("Evidence/ảnh vật tư dư là bắt buộc khi submit")
	}
	if evidenceID != "" {
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			return nil, validationError("Evidence xử lý vật tư dư không hợp lệ", Record{"evidence_id": evidenceID})
		}
		if cleanID(evidence["project_id"]) != projectID || cleanID(evidence["work_item_id"]) != workItemID || fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, validationError("Evidence xử lý vật tư dư không thuộc work item", Record{"evidence_id": evidenceID})
		}
	}
	materialSnapshotID, materialSnapshotItemID, err := s.validateSurplusHandlingSnapshotLink(ctx, workItemID, req.MaterialSnapshotID, req.MaterialSnapshotItemID)
	if err != nil {
		return nil, err
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú xử lý vật tư dư không được vượt quá 1000 ký tự")
	}
	return Record{
		"project_id":                projectID,
		"work_item_id":              workItemID,
		"site_balance_id":           siteBalanceID,
		"material_id":               cleanID(balance["material_id"]),
		"material_snapshot_id":      materialSnapshotID,
		"material_snapshot_item_id": materialSnapshotItemID,
		"handling_type":             handlingType,
		"quantity":                  materialSnapshotRound(quantity),
		"unit":                      unit,
		"warehouse_id":              warehouseID,
		"custody_location":          custodyLocation,
		"condition":                 condition,
		"evidence_id":               evidenceID,
		"comment":                   comment,
		"material_name":             cleanOptionalText(balance["material_name"]),
		"material_code":             cleanOptionalText(balance["code"]),
		"surplus_before_handling":   surplusQuantity,
	}, nil
}

func surplusHandlingQuantity(value *float64) (float64, error) {
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) || *value <= 0 {
		return 0, validationError("Số lượng xử lý vật tư dư phải lớn hơn 0")
	}
	return materialSnapshotRound(*value), nil
}

func (s *Service) requireActiveWarehouse(ctx context.Context, warehouseID string) error {
	warehouses, err := s.repo.ListActiveWarehouses(ctx)
	if err != nil {
		return err
	}
	for _, warehouse := range warehouses {
		if cleanID(warehouse["id"]) == warehouseID {
			return nil
		}
	}
	return validationError("Kho không tồn tại hoặc đã ngưng hoạt động", Record{"warehouse_id": warehouseID})
}

func (s *Service) validateSurplusHandlingSnapshotLink(ctx context.Context, workItemID, snapshotID, snapshotItemID string) (string, string, error) {
	snapshotID = cleanID(snapshotID)
	snapshotItemID = cleanID(snapshotItemID)
	if snapshotID == "" && snapshotItemID == "" {
		return "", "", nil
	}
	snapshot, err := s.repo.GetMaterialSnapshot(ctx, workItemID)
	if err != nil {
		return "", "", validationError("Snapshot vật tư liên kết không hợp lệ", Record{"material_snapshot_id": snapshotID})
	}
	if snapshotID != "" && cleanID(snapshot["id"]) != snapshotID {
		return "", "", validationError("Snapshot vật tư không thuộc work item", Record{"material_snapshot_id": snapshotID})
	}
	if snapshotID == "" {
		snapshotID = cleanID(snapshot["id"])
	}
	if snapshotItemID != "" {
		items, _ := snapshot["items"].([]Record)
		found := false
		for _, item := range items {
			if cleanID(item["id"]) == snapshotItemID {
				found = true
				break
			}
		}
		if !found {
			return "", "", validationError("Dòng snapshot vật tư không thuộc work item", Record{"material_snapshot_item_id": snapshotItemID})
		}
	}
	return snapshotID, snapshotItemID, nil
}

func (s *Service) ProjectIncomingAcceptances(ctx context.Context, projectID, actorID string) ([]Record, error) {
	projectID = cleanID(projectID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view", "warehouse.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu đầu vào", Record{"permissions": []string{"acceptance.view", "operation.view", "warehouse.view"}})
	}
	return s.repo.ListIncomingAcceptancesForProject(ctx, projectID)
}

func (s *Service) WorkItemIncomingAcceptances(ctx context.Context, workItemID, actorID string) (Record, error) {
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
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view", "warehouse.view", "operation.report", "acceptance.create"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu đầu vào", Record{"permissions": []string{"acceptance.view", "operation.view", "warehouse.view"}})
	}
	return s.incomingAcceptanceResponse(ctx, item)
}

func (s *Service) GetIncomingAcceptance(ctx context.Context, acceptanceID, actorID string) (Record, error) {
	record, err := s.repo.GetIncomingAcceptance(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(record["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view", "warehouse.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu đầu vào", Record{"permissions": []string{"acceptance.view", "operation.view", "warehouse.view"}})
	}
	workItemID := cleanID(record["work_item_id"])
	if workItemID != "" {
		canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
		if err != nil {
			return nil, err
		}
		if !canView {
			return nil, forbiddenError("Bạn không có quyền xem work item này")
		}
	}
	return record, nil
}

func (s *Service) CreateIncomingAcceptance(ctx context.Context, projectID string, req IncomingAcceptanceRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	item, err := s.requireIncomingAcceptanceAccess(ctx, projectID, req.WorkItemID, actorID, []string{"acceptance.create", "operation.report"}, "Bạn không có quyền lưu nghiệm thu đầu vào")
	if err != nil {
		return nil, err
	}
	header, items, err := s.normalizeIncomingAcceptanceRequest(ctx, projectID, item, nil, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveIncomingAcceptance(ctx, "", header, items, actorID, actorName)
}

func (s *Service) UpdateIncomingAcceptance(ctx context.Context, acceptanceID string, req IncomingAcceptanceRequest, actorID, actorName string) (Record, error) {
	existing, err := s.repo.GetIncomingAcceptance(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(existing["status"]) == "submitted" {
		return nil, validationError("Hồ sơ nghiệm thu đầu vào đã submit, không thể sửa", Record{"incoming_acceptance_id": acceptanceID})
	}
	projectID := cleanID(existing["project_id"])
	workItemID := req.WorkItemID
	if cleanID(workItemID) == "" {
		workItemID = cleanID(existing["work_item_id"])
	}
	item, err := s.requireIncomingAcceptanceAccess(ctx, projectID, workItemID, actorID, []string{"acceptance.create", "operation.report"}, "Bạn không có quyền lưu nghiệm thu đầu vào")
	if err != nil {
		return nil, err
	}
	header, items, err := s.normalizeIncomingAcceptanceRequest(ctx, projectID, item, existing, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveIncomingAcceptance(ctx, cleanID(existing["id"]), header, items, actorID, actorName)
}

func (s *Service) SubmitIncomingAcceptance(ctx context.Context, acceptanceID string, actorID, actorName string) (Record, error) {
	existing, err := s.repo.GetIncomingAcceptance(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(existing["status"]) == "submitted" {
		return nil, validationError("Hồ sơ nghiệm thu đầu vào đã submit", Record{"incoming_acceptance_id": acceptanceID})
	}
	projectID := cleanID(existing["project_id"])
	workItemID := cleanID(existing["work_item_id"])
	item, err := s.requireIncomingAcceptanceAccess(ctx, projectID, workItemID, actorID, []string{"acceptance.approve", "acceptance.create", "operation.report"}, "Bạn không có quyền submit nghiệm thu đầu vào")
	if err != nil {
		return nil, err
	}
	req := incomingAcceptanceRequestFromRecord(existing)
	header, _, err := s.normalizeIncomingAcceptanceRequest(ctx, projectID, item, existing, req, true)
	if err != nil {
		return nil, err
	}
	return s.repo.SubmitIncomingAcceptance(ctx, cleanID(existing["id"]), cleanOptionalText(header["result"]), actorID, actorName)
}

func (s *Service) requireIncomingAcceptanceAccess(ctx context.Context, projectID, workItemID, actorID string, permissions []string, message string) (Record, error) {
	projectID = cleanID(projectID)
	workItemID = cleanID(workItemID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, permissions)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError(message, Record{"permissions": permissions})
	}
	if workItemID == "" {
		return nil, nil
	}
	canAct, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canAct {
		return nil, forbiddenError("Bạn không có quyền thao tác work item này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if cleanID(item["project_id"]) != projectID {
		return nil, validationError("Work item không thuộc công trình nghiệm thu đầu vào", Record{"work_item_id": workItemID, "project_id": projectID})
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) incomingAcceptanceResponse(ctx context.Context, item Record) (Record, error) {
	projectID := cleanID(item["project_id"])
	materials, err := s.repo.ListActiveMaterials(ctx)
	if err != nil {
		return nil, err
	}
	warehouses, err := s.repo.ListActiveWarehouses(ctx)
	if err != nil {
		return nil, err
	}
	acceptances, err := s.repo.ListIncomingAcceptancesForWorkItem(ctx, cleanID(item["id"]))
	if err != nil {
		return nil, err
	}
	procurementRequests, err := s.repo.ListProjectProcurementRequests(ctx, projectID)
	if err != nil {
		return nil, err
	}
	procurementItems, err := s.repo.ListProjectProcurementRequestItems(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return Record{
		"available":                 true,
		"work_item_id":              item["id"],
		"project_id":                projectID,
		"records":                   acceptances,
		"materials":                 materials,
		"warehouses":                warehouses,
		"procurement_requests":      procurementRequests,
		"procurement_request_items": procurementItems,
	}, nil
}

func (s *Service) normalizeIncomingAcceptanceRequest(ctx context.Context, projectID string, item Record, existing Record, req IncomingAcceptanceRequest, submit bool) (Record, []Record, error) {
	projectID = cleanID(projectID)
	workItemID := cleanID(req.WorkItemID)
	if workItemID == "" && item != nil {
		workItemID = cleanID(item["id"])
	}
	if workItemID == "" && existing != nil {
		workItemID = cleanID(existing["work_item_id"])
	}
	if workItemID != "" {
		if item == nil || cleanID(item["id"]) != workItemID {
			workItem, err := s.repo.GetWorkItemDetail(ctx, workItemID)
			if err != nil {
				return nil, nil, err
			}
			item = workItem
		}
		if cleanID(item["project_id"]) != projectID {
			return nil, nil, validationError("Work item không thuộc công trình nghiệm thu đầu vào", Record{"work_item_id": workItemID, "project_id": projectID})
		}
	}

	sourceType := strings.TrimSpace(req.SourceType)
	procurementRequestID := cleanID(req.ProcurementRequestID)
	if sourceType == "" && procurementRequestID != "" {
		sourceType = "procurement"
	}
	if sourceType == "" {
		sourceType = "supplier"
	}
	if !statusIn(sourceType, "supplier", "subcontractor", "procurement", "other") {
		return nil, nil, validationError("Nguồn nghiệm thu đầu vào không hợp lệ", Record{"source_type": sourceType})
	}
	supplierName := strings.TrimSpace(req.SupplierName)
	subcontractorName := strings.TrimSpace(req.SubcontractorName)
	deliveryReference := strings.TrimSpace(req.DeliveryReference)
	for field, value := range map[string]string{
		"supplier_name":      supplierName,
		"subcontractor_name": subcontractorName,
		"delivery_reference": deliveryReference,
	} {
		if len([]rune(value)) > 240 {
			return nil, nil, validationError("Thông tin nghiệm thu đầu vào không được vượt quá 240 ký tự", Record{"field": field})
		}
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 2000 {
		return nil, nil, validationError("Ghi chú nghiệm thu đầu vào không được vượt quá 2000 ký tự")
	}
	receivedAt := strings.TrimSpace(req.ReceivedAt)
	if receivedAt != "" {
		if _, err := time.Parse("2006-01-02", receivedAt); err != nil {
			return nil, nil, validationError("Ngày nhận vật tư không hợp lệ", Record{"received_at": receivedAt})
		}
	} else if submit {
		return nil, nil, validationError("Ngày nhận vật tư là bắt buộc khi submit")
	}
	warehouseID := cleanID(req.WarehouseID)
	if warehouseID != "" {
		if err := s.requireActiveWarehouse(ctx, warehouseID); err != nil {
			return nil, nil, err
		}
	}

	procurementRequests, err := s.repo.ListProjectProcurementRequests(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	if procurementRequestID != "" {
		found := false
		for _, request := range procurementRequests {
			if cleanID(request["id"]) == procurementRequestID {
				found = true
				break
			}
		}
		if !found {
			return nil, nil, validationError("Yêu cầu mua không thuộc công trình nghiệm thu đầu vào", Record{"procurement_request_id": procurementRequestID})
		}
	}
	materials, err := s.repo.ListActiveMaterials(ctx)
	if err != nil {
		return nil, nil, err
	}
	materialByID := map[string]Record{}
	for _, material := range materials {
		materialByID[cleanID(material["id"])] = material
	}
	procurementItems, err := s.repo.ListProjectProcurementRequestItems(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	procurementItemByID := map[string]Record{}
	for _, item := range procurementItems {
		procurementItemByID[cleanID(item["id"])] = item
	}

	if submit && len(req.Items) == 0 {
		return nil, nil, validationError("Cần ít nhất một dòng vật tư để submit nghiệm thu đầu vào")
	}
	seen := map[int]bool{}
	normalized := make([]Record, 0, len(req.Items))
	for index, itemReq := range req.Items {
		itemIndex := itemReq.ItemIndex
		if itemIndex < 0 {
			itemIndex = index
		}
		if itemIndex < 0 {
			return nil, nil, validationError("Dòng nghiệm thu đầu vào không hợp lệ", Record{"item_index": itemReq.ItemIndex})
		}
		if seen[itemIndex] {
			return nil, nil, validationError("Dòng nghiệm thu đầu vào bị trùng", Record{"item_index": itemIndex})
		}
		seen[itemIndex] = true
		row, err := s.normalizeIncomingAcceptanceItem(ctx, projectID, workItemID, procurementRequestID, itemIndex, materialByID, procurementItemByID, itemReq, submit)
		if err != nil {
			return nil, nil, err
		}
		normalized = append(normalized, row)
	}
	summary := incomingAcceptanceSummary(normalized)
	result := strings.TrimSpace(req.Result)
	if result == "" && submit {
		result = incomingAcceptanceResultFromSummary(summary)
	}
	if result != "" && !statusIn(result, "pass", "conditional_pass", "fail") {
		return nil, nil, validationError("Kết quả nghiệm thu đầu vào không hợp lệ", Record{"result": result})
	}
	if submit {
		if result == "" {
			return nil, nil, validationError("Kết quả nghiệm thu đầu vào là bắt buộc khi submit")
		}
		if err := validateIncomingAcceptanceResult(result, summary); err != nil {
			return nil, nil, err
		}
	}
	header := Record{
		"project_id":              projectID,
		"work_item_id":            workItemID,
		"procurement_request_id":  procurementRequestID,
		"warehouse_id":            warehouseID,
		"source_type":             sourceType,
		"supplier_name":           supplierName,
		"subcontractor_name":      subcontractorName,
		"delivery_reference":      deliveryReference,
		"received_at":             receivedAt,
		"result":                  result,
		"comment":                 comment,
		"summary_json":            summary,
		"incoming_material_count": summary["total"],
	}
	return header, normalized, nil
}

func (s *Service) normalizeIncomingAcceptanceItem(ctx context.Context, projectID, workItemID, procurementRequestID string, itemIndex int, materialByID, procurementItemByID map[string]Record, req IncomingAcceptanceItemRequest, submit bool) (Record, error) {
	materialID := cleanID(req.MaterialID)
	procurementRequestItemID := cleanID(req.ProcurementRequestItemID)
	var procurementItem Record
	if procurementRequestItemID != "" {
		var ok bool
		procurementItem, ok = procurementItemByID[procurementRequestItemID]
		if !ok {
			return nil, validationError("Dòng yêu cầu mua không thuộc công trình nghiệm thu đầu vào", Record{"item_index": itemIndex, "procurement_request_item_id": procurementRequestItemID})
		}
		itemRequestID := cleanID(procurementItem["request_id"])
		if procurementRequestID != "" && itemRequestID != procurementRequestID {
			return nil, validationError("Dòng yêu cầu mua không thuộc yêu cầu mua đã chọn", Record{"item_index": itemIndex, "procurement_request_item_id": procurementRequestItemID})
		}
		procurementMaterialID := cleanID(procurementItem["material_id"])
		if materialID == "" {
			materialID = procurementMaterialID
		}
		if materialID != "" && procurementMaterialID != "" && materialID != procurementMaterialID {
			return nil, validationError("Vật tư không khớp dòng yêu cầu mua", Record{"item_index": itemIndex, "material_id": materialID, "procurement_request_item_id": procurementRequestItemID})
		}
	}

	var material Record
	if materialID != "" {
		var ok bool
		material, ok = materialByID[materialID]
		if !ok {
			return nil, validationError("Vật tư không tồn tại hoặc đã ngưng hoạt động", Record{"item_index": itemIndex, "material_id": materialID})
		}
	}
	materialName := strings.TrimSpace(req.MaterialName)
	materialCode := ""
	if material != nil {
		materialName = cleanOptionalText(material["name"])
		materialCode = cleanOptionalText(material["code"])
	} else if procurementItem != nil {
		materialName = cleanOptionalText(procurementItem["material_name"])
	}
	if materialName == "" {
		return nil, validationError("Tên vật tư nghiệm thu đầu vào là bắt buộc", Record{"item_index": itemIndex})
	}
	if len([]rune(materialName)) > 240 {
		return nil, validationError("Tên vật tư nghiệm thu đầu vào không được vượt quá 240 ký tự", Record{"item_index": itemIndex})
	}

	unit := strings.TrimSpace(req.Unit)
	if unit == "" && material != nil {
		unit = cleanOptionalText(material["unit"])
	}
	if unit == "" && procurementItem != nil {
		unit = cleanOptionalText(procurementItem["unit"])
	}
	if unit == "" {
		return nil, validationError("Đơn vị nghiệm thu đầu vào là bắt buộc", Record{"item_index": itemIndex})
	}
	if len([]rune(unit)) > 40 {
		return nil, validationError("Đơn vị nghiệm thu đầu vào không được vượt quá 40 ký tự", Record{"item_index": itemIndex})
	}

	deliveredQuantity, err := incomingAcceptanceQuantity(req.DeliveredQuantity, "Số lượng giao không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	acceptedQuantity, err := incomingAcceptanceQuantity(req.AcceptedQuantity, "Số lượng nhận không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	rejectedQuantity, err := incomingAcceptanceQuantity(req.RejectedQuantity, "Số lượng từ chối không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	if submit && deliveredQuantity <= 0 {
		return nil, validationError("Số lượng giao phải lớn hơn 0 khi submit", Record{"item_index": itemIndex})
	}
	if materialSnapshotQuantityExceeds(acceptedQuantity+rejectedQuantity, deliveredQuantity) {
		return nil, validationError("Số lượng nhận và từ chối vượt số lượng giao", Record{"item_index": itemIndex, "delivered_quantity": deliveredQuantity})
	}

	condition := strings.TrimSpace(req.Condition)
	if condition == "" {
		condition = "usable"
	}
	if !statusIn(condition, "usable", "damaged", "mixed", "unknown") {
		return nil, validationError("Tình trạng vật tư nghiệm thu đầu vào không hợp lệ", Record{"item_index": itemIndex, "condition": condition})
	}
	decision := strings.TrimSpace(req.Decision)
	if decision == "" {
		decision = incomingAcceptanceDefaultDecision(deliveredQuantity, acceptedQuantity, rejectedQuantity)
	}
	if !statusIn(decision, "pending", "accepted", "conditional_acceptance", "rejected") {
		return nil, validationError("Quyết định dòng nghiệm thu đầu vào không hợp lệ", Record{"item_index": itemIndex, "decision": decision})
	}
	if submit {
		if decision == "pending" {
			return nil, validationError("Dòng nghiệm thu đầu vào chưa có quyết định", Record{"item_index": itemIndex})
		}
		if err := validateIncomingAcceptanceDecision(decision, deliveredQuantity, acceptedQuantity, rejectedQuantity); err != nil {
			return nil, validationError(err.Error(), Record{"item_index": itemIndex, "decision": decision})
		}
	}

	defectNote := strings.TrimSpace(req.DefectNote)
	if len([]rune(defectNote)) > 1000 {
		return nil, validationError("Mô tả lỗi nghiệm thu đầu vào không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú dòng nghiệm thu đầu vào không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	evidenceID := cleanID(req.EvidenceID)
	evidenceRequired := decision != "accepted" || condition != "usable" || rejectedQuantity > 0
	if submit && evidenceRequired && evidenceID == "" {
		return nil, validationError("Dòng nghiệm thu lỗi/conditional cần evidence", Record{"item_index": itemIndex})
	}
	if submit && evidenceRequired && defectNote == "" {
		return nil, validationError("Dòng nghiệm thu lỗi/conditional cần mô tả lỗi", Record{"item_index": itemIndex})
	}
	if evidenceID != "" {
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			return nil, validationError("Evidence nghiệm thu đầu vào không hợp lệ", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if cleanID(evidence["project_id"]) != projectID || fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, validationError("Evidence nghiệm thu đầu vào không thuộc công trình", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if workItemID != "" && cleanID(evidence["work_item_id"]) != workItemID {
			return nil, validationError("Evidence nghiệm thu đầu vào không thuộc work item", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
	}

	return Record{
		"item_index":                  itemIndex,
		"material_id":                 materialID,
		"material_code":               materialCode,
		"material_name":               materialName,
		"procurement_request_item_id": procurementRequestItemID,
		"delivered_quantity":          deliveredQuantity,
		"accepted_quantity":           acceptedQuantity,
		"rejected_quantity":           rejectedQuantity,
		"unit":                        unit,
		"condition":                   condition,
		"defect_note":                 defectNote,
		"evidence_id":                 evidenceID,
		"decision":                    decision,
		"comment":                     comment,
		"payload_json": Record{
			"procurement_request_item_id": procurementRequestItemID,
			"accepted_plus_rejected":      materialSnapshotRound(acceptedQuantity + rejectedQuantity),
		},
	}, nil
}

func incomingAcceptanceRequestFromRecord(record Record) IncomingAcceptanceRequest {
	req := IncomingAcceptanceRequest{
		WorkItemID:           cleanID(record["work_item_id"]),
		ProcurementRequestID: cleanID(record["procurement_request_id"]),
		WarehouseID:          cleanID(record["warehouse_id"]),
		SourceType:           cleanOptionalText(record["source_type"]),
		SupplierName:         cleanOptionalText(record["supplier_name"]),
		SubcontractorName:    cleanOptionalText(record["subcontractor_name"]),
		DeliveryReference:    cleanOptionalText(record["delivery_reference"]),
		ReceivedAt:           dateOnly(record["received_at"]),
		Result:               cleanOptionalText(record["result"]),
		Comment:              cleanOptionalText(record["comment"]),
		Items:                []IncomingAcceptanceItemRequest{},
	}
	if items, ok := record["items"].([]Record); ok {
		req.Items = make([]IncomingAcceptanceItemRequest, 0, len(items))
		for _, item := range items {
			delivered := number(item["delivered_quantity"])
			accepted := number(item["accepted_quantity"])
			rejected := number(item["rejected_quantity"])
			req.Items = append(req.Items, IncomingAcceptanceItemRequest{
				ItemIndex:                int(number(item["item_index"])),
				MaterialID:               cleanID(item["material_id"]),
				MaterialName:             cleanOptionalText(item["material_name"]),
				ProcurementRequestItemID: cleanID(item["procurement_request_item_id"]),
				DeliveredQuantity:        &delivered,
				AcceptedQuantity:         &accepted,
				RejectedQuantity:         &rejected,
				Unit:                     cleanOptionalText(item["unit"]),
				Condition:                cleanOptionalText(item["condition"]),
				DefectNote:               cleanOptionalText(item["defect_note"]),
				EvidenceID:               cleanID(item["evidence_id"]),
				Decision:                 cleanOptionalText(item["decision"]),
				Comment:                  cleanOptionalText(item["comment"]),
			})
		}
	}
	return req
}

func incomingAcceptanceQuantity(value *float64, message string, itemIndex int) (float64, error) {
	if value == nil {
		return 0, nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 {
		return 0, validationError(message, Record{"item_index": itemIndex, "quantity": *value})
	}
	return materialSnapshotRound(*value), nil
}

func incomingAcceptanceDefaultDecision(delivered, accepted, rejected float64) string {
	if delivered <= 0 && accepted <= 0 && rejected <= 0 {
		return "pending"
	}
	if rejected > 0 && accepted > 0 {
		return "conditional_acceptance"
	}
	if rejected > 0 {
		return "rejected"
	}
	if accepted > 0 && !materialSnapshotQuantityExceeds(delivered, accepted) && !materialSnapshotQuantityExceeds(accepted, delivered) {
		return "accepted"
	}
	return "pending"
}

func validateIncomingAcceptanceDecision(decision string, delivered, accepted, rejected float64) error {
	switch decision {
	case "accepted":
		if rejected > 0 || accepted <= 0 || materialSnapshotQuantityExceeds(accepted, delivered) || materialSnapshotQuantityExceeds(delivered, accepted) {
			return errors.New("Dòng accepted phải nhận đủ và không có số lượng từ chối")
		}
	case "conditional_acceptance":
		if accepted <= 0 || rejected <= 0 {
			return errors.New("Dòng conditional cần có cả số lượng nhận và từ chối")
		}
	case "rejected":
		if rejected <= 0 || accepted > 0 {
			return errors.New("Dòng rejected phải có số lượng từ chối và không có số lượng nhận")
		}
	}
	return nil
}

func incomingAcceptanceSummary(items []Record) Record {
	summary := Record{
		"total":                len(items),
		"accepted":             0,
		"conditional":          0,
		"rejected":             0,
		"pending":              0,
		"defect_count":         0,
		"evidence_count":       0,
		"delivered_quantity":   0.0,
		"accepted_quantity":    0.0,
		"rejected_quantity":    0.0,
		"with_procurement_ref": 0,
	}
	for _, item := range items {
		switch cleanOptionalText(item["decision"]) {
		case "accepted":
			summary["accepted"] = int(number(summary["accepted"])) + 1
		case "conditional_acceptance":
			summary["conditional"] = int(number(summary["conditional"])) + 1
		case "rejected":
			summary["rejected"] = int(number(summary["rejected"])) + 1
		default:
			summary["pending"] = int(number(summary["pending"])) + 1
		}
		if cleanOptionalText(item["defect_note"]) != "" || number(item["rejected_quantity"]) > 0 || cleanOptionalText(item["condition"]) != "usable" {
			summary["defect_count"] = int(number(summary["defect_count"])) + 1
		}
		if cleanID(item["evidence_id"]) != "" {
			summary["evidence_count"] = int(number(summary["evidence_count"])) + 1
		}
		if cleanID(item["procurement_request_item_id"]) != "" {
			summary["with_procurement_ref"] = int(number(summary["with_procurement_ref"])) + 1
		}
		summary["delivered_quantity"] = materialSnapshotRound(number(summary["delivered_quantity"]) + number(item["delivered_quantity"]))
		summary["accepted_quantity"] = materialSnapshotRound(number(summary["accepted_quantity"]) + number(item["accepted_quantity"]))
		summary["rejected_quantity"] = materialSnapshotRound(number(summary["rejected_quantity"]) + number(item["rejected_quantity"]))
	}
	return summary
}

func incomingAcceptanceResultFromSummary(summary Record) string {
	total := int(number(summary["total"]))
	accepted := int(number(summary["accepted"]))
	conditional := int(number(summary["conditional"]))
	rejected := int(number(summary["rejected"]))
	if total == 0 {
		return ""
	}
	if accepted == total {
		return "pass"
	}
	if rejected == total {
		return "fail"
	}
	if conditional > 0 || rejected > 0 {
		return "conditional_pass"
	}
	return ""
}

func validateIncomingAcceptanceResult(result string, summary Record) error {
	total := int(number(summary["total"]))
	accepted := int(number(summary["accepted"]))
	conditional := int(number(summary["conditional"]))
	rejected := int(number(summary["rejected"]))
	pending := int(number(summary["pending"]))
	if total == 0 {
		return validationError("Cần ít nhất một dòng vật tư để submit nghiệm thu đầu vào")
	}
	if pending > 0 {
		return validationError("Không thể submit nghiệm thu đầu vào khi còn dòng pending", Record{"pending": pending})
	}
	switch result {
	case "pass":
		if accepted != total {
			return validationError("Kết quả pass chỉ hợp lệ khi mọi dòng đều accepted")
		}
	case "conditional_pass":
		if conditional+rejected == 0 {
			return validationError("Kết quả conditional pass cần có dòng conditional hoặc rejected")
		}
	case "fail":
		if rejected == 0 || accepted > 0 || conditional > 0 {
			return validationError("Kết quả fail chỉ hợp lệ khi có dòng rejected và không có dòng accepted/conditional")
		}
	}
	return nil
}

func dateOnly(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	if len(text) >= 10 {
		return text[:10]
	}
	return text
}

func (s *Service) SignWorkflowForm(ctx context.Context, instanceID string, req WorkflowFormSignRequest, actorID, actorName string) (Record, error) {
	if !validActivityUUID(instanceID) {
		return nil, validationError("workflow_form_instance_id không hợp lệ", Record{"workflow_form_instance_id": instanceID})
	}
	purpose := strings.TrimSpace(req.Purpose)
	if purpose == "" {
		purpose = "internal_ack"
	}
	if !validWorkflowSignaturePurpose(purpose) {
		return nil, validationError("Mục đích ký workflow form không hợp lệ", Record{"purpose": purpose})
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú chữ ký không được vượt quá 1000 ký tự")
	}
	form, err := s.repo.GetWorkflowFormInstanceDetail(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Workflow form phải được submit trước khi ký nội bộ", Record{"status": instance["status"]})
	}
	workItemID := cleanID(instance["work_item_id"])
	if workItemID == "" {
		return nil, validationError("Workflow form chưa liên kết work item", Record{"workflow_form_instance_id": instanceID})
	}
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền ký workflow form này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	imageBytes, imageMIME, err := parseSignatureImageDataURL(req.SignatureImageDataURL)
	if err != nil {
		return nil, err
	}
	strokes, err := validateSignatureStrokes(req.SignatureStrokes)
	if err != nil {
		return nil, err
	}
	storageKey := path.Join("workflow-signatures", cleanID(instance["project_id"]), instanceID, uuid.NewString()+".png")
	visualFile, storedPath, err := s.storeGeneratedFile(storageKey, imageBytes, imageMIME)
	if err != nil {
		return nil, err
	}
	signedForm, err := s.repo.SignWorkflowForm(ctx, instanceID, purpose, comment, visualFile, strokes, actorID, actorName)
	if err != nil {
		_ = os.Remove(storedPath)
		return nil, err
	}
	return signedForm, nil
}

func validWorkflowSignaturePurpose(purpose string) bool {
	return statusIn(purpose, "internal_ack", "prepared_by", "reviewed_by", "approved_by", "received_by")
}

func (s *Service) GenerateWorkflowFormPDFExport(ctx context.Context, instanceID, actorID, actorName string) (Record, error) {
	if !validActivityUUID(instanceID) {
		return nil, validationError("workflow_form_instance_id không hợp lệ", Record{"workflow_form_instance_id": instanceID})
	}
	form, err := s.repo.GetWorkflowFormInstanceDetail(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Workflow form phải được submit trước khi export PDF", Record{"status": instance["status"]})
	}
	workItemID := cleanID(instance["work_item_id"])
	projectID := cleanID(instance["project_id"])
	if workItemID == "" || projectID == "" {
		return nil, validationError("Workflow form chưa liên kết work item", Record{"workflow_form_instance_id": instanceID})
	}
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền export PDF workflow form này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	pdfBytes, payloadHash, err := GenerateWorkflowFormPDF(form)
	if err != nil {
		return nil, err
	}
	revisionNo := int(number(instance["current_revision"]))
	fileName := workflowFormPDFFileName(form, revisionNo)
	storageKey := path.Join("workflow-form-pdfs", projectID, instanceID, uuid.NewString()+".pdf")
	pdfFile, storedPath, err := s.storeGeneratedFile(storageKey, pdfBytes, "application/pdf")
	if err != nil {
		return nil, err
	}
	exportedForm, err := s.repo.CreateWorkflowFormPDFExport(ctx, instanceID, fileName, fmt.Sprint(instance["title"]), pdfFile, payloadHash, actorID, actorName)
	if err != nil {
		_ = os.Remove(storedPath)
		return nil, err
	}
	return exportedForm, nil
}

func (s *Service) WorkflowFormSignatureVisual(ctx context.Context, signatureID, actorID string) (EvidenceFileResult, error) {
	signatureID = cleanID(signatureID)
	if !validActivityUUID(signatureID) {
		return EvidenceFileResult{}, validationError("signature_id không hợp lệ", Record{"signature_id": signatureID})
	}
	signature, err := s.repo.GetWorkflowFormSignatureDetail(ctx, signatureID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	allowed, err := s.repo.CanViewWorkItem(ctx, cleanID(signature["work_item_id"]), actorID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	if !allowed {
		return EvidenceFileResult{}, forbiddenError("Bạn không có quyền xem chữ ký này")
	}
	return s.storedFileResult(signature, "visual_storage_key", "visual_mime_type", signatureVisualFilename(signature))
}

func (s *Service) WorkflowFormPDFExportFile(ctx context.Context, exportID, actorID string) (EvidenceFileResult, error) {
	exportID = cleanID(exportID)
	if !validActivityUUID(exportID) {
		return EvidenceFileResult{}, validationError("pdf_export_id không hợp lệ", Record{"pdf_export_id": exportID})
	}
	export, err := s.repo.GetWorkflowFormPDFExportDetail(ctx, exportID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	allowed, err := s.repo.CanViewWorkItem(ctx, cleanID(export["work_item_id"]), actorID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	if !allowed {
		return EvidenceFileResult{}, forbiddenError("Bạn không có quyền xem PDF workflow form này")
	}
	return s.storedFileResult(export, "storage_key", "mime_type", fmt.Sprint(export["file_name"]))
}

func (s *Service) saveWorkflowForm(ctx context.Context, instanceID string, req WorkflowFormSaveRequest, submit bool, actorID, actorName string) (Record, error) {
	if !validActivityUUID(instanceID) {
		return nil, validationError("workflow_form_instance_id không hợp lệ", Record{"workflow_form_instance_id": instanceID})
	}
	form, err := s.repo.GetWorkflowFormInstanceDetail(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	instance, _ := form["instance"].(Record)
	workItemID := cleanID(instance["work_item_id"])
	if workItemID == "" {
		return nil, validationError("Workflow form chưa liên kết work item", Record{"workflow_form_instance_id": instanceID})
	}
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thao tác workflow form này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	if submit && statusIn(fmt.Sprint(instance["status"]), "submitted", "cancelled") {
		return nil, validationError("Trạng thái workflow form không cho phép submit", Record{"status": instance["status"]})
	}
	if !submit && statusIn(fmt.Sprint(instance["status"]), "submitted", "cancelled") {
		return nil, validationError("Trạng thái workflow form không cho phép lưu nháp", Record{"status": instance["status"]})
	}
	fields, _ := form["fields"].([]Record)
	normalized, err := s.validateWorkflowFormAnswers(ctx, instance, fields, req.Answers, submit)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveWorkflowForm(ctx, instanceID, normalized, strings.TrimSpace(req.Comment), submit, actorID, actorName)
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
	if normalized.StorageKey != "" {
		return nil, validationError("Evidence storage nội bộ chỉ được tạo qua endpoint upload")
	}
	return s.repo.AddWorkItemEvidence(ctx, item, normalized, actorID, actorName)
}

func (s *Service) AddWorkItemEvidenceUpload(ctx context.Context, workItemID string, req WorkItemEvidenceRequest, fileHeader *multipart.FileHeader, actorID, actorName string) (Record, error) {
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
	storedPath, normalized, err := s.storeEvidenceUpload(item, req, fileHeader)
	if err != nil {
		return nil, err
	}
	evidence, err := s.repo.AddWorkItemEvidence(ctx, item, normalized, actorID, actorName)
	if err != nil {
		_ = os.Remove(storedPath)
		return nil, err
	}
	return evidence, nil
}

type EvidenceFileResult struct {
	Path     string
	MIMEType string
	Filename string
}

func (s *Service) EvidenceFile(ctx context.Context, evidenceID, actorID string) (EvidenceFileResult, error) {
	evidenceID = cleanID(evidenceID)
	if !validActivityUUID(evidenceID) {
		return EvidenceFileResult{}, validationError("evidence_id không hợp lệ", Record{"evidence_id": evidenceID})
	}
	evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	allowed, err := s.repo.CanViewWorkItem(ctx, cleanID(evidence["work_item_id"]), actorID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	if !allowed {
		return EvidenceFileResult{}, forbiddenError("Bạn không có quyền xem file evidence này")
	}
	if fmt.Sprint(evidence["storage_provider"]) != "local" || cleanID(evidence["storage_key"]) == "" {
		return EvidenceFileResult{}, validationError("Evidence này chưa có file nội bộ được bảo vệ", Record{"evidence_id": evidenceID})
	}
	root, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	filePath, err := safeStoragePath(root, fmt.Sprint(evidence["storage_key"]))
	if err != nil {
		return EvidenceFileResult{}, err
	}
	info, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return EvidenceFileResult{}, validationError("File evidence không còn tồn tại trong storage", Record{"evidence_id": evidenceID})
		}
		return EvidenceFileResult{}, err
	}
	if info.IsDir() {
		return EvidenceFileResult{}, validationError("Đường dẫn evidence không hợp lệ", Record{"evidence_id": evidenceID})
	}
	mimeType := strings.TrimSpace(fmt.Sprint(evidence["mime_type"]))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return EvidenceFileResult{
		Path:     filePath,
		MIMEType: mimeType,
		Filename: evidenceDownloadName(evidence),
	}, nil
}

func (s *Service) storeEvidenceUpload(item Record, req WorkItemEvidenceRequest, fileHeader *multipart.FileHeader) (string, WorkItemEvidenceRequest, error) {
	if fileHeader == nil {
		return "", req, validationError("File evidence là bắt buộc")
	}
	if fileHeader.Size <= 0 {
		return "", req, validationError("File evidence không được rỗng")
	}
	if fileHeader.Size > s.maxUploadBytes {
		return "", req, validationError("File evidence vượt quá dung lượng cho phép", Record{"max_bytes": s.maxUploadBytes})
	}
	source, err := fileHeader.Open()
	if err != nil {
		return "", req, err
	}
	defer source.Close()

	header := make([]byte, 512)
	n, readErr := io.ReadFull(source, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return "", req, readErr
	}
	detectedMIME := http.DetectContentType(header[:n])
	spec, ok := allowedEvidenceUploadMIMEs[detectedMIME]
	if !ok {
		return "", req, validationError("Định dạng evidence không được hỗ trợ", Record{"mime_type": detectedMIME})
	}
	requestedType := strings.TrimSpace(req.EvidenceType)
	if requestedType != "" && requestedType != spec.EvidenceType {
		return "", req, validationError("Loại evidence không khớp với định dạng file", Record{"evidence_type": requestedType, "mime_type": detectedMIME})
	}
	if requestedType == "" {
		req.EvidenceType = spec.EvidenceType
	}
	originalName := cleanUploadFilename(fileHeader.Filename)
	if strings.TrimSpace(req.Name) == "" {
		req.Name = originalName
	}
	if strings.TrimSpace(req.Title) == "" {
		req.Title = req.Name
	}
	req.MIMEType = detectedMIME
	req.FileSize = fileHeader.Size
	req.StorageProvider = "local"
	req.StorageKey = path.Join("evidence", cleanID(item["project_id"]), cleanID(item["id"]), uuid.NewString()+spec.Extension)
	if req.Payload == nil {
		req.Payload = Record{}
	}
	req.Payload["original_filename"] = originalName
	req.Payload["detected_mime_type"] = detectedMIME

	normalized, err := validateWorkItemEvidenceRequest(req)
	if err != nil {
		return "", req, err
	}
	root, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return "", req, err
	}
	fullPath, err := safeStoragePath(root, normalized.StorageKey)
	if err != nil {
		return "", normalized, err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return "", normalized, err
	}
	destination, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return "", normalized, err
	}
	cleanup := true
	defer func() {
		_ = destination.Close()
		if cleanup {
			_ = os.Remove(fullPath)
		}
	}()

	hasher := sha256.New()
	reader := io.MultiReader(bytes.NewReader(header[:n]), source)
	written, err := io.Copy(io.MultiWriter(destination, hasher), reader)
	if err != nil {
		return "", normalized, err
	}
	if written != fileHeader.Size {
		return "", normalized, validationError("Kích thước file evidence không khớp dữ liệu upload", Record{"expected": fileHeader.Size, "written": written})
	}
	normalized.ChecksumSHA256 = hex.EncodeToString(hasher.Sum(nil))
	cleanup = false
	return fullPath, normalized, nil
}

func (s *Service) storeInputInvoicePDF(projectID, payableID string, fileHeader *multipart.FileHeader) (StoredFileMetadata, string, string, error) {
	if fileHeader == nil {
		return StoredFileMetadata{}, "", "", validationError("File PDF hoá đơn là bắt buộc")
	}
	if fileHeader.Size <= 0 {
		return StoredFileMetadata{}, "", "", validationError("File PDF hoá đơn không được rỗng")
	}
	if fileHeader.Size > s.maxUploadBytes {
		return StoredFileMetadata{}, "", "", validationError("File PDF hoá đơn vượt quá dung lượng cho phép", Record{"max_bytes": s.maxUploadBytes})
	}
	source, err := fileHeader.Open()
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	defer source.Close()

	header := make([]byte, 512)
	n, readErr := io.ReadFull(source, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return StoredFileMetadata{}, "", "", readErr
	}
	detectedMIME := http.DetectContentType(header[:n])
	if detectedMIME != "application/pdf" {
		return StoredFileMetadata{}, "", "", validationError("Hoá đơn đầu vào chỉ hỗ trợ PDF", Record{"mime_type": detectedMIME})
	}
	originalName := cleanUploadFilename(fileHeader.Filename)
	if originalName == "evidence-file" {
		originalName = "input-invoice.pdf"
	}
	if filepath.Ext(originalName) == "" {
		originalName += ".pdf"
	}
	storageKey := path.Join("input-invoices", cleanID(projectID), cleanID(payableID), uuid.NewString()+".pdf")
	root, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	fullPath, err := safeStoragePath(root, storageKey)
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	destination, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	cleanup := true
	defer func() {
		_ = destination.Close()
		if cleanup {
			_ = os.Remove(fullPath)
		}
	}()

	hasher := sha256.New()
	reader := io.MultiReader(bytes.NewReader(header[:n]), source)
	written, err := io.Copy(io.MultiWriter(destination, hasher), reader)
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	if written != fileHeader.Size {
		return StoredFileMetadata{}, "", "", validationError("Kích thước file hoá đơn không khớp dữ liệu upload", Record{"expected": fileHeader.Size, "written": written})
	}
	cleanup = false
	return StoredFileMetadata{
		StorageProvider: "local",
		StorageKey:      storageKey,
		MIMEType:        "application/pdf",
		FileSize:        fileHeader.Size,
		ChecksumSHA256:  hex.EncodeToString(hasher.Sum(nil)),
	}, fullPath, originalName, nil
}

func (s *Service) storeOutputInvoicePDF(projectID, receivableID string, fileHeader *multipart.FileHeader) (StoredFileMetadata, string, string, error) {
	if fileHeader == nil {
		return StoredFileMetadata{}, "", "", validationError("File PDF hoá đơn là bắt buộc")
	}
	if fileHeader.Size <= 0 {
		return StoredFileMetadata{}, "", "", validationError("File PDF hoá đơn không được rỗng")
	}
	if fileHeader.Size > s.maxUploadBytes {
		return StoredFileMetadata{}, "", "", validationError("File PDF hoá đơn vượt quá dung lượng cho phép", Record{"max_bytes": s.maxUploadBytes})
	}
	source, err := fileHeader.Open()
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	defer source.Close()

	header := make([]byte, 512)
	n, readErr := io.ReadFull(source, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return StoredFileMetadata{}, "", "", readErr
	}
	detectedMIME := http.DetectContentType(header[:n])
	if detectedMIME != "application/pdf" {
		return StoredFileMetadata{}, "", "", validationError("Hoá đơn đầu ra chỉ hỗ trợ PDF", Record{"mime_type": detectedMIME})
	}
	originalName := cleanUploadFilename(fileHeader.Filename)
	if originalName == "evidence-file" {
		originalName = "output-invoice.pdf"
	}
	if filepath.Ext(originalName) == "" {
		originalName += ".pdf"
	}
	storageKey := path.Join("output-invoices", cleanID(projectID), cleanID(receivableID), uuid.NewString()+".pdf")
	root, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	fullPath, err := safeStoragePath(root, storageKey)
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	destination, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	cleanup := true
	defer func() {
		_ = destination.Close()
		if cleanup {
			_ = os.Remove(fullPath)
		}
	}()

	hasher := sha256.New()
	reader := io.MultiReader(bytes.NewReader(header[:n]), source)
	written, err := io.Copy(io.MultiWriter(destination, hasher), reader)
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	if written != fileHeader.Size {
		return StoredFileMetadata{}, "", "", validationError("Kích thước file hoá đơn không khớp dữ liệu upload", Record{"expected": fileHeader.Size, "written": written})
	}
	cleanup = false
	return StoredFileMetadata{
		StorageProvider: "local",
		StorageKey:      storageKey,
		MIMEType:        "application/pdf",
		FileSize:        fileHeader.Size,
		ChecksumSHA256:  hex.EncodeToString(hasher.Sum(nil)),
	}, fullPath, originalName, nil
}

func (s *Service) storeGeneratedFile(storageKey string, content []byte, mimeType string) (StoredFileMetadata, string, error) {
	if len(content) == 0 {
		return StoredFileMetadata{}, "", validationError("File generated không được rỗng")
	}
	root, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return StoredFileMetadata{}, "", err
	}
	fullPath, err := safeStoragePath(root, storageKey)
	if err != nil {
		return StoredFileMetadata{}, "", err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return StoredFileMetadata{}, "", err
	}
	destination, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return StoredFileMetadata{}, "", err
	}
	cleanup := true
	defer func() {
		_ = destination.Close()
		if cleanup {
			_ = os.Remove(fullPath)
		}
	}()
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(destination, hasher), bytes.NewReader(content))
	if err != nil {
		return StoredFileMetadata{}, "", err
	}
	if written != int64(len(content)) {
		return StoredFileMetadata{}, "", validationError("Kích thước generated file không khớp dữ liệu ghi", Record{"expected": len(content), "written": written})
	}
	cleanup = false
	return StoredFileMetadata{
		StorageProvider: "local",
		StorageKey:      storageKey,
		MIMEType:        mimeType,
		FileSize:        int64(len(content)),
		ChecksumSHA256:  hex.EncodeToString(hasher.Sum(nil)),
	}, fullPath, nil
}

func (s *Service) storedFileResult(record Record, storageKeyField, mimeField, filename string) (EvidenceFileResult, error) {
	storageKey := strings.TrimSpace(fmt.Sprint(record[storageKeyField]))
	if storageKey == "" || storageKey == "<nil>" {
		return EvidenceFileResult{}, validationError("File protected chưa có storage key")
	}
	root, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	filePath, err := safeStoragePath(root, storageKey)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	info, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return EvidenceFileResult{}, validationError("File protected không còn tồn tại trong storage")
		}
		return EvidenceFileResult{}, err
	}
	if info.IsDir() {
		return EvidenceFileResult{}, validationError("Đường dẫn protected file không hợp lệ")
	}
	mimeType := strings.TrimSpace(fmt.Sprint(record[mimeField]))
	if mimeType == "" || mimeType == "<nil>" {
		mimeType = "application/octet-stream"
	}
	return EvidenceFileResult{Path: filePath, MIMEType: mimeType, Filename: cleanUploadFilename(filename)}, nil
}

func parseSignatureImageDataURL(value string) ([]byte, string, error) {
	value = strings.TrimSpace(value)
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(value, prefix) {
		return nil, "", validationError("Chữ ký tay cần là PNG data URL")
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return nil, "", validationError("Dữ liệu ảnh chữ ký không hợp lệ")
	}
	if len(decoded) < 16 {
		return nil, "", validationError("Ảnh chữ ký quá nhỏ hoặc rỗng")
	}
	if len(decoded) > 1024*1024 {
		return nil, "", validationError("Ảnh chữ ký vượt quá dung lượng cho phép", Record{"max_bytes": 1024 * 1024})
	}
	pngHeader := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	if !bytes.HasPrefix(decoded, pngHeader) {
		return nil, "", validationError("Ảnh chữ ký phải là PNG hợp lệ")
	}
	return decoded, "image/png", nil
}

func validateSignatureStrokes(strokes []WorkflowFormSignatureStroke) ([]WorkflowFormSignatureStroke, error) {
	if len(strokes) == 0 {
		return nil, validationError("Cần nét ký tay trước khi ký workflow form")
	}
	if len(strokes) > 80 {
		return nil, validationError("Chữ ký có quá nhiều nét", Record{"max_strokes": 80})
	}
	totalPoints := 0
	out := make([]WorkflowFormSignatureStroke, 0, len(strokes))
	for _, stroke := range strokes {
		points := make([]WorkflowFormSignaturePoint, 0, len(stroke.Points))
		for _, point := range stroke.Points {
			if math.IsNaN(point.X) || math.IsNaN(point.Y) || math.IsInf(point.X, 0) || math.IsInf(point.Y, 0) {
				return nil, validationError("Tọa độ chữ ký không hợp lệ")
			}
			if point.X < 0 || point.X > 1 || point.Y < 0 || point.Y > 1 {
				return nil, validationError("Tọa độ chữ ký nằm ngoài canvas")
			}
			points = append(points, WorkflowFormSignaturePoint{X: point.X, Y: point.Y})
		}
		if len(points) >= 2 {
			totalPoints += len(points)
			out = append(out, WorkflowFormSignatureStroke{Points: points})
		}
	}
	if totalPoints < 2 {
		return nil, validationError("Cần nét ký tay trước khi ký workflow form")
	}
	if totalPoints > 2000 {
		return nil, validationError("Chữ ký có quá nhiều điểm", Record{"max_points": 2000})
	}
	return out, nil
}

func workflowFormPDFFileName(form Record, revisionNo int) string {
	instance, _ := form["instance"].(Record)
	title := cleanUploadFilename(fmt.Sprintf("%s-rev-%d.pdf", slugText(fmt.Sprint(instance["title"])), revisionNo))
	if title == "evidence-file" || title == ".pdf" {
		return fmt.Sprintf("workflow-form-rev-%d.pdf", revisionNo)
	}
	return title
}

func signatureVisualFilename(signature Record) string {
	name := slugText(fmt.Sprintf("%s-%s", signature["signer_name"], signature["purpose"]))
	if name == "" {
		name = "workflow-signature"
	}
	return name + ".png"
}

func slugText(value string) string {
	value = strings.ToLower(asciiFold(value))
	var builder strings.Builder
	lastDash := false
	for _, char := range value {
		allowed := (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')
		if allowed {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
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
	req.StorageProvider = strings.TrimSpace(req.StorageProvider)
	req.StorageKey = strings.TrimSpace(req.StorageKey)
	req.ChecksumSHA256 = strings.ToLower(strings.TrimSpace(req.ChecksumSHA256))
	req.Caption = strings.TrimSpace(req.Caption)
	req.RevisionNo = strings.TrimSpace(req.RevisionNo)
	req.Visibility = strings.TrimSpace(req.Visibility)
	if req.Visibility == "" {
		req.Visibility = "internal"
	}
	if req.StorageProvider == "" {
		req.StorageProvider = "external"
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
	validStorageProvider := map[string]bool{"external": true, "local": true}
	if !validStorageProvider[req.StorageProvider] {
		return req, validationError("Storage provider evidence không hợp lệ", Record{"storage_provider": req.StorageProvider})
	}
	if req.StorageKey != "" && req.StorageProvider != "local" {
		return req, validationError("storage_key chỉ hợp lệ với local storage", Record{"storage_provider": req.StorageProvider})
	}
	if req.ChecksumSHA256 != "" && !validSHA256Hex(req.ChecksumSHA256) {
		return req, validationError("Checksum evidence không hợp lệ", Record{"checksum_sha256": req.ChecksumSHA256})
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
	if statusIn(req.EvidenceType, "document", "photo", "link") && req.FileURL == "" && req.DocumentID == "" && req.StorageKey == "" {
		return req, validationError("Evidence dạng document/photo/link cần file_url, document_id hoặc file upload", Record{"evidence_type": req.EvidenceType})
	}
	return req, nil
}

func validSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func safeStoragePath(root, storageKey string) (string, error) {
	if strings.TrimSpace(storageKey) == "" {
		return "", validationError("storage_key evidence là bắt buộc")
	}
	if filepath.IsAbs(storageKey) {
		return "", validationError("storage_key evidence không hợp lệ")
	}
	cleanKey := filepath.Clean(filepath.FromSlash(storageKey))
	if cleanKey == "." || cleanKey == ".." || strings.HasPrefix(cleanKey, ".."+string(os.PathSeparator)) {
		return "", validationError("storage_key evidence không hợp lệ")
	}
	fullPath := filepath.Join(root, cleanKey)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	if absPath != root && !strings.HasPrefix(absPath, root+string(os.PathSeparator)) {
		return "", validationError("storage_key evidence vượt khỏi storage root")
	}
	return absPath, nil
}

func cleanUploadFilename(filename string) string {
	name := filepath.Base(strings.TrimSpace(filename))
	if name == "" || name == "." || name == "<nil>" || name == string(filepath.Separator) {
		return "evidence-file"
	}
	return strings.Trim(name, " \t\r\n")
}

func evidenceDownloadName(evidence Record) string {
	name := cleanUploadFilename(fmt.Sprint(evidence["name"]))
	storageKey := fmt.Sprint(evidence["storage_key"])
	extension := filepath.Ext(storageKey)
	if filepath.Ext(name) == "" && extension != "" {
		name += extension
	}
	return name
}

func (s *Service) validateWorkflowFormAnswers(ctx context.Context, instance Record, fields []Record, answers Record, submit bool) (Record, error) {
	if answers == nil {
		answers = Record{}
	}
	fieldByKey := map[string]Record{}
	for _, field := range fields {
		fieldByKey[fmt.Sprint(field["field_key"])] = field
	}
	for key := range answers {
		if _, ok := fieldByKey[key]; !ok {
			return nil, validationError("Workflow form có field không thuộc schema", Record{"field": key})
		}
	}
	normalized := Record{}
	missing := []string{}
	fieldErrors := Record{}
	for _, field := range fields {
		key := fmt.Sprint(field["field_key"])
		value, hasValue := answers[key]
		required := boolValue(field["required"])
		if submit && required && workflowFormValueEmpty(field, value) {
			missing = append(missing, key)
			fieldErrors[key] = fmt.Sprintf("%s là bắt buộc", field["label"])
			continue
		}
		if !hasValue || workflowFormValueEmpty(field, value) {
			continue
		}
		normalizedValue, err := s.normalizeWorkflowFormValue(ctx, instance, field, value)
		if err != nil {
			fieldErrors[key] = err.Error()
			continue
		}
		normalized[key] = normalizedValue
	}
	if len(fieldErrors) > 0 {
		return nil, validationError("Workflow form có dữ liệu không hợp lệ", Record{"fields": fieldErrors, "missing": missing})
	}
	if submit && fmt.Sprint(instance["form_type"]) == "checklist" {
		if err := validateChecklistSubmitReady(normalized); err != nil {
			return nil, err
		}
	}
	return normalized, nil
}

func workflowFormValueEmpty(field Record, value any) bool {
	fieldType := fmt.Sprint(field["field_type"])
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case []any:
		return len(v) == 0
	case []string:
		return len(v) == 0
	case []Record:
		return len(v) == 0
	case bool:
		return fieldType == "checkbox" && !v
	default:
		text := strings.TrimSpace(fmt.Sprint(v))
		return text == "" || text == "<nil>"
	}
}

func (s *Service) normalizeWorkflowFormValue(ctx context.Context, instance Record, field Record, value any) (any, error) {
	fieldType := fmt.Sprint(field["field_type"])
	switch fieldType {
	case "text", "textarea":
		text := strings.TrimSpace(fmt.Sprint(value))
		if err := validateWorkflowTextLength(field, text); err != nil {
			return nil, err
		}
		return text, nil
	case "number":
		numeric, err := workflowNumberValue(value)
		if err != nil {
			return nil, err
		}
		return numeric, validateWorkflowNumberRange(field, numeric)
	case "date":
		text := strings.TrimSpace(fmt.Sprint(value))
		if _, err := time.Parse("2006-01-02", text); err != nil {
			return nil, errors.New("Ngày không hợp lệ")
		}
		return text, nil
	case "select":
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" {
			return nil, errors.New("Lựa chọn không hợp lệ")
		}
		if !workflowOptionAllowed(field, text) {
			return nil, errors.New("Lựa chọn không nằm trong schema")
		}
		return text, nil
	case "multi_select":
		values, err := workflowStringList(value)
		if err != nil {
			return nil, err
		}
		for _, selected := range values {
			if !workflowOptionAllowed(field, selected) {
				return nil, errors.New("Lựa chọn không nằm trong schema")
			}
		}
		return values, nil
	case "checkbox":
		checked, ok := value.(bool)
		if !ok {
			return nil, errors.New("Giá trị checkbox phải là boolean")
		}
		return checked, nil
	case "evidence_reference":
		evidenceID := cleanID(value)
		if evidenceID == "" {
			return nil, errors.New("Evidence không hợp lệ")
		}
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, errors.New("Evidence không tồn tại")
			}
			return nil, err
		}
		if cleanID(evidence["project_id"]) != cleanID(instance["project_id"]) {
			return nil, errors.New("Evidence không thuộc công trình này")
		}
		if workItemID := cleanID(instance["work_item_id"]); workItemID != "" && cleanID(evidence["work_item_id"]) != workItemID {
			return nil, errors.New("Evidence không thuộc work item này")
		}
		if fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, errors.New("Evidence đã bị reject")
		}
		return evidenceID, nil
	case "structured_list":
		return s.normalizeWorkflowStructuredList(ctx, instance, field, value)
	default:
		return nil, errors.New("Loại field không được hỗ trợ")
	}
}

func validateChecklistSubmitReady(answers Record) error {
	rawItems, ok := answers["checklist_items"]
	if !ok {
		return nil
	}
	items, err := workflowRecordList(rawItems)
	if err != nil {
		return err
	}
	blocking := []Record{}
	for index, item := range items {
		status := strings.TrimSpace(fmt.Sprint(item["status"]))
		if !statusIn(status, "done", "not_applicable") {
			blocking = append(blocking, Record{
				"row":    index + 1,
				"title":  item["title"],
				"status": status,
			})
		}
	}
	if len(blocking) == 0 {
		return nil
	}
	return validationError("Checklist còn hạng mục chưa đạt, chỉ được submit khi tất cả item là Done hoặc N/A", Record{
		"fields":         Record{"checklist_items": "Tất cả checklist item phải là Done hoặc N/A trước khi submit"},
		"blocking_items": blocking,
	})
}

func (s *Service) normalizeWorkflowStructuredList(ctx context.Context, instance Record, field Record, value any) ([]Record, error) {
	items, err := workflowRecordList(value)
	if err != nil {
		return nil, err
	}
	validation, _ := field["validation_json"].(map[string]any)
	minItems := int(number(validation["min_items"]))
	maxItems := int(number(validation["max_items"]))
	if minItems > 0 && len(items) < minItems {
		return nil, fmt.Errorf("%s cần ít nhất %d dòng", field["label"], minItems)
	}
	if maxItems > 0 && len(items) > maxItems {
		return nil, fmt.Errorf("%s không được vượt quá %d dòng", field["label"], maxItems)
	}
	rawFields, _ := validation["item_fields"].([]any)
	if len(rawFields) == 0 {
		return items, nil
	}
	itemFields := make([]Record, 0, len(rawFields))
	fieldByKey := map[string]Record{}
	for _, rawField := range rawFields {
		spec, ok := rawField.(map[string]any)
		if !ok {
			continue
		}
		key := strings.TrimSpace(fmt.Sprint(spec["key"]))
		if key == "" || key == "<nil>" {
			continue
		}
		record := Record(spec)
		itemFields = append(itemFields, record)
		fieldByKey[key] = record
	}
	out := make([]Record, 0, len(items))
	for index, item := range items {
		for key := range item {
			if _, ok := fieldByKey[key]; !ok {
				return nil, fmt.Errorf("%s dòng %d có field không thuộc schema: %s", field["label"], index+1, key)
			}
		}
		normalized := Record{}
		for _, spec := range itemFields {
			key := fmt.Sprint(spec["key"])
			label := fmt.Sprint(spec["label"])
			if label == "" || label == "<nil>" {
				label = key
			}
			value, hasValue := item[key]
			required := boolValue(spec["required"])
			if required && structuredValueEmpty(spec, value) {
				return nil, fmt.Errorf("%s dòng %d: %s là bắt buộc", field["label"], index+1, label)
			}
			if !hasValue || structuredValueEmpty(spec, value) {
				continue
			}
			normalizedValue, err := s.normalizeStructuredFieldValue(ctx, instance, spec, value)
			if err != nil {
				return nil, fmt.Errorf("%s dòng %d: %s", field["label"], index+1, err.Error())
			}
			normalized[key] = normalizedValue
		}
		if len(normalized) > 0 {
			out = append(out, normalized)
		}
	}
	if minItems > 0 && len(out) < minItems {
		return nil, fmt.Errorf("%s cần ít nhất %d dòng hợp lệ", field["label"], minItems)
	}
	return out, nil
}

func workflowRecordList(value any) ([]Record, error) {
	rawItems, ok := value.([]any)
	if !ok {
		if records, ok := value.([]Record); ok {
			return records, nil
		}
		return nil, errors.New("Giá trị danh sách phải là array")
	}
	items := make([]Record, 0, len(rawItems))
	for _, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, errors.New("Mỗi dòng trong danh sách phải là object")
		}
		items = append(items, Record(item))
	}
	return items, nil
}

func structuredValueEmpty(field Record, value any) bool {
	if value == nil {
		return true
	}
	fieldType := fmt.Sprint(field["type"])
	if fieldType == "checkbox" {
		return value != true
	}
	if values, ok := value.([]any); ok {
		return len(values) == 0
	}
	return strings.TrimSpace(fmt.Sprint(value)) == "" || strings.TrimSpace(fmt.Sprint(value)) == "<nil>"
}

func (s *Service) normalizeStructuredFieldValue(ctx context.Context, instance Record, field Record, value any) (any, error) {
	fieldType := fmt.Sprint(field["type"])
	label := fmt.Sprint(field["label"])
	if label == "" || label == "<nil>" {
		label = fmt.Sprint(field["key"])
	}
	switch fieldType {
	case "text", "textarea":
		text := strings.TrimSpace(fmt.Sprint(value))
		if min := int(number(field["min_length"])); min > 0 && len([]rune(text)) < min {
			return nil, fmt.Errorf("%s phải có ít nhất %d ký tự", label, min)
		}
		if max := int(number(field["max_length"])); max > 0 && len([]rune(text)) > max {
			return nil, fmt.Errorf("%s không được vượt quá %d ký tự", label, max)
		}
		return text, nil
	case "number":
		numeric, err := workflowNumberValue(value)
		if err != nil {
			return nil, fmt.Errorf("%s phải là số", label)
		}
		if minValue, ok := field["min"]; ok && numeric < number(minValue) {
			return nil, fmt.Errorf("%s phải lớn hơn hoặc bằng %v", label, minValue)
		}
		if maxValue, ok := field["max"]; ok && numeric > number(maxValue) {
			return nil, fmt.Errorf("%s phải nhỏ hơn hoặc bằng %v", label, maxValue)
		}
		return numeric, nil
	case "date":
		text := strings.TrimSpace(fmt.Sprint(value))
		if _, err := time.Parse("2006-01-02", text); err != nil {
			return nil, fmt.Errorf("%s không hợp lệ", label)
		}
		return text, nil
	case "select":
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" {
			return nil, fmt.Errorf("%s không hợp lệ", label)
		}
		if !structuredOptionAllowed(field, text) {
			return nil, fmt.Errorf("%s không nằm trong schema", label)
		}
		return text, nil
	case "evidence_reference":
		evidenceID := cleanID(value)
		if evidenceID == "" {
			return nil, fmt.Errorf("%s không hợp lệ", label)
		}
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("%s không tồn tại", label)
			}
			return nil, err
		}
		if cleanID(evidence["project_id"]) != cleanID(instance["project_id"]) {
			return nil, fmt.Errorf("%s không thuộc công trình này", label)
		}
		if workItemID := cleanID(instance["work_item_id"]); workItemID != "" && cleanID(evidence["work_item_id"]) != workItemID {
			return nil, fmt.Errorf("%s không thuộc work item này", label)
		}
		if fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, fmt.Errorf("%s đã bị reject", label)
		}
		return evidenceID, nil
	default:
		return nil, fmt.Errorf("Loại field %s không được hỗ trợ", fieldType)
	}
}

func structuredOptionAllowed(field Record, selected string) bool {
	rawOptions, _ := field["options"].([]any)
	if len(rawOptions) == 0 {
		return true
	}
	for _, raw := range rawOptions {
		switch option := raw.(type) {
		case string:
			if option == selected {
				return true
			}
		case map[string]any:
			if strings.TrimSpace(fmt.Sprint(option["value"])) == selected {
				return true
			}
		}
	}
	return false
}

func validateWorkflowTextLength(field Record, text string) error {
	validation, _ := field["validation_json"].(map[string]any)
	if min := int(number(validation["min_length"])); min > 0 && len([]rune(text)) < min {
		return fmt.Errorf("%s phải có ít nhất %d ký tự", field["label"], min)
	}
	if max := int(number(validation["max_length"])); max > 0 && len([]rune(text)) > max {
		return fmt.Errorf("%s không được vượt quá %d ký tự", field["label"], max)
	}
	return nil
}

func workflowNumberValue(value any) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case string:
		out, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, errors.New("Giá trị phải là số")
		}
		return out, nil
	default:
		return 0, errors.New("Giá trị phải là số")
	}
}

func validateWorkflowNumberRange(field Record, numeric float64) error {
	validation, _ := field["validation_json"].(map[string]any)
	if minValue, ok := validation["min"]; ok && numeric < number(minValue) {
		return fmt.Errorf("%s phải lớn hơn hoặc bằng %v", field["label"], minValue)
	}
	if maxValue, ok := validation["max"]; ok && numeric > number(maxValue) {
		return fmt.Errorf("%s phải nhỏ hơn hoặc bằng %v", field["label"], maxValue)
	}
	return nil
}

func workflowStringList(value any) ([]string, error) {
	switch v := value.(type) {
	case []string:
		out := make([]string, 0, len(v))
		seen := map[string]bool{}
		for _, item := range v {
			text := strings.TrimSpace(item)
			if text == "" || seen[text] {
				continue
			}
			seen[text] = true
			out = append(out, text)
		}
		return out, nil
	case []any:
		out := make([]string, 0, len(v))
		seen := map[string]bool{}
		for _, item := range v {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text == "" || text == "<nil>" || seen[text] {
				continue
			}
			seen[text] = true
			out = append(out, text)
		}
		return out, nil
	default:
		return nil, errors.New("Giá trị phải là danh sách")
	}
}

func workflowOptionAllowed(field Record, value string) bool {
	options, _ := field["options_json"].([]any)
	if len(options) == 0 {
		return true
	}
	for _, rawOption := range options {
		option, _ := rawOption.(map[string]any)
		if strings.TrimSpace(fmt.Sprint(option["value"])) == value {
			return true
		}
		if option == nil && strings.TrimSpace(fmt.Sprint(rawOption)) == value {
			return true
		}
	}
	return false
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
		submittedForm, err := s.repo.HasSubmittedWorkflowFormForWorkItem(ctx, fmt.Sprint(item["id"]))
		if err != nil {
			return nil, err
		}
		if !submittedForm {
			missing = append(missing, "checklist")
		}
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

func (s *Service) ProjectBOQ(ctx context.Context, projectID string) ([]Record, error) {
	if cleanID(projectID) == "" {
		return nil, validationError("project_id không hợp lệ", Record{"project_id": projectID})
	}
	return s.repo.ListProjectBOQ(ctx, projectID)
}

func (s *Service) CreateBOQ(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	normalized, err := normalizeBOQItemRequest(projectID, body, true)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateBOQItem(ctx, normalized, actorID, actorName)
}

func (s *Service) UpdateBOQ(ctx context.Context, itemID string, body Record, actorID, actorName string) (Record, error) {
	if cleanID(itemID) == "" {
		return nil, validationError("boq_item_id không hợp lệ", Record{"boq_item_id": itemID})
	}
	normalized, err := normalizeBOQItemRequest("", body, false)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateBOQItem(ctx, itemID, normalized, actorID, actorName)
}

func (s *Service) UpdateBOQExpectedUnitPrice(ctx context.Context, itemID string, req BOQUnitPriceRequest, actorID, actorName string) (Record, error) {
	unitPrice, comment, err := normalizeBOQUnitPriceRequest(req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateBOQExpectedUnitPrice(ctx, itemID, unitPrice, comment, actorID, actorName)
}

func (s *Service) LockBOQNormUnitPrice(ctx context.Context, itemID string, req BOQUnitPriceRequest, actorID, actorName string) (Record, error) {
	unitPrice, comment, err := normalizeBOQUnitPriceRequest(req, true)
	if err != nil {
		return nil, err
	}
	return s.repo.LockBOQNormUnitPrice(ctx, itemID, unitPrice, comment, actorID, actorName)
}

func (s *Service) LockBOQFinalUnitPrice(ctx context.Context, itemID string, req BOQUnitPriceRequest, actorID, actorName string) (Record, error) {
	unitPrice, comment, err := normalizeBOQUnitPriceRequest(req, true)
	if err != nil {
		return nil, err
	}
	return s.repo.LockBOQFinalUnitPrice(ctx, itemID, unitPrice, comment, actorID, actorName)
}

func normalizeBOQUnitPriceRequest(req BOQUnitPriceRequest, lock bool) (float64, string, error) {
	if math.IsNaN(req.UnitPrice) || math.IsInf(req.UnitPrice, 0) {
		return 0, "", validationError("Đơn giá không hợp lệ")
	}
	if lock && req.UnitPrice <= 0 {
		return 0, "", validationError("Đơn giá lock phải lớn hơn 0")
	}
	if !lock && req.UnitPrice < 0 {
		return 0, "", validationError("Expected unit price không được âm")
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return 0, "", validationError("Ghi chú đơn giá không được vượt quá 1000 ký tự")
	}
	return req.UnitPrice, comment, nil
}

func normalizeBOQItemRequest(projectID string, body Record, create bool) (Record, error) {
	out := Record{}
	if projectID != "" {
		if cleanID(projectID) == "" {
			return nil, validationError("project_id không hợp lệ", Record{"project_id": projectID})
		}
		out["project_id"] = projectID
	}
	textFields := map[string]int{
		"category":       120,
		"work_item_name": 180,
		"description":    2000,
		"unit":           40,
	}
	for field, maxLen := range textFields {
		value, ok := body[field]
		if !ok {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if len([]rune(text)) > maxLen {
			return nil, validationError("Dữ liệu BOQ vượt quá độ dài cho phép", Record{"field": field, "max_length": maxLen})
		}
		out[field] = text
	}
	if create {
		for _, field := range []string{"category", "work_item_name", "unit"} {
			if strings.TrimSpace(fmt.Sprint(out[field])) == "" {
				return nil, validationError("Dòng BOQ thiếu dữ liệu bắt buộc", Record{"field": field})
			}
		}
	}
	if value, ok := body["quantity"]; ok {
		quantity, err := workflowNumberValue(value)
		if err != nil {
			return nil, validationError("Khối lượng BOQ phải là số")
		}
		if quantity < 0 {
			return nil, validationError("Khối lượng BOQ không được âm")
		}
		out["quantity"] = quantity
	} else if create {
		out["quantity"] = float64(0)
	}
	priceValue, hasExpected := body["expected_unit_price"]
	if !hasExpected {
		priceValue, hasExpected = body["unit_price"]
	}
	if hasExpected {
		price, err := workflowNumberValue(priceValue)
		if err != nil {
			return nil, validationError("Expected unit price phải là số")
		}
		if price < 0 {
			return nil, validationError("Expected unit price không được âm")
		}
		out["expected_unit_price"] = price
	} else if create {
		out["expected_unit_price"] = float64(0)
	}
	for _, field := range []string{"material_id", "node_id", "approved_by"} {
		if value, ok := body[field]; ok {
			out[field] = cleanID(value)
		}
	}
	if value, ok := body["status"]; ok {
		status := strings.TrimSpace(fmt.Sprint(value))
		if !statusIn(status, "draft", "submitted", "approved", "revised", "deleted") {
			return nil, validationError("Trạng thái BOQ không hợp lệ", Record{"status": status})
		}
		out["status"] = status
	} else if create {
		out["status"] = "draft"
	}
	return out, nil
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

func (s *Service) ProjectAcceptances(ctx context.Context, projectID, actorID string) ([]Record, error) {
	projectID = cleanID(projectID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu", Record{"permissions": []string{"acceptance.view", "operation.view"}})
	}
	return s.repo.ListProjectAcceptanceRecords(ctx, projectID)
}

func (s *Service) WorkItemOutgoingAcceptance(ctx context.Context, workItemID, actorID string) (Record, error) {
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
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view", "acceptance.create", "operation.report"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu đầu ra", Record{"permissions": []string{"acceptance.view", "operation.view"}})
	}
	records, err := s.repo.ListAcceptanceRecordsForWorkItem(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	if errors.Is(err, sql.ErrNoRows) {
		form = Record{"available": false}
	} else if err != nil {
		return nil, err
	} else {
		form["available"] = true
	}
	return Record{
		"available":    true,
		"work_item_id": item["id"],
		"project_id":   projectID,
		"records":      records,
		"form":         form,
	}, nil
}

func (s *Service) GetAcceptance(ctx context.Context, acceptanceID, actorID string) (Record, error) {
	record, err := s.repo.GetAcceptanceDetail(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(record["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu", Record{"permissions": []string{"acceptance.view", "operation.view"}})
	}
	workItemID := cleanID(record["work_item_id"])
	if workItemID != "" {
		canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
		if err != nil {
			return nil, err
		}
		if !canView {
			return nil, forbiddenError("Bạn không có quyền xem work item này")
		}
	}
	return record, nil
}

func (s *Service) CreateAcceptance(ctx context.Context, projectID string, req AcceptanceRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	item, err := s.requireAcceptanceWriteAccess(ctx, projectID, req.WorkItemID, actorID, []string{"acceptance.create", "operation.report"}, "Bạn không có quyền lưu nghiệm thu đầu ra")
	if err != nil {
		return nil, err
	}
	header, items, err := s.normalizeAcceptanceRequest(ctx, projectID, item, nil, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveAcceptance(ctx, "", header, items, actorID, actorName)
}

func (s *Service) UpdateAcceptance(ctx context.Context, acceptanceID string, req AcceptanceRequest, actorID, actorName string) (Record, error) {
	existing, err := s.repo.GetAcceptanceDetail(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(existing["status"]) != "draft" {
		return nil, validationError("Hồ sơ nghiệm thu đầu ra đã submit/chốt, không thể sửa", Record{"acceptance_id": acceptanceID, "status": existing["status"]})
	}
	projectID := cleanID(existing["project_id"])
	workItemID := req.WorkItemID
	if cleanID(workItemID) == "" {
		workItemID = cleanID(existing["work_item_id"])
	}
	item, err := s.requireAcceptanceWriteAccess(ctx, projectID, workItemID, actorID, []string{"acceptance.create", "operation.report"}, "Bạn không có quyền lưu nghiệm thu đầu ra")
	if err != nil {
		return nil, err
	}
	header, items, err := s.normalizeAcceptanceRequest(ctx, projectID, item, existing, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveAcceptance(ctx, cleanID(existing["id"]), header, items, actorID, actorName)
}

func (s *Service) SubmitAcceptance(ctx context.Context, acceptanceID string, actorID, actorName string) (Record, error) {
	existing, err := s.repo.GetAcceptanceDetail(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(existing["status"]) != "draft" {
		return nil, validationError("Chỉ hồ sơ nghiệm thu nháp mới được submit", Record{"acceptance_id": acceptanceID, "status": existing["status"]})
	}
	projectID := cleanID(existing["project_id"])
	item, err := s.requireAcceptanceWriteAccess(ctx, projectID, cleanID(existing["work_item_id"]), actorID, []string{"acceptance.create", "operation.report"}, "Bạn không có quyền submit nghiệm thu đầu ra")
	if err != nil {
		return nil, err
	}
	req := acceptanceRequestFromRecord(existing)
	if _, _, err := s.normalizeAcceptanceRequest(ctx, projectID, item, existing, req, true); err != nil {
		return nil, err
	}
	return s.repo.SubmitAcceptance(ctx, cleanID(existing["id"]), actorID, actorName)
}

func (s *Service) AcceptanceAction(ctx context.Context, id, result string, req ActionRequest, actorID, actorName string) (Record, error) {
	existing, err := s.repo.GetAcceptanceDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	if cleanID(existing["work_item_id"]) != "" {
		if fmt.Sprint(existing["status"]) != "submitted" {
			return nil, validationError("Nghiệm thu đầu ra cần submit trước khi chốt kết quả", Record{"acceptance_id": id, "status": existing["status"]})
		}
		if err := validateAcceptanceActionResult(result, recordValue(existing["summary_json"])); err != nil {
			return nil, err
		}
	}
	return s.repo.AcceptanceAction(ctx, id, result, req, actorID, actorName)
}

func (s *Service) CreateReworkFromAcceptance(ctx context.Context, acceptanceID, actorID, actorName string) (Record, error) {
	return s.repo.CreateReworkFromAcceptance(ctx, acceptanceID, actorID, actorName)
}

func (s *Service) requireAcceptanceWriteAccess(ctx context.Context, projectID, workItemID, actorID string, permissions []string, message string) (Record, error) {
	projectID = cleanID(projectID)
	workItemID = cleanID(workItemID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, permissions)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError(message, Record{"permissions": permissions})
	}
	if workItemID == "" {
		return nil, nil
	}
	canAct, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canAct {
		return nil, forbiddenError("Bạn không có quyền thao tác work item này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if cleanID(item["project_id"]) != projectID {
		return nil, validationError("Work item không thuộc công trình nghiệm thu", Record{"work_item_id": workItemID, "project_id": projectID})
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) normalizeAcceptanceRequest(ctx context.Context, projectID string, item Record, existing Record, req AcceptanceRequest, submit bool) (Record, []Record, error) {
	projectID = cleanID(projectID)
	workItemID := cleanID(req.WorkItemID)
	if workItemID == "" && item != nil {
		workItemID = cleanID(item["id"])
	}
	if workItemID == "" && existing != nil {
		workItemID = cleanID(existing["work_item_id"])
	}
	if workItemID != "" {
		if item == nil || cleanID(item["id"]) != workItemID {
			workItem, err := s.repo.GetWorkItemDetail(ctx, workItemID)
			if err != nil {
				return nil, nil, err
			}
			item = workItem
		}
		if cleanID(item["project_id"]) != projectID {
			return nil, nil, validationError("Work item không thuộc công trình nghiệm thu", Record{"work_item_id": workItemID, "project_id": projectID})
		}
	}

	acceptanceType := strings.TrimSpace(req.Type)
	if acceptanceType == "" {
		acceptanceType = "partial"
	}
	if !statusIn(acceptanceType, "partial", "final") {
		return nil, nil, validationError("Loại nghiệm thu không hợp lệ", Record{"type": acceptanceType})
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Nghiệm thu đầu ra"
	}
	if len([]rune(title)) > 240 {
		return nil, nil, validationError("Tiêu đề nghiệm thu không được vượt quá 240 ký tự")
	}
	acceptanceDate := strings.TrimSpace(req.AcceptanceDate)
	if acceptanceDate != "" {
		if _, err := time.Parse("2006-01-02", acceptanceDate); err != nil {
			return nil, nil, validationError("Ngày nghiệm thu không hợp lệ", Record{"acceptance_date": acceptanceDate})
		}
	} else if submit {
		return nil, nil, validationError("Ngày nghiệm thu là bắt buộc khi submit")
	}
	location := strings.TrimSpace(req.Location)
	clientRepresentative := strings.TrimSpace(req.ClientRepresentative)
	contractorRepresentative := strings.TrimSpace(req.ContractorRepresentative)
	scopeSummary := strings.TrimSpace(req.ScopeSummary)
	for field, value := range map[string]string{
		"location":                  location,
		"client_representative":     clientRepresentative,
		"contractor_representative": contractorRepresentative,
	} {
		if len([]rune(value)) > 240 {
			return nil, nil, validationError("Thông tin nghiệm thu không được vượt quá 240 ký tự", Record{"field": field})
		}
	}
	if len([]rune(scopeSummary)) > 1000 {
		return nil, nil, validationError("Phạm vi nghiệm thu không được vượt quá 1000 ký tự")
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, nil, validationError("Ghi chú nghiệm thu không được vượt quá 2000 ký tự")
	}

	formInstanceID := cleanID(req.FormInstanceID)
	formHasSignature := false
	if formInstanceID != "" {
		form, err := s.repo.GetWorkflowFormInstanceDetail(ctx, formInstanceID)
		if err != nil {
			return nil, nil, validationError("Workflow form liên kết nghiệm thu không hợp lệ", Record{"form_instance_id": formInstanceID})
		}
		instance, _ := form["instance"].(Record)
		if cleanID(instance["project_id"]) != projectID {
			return nil, nil, validationError("Workflow form không thuộc công trình nghiệm thu", Record{"form_instance_id": formInstanceID})
		}
		if workItemID != "" && cleanID(instance["work_item_id"]) != workItemID {
			return nil, nil, validationError("Workflow form không thuộc work item nghiệm thu", Record{"form_instance_id": formInstanceID})
		}
		if submit && fmt.Sprint(instance["status"]) != "submitted" {
			return nil, nil, validationError("Workflow form cần submit trước nghiệm thu đầu ra", Record{"form_instance_id": formInstanceID})
		}
		if signatures, ok := form["signatures"].([]Record); ok && len(signatures) > 0 {
			formHasSignature = true
		}
	}
	pdfExportID := cleanID(req.PDFExportID)
	if pdfExportID != "" {
		export, err := s.repo.GetWorkflowFormPDFExportDetail(ctx, pdfExportID)
		if err != nil {
			return nil, nil, validationError("PDF workflow form liên kết nghiệm thu không hợp lệ", Record{"pdf_export_id": pdfExportID})
		}
		if cleanID(export["project_id"]) != projectID {
			return nil, nil, validationError("PDF workflow form không thuộc công trình nghiệm thu", Record{"pdf_export_id": pdfExportID})
		}
		if workItemID != "" && cleanID(export["work_item_id"]) != workItemID {
			return nil, nil, validationError("PDF workflow form không thuộc work item nghiệm thu", Record{"pdf_export_id": pdfExportID})
		}
		if formInstanceID != "" && cleanID(export["form_instance_id"]) != formInstanceID {
			return nil, nil, validationError("PDF workflow form không thuộc workflow form đã chọn", Record{"pdf_export_id": pdfExportID, "form_instance_id": formInstanceID})
		}
		if formInstanceID == "" {
			formInstanceID = cleanID(export["form_instance_id"])
		}
	}
	signatureStatus := strings.TrimSpace(req.SignatureStatus)
	if signatureStatus == "" {
		if formHasSignature {
			signatureStatus = "signed"
		} else {
			signatureStatus = "pending"
		}
	}
	if !statusIn(signatureStatus, "pending", "signed", "waived") {
		return nil, nil, validationError("Trạng thái chữ ký nghiệm thu không hợp lệ", Record{"signature_status": signatureStatus})
	}
	documentStatus := strings.TrimSpace(req.DocumentStatus)
	if documentStatus == "" {
		if pdfExportID != "" {
			documentStatus = "linked"
		} else {
			documentStatus = "pending"
		}
	}
	if !statusIn(documentStatus, "pending", "linked", "waived") {
		return nil, nil, validationError("Trạng thái tài liệu nghiệm thu không hợp lệ", Record{"document_status": documentStatus})
	}
	if submit && signatureStatus == "pending" {
		return nil, nil, validationError("Nghiệm thu đầu ra cần chữ ký hoặc waiver chữ ký trước khi submit")
	}
	if submit && documentStatus == "pending" {
		return nil, nil, validationError("Nghiệm thu đầu ra cần tài liệu/PDF hoặc waiver tài liệu trước khi submit")
	}
	if submit && len(req.Checklist) == 0 {
		return nil, nil, validationError("Cần ít nhất một checklist item để submit nghiệm thu đầu ra")
	}

	seen := map[int]bool{}
	normalized := make([]Record, 0, len(req.Checklist))
	for index, itemReq := range req.Checklist {
		itemIndex := itemReq.ItemIndex
		if itemIndex < 0 {
			itemIndex = index
		}
		if itemIndex < 0 {
			return nil, nil, validationError("Dòng checklist nghiệm thu không hợp lệ", Record{"item_index": itemReq.ItemIndex})
		}
		if seen[itemIndex] {
			return nil, nil, validationError("Dòng checklist nghiệm thu bị trùng", Record{"item_index": itemIndex})
		}
		seen[itemIndex] = true
		row, err := s.normalizeAcceptanceChecklistItem(ctx, projectID, workItemID, itemIndex, itemReq, submit)
		if err != nil {
			return nil, nil, err
		}
		normalized = append(normalized, row)
	}
	summary := acceptanceSummary(normalized)
	if submit {
		if pending := int(number(summary["pending"])); pending > 0 {
			return nil, nil, validationError("Không thể submit nghiệm thu đầu ra khi còn checklist pending", Record{"pending": pending})
		}
	}
	header := Record{
		"project_id":                projectID,
		"work_item_id":              workItemID,
		"form_instance_id":          formInstanceID,
		"pdf_export_id":             pdfExportID,
		"type":                      acceptanceType,
		"title":                     title,
		"acceptance_date":           acceptanceDate,
		"location":                  location,
		"client_representative":     clientRepresentative,
		"contractor_representative": contractorRepresentative,
		"scope_summary":             scopeSummary,
		"signature_status":          signatureStatus,
		"document_status":           documentStatus,
		"notes":                     notes,
		"summary_json":              summary,
	}
	return header, normalized, nil
}

func (s *Service) normalizeAcceptanceChecklistItem(ctx context.Context, projectID, workItemID string, itemIndex int, req AcceptanceChecklistItemRequest, submit bool) (Record, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, validationError("Tên checklist nghiệm thu là bắt buộc", Record{"item_index": itemIndex})
	}
	if len([]rune(title)) > 240 {
		return nil, validationError("Tên checklist nghiệm thu không được vượt quá 240 ký tự", Record{"item_index": itemIndex})
	}
	description := strings.TrimSpace(req.Description)
	if len([]rune(description)) > 1000 {
		return nil, validationError("Mô tả checklist nghiệm thu không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	decision := strings.TrimSpace(req.Decision)
	if decision == "" {
		decision = "pending"
	}
	if !statusIn(decision, "pending", "pass", "conditional_pass", "fail") {
		return nil, validationError("Quyết định checklist nghiệm thu không hợp lệ", Record{"item_index": itemIndex, "decision": decision})
	}
	if submit && decision == "pending" {
		return nil, validationError("Checklist nghiệm thu chưa có quyết định", Record{"item_index": itemIndex})
	}
	severity := strings.TrimSpace(req.Severity)
	if severity == "" {
		severity = "medium"
	}
	if !statusIn(severity, "low", "medium", "high", "critical") {
		return nil, validationError("Mức độ checklist nghiệm thu không hợp lệ", Record{"item_index": itemIndex, "severity": severity})
	}
	requiredAction := strings.TrimSpace(req.RequiredAction)
	if len([]rune(requiredAction)) > 1000 {
		return nil, validationError("Yêu cầu xử lý checklist nghiệm thu không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	dueDate := strings.TrimSpace(req.DueDate)
	if dueDate != "" {
		if _, err := time.Parse("2006-01-02", dueDate); err != nil {
			return nil, validationError("Ngày hạn xử lý checklist nghiệm thu không hợp lệ", Record{"item_index": itemIndex, "due_date": dueDate})
		}
	}
	note := strings.TrimSpace(req.Note)
	if len([]rune(note)) > 1000 {
		return nil, validationError("Ghi chú checklist nghiệm thu không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	photoURL := strings.TrimSpace(req.PhotoURL)
	if len([]rune(photoURL)) > 1000 {
		return nil, validationError("URL ảnh checklist nghiệm thu không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	evidenceID := cleanID(req.EvidenceID)
	if evidenceID != "" {
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			return nil, validationError("Evidence checklist nghiệm thu không hợp lệ", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if cleanID(evidence["project_id"]) != projectID || fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, validationError("Evidence checklist nghiệm thu không thuộc công trình", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if workItemID != "" && cleanID(evidence["work_item_id"]) != workItemID {
			return nil, validationError("Evidence checklist nghiệm thu không thuộc work item", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
	}
	evidenceRequired := req.PhotoRequired || decision == "conditional_pass" || decision == "fail"
	if submit && evidenceRequired && evidenceID == "" && photoURL == "" {
		return nil, validationError("Checklist nghiệm thu cần evidence/ảnh", Record{"item_index": itemIndex})
	}
	if submit && statusIn(decision, "conditional_pass", "fail") && requiredAction == "" {
		return nil, validationError("Checklist conditional/fail cần yêu cầu xử lý", Record{"item_index": itemIndex})
	}
	return Record{
		"item_index":      itemIndex,
		"title":           title,
		"description":     description,
		"decision":        decision,
		"severity":        severity,
		"required_action": requiredAction,
		"due_date":        dueDate,
		"evidence_id":     evidenceID,
		"note":            note,
		"photo_required":  req.PhotoRequired,
		"photo_url":       photoURL,
		"payload_json": Record{
			"evidence_required": evidenceRequired,
		},
	}, nil
}

func acceptanceRequestFromRecord(record Record) AcceptanceRequest {
	req := AcceptanceRequest{
		WorkItemID:               cleanID(record["work_item_id"]),
		FormInstanceID:           cleanID(record["form_instance_id"]),
		PDFExportID:              cleanID(record["pdf_export_id"]),
		Type:                     cleanOptionalText(record["type"]),
		Title:                    cleanOptionalText(record["title"]),
		AcceptanceDate:           dateOnly(record["acceptance_date"]),
		Location:                 cleanOptionalText(record["location"]),
		ClientRepresentative:     cleanOptionalText(record["client_representative"]),
		ContractorRepresentative: cleanOptionalText(record["contractor_representative"]),
		ScopeSummary:             cleanOptionalText(record["scope_summary"]),
		SignatureStatus:          cleanOptionalText(record["signature_status"]),
		DocumentStatus:           cleanOptionalText(record["document_status"]),
		Notes:                    cleanOptionalText(record["notes"]),
		Checklist:                []AcceptanceChecklistItemRequest{},
	}
	if items, ok := record["checklist"].([]Record); ok {
		req.Checklist = make([]AcceptanceChecklistItemRequest, 0, len(items))
		for _, item := range items {
			req.Checklist = append(req.Checklist, AcceptanceChecklistItemRequest{
				ItemIndex:      int(number(item["item_index"])),
				Title:          cleanOptionalText(item["title"]),
				Description:    cleanOptionalText(item["description"]),
				Decision:       cleanOptionalText(item["decision"]),
				Severity:       cleanOptionalText(item["severity"]),
				RequiredAction: cleanOptionalText(item["required_action"]),
				DueDate:        dateOnly(item["due_date"]),
				EvidenceID:     cleanID(item["evidence_id"]),
				Note:           cleanOptionalText(item["note"]),
				PhotoRequired:  boolValue(item["photo_required"]),
				PhotoURL:       cleanOptionalText(item["photo_url"]),
			})
		}
	}
	return req
}

func acceptanceSummary(items []Record) Record {
	summary := Record{
		"total":          len(items),
		"pass":           0,
		"conditional":    0,
		"fail":           0,
		"pending":        0,
		"evidence_count": 0,
		"critical_count": 0,
	}
	for _, item := range items {
		switch cleanOptionalText(item["decision"]) {
		case "pass":
			summary["pass"] = int(number(summary["pass"])) + 1
		case "conditional_pass":
			summary["conditional"] = int(number(summary["conditional"])) + 1
		case "fail":
			summary["fail"] = int(number(summary["fail"])) + 1
		default:
			summary["pending"] = int(number(summary["pending"])) + 1
		}
		if cleanID(item["evidence_id"]) != "" || cleanOptionalText(item["photo_url"]) != "" {
			summary["evidence_count"] = int(number(summary["evidence_count"])) + 1
		}
		if cleanOptionalText(item["severity"]) == "critical" {
			summary["critical_count"] = int(number(summary["critical_count"])) + 1
		}
	}
	return summary
}

func validateAcceptanceActionResult(result string, summary Record) error {
	result = strings.TrimSpace(result)
	total := int(number(summary["total"]))
	passed := int(number(summary["pass"]))
	conditional := int(number(summary["conditional"]))
	failed := int(number(summary["fail"]))
	pending := int(number(summary["pending"]))
	if total == 0 {
		return validationError("Cần checklist item trước khi chốt nghiệm thu đầu ra")
	}
	if pending > 0 {
		return validationError("Không thể chốt nghiệm thu khi còn checklist pending", Record{"pending": pending})
	}
	switch result {
	case "pass":
		if passed != total {
			return validationError("Kết quả pass chỉ hợp lệ khi mọi checklist đều pass")
		}
	case "conditional_pass":
		if conditional+failed == 0 {
			return validationError("Kết quả conditional pass cần checklist conditional hoặc fail")
		}
	case "fail":
		if failed == 0 {
			return validationError("Kết quả fail cần ít nhất một checklist fail")
		}
	default:
		return validationError("Kết quả nghiệm thu không hợp lệ", Record{"result": result})
	}
	return nil
}

func recordValue(value any) Record {
	switch v := value.(type) {
	case Record:
		return v
	case map[string]any:
		return Record(v)
	default:
		return Record{}
	}
}

func (s *Service) HandoverCheck(ctx context.Context, id string) (Record, error) {
	return s.repo.HandoverCheck(ctx, id)
}

func (s *Service) CompleteHandover(ctx context.Context, id string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.CompleteHandover(ctx, id, req, actorID, actorName)
}

func (s *Service) PayableInputInvoices(ctx context.Context, payableID, actorID string) ([]Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.view", "accounting.payable_view"}, "Bạn không có quyền xem hoá đơn đầu vào")
	if err != nil {
		return nil, err
	}
	return s.repo.ListInputInvoicesForPayable(ctx, cleanID(payable["id"]))
}

func (s *Service) ProjectInputInvoices(ctx context.Context, projectID, actorID string) ([]Record, error) {
	projectID = cleanID(projectID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.view", "accounting.payable_view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem hoá đơn đầu vào", Record{"permissions": []string{"accounting.view", "accounting.payable_view"}})
	}
	return s.repo.ListInputInvoicesForProject(ctx, projectID)
}

func (s *Service) CreateInputInvoice(ctx context.Context, payableID string, req InputInvoiceRequest, fileHeader *multipart.FileHeader, actorID, actorName string) (Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.payable_approve"}, "Bạn không có quyền upload hoá đơn đầu vào")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizeInputInvoiceRequest(ctx, payable, req)
	if err != nil {
		return nil, err
	}
	fileMeta, storedPath, originalName, err := s.storeInputInvoicePDF(cleanID(payable["project_id"]), cleanID(payable["id"]), fileHeader)
	if err != nil {
		return nil, err
	}
	body["file_name"] = originalName
	record, err := s.repo.CreateInputInvoice(ctx, payable, body, fileMeta, actorID, actorName)
	if err != nil {
		_ = os.Remove(storedPath)
		return nil, err
	}
	return record, nil
}

func (s *Service) InputInvoiceAction(ctx context.Context, invoiceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	invoice, err := s.repo.GetInputInvoiceDetail(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(invoice["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.payable_approve"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền cập nhật hoá đơn đầu vào", Record{"permissions": []string{"accounting.payable_approve"}})
	}
	return s.repo.InputInvoiceAction(ctx, cleanID(invoice["id"]), action, req, actorID, actorName)
}

func (s *Service) InputInvoiceFile(ctx context.Context, invoiceID, actorID string) (EvidenceFileResult, error) {
	invoiceID = cleanID(invoiceID)
	if !validActivityUUID(invoiceID) {
		return EvidenceFileResult{}, validationError("invoice_id không hợp lệ", Record{"invoice_id": invoiceID})
	}
	invoice, err := s.repo.GetInputInvoiceDetail(ctx, invoiceID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	projectID := cleanID(invoice["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.view", "accounting.payable_view"})
	if err != nil {
		return EvidenceFileResult{}, err
	}
	if !allowed {
		return EvidenceFileResult{}, forbiddenError("Bạn không có quyền xem file hoá đơn đầu vào")
	}
	if fmt.Sprint(invoice["status"]) == "deleted" {
		return EvidenceFileResult{}, validationError("Hoá đơn đầu vào đã bị xoá", Record{"invoice_id": invoiceID})
	}
	return s.storedFileResult(invoice, "storage_key", "mime_type", fmt.Sprint(invoice["file_name"]))
}

func (s *Service) requirePayableAccess(ctx context.Context, payableID, actorID string, permissions []string, message string) (Record, error) {
	payableID = cleanID(payableID)
	if !validActivityUUID(payableID) {
		return nil, validationError("payable_id không hợp lệ", Record{"payable_id": payableID})
	}
	payable, err := s.repo.GetPayableDetail(ctx, payableID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(payable["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, permissions)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError(message, Record{"permissions": permissions})
	}
	return payable, nil
}

func (s *Service) normalizeInputInvoiceRequest(ctx context.Context, payable Record, req InputInvoiceRequest) (Record, error) {
	projectID := cleanID(payable["project_id"])
	invoiceNumber := strings.TrimSpace(req.InvoiceNumber)
	if invoiceNumber == "" {
		return nil, validationError("Số hoá đơn là bắt buộc")
	}
	if len([]rune(invoiceNumber)) > 120 {
		return nil, validationError("Số hoá đơn không được vượt quá 120 ký tự")
	}
	invoiceSymbol := strings.TrimSpace(req.InvoiceSymbol)
	if len([]rune(invoiceSymbol)) > 80 {
		return nil, validationError("Ký hiệu hoá đơn không được vượt quá 80 ký tự")
	}
	invoiceDate := strings.TrimSpace(req.InvoiceDate)
	if invoiceDate == "" {
		return nil, validationError("Ngày hoá đơn là bắt buộc")
	}
	if _, err := time.Parse("2006-01-02", invoiceDate); err != nil {
		return nil, validationError("Ngày hoá đơn không hợp lệ", Record{"invoice_date": invoiceDate})
	}
	vendorName := strings.TrimSpace(req.VendorName)
	if vendorName == "" {
		vendorName = cleanOptionalText(payable["vendor_name"])
	}
	if vendorName == "" {
		return nil, validationError("Tên vendor trên hoá đơn là bắt buộc")
	}
	if len([]rune(vendorName)) > 240 {
		return nil, validationError("Tên vendor trên hoá đơn không được vượt quá 240 ký tự")
	}
	vendorTaxCode := strings.TrimSpace(req.VendorTaxCode)
	if len([]rune(vendorTaxCode)) > 80 {
		return nil, validationError("Mã số thuế vendor không được vượt quá 80 ký tự")
	}
	if req.AmountBeforeVAT < 0 || req.VATAmount < 0 || req.TotalAmount < 0 {
		return nil, validationError("Số tiền hoá đơn không được âm")
	}
	totalAmount := req.TotalAmount
	if totalAmount == 0 && req.AmountBeforeVAT+req.VATAmount > 0 {
		totalAmount = req.AmountBeforeVAT + req.VATAmount
	}
	if totalAmount <= 0 {
		return nil, validationError("Tổng tiền hoá đơn phải lớn hơn 0")
	}
	if req.AmountBeforeVAT+req.VATAmount > 0 && totalAmount+1 < req.AmountBeforeVAT+req.VATAmount {
		return nil, validationError("Tổng tiền hoá đơn nhỏ hơn trước VAT cộng VAT", Record{
			"amount_before_vat": req.AmountBeforeVAT,
			"vat_amount":        req.VATAmount,
			"total_amount":      totalAmount,
		})
	}
	procurementRequestID := cleanID(req.ProcurementRequestID)
	if procurementRequestID != "" {
		if !validActivityUUID(procurementRequestID) {
			return nil, validationError("procurement_request_id không hợp lệ", Record{"procurement_request_id": procurementRequestID})
		}
		procurement, err := s.repo.GetByID(ctx, "procurement_requests", procurementRequestID)
		if err != nil {
			return nil, validationError("Yêu cầu mua liên kết hoá đơn không tồn tại", Record{"procurement_request_id": procurementRequestID})
		}
		if cleanID(procurement["project_id"]) != projectID || fmt.Sprint(procurement["status"]) == "cancelled" {
			return nil, validationError("Yêu cầu mua liên kết hoá đơn không thuộc công trình", Record{"procurement_request_id": procurementRequestID})
		}
	}
	documentID := cleanID(req.DocumentID)
	if documentID != "" {
		if !validActivityUUID(documentID) {
			return nil, validationError("document_id không hợp lệ", Record{"document_id": documentID})
		}
		document, err := s.repo.GetByID(ctx, "documents", documentID)
		if err != nil {
			return nil, validationError("Document liên kết hoá đơn không tồn tại", Record{"document_id": documentID})
		}
		if cleanID(document["project_id"]) != projectID || fmt.Sprint(document["status"]) != "active" {
			return nil, validationError("Document liên kết hoá đơn không thuộc công trình", Record{"document_id": documentID})
		}
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú hoá đơn không được vượt quá 2000 ký tự")
	}
	return Record{
		"project_id":             projectID,
		"payable_id":             cleanID(payable["id"]),
		"procurement_request_id": procurementRequestID,
		"document_id":            documentID,
		"invoice_number":         invoiceNumber,
		"invoice_symbol":         invoiceSymbol,
		"invoice_date":           invoiceDate,
		"vendor_name":            vendorName,
		"vendor_tax_code":        vendorTaxCode,
		"amount_before_vat":      req.AmountBeforeVAT,
		"vat_amount":             req.VATAmount,
		"total_amount":           totalAmount,
		"notes":                  notes,
	}, nil
}

func (s *Service) ReceivableOutputInvoices(ctx context.Context, receivableID, actorID string) ([]Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.view", "accounting.receivable_view"}, "Bạn không có quyền xem hoá đơn đầu ra")
	if err != nil {
		return nil, err
	}
	return s.repo.ListOutputInvoicesForReceivable(ctx, cleanID(receivable["id"]))
}

func (s *Service) ProjectOutputInvoices(ctx context.Context, projectID, actorID string) ([]Record, error) {
	projectID = cleanID(projectID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.view", "accounting.receivable_view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem hoá đơn đầu ra", Record{"permissions": []string{"accounting.view", "accounting.receivable_view"}})
	}
	return s.repo.ListOutputInvoicesForProject(ctx, projectID)
}

func (s *Service) CreateOutputInvoice(ctx context.Context, receivableID string, req OutputInvoiceRequest, fileHeader *multipart.FileHeader, actorID, actorName string) (Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.receivable_approve"}, "Bạn không có quyền upload hoá đơn đầu ra")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizeOutputInvoiceRequest(ctx, receivable, req)
	if err != nil {
		return nil, err
	}
	fileMeta, storedPath, originalName, err := s.storeOutputInvoicePDF(cleanID(receivable["project_id"]), cleanID(receivable["id"]), fileHeader)
	if err != nil {
		return nil, err
	}
	body["file_name"] = originalName
	record, err := s.repo.CreateOutputInvoice(ctx, receivable, body, fileMeta, actorID, actorName)
	if err != nil {
		_ = os.Remove(storedPath)
		return nil, err
	}
	return record, nil
}

func (s *Service) OutputInvoiceAction(ctx context.Context, invoiceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	invoice, err := s.repo.GetOutputInvoiceDetail(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(invoice["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.receivable_approve"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền cập nhật hoá đơn đầu ra", Record{"permissions": []string{"accounting.receivable_approve"}})
	}
	return s.repo.OutputInvoiceAction(ctx, cleanID(invoice["id"]), action, req, actorID, actorName)
}

func (s *Service) OutputInvoiceFile(ctx context.Context, invoiceID, actorID string) (EvidenceFileResult, error) {
	invoiceID = cleanID(invoiceID)
	if !validActivityUUID(invoiceID) {
		return EvidenceFileResult{}, validationError("invoice_id không hợp lệ", Record{"invoice_id": invoiceID})
	}
	invoice, err := s.repo.GetOutputInvoiceDetail(ctx, invoiceID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	projectID := cleanID(invoice["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.view", "accounting.receivable_view"})
	if err != nil {
		return EvidenceFileResult{}, err
	}
	if !allowed {
		return EvidenceFileResult{}, forbiddenError("Bạn không có quyền xem file hoá đơn đầu ra")
	}
	return s.storedFileResult(invoice, "storage_key", "mime_type", fmt.Sprint(invoice["file_name"]))
}

func (s *Service) requireReceivableAccess(ctx context.Context, receivableID, actorID string, permissions []string, message string) (Record, error) {
	receivableID = cleanID(receivableID)
	if !validActivityUUID(receivableID) {
		return nil, validationError("receivable_id không hợp lệ", Record{"receivable_id": receivableID})
	}
	receivable, err := s.repo.GetReceivableDetail(ctx, receivableID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(receivable["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, permissions)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError(message, Record{"permissions": permissions})
	}
	return receivable, nil
}

func (s *Service) normalizeOutputInvoiceRequest(ctx context.Context, receivable Record, req OutputInvoiceRequest) (Record, error) {
	projectID := cleanID(receivable["project_id"])
	invoiceNumber := strings.TrimSpace(req.InvoiceNumber)
	if invoiceNumber == "" {
		return nil, validationError("Số hoá đơn là bắt buộc")
	}
	if len([]rune(invoiceNumber)) > 120 {
		return nil, validationError("Số hoá đơn không được vượt quá 120 ký tự")
	}
	invoiceSymbol := strings.TrimSpace(req.InvoiceSymbol)
	if len([]rune(invoiceSymbol)) > 80 {
		return nil, validationError("Ký hiệu hoá đơn không được vượt quá 80 ký tự")
	}
	invoiceDate := strings.TrimSpace(req.InvoiceDate)
	if invoiceDate == "" {
		return nil, validationError("Ngày hoá đơn là bắt buộc")
	}
	if _, err := time.Parse("2006-01-02", invoiceDate); err != nil {
		return nil, validationError("Ngày hoá đơn không hợp lệ", Record{"invoice_date": invoiceDate})
	}
	clientName := strings.TrimSpace(req.ClientName)
	if clientName == "" {
		clientName = cleanOptionalText(receivable["client_name"])
	}
	if clientName == "" {
		return nil, validationError("Tên khách hàng trên hoá đơn là bắt buộc")
	}
	if len([]rune(clientName)) > 240 {
		return nil, validationError("Tên khách hàng trên hoá đơn không được vượt quá 240 ký tự")
	}
	clientTaxCode := strings.TrimSpace(req.ClientTaxCode)
	if len([]rune(clientTaxCode)) > 80 {
		return nil, validationError("Mã số thuế khách hàng không được vượt quá 80 ký tự")
	}
	if req.AmountBeforeVAT < 0 || req.VATAmount < 0 || req.TotalAmount < 0 {
		return nil, validationError("Số tiền hoá đơn không được âm")
	}
	totalAmount := req.TotalAmount
	if totalAmount == 0 && req.AmountBeforeVAT+req.VATAmount > 0 {
		totalAmount = req.AmountBeforeVAT + req.VATAmount
	}
	if totalAmount <= 0 {
		return nil, validationError("Tổng tiền hoá đơn phải lớn hơn 0")
	}
	if req.AmountBeforeVAT+req.VATAmount > 0 && totalAmount+1 < req.AmountBeforeVAT+req.VATAmount {
		return nil, validationError("Tổng tiền hoá đơn nhỏ hơn trước VAT cộng VAT", Record{
			"amount_before_vat": req.AmountBeforeVAT,
			"vat_amount":        req.VATAmount,
			"total_amount":      totalAmount,
		})
	}
	paymentID := cleanID(req.PaymentID)
	if paymentID != "" {
		if !validActivityUUID(paymentID) {
			return nil, validationError("payment_id không hợp lệ", Record{"payment_id": paymentID})
		}
		payment, err := s.repo.GetByID(ctx, "payments", paymentID)
		if err != nil {
			return nil, validationError("Phiếu thu liên kết hoá đơn không tồn tại", Record{"payment_id": paymentID})
		}
		if cleanID(payment["project_id"]) != projectID || fmt.Sprint(payment["status"]) == "cancelled" || fmt.Sprint(payment["payment_type"]) != "client_receipt" {
			return nil, validationError("Phiếu thu liên kết hoá đơn không thuộc công trình hoặc không phải client receipt", Record{"payment_id": paymentID})
		}
	}
	documentID := cleanID(req.DocumentID)
	if documentID != "" {
		if !validActivityUUID(documentID) {
			return nil, validationError("document_id không hợp lệ", Record{"document_id": documentID})
		}
		document, err := s.repo.GetByID(ctx, "documents", documentID)
		if err != nil {
			return nil, validationError("Document liên kết hoá đơn không tồn tại", Record{"document_id": documentID})
		}
		if cleanID(document["project_id"]) != projectID || fmt.Sprint(document["status"]) != "active" {
			return nil, validationError("Document liên kết hoá đơn không thuộc công trình", Record{"document_id": documentID})
		}
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú hoá đơn không được vượt quá 2000 ký tự")
	}
	return Record{
		"project_id":        projectID,
		"receivable_id":     cleanID(receivable["id"]),
		"payment_id":        paymentID,
		"document_id":       documentID,
		"invoice_number":    invoiceNumber,
		"invoice_symbol":    invoiceSymbol,
		"invoice_date":      invoiceDate,
		"client_name":       clientName,
		"client_tax_code":   clientTaxCode,
		"amount_before_vat": req.AmountBeforeVAT,
		"vat_amount":        req.VATAmount,
		"total_amount":      totalAmount,
		"notes":             notes,
	}, nil
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
