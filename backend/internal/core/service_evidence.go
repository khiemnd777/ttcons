package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path"
	"path/filepath"
	"strings"

	sharedstorage "consflow/backend/internal/shared/storage"

	"github.com/google/uuid"
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

func (s *Service) AddWorkItemEvidence(ctx context.Context, workItemID string, req WorkItemEvidenceRequest, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thêm minh chứng cho đầu việc này")
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
	normalized, err := validateWorkItemEvidenceRequest(req)
	if err != nil {
		return nil, err
	}
	if normalized.StorageKey != "" {
		return nil, validationError("Storage minh chứng nội bộ chỉ được tạo qua endpoint tải tệp")
	}
	evidence, err := s.repo.AddWorkItemEvidence(ctx, item, normalized, actorID, actorName)
	if err == nil {
		s.publishDiscussionWorkUpdate(cleanID(item["project_id"]), workItemID, actorID)
	}
	return evidence, err
}

func (s *Service) AddWorkItemEvidenceUpload(ctx context.Context, workItemID string, req WorkItemEvidenceRequest, fileHeader *multipart.FileHeader, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thêm minh chứng cho đầu việc này")
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
	storedObject, normalized, err := s.storeEvidenceUpload(ctx, item, req, fileHeader)
	if err != nil {
		return nil, err
	}
	evidence, err := s.repo.AddWorkItemEvidence(ctx, item, normalized, actorID, actorName)
	if err != nil {
		return nil, s.cleanupStoredObjectAfterError(storedObject, err)
	}
	s.publishDiscussionWorkUpdate(cleanID(item["project_id"]), workItemID, actorID)
	return evidence, nil
}

func (s *Service) EvidenceDetail(ctx context.Context, evidenceID, actorID string) (Record, error) {
	evidenceID = cleanID(evidenceID)
	if !validActivityUUID(evidenceID) {
		return nil, validationError("Mã minh chứng không hợp lệ", Record{"evidence_id": evidenceID})
	}
	evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
	if err != nil {
		return nil, err
	}
	allowed, err := s.repo.CanViewWorkItem(ctx, cleanID(evidence["work_item_id"]), actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem tệp minh chứng này")
	}
	return evidence, nil
}

func (s *Service) EvidenceFile(ctx context.Context, evidenceID, actorID string) (EvidenceFileResult, error) {
	evidence, err := s.EvidenceDetail(ctx, evidenceID, actorID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	provider := strings.TrimSpace(fmt.Sprint(evidence["storage_provider"]))
	storageKey := strings.TrimSpace(fmt.Sprint(evidence["storage_key"]))
	if provider == "" || provider == "<nil>" || storageKey == "" || storageKey == "<nil>" {
		return EvidenceFileResult{}, validationError("Minh chứng này chưa có tệp nội bộ được bảo vệ", Record{"evidence_id": evidenceID})
	}
	registry, err := s.storageBackend()
	if err != nil {
		return EvidenceFileResult{}, err
	}
	object, err := registry.Open(ctx, provider, storageKey)
	if err != nil {
		if errors.Is(err, sharedstorage.ErrNotFound) {
			return EvidenceFileResult{}, validationError("Tệp minh chứng không còn tồn tại trong kho lưu trữ", Record{"evidence_id": evidenceID})
		}
		return EvidenceFileResult{}, err
	}
	mimeType := strings.TrimSpace(fmt.Sprint(evidence["mime_type"]))
	if mimeType == "" || mimeType == "<nil>" {
		mimeType = object.Info.ContentType
	}
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "application/octet-stream"
	}
	return EvidenceFileResult{
		Reader:   object.Reader,
		Size:     object.Info.Size,
		MIMEType: mimeType,
		Filename: evidenceDownloadName(evidence),
	}, nil
}

func (s *Service) storeEvidenceUpload(ctx context.Context, item Record, req WorkItemEvidenceRequest, fileHeader *multipart.FileHeader) (storedObjectRef, WorkItemEvidenceRequest, error) {
	if fileHeader == nil {
		return storedObjectRef{}, req, validationError("Tệp minh chứng là bắt buộc")
	}
	if fileHeader.Size <= 0 {
		return storedObjectRef{}, req, validationError("Tệp minh chứng không được rỗng")
	}
	if fileHeader.Size > s.maxUploadBytes {
		return storedObjectRef{}, req, validationError("Tệp minh chứng vượt quá dung lượng cho phép", Record{"max_bytes": s.maxUploadBytes})
	}
	source, err := fileHeader.Open()
	if err != nil {
		return storedObjectRef{}, req, err
	}
	defer source.Close()

	spec, err := detectEvidenceUploadFile(source, fileHeader.Size, fileHeader.Filename)
	if err != nil {
		return storedObjectRef{}, req, err
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return storedObjectRef{}, req, fmt.Errorf("rewind evidence upload: %w", err)
	}
	detectedMIME := spec.MIMEType
	requestedType := strings.TrimSpace(req.EvidenceType)
	if requestedType != "" && requestedType != spec.EvidenceType {
		return storedObjectRef{}, req, validationError("Loại minh chứng không khớp với định dạng tệp", Record{"evidence_type": requestedType, "mime_type": detectedMIME})
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
	registry, writeProvider, err := s.storageWriter()
	if err != nil {
		return storedObjectRef{}, req, err
	}
	req.StorageProvider = writeProvider
	req.StorageKey = path.Join("evidence", cleanID(item["project_id"]), cleanID(item["id"]), uuid.NewString()+spec.Extension)
	if req.Payload == nil {
		req.Payload = Record{}
	}
	req.Payload["original_filename"] = originalName
	req.Payload["detected_mime_type"] = detectedMIME

	normalized, err := validateWorkItemEvidenceRequestForStorage(req, true)
	if err != nil {
		return storedObjectRef{}, req, err
	}
	object, err := registry.Put(ctx, normalized.StorageProvider, normalized.StorageKey, source, sharedstorage.PutOptions{
		ContentType:  detectedMIME,
		ExpectedSize: fileHeader.Size,
	})
	if err != nil {
		return storedObjectRef{}, normalized, err
	}
	normalized.FileSize = object.Size
	normalized.ChecksumSHA256 = object.ChecksumSHA256
	return storedObjectRef{Provider: object.Provider, Key: object.Key}, normalized, nil
}

func (s *Service) EvidenceAction(ctx context.Context, evidenceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	if action != "accept" && action != "reject" {
		return nil, validationError("Hành động minh chứng không hợp lệ", Record{"action": action})
	}
	evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
	if err != nil {
		return nil, err
	}
	item, err := s.repo.GetWorkItemDetail(ctx, fmt.Sprint(evidence["work_item_id"]))
	if err != nil {
		return nil, err
	}
	if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
		return nil, err
	}
	allowed, err := s.repo.CanDecideWorkItem(ctx, item, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền rà soát minh chứng này")
	}
	return s.repo.EvidenceAction(ctx, evidence, item, action, req, actorID, actorName)
}

func validateWorkItemEvidenceRequest(req WorkItemEvidenceRequest) (WorkItemEvidenceRequest, error) {
	return validateWorkItemEvidenceRequestForStorage(req, false)
}

func validateWorkItemEvidenceRequestForStorage(req WorkItemEvidenceRequest, allowS3 bool) (WorkItemEvidenceRequest, error) {
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
	// Evidence is an internal operational record. Keep the column for historical
	// compatibility, but do not allow new writes to select a broader audience.
	req.Visibility = "internal"
	if req.StorageProvider == "" {
		req.StorageProvider = "external"
	}
	if req.Payload == nil {
		req.Payload = Record{}
	}
	validType := map[string]bool{"document": true, "photo": true, "checklist": true, "link": true, "note": true, "other": true}
	if !validType[req.EvidenceType] {
		return req, validationError("Loại minh chứng không hợp lệ", Record{"evidence_type": req.EvidenceType})
	}
	validStorageProvider := map[string]bool{"external": true, sharedstorage.LocalProvider: true}
	if allowS3 {
		validStorageProvider[sharedstorage.S3Provider] = true
	}
	if !validStorageProvider[req.StorageProvider] {
		return req, validationError("Nơi lưu minh chứng không hợp lệ", Record{"storage_provider": req.StorageProvider})
	}
	if req.StorageKey != "" && req.StorageProvider != sharedstorage.LocalProvider && (!allowS3 || req.StorageProvider != sharedstorage.S3Provider) {
		return req, validationError("Khóa lưu trữ chỉ hợp lệ với kho lưu trữ nội bộ", Record{"storage_provider": req.StorageProvider})
	}
	if req.ChecksumSHA256 != "" && !validSHA256Hex(req.ChecksumSHA256) {
		return req, validationError("Checksum minh chứng không hợp lệ", Record{"checksum_sha256": req.ChecksumSHA256})
	}
	if req.Name == "" {
		req.Name = req.Title
	}
	if req.Name == "" && req.Caption != "" {
		req.Name = req.Caption
	}
	if req.Name == "" {
		return req, validationError("Tên minh chứng là bắt buộc")
	}
	if req.FileSize < 0 {
		return req, validationError("Kích thước file không hợp lệ")
	}
	if statusIn(req.EvidenceType, "document", "photo", "link") && req.FileURL == "" && req.DocumentID == "" && req.StorageKey == "" {
		return req, validationError("Minh chứng dạng tài liệu/ảnh/liên kết cần URL tệp, tài liệu liên kết hoặc tệp tải lên", Record{"evidence_type": req.EvidenceType})
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

func cleanUploadFilename(filename string) string {
	name := filepath.Base(strings.TrimSpace(filename))
	if name == "" || name == "." || name == "<nil>" || name == string(filepath.Separator) {
		return "minh-chung"
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
