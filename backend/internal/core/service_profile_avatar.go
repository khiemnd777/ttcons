package core

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"path"

	"github.com/google/uuid"
)

func (s *Service) UpdateOwnAvatar(ctx context.Context, userID, actorName string, fileHeader *multipart.FileHeader) (Record, error) {
	userID = cleanID(userID)
	if !validActivityUUID(userID) {
		return nil, validationError("Tài khoản không hợp lệ")
	}
	avatar, storedObject, err := s.storeOwnAvatarUpload(ctx, userID, fileHeader)
	if err != nil {
		return nil, err
	}
	previous, err := s.repo.ReplaceOwnAvatar(ctx, userID, actorName, avatar)
	if err != nil {
		return nil, s.cleanupStoredObjectAfterError(storedObject, err)
	}
	// A replacement always has a new opaque key, so deleting the previously
	// committed object cannot remove the image that was just stored.
	s.deleteStoredObject(previous)
	return s.Me(ctx, userID)
}

func (s *Service) OwnAvatarFile(ctx context.Context, userID string) (EvidenceFileResult, error) {
	userID = cleanID(userID)
	if !validActivityUUID(userID) {
		return EvidenceFileResult{}, validationError("Tài khoản không hợp lệ")
	}
	avatar, err := s.repo.GetOwnAvatarStorage(ctx, userID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	return s.storedFileResult(ctx, avatar, "avatar_storage_provider", "avatar_storage_key", "avatar_mime_type", "anh-dai-dien.png")
}

func (s *Service) storeOwnAvatarUpload(ctx context.Context, userID string, fileHeader *multipart.FileHeader) (StoredFileMetadata, storedObjectRef, error) {
	uploadLimit := min(s.maxUploadBytes, maxProfileAvatarUploadBytes)
	if fileHeader == nil {
		return StoredFileMetadata{}, storedObjectRef{}, validationError("Ảnh đại diện là bắt buộc")
	}
	if fileHeader.Size <= 0 {
		return StoredFileMetadata{}, storedObjectRef{}, validationError("Ảnh đại diện không được rỗng")
	}
	if fileHeader.Size > uploadLimit {
		return StoredFileMetadata{}, storedObjectRef{}, validationError("Ảnh đại diện vượt quá dung lượng cho phép", Record{"max_bytes": uploadLimit})
	}
	source, err := fileHeader.Open()
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, err
	}
	defer source.Close()
	raw, err := io.ReadAll(io.LimitReader(source, uploadLimit+1))
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, err
	}
	if int64(len(raw)) > uploadLimit {
		return StoredFileMetadata{}, storedObjectRef{}, validationError("Ảnh đại diện vượt quá dung lượng cho phép", Record{"max_bytes": uploadLimit})
	}

	sourceMIME := http.DetectContentType(raw[:min(len(raw), 512)])
	if !statusIn(sourceMIME, "image/png", "image/jpeg", "image/webp") {
		return StoredFileMetadata{}, storedObjectRef{}, validationError("Ảnh đại diện chỉ hỗ trợ PNG, JPG hoặc WebP", Record{"mime_type": sourceMIME})
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, validationError("Không đọc được kích thước ảnh đại diện")
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > maxProfileAvatarImageDimension || config.Height > maxProfileAvatarImageDimension || int64(config.Width)*int64(config.Height) > maxProfileAvatarImagePixels {
		return StoredFileMetadata{}, storedObjectRef{}, validationError("Kích thước ảnh đại diện vượt quá giới hạn 2048 x 2048 px", Record{"width_px": config.Width, "height_px": config.Height})
	}
	decoded, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, validationError("Ảnh đại diện không hợp lệ")
	}
	var normalized bytes.Buffer
	if err := png.Encode(&normalized, decoded); err != nil {
		return StoredFileMetadata{}, storedObjectRef{}, fmt.Errorf("normalize profile avatar: %w", err)
	}
	if int64(normalized.Len()) > uploadLimit {
		return StoredFileMetadata{}, storedObjectRef{}, validationError("Ảnh đại diện sau chuẩn hóa vượt quá dung lượng cho phép", Record{"max_bytes": uploadLimit})
	}
	return s.storeGeneratedFile(ctx, path.Join("profile-avatars", userID, uuid.NewString()+".png"), normalized.Bytes(), "image/png")
}
