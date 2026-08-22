package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

type staffAccountSecurityMutation struct {
	passwordValue string
	passwordHash  string
	recoveryEmail string
	setPassword   bool
	setRecovery   bool
}

func (s *Service) CreateStaff(ctx context.Context, actorID, actorName string, req CreateStaffRequest) (Record, error) {
	fullName, err := normalizeStaffFullName(req.FullName)
	if err != nil {
		return nil, err
	}
	email, err := normalizeProfileEmail(req.Email)
	if err != nil {
		return nil, err
	}
	phone, err := normalizeProfilePhone(req.Phone)
	if err != nil {
		return nil, err
	}
	department, err := normalizeOptionalStaffText(req.Department, "department", "Phòng ban")
	if err != nil {
		return nil, err
	}
	position, err := normalizeOptionalStaffText(req.Position, "position", "Chức vụ")
	if err != nil {
		return nil, err
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		status = "active"
	}
	if !statusIn(status, "active", "inactive") {
		return nil, validationError("Trạng thái nhân sự không hợp lệ", Record{"field": "status"})
	}
	if err := validatePasswordField(req.Password, "password", "Mật khẩu"); err != nil {
		return nil, err
	}

	recoveryEmail := ""
	if strings.TrimSpace(req.RecoveryEmail) != "" {
		recoveryEmail, err = normalizeRecoveryEmail(req.RecoveryEmail)
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(email, recoveryEmail) {
			return nil, validationError("Email khôi phục phải khác email đăng nhập", Record{"field": "recovery_email"})
		}
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	return s.repo.CreateStaffWithAccount(ctx, actorID, actorName, CreateStaffRequest{
		FullName:      fullName,
		Email:         email,
		Phone:         phone,
		Department:    department,
		Position:      position,
		Status:        status,
		RecoveryEmail: recoveryEmail,
	}, string(passwordHash))
}

func (r *Repository) CreateStaffWithAccount(ctx context.Context, actorID, actorName string, req CreateStaffRequest, passwordHash string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var emailInUse bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE lower(email)=lower($1))`, req.Email).Scan(&emailInUse); err != nil {
		return nil, err
	}
	if emailInUse {
		return nil, conflictError("Email đăng nhập đã được sử dụng", Record{"field": "email"})
	}

	var userID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO users (email,password_hash,full_name,status,recovery_email,recovery_email_verified_at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),CASE WHEN NULLIF($5,'') IS NULL THEN NULL ELSE now() END)
		RETURNING id::text
	`, req.Email, passwordHash, req.FullName, req.Status, req.RecoveryEmail).Scan(&userID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, conflictError("Email đăng nhập đã được sử dụng", Record{"field": "email"})
		}
		return nil, err
	}

	staffRow, err := queryOneRecord(ctx, tx, `
		INSERT INTO staff (user_id,full_name,email,phone,department,position,status)
		VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),$7)
		RETURNING id::text,user_id::text,full_name,email,COALESCE(phone,'') AS phone,department,position,status,created_at,updated_at
	`, userID, req.FullName, req.Email, req.Phone, req.Department, req.Position, req.Status)
	if err != nil {
		return nil, err
	}
	staffID := cleanID(staffRow["id"])
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "user_account_created_by_admin", "user_account", userID, "Quản trị viên tạo tài khoản đăng nhập cho nhân sự", Record{
		"staff_id":           staffID,
		"status":             req.Status,
		"has_recovery_email": req.RecoveryEmail != "",
	}); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "staff_created_with_account", "staff", staffID, "Quản trị viên tạo nhân sự kèm tài khoản đăng nhập", Record{
		"target_user_id": userID,
		"status":         req.Status,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	staffRow["user_roles"] = []Record{}
	staffRow["has_recovery_email"] = req.RecoveryEmail != ""
	if req.RecoveryEmail != "" {
		staffRow["recovery_email_masked"] = maskEmail(req.RecoveryEmail)
	}
	return staffRow, nil
}

func (s *Service) ListStaff(ctx context.Context, actorID string) ([]Record, error) {
	canManageAccountSecurity, err := s.repo.CanUsePermission(ctx, actorID, []string{"staff.account_security.manage"})
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListStaff(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		recoveryEmail := strings.TrimSpace(fmt.Sprint(row["recovery_email_private"]))
		delete(row, "recovery_email_private")
		row["has_recovery_email"] = recoveryEmail != ""
		if canManageAccountSecurity && recoveryEmail != "" {
			row["recovery_email_masked"] = maskEmail(recoveryEmail)
		}
	}
	return rows, nil
}

func (r *Repository) ListStaff(ctx context.Context) ([]Record, error) {
	return r.Query(ctx, `
		SELECT s.*,
			CASE
				WHEN account.recovery_email IS NOT NULL AND account.recovery_email_verified_at IS NOT NULL
				THEN account.recovery_email
				ELSE ''
			END AS recovery_email_private,
			COALESCE((
				SELECT jsonb_agg(jsonb_build_object(
					'id', role.id::text,
					'name', role.name,
					'display_name', role.display_name,
					'project_id', COALESCE(user_role.project_id::text, '')
				) ORDER BY role.display_name)
				FROM user_roles user_role
				JOIN roles role ON role.id=user_role.role_id
				WHERE user_role.user_id=s.user_id
			), '[]'::jsonb) AS user_roles
		FROM staff s
		LEFT JOIN users account ON account.id=s.user_id
		WHERE s.status <> 'deleted'
		ORDER BY s.full_name
	`)
}

func (req UpdateStaffRequest) staffRecord() Record {
	row := Record{}
	values := []struct {
		name  string
		value *string
	}{
		{"full_name", req.FullName},
		{"email", req.Email},
		{"phone", req.Phone},
		{"status", req.Status},
	}
	for _, field := range values {
		if field.value != nil {
			row[field.name] = *field.value
		}
	}
	optionalValues := []struct {
		name  string
		value *string
	}{
		{"department", req.Department},
		{"position", req.Position},
	}
	for _, field := range optionalValues {
		if field.value == nil {
			continue
		}
		if *field.value == "" {
			row[field.name] = nil
			continue
		}
		row[field.name] = *field.value
	}
	return row
}

func (req UpdateStaffRequest) hasAccountSecurityChange() bool {
	return req.UserID != nil || req.NewPassword != "" || req.NewPasswordConfirmation != "" || strings.TrimSpace(req.RecoveryEmail) != ""
}

func (s *Service) UpdateStaff(ctx context.Context, staffID, actorID, actorName string, req UpdateStaffRequest) (Record, error) {
	if req.UserID != nil {
		return nil, validationError("Không thể thay đổi liên kết tài khoản qua màn hình nhân sự", Record{"field": "user_id"})
	}
	if req.FullName != nil {
		value, err := normalizeStaffFullName(*req.FullName)
		if err != nil {
			return nil, err
		}
		req.FullName = &value
	}
	if req.Email != nil {
		value, err := normalizeProfileEmail(*req.Email)
		if err != nil {
			return nil, err
		}
		req.Email = &value
	}
	if req.Phone != nil {
		value, err := normalizeProfilePhone(*req.Phone)
		if err != nil {
			return nil, err
		}
		req.Phone = &value
	}
	if req.Department != nil {
		value, err := normalizeOptionalStaffText(*req.Department, "department", "Phòng ban")
		if err != nil {
			return nil, err
		}
		req.Department = &value
	}
	if req.Position != nil {
		value, err := normalizeOptionalStaffText(*req.Position, "position", "Chức vụ")
		if err != nil {
			return nil, err
		}
		req.Position = &value
	}
	if req.Status != nil {
		value := strings.ToLower(strings.TrimSpace(*req.Status))
		if !statusIn(value, "active", "inactive", "deleted") {
			return nil, validationError("Trạng thái nhân sự không hợp lệ", Record{"field": "status"})
		}
		req.Status = &value
	}

	mutation := staffAccountSecurityMutation{}
	if req.NewPassword != "" || req.NewPasswordConfirmation != "" {
		if err := validateNewPassword(req.NewPassword, req.NewPasswordConfirmation); err != nil {
			return nil, err
		}
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		mutation.passwordValue = req.NewPassword
		mutation.passwordHash = string(passwordHash)
		mutation.setPassword = true
	}
	if strings.TrimSpace(req.RecoveryEmail) != "" {
		recoveryEmail, err := normalizeRecoveryEmail(req.RecoveryEmail)
		if err != nil {
			return nil, err
		}
		mutation.recoveryEmail = recoveryEmail
		mutation.setRecovery = true
	}
	return s.repo.UpdateStaffWithAccountSecurity(ctx, staffID, actorID, actorName, req.staffRecord(), mutation)
}

func normalizeStaffFullName(raw string) (string, error) {
	fullName := strings.Join(strings.Fields(raw), " ")
	if count := len([]rune(fullName)); count < 2 || count > 120 {
		return "", validationError("Họ tên phải có từ 2 đến 120 ký tự", Record{"field": "full_name"})
	}
	return fullName, nil
}

func normalizeOptionalStaffText(raw, field, label string) (string, error) {
	value := strings.Join(strings.Fields(raw), " ")
	if len([]rune(value)) > 120 {
		return "", validationError(label+" không được vượt quá 120 ký tự", Record{"field": field})
	}
	return value, nil
}

func (r *Repository) UpdateStaffWithAccountSecurity(
	ctx context.Context,
	staffID string,
	actorID string,
	actorName string,
	staffValues Record,
	security staffAccountSecurityMutation,
) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	currentStaff, err := queryOneRecord(ctx, tx, `SELECT * FROM staff WHERE id=$1 AND status <> 'deleted' FOR UPDATE`, staffID)
	if err != nil {
		return nil, err
	}
	changedStaffFields := changedRecordFields(currentStaff, staffValues)
	staffRow, err := updateRecord(ctx, tx, "staff", staffColumns(), staffID, staffValues)
	if err != nil {
		return nil, err
	}

	targetUserID := cleanID(staffRow["user_id"])
	passwordChanged := false
	recoveryChanged := false
	identityChangedFields := []string{}
	identityRequiresReauth := false
	sessionsRevoked := int64(0)
	tokensInvalidated := int64(0)
	hasRecoveryEmail := false
	account := Record{}

	if targetUserID == "" {
		if security.setPassword || security.setRecovery {
			return nil, validationError("Nhân sự chưa liên kết tài khoản đăng nhập", Record{"field": "user_id"})
		}
	} else {
		account, err = queryOneRecord(ctx, tx, `
			SELECT id::text,email,password_hash,recovery_email,recovery_email_verified_at,full_name,status
			FROM users
			WHERE id=$1
			FOR UPDATE
		`, targetUserID)
		if err != nil {
			return nil, err
		}

		accountEmail := strings.TrimSpace(fmt.Sprint(staffRow["email"]))
		accountFullName := strings.TrimSpace(fmt.Sprint(staffRow["full_name"]))
		accountStatus := "inactive"
		var hasActiveLinkedStaff bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM staff WHERE user_id=$1 AND status='active')
		`, targetUserID).Scan(&hasActiveLinkedStaff); err != nil {
			return nil, err
		}
		if hasActiveLinkedStaff {
			accountStatus = "active"
			var externalOnly bool
			if err := tx.QueryRowContext(ctx, `
				SELECT EXISTS(
					SELECT 1 FROM user_roles assignment
					JOIN roles role ON role.id=assignment.role_id
					WHERE assignment.user_id=$1 AND role.external_only
				)
			`, targetUserID).Scan(&externalOnly); err != nil {
				return nil, err
			}
			if externalOnly {
				return nil, validationError("Tài khoản đối tác ngoài App không thể được kích hoạt", Record{"field": "status"})
			}
		}

		if !strings.EqualFold(strings.TrimSpace(fmt.Sprint(account["email"])), accountEmail) {
			var emailInUse bool
			if err := tx.QueryRowContext(ctx, `
				SELECT EXISTS(SELECT 1 FROM users WHERE id<>$1 AND lower(email)=lower($2))
			`, targetUserID, accountEmail).Scan(&emailInUse); err != nil {
				return nil, err
			}
			if emailInUse {
				return nil, conflictError("Email đăng nhập đã được sử dụng", Record{"field": "email"})
			}
		}

		if fmt.Sprint(account["full_name"]) != accountFullName {
			identityChangedFields = append(identityChangedFields, "full_name")
		}
		if fmt.Sprint(account["email"]) != accountEmail {
			identityChangedFields = append(identityChangedFields, "email")
			identityRequiresReauth = true
		}
		if fmt.Sprint(account["status"]) != accountStatus {
			identityChangedFields = append(identityChangedFields, "status")
			identityRequiresReauth = true
		}
		if len(identityChangedFields) > 0 {
			if _, err := tx.ExecContext(ctx, `
				UPDATE users SET full_name=$2,email=$3,status=$4,updated_at=now() WHERE id=$1
			`, targetUserID, accountFullName, accountEmail, accountStatus); err != nil {
				var pgErr *pgconn.PgError
				if errors.As(err, &pgErr) && pgErr.Code == "23505" {
					return nil, conflictError("Email đăng nhập đã được sử dụng", Record{"field": "email"})
				}
				return nil, err
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE staff
				SET full_name=$2,email=$3,updated_at=CASE WHEN full_name<>$2 OR email<>$3 THEN now() ELSE updated_at END
				WHERE user_id=$1 AND status<>'deleted'
			`, targetUserID, accountFullName, accountEmail); err != nil {
				return nil, err
			}
			account["full_name"] = accountFullName
			account["email"] = accountEmail
			account["status"] = accountStatus
		}

		if security.setRecovery {
			if strings.EqualFold(accountEmail, security.recoveryEmail) {
				return nil, validationError("Email khôi phục phải khác email đăng nhập", Record{"field": "recovery_email"})
			}
			currentRecovery := strings.TrimSpace(fmt.Sprint(account["recovery_email"]))
			recoveryChanged = !strings.EqualFold(currentRecovery, security.recoveryEmail) || account["recovery_email_verified_at"] == nil
			if recoveryChanged {
				if _, err := tx.ExecContext(ctx, `
					UPDATE users
					SET recovery_email=$2,recovery_email_verified_at=now(),updated_at=now()
					WHERE id=$1
				`, targetUserID, security.recoveryEmail); err != nil {
					return nil, err
				}
				account["recovery_email"] = security.recoveryEmail
				account["recovery_email_verified_at"] = true
			}
			hasRecoveryEmail = true
		} else {
			hasRecoveryEmail = strings.TrimSpace(fmt.Sprint(account["recovery_email"])) != "" && account["recovery_email_verified_at"] != nil
		}

		if security.setPassword {
			if passwordMatches(fmt.Sprint(account["password_hash"]), security.passwordValue) {
				return nil, validationError("Mật khẩu mới phải khác mật khẩu hiện tại", Record{"field": "new_password"})
			}
			if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash=$2,updated_at=now() WHERE id=$1`, targetUserID, security.passwordHash); err != nil {
				return nil, err
			}
			passwordChanged = true
		}

		if passwordChanged || identityRequiresReauth {
			sessionResult, err := tx.ExecContext(ctx, `
				UPDATE auth_sessions
				SET revoked_at=COALESCE(revoked_at,now())
				WHERE user_id=$1 AND revoked_at IS NULL
			`, targetUserID)
			if err != nil {
				return nil, err
			}
			sessionsRevoked, _ = sessionResult.RowsAffected()
		}

		if passwordChanged || recoveryChanged {
			tokenResult, err := tx.ExecContext(ctx, `
				UPDATE auth_email_tokens
				SET invalidated_at=now()
				WHERE user_id=$1 AND consumed_at IS NULL AND invalidated_at IS NULL
			`, targetUserID)
			if err != nil {
				return nil, err
			}
			tokensInvalidated, _ = tokenResult.RowsAffected()
		}
	}

	if len(changedStaffFields) > 0 {
		if err := addAuditTx(ctx, tx, "", actorID, actorName, "staff_updated_by_admin", "staff", staffID, "Quản trị viên cập nhật thông tin nhân sự", Record{
			"changed_fields": changedStaffFields,
			"target_user_id": targetUserID,
		}); err != nil {
			return nil, err
		}
	}
	if passwordChanged {
		if err := addAuditTx(ctx, tx, "", actorID, actorName, "user_password_changed_by_admin", "user_account", targetUserID, "Quản trị viên đặt mật khẩu mới cho tài khoản nhân sự", Record{
			"staff_id":           staffID,
			"sessions_revoked":   sessionsRevoked,
			"tokens_invalidated": tokensInvalidated,
		}); err != nil {
			return nil, err
		}
	}
	if len(identityChangedFields) > 0 {
		if err := addAuditTx(ctx, tx, "", actorID, actorName, "user_identity_synced_by_admin", "user_account", targetUserID, "Quản trị viên đồng bộ danh tính tài khoản từ hồ sơ nhân sự", Record{
			"staff_id":         staffID,
			"changed_fields":   identityChangedFields,
			"sessions_revoked": sessionsRevoked,
		}); err != nil {
			return nil, err
		}
	}
	if recoveryChanged {
		if err := addAuditTx(ctx, tx, "", actorID, actorName, "user_recovery_email_set_by_admin", "user_account", targetUserID, "Quản trị viên đặt email khôi phục cho tài khoản nhân sự", Record{
			"staff_id":              staffID,
			"recovery_email_masked": maskEmail(security.recoveryEmail),
			"tokens_invalidated":    tokensInvalidated,
		}); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	staffRow["password_changed"] = passwordChanged
	staffRow["recovery_email_changed"] = recoveryChanged
	staffRow["identity_changed"] = len(identityChangedFields) > 0
	staffRow["reauth_required"] = targetUserID == actorID && (passwordChanged || identityRequiresReauth)
	staffRow["has_recovery_email"] = hasRecoveryEmail
	if hasRecoveryEmail {
		staffRow["recovery_email_masked"] = maskEmail(strings.TrimSpace(fmt.Sprint(account["recovery_email"])))
	}
	return staffRow, nil
}

func changedRecordFields(current Record, requested Record) []string {
	fields := make([]string, 0, len(requested))
	for name, value := range requested {
		if fmt.Sprint(current[name]) != fmt.Sprint(value) {
			fields = append(fields, name)
		}
	}
	sort.Strings(fields)
	return fields
}
