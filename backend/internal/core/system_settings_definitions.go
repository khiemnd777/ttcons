package core

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"consflow/backend/internal/shared/config"
)

type SystemSettingOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type systemSettingDefinition struct {
	Key             string
	Category        string
	Label           string
	Description     string
	ValueType       string
	Unit            string
	Sensitive       bool
	RequiresRestart bool
	Options         []SystemSettingOption
	Min             *float64
	Max             *float64
	AllowEmpty      bool
	AllowZero       bool
}

var systemSettingDefinitions = buildSystemSettingDefinitions()

func buildSystemSettingDefinitions() []systemSettingDefinition {
	definitions := []systemSettingDefinition{
		multiSelectSetting("surplus_handling_options", "business", "Cách xử lý vật tư dư", "Các phương án được phép chọn khi xử lý vật tư dư tại công trình.", []SystemSettingOption{
			{Value: "return_to_warehouse", Label: "Lưu kho"},
			{Value: "return_to_subcontractor", Label: "Trả thầu phụ"},
		}, false),

		integerSetting("AUTH_ACCESS_TOKEN_MINUTES", "authentication", "Thời hạn access token", "Số phút hiệu lực của access token mới.", "phút", 1, 1440, true),
		integerSetting("AUTH_REFRESH_TOKEN_HOURS", "authentication", "Thời hạn refresh token", "Số giờ hiệu lực của refresh token mới.", "giờ", 1, 24*365, true),
		enumSetting("AUTH_REFRESH_COOKIE_SAME_SITE", "authentication", "SameSite của refresh cookie", "Chính sách SameSite áp dụng cho refresh cookie.", []SystemSettingOption{
			{Value: "Lax", Label: "Lax"}, {Value: "Strict", Label: "Strict"}, {Value: "None", Label: "None"},
		}, true),
		booleanSetting("AUTH_RECOVERY_ENABLED", "authentication", "Bật khôi phục tài khoản", "Cho phép luồng quên mật khẩu qua email khôi phục.", true),
		durationSetting("AUTH_RECOVERY_TOKEN_TTL", "authentication", "Thời hạn token đặt lại mật khẩu", "Thời gian token đặt lại mật khẩu còn hiệu lực.", false, true),
		durationSetting("AUTH_RECOVERY_VERIFY_TTL", "authentication", "Thời hạn xác minh email", "Thời gian token xác minh email khôi phục còn hiệu lực.", false, true),

		stringSetting("SMTP_HOST", "email", "Máy chủ SMTP", "Hostname của SMTP relay.", true, true),
		integerSetting("SMTP_PORT", "email", "Cổng SMTP", "Cổng kết nối SMTP relay.", "", 1, 65535, true),
		stringSetting("SMTP_USERNAME", "email", "Tài khoản SMTP", "Tên đăng nhập SMTP; để trống khi relay không yêu cầu xác thực.", true, true),
		secretSetting("SMTP_PASSWORD", "email", "Mật khẩu SMTP", "Secret được mã hóa trong database và không bao giờ trả ngược qua API.", true),
		stringSetting("SMTP_FROM_ADDRESS", "email", "Email người gửi", "Địa chỉ email gửi thông báo khôi phục tài khoản.", true, true),
		stringSetting("SMTP_FROM_NAME", "email", "Tên người gửi", "Tên hiển thị của email hệ thống.", true, true),
		enumSetting("SMTP_TLS_MODE", "email", "Chế độ TLS SMTP", "Production chỉ chấp nhận STARTTLS hoặc TLS.", []SystemSettingOption{
			{Value: "starttls", Label: "STARTTLS"}, {Value: "tls", Label: "TLS"}, {Value: "none", Label: "Không mã hóa"},
		}, true),
		durationSetting("SMTP_TIMEOUT", "email", "Timeout SMTP", "Thời gian tối đa cho một thao tác SMTP.", false, true),

		integerSetting("MAX_UPLOAD_BYTES", "http", "Dung lượng upload tối đa", "Giới hạn body upload chung của backend.", "byte", 1024, 1024*1024*1024, true),
		durationSetting("HTTP_READ_TIMEOUT", "http", "HTTP read timeout", "Thời gian tối đa đọc request.", false, true),
		durationSetting("HTTP_WRITE_TIMEOUT", "http", "HTTP write timeout", "Có thể đặt 0 để hỗ trợ SSE dài hạn.", true, true),
		durationSetting("HTTP_IDLE_TIMEOUT", "http", "HTTP idle timeout", "Thời gian giữ kết nối HTTP nhàn rỗi.", false, true),
		durationSetting("SHUTDOWN_TIMEOUT", "http", "Graceful shutdown timeout", "Thời gian tối đa backend chờ request hoàn tất khi dừng.", false, true),
		durationSetting("READINESS_TIMEOUT", "http", "Readiness timeout", "Timeout kiểm tra PostgreSQL cho readiness endpoint.", false, true),

		durationSetting("DB_STATEMENT_TIMEOUT", "database", "Statement timeout", "Giới hạn thời gian câu lệnh PostgreSQL runtime.", false, true),
		durationSetting("DB_LOCK_TIMEOUT", "database", "Lock timeout", "Giới hạn thời gian chờ lock PostgreSQL runtime.", false, true),
		integerSetting("DB_MAX_OPEN_CONNS", "database", "Số kết nối DB tối đa", "Kích thước tối đa của connection pool.", "kết nối", 1, 1000, true),
		integerSetting("DB_MAX_IDLE_CONNS", "database", "Số kết nối DB nhàn rỗi", "Số kết nối nhàn rỗi tối đa trong pool.", "kết nối", 0, 1000, true),
		durationSetting("DB_CONN_MAX_LIFETIME", "database", "Tuổi thọ kết nối DB", "Tuổi thọ tối đa của một kết nối PostgreSQL.", false, true),
		durationSetting("DB_CONN_MAX_IDLE_TIME", "database", "Thời gian nhàn rỗi kết nối DB", "Thời gian một kết nối được phép nhàn rỗi.", false, true),

		durationSetting("REDIS_OPERATION_TIMEOUT", "cache", "Redis operation timeout", "Timeout cho Redis rate-limit và Pub/Sub.", false, true),
		enumSetting("PROJECT_EXPLORER_CACHE_MODE", "cache", "Chế độ Project Explorer cache", "Triển khai theo thứ tự off → shadow → serve.", []SystemSettingOption{
			{Value: "off", Label: "Tắt"}, {Value: "shadow", Label: "Shadow"}, {Value: "serve", Label: "Phục vụ từ cache"},
		}, true),
		durationSetting("PROJECT_EXPLORER_CACHE_TIMEOUT", "cache", "Project cache timeout", "Timeout cho Redis cache riêng của Project Explorer.", false, true),
		durationSetting("PROJECT_EXPLORER_CACHE_INDEX_TTL", "cache", "Project cache index TTL", "TTL của trang ID theo actor/filter.", false, true),
		durationSetting("PROJECT_EXPLORER_CACHE_ROW_TTL", "cache", "Project cache row TTL", "TTL của bản ghi project đã cache.", false, true),
		durationSetting("PROJECT_EXPLORER_CACHE_LOCK_TTL", "cache", "Project cache lock TTL", "TTL của khóa chống stampede.", false, true),
		integerSetting("PROJECT_EXPLORER_CACHE_MAX_VALUE_BYTES", "cache", "Kích thước cache value tối đa", "Giới hạn kích thước một value trong Project Explorer cache.", "byte", 1024, 16*1024*1024, true),

		integerSetting("RATE_LIMIT_REDIS_MAX_IN_FLIGHT", "rate_limit", "Redis limiter in-flight tối đa", "Số thao tác Redis limiter được phép đồng thời.", "request", 1, 10000, true),
		booleanSetting("RATE_LIMIT_ENABLED", "rate_limit", "Bật rate limit", "Production luôn bắt buộc bật rate limit.", true),
		enumSetting("RATE_LIMIT_MODE", "rate_limit", "Chế độ rate limit", "Observe chỉ ghi nhận; Enforce trả 429 khi vượt hạn mức.", []SystemSettingOption{
			{Value: "observe", Label: "Observe"}, {Value: "enforce", Label: "Enforce"},
		}, true),
		integerSetting("RATE_LIMIT_LOCAL_MAX_KEYS", "rate_limit", "Số key limiter local tối đa", "Giới hạn bộ nhớ fallback limiter trong mỗi process.", "key", 100, 10000000, true),

		integerSetting("UPLOAD_CONCURRENCY", "concurrency", "Upload đồng thời", "Số request upload nặng được xử lý đồng thời.", "request", 1, 1000, true),
		integerSetting("EXPORT_CONCURRENCY", "concurrency", "Export đồng thời", "Số request export được xử lý đồng thời.", "request", 1, 1000, true),
		integerSetting("REPORT_CONCURRENCY", "concurrency", "Báo cáo đồng thời", "Số request báo cáo nặng được xử lý đồng thời.", "request", 1, 1000, true),

		integerSetting("SSE_MAX_CONNECTIONS", "sse", "SSE toàn process", "Số kết nối SSE tối đa trong một backend process.", "kết nối", 1, 100000, true),
		integerSetting("SSE_MAX_CONNECTIONS_PER_IP", "sse", "SSE tối đa mỗi IP", "Số kết nối SSE tối đa cho một IP.", "kết nối", 1, 10000, true),
		integerSetting("SSE_MAX_CONNECTIONS_PER_ACTOR", "sse", "SSE tối đa mỗi người dùng", "Số kết nối SSE tối đa cho một actor.", "kết nối", 1, 1000, true),
		durationSetting("SSE_VISIBILITY_TIMEOUT", "sse", "SSE visibility timeout", "Timeout xây snapshot quyền hiển thị cho stream.", false, true),
		durationSetting("SSE_MAX_CONNECTION_LIFETIME", "sse", "Tuổi thọ kết nối SSE", "Buộc reconnect để tái xác minh JWT/session và quyền.", false, true),
		stringSetting("SSE_BROKER_CHANNEL", "sse", "Redis Pub/Sub channel", "Channel dùng phát invalidation giữa các backend replica.", false, true),
	}

	requestClasses := []struct {
		Key   string
		Label string
	}{
		{Key: "AUTH", Label: "Đăng nhập"},
		{Key: "GENERAL", Label: "Chung"},
		{Key: "UPLOAD", Label: "Upload"},
		{Key: "EXPORT", Label: "Export"},
		{Key: "SSE", Label: "SSE"},
	}
	for _, scope := range []struct {
		Key   string
		Label string
	}{{Key: "IP", Label: "IP"}, {Key: "ACTOR", Label: "Người dùng"}} {
		for _, class := range requestClasses {
			prefix := "RATE_LIMIT_" + scope.Key + "_" + class.Key
			labelPrefix := scope.Label + " · " + class.Label
			definitions = append(definitions,
				integerSetting(prefix+"_LIMIT", "rate_limit", labelPrefix+" · Giới hạn", "Số request trong mỗi cửa sổ thời gian.", "request", 1, 10000000, true),
				integerSetting(prefix+"_BURST", "rate_limit", labelPrefix+" · Burst", "Dung lượng burst bổ sung của token bucket.", "request", 1, 10000000, true),
				durationSetting(prefix+"_WINDOW", "rate_limit", labelPrefix+" · Cửa sổ", "Khoảng thời gian tính hạn mức.", false, true),
			)
		}
	}
	return definitions
}

func booleanSetting(key, category, label, description string, restart bool) systemSettingDefinition {
	return systemSettingDefinition{Key: key, Category: category, Label: label, Description: description, ValueType: "boolean", RequiresRestart: restart}
}

func integerSetting(key, category, label, description, unit string, min, max float64, restart bool) systemSettingDefinition {
	return systemSettingDefinition{Key: key, Category: category, Label: label, Description: description, ValueType: "integer", Unit: unit, Min: floatPointer(min), Max: floatPointer(max), RequiresRestart: restart}
}

func numberSetting(key, category, label, description, unit string, min, max float64, restart bool) systemSettingDefinition {
	return systemSettingDefinition{Key: key, Category: category, Label: label, Description: description, ValueType: "number", Unit: unit, Min: floatPointer(min), Max: floatPointer(max), RequiresRestart: restart}
}

func durationSetting(key, category, label, description string, allowZero, restart bool) systemSettingDefinition {
	return systemSettingDefinition{Key: key, Category: category, Label: label, Description: description, ValueType: "duration", AllowZero: allowZero, RequiresRestart: restart}
}

func stringSetting(key, category, label, description string, allowEmpty, restart bool) systemSettingDefinition {
	return systemSettingDefinition{Key: key, Category: category, Label: label, Description: description, ValueType: "string", AllowEmpty: allowEmpty, RequiresRestart: restart}
}

func secretSetting(key, category, label, description string, restart bool) systemSettingDefinition {
	return systemSettingDefinition{Key: key, Category: category, Label: label, Description: description, ValueType: "secret", Sensitive: true, AllowEmpty: true, RequiresRestart: restart}
}

func enumSetting(key, category, label, description string, options []SystemSettingOption, restart bool) systemSettingDefinition {
	return systemSettingDefinition{Key: key, Category: category, Label: label, Description: description, ValueType: "enum", Options: options, RequiresRestart: restart}
}

func multiSelectSetting(key, category, label, description string, options []SystemSettingOption, restart bool) systemSettingDefinition {
	return systemSettingDefinition{Key: key, Category: category, Label: label, Description: description, ValueType: "multi_select", Options: options, RequiresRestart: restart}
}

func floatPointer(value float64) *float64 { return &value }

func systemSettingDefinitionByKey(key string) (systemSettingDefinition, bool) {
	for _, definition := range systemSettingDefinitions {
		if definition.Key == key {
			return definition, true
		}
	}
	return systemSettingDefinition{}, false
}

func normalizeSystemSettingValue(definition systemSettingDefinition, value any) (any, error) {
	switch definition.ValueType {
	case "boolean":
		switch typed := value.(type) {
		case bool:
			return typed, nil
		case string:
			parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
			if err == nil {
				return parsed, nil
			}
		}
		return nil, fmt.Errorf("%s phải là bật hoặc tắt", definition.Label)
	case "integer":
		numeric, ok := numericSettingValue(value)
		if !ok || math.Trunc(numeric) != numeric {
			return nil, fmt.Errorf("%s phải là số nguyên", definition.Label)
		}
		if err := validateSettingNumberRange(definition, numeric); err != nil {
			return nil, err
		}
		return int64(numeric), nil
	case "number":
		numeric, ok := numericSettingValue(value)
		if !ok {
			return nil, fmt.Errorf("%s phải là số", definition.Label)
		}
		if err := validateSettingNumberRange(definition, numeric); err != nil {
			return nil, err
		}
		return numeric, nil
	case "duration":
		text := strings.TrimSpace(fmt.Sprint(value))
		duration, err := time.ParseDuration(text)
		if err != nil || duration < 0 || (!definition.AllowZero && duration == 0) {
			return nil, fmt.Errorf("%s phải là duration hợp lệ%s", definition.Label, map[bool]string{true: "", false: " và lớn hơn 0"}[definition.AllowZero])
		}
		return duration.String(), nil
	case "string", "secret":
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%s phải là chuỗi", definition.Label)
		}
		if definition.ValueType != "secret" {
			text = strings.TrimSpace(text)
		}
		if !definition.AllowEmpty && text == "" {
			return nil, fmt.Errorf("%s không được để trống", definition.Label)
		}
		if len([]rune(text)) > 1000 {
			return nil, fmt.Errorf("%s không được vượt quá 1000 ký tự", definition.Label)
		}
		return text, nil
	case "enum":
		text := strings.TrimSpace(fmt.Sprint(value))
		for _, option := range definition.Options {
			if option.Value == text {
				return text, nil
			}
		}
		return nil, fmt.Errorf("%s có giá trị không được hỗ trợ", definition.Label)
	case "multi_select":
		values := stringSlice(value)
		if len(values) == 0 {
			return nil, fmt.Errorf("%s phải có ít nhất một lựa chọn", definition.Label)
		}
		allowed := map[string]bool{}
		for _, option := range definition.Options {
			allowed[option.Value] = true
		}
		seen := map[string]bool{}
		normalized := make([]string, 0, len(values))
		for _, item := range values {
			item = strings.TrimSpace(item)
			if !allowed[item] {
				return nil, fmt.Errorf("%s chứa lựa chọn không được hỗ trợ", definition.Label)
			}
			if !seen[item] {
				seen[item] = true
				normalized = append(normalized, item)
			}
		}
		return normalized, nil
	default:
		return nil, fmt.Errorf("kiểu cấu hình %s không được hỗ trợ", definition.ValueType)
	}
}

func validateSettingNumberRange(definition systemSettingDefinition, value float64) error {
	if definition.Min != nil && value < *definition.Min {
		return fmt.Errorf("%s phải lớn hơn hoặc bằng %s", definition.Label, formatSettingNumber(*definition.Min))
	}
	if definition.Max != nil && value > *definition.Max {
		return fmt.Errorf("%s phải nhỏ hơn hoặc bằng %s", definition.Label, formatSettingNumber(*definition.Max))
	}
	return nil
}

func numericSettingValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, !math.IsNaN(typed) && !math.IsInf(typed, 0)
	case float32:
		return float64(typed), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func formatSettingNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func systemSettingConfigValue(cfg config.Config, key string) any {
	switch key {
	case "surplus_handling_options":
		return []string{"return_to_warehouse", "return_to_subcontractor"}
	case "AUTH_ACCESS_TOKEN_MINUTES":
		return cfg.AuthAccessTokenMinutes
	case "AUTH_REFRESH_TOKEN_HOURS":
		return cfg.AuthRefreshTokenHours
	case "AUTH_REFRESH_COOKIE_SAME_SITE":
		return cfg.AuthRefreshCookieSameSite
	case "AUTH_RECOVERY_ENABLED":
		return cfg.AuthRecoveryEnabled
	case "AUTH_RECOVERY_TOKEN_TTL":
		return cfg.AuthRecoveryTokenTTL.String()
	case "AUTH_RECOVERY_VERIFY_TTL":
		return cfg.AuthRecoveryVerifyTTL.String()
	case "SMTP_HOST":
		return cfg.SMTPHost
	case "SMTP_PORT":
		return cfg.SMTPPort
	case "SMTP_USERNAME":
		return cfg.SMTPUsername
	case "SMTP_PASSWORD":
		return cfg.SMTPPassword
	case "SMTP_FROM_ADDRESS":
		return cfg.SMTPFromAddress
	case "SMTP_FROM_NAME":
		return cfg.SMTPFromName
	case "SMTP_TLS_MODE":
		return cfg.SMTPTLSMode
	case "SMTP_TIMEOUT":
		return cfg.SMTPTimeout.String()
	case "MAX_UPLOAD_BYTES":
		return cfg.MaxUploadBytes
	case "HTTP_READ_TIMEOUT":
		return cfg.HTTPReadTimeout.String()
	case "HTTP_WRITE_TIMEOUT":
		return cfg.HTTPWriteTimeout.String()
	case "HTTP_IDLE_TIMEOUT":
		return cfg.HTTPIdleTimeout.String()
	case "SHUTDOWN_TIMEOUT":
		return cfg.ShutdownTimeout.String()
	case "READINESS_TIMEOUT":
		return cfg.ReadinessTimeout.String()
	case "DB_STATEMENT_TIMEOUT":
		return cfg.DBStatementTimeout.String()
	case "DB_LOCK_TIMEOUT":
		return cfg.DBLockTimeout.String()
	case "DB_MAX_OPEN_CONNS":
		return cfg.DBMaxOpenConns
	case "DB_MAX_IDLE_CONNS":
		return cfg.DBMaxIdleConns
	case "DB_CONN_MAX_LIFETIME":
		return cfg.DBConnMaxLifetime.String()
	case "DB_CONN_MAX_IDLE_TIME":
		return cfg.DBConnMaxIdleTime.String()
	case "REDIS_OPERATION_TIMEOUT":
		return cfg.RedisOperationTimeout.String()
	case "PROJECT_EXPLORER_CACHE_MODE":
		return cfg.ProjectExplorerCacheMode
	case "PROJECT_EXPLORER_CACHE_TIMEOUT":
		return cfg.ProjectExplorerCacheTimeout.String()
	case "PROJECT_EXPLORER_CACHE_INDEX_TTL":
		return cfg.ProjectExplorerCacheIndexTTL.String()
	case "PROJECT_EXPLORER_CACHE_ROW_TTL":
		return cfg.ProjectExplorerCacheRowTTL.String()
	case "PROJECT_EXPLORER_CACHE_LOCK_TTL":
		return cfg.ProjectExplorerCacheLockTTL.String()
	case "PROJECT_EXPLORER_CACHE_MAX_VALUE_BYTES":
		return cfg.ProjectExplorerCacheMaxBytes
	case "RATE_LIMIT_REDIS_MAX_IN_FLIGHT":
		return cfg.RateLimitRedisMaxInFlight
	case "RATE_LIMIT_ENABLED":
		return cfg.RateLimitEnabled
	case "RATE_LIMIT_MODE":
		return cfg.RateLimitMode
	case "RATE_LIMIT_LOCAL_MAX_KEYS":
		return cfg.RateLimitLocalMaxKeys
	case "UPLOAD_CONCURRENCY":
		return cfg.UploadConcurrency
	case "EXPORT_CONCURRENCY":
		return cfg.ExportConcurrency
	case "REPORT_CONCURRENCY":
		return cfg.ReportConcurrency
	case "SSE_MAX_CONNECTIONS":
		return cfg.SSEMaxConnections
	case "SSE_MAX_CONNECTIONS_PER_IP":
		return cfg.SSEMaxConnectionsPerIP
	case "SSE_MAX_CONNECTIONS_PER_ACTOR":
		return cfg.SSEMaxConnectionsPerActor
	case "SSE_VISIBILITY_TIMEOUT":
		return cfg.SSEVisibilityTimeout.String()
	case "SSE_MAX_CONNECTION_LIFETIME":
		return cfg.SSEMaxConnectionLifetime.String()
	case "SSE_BROKER_CHANNEL":
		return cfg.SSEBrokerChannel
	}
	if value, ok := rateLimitConfigValue(cfg, key); ok {
		return value
	}
	return nil
}

func applySystemSetting(cfg *config.Config, key string, value any) error {
	integer := func() int { numeric, _ := numericSettingValue(value); return int(numeric) }
	integer64 := func() int64 { numeric, _ := numericSettingValue(value); return int64(numeric) }
	duration := func() time.Duration { parsed, _ := time.ParseDuration(fmt.Sprint(value)); return parsed }
	switch key {
	case "surplus_handling_options":
		return nil
	case "AUTH_ACCESS_TOKEN_MINUTES":
		cfg.AuthAccessTokenMinutes = integer64()
	case "AUTH_REFRESH_TOKEN_HOURS":
		cfg.AuthRefreshTokenHours = integer64()
	case "AUTH_REFRESH_COOKIE_SAME_SITE":
		cfg.AuthRefreshCookieSameSite = fmt.Sprint(value)
	case "AUTH_RECOVERY_ENABLED":
		cfg.AuthRecoveryEnabled, _ = value.(bool)
	case "AUTH_RECOVERY_TOKEN_TTL":
		cfg.AuthRecoveryTokenTTL = duration()
	case "AUTH_RECOVERY_VERIFY_TTL":
		cfg.AuthRecoveryVerifyTTL = duration()
	case "SMTP_HOST":
		cfg.SMTPHost = fmt.Sprint(value)
	case "SMTP_PORT":
		cfg.SMTPPort = integer()
	case "SMTP_USERNAME":
		cfg.SMTPUsername = fmt.Sprint(value)
	case "SMTP_PASSWORD":
		cfg.SMTPPassword = fmt.Sprint(value)
	case "SMTP_FROM_ADDRESS":
		cfg.SMTPFromAddress = strings.ToLower(fmt.Sprint(value))
	case "SMTP_FROM_NAME":
		cfg.SMTPFromName = fmt.Sprint(value)
	case "SMTP_TLS_MODE":
		cfg.SMTPTLSMode = fmt.Sprint(value)
	case "SMTP_TIMEOUT":
		cfg.SMTPTimeout = duration()
	case "MAX_UPLOAD_BYTES":
		cfg.MaxUploadBytes = integer64()
	case "HTTP_READ_TIMEOUT":
		cfg.HTTPReadTimeout = duration()
	case "HTTP_WRITE_TIMEOUT":
		cfg.HTTPWriteTimeout = duration()
	case "HTTP_IDLE_TIMEOUT":
		cfg.HTTPIdleTimeout = duration()
	case "SHUTDOWN_TIMEOUT":
		cfg.ShutdownTimeout = duration()
	case "READINESS_TIMEOUT":
		cfg.ReadinessTimeout = duration()
	case "DB_STATEMENT_TIMEOUT":
		cfg.DBStatementTimeout = duration()
	case "DB_LOCK_TIMEOUT":
		cfg.DBLockTimeout = duration()
	case "DB_MAX_OPEN_CONNS":
		cfg.DBMaxOpenConns = integer()
	case "DB_MAX_IDLE_CONNS":
		cfg.DBMaxIdleConns = integer()
	case "DB_CONN_MAX_LIFETIME":
		cfg.DBConnMaxLifetime = duration()
	case "DB_CONN_MAX_IDLE_TIME":
		cfg.DBConnMaxIdleTime = duration()
	case "REDIS_OPERATION_TIMEOUT":
		cfg.RedisOperationTimeout = duration()
	case "PROJECT_EXPLORER_CACHE_MODE":
		cfg.ProjectExplorerCacheMode = fmt.Sprint(value)
	case "PROJECT_EXPLORER_CACHE_TIMEOUT":
		cfg.ProjectExplorerCacheTimeout = duration()
	case "PROJECT_EXPLORER_CACHE_INDEX_TTL":
		cfg.ProjectExplorerCacheIndexTTL = duration()
	case "PROJECT_EXPLORER_CACHE_ROW_TTL":
		cfg.ProjectExplorerCacheRowTTL = duration()
	case "PROJECT_EXPLORER_CACHE_LOCK_TTL":
		cfg.ProjectExplorerCacheLockTTL = duration()
	case "PROJECT_EXPLORER_CACHE_MAX_VALUE_BYTES":
		cfg.ProjectExplorerCacheMaxBytes = integer()
	case "RATE_LIMIT_REDIS_MAX_IN_FLIGHT":
		cfg.RateLimitRedisMaxInFlight = integer()
	case "RATE_LIMIT_ENABLED":
		cfg.RateLimitEnabled, _ = value.(bool)
	case "RATE_LIMIT_MODE":
		cfg.RateLimitMode = fmt.Sprint(value)
	case "RATE_LIMIT_LOCAL_MAX_KEYS":
		cfg.RateLimitLocalMaxKeys = integer()
	case "UPLOAD_CONCURRENCY":
		cfg.UploadConcurrency = integer()
	case "EXPORT_CONCURRENCY":
		cfg.ExportConcurrency = integer()
	case "REPORT_CONCURRENCY":
		cfg.ReportConcurrency = integer()
	case "SSE_MAX_CONNECTIONS":
		cfg.SSEMaxConnections = integer()
	case "SSE_MAX_CONNECTIONS_PER_IP":
		cfg.SSEMaxConnectionsPerIP = integer()
	case "SSE_MAX_CONNECTIONS_PER_ACTOR":
		cfg.SSEMaxConnectionsPerActor = integer()
	case "SSE_VISIBILITY_TIMEOUT":
		cfg.SSEVisibilityTimeout = duration()
	case "SSE_MAX_CONNECTION_LIFETIME":
		cfg.SSEMaxConnectionLifetime = duration()
	case "SSE_BROKER_CHANNEL":
		cfg.SSEBrokerChannel = fmt.Sprint(value)
	default:
		if applyRateLimitConfigValue(cfg, key, value) {
			return nil
		}
		return fmt.Errorf("unknown system setting %s", key)
	}
	return nil
}

func rateLimitConfigValue(cfg config.Config, key string) (any, bool) {
	policy, field, ok := rateLimitPolicyForKey(&cfg, key)
	if !ok {
		return nil, false
	}
	switch field {
	case "LIMIT":
		return policy.Limit, true
	case "BURST":
		return policy.Burst, true
	case "WINDOW":
		return policy.Window.String(), true
	default:
		return nil, false
	}
}

func applyRateLimitConfigValue(cfg *config.Config, key string, value any) bool {
	policy, field, ok := rateLimitPolicyForKey(cfg, key)
	if !ok {
		return false
	}
	switch field {
	case "LIMIT":
		numeric, _ := numericSettingValue(value)
		policy.Limit = int(numeric)
	case "BURST":
		numeric, _ := numericSettingValue(value)
		policy.Burst = int(numeric)
	case "WINDOW":
		policy.Window, _ = time.ParseDuration(fmt.Sprint(value))
	default:
		return false
	}
	return true
}

func rateLimitPolicyForKey(cfg *config.Config, key string) (*config.RateLimitPolicy, string, bool) {
	prefix := ""
	policies := &cfg.RateLimitIP
	if strings.HasPrefix(key, "RATE_LIMIT_IP_") {
		prefix = "RATE_LIMIT_IP_"
	} else if strings.HasPrefix(key, "RATE_LIMIT_ACTOR_") {
		prefix = "RATE_LIMIT_ACTOR_"
		policies = &cfg.RateLimitActor
	} else {
		return nil, "", false
	}
	remainder := strings.TrimPrefix(key, prefix)
	parts := strings.Split(remainder, "_")
	if len(parts) != 2 {
		return nil, "", false
	}
	var policy *config.RateLimitPolicy
	switch parts[0] {
	case "AUTH":
		policy = &policies.Auth
	case "GENERAL":
		policy = &policies.General
	case "UPLOAD":
		policy = &policies.Upload
	case "EXPORT":
		policy = &policies.Export
	case "SSE":
		policy = &policies.SSE
	default:
		return nil, "", false
	}
	return policy, parts[1], true
}

func sortedSystemSettingDefinitions() []systemSettingDefinition {
	definitions := append([]systemSettingDefinition(nil), systemSettingDefinitions...)
	sort.SliceStable(definitions, func(i, j int) bool {
		if definitions[i].Category == definitions[j].Category {
			return definitions[i].Key < definitions[j].Key
		}
		return systemSettingCategoryOrder(definitions[i].Category) < systemSettingCategoryOrder(definitions[j].Category)
	})
	return definitions
}

func systemSettingCategoryOrder(category string) int {
	order := map[string]int{
		"business": 0, "authentication": 1, "email": 2, "http": 3,
		"database": 4, "cache": 5, "rate_limit": 6, "concurrency": 7, "sse": 8,
	}
	if value, ok := order[category]; ok {
		return value
	}
	return 99
}
