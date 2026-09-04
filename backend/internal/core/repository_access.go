package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
	"time"
)

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT id, email, password_hash, full_name, status, must_change_password, created_at, updated_at
		FROM users
		WHERE email=$1
		  AND status='active'
		  AND NOT EXISTS (
			SELECT 1
			FROM user_roles assignment
			JOIN roles role ON role.id=assignment.role_id
			WHERE assignment.user_id=users.id
			  AND role.external_only
		  )
	`, email)
}

func (r *Repository) GetOwnProfile(ctx context.Context, userID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT account.id::text,account.email,account.full_name,account.status,account.must_change_password,
			account.created_at,account.updated_at,
			COALESCE(CASE WHEN account.recovery_email_verified_at IS NOT NULL THEN account.recovery_email END,'') AS recovery_email_private,
			COALESCE(pending.target_email,'') AS pending_recovery_email_private,
			COALESCE(profile.phone,'') AS phone,
			COALESCE(profile.department,'') AS department,
			COALESCE(profile.position,'') AS position
		FROM users account
		LEFT JOIN LATERAL (
			SELECT staff.phone,staff.department,staff.position
			FROM staff
			WHERE staff.user_id=account.id
			  AND staff.status <> 'deleted'
			ORDER BY CASE WHEN staff.status='active' THEN 0 ELSE 1 END,staff.updated_at DESC,staff.id
			LIMIT 1
		) profile ON true
		LEFT JOIN LATERAL (
			SELECT token.target_email
			FROM auth_email_tokens token
			WHERE token.user_id=account.id
			  AND token.purpose='verify_recovery_email'
			  AND token.consumed_at IS NULL
			  AND token.invalidated_at IS NULL
			  AND token.expires_at > now()
			ORDER BY token.created_at DESC,token.id
			LIMIT 1
		) pending ON true
		WHERE account.id=$1
		  AND account.status='active'
	`, userID)
}

func (r *Repository) UpdateOwnProfile(ctx context.Context, userID, actorName, fullName, email, phone string) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var currentFullName string
	var currentEmail string
	if err := tx.QueryRowContext(ctx, `
		SELECT full_name,email
		FROM users
		WHERE id=$1 AND status='active'
		FOR UPDATE
	`, userID).Scan(&currentFullName, &currentEmail); err != nil {
		return false, err
	}

	var staffCount int
	var currentPhone string
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*),COALESCE((
			SELECT linked.phone
			FROM staff linked
			WHERE linked.user_id=$1 AND linked.status <> 'deleted'
			ORDER BY CASE WHEN linked.status='active' THEN 0 ELSE 1 END,linked.updated_at DESC,linked.id
			LIMIT 1
		),'')
		FROM staff
		WHERE user_id=$1 AND status <> 'deleted'
	`, userID).Scan(&staffCount, &currentPhone); err != nil {
		return false, err
	}
	if staffCount == 0 && phone != "" {
		return false, validationError("Tài khoản chưa được liên kết hồ sơ nhân sự nên chưa thể lưu số điện thoại", Record{"field": "phone"})
	}

	emailChanged := currentEmail != email
	changedFields := make([]string, 0, 3)
	if currentFullName != fullName {
		changedFields = append(changedFields, "full_name")
	}
	if emailChanged {
		changedFields = append(changedFields, "email")
	}
	if currentPhone != phone {
		changedFields = append(changedFields, "phone")
	}
	if len(changedFields) == 0 {
		return false, nil
	}

	var emailInUse bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM users
			WHERE id <> $1 AND lower(email)=lower($2)
		)
	`, userID, email).Scan(&emailInUse); err != nil {
		return false, err
	}
	if emailInUse {
		return false, conflictError("Email đăng nhập đã được sử dụng", Record{"field": "email"})
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET full_name=$2,email=$3,updated_at=now()
		WHERE id=$1 AND status='active'
	`, userID, fullName, email); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return false, conflictError("Email đăng nhập đã được sử dụng", Record{"field": "email"})
		}
		return false, err
	}
	if staffCount > 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE staff
			SET full_name=$2,email=$3,phone=NULLIF($4,''),updated_at=now()
			WHERE user_id=$1 AND status <> 'deleted'
		`, userID, fullName, email, phone); err != nil {
			return false, err
		}
	}
	if emailChanged {
		if _, err := tx.ExecContext(ctx, `
			UPDATE auth_sessions
			SET revoked_at=COALESCE(revoked_at,now())
			WHERE user_id=$1 AND revoked_at IS NULL
		`, userID); err != nil {
			return false, err
		}
	}
	if err := addAuditTx(ctx, tx, "", userID, actorName, "user_profile_updated", "user_account", userID, "Người dùng cập nhật hồ sơ cá nhân", Record{
		"changed_fields":  changedFields,
		"session_revoked": emailChanged,
	}); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return emailChanged, nil
}

func (r *Repository) ChangeOwnPassword(ctx context.Context, userID, actorName, passwordHash string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE users
		SET password_hash=$2,must_change_password=false,updated_at=now()
		WHERE id=$1 AND status='active'
	`, userID, passwordHash)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE auth_sessions
		SET revoked_at=COALESCE(revoked_at,now())
		WHERE user_id=$1 AND revoked_at IS NULL
	`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE auth_email_tokens
		SET invalidated_at=now()
		WHERE user_id=$1 AND consumed_at IS NULL AND invalidated_at IS NULL
	`, userID); err != nil {
		return err
	}
	if err := addAuditTx(ctx, tx, "", userID, actorName, "user_password_changed", "user_account", userID, "Người dùng đổi mật khẩu", Record{
		"session_revoked":                 true,
		"account_recovery_tokens_revoked": true,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) InitialPasswordChangeRequired(ctx context.Context, userID string) (bool, error) {
	var required bool
	err := r.db.QueryRowContext(ctx, `
		SELECT must_change_password
		FROM users
		WHERE id=$1 AND status='active'
	`, userID).Scan(&required)
	return required, err
}

func (r *Repository) UserRoles(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.Query(ctx, `
		SELECT DISTINCT r.display_name
		FROM user_roles ur
		JOIN roles r ON r.id=ur.role_id
		WHERE ur.user_id=$1
		ORDER BY r.display_name
	`, userID)
	if err != nil {
		return nil, err
	}
	roles := make([]string, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, fmt.Sprint(row["display_name"]))
	}
	return roles, nil
}

func (r *Repository) UserHasGlobalRole(ctx context.Context, userID, roleName string) (bool, error) {
	var assigned bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM user_roles ur
			JOIN roles r ON r.id=ur.role_id
			WHERE ur.user_id=$1
			  AND ur.project_id IS NULL
			  AND r.name=$2
		)
	`, userID, roleName).Scan(&assigned)
	return assigned, err
}

func (r *Repository) UserPermissions(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.Query(ctx, `
		SELECT DISTINCT p.key
		FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id=ur.role_id
		JOIN permissions p ON p.id=rp.permission_id
		WHERE ur.user_id=$1
		ORDER BY p.key
	`, userID)
	if err != nil {
		return nil, err
	}
	permissions := make([]string, 0, len(rows))
	for _, row := range rows {
		permissions = append(permissions, fmt.Sprint(row["key"]))
	}
	return permissions, nil
}

func (r *Repository) CreateAuthSession(ctx context.Context, userID, refreshHash, accessTokenID, userAgent, ipAddress string, expiresAt time.Time) (Record, error) {
	return r.QueryOne(ctx, `
		INSERT INTO auth_sessions (user_id, refresh_token_hash, access_token_id, user_agent, ip_address, expires_at)
		SELECT account.id,$2,$3,NULLIF($4,''),NULLIF($5,''),$6
		FROM users account
		WHERE account.id=$1
		  AND account.status='active'
		  AND NOT EXISTS (
			SELECT 1
			FROM user_roles assignment
			JOIN roles role ON role.id=assignment.role_id
			WHERE assignment.user_id=account.id
			  AND role.external_only
		  )
		RETURNING id::text,user_id::text,expires_at
	`, userID, refreshHash, accessTokenID, strings.TrimSpace(userAgent), strings.TrimSpace(ipAddress), expiresAt)
}

func (r *Repository) RotateAuthSession(ctx context.Context, currentRefreshHash, nextRefreshHash, accessTokenID, userAgent, ipAddress string, expiresAt time.Time) (Record, error) {
	return r.QueryOne(ctx, `
		UPDATE auth_sessions s
		SET refresh_token_hash=$2,
			access_token_id=$3,
			user_agent=NULLIF($4,''),
			ip_address=NULLIF($5,''),
			last_used_at=now(),
			expires_at=$6
		FROM users u
		WHERE s.refresh_token_hash=$1
		  AND s.user_id=u.id
		  AND u.status='active'
		  AND NOT EXISTS (
			SELECT 1
			FROM user_roles assignment
			JOIN roles role ON role.id=assignment.role_id
			WHERE assignment.user_id=u.id
			  AND role.external_only
		  )
		  AND s.revoked_at IS NULL
		  AND s.expires_at > now()
		RETURNING s.id::text, s.user_id::text, s.expires_at
	`, currentRefreshHash, nextRefreshHash, accessTokenID, strings.TrimSpace(userAgent), strings.TrimSpace(ipAddress), expiresAt)
}

func (r *Repository) RevokeAuthSessionByRefreshHash(ctx context.Context, refreshHash string) error {
	return r.Exec(ctx, `
		UPDATE auth_sessions
		SET revoked_at=COALESCE(revoked_at, now())
		WHERE refresh_token_hash=$1
	`, refreshHash)
}

func (r *Repository) AuthSessionActive(ctx context.Context, userID, sessionID, accessTokenID string) (bool, error) {
	rows, err := r.Query(ctx, `
		SELECT COUNT(*) AS total
		FROM auth_sessions s
		JOIN users u ON u.id=s.user_id
		WHERE s.id=$1
		  AND s.user_id=$2
		  AND s.access_token_id=$3
		  AND s.revoked_at IS NULL
		  AND s.expires_at > now()
		  AND u.status='active'
		  AND NOT EXISTS (
			SELECT 1
			FROM user_roles assignment
			JOIN roles role ON role.id=assignment.role_id
			WHERE assignment.user_id=u.id
			  AND role.external_only
		  )
	`, sessionID, userID, accessTokenID)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && number(rows[0]["total"]) > 0, nil
}

func (r *Repository) RoleExternalOnly(ctx context.Context, roleID string) (bool, error) {
	var externalOnly bool
	if err := r.db.QueryRowContext(ctx, `SELECT external_only FROM roles WHERE id=$1`, roleID).Scan(&externalOnly); err != nil {
		return false, err
	}
	return externalOnly, nil
}

func (r *Repository) ReplaceRolePermissions(ctx context.Context, roleID string, permissionIDs []string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var roleIDText string
	var roleName string
	var roleDisplayName string
	var externalOnly bool
	if err := tx.QueryRowContext(ctx, `SELECT id::text,name,display_name,external_only FROM roles WHERE id=$1 FOR UPDATE`, roleID).Scan(&roleIDText, &roleName, &roleDisplayName, &externalOnly); err != nil {
		return nil, err
	}
	if externalOnly {
		return nil, validationError("Vai trò đối tác ngoài App không được gán quyền", Record{"role_id": roleIDText, "role": roleDisplayName})
	}
	if roleName == "developer" {
		return nil, validationError("Vai trò Nhà phát triển luôn có toàn quyền và không cho phép sửa bộ quyền", Record{"role_id": roleIDText})
	}
	requestedKeys := map[string]bool{}
	for _, permissionID := range permissionIDs {
		var permissionKey string
		if err := tx.QueryRowContext(ctx, `SELECT key FROM permissions WHERE id=$1`, permissionID).Scan(&permissionKey); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, validationError("Quyền không tồn tại", Record{"permission_id": permissionID})
			}
			return nil, err
		}
		requestedKeys[permissionKey] = true
	}
	if roleName == "admin" {
		if requestedKeys["settings.update"] || requestedKeys["staff.import"] || requestedKeys["sample_data.create"] {
			return nil, validationError("Quyền chỉ dành cho Nhà phát triển", Record{"permissions": []string{"settings.update", "staff.import", "sample_data.create"}})
		}
		for _, required := range []string{"settings.view", "seal.use", "seal.manage", "event_log.view", "staff.delete", "project.import", "project.export", "workflow_definition.import", "workflow_definition.export"} {
			if !requestedKeys[required] {
				return nil, validationError("Không thể gỡ quyền cấu hình bắt buộc của Quản trị viên", Record{"permission": required})
			}
		}
	} else {
		for _, reserved := range []string{"settings.view", "settings.update", "seal.manage", "event_log.view", "staff.import", "staff.delete", "project.import", "workflow_definition.import", "sample_data.create"} {
			if requestedKeys[reserved] {
				return nil, validationError("Quyền này chỉ dành cho Quản trị viên hoặc Nhà phát triển", Record{"permission": reserved})
			}
		}
		if roleName != "pm" && requestedKeys["project.export"] {
			return nil, validationError("Quyền xuất công trình chỉ dành cho Quản trị viên, Nhà phát triển hoặc PM", Record{"permission": "project.export"})
		}
		if roleName != "pm" && requestedKeys["workflow_definition.export"] {
			return nil, validationError("Quyền xuất định nghĩa workflow chỉ dành cho Quản trị viên, Nhà phát triển hoặc PM", Record{"permission": "workflow_definition.export"})
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, roleID); err != nil {
		return nil, err
	}
	assigned := 0
	for _, permissionID := range permissionIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, roleID, permissionID); err != nil {
			return nil, err
		}
		assigned++
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return Record{"id": roleIDText, "display_name": roleDisplayName, "assigned": assigned}, nil
}

func (r *Repository) ReplaceUserRoles(ctx context.Context, userID string, roleIDs []string, projectID string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var userEmail, sampleRoleID string
	var userIsSample bool
	if err := tx.QueryRowContext(ctx, `
		SELECT email,is_sample,COALESCE(sample_role_id::text,'')
		FROM users WHERE id=$1 AND status <> 'deleted' FOR UPDATE
	`, userID).Scan(&userEmail, &userIsSample, &sampleRoleID); err != nil {
		return nil, err
	}
	projectID = cleanID(projectID)
	projectIsSample := false
	if projectID != "" {
		var exists bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND status <> 'cancelled'),
				COALESCE((SELECT is_sample FROM projects WHERE id=$1 AND status <> 'cancelled'),false)
		`, projectID).Scan(&exists, &projectIsSample); err != nil {
			return nil, err
		}
		if !exists {
			return nil, validationError("Công trình không tồn tại hoặc đã huỷ", Record{"project_id": projectID})
		}
	}
	if userIsSample {
		if projectID == "" {
			return nil, validationError("Tài khoản mẫu không được gán vai trò toàn hệ thống", Record{"user_id": userID})
		}
		if !projectIsSample {
			return nil, validationError("Tài khoản mẫu chỉ được gán vào công trình mẫu", Record{"user_id": userID, "project_id": projectID})
		}
		if len(roleIDs) != 1 || cleanID(roleIDs[0]) != sampleRoleID {
			return nil, validationError("Tài khoản mẫu chỉ được dùng đúng vai trò mẫu của tài khoản", Record{"user_id": userID, "sample_role_id": sampleRoleID})
		}
	} else if projectIsSample {
		return nil, validationError("Tài khoản thật không được gán vào công trình mẫu", Record{"user_id": userID, "project_id": projectID})
	}
	if projectID == "" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1 AND project_id IS NULL`, userID); err != nil {
			return nil, err
		}
	} else if _, err := tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1 AND project_id=$2`, userID, projectID); err != nil {
		return nil, err
	}

	assigned := 0
	for _, roleID := range roleIDs {
		var roleDisplayName string
		var externalOnly bool
		if err := tx.QueryRowContext(ctx, `SELECT display_name,external_only FROM roles WHERE id=$1 FOR SHARE`, roleID).Scan(&roleDisplayName, &externalOnly); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, validationError("Vai trò không tồn tại", Record{"role_id": roleID})
			}
			return nil, err
		}
		if externalOnly {
			return nil, validationError("Vai trò đối tác ngoài App không thể gán cho người dùng", Record{"role_id": roleID, "role": roleDisplayName})
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_roles (user_id, role_id, project_id) VALUES ($1,$2,NULLIF($3,'')::uuid) ON CONFLICT DO NOTHING`, userID, roleID, projectID); err != nil {
			return nil, err
		}
		assigned++
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return Record{"user_id": userID, "email": userEmail, "project_id": projectID, "assigned": assigned}, nil
}

type workItemAccessProfile struct {
	UserID                  string
	StaffID                 string
	Department              string
	RoleLabels              []string
	IsAdmin                 bool
	IsDirector              bool
	IsManager               bool
	CanReconcileAllProjects bool
}

func (r *Repository) WorkItemAccessProfile(ctx context.Context, userID string) (workItemAccessProfile, error) {
	profile := workItemAccessProfile{UserID: userID}
	staffRows, err := r.Query(ctx, `
		SELECT COALESCE(s.id::text, '') AS staff_id, COALESCE(s.department, '') AS department,
			EXISTS (
				SELECT 1
				FROM user_roles reconciliation_role
				JOIN role_permissions reconciliation_grant ON reconciliation_grant.role_id=reconciliation_role.role_id
				JOIN permissions reconciliation_permission ON reconciliation_permission.id=reconciliation_grant.permission_id
				WHERE reconciliation_role.user_id=u.id
				  AND reconciliation_role.project_id IS NULL
				  AND reconciliation_permission.key='project.reconciliation.view'
			) AS can_reconcile_all_projects
		FROM users u
		LEFT JOIN staff s ON s.user_id=u.id AND s.status='active'
		WHERE u.id=$1
	`, userID)
	if err != nil {
		return profile, err
	}
	if len(staffRows) == 0 {
		return profile, sql.ErrNoRows
	}
	profile.StaffID = fmt.Sprint(staffRows[0]["staff_id"])
	profile.Department = fmt.Sprint(staffRows[0]["department"])
	profile.CanReconcileAllProjects = boolValue(staffRows[0]["can_reconcile_all_projects"])

	roleRows, err := r.Query(ctx, `
		SELECT DISTINCT r.id::text AS id, r.name, r.display_name, COALESCE(ur.project_id::text, '') AS project_id
		FROM user_roles ur
		JOIN roles r ON r.id=ur.role_id
		WHERE ur.user_id=$1
	`, userID)
	if err != nil {
		return profile, err
	}
	labels := map[string]bool{}
	for _, row := range roleRows {
		if cleanID(row["project_id"]) != "" {
			continue
		}
		name := fmt.Sprint(row["name"])
		displayName := fmt.Sprint(row["display_name"])
		id := fmt.Sprint(row["id"])
		for _, label := range []string{name, displayName, id} {
			if label != "" && label != "<nil>" {
				labels[label] = true
			}
		}
		switch name {
		case "admin":
			profile.IsAdmin = true
			profile.IsManager = true
		case "director":
			profile.IsDirector = true
			profile.IsManager = true
		}
	}
	for label := range labels {
		profile.RoleLabels = append(profile.RoleLabels, label)
	}
	return profile, nil
}

var projectOperationsViewPermissions = []string{"operation.view", "project.view", "pipeline.view"}

var projectReconciliationReadPermissions = map[string]bool{
	"project.reconciliation.view": true,
	"project.view":                true,
	"pipeline.view":               true,
	"planning.view":               true,
	"boq.view":                    true,
	"procurement.view":            true,
	"contract.view":               true,
	"warehouse.view":              true,
	"operation.view":              true,
	"quality.view":                true,
	"acceptance.view":             true,
	"rework.view":                 true,
	"replanning.view":             true,
	"handover.view":               true,
	"accounting_handoff.view":     true,
	"project_finance.view":        true,
	"project_finance.report_view": true,
	"report.view":                 true,
	"statistics.view":             true,
}

var projectReconciliationGlobalActionPermissions = map[string]bool{
	"accounting_handoff.acknowledge": true,
	"project_finance.cost_create":    true,
	"project_finance.cost_review":    true,
	"project_finance.request_create": true,
	"project_finance.process":        true,
}

func reconciliationCompanyScopeAllowed(profile workItemAccessProfile, granted, required []string) bool {
	if !profile.CanReconcileAllProjects {
		return false
	}
	grantedSet := make(map[string]bool, len(granted))
	for _, permission := range granted {
		grantedSet[permission] = true
	}
	for _, permission := range required {
		if !grantedSet[permission] {
			continue
		}
		if projectReconciliationReadPermissions[permission] || projectReconciliationGlobalActionPermissions[permission] {
			return true
		}
	}
	return false
}

func profileCanObserveAllProjectWork(profile workItemAccessProfile) bool {
	return profile.IsManager || profile.CanReconcileAllProjects
}

func (r *Repository) UserHasAnyPermission(ctx context.Context, userID string, required []string) (bool, error) {
	if len(required) == 0 {
		return true, nil
	}
	permissions, err := r.UserPermissions(ctx, userID)
	if err != nil {
		return false, err
	}
	granted := map[string]bool{}
	for _, permission := range permissions {
		granted[permission] = true
	}
	for _, permission := range required {
		if granted[permission] {
			return true, nil
		}
	}
	return false, nil
}

func (r *Repository) UserHasAnyProjectPermission(ctx context.Context, userID, projectID string, required []string) (bool, error) {
	if len(required) == 0 {
		return true, nil
	}
	permissions, err := r.UserProjectPermissions(ctx, userID, projectID)
	if err != nil {
		return false, err
	}
	granted := map[string]bool{}
	for _, permission := range permissions {
		granted[permission] = true
	}
	for _, permission := range required {
		if granted[permission] {
			return true, nil
		}
	}
	return false, nil
}

func (r *Repository) UserProjectPermissions(ctx context.Context, userID, projectID string) ([]string, error) {
	rows, err := r.Query(ctx, `
		SELECT DISTINCT p.key
		FROM user_roles ur
			JOIN role_permissions rp ON rp.role_id=ur.role_id
			JOIN permissions p ON p.id=rp.permission_id
			WHERE ur.user_id=$1
			  AND (ur.project_id IS NULL OR ur.project_id=NULLIF($2,'')::uuid)
		`, userID, projectID)
	if err != nil {
		return nil, err
	}
	permissions := make([]string, 0, len(rows))
	for _, row := range rows {
		permissions = append(permissions, fmt.Sprint(row["key"]))
	}
	return permissions, nil
}

func (r *Repository) CanUsePermission(ctx context.Context, userID string, required []string) (bool, error) {
	return r.UserHasAnyPermission(ctx, userID, required)
}

func (r *Repository) CanAccessProjectPermission(ctx context.Context, userID, projectID string, required []string) (bool, error) {
	projectID = cleanID(projectID)
	if projectID == "" {
		return false, nil
	}
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	activeProject, err := r.Query(ctx, `
		SELECT COUNT(*) AS total
		FROM projects
		WHERE id=$1 AND status <> 'cancelled'
	`, projectID)
	if err != nil {
		return false, err
	}
	if len(activeProject) == 0 || number(activeProject[0]["total"]) == 0 {
		return false, nil
	}
	permissions, err := r.UserProjectPermissions(ctx, userID, projectID)
	if err != nil {
		return false, err
	}
	if !hasAnyString(permissions, required) {
		return false, nil
	}
	if profile.IsAdmin || profile.IsDirector {
		return true, nil
	}
	if reconciliationCompanyScopeAllowed(profile, permissions, required) {
		return true, nil
	}
	return r.UserHasProjectScope(ctx, userID, projectID)
}

func (r *Repository) CanMutateProjectActivity(ctx context.Context, userID, projectID string) (bool, error) {
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return false, err
	}
	if !profile.CanReconcileAllProjects || profile.IsAdmin || profile.IsDirector {
		return true, nil
	}
	return r.UserHasProjectScope(ctx, userID, projectID)
}

func (r *Repository) RecordProjectID(ctx context.Context, table, id string) (string, error) {
	table = safeTable(table)
	if table == "" {
		return "", errors.New("Bảng không hợp lệ")
	}
	row, err := r.QueryOne(ctx, fmt.Sprintf(`SELECT COALESCE(project_id::text, '') AS project_id FROM %s WHERE id=$1`, table), id)
	if err != nil {
		return "", err
	}
	return cleanID(row["project_id"]), nil
}

func (r *Repository) ProjectAccessFilter(ctx context.Context, userID, projectAlias string, start int) (string, []any, error) {
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return "", nil, err
	}
	return projectAccessSQL(projectAlias, start), []any{profile.IsAdmin || profile.IsDirector || profile.CanReconcileAllProjects, profile.UserID, profile.StaffID}, nil
}

func projectAccessSQL(projectAlias string, start int) string {
	bypass := fmt.Sprintf("$%d::boolean", start)
	userID := fmt.Sprintf("NULLIF($%d,'')::uuid", start+1)
	staffID := fmt.Sprintf("NULLIF($%d,'')::uuid", start+2)
	return fmt.Sprintf(`(
		%s
		OR %s.created_by=%s
		OR EXISTS (
			SELECT 1
			FROM project_members pm
			WHERE pm.project_id=%s.id
			  AND pm.staff_id=%s
			  AND pm.active
		)
		OR EXISTS (
			SELECT 1
			FROM user_roles ur
			WHERE ur.user_id=%s
			  AND ur.project_id=%s.id
		)
	)`, bypass, projectAlias, userID, projectAlias, staffID, userID, projectAlias)
}

func (r *Repository) RunNodePermissionGate(ctx context.Context, userID, nodeID string) (bool, []string, error) {
	node, err := r.GetByID(ctx, "pipeline_run_nodes", nodeID)
	if err != nil {
		return false, nil, err
	}
	permissions := nodePermissions(node)
	inScope, err := r.CanViewProjectOperations(ctx, cleanID(node["project_id"]), userID)
	if err != nil {
		return false, permissions, err
	}
	if !inScope {
		return false, permissions, nil
	}
	allowed, err := r.UserHasAnyProjectPermission(ctx, userID, cleanID(node["project_id"]), permissions)
	return allowed, permissions, err
}

func (r *Repository) RunStepPermissionGate(ctx context.Context, userID, stepID string) (bool, []string, error) {
	step, err := r.GetByID(ctx, "pipeline_run_steps", stepID)
	if err != nil {
		return false, nil, err
	}
	return r.RunNodePermissionGate(ctx, userID, fmt.Sprint(step["run_node_id"]))
}

func (r *Repository) ListPartners(ctx context.Context, filter PartnerFilter) ([]Record, error) {
	conditions := []string{"1=1"}
	args := make([]any, 0)
	switch filter.Status {
	case "all":
	case "inactive":
		conditions = append(conditions, "NOT active")
	default:
		conditions = append(conditions, "active")
	}
	if filter.Type != "" {
		args = append(args, filter.Type)
		conditions = append(conditions, fmt.Sprintf("partner_types ? $%d", len(args)))
	}
	switch filter.Group {
	case "input":
		conditions = append(conditions, "partner_types ? 'client'")
	case "output":
		outputTypes := vendorPartnerTypes()
		placeholders := make([]string, 0, len(outputTypes))
		for _, partnerType := range outputTypes {
			args = append(args, partnerType)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		conditions = append(conditions, fmt.Sprintf("partner_types ?| ARRAY[%s]", strings.Join(placeholders, ",")))
	}
	if filter.Query != "" {
		args = append(args, "%"+filter.Query+"%")
		conditions = append(conditions, fmt.Sprintf(`(
			partner_code ILIKE $%d
			OR name ILIKE $%d
			OR COALESCE(legal_name, '') ILIKE $%d
			OR COALESCE(tax_code, '') ILIKE $%d
			OR COALESCE(contact_name, '') ILIKE $%d
		)`, len(args), len(args), len(args), len(args), len(args)))
	}
	return r.Query(ctx, fmt.Sprintf(`
		SELECT *
		FROM partners
		WHERE %s
		ORDER BY active DESC, lower(name), partner_code
	`, strings.Join(conditions, " AND ")), args...)
}

func (r *Repository) CanViewProjectOperations(ctx context.Context, projectID, userID string) (bool, error) {
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return false, err
	}
	activeProject, err := r.Query(ctx, `
		SELECT COUNT(*) AS total
		FROM projects
		WHERE id=$1 AND status <> 'cancelled'
	`, projectID)
	if err != nil {
		return false, err
	}
	if len(activeProject) == 0 || number(activeProject[0]["total"]) == 0 {
		return false, nil
	}
	if profile.IsAdmin || profile.CanReconcileAllProjects {
		return true, nil
	}
	permissions, err := r.UserProjectPermissions(ctx, userID, projectID)
	if err != nil {
		return false, err
	}
	if !hasAnyString(permissions, projectOperationsViewPermissions) {
		return false, nil
	}
	if profile.IsDirector {
		return true, nil
	}
	rows, err := r.Query(ctx, `
		SELECT COUNT(*) AS total
		FROM projects p
		WHERE p.id=$1
		  AND p.status <> 'cancelled'
		  AND (
			p.created_by=$3
			OR
			(NULLIF($2,'')::uuid IS NOT NULL AND EXISTS (
				SELECT 1
				FROM project_members pm
				WHERE pm.project_id=p.id
				  AND pm.staff_id=NULLIF($2,'')::uuid
				  AND pm.active
			))
			OR EXISTS (
				SELECT 1
				FROM user_roles ur
				WHERE ur.user_id=$3
				  AND ur.project_id=p.id
			)
		  )
	`, projectID, profile.StaffID, profile.UserID)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && number(rows[0]["total"]) > 0, nil
}

func (r *Repository) ProjectOperationsVisibleProjectIDs(ctx context.Context, userID string) (map[string]struct{}, error) {
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := r.Query(ctx, `
		SELECT p.id::text AS id
		FROM projects p
		WHERE p.status <> 'cancelled'
		  AND (
			$1::boolean
			OR (
				EXISTS (
					SELECT 1
					FROM user_roles ur
					JOIN role_permissions rp ON rp.role_id=ur.role_id
					JOIN permissions permission ON permission.id=rp.permission_id
					WHERE ur.user_id=$3
					  AND (ur.project_id IS NULL OR ur.project_id=p.id)
					  AND permission.key IN ('operation.view', 'project.view', 'pipeline.view')
				)
				AND (
					$2::boolean
					OR p.created_by=$3
					OR EXISTS (
						SELECT 1
						FROM project_members pm
						WHERE pm.project_id=p.id
						  AND pm.staff_id=NULLIF($4,'')::uuid
						  AND pm.active
					)
					OR EXISTS (
						SELECT 1
						FROM user_roles project_role
						WHERE project_role.user_id=$3
						  AND project_role.project_id=p.id
					)
				)
			)
		  )
	`, profile.IsAdmin || profile.CanReconcileAllProjects, profile.IsDirector, profile.UserID, profile.StaffID)
	if err != nil {
		return nil, err
	}
	projectIDs := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if projectID := cleanID(row["id"]); projectID != "" {
			projectIDs[projectID] = struct{}{}
		}
	}
	return projectIDs, nil
}

func hasAnyString(values []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range required {
		if seen[value] {
			return true
		}
	}
	return false
}

func (r *Repository) CanViewWorkItem(ctx context.Context, workItemID, userID string) (bool, error) {
	return r.canAccessWorkItem(ctx, workItemID, userID, true, true)
}

func (r *Repository) CanActOnWorkItem(ctx context.Context, workItemID, userID string) (bool, error) {
	return r.canAccessWorkItem(ctx, workItemID, userID, false, false)
}

// IsWorkItemResponsible deliberately excludes the global admin override used by
// interactive authorization. Notification fan-out must target the operational
// assignment pool, not every administrator who is allowed to intervene.
func (r *Repository) IsWorkItemResponsible(ctx context.Context, workItemID, userID string) (bool, error) {
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	args := append([]any{workItemID}, workItemScopeArgs(profile, false, false)...)
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT COUNT(*) AS total
		FROM work_items wi
		JOIN projects project ON project.id=wi.project_id AND project.status <> 'cancelled'
		WHERE wi.id=$1 AND wi.status <> 'cancelled' AND (%s)
	`, workItemScopeSQL("wi", 2)), args...)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && number(rows[0]["total"]) > 0, nil
}

func (r *Repository) CanDecideWorkItem(ctx context.Context, item Record, userID string) (bool, error) {
	projectID := cleanID(item["project_id"])
	if projectID == "" {
		return false, nil
	}
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return false, err
	}
	activeProject, err := r.Query(ctx, `SELECT COUNT(*) AS total FROM projects WHERE id=$1 AND status <> 'cancelled'`, projectID)
	if err != nil {
		return false, err
	}
	if len(activeProject) == 0 || number(activeProject[0]["total"]) == 0 {
		return false, nil
	}
	requiredRoles, requestedBy, err := r.workItemDecisionContext(ctx, item)
	if err != nil {
		return false, err
	}
	if requestedBy != "" && requestedBy == userID {
		return false, nil
	}
	if profile.IsAdmin || profile.IsDirector {
		return true, nil
	}
	isProjectManager, err := r.UserIsProjectManager(ctx, userID, projectID)
	if err != nil {
		return false, err
	}
	if isProjectManager {
		return true, nil
	}
	inScope, err := r.UserHasProjectScope(ctx, userID, projectID)
	if err != nil {
		return false, err
	}
	if !inScope {
		return false, nil
	}
	if len(requiredRoles) == 0 {
		return false, nil
	}
	projectLabels, err := r.UserProjectRoleLabels(ctx, userID, projectID)
	if err != nil {
		return false, err
	}
	labels := append([]string{}, profile.RoleLabels...)
	labels = append(labels, projectLabels...)
	return roleLabelsIntersect(labels, requiredRoles), nil
}

func (r *Repository) workItemDecisionContext(ctx context.Context, item Record) ([]string, string, error) {
	requiredRoles := stringSlice(item["role_ids"])
	requestedBy := ""
	if approvalID := cleanID(item["approval_request_id"]); approvalID != "" {
		approval, err := r.GetByID(ctx, "approval_requests", approvalID)
		if err != nil {
			return nil, "", err
		}
		if roles := stringSlice(approval["required_roles"]); len(roles) > 0 {
			requiredRoles = roles
		}
		requestedBy = cleanID(approval["requested_by"])
	} else if fmt.Sprint(item["status"]) == "waiting_review" {
		rows, err := r.Query(ctx, `
			SELECT COALESCE(actor_id::text, '') AS actor_id
			FROM work_item_activities
			WHERE work_item_id=$1 AND activity_type='review_requested'
			ORDER BY created_at DESC
			LIMIT 1
		`, item["id"])
		if err != nil {
			return nil, "", err
		}
		if len(rows) > 0 {
			requestedBy = cleanID(rows[0]["actor_id"])
		}
	}
	return requiredRoles, requestedBy, nil
}

func (r *Repository) CanDecideApprovalRequest(ctx context.Context, request Record, userID string) (bool, error) {
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return false, err
	}
	policy := approvalPolicyFromValue(request["policy_json"])
	requesterCannotApprove := !policy.enabled() || policy.RequesterCannotApprove
	if requestedBy := cleanID(request["requested_by"]); requesterCannotApprove && requestedBy != "" && requestedBy == userID {
		return false, nil
	}
	if approvalRequestID := cleanID(request["id"]); validActivityUUID(approvalRequestID) {
		priorApprovals, err := r.Query(ctx, `
			SELECT COUNT(*) AS total
			FROM approval_actions
			WHERE approval_request_id=$1::uuid AND user_id=$2::uuid AND action='approve'
		`, approvalRequestID, userID)
		if err != nil {
			return false, err
		}
		if len(priorApprovals) > 0 && number(priorApprovals[0]["total"]) > 0 {
			return false, nil
		}
	}
	projectID := cleanID(request["project_id"])
	if projectID != "" {
		activeProject, err := r.Query(ctx, `SELECT COUNT(*) AS total FROM projects WHERE id=$1 AND status <> 'cancelled'`, projectID)
		if err != nil {
			return false, err
		}
		if len(activeProject) == 0 || number(activeProject[0]["total"]) == 0 {
			return false, nil
		}
	}
	labels := append([]string{}, profile.RoleLabels...)
	if profile.IsAdmin || profile.IsDirector {
		return true, nil
	}
	if projectID != "" {
		isProjectManager, err := r.UserIsProjectManager(ctx, userID, projectID)
		if err != nil {
			return false, err
		}
		if isProjectManager {
			return true, nil
		}
		inScope, err := r.UserHasProjectScope(ctx, userID, projectID)
		if err != nil {
			return false, err
		}
		if !inScope {
			return false, nil
		}
		projectLabels, err := r.UserProjectRoleLabels(ctx, userID, projectID)
		if err != nil {
			return false, err
		}
		labels = append(labels, projectLabels...)
	}
	requiredRoles := stringSlice(request["required_roles"])
	if policy.enabled() {
		requiredRoles = policy.rolesForStage(int(number(request["current_stage"])))
	}
	if len(requiredRoles) == 0 {
		return false, nil
	}
	return roleLabelsIntersect(labels, requiredRoles), nil
}

func (r *Repository) ListPendingApprovalRequests(ctx context.Context, actorID, projectID string) ([]Record, error) {
	scopeSQL, args, err := r.ProjectAccessFilter(ctx, actorID, "p", 1)
	if err != nil {
		return nil, err
	}
	projectFilter := ""
	if projectID != "" {
		projectFilter = fmt.Sprintf(" AND p.id=$%d", len(args)+1)
		args = append(args, projectID)
	}
	return r.Query(ctx, fmt.Sprintf(`
		SELECT ar.*,p.name AS project_name,u.full_name AS requested_by_name,
			COALESCE(u.deleted_at IS NOT NULL,false) AS requested_by_account_deleted,
			COALESCE(prn.name, wi.title, ar.reason, 'Yêu cầu phê duyệt') AS target_name
		FROM approval_requests ar
		JOIN projects p ON p.id=ar.project_id AND p.status <> 'cancelled'
		LEFT JOIN users u ON u.id=ar.requested_by
		LEFT JOIN pipeline_run_nodes prn ON ar.target_type='pipeline_run_node' AND prn.id=ar.target_id
		LEFT JOIN work_items wi ON ar.target_type='work_item' AND wi.id=ar.target_id
		WHERE ar.status='pending' AND %s%s
		ORDER BY ar.created_at DESC
	`, scopeSQL, projectFilter), args...)
}

func (r *Repository) UserHasProjectScope(ctx context.Context, userID, projectID string) (bool, error) {
	rows, err := r.Query(ctx, `
		SELECT COUNT(*) AS total
		FROM projects p
		WHERE p.id=$1 AND p.status <> 'cancelled'
		  AND (
			p.created_by=$2
			OR
			EXISTS (
				SELECT 1
				FROM staff s
				JOIN project_members pm ON pm.staff_id=s.id AND pm.active
				WHERE s.user_id=$2 AND pm.project_id=p.id AND s.status='active'
			)
			OR EXISTS (
				SELECT 1
				FROM user_roles ur
				WHERE ur.user_id=$2 AND ur.project_id=p.id
			)
		  )
	`, projectID, userID)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && number(rows[0]["total"]) > 0, nil
}

func (r *Repository) UserIsProjectManager(ctx context.Context, userID, projectID string) (bool, error) {
	rows, err := r.Query(ctx, `
		SELECT COUNT(*) AS total
		FROM projects p
		WHERE p.id=$1 AND p.status <> 'cancelled'
		  AND (
			EXISTS (
				SELECT 1
				FROM staff s
				JOIN project_members pm ON pm.staff_id=s.id AND pm.active
				WHERE s.user_id=$2
				  AND pm.project_id=p.id
				  AND (
					pm.role_ids ? 'pm'
					OR pm.role_ids ? 'PM'
					OR pm.role_ids ? 'Project Manager'
					OR pm.role_ids ? 'admin'
					OR pm.role_ids ? 'director'
					OR pm.role_ids ? 'Giám đốc'
				  )
			)
			OR EXISTS (
				SELECT 1
				FROM user_roles ur
				JOIN roles r ON r.id=ur.role_id
				WHERE ur.user_id=$2
				  AND ur.project_id=p.id
				  AND (r.name IN ('pm','admin','director') OR r.display_name IN ('PM','Project Manager','Admin','Giám đốc'))
			)
		  )
	`, projectID, userID)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && number(rows[0]["total"]) > 0, nil
}

func (r *Repository) UserProjectRoleLabels(ctx context.Context, userID, projectID string) ([]string, error) {
	rows, err := r.Query(ctx, `
		SELECT DISTINCT role_label
		FROM (
			SELECT jsonb_array_elements_text(pm.role_ids) AS role_label
			FROM staff s
			JOIN project_members pm ON pm.staff_id=s.id AND pm.active
			WHERE s.user_id=$1 AND s.status='active' AND pm.project_id=$2
			UNION
			SELECT r.name AS role_label
			FROM user_roles ur
			JOIN roles r ON r.id=ur.role_id
			WHERE ur.user_id=$1 AND ur.project_id=$2
			UNION
			SELECT r.display_name AS role_label
			FROM user_roles ur
			JOIN roles r ON r.id=ur.role_id
			WHERE ur.user_id=$1 AND ur.project_id=$2
			UNION
			SELECT r.id::text AS role_label
			FROM user_roles ur
			JOIN roles r ON r.id=ur.role_id
			WHERE ur.user_id=$1 AND ur.project_id=$2
		) role_labels
		WHERE NULLIF(btrim(role_label), '') IS NOT NULL
	`, userID, projectID)
	if err != nil {
		return nil, err
	}
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, fmt.Sprint(row["role_label"]))
	}
	return labels, nil
}

func roleLabelsIntersect(labels []string, required []string) bool {
	seen := map[string]bool{}
	for _, label := range labels {
		normalized := normalizeRoleLabel(label)
		if normalized != "" {
			seen[normalized] = true
		}
	}
	for _, role := range required {
		if seen[normalizeRoleLabel(role)] {
			return true
		}
	}
	return false
}

func normalizeRoleLabel(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (r *Repository) canAccessWorkItem(ctx context.Context, workItemID, userID string, includeWatchers, includeCancelled bool) (bool, error) {
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return false, err
	}
	managerOverride := profileCanObserveAllProjectWork(profile)
	if !includeWatchers {
		managerOverride = profile.IsAdmin
	}
	args := workItemScopeArgs(profile, managerOverride, includeWatchers)
	args = append([]any{workItemID}, args...)
	statusCondition := "wi.status <> 'cancelled'"
	if includeCancelled {
		statusCondition = "TRUE"
	}
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT COUNT(*) AS total
		FROM work_items wi
		JOIN projects p ON p.id=wi.project_id
		WHERE wi.id=$1 AND %s AND p.status <> 'cancelled'
		  AND (%s)
	`, statusCondition, workItemScopeSQL("wi", 2)), args...)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && number(rows[0]["total"]) > 0, nil
}
