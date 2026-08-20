package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"consflow/backend/internal/shared/config"
)

func readSystemSettingsBackupUpload(fileHeader *multipart.FileHeader) ([]byte, error) {
	if fileHeader == nil {
		return nil, validationError("Tệp JSON bản sao cấu hình là bắt buộc")
	}
	if !strings.EqualFold(filepath.Ext(strings.TrimSpace(fileHeader.Filename)), ".json") {
		return nil, validationError("Chỉ chấp nhận tệp cấu hình có phần mở rộng .json")
	}
	if fileHeader.Size > maxSystemSettingsBackupBytes {
		return nil, validationError("Bản sao cấu hình không được vượt quá 256 KB")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return nil, validationError("Không đọc được tệp bản sao cấu hình")
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxSystemSettingsBackupBytes+1))
	if err != nil {
		return nil, validationError("Không đọc được tệp bản sao cấu hình")
	}
	if len(raw) > maxSystemSettingsBackupBytes {
		return nil, validationError("Bản sao cấu hình không được vượt quá 256 KB")
	}
	return raw, nil
}

const (
	systemSettingsBackupFormat        = "consflow-system-settings"
	systemSettingsBackupFormatVersion = 1
	maxSystemSettingsBackupBytes      = 256 * 1024
	maxSystemSettingsBackupEntries    = 256
	maxSystemSettingsBackupKeyLength  = 160
)

type systemSettingsBackupDocument struct {
	Format              string                       `json:"format"`
	FormatVersion       int                          `json:"format_version"`
	ExportedAt          string                       `json:"exported_at"`
	SourceEnvironment   string                       `json:"source_environment"`
	RegistryFingerprint string                       `json:"registry_fingerprint"`
	Settings            []systemSettingsBackupEntry  `json:"settings"`
	ExcludedSecrets     []systemSettingsBackupSecret `json:"excluded_secrets"`
}

type systemSettingsBackupEntry struct {
	Key       string `json:"key"`
	ValueType string `json:"value_type"`
	Value     any    `json:"value"`
}

type systemSettingsBackupSecret struct {
	Key        string `json:"key"`
	Configured bool   `json:"configured"`
}

type systemSettingsBackupAnalysis struct {
	Document              systemSettingsBackupDocument
	FileHash              string
	CurrentFingerprint    string
	Updates               []SystemSettingUpdateRequest
	Changes               []Record
	Warnings              []Record
	ExpectedVersions      map[string]int64
	ChangedCount          int
	UnchangedCount        int
	UnsupportedCount      int
	PreservedMissingCount int
	RestartRequiredCount  int
	ImmediateCount        int
}

func systemSettingsRegistryFingerprint() string {
	hasher := sha256.New()
	for _, definition := range sortedSystemSettingDefinitions() {
		_, _ = fmt.Fprintf(hasher, "%s\x00%s\x00%s\x00%t\n", definition.Key, definition.Category, definition.ValueType, definition.Sensitive)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

func systemSettingsBackupHash(raw []byte) string {
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func buildSystemSettingsBackup(rows map[string]storedSystemSetting, cfg config.Config, exportedAt time.Time) ([]byte, systemSettingsBackupDocument, error) {
	document := systemSettingsBackupDocument{
		Format:              systemSettingsBackupFormat,
		FormatVersion:       systemSettingsBackupFormatVersion,
		ExportedAt:          exportedAt.UTC().Format(time.RFC3339),
		SourceEnvironment:   cfg.AppEnv,
		RegistryFingerprint: systemSettingsRegistryFingerprint(),
		Settings:            make([]systemSettingsBackupEntry, 0, len(systemSettingDefinitions)),
		ExcludedSecrets:     []systemSettingsBackupSecret{},
	}
	for _, definition := range sortedSystemSettingDefinitions() {
		row, ok := rows[definition.Key]
		if !ok {
			return nil, document, fmt.Errorf("system setting %s is missing", definition.Key)
		}
		if definition.Sensitive {
			document.ExcludedSecrets = append(document.ExcludedSecrets, systemSettingsBackupSecret{Key: definition.Key, Configured: row.Configured})
			continue
		}
		document.Settings = append(document.Settings, systemSettingsBackupEntry{Key: definition.Key, ValueType: definition.ValueType, Value: row.Value})
	}
	raw, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, document, err
	}
	raw = append(raw, '\n')
	return raw, document, nil
}

func parseSystemSettingsBackup(raw []byte) (systemSettingsBackupDocument, error) {
	document := systemSettingsBackupDocument{}
	if len(raw) > maxSystemSettingsBackupBytes {
		return document, validationError("Bản sao cấu hình không được vượt quá 256 KB")
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return document, validationError("Bản sao cấu hình không được rỗng")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return document, validationError("JSON bản sao cấu hình không hợp lệ", Record{"detail": err.Error()})
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return document, validationError("JSON bản sao cấu hình chỉ được chứa một document")
	}
	if document.Format != systemSettingsBackupFormat {
		return document, validationError("Định dạng bản sao cấu hình không được hỗ trợ", Record{"format": document.Format})
	}
	if document.FormatVersion != systemSettingsBackupFormatVersion {
		return document, validationError("Phiên bản bản sao cấu hình không được hỗ trợ", Record{"format_version": document.FormatVersion, "supported_version": systemSettingsBackupFormatVersion})
	}
	if _, err := time.Parse(time.RFC3339, document.ExportedAt); err != nil {
		return document, validationError("Thời điểm xuất trong bản sao không hợp lệ", Record{"exported_at": document.ExportedAt})
	}
	if strings.TrimSpace(document.SourceEnvironment) == "" || len(document.SourceEnvironment) > 32 {
		return document, validationError("Môi trường nguồn trong bản sao không hợp lệ")
	}
	if len(document.RegistryFingerprint) != len("sha256:")+sha256.Size*2 || !strings.HasPrefix(document.RegistryFingerprint, "sha256:") {
		return document, validationError("Registry fingerprint trong bản sao không hợp lệ")
	}
	if _, err := hex.DecodeString(strings.TrimPrefix(document.RegistryFingerprint, "sha256:")); err != nil {
		return document, validationError("Registry fingerprint trong bản sao không hợp lệ")
	}
	if len(document.Settings) == 0 {
		return document, validationError("Bản sao không có thiết lập để khôi phục")
	}
	if len(document.Settings) > maxSystemSettingsBackupEntries {
		return document, validationError("Bản sao có quá nhiều thiết lập", Record{"max_entries": maxSystemSettingsBackupEntries})
	}
	return document, nil
}

func (s *SystemSettingsStore) ExportBackup(ctx context.Context) ([]byte, string, Record, error) {
	rows, err := s.load(ctx, s.database())
	if err != nil {
		return nil, "", nil, err
	}
	s.mu.RLock()
	cfg := s.bootstrap
	s.mu.RUnlock()
	now := time.Now().UTC()
	raw, document, err := buildSystemSettingsBackup(rows, cfg, now)
	if err != nil {
		return nil, "", nil, err
	}
	filename := fmt.Sprintf("consflow-settings-%s.json", now.Format("20060102-150405"))
	metadata := Record{
		"format_version":        document.FormatVersion,
		"setting_count":         len(document.Settings),
		"excluded_secret_count": len(document.ExcludedSecrets),
		"registry_fingerprint":  document.RegistryFingerprint,
		"source_environment":    document.SourceEnvironment,
	}
	return raw, filename, metadata, nil
}

func (s *SystemSettingsStore) analyzeBackup(ctx context.Context, raw []byte) (*systemSettingsBackupAnalysis, error) {
	rows, err := s.load(ctx, s.database())
	if err != nil {
		return nil, err
	}
	return s.analyzeBackupRows(raw, rows)
}

func (s *SystemSettingsStore) analyzeBackupRows(raw []byte, rows map[string]storedSystemSetting) (*systemSettingsBackupAnalysis, error) {
	document, err := parseSystemSettingsBackup(raw)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	candidate := s.bootstrap
	s.mu.RUnlock()
	analysis := &systemSettingsBackupAnalysis{
		Document:           document,
		FileHash:           systemSettingsBackupHash(raw),
		CurrentFingerprint: systemSettingsRegistryFingerprint(),
		Updates:            []SystemSettingUpdateRequest{},
		Changes:            []Record{},
		Warnings:           []Record{},
		ExpectedVersions:   map[string]int64{},
	}
	if document.RegistryFingerprint != analysis.CurrentFingerprint {
		analysis.Warnings = append(analysis.Warnings, Record{
			"code": "REGISTRY_MISMATCH", "message": "Danh sách thiết lập của bản sao khác phiên bản hệ thống hiện tại.",
		})
	}
	if document.SourceEnvironment != candidate.AppEnv {
		analysis.Warnings = append(analysis.Warnings, Record{
			"code": "ENVIRONMENT_MISMATCH", "message": fmt.Sprintf("Bản sao được xuất từ môi trường %s; môi trường hiện tại là %s.", document.SourceEnvironment, candidate.AppEnv),
		})
	}
	coreKeys := map[string]bool{}
	for _, item := range s.coreSettings() {
		coreKeys[fmt.Sprint(item["key"])] = true
	}
	seen := map[string]bool{}
	nextValues := map[string]any{}
	for key, row := range rows {
		nextValues[key] = row.Value
	}
	for _, entry := range document.Settings {
		entry.Key = strings.TrimSpace(entry.Key)
		if entry.Key == "" {
			return nil, validationError("Bản sao có key cấu hình rỗng")
		}
		if len(entry.Key) > maxSystemSettingsBackupKeyLength {
			return nil, validationError("Key cấu hình trong bản sao quá dài")
		}
		if seen[entry.Key] {
			return nil, validationError("Bản sao có key cấu hình bị lặp", Record{"key": entry.Key})
		}
		seen[entry.Key] = true
		if coreKeys[entry.Key] {
			return nil, validationError("Bản sao không được chứa cấu hình lõi .env", Record{"key": entry.Key})
		}
		definition, ok := systemSettingDefinitionByKey(entry.Key)
		if !ok {
			analysis.UnsupportedCount++
			analysis.Warnings = append(analysis.Warnings, Record{
				"code": "UNSUPPORTED_KEY", "key": entry.Key, "message": "Thiết lập không tồn tại trên phiên bản hiện tại và sẽ được bỏ qua.",
			})
			continue
		}
		if definition.Sensitive {
			return nil, validationError("Bản sao không được chứa giá trị secret", Record{"key": entry.Key})
		}
		if entry.ValueType != definition.ValueType {
			return nil, validationError("Kiểu dữ liệu trong bản sao không khớp registry hiện tại", Record{"key": entry.Key, "backup_type": entry.ValueType, "current_type": definition.ValueType})
		}
		row, ok := rows[entry.Key]
		if !ok {
			return nil, fmt.Errorf("system setting %s is missing", entry.Key)
		}
		normalized, err := normalizeSystemSettingValue(definition, entry.Value)
		if err != nil {
			return nil, validationError(err.Error(), Record{"key": entry.Key})
		}
		nextValues[entry.Key] = normalized
		if canonicalSystemSettingValue(row.Value) == canonicalSystemSettingValue(normalized) {
			analysis.UnchangedCount++
			continue
		}
		analysis.ChangedCount++
		if definition.RequiresRestart {
			analysis.RestartRequiredCount++
		} else {
			analysis.ImmediateCount++
		}
		analysis.ExpectedVersions[entry.Key] = row.Version
		analysis.Updates = append(analysis.Updates, SystemSettingUpdateRequest{Key: entry.Key, Value: normalized, ExpectedVersion: row.Version})
		analysis.Changes = append(analysis.Changes, Record{
			"key": entry.Key, "label": definition.Label, "category": definition.Category,
			"old_value": row.Value, "new_value": normalized, "requires_restart": definition.RequiresRestart,
			"expected_version": row.Version,
		})
	}
	for _, secret := range document.ExcludedSecrets {
		secret.Key = strings.TrimSpace(secret.Key)
		if secret.Key == "" || seen[secret.Key] {
			return nil, validationError("Bản sao có metadata secret bị rỗng hoặc trùng key", Record{"key": secret.Key})
		}
		if len(secret.Key) > maxSystemSettingsBackupKeyLength {
			return nil, validationError("Key secret metadata trong bản sao quá dài")
		}
		seen[secret.Key] = true
		definition, ok := systemSettingDefinitionByKey(secret.Key)
		if !ok {
			analysis.UnsupportedCount++
			analysis.Warnings = append(analysis.Warnings, Record{
				"code": "UNSUPPORTED_SECRET", "key": secret.Key, "message": "Secret metadata không tồn tại trên phiên bản hiện tại và sẽ được bỏ qua.",
			})
			continue
		}
		if !definition.Sensitive {
			return nil, validationError("Metadata excluded_secrets chỉ được chứa secret", Record{"key": secret.Key})
		}
		analysis.Warnings = append(analysis.Warnings, Record{
			"code": "SECRET_PRESERVED", "key": secret.Key, "message": "Secret không có trong bản sao; giá trị hiện tại sẽ được giữ nguyên.",
		})
	}
	for _, definition := range sortedSystemSettingDefinitions() {
		if definition.Sensitive || seen[definition.Key] {
			continue
		}
		analysis.PreservedMissingCount++
	}
	for _, definition := range sortedSystemSettingDefinitions() {
		value, ok := nextValues[definition.Key]
		if !ok {
			return nil, fmt.Errorf("system setting %s is missing", definition.Key)
		}
		if err := applySystemSetting(&candidate, definition.Key, value); err != nil {
			return nil, validationError("Cấu hình tổng thể trong bản sao không hợp lệ", Record{"key": definition.Key, "detail": err.Error()})
		}
	}
	candidate.MarkRuntimeSettingsDatabaseBacked()
	if err := candidate.Validate(); err != nil {
		return nil, validationError("Cấu hình tổng thể trong bản sao không hợp lệ", Record{"detail": err.Error()})
	}
	return analysis, nil
}

func (analysis *systemSettingsBackupAnalysis) responseRecord() Record {
	return Record{
		"file_hash": analysis.FileHash, "format_version": analysis.Document.FormatVersion,
		"exported_at": analysis.Document.ExportedAt, "source_environment": analysis.Document.SourceEnvironment,
		"registry_fingerprint": analysis.Document.RegistryFingerprint, "current_registry_fingerprint": analysis.CurrentFingerprint,
		"changed_count": analysis.ChangedCount, "unchanged_count": analysis.UnchangedCount,
		"unsupported_count": analysis.UnsupportedCount, "preserved_missing_count": analysis.PreservedMissingCount,
		"restart_required_count": analysis.RestartRequiredCount, "immediate_count": analysis.ImmediateCount,
		"changes": analysis.Changes, "warnings": analysis.Warnings, "expected_versions": analysis.ExpectedVersions,
		"excluded_secrets": analysis.Document.ExcludedSecrets,
	}
}

func (s *SystemSettingsStore) PreviewBackupRestore(ctx context.Context, raw []byte) (Record, error) {
	analysis, err := s.analyzeBackup(ctx, raw)
	if err != nil {
		return nil, err
	}
	return analysis.responseRecord(), nil
}

func (s *SystemSettingsStore) RestoreBackup(ctx context.Context, raw []byte, previewHash string, expectedVersions map[string]int64, actorID, actorName string) (Record, error) {
	analysis, err := s.analyzeBackup(ctx, raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(previewHash) == "" || previewHash != analysis.FileHash {
		return nil, conflictError("Bản sao đã thay đổi sau khi kiểm tra; vui lòng kiểm tra lại")
	}
	if len(analysis.Updates) == 0 {
		return nil, validationError("Bản sao không có thay đổi cần khôi phục")
	}
	if len(expectedVersions) != len(analysis.ExpectedVersions) {
		return nil, conflictError("Thiết lập đã thay đổi sau khi kiểm tra; vui lòng kiểm tra lại")
	}
	for key, currentVersion := range analysis.ExpectedVersions {
		if expectedVersions[key] != currentVersion {
			return nil, conflictError("Thiết lập đã thay đổi sau khi kiểm tra; vui lòng kiểm tra lại", Record{"key": key})
		}
	}
	result, err := s.update(ctx, analysis.Updates, actorID, actorName, settingsUpdateOptions{
		Source: "backup_restore",
		SummaryAudit: Record{
			"file_hash": analysis.FileHash, "format_version": analysis.Document.FormatVersion,
			"source_environment": analysis.Document.SourceEnvironment, "updated_count": analysis.ChangedCount,
			"unsupported_count": analysis.UnsupportedCount, "restart_required_count": analysis.RestartRequiredCount,
		},
	})
	if err != nil {
		return nil, err
	}
	result["restart_required_count"] = analysis.RestartRequiredCount
	result["immediate_count"] = analysis.ImmediateCount
	result["warnings"] = analysis.Warnings
	return result, nil
}
