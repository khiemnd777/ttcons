package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	_ "image/jpeg"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	sharedstorage "consflow/backend/internal/shared/storage"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

type EvidenceFileResult struct {
	Reader   io.ReadCloser
	Size     int64
	MIMEType string
	Filename string
}

func (s *Service) storeInputInvoicePDF(ctx context.Context, projectID, payableID string, fileHeader *multipart.FileHeader) (StoredFileMetadata, storedObjectRef, string, error) {
	if fileHeader == nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", validationError("File PDF hoá đơn là bắt buộc")
	}
	if fileHeader.Size <= 0 {
		return StoredFileMetadata{}, storedObjectRef{}, "", validationError("File PDF hoá đơn không được rỗng")
	}
	if fileHeader.Size > s.maxUploadBytes {
		return StoredFileMetadata{}, storedObjectRef{}, "", validationError("File PDF hoá đơn vượt quá dung lượng cho phép", Record{"max_bytes": s.maxUploadBytes})
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
	if detectedMIME != "application/pdf" {
		return StoredFileMetadata{}, storedObjectRef{}, "", validationError("Hoá đơn đầu vào chỉ hỗ trợ PDF", Record{"mime_type": detectedMIME})
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", fmt.Errorf("rewind input invoice upload: %w", err)
	}
	originalName := cleanUploadFilename(fileHeader.Filename)
	if originalName == "minh-chung" || originalName == "evidence-file" {
		originalName = "input-invoice.pdf"
	}
	if filepath.Ext(originalName) == "" {
		originalName += ".pdf"
	}
	storageKey := path.Join("input-invoices", cleanID(projectID), cleanID(payableID), uuid.NewString()+".pdf")
	registry, writeProvider, err := s.storageWriter()
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", err
	}
	object, err := registry.Put(ctx, writeProvider, storageKey, source, sharedstorage.PutOptions{ContentType: "application/pdf", ExpectedSize: fileHeader.Size})
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", err
	}
	return StoredFileMetadata{
		StorageProvider: object.Provider,
		StorageKey:      object.Key,
		MIMEType:        "application/pdf",
		FileSize:        object.Size,
		ChecksumSHA256:  object.ChecksumSHA256,
	}, storedObjectRef{Provider: object.Provider, Key: object.Key}, originalName, nil
}

func (s *Service) storeOutputInvoicePDF(ctx context.Context, projectID, receivableID string, fileHeader *multipart.FileHeader) (StoredFileMetadata, storedObjectRef, string, error) {
	if fileHeader == nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", validationError("File PDF hoá đơn là bắt buộc")
	}
	if fileHeader.Size <= 0 {
		return StoredFileMetadata{}, storedObjectRef{}, "", validationError("File PDF hoá đơn không được rỗng")
	}
	if fileHeader.Size > s.maxUploadBytes {
		return StoredFileMetadata{}, storedObjectRef{}, "", validationError("File PDF hoá đơn vượt quá dung lượng cho phép", Record{"max_bytes": s.maxUploadBytes})
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
	if detectedMIME != "application/pdf" {
		return StoredFileMetadata{}, storedObjectRef{}, "", validationError("Hoá đơn đầu ra chỉ hỗ trợ PDF", Record{"mime_type": detectedMIME})
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", fmt.Errorf("rewind output invoice upload: %w", err)
	}
	originalName := cleanUploadFilename(fileHeader.Filename)
	if originalName == "minh-chung" || originalName == "evidence-file" {
		originalName = "output-invoice.pdf"
	}
	if filepath.Ext(originalName) == "" {
		originalName += ".pdf"
	}
	storageKey := path.Join("output-invoices", cleanID(projectID), cleanID(receivableID), uuid.NewString()+".pdf")
	registry, writeProvider, err := s.storageWriter()
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", err
	}
	object, err := registry.Put(ctx, writeProvider, storageKey, source, sharedstorage.PutOptions{ContentType: "application/pdf", ExpectedSize: fileHeader.Size})
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, "", err
	}
	return StoredFileMetadata{
		StorageProvider: object.Provider,
		StorageKey:      object.Key,
		MIMEType:        "application/pdf",
		FileSize:        object.Size,
		ChecksumSHA256:  object.ChecksumSHA256,
	}, storedObjectRef{Provider: object.Provider, Key: object.Key}, originalName, nil
}

type storedObjectRef struct {
	Provider string
	Key      string
}

func (s *Service) storeGeneratedFile(ctx context.Context, storageKey string, content []byte, mimeType string) (StoredFileMetadata, storedObjectRef, error) {
	if len(content) == 0 {
		return StoredFileMetadata{}, storedObjectRef{}, validationError("File generated không được rỗng")
	}
	registry, writeProvider, err := s.storageWriter()
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, err
	}
	object, err := registry.Put(ctx, writeProvider, storageKey, bytes.NewReader(content), sharedstorage.PutOptions{ContentType: mimeType, ExpectedSize: int64(len(content))})
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, err
	}
	return StoredFileMetadata{
		StorageProvider: object.Provider,
		StorageKey:      object.Key,
		MIMEType:        mimeType,
		FileSize:        object.Size,
		ChecksumSHA256:  object.ChecksumSHA256,
	}, storedObjectRef{Provider: object.Provider, Key: object.Key}, nil
}

func (s *Service) deleteStoredObject(object storedObjectRef) {
	if strings.TrimSpace(object.Provider) == "" || strings.TrimSpace(object.Key) == "" {
		return
	}
	registry, err := s.storageBackend()
	if err != nil {
		log.Printf("level=error component=storage operation=cleanup provider=%q key=%q orphan_risk=true error=%q", object.Provider, object.Key, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := registry.Delete(ctx, object.Provider, object.Key); err != nil {
		log.Printf("level=error component=storage operation=cleanup provider=%q key=%q orphan_risk=true error=%q", object.Provider, object.Key, err)
	}
}

func (s *Service) cleanupStoredObjectAfterError(object storedObjectRef, originalErr error) error {
	if originalErr == nil {
		return nil
	}
	if !storedMutationCleanupSafe(originalErr) {
		log.Printf("level=error component=storage operation=cleanup_skipped provider=%q key=%q commit_state=unknown orphan_risk=true error=%q", object.Provider, object.Key, originalErr)
		return originalErr
	}
	s.deleteStoredObject(object)
	return originalErr
}

func (s *Service) storedFileResult(ctx context.Context, record Record, providerField, storageKeyField, mimeField, filename string) (EvidenceFileResult, error) {
	provider := strings.TrimSpace(fmt.Sprint(record[providerField]))
	storageKey := strings.TrimSpace(fmt.Sprint(record[storageKeyField]))
	if provider == "" || provider == "<nil>" || storageKey == "" || storageKey == "<nil>" {
		return EvidenceFileResult{}, validationError("Tệp được bảo vệ chưa có khóa lưu trữ")
	}
	registry, err := s.storageBackend()
	if err != nil {
		return EvidenceFileResult{}, err
	}
	object, err := registry.Open(ctx, provider, storageKey)
	if err != nil {
		if errors.Is(err, sharedstorage.ErrNotFound) {
			return EvidenceFileResult{}, validationError("Tệp được bảo vệ không còn tồn tại trong kho lưu trữ")
		}
		return EvidenceFileResult{}, err
	}
	mimeType := strings.TrimSpace(fmt.Sprint(record[mimeField]))
	if mimeType == "" || mimeType == "<nil>" {
		mimeType = object.Info.ContentType
	}
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "application/octet-stream"
	}
	return EvidenceFileResult{Reader: object.Reader, Size: object.Info.Size, MIMEType: mimeType, Filename: cleanUploadFilename(filename)}, nil
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
		return nil, validationError("Cần nét ký tay trước khi ký biểu mẫu quy trình")
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
		return nil, validationError("Cần nét ký tay trước khi ký biểu mẫu quy trình")
	}
	if totalPoints > 2000 {
		return nil, validationError("Chữ ký có quá nhiều điểm", Record{"max_points": 2000})
	}
	return out, nil
}

func workflowFormPDFFileName(form Record, revisionNo int) string {
	instance, _ := form["instance"].(Record)
	title := cleanUploadFilename(fmt.Sprintf("%s-rev-%d.pdf", slugText(fmt.Sprint(instance["title"])), revisionNo))
	if title == "minh-chung" || title == "evidence-file" || title == ".pdf" {
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
