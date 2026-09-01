package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strings"

	sharedstorage "consflow/backend/internal/shared/storage"

	"github.com/google/uuid"
)

func (s *Service) UploadProjectDocument(ctx context.Context, projectID, targetType, targetID, documentType, name string, fileHeader *multipart.FileHeader, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	targetType = strings.TrimSpace(targetType)
	targetID = cleanID(targetID)
	documentType = strings.TrimSpace(documentType)
	if !statusIn(targetType, "project_intake", "contract", "contract_dispatch", "contract_returned_scan") {
		return nil, validationError("Loại hồ sơ công trình không hợp lệ", Record{"target_type": targetType})
	}
	if targetType == "project_intake" {
		targetID = projectID
	} else if targetID == "" {
		return nil, validationError("Đối tượng liên kết hồ sơ là bắt buộc")
	}
	if targetType == "contract" || targetType == "contract_dispatch" || targetType == "contract_returned_scan" {
		contractID := targetID
		if targetType != "contract" {
			row, err := s.repo.QueryOne(ctx, `SELECT contract_id FROM contract_dispatches WHERE id=$1`, targetID)
			if err != nil {
				return nil, err
			}
			contractID = cleanID(row["contract_id"])
		}
		row, err := s.repo.QueryOne(ctx, `SELECT project_id FROM contracts WHERE id=$1 AND status <> 'terminated'`, contractID)
		if err != nil {
			return nil, err
		}
		if cleanID(row["project_id"]) != projectID {
			return nil, validationError("Hồ sơ không thuộc công trình của hợp đồng")
		}
	}
	file, storedObject, originalName, err := s.storeProjectDocumentUpload(ctx, projectID, targetType, targetID, fileHeader)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = originalName
	}
	document, err := s.repo.CreateProjectDocument(ctx, projectID, targetType, targetID, documentType, name, originalName, file, actorID, actorName)
	if err != nil {
		return nil, s.cleanupStoredObjectAfterError(storedObject, err)
	}
	return document, nil
}

func (s *Service) storeProjectDocumentUpload(ctx context.Context, projectID, targetType, targetID string, fileHeader *multipart.FileHeader) (StoredFileMetadata, storedObjectRef, string, error) {
	if fileHeader == nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", validationError("Tệp hồ sơ là bắt buộc")
	}
	if fileHeader.Size <= 0 {
		return StoredFileMetadata{}, storedObjectRef{}, "", validationError("Tệp hồ sơ không được rỗng")
	}
	if fileHeader.Size > s.maxUploadBytes {
		return StoredFileMetadata{}, storedObjectRef{}, "", validationError("Tệp hồ sơ vượt quá dung lượng cho phép", Record{"max_bytes": s.maxUploadBytes})
	}
	source, err := fileHeader.Open()
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", err
	}
	defer source.Close()
	header := make([]byte, 512)
	n, readErr := io.ReadFull(source, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return StoredFileMetadata{}, storedObjectRef{}, "", readErr
	}
	detectedMIME := http.DetectContentType(header[:n])
	spec, ok := allowedEvidenceUploadMIMEs[detectedMIME]
	if !ok {
		return StoredFileMetadata{}, storedObjectRef{}, "", validationError("Hồ sơ chỉ hỗ trợ PDF, PNG, JPG hoặc WebP", Record{"mime_type": detectedMIME})
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", fmt.Errorf("rewind project document upload: %w", err)
	}
	originalName := cleanUploadFilename(fileHeader.Filename)
	storageKey := path.Join("project-documents", projectID, targetType, targetID, uuid.NewString()+spec.Extension)
	registry, writeProvider, err := s.storageWriter()
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", err
	}
	object, err := registry.Put(ctx, writeProvider, storageKey, source, sharedstorage.PutOptions{ContentType: detectedMIME, ExpectedSize: fileHeader.Size})
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", err
	}
	return StoredFileMetadata{
		StorageProvider: object.Provider,
		StorageKey:      object.Key,
		MIMEType:        detectedMIME,
		FileSize:        object.Size,
		ChecksumSHA256:  object.ChecksumSHA256,
	}, storedObjectRef{Provider: object.Provider, Key: object.Key}, originalName, nil
}

func (s *Service) ProjectDocumentFile(ctx context.Context, documentID, actorID string) (EvidenceFileResult, error) {
	document, err := s.repo.GetByID(ctx, "documents", cleanID(documentID))
	if err != nil {
		return EvidenceFileResult{}, err
	}
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, cleanID(document["project_id"]), []string{"project.view", "contract.view"})
	if err != nil {
		return EvidenceFileResult{}, err
	}
	if !allowed {
		return EvidenceFileResult{}, forbiddenError("Bạn không có quyền xem tệp hồ sơ này")
	}
	provider := strings.TrimSpace(fmt.Sprint(document["storage_provider"]))
	storageKey := strings.TrimSpace(fmt.Sprint(document["storage_key"]))
	if !statusIn(provider, sharedstorage.LocalProvider, sharedstorage.S3Provider) || storageKey == "" || storageKey == "<nil>" {
		return EvidenceFileResult{}, validationError("Hồ sơ này chưa có tệp nội bộ được bảo vệ")
	}
	registry, err := s.storageBackend()
	if err != nil {
		return EvidenceFileResult{}, err
	}
	object, err := registry.Open(ctx, provider, storageKey)
	if err != nil {
		if errors.Is(err, sharedstorage.ErrNotFound) {
			return EvidenceFileResult{}, validationError("Tệp hồ sơ không còn tồn tại trong kho lưu trữ")
		}
		return EvidenceFileResult{}, err
	}
	mimeType := strings.TrimSpace(fmt.Sprint(document["mime_type"]))
	if mimeType == "" || mimeType == "<nil>" {
		mimeType = object.Info.ContentType
	}
	return EvidenceFileResult{Reader: object.Reader, Size: object.Info.Size, MIMEType: mimeType, Filename: strings.TrimSpace(fmt.Sprint(document["name"]))}, nil
}
