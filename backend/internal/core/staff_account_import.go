package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

const (
	staffAccountImportMaximumRows      = 100
	staffAccountImportMaximumFileBytes = int64(1024 * 1024)
)

var restrictedStaffImportRoles = map[string]bool{
	"admin":     true,
	"client":    true,
	"developer": true,
}

type normalizedStaffAccountImportRow struct {
	RowNumber     int    `json:"row_number"`
	FullName      string `json:"full_name"`
	LoginEmail    string `json:"login_email"`
	Password      string `json:"password"`
	RecoveryEmail string `json:"recovery_email"`
	Phone         string `json:"phone"`
	Department    string `json:"department"`
	Position      string `json:"position"`
	RoleID        string `json:"role_id"`
	RoleCode      string `json:"role_code"`
	RoleName      string `json:"role_name"`
	PasswordHash  string `json:"-"`
}

type staffImportRole struct {
	ID           string
	Code         string
	DisplayName  string
	ExternalOnly bool
}

type staffAccountImportValidation struct {
	Preview Record
	Rows    []normalizedStaffAccountImportRow
	Hash    string
}

func (s *Service) PreviewStaffAccountImport(ctx context.Context, req StaffAccountImportPreviewRequest) (Record, error) {
	validation, err := s.validateStaffAccountImport(ctx, req)
	if err != nil {
		return nil, err
	}
	return validation.Preview, nil
}

func (s *Service) ExecuteStaffAccountImport(ctx context.Context, actorID, actorName string, req StaffAccountImportExecuteRequest) (Record, error) {
	if _, err := uuid.Parse(strings.TrimSpace(req.IdempotencyKey)); err != nil {
		return nil, validationError("Mã chống tạo trùng không hợp lệ", Record{"field": "idempotency_key"})
	}
	if stored, found, err := s.repo.StaffAccountImportResult(ctx, actorID, strings.TrimSpace(req.IdempotencyKey), strings.TrimSpace(req.PreviewHash)); err != nil {
		return nil, err
	} else if found {
		stored["idempotent_replay"] = true
		return stored, nil
	}
	validation, err := s.validateStaffAccountImport(ctx, req.StaffAccountImportPreviewRequest)
	if err != nil {
		return nil, err
	}
	if number(validation.Preview["invalid_count"]) > 0 {
		return nil, validationError("Danh sách import còn dòng không hợp lệ", validation.Preview)
	}
	if strings.TrimSpace(req.PreviewHash) == "" || req.PreviewHash != validation.Hash {
		return nil, conflictError("Dữ liệu đã thay đổi sau bước xem trước. Vui lòng xem trước lại", Record{"field": "preview_hash"})
	}
	for index := range validation.Rows {
		hash, err := bcrypt.GenerateFromPassword([]byte(validation.Rows[index].Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		validation.Rows[index].PasswordHash = string(hash)
	}
	return s.repo.ExecuteStaffAccountImport(ctx, actorID, actorName, strings.TrimSpace(req.IdempotencyKey), strings.TrimSpace(req.SourceFilename), validation.Hash, validation.Rows)
}

func (s *Service) validateStaffAccountImport(ctx context.Context, req StaffAccountImportPreviewRequest) (staffAccountImportValidation, error) {
	filename := strings.TrimSpace(req.SourceFilename)
	if filename == "" || len([]rune(filename)) > 255 {
		return staffAccountImportValidation{}, validationError("Tên tệp import không hợp lệ", Record{"field": "source_filename"})
	}
	extension := strings.ToLower(filepath.Ext(filename))
	if extension != ".xlsx" && extension != ".csv" {
		return staffAccountImportValidation{}, validationError("Chỉ chấp nhận tệp .xlsx hoặc .csv", Record{"field": "source_filename"})
	}
	if req.SourceSizeBytes <= 0 || req.SourceSizeBytes > staffAccountImportMaximumFileBytes {
		return staffAccountImportValidation{}, validationError("Tệp import phải có dung lượng không quá 1 MB", Record{"field": "source_size_bytes", "maximum": staffAccountImportMaximumFileBytes})
	}
	if len(req.Rows) == 0 || len(req.Rows) > staffAccountImportMaximumRows {
		return staffAccountImportValidation{}, validationError("Mỗi lần import phải có từ 1 đến 100 tài khoản", Record{"field": "rows", "maximum": staffAccountImportMaximumRows})
	}

	roles, err := s.repo.StaffImportRoles(ctx)
	if err != nil {
		return staffAccountImportValidation{}, err
	}
	roleByCode := make(map[string]staffImportRole, len(roles))
	for _, role := range roles {
		role.Code = strings.ToLower(strings.TrimSpace(role.Code))
		roleByCode[role.Code] = role
	}

	normalizedRows := make([]normalizedStaffAccountImportRow, len(req.Rows))
	issues := make([][]Record, len(req.Rows))
	warnings := make([][]Record, len(req.Rows))
	loginEmailRows := map[string][]int{}
	recoveryEmailRows := map[string][]int{}
	rowNumbers := map[int]bool{}

	for index, raw := range req.Rows {
		issues[index] = []Record{}
		warnings[index] = []Record{}
		rowNumber := raw.RowNumber
		if rowNumber < 2 || rowNumbers[rowNumber] {
			rowNumber = index + 2
		}
		rowNumbers[rowNumber] = true
		row := normalizedStaffAccountImportRow{RowNumber: rowNumber}
		value, valueErr := normalizeStaffFullName(raw.FullName)
		row.FullName = normalizedImportValue(value, valueErr, "full_name", &issues[index])
		value, valueErr = normalizeProfileEmail(raw.LoginEmail)
		row.LoginEmail = normalizedImportValue(value, valueErr, "login_email", &issues[index])
		value, valueErr = normalizeProfilePhone(raw.Phone)
		row.Phone = normalizedImportValue(value, valueErr, "phone", &issues[index])
		value, valueErr = normalizeOptionalStaffText(raw.Department, "department", "Phòng ban")
		row.Department = normalizedImportValue(value, valueErr, "department", &issues[index])
		value, valueErr = normalizeOptionalStaffText(raw.Position, "position", "Chức vụ")
		row.Position = normalizedImportValue(value, valueErr, "position", &issues[index])
		row.Password = raw.Password
		if err := validatePasswordField(raw.Password, "password", "Mật khẩu"); err != nil {
			issues[index] = append(issues[index], importIssue(err, "password"))
		}
		if strings.TrimSpace(raw.RecoveryEmail) != "" {
			value, valueErr = normalizeRecoveryEmail(raw.RecoveryEmail)
			row.RecoveryEmail = normalizedImportValue(value, valueErr, "recovery_email", &issues[index])
			if row.RecoveryEmail != "" && strings.EqualFold(row.RecoveryEmail, row.LoginEmail) {
				issues[index] = append(issues[index], Record{"field": "recovery_email", "message": "Email khôi phục phải khác email đăng nhập"})
			}
		}
		row.RoleCode = strings.ToLower(strings.TrimSpace(raw.RoleCode))
		role, roleExists := roleByCode[row.RoleCode]
		if row.RoleCode == "" {
			issues[index] = append(issues[index], Record{"field": "role_code", "message": "Mã vai trò là bắt buộc"})
		} else if !roleExists {
			issues[index] = append(issues[index], Record{"field": "role_code", "message": "Mã vai trò không tồn tại"})
		} else if role.ExternalOnly || restrictedStaffImportRoles[role.Code] {
			issues[index] = append(issues[index], Record{"field": "role_code", "message": "Vai trò này không được phép import tài khoản"})
		} else {
			row.RoleID = role.ID
			row.RoleName = role.DisplayName
		}
		if row.LoginEmail != "" {
			loginEmailRows[row.LoginEmail] = append(loginEmailRows[row.LoginEmail], index)
		}
		if row.RecoveryEmail != "" {
			recoveryEmailRows[row.RecoveryEmail] = append(recoveryEmailRows[row.RecoveryEmail], index)
		}
		normalizedRows[index] = row
	}

	for email, indexes := range loginEmailRows {
		if len(indexes) > 1 {
			for _, index := range indexes {
				issues[index] = append(issues[index], Record{"field": "login_email", "message": "Email đăng nhập bị trùng trong tệp", "value": email})
			}
		}
	}
	for email, indexes := range recoveryEmailRows {
		if len(indexes) > 1 {
			for _, index := range indexes {
				warnings[index] = append(warnings[index], Record{"field": "recovery_email", "message": "Email khôi phục được dùng cho nhiều tài khoản", "value": maskEmail(email)})
			}
		}
	}
	existing, err := s.repo.ExistingStaffImportEmails(ctx, sortedKeys(loginEmailRows))
	if err != nil {
		return staffAccountImportValidation{}, err
	}
	for email := range existing {
		for _, index := range loginEmailRows[email] {
			issues[index] = append(issues[index], Record{"field": "login_email", "message": "Email đăng nhập đã tồn tại trong hệ thống"})
		}
	}

	previewRows := make([]Record, 0, len(normalizedRows))
	validCount := 0
	warningCount := 0
	for index, row := range normalizedRows {
		valid := len(issues[index]) == 0
		if valid {
			validCount++
		}
		warningCount += len(warnings[index])
		previewRows = append(previewRows, Record{
			"row_number": row.RowNumber, "full_name": row.FullName, "login_email": row.LoginEmail,
			"phone": row.Phone, "department": row.Department, "position": row.Position,
			"role_code": row.RoleCode, "role_name": row.RoleName,
			"has_recovery_email": row.RecoveryEmail != "", "recovery_email_masked": maskedImportEmail(row.RecoveryEmail),
			"password_valid": validPasswordOnly(issues[index]), "valid": valid,
			"errors": issues[index], "warnings": warnings[index],
		})
	}

	hash := ""
	if validCount == len(normalizedRows) {
		payload, err := json.Marshal(Record{"source_filename": filename, "source_size_bytes": req.SourceSizeBytes, "rows": normalizedRows})
		if err != nil {
			return staffAccountImportValidation{}, err
		}
		mac := hmac.New(sha256.New, []byte(s.jwtSecret))
		if _, err := mac.Write(payload); err != nil {
			return staffAccountImportValidation{}, err
		}
		hash = hex.EncodeToString(mac.Sum(nil))
	}
	preview := Record{
		"source_filename": filename, "total_count": len(normalizedRows), "valid_count": validCount,
		"invalid_count": len(normalizedRows) - validCount, "warning_count": warningCount,
		"preview_hash": hash, "rows": previewRows,
	}
	return staffAccountImportValidation{Preview: preview, Rows: normalizedRows, Hash: hash}, nil
}

func normalizedImportValue(value string, err error, field string, issues *[]Record) string {
	if err != nil {
		*issues = append(*issues, importIssue(err, field))
		return ""
	}
	return value
}

func importIssue(err error, fallbackField string) Record {
	issue := Record{"field": fallbackField, "message": err.Error()}
	var statusErr StatusError
	if errors.As(err, &statusErr) {
		if details, ok := statusErr.Details.(Record); ok {
			if field := strings.TrimSpace(fmt.Sprint(details["field"])); field != "" {
				issue["field"] = field
			}
		}
	}
	return issue
}

func validPasswordOnly(issues []Record) bool {
	for _, issue := range issues {
		if issue["field"] == "password" {
			return false
		}
	}
	return true
}

func maskedImportEmail(value string) string {
	if value == "" {
		return ""
	}
	return maskEmail(value)
}

func sortedKeys(values map[string][]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (r *Repository) StaffImportRoles(ctx context.Context) ([]staffImportRole, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id::text,name,display_name,external_only FROM roles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []staffImportRole{}
	for rows.Next() {
		var role staffImportRole
		if err := rows.Scan(&role.ID, &role.Code, &role.DisplayName, &role.ExternalOnly); err != nil {
			return nil, err
		}
		result = append(result, role)
	}
	return result, rows.Err()
}

func (r *Repository) ExistingStaffImportEmails(ctx context.Context, emails []string) (map[string]bool, error) {
	result := map[string]bool{}
	if len(emails) == 0 {
		return result, nil
	}
	placeholders := make([]string, len(emails))
	args := make([]any, len(emails))
	for index, email := range emails {
		placeholders[index] = fmt.Sprintf("$%d", index+1)
		args[index] = email
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT lower(email)
		FROM users
		WHERE lower(email) IN (`+strings.Join(placeholders, ",")+`)
		UNION
		SELECT lower(email)
		FROM staff
		WHERE status <> 'deleted' AND lower(email) IN (`+strings.Join(placeholders, ",")+`)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		result[email] = true
	}
	return result, rows.Err()
}

func (r *Repository) StaffAccountImportResult(ctx context.Context, actorID, idempotencyKey, previewHash string) (Record, bool, error) {
	var storedActorID sql.NullString
	var storedHash string
	var storedJSON []byte
	err := r.db.QueryRowContext(ctx, `SELECT actor_id::text,preview_hash,result_json FROM staff_account_imports WHERE idempotency_key=$1`, idempotencyKey).Scan(&storedActorID, &storedHash, &storedJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !storedActorID.Valid || storedActorID.String != actorID || previewHash == "" || storedHash != previewHash {
		return nil, false, conflictError("Mã chống tạo trùng đã được dùng cho một danh sách import khác", Record{"field": "idempotency_key"})
	}
	var result Record
	if err := json.Unmarshal(storedJSON, &result); err != nil {
		return nil, false, err
	}
	return result, true, nil
}

func (r *Repository) ExecuteStaffAccountImport(ctx context.Context, actorID, actorName, idempotencyKey, sourceFilename, previewHash string, rows []normalizedStaffAccountImportRow) (result Record, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, idempotencyKey); err != nil {
		return nil, err
	}
	var storedHash string
	var storedActorID sql.NullString
	var storedJSON []byte
	if err := tx.QueryRowContext(ctx, `SELECT actor_id::text,preview_hash,result_json FROM staff_account_imports WHERE idempotency_key=$1`, idempotencyKey).Scan(&storedActorID, &storedHash, &storedJSON); err == nil {
		if !storedActorID.Valid || storedActorID.String != actorID || storedHash != previewHash {
			return nil, conflictError("Mã chống tạo trùng đã được dùng cho một danh sách import khác", Record{"field": "idempotency_key"})
		}
		if err := json.Unmarshal(storedJSON, &result); err != nil {
			return nil, err
		}
		result["idempotent_replay"] = true
		return result, tx.Commit()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	importID := uuid.NewString()
	created := make([]Record, 0, len(rows))
	for _, row := range rows {
		var userID string
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO users (email,password_hash,full_name,status,recovery_email,recovery_email_verified_at,must_change_password)
			VALUES ($1,$2,$3,'active',NULLIF($4,''),CASE WHEN NULLIF($4,'') IS NULL THEN NULL ELSE now() END,true)
			RETURNING id::text
		`, row.LoginEmail, row.PasswordHash, row.FullName, row.RecoveryEmail).Scan(&userID); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return nil, conflictError("Email đăng nhập đã được sử dụng trong lúc import", Record{"row_number": row.RowNumber, "field": "login_email"})
			}
			return nil, err
		}
		var staffID string
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO staff (user_id,full_name,email,phone,department,position,status)
			VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),'active')
			RETURNING id::text
		`, userID, row.FullName, row.LoginEmail, row.Phone, row.Department, row.Position).Scan(&staffID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_roles (user_id,role_id,project_id) VALUES ($1,$2,NULL)`, userID, row.RoleID); err != nil {
			return nil, err
		}
		if err := addAuditTx(ctx, tx, "", actorID, actorName, "staff_account_imported", "staff", staffID, "Nhà phát triển import và kích hoạt tài khoản nhân sự", Record{
			"import_id": importID, "target_user_id": userID, "role_code": row.RoleCode, "must_change_password": true,
		}); err != nil {
			return nil, err
		}
		if err := addAuditTx(ctx, tx, "", actorID, actorName, "user_role_assigned_by_import", "user_account", userID, "Nhà phát triển gán vai trò toàn hệ thống khi import", Record{
			"import_id": importID, "role_code": row.RoleCode, "role_name": row.RoleName,
		}); err != nil {
			return nil, err
		}
		created = append(created, Record{"row_number": row.RowNumber, "staff_id": staffID, "user_id": userID, "login_email": row.LoginEmail, "role_code": row.RoleCode})
	}
	result = Record{
		"import_id": importID, "source_filename": sourceFilename, "created_count": len(rows),
		"activated_count": len(rows), "role_assignment_count": len(rows), "must_change_password_count": len(rows),
		"items": created, "idempotent_replay": false,
	}
	storedResult := Record{
		"import_id": importID, "source_filename": sourceFilename, "created_count": len(rows),
		"activated_count": len(rows), "role_assignment_count": len(rows), "must_change_password_count": len(rows),
		"items": created,
	}
	resultJSON, err := json.Marshal(storedResult)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO staff_account_imports (id,idempotency_key,actor_id,source_filename,preview_hash,row_count,created_count,role_assignment_count,result_json)
		VALUES ($1,$2,$3,$4,$5,$6,$6,$6,$7::jsonb)
	`, importID, idempotencyKey, actorID, sourceFilename, previewHash, len(rows), resultJSON); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "staff_account_import_completed", "staff_account_import", importID, "Hoàn tất import tài khoản nhân sự", Record{
		"source_filename": sourceFilename, "row_count": len(rows), "created_count": len(rows), "role_assignment_count": len(rows),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
