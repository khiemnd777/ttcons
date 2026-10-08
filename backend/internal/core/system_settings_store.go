package core

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"

	"consflow/backend/internal/shared/config"
)

type SystemSettingsStore struct {
	mu            sync.RWMutex
	db            *sql.DB
	bootstrap     config.Config
	startupValues map[string]string
	encryptionKey []byte
}

type SystemSettingUpdateRequest struct {
	Key             string `json:"key"`
	Value           any    `json:"value"`
	ExpectedVersion int64  `json:"expected_version"`
	ClearSecret     bool   `json:"clear_secret"`
}

type settingsUpdateOptions struct {
	Source       string
	SummaryAudit Record
}

type storedSystemSetting struct {
	Key             string
	Category        string
	Description     string
	ValueType       string
	IsSecret        bool
	RequiresRestart bool
	Value           any
	Configured      bool
	Version         int64
	Source          string
	UpdatedAt       string
	UpdatedBy       string
}

func NewSystemSettingsStore(db *sql.DB, cfg config.Config) *SystemSettingsStore {
	store := &SystemSettingsStore{db: db, bootstrap: cfg, startupValues: map[string]string{}}
	if cfg.SettingsEncryptionKey != "" {
		digest := sha256.Sum256([]byte(cfg.SettingsEncryptionKey))
		store.encryptionKey = digest[:]
	}
	return store
}

func (s *SystemSettingsStore) Rebind(db *sql.DB) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db = db
}

func (s *SystemSettingsStore) database() *sql.DB {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db
}

// Bootstrap inserts only missing registry rows, imports legacy environment
// values once, then overlays the authoritative database values onto cfg.
func (s *SystemSettingsStore) Bootstrap(ctx context.Context, cfg config.Config) (config.Config, error) {
	if err := s.ensureDefinitions(ctx, cfg); err != nil {
		return cfg, err
	}
	rows, err := s.load(ctx, s.database())
	if err != nil {
		return cfg, err
	}
	for _, definition := range sortedSystemSettingDefinitions() {
		row, ok := rows[definition.Key]
		if !ok {
			return cfg, fmt.Errorf("system setting %s is missing after bootstrap", definition.Key)
		}
		if err := applySystemSetting(&cfg, definition.Key, row.Value); err != nil {
			return cfg, fmt.Errorf("apply system setting %s: %w", definition.Key, err)
		}
		s.startupValues[definition.Key] = canonicalSystemSettingValue(row.Value)
	}
	cfg.MarkRuntimeSettingsDatabaseBacked()
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("validate database-backed settings: %w", err)
	}
	s.mu.Lock()
	s.bootstrap = cfg
	s.mu.Unlock()
	return cfg, nil
}

func (s *SystemSettingsStore) ensureDefinitions(ctx context.Context, cfg config.Config) error {
	tx, err := s.database().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, definition := range sortedSystemSettingDefinitions() {
		value, err := normalizeSystemSettingValue(definition, systemSettingConfigValue(cfg, definition.Key))
		if err != nil {
			return fmt.Errorf("normalize initial system setting %s: %w", definition.Key, err)
		}
		valueJSON, ciphertext, nonce, keyVersion, err := s.storageValue(definition, value)
		if err != nil {
			return fmt.Errorf("prepare initial system setting %s: %w", definition.Key, err)
		}
		source := "built_in_default"
		if definition.Category == "business" {
			source = "business_default"
		} else if legacyEnvironmentConfigured(definition.Key) {
			source = "legacy_env"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO system_settings (
				key, description, value_json, category, value_type, is_secret,
				requires_restart, value_ciphertext, value_nonce, key_version, source
			) VALUES ($1,$2,$3::jsonb,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (key) DO UPDATE SET
				description=EXCLUDED.description,
				category=EXCLUDED.category,
				value_type=EXCLUDED.value_type,
				is_secret=EXCLUDED.is_secret,
				requires_restart=EXCLUDED.requires_restart
		`, definition.Key, definition.Description, valueJSON, definition.Category, definition.ValueType,
			definition.Sensitive, definition.RequiresRestart, ciphertext, nonce, keyVersion, source); err != nil {
			return fmt.Errorf("upsert system setting definition %s: %w", definition.Key, err)
		}
	}
	return tx.Commit()
}

func legacyEnvironmentConfigured(key string) bool {
	_, direct := os.LookupEnv(key)
	_, file := os.LookupEnv(key + "_FILE")
	return direct || file
}

func (s *SystemSettingsStore) storageValue(definition systemSettingDefinition, value any) (string, []byte, []byte, any, error) {
	if !definition.Sensitive {
		raw, err := json.Marshal(value)
		return string(raw), nil, nil, nil, err
	}
	secret := fmt.Sprint(value)
	if secret == "" {
		return "null", nil, nil, nil, nil
	}
	ciphertext, nonce, err := s.encrypt([]byte(secret))
	if err != nil {
		return "", nil, nil, nil, err
	}
	return "null", ciphertext, nonce, 1, nil
}

func (s *SystemSettingsStore) encrypt(plaintext []byte) ([]byte, []byte, error) {
	if len(s.encryptionKey) == 0 {
		return nil, nil, errors.New("SETTINGS_ENCRYPTION_KEY is required before storing a non-empty secret")
	}
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	return gcm.Seal(nil, nonce, plaintext, nil), nonce, nil
}

func (s *SystemSettingsStore) decrypt(ciphertext, nonce []byte) (string, error) {
	if len(ciphertext) == 0 && len(nonce) == 0 {
		return "", nil
	}
	if len(s.encryptionKey) == 0 {
		return "", errors.New("SETTINGS_ENCRYPTION_KEY is required to decrypt database secrets")
	}
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("cannot decrypt database setting with the configured encryption key")
	}
	return string(plaintext), nil
}

type systemSettingsQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (s *SystemSettingsStore) load(ctx context.Context, queryer systemSettingsQueryer) (map[string]storedSystemSetting, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT setting.key, setting.category, setting.description, setting.value_type,
		       setting.is_secret, setting.requires_restart, setting.value_json,
		       setting.value_ciphertext, setting.value_nonce, setting.version,
		       setting.source, setting.updated_at::text, COALESCE(actor.email,'')
		FROM system_settings setting
		LEFT JOIN users actor ON actor.id=setting.updated_by
		ORDER BY setting.key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]storedSystemSetting{}
	for rows.Next() {
		var row storedSystemSetting
		var rawJSON []byte
		var ciphertext, nonce []byte
		if err := rows.Scan(&row.Key, &row.Category, &row.Description, &row.ValueType,
			&row.IsSecret, &row.RequiresRestart, &rawJSON, &ciphertext, &nonce, &row.Version,
			&row.Source, &row.UpdatedAt, &row.UpdatedBy); err != nil {
			return nil, err
		}
		definition, ok := systemSettingDefinitionByKey(row.Key)
		if !ok {
			continue
		}
		if row.IsSecret {
			secret, err := s.decrypt(ciphertext, nonce)
			if err != nil {
				return nil, fmt.Errorf("decrypt system setting %s: %w", row.Key, err)
			}
			row.Value = secret
			row.Configured = secret != ""
		} else {
			var decoded any
			if err := json.Unmarshal(rawJSON, &decoded); err != nil {
				return nil, fmt.Errorf("decode system setting %s: %w", row.Key, err)
			}
			normalized, err := normalizeSystemSettingValue(definition, decoded)
			if err != nil {
				return nil, fmt.Errorf("validate stored system setting %s: %w", row.Key, err)
			}
			row.Value = normalized
			row.Configured = true
		}
		out[row.Key] = row
	}
	return out, rows.Err()
}

func (s *SystemSettingsStore) List(ctx context.Context, canUpdate bool) (Record, error) {
	rows, err := s.load(ctx, s.database())
	if err != nil {
		return nil, err
	}
	settings := make([]Record, 0, len(systemSettingDefinitions))
	restartRequired := 0
	categorySet := map[string]bool{}
	for _, definition := range sortedSystemSettingDefinitions() {
		row := rows[definition.Key]
		restartPending := definition.RequiresRestart && s.startupValues[definition.Key] != canonicalSystemSettingValue(row.Value)
		if restartPending {
			restartRequired++
		}
		categorySet[definition.Category] = true
		item := Record{
			"key": definition.Key, "category": definition.Category, "label": definition.Label,
			"description": definition.Description, "value_type": definition.ValueType,
			"unit": definition.Unit, "options": definition.Options, "min": definition.Min, "max": definition.Max,
			"sensitive": definition.Sensitive, "configured": row.Configured,
			"requires_restart": definition.RequiresRestart, "restart_pending": restartPending,
			"version": row.Version, "source": row.Source, "updated_at": row.UpdatedAt, "updated_by": row.UpdatedBy,
		}
		if !definition.Sensitive {
			item["value"] = row.Value
		}
		settings = append(settings, item)
	}
	categories := make([]Record, 0, len(categorySet))
	for key, label := range systemSettingCategoryLabels() {
		if categorySet[key] {
			categories = append(categories, Record{"key": key, "label": label, "order": systemSettingCategoryOrder(key)})
		}
	}
	sort.Slice(categories, func(i, j int) bool { return number(categories[i]["order"]) < number(categories[j]["order"]) })
	return Record{
		"capabilities": Record{"can_update": canUpdate},
		"categories":   categories, "settings": settings,
		"core_settings": s.coreSettings(), "restart_required_count": restartRequired,
	}, nil
}

func systemSettingCategoryLabels() map[string]string {
	return map[string]string{
		"business": "Nghiệp vụ", "authentication": "Xác thực", "email": "Email",
		"http": "HTTP & upload", "database": "Database runtime", "cache": "Redis & cache",
		"rate_limit": "Rate limit", "concurrency": "Giới hạn đồng thời", "sse": "SSE",
	}
}

func (s *SystemSettingsStore) coreSettings() []Record {
	s.mu.RLock()
	cfg := s.bootstrap
	s.mu.RUnlock()
	core := []struct {
		Key, Label, Description string
		Value                   any
		Sensitive               bool
	}{
		{"APP_ENV", "Môi trường chạy", "Chọn chính sách an toàn development, test, staging hoặc production.", cfg.AppEnv, false},
		{"PORT", "Cổng backend", "Cổng lắng nghe của tiến trình backend.", cfg.Port, false},
		{"DATABASE_URL", "Kết nối PostgreSQL", "Điểm kết nối cần có trước khi tải cấu hình từ database.", cfg.DatabaseURL, true},
		{"REDIS_URL", "Kết nối Redis", "Redis chính cho session/rate-limit/PubSub.", cfg.RedisURL, true},
		{"REDIS_CACHE_URL", "Kết nối Redis cache", "Redis cache riêng, nếu bật Project Explorer cache.", cfg.RedisCacheURL, true},
		{"JWT_SECRET", "Khóa JWT", "Root secret ký access token.", cfg.JWTSecret, true},
		{"SETTINGS_ENCRYPTION_KEY", "Khóa mã hóa cấu hình", "Root secret mã hóa các secret lưu trong database.", cfg.SettingsEncryptionKey, true},
		{"FRONTEND_ORIGIN", "Frontend origin", "Origin tin cậy cho CORS và liên kết khôi phục.", cfg.FrontendOrigin, false},
		{"MIGRATIONS_DIR", "Thư mục migration", "Nguồn migration của custom migration runner.", cfg.MigrationsDir, false},
		{"AUTO_MIGRATE", "Tự chạy migration", "Production bắt buộc tắt và dùng migration job riêng.", cfg.AutoMigrate, false},
		{"MIGRATION_LOCK_TIMEOUT", "Migration lock timeout", "Thời gian chờ advisory lock migration.", cfg.MigrationLockTimeout.String(), false},
		{"MIGRATION_TIMEOUT", "Migration timeout", "Deadline toàn bộ migration startup/job.", cfg.MigrationTimeout.String(), false},
		{"STORAGE_PROVIDER", "Storage provider", "Provider ghi file chính; production bắt buộc S3.", cfg.StorageWriteProvider(), false},
		{"STORAGE_PATH", "Local storage path", "Root local storage dùng ở development/test.", cfg.StoragePath, false},
		{"STORAGE_OPERATION_TIMEOUT", "Storage operation timeout", "Deadline cho mỗi thao tác đọc/ghi/xóa object.", cfg.StorageOperationTimeout.String(), false},
		{"STORAGE_MIGRATION_TIMEOUT", "Storage migration timeout", "Deadline cho job chuyển object Local sang S3.", cfg.StorageMigrationTimeout.String(), false},
		{"STORAGE_S3_BUCKET", "S3 bucket", "Bucket lưu tài liệu production.", cfg.StorageS3Bucket, false},
		{"STORAGE_S3_REGION", "S3 region", "Region của S3 bucket.", cfg.StorageS3Region, false},
		{"STORAGE_S3_ENDPOINT", "S3 endpoint", "Endpoint tùy chọn cho S3-compatible storage.", cfg.StorageS3Endpoint, false},
		{"STORAGE_S3_USE_PATH_STYLE", "S3 path-style", "Bật địa chỉ path-style cho S3-compatible storage khi cần.", cfg.StorageS3UsePathStyle, false},
		{"STORAGE_S3_ALLOW_INSECURE_ENDPOINT", "Cho phép S3 endpoint HTTP", "Chỉ dành cho development/test; production yêu cầu HTTPS.", cfg.StorageS3AllowInsecureEndpoint, false},
		{"AWS_ACCESS_KEY_ID", "AWS access key", "Credential do runtime hoặc workload identity cung cấp.", os.Getenv("AWS_ACCESS_KEY_ID"), true},
		{"AWS_SECRET_ACCESS_KEY", "AWS secret key", "Credential do runtime hoặc secret manager cung cấp.", os.Getenv("AWS_SECRET_ACCESS_KEY"), true},
		{"AWS_SESSION_TOKEN", "AWS session token", "Credential tạm thời do runtime cung cấp khi có.", os.Getenv("AWS_SESSION_TOKEN"), true},
		{"TRUSTED_PROXY_CIDRS", "Trusted proxies", "Danh sách IP/CIDR proxy tin cậy.", cfg.TrustedProxies, false},
		{"AUTH_REFRESH_COOKIE_SECURE", "Secure refresh cookie", "Security anchor cho refresh cookie; production bắt buộc bật.", cfg.AuthRefreshCookieSecure, false},
	}
	out := make([]Record, 0, len(core))
	for _, item := range core {
		row := Record{"key": item.Key, "label": item.Label, "description": item.Description, "sensitive": item.Sensitive, "source": ".env", "readonly": true}
		if item.Sensitive {
			row["configured"] = strings.TrimSpace(fmt.Sprint(item.Value)) != ""
		} else {
			row["value"] = item.Value
			row["configured"] = true
		}
		out = append(out, row)
	}
	return out
}

func (s *SystemSettingsStore) Update(ctx context.Context, updates []SystemSettingUpdateRequest, actorID, actorName string) (Record, error) {
	return s.update(ctx, updates, actorID, actorName, settingsUpdateOptions{Source: "database_ui"})
}

func (s *SystemSettingsStore) update(ctx context.Context, updates []SystemSettingUpdateRequest, actorID, actorName string, options settingsUpdateOptions) (Record, error) {
	if len(updates) == 0 {
		return nil, validationError("Chưa có cấu hình cần lưu", Record{"field": "updates"})
	}
	if len(updates) > len(systemSettingDefinitions) {
		return nil, validationError("Danh sách cấu hình cập nhật quá lớn", Record{"field": "updates"})
	}
	tx, err := s.database().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	current, err := s.load(ctx, tx)
	if err != nil {
		return nil, err
	}
	nextValues := map[string]any{}
	for key, row := range current {
		nextValues[key] = row.Value
	}
	seen := map[string]bool{}
	normalizedUpdates := make([]SystemSettingUpdateRequest, 0, len(updates))
	for _, update := range updates {
		update.Key = strings.TrimSpace(update.Key)
		definition, ok := systemSettingDefinitionByKey(update.Key)
		if !ok {
			return nil, validationError("Cấu hình không được phép quản lý qua UI", Record{"key": update.Key})
		}
		if seen[update.Key] {
			return nil, validationError("Cấu hình bị lặp trong yêu cầu", Record{"key": update.Key})
		}
		seen[update.Key] = true
		row := current[update.Key]
		if update.ExpectedVersion != row.Version {
			return nil, validationError("Cấu hình đã được người khác cập nhật; vui lòng tải lại", Record{"key": update.Key, "expected_version": update.ExpectedVersion, "current_version": row.Version})
		}
		value := update.Value
		if definition.Sensitive {
			if update.ClearSecret {
				value = ""
			} else if strings.TrimSpace(fmt.Sprint(value)) == "" {
				return nil, validationError("Secret mới không được để trống; dùng thao tác xóa secret nếu cần", Record{"key": update.Key})
			}
		}
		normalized, err := normalizeSystemSettingValue(definition, value)
		if err != nil {
			return nil, validationError(err.Error(), Record{"key": update.Key})
		}
		update.Value = normalized
		nextValues[update.Key] = normalized
		normalizedUpdates = append(normalizedUpdates, update)
	}

	s.mu.RLock()
	candidate := s.bootstrap
	s.mu.RUnlock()
	for _, definition := range sortedSystemSettingDefinitions() {
		if err := applySystemSetting(&candidate, definition.Key, nextValues[definition.Key]); err != nil {
			return nil, validationError("Cấu hình tổng thể không hợp lệ", Record{"key": definition.Key, "detail": err.Error()})
		}
	}
	candidate.MarkRuntimeSettingsDatabaseBacked()
	if err := candidate.Validate(); err != nil {
		return nil, validationError("Cấu hình tổng thể không hợp lệ", Record{"detail": err.Error()})
	}
	if strings.TrimSpace(options.Source) == "" {
		options.Source = "database_ui"
	}
	for _, update := range normalizedUpdates {
		definition, _ := systemSettingDefinitionByKey(update.Key)
		valueJSON, ciphertext, nonce, keyVersion, err := s.storageValue(definition, update.Value)
		if err != nil {
			return nil, validationError("Không thể mã hóa secret cấu hình", Record{"key": update.Key, "detail": err.Error()})
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE system_settings
			SET value_json=$1::jsonb, value_ciphertext=$2, value_nonce=$3, key_version=$4,
			    version=version+1, source=$5, updated_by=NULLIF($6,'')::uuid, updated_at=now()
			WHERE key=$7 AND version=$8
		`, valueJSON, ciphertext, nonce, keyVersion, options.Source, actorID, update.Key, update.ExpectedVersion)
		if err != nil {
			return nil, err
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			return nil, validationError("Cấu hình đã thay đổi; vui lòng tải lại", Record{"key": update.Key})
		}
		oldValue := current[update.Key].Value
		metadata := Record{"key": update.Key, "requires_restart": definition.RequiresRestart, "source": options.Source}
		if definition.Sensitive {
			metadata["old_configured"] = strings.TrimSpace(fmt.Sprint(oldValue)) != ""
			metadata["new_configured"] = strings.TrimSpace(fmt.Sprint(update.Value)) != ""
		} else {
			metadata["old_value"] = oldValue
			metadata["new_value"] = update.Value
		}
		if err := addAuditTx(ctx, tx, "", actorID, actorName, "system_setting_updated", "system_setting", "", "Đã cập nhật thiết lập hệ thống", metadata); err != nil {
			return nil, err
		}
	}
	if options.SummaryAudit != nil {
		if err := addAuditTx(ctx, tx, "", actorID, actorName, "system_settings_backup_restored", "system_settings_backup", "", "Đã khôi phục thiết lập hệ thống từ bản sao JSON", options.SummaryAudit); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return Record{"updated": len(normalizedUpdates)}, nil
}

func canonicalSystemSettingValue(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func (s *SystemSettingsStore) SurplusHandlingOptions(ctx context.Context) []string {
	value, err := s.liveValue(ctx, "surplus_handling_options")
	if err != nil {
		return []string{"return_to_warehouse", "return_to_subcontractor"}
	}
	options := stringSlice(value)
	if len(options) == 0 {
		return []string{"return_to_warehouse", "return_to_subcontractor"}
	}
	return options
}

func (s *SystemSettingsStore) liveValue(ctx context.Context, key string) (any, error) {
	definition, ok := systemSettingDefinitionByKey(key)
	if !ok || definition.Sensitive {
		return nil, errors.New("unsupported live system setting")
	}
	var raw []byte
	if err := s.database().QueryRowContext(ctx, `SELECT value_json FROM system_settings WHERE key=$1`, key).Scan(&raw); err != nil {
		return nil, err
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	return normalizeSystemSettingValue(definition, decoded)
}
