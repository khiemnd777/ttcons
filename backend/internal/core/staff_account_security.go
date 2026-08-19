package core

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type staffAccountSecurityMutation struct {
	passwordHash  string
	recoveryEmail string
	setPassword   bool
	setRecovery   bool
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
		{"user_id", req.UserID},
		{"full_name", req.FullName},
		{"email", req.Email},
		{"phone", req.Phone},
		{"department", req.Department},
		{"position", req.Position},
		{"status", req.Status},
	}
	for _, field := range values {
		if field.value != nil {
			row[field.name] = *field.value
		}
	}
	return row
}

func (req UpdateStaffRequest) hasAccountSecurityChange() bool {
	return strings.TrimSpace(req.NewPassword) != "" || strings.TrimSpace(req.RecoveryEmail) != ""
}

func (s *Service) UpdateStaff(ctx context.Context, staffID, actorID, actorName string, req UpdateStaffRequest) (Record, error) {
	mutation := staffAccountSecurityMutation{}
	if strings.TrimSpace(req.NewPassword) != "" {
		if err := validatePasswordValue(req.NewPassword); err != nil {
			return nil, err
		}
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
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
	sessionsRevoked := int64(0)
	tokensInvalidated := int64(0)
	hasRecoveryEmail := false

	if security.setPassword || security.setRecovery {
		if targetUserID == "" {
			return nil, validationError("Nhân sự chưa liên kết tài khoản đăng nhập", Record{"field": "user_id"})
		}
		account, err := queryOneRecord(ctx, tx, `
			SELECT id::text,email,recovery_email,recovery_email_verified_at,status
			FROM users
			WHERE id=$1
			FOR UPDATE
		`, targetUserID)
		if err != nil {
			return nil, err
		}

		if security.setRecovery {
			if strings.EqualFold(strings.TrimSpace(fmt.Sprint(account["email"])), security.recoveryEmail) {
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
			}
			hasRecoveryEmail = true
		} else {
			hasRecoveryEmail = strings.TrimSpace(fmt.Sprint(account["recovery_email"])) != "" && account["recovery_email_verified_at"] != nil
		}

		if security.setPassword {
			if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash=$2,updated_at=now() WHERE id=$1`, targetUserID, security.passwordHash); err != nil {
				return nil, err
			}
			passwordChanged = true
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
	staffRow["reauth_required"] = passwordChanged && targetUserID == actorID
	staffRow["has_recovery_email"] = hasRecoveryEmail
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
