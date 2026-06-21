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
	formVersionID, err := s.repo.DefaultWorkflowFormVersionID(ctx, stepType, checklistRequired)
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
