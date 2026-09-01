package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *Service) WorkItemForm(ctx context.Context, workItemID, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem biểu mẫu quy trình này")
	}
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{"available": false}, nil
	}
	if err != nil {
		return nil, err
	}
	form["available"] = true
	actorName := ""
	if actorID != "" {
		if actor, actorErr := s.repo.GetByID(ctx, "users", actorID); actorErr == nil {
			actorName = strings.TrimSpace(fmt.Sprint(actor["full_name"]))
		}
	}
	instance := recordFromAny(form["instance"])
	fields := workflowAnswerRecordList(form["fields"])
	if !statusIn(fmt.Sprint(instance["status"]), "submitted", "cancelled") {
		form["suggested_answers"] = workflowFormSuggestedAnswers(instance, fields, actorName, time.Now())
	}
	canAmend, err := s.canAmendWorkflowForm(ctx, form, actorID)
	if err != nil {
		return nil, err
	}
	form["capabilities"] = Record{"can_amend": canAmend}
	if len(instance) > 0 {
		runNodeID := cleanID(instance["run_node_id"])
		if runNodeID != "" {
			if node, nodeErr := s.repo.GetByID(ctx, "pipeline_run_nodes", runNodeID); nodeErr == nil {
				exitConditions := recordFromAny(node["exit_conditions"])
				form["auto_advance_when_ready"] = boolValue(exitConditions["auto_advance_when_ready"])
			}
		}
	}
	return form, nil
}

func (s *Service) SaveWorkflowFormDraft(ctx context.Context, instanceID string, req WorkflowFormSaveRequest, actorID, actorName string) (Record, error) {
	return s.saveWorkflowForm(ctx, instanceID, req, false, actorID, actorName)
}

func (s *Service) SubmitWorkflowForm(ctx context.Context, instanceID string, req WorkflowFormSaveRequest, actorID, actorName string) (Record, error) {
	return s.saveWorkflowForm(ctx, instanceID, req, true, actorID, actorName)
}

func (s *Service) AmendWorkflowForm(ctx context.Context, instanceID string, req WorkflowFormAmendRequest, actorID, actorName string) (Record, error) {
	instanceID = cleanID(instanceID)
	if !validActivityUUID(instanceID) {
		return nil, validationError("workflow_form_instance_id không hợp lệ", Record{"workflow_form_instance_id": instanceID})
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, validationError("Lý do chỉnh sửa hồ sơ nghiệp vụ là bắt buộc", Record{"field": "reason"})
	}
	if len([]rune(reason)) > 1000 {
		return nil, validationError("Lý do chỉnh sửa không được vượt quá 1000 ký tự", Record{"field": "reason", "max_length": 1000})
	}
	if req.ExpectedRevision <= 0 {
		return nil, validationError("Phiên bản hồ sơ cần chỉnh sửa không hợp lệ", Record{"expected_revision": req.ExpectedRevision})
	}
	form, err := s.repo.GetWorkflowFormInstanceDetail(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	instance := recordFromAny(form["instance"])
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Chỉ hồ sơ nghiệp vụ đã chốt mới được chỉnh sửa", Record{"status": instance["status"]})
	}
	canAmend, err := s.canAmendWorkflowForm(ctx, form, actorID)
	if err != nil {
		return nil, err
	}
	if !canAmend {
		return nil, forbiddenError("Bạn không có quyền chỉnh sửa hồ sơ nghiệp vụ này")
	}
	fields := workflowAnswerRecordList(form["fields"])
	answers := applyWorkflowFormSuggestedAnswers(instance, fields, req.Answers, actorName, time.Now())
	normalized, err := s.validateWorkflowFormAnswers(ctx, instance, fields, answers, true)
	if err != nil {
		return nil, err
	}
	updated, err := s.repo.AmendWorkflowForm(ctx, instanceID, normalized, reason, req.ExpectedRevision, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishWorkItemChanged(Record{"id": instance["work_item_id"], "project_id": instance["project_id"]}, actorID)
	return updated, nil
}

func (s *Service) canAmendWorkflowForm(ctx context.Context, form Record, actorID string) (bool, error) {
	instance := recordFromAny(form["instance"])
	if fmt.Sprint(instance["status"]) != "submitted" {
		return false, nil
	}
	workItemID := cleanID(instance["work_item_id"])
	if workItemID == "" {
		return false, nil
	}
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil || !allowed {
		return false, err
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return false, err
	}
	requirement := recordFromAny(item["evidence_requirement_json"])
	requiredPermission := strings.TrimSpace(fmt.Sprint(requirement["required_permission"]))
	if requiredPermission == "" || requiredPermission == "<nil>" {
		return true, nil
	}
	return s.repo.UserHasAnyProjectPermission(ctx, actorID, cleanID(item["project_id"]), []string{requiredPermission})
}

func (s *Service) SealAssets(ctx context.Context, includeArchived bool, actorID string) ([]Record, error) {
	allowed, err := s.repo.CanUsePermission(ctx, actorID, []string{"seal.use", "seal.manage"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem tài nguyên con dấu")
	}
	if includeArchived {
		canManage, err := s.repo.CanUsePermission(ctx, actorID, []string{"seal.manage"})
		if err != nil {
			return nil, err
		}
		if !canManage {
			return nil, forbiddenError("Bạn không có quyền xem con dấu đã lưu trữ")
		}
	}
	return s.repo.ListSealAssets(ctx, includeArchived)
}

func (s *Service) CreateSealAsset(ctx context.Context, name string, fileHeader *multipart.FileHeader, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanUsePermission(ctx, actorID, []string{"seal.manage"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền tải tài nguyên con dấu")
	}
	name = strings.TrimSpace(name)
	if name == "" && fileHeader != nil {
		originalName := cleanUploadFilename(fileHeader.Filename)
		name = strings.TrimSpace(strings.TrimSuffix(originalName, filepath.Ext(originalName)))
	}
	if name == "" {
		return nil, validationError("Tên con dấu là bắt buộc")
	}
	if len([]rune(name)) > 120 {
		return nil, validationError("Tên con dấu không được vượt quá 120 ký tự")
	}
	file, storedObject, originalName, sourceMIME, width, height, err := s.storeSealAssetUpload(ctx, fileHeader)
	if err != nil {
		return nil, err
	}
	asset, err := s.repo.CreateSealAsset(ctx, name, originalName, sourceMIME, file, width, height, actorID, actorName)
	if err != nil {
		return nil, s.cleanupStoredObjectAfterError(storedObject, err)
	}
	return asset, nil
}

func (s *Service) storeSealAssetUpload(ctx context.Context, fileHeader *multipart.FileHeader) (StoredFileMetadata, storedObjectRef, string, string, int, int, error) {
	uploadLimit := min(s.maxUploadBytes, maxSealUploadBytes)
	if fileHeader == nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", "", 0, 0, validationError("Tệp ảnh con dấu là bắt buộc")
	}
	if fileHeader.Size <= 0 {
		return StoredFileMetadata{}, storedObjectRef{}, "", "", 0, 0, validationError("Tệp ảnh con dấu không được rỗng")
	}
	if fileHeader.Size > uploadLimit {
		return StoredFileMetadata{}, storedObjectRef{}, "", "", 0, 0, validationError("Tệp ảnh con dấu vượt quá dung lượng cho phép", Record{"max_bytes": uploadLimit})
	}
	source, err := fileHeader.Open()
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", "", 0, 0, err
	}
	defer source.Close()
	raw, err := io.ReadAll(io.LimitReader(source, uploadLimit+1))
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", "", 0, 0, err
	}
	if int64(len(raw)) > uploadLimit {
		return StoredFileMetadata{}, storedObjectRef{}, "", "", 0, 0, validationError("Tệp ảnh con dấu vượt quá dung lượng cho phép", Record{"max_bytes": uploadLimit})
	}
	headerLength := min(len(raw), 512)
	sourceMIME := http.DetectContentType(raw[:headerLength])
	if !statusIn(sourceMIME, "image/png", "image/jpeg", "image/webp") {
		return StoredFileMetadata{}, storedObjectRef{}, "", "", 0, 0, validationError("Con dấu chỉ hỗ trợ PNG, JPG hoặc WebP", Record{"mime_type": sourceMIME})
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", "", 0, 0, validationError("Không đọc được kích thước ảnh con dấu")
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > maxSealImageDimension || config.Height > maxSealImageDimension || int64(config.Width)*int64(config.Height) > maxSealImagePixels {
		return StoredFileMetadata{}, storedObjectRef{}, "", "", 0, 0, validationError("Kích thước ảnh con dấu vượt quá giới hạn 4096 x 4096 px", Record{"width_px": config.Width, "height_px": config.Height})
	}
	decoded, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", "", 0, 0, validationError("Ảnh con dấu không hợp lệ")
	}
	var normalized bytes.Buffer
	if err := png.Encode(&normalized, decoded); err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", "", 0, 0, fmt.Errorf("normalize seal image: %w", err)
	}
	if int64(normalized.Len()) > uploadLimit {
		return StoredFileMetadata{}, storedObjectRef{}, "", "", 0, 0, validationError("Ảnh con dấu sau chuẩn hóa vượt quá dung lượng cho phép", Record{"max_bytes": uploadLimit})
	}
	storageKey := path.Join("seal-assets", uuid.NewString()+".png")
	file, storedObject, err := s.storeGeneratedFile(ctx, storageKey, normalized.Bytes(), "image/png")
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", "", 0, 0, err
	}
	return file, storedObject, cleanUploadFilename(fileHeader.Filename), sourceMIME, config.Width, config.Height, nil
}

func (s *Service) ArchiveSealAsset(ctx context.Context, sealAssetID, actorID, actorName string) (Record, error) {
	sealAssetID = cleanID(sealAssetID)
	if !validActivityUUID(sealAssetID) {
		return nil, validationError("seal_asset_id không hợp lệ", Record{"seal_asset_id": sealAssetID})
	}
	allowed, err := s.repo.CanUsePermission(ctx, actorID, []string{"seal.manage"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền lưu trữ tài nguyên con dấu")
	}
	return s.repo.ArchiveSealAsset(ctx, sealAssetID, actorID, actorName)
}

func (s *Service) SealAssetFile(ctx context.Context, sealAssetID, actorID string) (EvidenceFileResult, error) {
	sealAssetID = cleanID(sealAssetID)
	if !validActivityUUID(sealAssetID) {
		return EvidenceFileResult{}, validationError("seal_asset_id không hợp lệ", Record{"seal_asset_id": sealAssetID})
	}
	asset, err := s.repo.GetSealAsset(ctx, sealAssetID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	allowed, err := s.repo.CanUsePermission(ctx, actorID, []string{"seal.use", "seal.manage"})
	if err != nil {
		return EvidenceFileResult{}, err
	}
	if !allowed {
		return EvidenceFileResult{}, forbiddenError("Bạn không có quyền xem tài nguyên con dấu")
	}
	if fmt.Sprint(asset["status"]) == "archived" {
		canManage, err := s.repo.CanUsePermission(ctx, actorID, []string{"seal.manage"})
		if err != nil {
			return EvidenceFileResult{}, err
		}
		if !canManage {
			return EvidenceFileResult{}, forbiddenError("Con dấu này đã được lưu trữ")
		}
	}
	return s.storedFileResult(ctx, asset, "storage_provider", "storage_key", "mime_type", sealAssetFilename(asset))
}

func sealAssetFilename(asset Record) string {
	name := strings.TrimSpace(fmt.Sprint(asset["name"]))
	if name == "" || name == "<nil>" {
		name = "con-dau"
	}
	return cleanUploadFilename(name + ".png")
}

func (s *Service) ApplyWorkflowFormSeal(ctx context.Context, instanceID string, req WorkflowFormSealApplyRequest, actorID, actorName string) (Record, error) {
	instanceID = cleanID(instanceID)
	if !validActivityUUID(instanceID) {
		return nil, validationError("workflow_form_instance_id không hợp lệ", Record{"workflow_form_instance_id": instanceID})
	}
	sealAssetID := cleanID(req.SealAssetID)
	if !validActivityUUID(sealAssetID) {
		return nil, validationError("seal_asset_id không hợp lệ", Record{"seal_asset_id": req.SealAssetID})
	}
	purpose := strings.TrimSpace(req.Purpose)
	if purpose == "" {
		purpose = "internal_ack"
	}
	if !validWorkflowSignaturePurpose(purpose) {
		return nil, validationError("Mục đích xác nhận bằng con dấu không hợp lệ", Record{"purpose": purpose})
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú con dấu không được vượt quá 1000 ký tự")
	}
	allowed, err := s.repo.CanUsePermission(ctx, actorID, []string{"seal.use"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền sử dụng con dấu công ty")
	}
	form, err := s.repo.GetWorkflowFormInstanceDetail(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Biểu mẫu quy trình phải được chốt trước khi đóng dấu", Record{"status": instance["status"]})
	}
	workItemID := cleanID(instance["work_item_id"])
	if workItemID == "" {
		return nil, validationError("Biểu mẫu quy trình chưa liên kết đầu việc", Record{"workflow_form_instance_id": instanceID})
	}
	canAct, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canAct {
		return nil, forbiddenError("Bạn không có quyền đóng dấu hồ sơ quy trình này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkflowFormArtifactScope(ctx, item, instance, actorID); err != nil {
		return nil, err
	}
	return s.repo.ApplyWorkflowFormSeal(ctx, instanceID, sealAssetID, purpose, comment, actorID, actorName)
}

func (s *Service) WorkflowFormSealApplicationVisual(ctx context.Context, applicationID, actorID string) (EvidenceFileResult, error) {
	applicationID = cleanID(applicationID)
	if !validActivityUUID(applicationID) {
		return EvidenceFileResult{}, validationError("seal_application_id không hợp lệ", Record{"seal_application_id": applicationID})
	}
	application, err := s.repo.GetWorkflowFormSealApplicationDetail(ctx, applicationID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	allowed, err := s.repo.CanViewWorkItem(ctx, cleanID(application["work_item_id"]), actorID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	if !allowed {
		return EvidenceFileResult{}, forbiddenError("Bạn không có quyền xem con dấu trên hồ sơ này")
	}
	return s.storedFileResult(ctx, application, "storage_provider", "storage_key", "mime_type", sealAssetFilename(application))
}

func (s *Service) workflowFormSealImages(ctx context.Context, form Record) (map[string][]byte, error) {
	applications := workflowAnswerRecordList(form["seal_applications"])
	images := make(map[string][]byte, len(applications))
	if len(applications) == 0 {
		return images, nil
	}
	registry, err := s.storageBackend()
	if err != nil {
		return nil, err
	}
	for _, application := range applications {
		applicationID := cleanID(application["id"])
		provider := strings.TrimSpace(fmt.Sprint(application["storage_provider"]))
		storageKey := strings.TrimSpace(fmt.Sprint(application["storage_key"]))
		if applicationID == "" || provider == "" || storageKey == "" {
			return nil, validationError("Bản ghi con dấu thiếu thông tin tệp bảo vệ", Record{"seal_application_id": applicationID})
		}
		object, err := registry.Open(ctx, provider, storageKey)
		if err != nil {
			return nil, err
		}
		content, readErr := io.ReadAll(io.LimitReader(object.Reader, s.maxUploadBytes+1))
		closeErr := object.Reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if int64(len(content)) > s.maxUploadBytes {
			return nil, validationError("Ảnh con dấu vượt quá dung lượng tạo PDF", Record{"seal_application_id": applicationID})
		}
		checksum := sha256.Sum256(content)
		if expected := strings.TrimSpace(fmt.Sprint(application["checksum_sha256"])); expected != "" && expected != "<nil>" && hex.EncodeToString(checksum[:]) != expected {
			return nil, validationError("Checksum ảnh con dấu không khớp", Record{"seal_application_id": applicationID})
		}
		images[applicationID] = content
	}
	return images, nil
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
		return nil, validationError("Mục đích ký biểu mẫu quy trình không hợp lệ", Record{"purpose": purpose})
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
		return nil, validationError("Biểu mẫu quy trình phải được gửi trước khi ký nội bộ", Record{"status": instance["status"]})
	}
	workItemID := cleanID(instance["work_item_id"])
	if workItemID == "" {
		return nil, validationError("Biểu mẫu quy trình chưa liên kết đầu việc", Record{"workflow_form_instance_id": instanceID})
	}
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền ký biểu mẫu quy trình này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkflowFormArtifactScope(ctx, item, instance, actorID); err != nil {
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
	visualFile, storedObject, err := s.storeGeneratedFile(ctx, storageKey, imageBytes, imageMIME)
	if err != nil {
		return nil, err
	}
	signedForm, err := s.repo.SignWorkflowForm(ctx, instanceID, purpose, comment, visualFile, strokes, actorID, actorName)
	if err != nil {
		return nil, s.cleanupStoredObjectAfterError(storedObject, err)
	}
	return signedForm, nil
}

func validWorkflowSignaturePurpose(purpose string) bool {
	return statusIn(purpose, "internal_ack", "prepared_by", "reviewed_by", "approved_by", "received_by")
}

func (s *Service) SaveWorkflowFormPDFConfirmationPreference(ctx context.Context, instanceID string, req WorkflowFormPDFConfirmationPreferenceRequest, actorID, actorName string) (Record, error) {
	instanceID = cleanID(instanceID)
	if !validActivityUUID(instanceID) {
		return nil, validationError("workflow_form_instance_id không hợp lệ", Record{"workflow_form_instance_id": instanceID})
	}
	purpose := strings.TrimSpace(req.Purpose)
	if !validWorkflowSignaturePurpose(purpose) {
		return nil, validationError("Mục đích xác nhận hiển thị trong PDF không hợp lệ", Record{"purpose": purpose})
	}
	if !req.IncludeSignatures && !req.IncludeSeals {
		return nil, validationError("Mỗi mục đích phải hiển thị ít nhất chữ ký cá nhân hoặc con dấu công ty trong PDF", Record{"purpose": purpose})
	}
	form, err := s.repo.GetWorkflowFormInstanceDetail(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Biểu mẫu quy trình phải được chốt trước khi chọn xác nhận hiển thị trong PDF", Record{"status": instance["status"]})
	}
	currentRevision := int(number(instance["current_revision"]))
	hasSignature := false
	for _, signature := range workflowAnswerRecordList(form["signatures"]) {
		if int(number(signature["form_revision_no"])) == currentRevision && strings.TrimSpace(fmt.Sprint(signature["purpose"])) == purpose {
			hasSignature = true
			break
		}
	}
	hasSeal := false
	for _, application := range workflowAnswerRecordList(form["seal_applications"]) {
		if int(number(application["form_revision_no"])) == currentRevision && strings.TrimSpace(fmt.Sprint(application["purpose"])) == purpose {
			hasSeal = true
			break
		}
	}
	if !hasSignature || !hasSeal {
		return nil, validationError("Chỉ chọn thành phần PDF khi cùng mục đích đã có cả chữ ký cá nhân và con dấu công ty", Record{
			"purpose":        purpose,
			"has_signatures": hasSignature,
			"has_seals":      hasSeal,
		})
	}
	workItemID := cleanID(instance["work_item_id"])
	if workItemID == "" {
		return nil, validationError("Biểu mẫu quy trình chưa liên kết đầu việc", Record{"workflow_form_instance_id": instanceID})
	}
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền chọn xác nhận hiển thị trong PDF của hồ sơ này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkflowFormArtifactScope(ctx, item, instance, actorID); err != nil {
		return nil, err
	}
	return s.repo.SaveWorkflowFormPDFConfirmationPreference(ctx, instanceID, purpose, req.IncludeSignatures, req.IncludeSeals, actorID, actorName)
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
		return nil, validationError("Biểu mẫu quy trình phải được gửi trước khi xuất PDF", Record{"status": instance["status"]})
	}
	workItemID := cleanID(instance["work_item_id"])
	projectID := cleanID(instance["project_id"])
	if workItemID == "" || projectID == "" {
		return nil, validationError("Biểu mẫu quy trình chưa liên kết đầu việc", Record{"workflow_form_instance_id": instanceID})
	}
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xuất PDF biểu mẫu quy trình này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkflowFormArtifactScope(ctx, item, instance, actorID); err != nil {
		return nil, err
	}
	pdfForm, confirmationSelection := selectWorkflowFormPDFConfirmations(form)
	sealImages, err := s.workflowFormSealImages(ctx, pdfForm)
	if err != nil {
		return nil, err
	}
	pdfBytes, payloadHash, err := GenerateWorkflowFormPDFWithContext(pdfForm, WorkflowFormPDFContext{
		GeneratedAt:   time.Now(),
		GeneratedBy:   actorName,
		ProjectCode:   strings.TrimSpace(fmt.Sprint(item["project_code"])),
		ProjectName:   strings.TrimSpace(fmt.Sprint(item["project_name"])),
		NodeName:      strings.TrimSpace(fmt.Sprint(item["run_node_name"])),
		NodeOrder:     int(number(item["run_node_order_index"])),
		StepName:      strings.TrimSpace(fmt.Sprint(item["run_step_name"])),
		StepOrder:     int(number(item["run_step_order_index"])),
		WorkItemTitle: strings.TrimSpace(fmt.Sprint(item["title"])),
		SealImages:    sealImages,
	})
	if err != nil {
		return nil, err
	}
	revisionNo := int(number(instance["current_revision"]))
	fileName := workflowFormPDFFileName(pdfForm, revisionNo)
	storageKey := path.Join("workflow-form-pdfs", projectID, instanceID, uuid.NewString()+".pdf")
	pdfFile, storedObject, err := s.storeGeneratedFile(ctx, storageKey, pdfBytes, "application/pdf")
	if err != nil {
		return nil, err
	}
	exportedForm, err := s.repo.CreateWorkflowFormPDFExport(ctx, instanceID, fileName, fmt.Sprint(instance["title"]), pdfFile, payloadHash, confirmationSelection, actorID, actorName)
	if err != nil {
		return nil, s.cleanupStoredObjectAfterError(storedObject, err)
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
	return s.storedFileResult(ctx, signature, "visual_storage_provider", "visual_storage_key", "visual_mime_type", signatureVisualFilename(signature))
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
		return EvidenceFileResult{}, forbiddenError("Bạn không có quyền xem PDF biểu mẫu quy trình này")
	}
	return s.storedFileResult(ctx, export, "storage_provider", "storage_key", "mime_type", fmt.Sprint(export["file_name"]))
}

func (s *Service) DeleteWorkflowFormPDFExport(ctx context.Context, exportID, actorID, actorName string) (Record, error) {
	exportID = cleanID(exportID)
	if !validActivityUUID(exportID) {
		return nil, validationError("pdf_export_id không hợp lệ", Record{"pdf_export_id": exportID})
	}
	export, err := s.repo.GetWorkflowFormPDFExportForDeletion(ctx, exportID)
	if err != nil {
		return nil, err
	}
	workItemID := cleanID(export["work_item_id"])
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xoá PDF biểu mẫu quy trình này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}

	prepared, err := s.repo.PrepareWorkflowFormPDFExportDeletion(ctx, exportID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	instanceID := cleanID(prepared["form_instance_id"])
	if prepared["already_deleted"] == true || fmt.Sprint(prepared["deletion_status"]) == "deleted" {
		return s.repo.GetWorkflowFormInstanceDetail(ctx, instanceID)
	}

	provider := strings.TrimSpace(fmt.Sprint(prepared["storage_provider"]))
	storageKey := strings.TrimSpace(fmt.Sprint(prepared["storage_key"]))
	registry, err := s.storageBackend()
	if err == nil {
		err = registry.Delete(ctx, provider, storageKey)
	}
	if err != nil {
		if markErr := s.repo.MarkWorkflowFormPDFExportDeletionFailed(ctx, exportID, boundedStorageError(err)); markErr != nil {
			log.Printf("level=error component=storage operation=delete_pdf_mark_failed pdf_export_id=%q error=%q", exportID, markErr)
		}
		log.Printf("level=error component=storage operation=delete_workflow_form_pdf provider=%q key=%q pdf_export_id=%q error=%q", provider, storageKey, exportID, err)
		return nil, errors.New("Không thể xoá tệp PDF khỏi kho lưu trữ")
	}
	return s.repo.CompleteWorkflowFormPDFExportDeletion(ctx, exportID, actorID, actorName)
}

func boundedStorageError(err error) string {
	if err == nil {
		return ""
	}
	value := []rune(err.Error())
	if len(value) > 2000 {
		value = value[:2000]
	}
	return string(value)
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
		return nil, validationError("Biểu mẫu quy trình chưa liên kết đầu việc", Record{"workflow_form_instance_id": instanceID})
	}
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thao tác biểu mẫu quy trình này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	if submit && statusIn(fmt.Sprint(instance["status"]), "submitted", "cancelled") {
		return nil, validationError("Trạng thái biểu mẫu quy trình không cho phép gửi", Record{"status": instance["status"]})
	}
	if !submit && statusIn(fmt.Sprint(instance["status"]), "submitted", "cancelled") {
		return nil, validationError("Trạng thái biểu mẫu quy trình không cho phép lưu nháp", Record{"status": instance["status"]})
	}
	fields, _ := form["fields"].([]Record)
	answers := applyWorkflowFormSuggestedAnswers(instance, fields, req.Answers, actorName, time.Now())
	normalized, err := s.validateWorkflowFormAnswers(ctx, instance, fields, answers, submit)
	if err != nil {
		return nil, err
	}
	saved, err := s.repo.SaveWorkflowForm(ctx, instanceID, normalized, strings.TrimSpace(req.Comment), submit, actorID, actorName)
	if err != nil {
		return saved, err
	}
	if statusIn(fmt.Sprint(item["status"]), "ready", "needs_changes") {
		if startedItem, detailErr := s.repo.GetWorkItemDetail(ctx, workItemID); detailErr == nil && fmt.Sprint(startedItem["status"]) == "in_progress" {
			s.publishWorkItemChanged(startedItem, actorID)
		}
	}
	if !submit {
		return saved, err
	}
	saved["auto_transition"] = s.autoAdvanceSubmittedWorkflowForm(ctx, instance, actorID, actorName)
	return saved, nil
}

func (s *Service) autoAdvanceSubmittedWorkflowForm(ctx context.Context, instance Record, actorID, actorName string) Record {
	runNodeID := cleanID(instance["run_node_id"])
	workItemID := cleanID(instance["work_item_id"])
	if runNodeID == "" || workItemID == "" {
		return Record{"status": "not_configured"}
	}
	node, err := s.repo.GetByID(ctx, "pipeline_run_nodes", runNodeID)
	if err != nil {
		return Record{"status": "blocked", "message": err.Error()}
	}
	exitConditions := recordFromAny(node["exit_conditions"])
	if !boolValue(exitConditions["auto_advance_when_ready"]) {
		return Record{"status": "not_configured"}
	}
	if _, err := s.WorkItemAction(ctx, workItemID, "complete", WorkItemActionRequest{}, actorID, actorName); err != nil {
		return Record{"status": "blocked", "message": err.Error()}
	}
	run, err := s.AdvanceRunNode(ctx, runNodeID, actorID, actorName)
	if err != nil {
		return Record{"status": "blocked", "message": err.Error()}
	}
	if fmt.Sprint(run["status"]) == "blocked" {
		return Record{"status": "blocked", "message": run["block_reason"]}
	}
	return Record{"status": "advanced", "current_node_id": run["current_node_id"]}
}
