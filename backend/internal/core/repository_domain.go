package core

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT id, email, password_hash, full_name, status, created_at, updated_at
		FROM users
		WHERE email=$1 AND status='active'
	`, email)
}

func (r *Repository) UserRoles(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.Query(ctx, `
		SELECT r.display_name
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
		VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6)
		RETURNING id::text, user_id::text, expires_at
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
	`, sessionID, userID, accessTokenID)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && number(rows[0]["total"]) > 0, nil
}

func (r *Repository) ReplaceRolePermissions(ctx context.Context, roleID string, permissionIDs []string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var roleIDText string
	var roleDisplayName string
	if err := tx.QueryRowContext(ctx, `SELECT id::text, display_name FROM roles WHERE id=$1 FOR UPDATE`, roleID).Scan(&roleIDText, &roleDisplayName); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, roleID); err != nil {
		return nil, err
	}
	assigned := 0
	for _, permissionID := range permissionIDs {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM permissions WHERE id=$1`, permissionID).Scan(&exists); err != nil {
			return nil, err
		}
		if exists == 0 {
			return nil, validationError("Permission không tồn tại", Record{"permission_id": permissionID})
		}
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

	var userEmail string
	if err := tx.QueryRowContext(ctx, `SELECT email FROM users WHERE id=$1 AND status <> 'deleted' FOR UPDATE`, userID).Scan(&userEmail); err != nil {
		return nil, err
	}
	projectID = cleanID(projectID)
	if projectID != "" {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE id=$1 AND status <> 'cancelled'`, projectID).Scan(&exists); err != nil {
			return nil, err
		}
		if exists == 0 {
			return nil, validationError("Công trình không tồn tại hoặc đã huỷ", Record{"project_id": projectID})
		}
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
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM roles WHERE id=$1`, roleID).Scan(&exists); err != nil {
			return nil, err
		}
		if exists == 0 {
			return nil, validationError("Role không tồn tại", Record{"role_id": roleID})
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
	UserID     string
	StaffID    string
	Department string
	RoleLabels []string
	IsAdmin    bool
	IsDirector bool
	IsManager  bool
}

func (r *Repository) WorkItemAccessProfile(ctx context.Context, userID string) (workItemAccessProfile, error) {
	profile := workItemAccessProfile{UserID: userID}
	staffRows, err := r.Query(ctx, `
		SELECT COALESCE(s.id::text, '') AS staff_id, COALESCE(s.department, '') AS department
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
	return r.UserHasProjectScope(ctx, userID, projectID)
}

func (r *Repository) RecordProjectID(ctx context.Context, table, id string) (string, error) {
	table = safeTable(table)
	if table == "" {
		return "", errors.New("invalid table")
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
	return projectAccessSQL(projectAlias, start), []any{profile.IsAdmin || profile.IsDirector, profile.UserID, profile.StaffID}, nil
}

func projectAccessSQL(projectAlias string, start int) string {
	bypass := fmt.Sprintf("$%d::boolean", start)
	userID := fmt.Sprintf("NULLIF($%d,'')::uuid", start+1)
	staffID := fmt.Sprintf("NULLIF($%d,'')::uuid", start+2)
	return fmt.Sprintf(`(
		%s
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
	)`, bypass, projectAlias, staffID, userID, projectAlias)
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

func (r *Repository) AddAudit(ctx context.Context, projectID, actorID, actorName, eventType, targetType, targetID, description string, metadata any) error {
	raw, _ := json.Marshal(metadata)
	var p any = nil
	var t any = nil
	if projectID != "" {
		p = projectID
	}
	if targetID != "" {
		t = targetID
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7, $8::jsonb)
	`, p, actorID, actorName, eventType, targetType, t, description, string(raw))
	return err
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

func addAuditTx(ctx context.Context, tx *sql.Tx, projectID, actorID, actorName, eventType, targetType, targetID, description string, metadata any) error {
	raw, _ := json.Marshal(metadata)
	var p any = nil
	var t any = nil
	if projectID != "" {
		p = projectID
	}
	if targetID != "" {
		t = targetID
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7, $8::jsonb)
	`, p, actorID, actorName, eventType, targetType, t, description, string(raw))
	return err
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
	if profile.IsAdmin {
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
	return r.canAccessWorkItem(ctx, workItemID, userID, true)
}

func (r *Repository) CanActOnWorkItem(ctx context.Context, workItemID, userID string) (bool, error) {
	return r.canAccessWorkItem(ctx, workItemID, userID, false)
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
	if requestedBy := cleanID(request["requested_by"]); requestedBy != "" && requestedBy == userID {
		return false, nil
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
	if len(requiredRoles) == 0 {
		return false, nil
	}
	return roleLabelsIntersect(labels, requiredRoles), nil
}

func (r *Repository) UserHasProjectScope(ctx context.Context, userID, projectID string) (bool, error) {
	rows, err := r.Query(ctx, `
		SELECT COUNT(*) AS total
		FROM projects p
		WHERE p.id=$1 AND p.status <> 'cancelled'
		  AND (
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

func (r *Repository) canAccessWorkItem(ctx context.Context, workItemID, userID string, includeWatchers bool) (bool, error) {
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return false, err
	}
	managerOverride := profile.IsManager
	if !includeWatchers {
		managerOverride = profile.IsAdmin
	}
	args := workItemScopeArgs(profile, managerOverride, includeWatchers)
	args = append([]any{workItemID}, args...)
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT COUNT(*) AS total
		FROM work_items wi
		JOIN projects p ON p.id=wi.project_id
		WHERE wi.id=$1 AND wi.status <> 'cancelled' AND p.status <> 'cancelled'
		  AND (%s)
	`, workItemScopeSQL("wi", 2)), args...)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && number(rows[0]["total"]) > 0, nil
}

func (r *Repository) ListProjectWorkItems(ctx context.Context, projectID, userID string) ([]Record, error) {
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	args := append([]any{projectID}, workItemScopeArgs(profile, profile.IsManager, true)...)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT wi.*, p.name AS project_name, p.code AS project_code, rn.name AS run_node_name, rs.name AS run_step_name,
			s.full_name AS assignee_name,
			COALESCE((SELECT COUNT(*) FROM evidence_records er WHERE er.work_item_id=wi.id AND er.status <> 'rejected'),0) AS evidence_count,
			wfi.id AS workflow_form_instance_id,
			wfi.status AS workflow_form_status,
			wfi.form_type AS workflow_form_type,
			wfi.current_revision AS workflow_form_revision,
			wfi.submitted_at AS workflow_form_submitted_at,
			wfi.form_template_id AS workflow_form_template_id,
			wfi.form_version_id AS workflow_form_version_id,
			wfv.version_no AS workflow_form_version_no,
			wfv.status AS workflow_form_version_status,
			wft.code AS workflow_form_template_code,
			wft.name AS workflow_form_template_name,
			COALESCE(pts.form_required, false) AS workflow_form_required
		FROM work_items wi
		JOIN projects p ON p.id=wi.project_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		LEFT JOIN pipeline_template_steps pts ON pts.id=rs.template_step_id
		LEFT JOIN staff s ON s.id=wi.assignee_staff_id
		LEFT JOIN workflow_form_instances wfi ON wfi.work_item_id=wi.id AND wfi.status <> 'cancelled'
		LEFT JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
		LEFT JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		WHERE wi.project_id=$1 AND wi.status <> 'cancelled' AND p.status <> 'cancelled'
		  AND (%s)
		ORDER BY
			CASE wi.status
				WHEN 'blocked' THEN 1
				WHEN 'in_progress' THEN 2
				WHEN 'ready' THEN 3
				WHEN 'pending' THEN 4
				ELSE 5
			END,
			wi.due_at NULLS LAST,
			wi.created_at
	`, workItemScopeSQL("wi", 2)), args...)
}

func (r *Repository) ListMyWorkItems(ctx context.Context, userID string) ([]Record, error) {
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	args := workItemScopeArgs(profile, false, true)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT wi.*, p.name AS project_name, p.code AS project_code, rn.name AS run_node_name, rs.name AS run_step_name,
			s.full_name AS assignee_name,
			COALESCE((SELECT COUNT(*) FROM evidence_records er WHERE er.work_item_id=wi.id AND er.status <> 'rejected'),0) AS evidence_count,
			wfi.id AS workflow_form_instance_id,
			wfi.status AS workflow_form_status,
			wfi.form_type AS workflow_form_type,
			wfi.current_revision AS workflow_form_revision,
			wfi.submitted_at AS workflow_form_submitted_at,
			wfi.form_template_id AS workflow_form_template_id,
			wfi.form_version_id AS workflow_form_version_id,
			wfv.version_no AS workflow_form_version_no,
			wfv.status AS workflow_form_version_status,
			wft.code AS workflow_form_template_code,
			wft.name AS workflow_form_template_name,
			COALESCE(pts.form_required, false) AS workflow_form_required,
			COALESCE((
				SELECT jsonb_agg(scope)
				FROM (
					VALUES
						('mine', (
							wi.assignee_staff_id=NULLIF($3,'')::uuid
							OR EXISTS (
								SELECT 1
								FROM work_item_assignments wias
								WHERE wias.work_item_id=wi.id
								  AND wias.assignment_type <> 'watcher'
								  AND wias.staff_id=NULLIF($3,'')::uuid
							)
						)),
						('department', (
							NULLIF($4,'') IS NOT NULL
							AND (
								wi.department=$4
								OR EXISTS (
									SELECT 1
									FROM work_item_assignments wias
									WHERE wias.work_item_id=wi.id
									  AND wias.assignment_type <> 'watcher'
									  AND wias.department=$4
								)
							)
						)),
						('pending_approval', wi.status IN ('waiting_review','waiting_approval')),
						('watching', (
							wi.watcher_staff_ids ? $3
							OR EXISTS (
								SELECT 1
								FROM work_item_assignments wias
								WHERE wias.work_item_id=wi.id
								  AND wias.assignment_type='watcher'
								  AND wias.staff_id=NULLIF($3,'')::uuid
							)
						))
				) AS queue(scope, matched)
				WHERE matched
			), '[]'::jsonb) AS queue_scopes
		FROM work_items wi
		JOIN projects p ON p.id=wi.project_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		LEFT JOIN pipeline_template_steps pts ON pts.id=rs.template_step_id
		LEFT JOIN staff s ON s.id=wi.assignee_staff_id
		LEFT JOIN workflow_form_instances wfi ON wfi.work_item_id=wi.id AND wfi.status <> 'cancelled'
		LEFT JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
		LEFT JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		WHERE wi.status <> 'cancelled' AND p.status <> 'cancelled'
		  AND (%s)
		ORDER BY
			CASE wi.status
				WHEN 'blocked' THEN 1
				WHEN 'in_progress' THEN 2
				WHEN 'ready' THEN 3
				WHEN 'pending' THEN 4
				ELSE 5
			END,
			wi.due_at NULLS LAST,
			wi.created_at
		LIMIT 200
	`, workItemScopeSQL("wi", 1)), args...)
}

func activityAttachmentSelectSQL() string {
	return "COALESCE(attachment_state.attachments, '[]'::jsonb) AS attachments"
}

func activityAttachmentJoinSQL() string {
	uuidRegex := "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"
	return fmt.Sprintf(`
		LEFT JOIN LATERAL (
			SELECT jsonb_agg(jsonb_build_object(
				'type', parsed.attachment_type,
				'id', parsed.attachment_id,
				'label', COALESCE(NULLIF(parsed.attachment_label, ''), er.name, er.title, d.name, parsed.attachment_id),
				'status', COALESCE(er.status, d.status),
				'visibility', er.visibility,
				'file_url', COALESCE(er.file_url, d.file_url),
				'evidence_type', er.evidence_type,
				'document_type', COALESCE(er.document_type, d.document_type),
				'document_id', er.document_id
			) ORDER BY raw_attachment.ord) AS attachments
			FROM (
				SELECT value, ord
				FROM jsonb_array_elements(COALESCE(wia.metadata_json->'attachments', '[]'::jsonb)) WITH ORDINALITY AS attachments(value, ord)
				UNION ALL
				SELECT jsonb_build_object('type', 'evidence', 'id', wia.metadata_json->>'evidence_id'), 10001
				WHERE jsonb_array_length(COALESCE(wia.metadata_json->'attachments', '[]'::jsonb)) = 0
				  AND COALESCE(wia.metadata_json->>'evidence_id', '') <> ''
				UNION ALL
				SELECT jsonb_build_object('type', 'document', 'id', wia.metadata_json->>'document_id'), 10002
				WHERE jsonb_array_length(COALESCE(wia.metadata_json->'attachments', '[]'::jsonb)) = 0
				  AND COALESCE(wia.metadata_json->>'document_id', '') <> ''
			) raw_attachment
			CROSS JOIN LATERAL (
				SELECT
					raw_attachment.value->>'type' AS attachment_type,
					raw_attachment.value->>'id' AS attachment_id,
					COALESCE(raw_attachment.value->>'label', '') AS attachment_label
			) parsed
			LEFT JOIN evidence_records er
				ON parsed.attachment_type='evidence'
			   AND er.id=(CASE WHEN parsed.attachment_id ~* '%s' THEN parsed.attachment_id::uuid ELSE NULL END)
			LEFT JOIN documents d
				ON parsed.attachment_type='document'
			   AND d.id=(CASE WHEN parsed.attachment_id ~* '%s' THEN parsed.attachment_id::uuid ELSE NULL END)
			WHERE parsed.attachment_type IN ('evidence', 'document')
			  AND COALESCE(parsed.attachment_id, '') <> ''
		) attachment_state ON true
	`, uuidRegex, uuidRegex)
}

func (r *Repository) ListMyActivityAttention(ctx context.Context, userID string, profile workItemAccessProfile, filter ActivityAttentionFilter) ([]Record, error) {
	projectAccess := projectAccessSQL("p", 2)
	workItemAccess := workItemScopeSQL("wi", 5)
	args := []any{userID, profile.IsAdmin || profile.IsDirector, profile.UserID, profile.StaffID}
	args = append(args, workItemScopeArgs(profile, profile.IsManager, true)...)

	mentioned := "COALESCE(mention_state.mentioned_me, false)"
	unread := "(ars.user_id IS NOT NULL AND ars.read_at IS NULL)"
	openQuestion := "(wia.activity_type='question' AND COALESCE(wia.metadata_json->>'answered_at', wia.metadata_json->>'resolved_at', wia.metadata_json->>'closed_at', '')='')"
	openBlocker := "(wia.activity_type='blocker' AND COALESCE(wia.metadata_json->>'resolved_at', wia.metadata_json->>'closed_at', '')='')"
	dueReminder := "(wia.activity_type='reminder' AND COALESCE(wia.metadata_json->>'completed_at', wia.metadata_json->>'done_at', wia.metadata_json->>'closed_at', '')='' AND COALESCE(wia.metadata_json->>'due_at', '') ~ '^\\d{4}-\\d{2}-\\d{2}' AND left(wia.metadata_json->>'due_at', 10)::date <= current_date + INTERVAL '7 days')"

	conditions := []string{
		"p.status <> 'cancelled'",
		"(wi.id IS NULL OR wi.status <> 'cancelled')",
		fmt.Sprintf("(%s)", projectAccess),
		`EXISTS (
			SELECT 1
			FROM user_roles ur
			JOIN role_permissions rp ON rp.role_id=ur.role_id
			JOIN permissions perm ON perm.id=rp.permission_id
			WHERE ur.user_id=$1
			  AND (ur.project_id IS NULL OR ur.project_id=p.id)
			  AND perm.key IN ('operation.view', 'project.view', 'pipeline.view')
		)`,
		fmt.Sprintf("(wia.scope_type <> 'work_item' OR (wi.id IS NOT NULL AND (%s)))", workItemAccess),
	}
	if profileIsClient(profile) && !profile.IsManager {
		conditions = append(conditions, "wia.visibility='client'")
	}
	if filter.ProjectID != "" {
		args = append(args, filter.ProjectID)
		conditions = append(conditions, fmt.Sprintf("wia.project_id=$%d::uuid", len(args)))
	}
	switch filter.Queue {
	case "mentions":
		conditions = append(conditions, mentioned)
	case "questions":
		conditions = append(conditions, openQuestion)
	case "blockers":
		conditions = append(conditions, openBlocker)
	case "reminders":
		conditions = append(conditions, dueReminder)
	case "unread":
		conditions = append(conditions, unread)
	default:
		conditions = append(conditions, fmt.Sprintf("(%s OR %s OR %s OR %s)", mentioned, openQuestion, openBlocker, dueReminder))
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 80
	}
	args = append(args, filter.Limit)
	limitPlaceholder := len(args)

	return r.Query(ctx, fmt.Sprintf(`
		SELECT
			wia.*,
			wi.title AS work_item_title,
			u.full_name AS actor_name,
			COALESCE(
				CASE
					WHEN wia.scope_type='project' THEN p.code || ' · ' || p.name
					WHEN wia.scope_type='work_item' THEN wi.title
					WHEN wia.scope_type='boq_item' THEN bi.work_item_name
					WHEN wia.scope_type='procurement_request' THEN pr.request_code || ' · ' || pr.title
					WHEN wia.scope_type='stock_movement' THEN sm.movement_code
					WHEN wia.scope_type='acceptance' THEN ar.title
					WHEN wia.scope_type='rework' THEN rw.title
					WHEN wia.scope_type='handover' THEN 'Bàn giao · ' || hr.status
					WHEN wia.scope_type='payable' THEN pay.payable_code || ' · ' || pay.vendor_name
					WHEN wia.scope_type='receivable' THEN rec.receivable_code || ' · ' || rec.client_name
					WHEN wia.scope_type='payment' THEN pm.payment_code || ' · ' || pm.payment_type
					ELSE wi.title
				END,
				'Project'
			) AS target_label,
			COALESCE(mention_state.mentions, '[]'::jsonb) AS mentions,
			COALESCE(mention_state.mentioned_me, false) AS mentioned_me,
			%s,
			ars.read_at,
			%s AS is_unread,
			COALESCE((
				SELECT jsonb_agg(queue_name)
				FROM (
					VALUES
						('mentions', %s),
						('questions', %s),
						('blockers', %s),
						('reminders', %s),
						('unread', %s)
				) AS queues(queue_name, matched)
				WHERE matched
			), '[]'::jsonb) AS queue_types
		FROM work_item_activities wia
		JOIN projects p ON p.id=wia.project_id
		LEFT JOIN work_items wi ON wi.id=wia.work_item_id
		LEFT JOIN users u ON u.id=wia.actor_id
		%s
		LEFT JOIN activity_read_states ars ON ars.activity_id=wia.id AND ars.user_id=$1
		LEFT JOIN LATERAL (
			SELECT
				jsonb_agg(jsonb_build_object(
					'type', am.mention_type,
					'key', am.mention_key,
					'user_id', am.recipient_user_id,
					'staff_id', am.recipient_staff_id
				) ORDER BY am.created_at) AS mentions,
				bool_or(am.recipient_user_id=$1) AS mentioned_me
			FROM activity_mentions am
			WHERE am.activity_id=wia.id
		) mention_state ON true
		LEFT JOIN boq_items bi ON bi.id=wia.scope_id AND wia.scope_type='boq_item'
		LEFT JOIN procurement_requests pr ON pr.id=wia.scope_id AND wia.scope_type='procurement_request'
		LEFT JOIN stock_movements sm ON sm.id=wia.scope_id AND wia.scope_type='stock_movement'
		LEFT JOIN acceptance_records ar ON ar.id=wia.scope_id AND wia.scope_type='acceptance'
		LEFT JOIN rework_requests rw ON rw.id=wia.scope_id AND wia.scope_type='rework'
		LEFT JOIN handover_records hr ON hr.id=wia.scope_id AND wia.scope_type='handover'
		LEFT JOIN payables pay ON pay.id=wia.scope_id AND wia.scope_type='payable'
		LEFT JOIN receivables rec ON rec.id=wia.scope_id AND wia.scope_type='receivable'
		LEFT JOIN payments pm ON pm.id=wia.scope_id AND wia.scope_type='payment'
		WHERE %s
		ORDER BY
			%s DESC,
			CASE
				WHEN %s THEN 1
				WHEN %s THEN 2
				WHEN %s THEN 3
				WHEN %s THEN 4
				ELSE 5
			END,
			wia.created_at DESC
		LIMIT $%d
	`, activityAttachmentSelectSQL(), unread, mentioned, openQuestion, openBlocker, dueReminder, unread, activityAttachmentJoinSQL(), strings.Join(conditions, " AND "), unread, mentioned, openQuestion, openBlocker, dueReminder, limitPlaceholder), args...)
}

func (r *Repository) GetWorkItemDetail(ctx context.Context, workItemID string, clientVisibleOnly ...bool) (Record, error) {
	item, err := r.QueryOne(ctx, `
		SELECT wi.*, p.name AS project_name, p.code AS project_code, rn.name AS run_node_name, rs.name AS run_step_name,
			s.full_name AS assignee_name,
			wfi.id AS workflow_form_instance_id,
			wfi.status AS workflow_form_status,
			wfi.form_type AS workflow_form_type,
			wfi.current_revision AS workflow_form_revision,
			wfi.submitted_at AS workflow_form_submitted_at,
			wfi.form_template_id AS workflow_form_template_id,
			wfi.form_version_id AS workflow_form_version_id,
			wfv.version_no AS workflow_form_version_no,
			wfv.status AS workflow_form_version_status,
			wft.code AS workflow_form_template_code,
			wft.name AS workflow_form_template_name,
			COALESCE(pts.form_required, false) AS workflow_form_required
		FROM work_items wi
		JOIN projects p ON p.id=wi.project_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		LEFT JOIN pipeline_template_steps pts ON pts.id=rs.template_step_id
		LEFT JOIN staff s ON s.id=wi.assignee_staff_id
		LEFT JOIN workflow_form_instances wfi ON wfi.work_item_id=wi.id AND wfi.status <> 'cancelled'
		LEFT JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
		LEFT JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		WHERE wi.id=$1 AND wi.status <> 'cancelled' AND p.status <> 'cancelled'
	`, workItemID)
	if err != nil {
		return nil, err
	}
	visibilitySQL := "TRUE"
	if len(clientVisibleOnly) > 0 && clientVisibleOnly[0] {
		visibilitySQL = "wia.visibility='client'"
	}
	activities, _ := r.Query(ctx, fmt.Sprintf(`
		SELECT wia.*, u.full_name AS actor_name, %s
		FROM work_item_activities wia
		LEFT JOIN users u ON u.id=wia.actor_id
		%s
		WHERE wia.work_item_id=$1
		  AND (%s)
		ORDER BY wia.created_at DESC
		LIMIT 100
	`, activityAttachmentSelectSQL(), activityAttachmentJoinSQL(), visibilitySQL), workItemID)
	evidence, _ := r.Query(ctx, `
		SELECT er.*, u.full_name AS submitted_by_name, d.name AS document_name
		FROM evidence_records er
		LEFT JOIN users u ON u.id=er.submitted_by
		LEFT JOIN documents d ON d.id=er.document_id
		WHERE er.work_item_id=$1
		ORDER BY er.created_at DESC
	`, workItemID)
	assignments, _ := r.Query(ctx, `
		SELECT wia.*, s.full_name AS staff_name, r.display_name AS role_display_name
		FROM work_item_assignments wia
		LEFT JOIN staff s ON s.id=wia.staff_id
		LEFT JOIN roles r ON r.id=wia.role_id
		WHERE wia.work_item_id=$1
		ORDER BY wia.created_at
	`, workItemID)
	item["activities"] = activities
	item["evidence"] = evidence
	item["assignments"] = assignments
	return item, nil
}

func (r *Repository) GetWorkItemByRunStep(ctx context.Context, stepID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT wi.*, p.name AS project_name, p.code AS project_code, rn.name AS run_node_name, rs.name AS run_step_name,
			s.full_name AS assignee_name,
			wfi.id AS workflow_form_instance_id,
			wfi.status AS workflow_form_status,
			wfi.form_type AS workflow_form_type,
			wfi.current_revision AS workflow_form_revision,
			wfi.submitted_at AS workflow_form_submitted_at,
			wfi.form_template_id AS workflow_form_template_id,
			wfi.form_version_id AS workflow_form_version_id,
			wfv.version_no AS workflow_form_version_no,
			wfv.status AS workflow_form_version_status,
			wft.code AS workflow_form_template_code,
			wft.name AS workflow_form_template_name,
			COALESCE(pts.form_required, false) AS workflow_form_required
		FROM work_items wi
		JOIN projects p ON p.id=wi.project_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		LEFT JOIN pipeline_template_steps pts ON pts.id=rs.template_step_id
		LEFT JOIN staff s ON s.id=wi.assignee_staff_id
		LEFT JOIN workflow_form_instances wfi ON wfi.work_item_id=wi.id AND wfi.status <> 'cancelled'
		LEFT JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
		LEFT JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		WHERE wi.run_step_id=$1 AND wi.status <> 'cancelled' AND p.status <> 'cancelled'
	`, stepID)
}

func (r *Repository) WorkItemEvidenceCounts(ctx context.Context, workItemID string) (map[string]int, error) {
	rows, err := r.Query(ctx, `
		SELECT evidence_type, COUNT(*) AS total
		FROM evidence_records
		WHERE work_item_id=$1 AND status IN ('submitted','accepted')
		GROUP BY evidence_type
	`, workItemID)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, row := range rows {
		counts[fmt.Sprint(row["evidence_type"])] = int(number(row["total"]))
	}
	return counts, nil
}

func (r *Repository) GetEvidenceDetail(ctx context.Context, evidenceID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT er.*, wi.title AS work_item_title, wi.status AS work_item_status, u.full_name AS submitted_by_name, reviewer.full_name AS reviewed_by_name, d.name AS document_name
		FROM evidence_records er
		JOIN work_items wi ON wi.id=er.work_item_id AND wi.project_id=er.project_id
		LEFT JOIN users u ON u.id=er.submitted_by
		LEFT JOIN users reviewer ON reviewer.id=er.reviewed_by
		LEFT JOIN documents d ON d.id=er.document_id
		WHERE er.id=$1 AND wi.status <> 'cancelled'
	`, evidenceID)
}

func (r *Repository) GetWorkflowFormForWorkItem(ctx context.Context, workItemID string) (Record, error) {
	instance, err := r.QueryOne(ctx, `
		SELECT wfi.id
		FROM workflow_form_instances wfi
		JOIN work_items wi ON wi.id=wfi.work_item_id AND wi.project_id=wfi.project_id
		JOIN projects p ON p.id=wfi.project_id
		WHERE wfi.work_item_id=$1
		  AND wfi.status <> 'cancelled'
		  AND wi.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY wfi.created_at DESC
		LIMIT 1
	`, workItemID)
	if err != nil {
		return nil, err
	}
	return r.GetWorkflowFormInstanceDetail(ctx, fmt.Sprint(instance["id"]))
}

func (r *Repository) PlanningDailyReports(ctx context.Context, filter PlanningDailyReportFilter, actorID string) ([]Record, error) {
	profile, err := r.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	args := []any{}
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	conditions := []string{"true"}
	if !profile.IsAdmin && !profile.IsDirector {
		actorParam := addArg(actorID)
		staffParam := addArg(profile.StaffID)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
				SELECT 1
				FROM user_roles ur
				JOIN role_permissions rp ON rp.role_id=ur.role_id
				JOIN permissions perm ON perm.id=rp.permission_id
				WHERE ur.user_id=%s
				  AND (ur.project_id IS NULL OR ur.project_id=report_rows.project_id_uuid)
				  AND perm.key IN ('planning.view', 'pipeline.view')
			)`, actorParam))
		conditions = append(conditions, fmt.Sprintf(`(
				(NULLIF(%s,'')::uuid IS NOT NULL AND EXISTS (
					SELECT 1
					FROM project_members pm
					WHERE pm.project_id=report_rows.project_id_uuid
					  AND pm.staff_id=NULLIF(%s,'')::uuid
					  AND pm.active
				))
				OR EXISTS (
					SELECT 1
					FROM user_roles ur_scope
					WHERE ur_scope.user_id=%s
					  AND ur_scope.project_id=report_rows.project_id_uuid
				)
			)`, staffParam, staffParam, actorParam))
	}
	if filter.ProjectID != "" {
		conditions = append(conditions, "report_rows.project_id_uuid="+addArg(filter.ProjectID)+"::uuid")
	}
	if filter.DateFrom != "" {
		conditions = append(conditions, "report_rows.report_date_value >= "+addArg(filter.DateFrom)+"::date")
	}
	if filter.DateTo != "" {
		conditions = append(conditions, "report_rows.report_date_value <= "+addArg(filter.DateTo)+"::date")
	}
	if filter.ReportedBy != "" {
		pattern := "%" + strings.ToLower(filter.ReportedBy) + "%"
		conditions = append(conditions, `lower(concat_ws(' ', report_rows.reported_by, report_rows.submitted_by_name, report_rows.submitted_by::text)) LIKE `+addArg(pattern))
	}
	if filter.Owner != "" {
		pattern := "%" + strings.ToLower(filter.Owner) + "%"
		param := addArg(pattern)
		conditions = append(conditions, fmt.Sprintf(`(
				EXISTS (
					SELECT 1
					FROM jsonb_array_elements(report_rows.completed_items) AS completed_item(value)
					WHERE lower(concat_ws(' ', completed_item.value->>'owner_role', completed_item.value->>'owner_name', completed_item.value->>'note')) LIKE %s
				)
				OR EXISTS (
					SELECT 1
					FROM jsonb_array_elements(report_rows.next_plan_items) AS next_item(value)
					WHERE lower(concat_ws(' ', next_item.value->>'owner_role', next_item.value->>'owner_name', next_item.value->>'description')) LIKE %s
				)
				OR EXISTS (
					SELECT 1
					FROM jsonb_array_elements(report_rows.blockers) AS blocker_item(value)
					WHERE lower(concat_ws(' ', blocker_item.value->>'owner_role', blocker_item.value->>'owner_name', blocker_item.value->>'content')) LIKE %s
				)
			)`, param, param, param))
	}
	if filter.Status != "" {
		conditions = append(conditions, "report_rows.status="+addArg(filter.Status))
	}
	if filter.Search != "" {
		pattern := "%" + strings.ToLower(filter.Search) + "%"
		conditions = append(conditions, `lower(concat_ws(' ',
				report_rows.project_name,
				report_rows.work_item_title,
				report_rows.run_node_name,
				report_rows.run_step_name,
				report_rows.title,
				report_rows.report_date,
				report_rows.reported_by,
				report_rows.planning_scope,
				report_rows.status,
				report_rows.answers::text
			)) LIKE `+addArg(pattern))
	}
	query := `
		WITH base AS (
			SELECT
				wfi.id AS form_instance_uuid,
				wfi.id::text AS form_instance_id,
				wfi.project_id AS project_id_uuid,
				wfi.project_id::text AS project_id,
				p.name AS project_name,
				wfi.work_item_id::text AS work_item_id,
				wi.title AS work_item_title,
				wi.status AS work_item_status,
				wfi.run_node_id::text AS run_node_id,
				prn.name AS run_node_name,
				wfi.run_step_id::text AS run_step_id,
				prs.name AS run_step_name,
				wfi.form_type,
				wfi.title,
				wfi.status,
				wfi.current_revision,
				wfi.submitted_by,
				u.full_name AS submitted_by_name,
				wfi.submitted_at,
				wfi.updated_at,
				wft.code AS template_code,
				wft.name AS template_name,
				wfv.version_no,
				COALESCE(wfi.answers_snapshot_json, '{}'::jsonb) AS answers
			FROM workflow_form_instances wfi
			JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
			JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
			JOIN projects p ON p.id=wfi.project_id
			LEFT JOIN users u ON u.id=wfi.submitted_by
			LEFT JOIN work_items wi ON wi.id=wfi.work_item_id AND wi.project_id=wfi.project_id
			LEFT JOIN pipeline_run_nodes prn ON prn.id=wfi.run_node_id AND prn.project_id=wfi.project_id
			LEFT JOIN pipeline_run_steps prs ON prs.id=wfi.run_step_id AND prs.project_id=wfi.project_id
			WHERE wfi.form_type='daily_report'
			  AND wfi.status <> 'cancelled'
			  AND p.status <> 'cancelled'
			  AND COALESCE(wi.status, '') <> 'cancelled'
			  AND wft.code <> 'builtin_construction_daily_report'
			  AND COALESCE(wfv.validation_json->>'phase', 'planning')='planning'
		),
		report_rows AS (
			SELECT
				base.*,
				COALESCE(base.answers->>'report_date', '') AS report_date,
				CASE
					WHEN COALESCE(base.answers->>'report_date', '') ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'
					THEN (base.answers->>'report_date')::date
					ELSE NULL
				END AS report_date_value,
				COALESCE(base.answers->>'reported_by', '') AS reported_by,
				COALESCE(base.answers->>'planning_scope', '') AS planning_scope,
				CASE
					WHEN COALESCE(base.answers->>'progress_percent', '') ~ '^-?[0-9]+(\.[0-9]+)?$'
					THEN (base.answers->>'progress_percent')::numeric
					ELSE NULL
				END AS progress_percent,
				CASE WHEN jsonb_typeof(base.answers->'completed_items')='array' THEN base.answers->'completed_items' ELSE '[]'::jsonb END AS completed_items,
				CASE WHEN jsonb_typeof(base.answers->'next_plan_items')='array' THEN base.answers->'next_plan_items' ELSE '[]'::jsonb END AS next_plan_items,
				CASE WHEN jsonb_typeof(base.answers->'blockers')='array' THEN base.answers->'blockers' ELSE '[]'::jsonb END AS blockers
			FROM base
		)
		SELECT
			report_rows.form_instance_id AS id,
			report_rows.form_instance_id,
			report_rows.project_id,
			report_rows.project_name,
			report_rows.work_item_id,
			report_rows.work_item_title,
			report_rows.work_item_status,
			report_rows.run_node_id,
			report_rows.run_node_name,
			report_rows.run_step_id,
			report_rows.run_step_name,
			report_rows.form_type,
			report_rows.title,
			report_rows.status,
			report_rows.current_revision,
			report_rows.template_code,
			report_rows.template_name,
			report_rows.version_no,
			report_rows.report_date,
			report_rows.reported_by,
			report_rows.planning_scope,
			report_rows.progress_percent,
			report_rows.completed_items,
			report_rows.next_plan_items,
			report_rows.blockers,
			jsonb_array_length(report_rows.completed_items) AS completed_count,
			jsonb_array_length(report_rows.next_plan_items) AS next_plan_count,
			jsonb_array_length(report_rows.blockers) AS blocker_count,
			(
				SELECT COUNT(*)
				FROM work_items generated
				WHERE generated.source_type='workflow_form_action_item'
				  AND generated.source_id=report_rows.form_instance_uuid
				  AND generated.status <> 'cancelled'
			) AS generated_work_item_count,
			(
				SELECT COUNT(*)
				FROM evidence_records er
				WHERE er.work_item_id=report_rows.work_item_id::uuid
				  AND er.status <> 'rejected'
			) AS evidence_count,
			(
				SELECT COUNT(*)
				FROM workflow_form_signatures wfs
				WHERE wfs.form_instance_id=report_rows.form_instance_uuid
			) AS signature_count,
			(
				SELECT COUNT(*)
				FROM workflow_form_pdf_exports wfp
				WHERE wfp.form_instance_id=report_rows.form_instance_uuid
			) AS pdf_export_count,
			report_rows.submitted_by::text AS submitted_by,
			report_rows.submitted_by_name,
			report_rows.submitted_at,
			report_rows.updated_at,
			report_rows.answers
		FROM report_rows
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY report_rows.report_date_value DESC NULLS LAST, report_rows.updated_at DESC, report_rows.form_instance_id
	`
	return r.Query(ctx, query, args...)
}

func (r *Repository) PlanningMeetingMinutes(ctx context.Context, filter PlanningMeetingMinutesFilter, actorID string) ([]Record, error) {
	profile, err := r.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	args := []any{}
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	conditions := []string{"true"}
	if !profile.IsAdmin && !profile.IsDirector {
		actorParam := addArg(actorID)
		staffParam := addArg(profile.StaffID)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
				SELECT 1
				FROM user_roles ur
				JOIN role_permissions rp ON rp.role_id=ur.role_id
				JOIN permissions perm ON perm.id=rp.permission_id
				WHERE ur.user_id=%s
				  AND (ur.project_id IS NULL OR ur.project_id=meeting_rows.project_id_uuid)
				  AND perm.key IN ('planning.view', 'pipeline.view')
			)`, actorParam))
		conditions = append(conditions, fmt.Sprintf(`(
				(NULLIF(%s,'')::uuid IS NOT NULL AND EXISTS (
					SELECT 1
					FROM project_members pm
					WHERE pm.project_id=meeting_rows.project_id_uuid
					  AND pm.staff_id=NULLIF(%s,'')::uuid
					  AND pm.active
				))
				OR EXISTS (
					SELECT 1
					FROM user_roles ur_scope
					WHERE ur_scope.user_id=%s
					  AND ur_scope.project_id=meeting_rows.project_id_uuid
				)
			)`, staffParam, staffParam, actorParam))
	}
	if filter.ProjectID != "" {
		conditions = append(conditions, "meeting_rows.project_id_uuid="+addArg(filter.ProjectID)+"::uuid")
	}
	if filter.DateFrom != "" {
		conditions = append(conditions, "meeting_rows.meeting_date_value >= "+addArg(filter.DateFrom)+"::date")
	}
	if filter.DateTo != "" {
		conditions = append(conditions, "meeting_rows.meeting_date_value <= "+addArg(filter.DateTo)+"::date")
	}
	if filter.Chair != "" {
		pattern := "%" + strings.ToLower(filter.Chair) + "%"
		conditions = append(conditions, `lower(concat_ws(' ', meeting_rows.chair, meeting_rows.submitted_by_name, meeting_rows.submitted_by::text)) LIKE `+addArg(pattern))
	}
	if filter.Owner != "" {
		pattern := "%" + strings.ToLower(filter.Owner) + "%"
		param := addArg(pattern)
		conditions = append(conditions, fmt.Sprintf(`(
				EXISTS (
					SELECT 1
					FROM jsonb_array_elements(meeting_rows.decisions) AS decision_item(value)
					WHERE lower(concat_ws(' ', decision_item.value->>'content', decision_item.value->>'owner', decision_item.value->>'target')) LIKE %s
				)
				OR EXISTS (
					SELECT 1
					FROM jsonb_array_elements(meeting_rows.action_items) AS action_item(value)
					WHERE lower(concat_ws(' ', action_item.value->>'title', action_item.value->>'owner_role', action_item.value->>'owner_name', action_item.value->>'description')) LIKE %s
				)
				OR lower(concat_ws(' ', meeting_rows.decisions_text, meeting_rows.action_items_text)) LIKE %s
			)`, param, param, param))
	}
	if filter.Status != "" {
		conditions = append(conditions, "meeting_rows.status="+addArg(filter.Status))
	}
	if filter.Search != "" {
		pattern := "%" + strings.ToLower(filter.Search) + "%"
		conditions = append(conditions, `lower(concat_ws(' ',
				meeting_rows.project_name,
				meeting_rows.work_item_title,
				meeting_rows.run_node_name,
				meeting_rows.run_step_name,
				meeting_rows.title,
				meeting_rows.meeting_title,
				meeting_rows.meeting_date,
				meeting_rows.location,
				meeting_rows.chair,
				meeting_rows.agenda,
				meeting_rows.status,
				meeting_rows.answers::text
			)) LIKE `+addArg(pattern))
	}
	query := `
		WITH base AS (
			SELECT
				wfi.id AS form_instance_uuid,
				wfi.id::text AS form_instance_id,
				wfi.project_id AS project_id_uuid,
				wfi.project_id::text AS project_id,
				p.name AS project_name,
				wfi.work_item_id::text AS work_item_id,
				wi.title AS work_item_title,
				wi.status AS work_item_status,
				wfi.run_node_id::text AS run_node_id,
				prn.name AS run_node_name,
				wfi.run_step_id::text AS run_step_id,
				prs.name AS run_step_name,
				wfi.form_type,
				wfi.title,
				wfi.status,
				wfi.current_revision,
				wfi.submitted_by,
				u.full_name AS submitted_by_name,
				wfi.submitted_at,
				wfi.updated_at,
				wft.code AS template_code,
				wft.name AS template_name,
				wfv.version_no,
				COALESCE(wfi.answers_snapshot_json, '{}'::jsonb) AS answers
			FROM workflow_form_instances wfi
			JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
			JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
			JOIN projects p ON p.id=wfi.project_id
			LEFT JOIN users u ON u.id=wfi.submitted_by
			LEFT JOIN work_items wi ON wi.id=wfi.work_item_id AND wi.project_id=wfi.project_id
			LEFT JOIN pipeline_run_nodes prn ON prn.id=wfi.run_node_id AND prn.project_id=wfi.project_id
			LEFT JOIN pipeline_run_steps prs ON prs.id=wfi.run_step_id AND prs.project_id=wfi.project_id
			WHERE wfi.form_type='meeting_minutes'
			  AND wfi.status <> 'cancelled'
			  AND p.status <> 'cancelled'
			  AND COALESCE(wi.status, '') <> 'cancelled'
		),
		meeting_rows AS (
			SELECT
				base.*,
				COALESCE(base.answers->>'meeting_title', base.title, '') AS meeting_title,
				COALESCE(base.answers->>'meeting_date', '') AS meeting_date,
				CASE
					WHEN COALESCE(base.answers->>'meeting_date', '') ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'
					THEN (base.answers->>'meeting_date')::date
					ELSE NULL
				END AS meeting_date_value,
				COALESCE(base.answers->>'location', '') AS location,
				COALESCE(base.answers->>'chair', '') AS chair,
				COALESCE(base.answers->>'agenda', '') AS agenda,
				CASE WHEN jsonb_typeof(base.answers->'attendees')='array' THEN base.answers->'attendees' ELSE '[]'::jsonb END AS attendees,
				CASE WHEN jsonb_typeof(base.answers->'attendees')='string' THEN base.answers->>'attendees' ELSE '' END AS attendees_text,
				CASE WHEN jsonb_typeof(base.answers->'decisions')='array' THEN base.answers->'decisions' ELSE '[]'::jsonb END AS decisions,
				CASE WHEN jsonb_typeof(base.answers->'decisions')='string' THEN base.answers->>'decisions' ELSE '' END AS decisions_text,
				CASE WHEN jsonb_typeof(base.answers->'action_items')='array' THEN base.answers->'action_items' ELSE '[]'::jsonb END AS action_items,
				CASE WHEN jsonb_typeof(base.answers->'action_items')='string' THEN base.answers->>'action_items' ELSE '' END AS action_items_text,
				CASE WHEN jsonb_typeof(base.answers->'confirmed')='boolean' THEN (base.answers->>'confirmed')::boolean ELSE false END AS confirmed
			FROM base
		)
		SELECT
			meeting_rows.form_instance_id AS id,
			meeting_rows.form_instance_id,
			meeting_rows.project_id,
			meeting_rows.project_name,
			meeting_rows.work_item_id,
			meeting_rows.work_item_title,
			meeting_rows.work_item_status,
			meeting_rows.run_node_id,
			meeting_rows.run_node_name,
			meeting_rows.run_step_id,
			meeting_rows.run_step_name,
			meeting_rows.form_type,
			meeting_rows.title,
			meeting_rows.status,
			meeting_rows.current_revision,
			meeting_rows.template_code,
			meeting_rows.template_name,
			meeting_rows.version_no,
			meeting_rows.meeting_title,
			meeting_rows.meeting_date,
			meeting_rows.location,
			meeting_rows.chair,
			meeting_rows.agenda,
			meeting_rows.attendees,
			meeting_rows.attendees_text,
			meeting_rows.decisions,
			meeting_rows.decisions_text,
			meeting_rows.action_items,
			meeting_rows.action_items_text,
			meeting_rows.confirmed,
			CASE
				WHEN jsonb_array_length(meeting_rows.attendees) > 0 THEN jsonb_array_length(meeting_rows.attendees)
				WHEN meeting_rows.attendees_text <> '' THEN 1
				ELSE 0
			END AS attendee_count,
			CASE
				WHEN jsonb_array_length(meeting_rows.decisions) > 0 THEN jsonb_array_length(meeting_rows.decisions)
				WHEN meeting_rows.decisions_text <> '' THEN 1
				ELSE 0
			END AS decision_count,
			CASE
				WHEN jsonb_array_length(meeting_rows.action_items) > 0 THEN jsonb_array_length(meeting_rows.action_items)
				WHEN meeting_rows.action_items_text <> '' THEN 1
				ELSE 0
			END AS action_item_count,
			(
				SELECT COUNT(*)
				FROM work_items generated
				WHERE generated.source_type='workflow_form_action_item'
				  AND generated.source_id=meeting_rows.form_instance_uuid
				  AND generated.status <> 'cancelled'
			) AS generated_work_item_count,
			(
				SELECT COUNT(*)
				FROM evidence_records er
				WHERE er.work_item_id=meeting_rows.work_item_id::uuid
				  AND er.status <> 'rejected'
			) AS evidence_count,
			(
				SELECT COUNT(*)
				FROM workflow_form_signatures wfs
				WHERE wfs.form_instance_id=meeting_rows.form_instance_uuid
			) AS signature_count,
			(
				SELECT COUNT(*)
				FROM workflow_form_pdf_exports wfp
				WHERE wfp.form_instance_id=meeting_rows.form_instance_uuid
			) AS pdf_export_count,
			meeting_rows.submitted_by::text AS submitted_by,
			meeting_rows.submitted_by_name,
			meeting_rows.submitted_at,
			meeting_rows.updated_at,
			meeting_rows.answers
		FROM meeting_rows
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY meeting_rows.meeting_date_value DESC NULLS LAST, meeting_rows.updated_at DESC, meeting_rows.form_instance_id
	`
	return r.Query(ctx, query, args...)
}

func (r *Repository) PlanningChecklists(ctx context.Context, filter PlanningChecklistFilter, actorID string) ([]Record, error) {
	profile, err := r.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	args := []any{}
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	conditions := []string{"true"}
	if !profile.IsAdmin && !profile.IsDirector {
		actorParam := addArg(actorID)
		staffParam := addArg(profile.StaffID)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
				SELECT 1
				FROM user_roles ur
				JOIN role_permissions rp ON rp.role_id=ur.role_id
				JOIN permissions perm ON perm.id=rp.permission_id
				WHERE ur.user_id=%s
				  AND (ur.project_id IS NULL OR ur.project_id=checklist_rows.project_id_uuid)
				  AND perm.key IN ('planning.view', 'pipeline.view')
			)`, actorParam))
		conditions = append(conditions, fmt.Sprintf(`(
				(NULLIF(%s,'')::uuid IS NOT NULL AND EXISTS (
					SELECT 1
					FROM project_members pm
					WHERE pm.project_id=checklist_rows.project_id_uuid
					  AND pm.staff_id=NULLIF(%s,'')::uuid
					  AND pm.active
				))
				OR EXISTS (
					SELECT 1
					FROM user_roles ur_scope
					WHERE ur_scope.user_id=%s
					  AND ur_scope.project_id=checklist_rows.project_id_uuid
				)
			)`, staffParam, staffParam, actorParam))
	}
	if filter.ProjectID != "" {
		conditions = append(conditions, "checklist_rows.project_id_uuid="+addArg(filter.ProjectID)+"::uuid")
	}
	if filter.DateFrom != "" {
		conditions = append(conditions, "checklist_rows.check_date_value >= "+addArg(filter.DateFrom)+"::date")
	}
	if filter.DateTo != "" {
		conditions = append(conditions, "checklist_rows.check_date_value <= "+addArg(filter.DateTo)+"::date")
	}
	if filter.CheckedBy != "" {
		pattern := "%" + strings.ToLower(filter.CheckedBy) + "%"
		conditions = append(conditions, `lower(concat_ws(' ', checklist_rows.checked_by, checklist_rows.submitted_by_name, checklist_rows.submitted_by::text)) LIKE `+addArg(pattern))
	}
	if filter.Assignee != "" {
		pattern := "%" + strings.ToLower(filter.Assignee) + "%"
		param := addArg(pattern)
		conditions = append(conditions, fmt.Sprintf(`(
				EXISTS (
					SELECT 1
					FROM jsonb_array_elements(checklist_rows.checklist_items) AS checklist_item(value)
					WHERE lower(concat_ws(' ', checklist_item.value->>'title', checklist_item.value->>'assignee_role', checklist_item.value->>'assignee_name', checklist_item.value->>'comment')) LIKE %s
				)
				OR lower(concat_ws(' ', checklist_rows.items_done_text, checklist_rows.exceptions_text)) LIKE %s
			)`, param, param))
	}
	if filter.ItemStatus != "" {
		statusParam := addArg(filter.ItemStatus)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
				SELECT 1
				FROM jsonb_array_elements(checklist_rows.checklist_items) AS checklist_item(value)
				WHERE checklist_item.value->>'status' = %s
			)`, statusParam))
	}
	if filter.Evidence == "linked" {
		conditions = append(conditions, "checklist_rows.evidence_linked_count > 0")
	}
	if filter.Evidence == "missing" {
		conditions = append(conditions, "checklist_rows.evidence_missing_count > 0")
	}
	if filter.Status != "" {
		conditions = append(conditions, "checklist_rows.status="+addArg(filter.Status))
	}
	if filter.Search != "" {
		pattern := "%" + strings.ToLower(filter.Search) + "%"
		conditions = append(conditions, `lower(concat_ws(' ',
				checklist_rows.project_name,
				checklist_rows.work_item_title,
				checklist_rows.run_node_name,
				checklist_rows.run_step_name,
				checklist_rows.title,
				checklist_rows.check_date,
				checklist_rows.checked_by,
				checklist_rows.planning_scope,
				checklist_rows.items_done_text,
				checklist_rows.exceptions_text,
				checklist_rows.overall_comment,
				checklist_rows.status,
				checklist_rows.answers::text
			)) LIKE `+addArg(pattern))
	}
	query := `
		WITH base AS (
			SELECT
				wfi.id AS form_instance_uuid,
				wfi.id::text AS form_instance_id,
				wfi.project_id AS project_id_uuid,
				wfi.project_id::text AS project_id,
				p.name AS project_name,
				wfi.work_item_id::text AS work_item_id,
				wi.title AS work_item_title,
				wi.status AS work_item_status,
				wfi.run_node_id::text AS run_node_id,
				prn.name AS run_node_name,
				wfi.run_step_id::text AS run_step_id,
				prs.name AS run_step_name,
				wfi.form_type,
				wfi.title,
				wfi.status,
				wfi.current_revision,
				wfi.submitted_by,
				u.full_name AS submitted_by_name,
				wfi.submitted_at,
				wfi.updated_at,
				wft.code AS template_code,
				wft.name AS template_name,
				wfv.version_no,
				COALESCE(wfi.answers_snapshot_json, '{}'::jsonb) AS answers
			FROM workflow_form_instances wfi
			JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
			JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
			JOIN projects p ON p.id=wfi.project_id
			LEFT JOIN users u ON u.id=wfi.submitted_by
			LEFT JOIN work_items wi ON wi.id=wfi.work_item_id AND wi.project_id=wfi.project_id
			LEFT JOIN pipeline_run_nodes prn ON prn.id=wfi.run_node_id AND prn.project_id=wfi.project_id
			LEFT JOIN pipeline_run_steps prs ON prs.id=wfi.run_step_id AND prs.project_id=wfi.project_id
			WHERE wfi.form_type='checklist'
			  AND wfi.status <> 'cancelled'
			  AND p.status <> 'cancelled'
			  AND COALESCE(wi.status, '') <> 'cancelled'
			  AND wft.code='builtin_checklist'
		),
		normalized AS (
			SELECT
				base.*,
				COALESCE(base.answers->>'check_date', '') AS check_date,
				CASE
					WHEN COALESCE(base.answers->>'check_date', '') ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'
					THEN (base.answers->>'check_date')::date
					ELSE NULL
				END AS check_date_value,
				COALESCE(base.answers->>'checked_by', '') AS checked_by,
				COALESCE(base.answers->>'planning_scope', '') AS planning_scope,
				CASE WHEN jsonb_typeof(base.answers->'checklist_items')='array' THEN base.answers->'checklist_items' ELSE '[]'::jsonb END AS checklist_items,
				COALESCE(base.answers->>'items_done', '') AS items_done_text,
				COALESCE(base.answers->>'exceptions', '') AS exceptions_text,
				COALESCE(base.answers->>'overall_comment', '') AS overall_comment,
				CASE WHEN jsonb_typeof(base.answers->'confirmed')='boolean' THEN (base.answers->>'confirmed')::boolean ELSE false END AS confirmed
			FROM base
		),
		checklist_rows AS (
			SELECT
				normalized.*,
				CASE
					WHEN jsonb_array_length(normalized.checklist_items) > 0 THEN jsonb_array_length(normalized.checklist_items)
					WHEN normalized.items_done_text <> '' THEN 1
					ELSE 0
				END AS item_count,
				CASE
					WHEN jsonb_array_length(normalized.checklist_items) > 0 THEN (
						SELECT COUNT(*)
						FROM jsonb_array_elements(normalized.checklist_items) AS checklist_item(value)
						WHERE checklist_item.value->>'status' IN ('done', 'not_applicable')
					)
					WHEN normalized.items_done_text <> '' AND normalized.exceptions_text = '' THEN 1
					ELSE 0
				END AS done_count,
				CASE
					WHEN jsonb_array_length(normalized.checklist_items) > 0 THEN (
						SELECT COUNT(*)
						FROM jsonb_array_elements(normalized.checklist_items) AS checklist_item(value)
						WHERE COALESCE(checklist_item.value->>'status', '') NOT IN ('done', 'not_applicable')
					)
					WHEN normalized.exceptions_text <> '' THEN 1
					ELSE 0
				END AS needs_action_count,
				CASE
					WHEN jsonb_array_length(normalized.checklist_items) > 0 THEN (
						SELECT COUNT(*)
						FROM jsonb_array_elements(normalized.checklist_items) AS checklist_item(value)
						WHERE COALESCE(checklist_item.value->>'evidence_id', '') <> ''
					)
					ELSE 0
				END AS evidence_linked_count,
				CASE
					WHEN jsonb_array_length(normalized.checklist_items) > 0 THEN (
						SELECT COUNT(*)
						FROM jsonb_array_elements(normalized.checklist_items) AS checklist_item(value)
						WHERE COALESCE(checklist_item.value->>'evidence_id', '') = ''
					)
					WHEN normalized.items_done_text <> '' THEN 1
					ELSE 0
				END AS evidence_missing_count
			FROM normalized
		)
		SELECT
			checklist_rows.form_instance_id AS id,
			checklist_rows.form_instance_id,
			checklist_rows.project_id,
			checklist_rows.project_name,
			checklist_rows.work_item_id,
			checklist_rows.work_item_title,
			checklist_rows.work_item_status,
			checklist_rows.run_node_id,
			checklist_rows.run_node_name,
			checklist_rows.run_step_id,
			checklist_rows.run_step_name,
			checklist_rows.form_type,
			checklist_rows.title,
			checklist_rows.status,
			checklist_rows.current_revision,
			checklist_rows.template_code,
			checklist_rows.template_name,
			checklist_rows.version_no,
			checklist_rows.check_date,
			checklist_rows.checked_by,
			checklist_rows.planning_scope,
			checklist_rows.checklist_items,
			checklist_rows.items_done_text,
			checklist_rows.exceptions_text,
			checklist_rows.overall_comment,
			checklist_rows.confirmed,
			checklist_rows.item_count,
			checklist_rows.done_count,
			checklist_rows.needs_action_count,
			checklist_rows.evidence_linked_count,
			checklist_rows.evidence_missing_count,
			(
				SELECT COUNT(*)
				FROM evidence_records er
				WHERE er.work_item_id=checklist_rows.work_item_id::uuid
				  AND er.status <> 'rejected'
			) AS evidence_count,
			(
				SELECT COUNT(*)
				FROM workflow_form_signatures wfs
				WHERE wfs.form_instance_id=checklist_rows.form_instance_uuid
			) AS signature_count,
			(
				SELECT COUNT(*)
				FROM workflow_form_pdf_exports wfp
				WHERE wfp.form_instance_id=checklist_rows.form_instance_uuid
			) AS pdf_export_count,
			checklist_rows.submitted_by::text AS submitted_by,
			checklist_rows.submitted_by_name,
			checklist_rows.submitted_at,
			checklist_rows.updated_at,
			checklist_rows.answers
		FROM checklist_rows
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY checklist_rows.check_date_value DESC NULLS LAST, checklist_rows.updated_at DESC, checklist_rows.form_instance_id
	`
	return r.Query(ctx, query, args...)
}

func (r *Repository) GetWorkflowFormInstanceDetail(ctx context.Context, instanceID string) (Record, error) {
	instance, err := r.QueryOne(ctx, `
		SELECT
			wfi.*,
			wft.code AS form_code,
			wft.name AS form_name,
			wft.description AS form_description,
			wfv.version_no AS form_version_no,
			wfv.status AS form_version_status,
			wi.title AS work_item_title,
			wi.status AS work_item_status
		FROM workflow_form_instances wfi
		JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
		JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		LEFT JOIN work_items wi ON wi.id=wfi.work_item_id AND wi.project_id=wfi.project_id
		WHERE wfi.id=$1
	`, instanceID)
	if err != nil {
		return nil, err
	}
	fields, err := r.Query(ctx, `
		SELECT id, form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index, visibility_condition_json
		FROM workflow_form_fields
		WHERE form_version_id=$1
		ORDER BY order_index, created_at, id
	`, instance["form_version_id"])
	if err != nil {
		return nil, err
	}
	answerRows, err := r.Query(ctx, `
		SELECT field_key, value_json, revision_no, updated_by, updated_at
		FROM workflow_form_answers
		WHERE instance_id=$1
		ORDER BY field_key
	`, instanceID)
	if err != nil {
		return nil, err
	}
	answers := Record{}
	for _, row := range answerRows {
		answers[fmt.Sprint(row["field_key"])] = row["value_json"]
	}
	events, err := r.Query(ctx, `
		SELECT wfe.*, u.full_name AS actor_name
		FROM workflow_form_events wfe
		LEFT JOIN users u ON u.id=wfe.actor_id
		WHERE wfe.instance_id=$1
		ORDER BY wfe.created_at DESC
		LIMIT 30
	`, instanceID)
	if err != nil {
		return nil, err
	}
	signatures, err := r.Query(ctx, `
		SELECT *
		FROM workflow_form_signatures
		WHERE form_instance_id=$1
		ORDER BY signed_at DESC, created_at DESC
	`, instanceID)
	if err != nil {
		return nil, err
	}
	pdfExports, err := r.Query(ctx, `
		SELECT *
		FROM workflow_form_pdf_exports
		WHERE form_instance_id=$1
		ORDER BY generated_at DESC, created_at DESC
	`, instanceID)
	if err != nil {
		return nil, err
	}
	return Record{
		"instance": instance,
		"template": Record{
			"id":          instance["form_template_id"],
			"code":        instance["form_code"],
			"name":        instance["form_name"],
			"description": instance["form_description"],
			"form_type":   instance["form_type"],
		},
		"version": Record{
			"id":         instance["form_version_id"],
			"version_no": instance["form_version_no"],
			"status":     instance["form_version_status"],
		},
		"fields":      fields,
		"answers":     answers,
		"events":      events,
		"signatures":  signatures,
		"pdf_exports": pdfExports,
	}, nil
}

func (r *Repository) GetSubcontractorReconciliation(ctx context.Context, workItemID string) (Record, error) {
	reconciliation, err := r.QueryOne(ctx, `
		SELECT scr.*, submitted.full_name AS submitted_by_name, updated.full_name AS updated_by_name
		FROM subcontractor_checklist_reconciliations scr
		LEFT JOIN users submitted ON submitted.id=scr.submitted_by
		LEFT JOIN users updated ON updated.id=scr.updated_by
		WHERE scr.work_item_id=$1
	`, workItemID)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT
			scri.*,
			b.status AS boq_status,
			b.category AS boq_category,
			b.work_item_name AS current_boq_work_item_name,
			b.quantity AS current_boq_quantity,
			b.unit AS current_boq_unit,
			er.name AS evidence_name,
			er.status AS evidence_status
		FROM subcontractor_checklist_reconciliation_items scri
		LEFT JOIN boq_items b ON b.id=scri.boq_item_id
		LEFT JOIN evidence_records er ON er.id=scri.evidence_id
		WHERE scri.reconciliation_id=$1
		ORDER BY scri.item_index, scri.created_at
	`, reconciliation["id"])
	if err != nil {
		return nil, err
	}
	reconciliation["items"] = items
	return reconciliation, nil
}

func (r *Repository) SaveSubcontractorReconciliation(ctx context.Context, item Record, form Record, comment string, items []Record, summary Record, submit bool, actorID, actorName string) error {
	workItemID := cleanID(item["id"])
	projectID := cleanID(item["project_id"])
	instance, _ := form["instance"].(Record)
	answers, _ := form["answers"].(Record)
	formInstanceID := cleanID(instance["id"])
	formRevisionNo := int(number(instance["current_revision"]))
	status := "draft"
	activityComment := "Đã lưu nháp đối soát checklist thầu phụ"
	eventType := "subcontractor_reconciliation_saved"
	if submit {
		status = "submitted"
		activityComment = "Đã submit đối soát checklist thầu phụ"
		eventType = "subcontractor_reconciliation_submitted"
	}
	if strings.TrimSpace(comment) != "" {
		activityComment = strings.TrimSpace(comment)
	}
	rawSummary, _ := json.Marshal(summary)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existingRows, err := queryTx(ctx, tx, `
		SELECT *
		FROM subcontractor_checklist_reconciliations
		WHERE work_item_id=$1
		FOR UPDATE
	`, workItemID)
	if err != nil {
		return err
	}
	reconciliationID := ""
	if len(existingRows) > 0 {
		if fmt.Sprint(existingRows[0]["status"]) == "submitted" {
			return validationError("Đối soát checklist thầu phụ đã submit, không thể sửa", Record{"work_item_id": workItemID})
		}
		rows, err := queryTx(ctx, tx, `
			UPDATE subcontractor_checklist_reconciliations
			SET form_instance_id=$2,
				form_revision_no=$3,
				status=$4,
				subcontractor_name=NULLIF($5,''),
				work_package=NULLIF($6,''),
				construction_area=NULLIF($7,''),
				summary_json=$8::jsonb,
				comment=NULLIF($9,''),
				submitted_by=CASE WHEN $10 THEN NULLIF($11,'')::uuid ELSE submitted_by END,
				submitted_at=CASE WHEN $10 THEN now() ELSE submitted_at END,
				updated_by=NULLIF($11,'')::uuid,
				updated_at=now()
			WHERE id=$1
			RETURNING id
		`, existingRows[0]["id"], formInstanceID, formRevisionNo, status, cleanOptionalText(answers["subcontractor_name"]), cleanOptionalText(answers["work_package"]), cleanOptionalText(answers["construction_area"]), string(rawSummary), comment, submit, actorID)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return sql.ErrNoRows
		}
		reconciliationID = cleanID(rows[0]["id"])
	} else {
		rows, err := queryTx(ctx, tx, `
			INSERT INTO subcontractor_checklist_reconciliations (
				project_id, work_item_id, form_instance_id, form_revision_no, status,
				subcontractor_name, work_package, construction_area, summary_json, comment,
				created_by, submitted_by, submitted_at, updated_by
			)
			VALUES (
				$1, $2, $3, $4, $5,
				NULLIF($6,''), NULLIF($7,''), NULLIF($8,''), $9::jsonb, NULLIF($10,''),
				NULLIF($11,'')::uuid,
				CASE WHEN $12 THEN NULLIF($11,'')::uuid ELSE NULL END,
				CASE WHEN $12 THEN now() ELSE NULL END,
				NULLIF($11,'')::uuid
			)
			RETURNING id
		`, projectID, workItemID, formInstanceID, formRevisionNo, status, cleanOptionalText(answers["subcontractor_name"]), cleanOptionalText(answers["work_package"]), cleanOptionalText(answers["construction_area"]), string(rawSummary), comment, actorID, submit)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return sql.ErrNoRows
		}
		reconciliationID = cleanID(rows[0]["id"])
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM subcontractor_checklist_reconciliation_items WHERE reconciliation_id=$1`, reconciliationID); err != nil {
		return err
	}
	for _, row := range items {
		rawPayload, _ := json.Marshal(row["payload_json"])
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO subcontractor_checklist_reconciliation_items (
				reconciliation_id, project_id, work_item_id, form_instance_id, item_index,
				checklist_title, boq_item_id, boq_item_ref, boq_work_item_name, boq_quantity, boq_unit,
				claimed_quantity, confirmed_quantity, variance_quantity, unit, status, evidence_id,
				owner_response, decision, due_date, comment, payload_json
			)
			VALUES (
				$1, $2, $3, $4, $5,
				$6, NULLIF($7,'')::uuid, NULLIF($8,''), NULLIF($9,''), $10, NULLIF($11,''),
				$12, $13, $14, NULLIF($15,''), NULLIF($16,''), NULLIF($17,'')::uuid,
				NULLIF($18,''), $19, NULLIF($20,'')::date, NULLIF($21,''), $22::jsonb
			)
		`, reconciliationID, projectID, workItemID, formInstanceID, row["item_index"],
			row["checklist_title"], cleanID(row["boq_item_id"]), row["boq_item_ref"], row["boq_work_item_name"], row["boq_quantity"], row["boq_unit"],
			row["claimed_quantity"], row["confirmed_quantity"], row["variance_quantity"], row["unit"], row["status"], cleanID(row["evidence_id"]),
			row["owner_response"], row["decision"], row["due_date"], row["comment"], string(rawPayload)); err != nil {
			return err
		}
	}
	rawMetadata, _ := json.Marshal(Record{
		"subcontractor_reconciliation_id": reconciliationID,
		"workflow_form_instance_id":       formInstanceID,
		"status":                          status,
		"summary":                         summary,
	})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'project', $5::jsonb)
	`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE work_items
		SET updated_by=NULLIF($2,'')::uuid,
			updated_at=now()
		WHERE id=$1
	`, workItemID, actorID); err != nil {
		return err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, eventType, "subcontractor_checklist_reconciliation", reconciliationID, activityComment, Record{
		"work_item_id":              workItemID,
		"workflow_form_instance_id": formInstanceID,
		"status":                    status,
		"summary":                   summary,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ListProjectAcceptanceRecords(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			ar.*,
			requested.full_name AS requested_by_name,
			submitted.full_name AS submitted_by_name,
			approved.full_name AS approved_by_name,
			wi.title AS work_item_title,
			wfi.title AS workflow_form_title,
			wfpe.file_name AS pdf_file_name,
			COALESCE(item_counts.item_count, 0) AS checklist_count
		FROM acceptance_records ar
		LEFT JOIN users requested ON requested.id=ar.requested_by
		LEFT JOIN users submitted ON submitted.id=ar.submitted_by
		LEFT JOIN users approved ON approved.id=ar.approved_by
		LEFT JOIN work_items wi ON wi.id=ar.work_item_id
		LEFT JOIN workflow_form_instances wfi ON wfi.id=ar.form_instance_id
		LEFT JOIN workflow_form_pdf_exports wfpe ON wfpe.id=ar.pdf_export_id
		LEFT JOIN (
			SELECT acceptance_id, COUNT(*) AS item_count
			FROM acceptance_checklist_items
			GROUP BY acceptance_id
		) item_counts ON item_counts.acceptance_id=ar.id
		WHERE ar.project_id=$1
		  AND ar.status <> 'deleted'
		  AND ar.status <> 'cancelled'
		ORDER BY ar.updated_at DESC, ar.created_at DESC, ar.id
	`, projectID)
}

func (r *Repository) ListAcceptanceRecordsForWorkItem(ctx context.Context, workItemID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			ar.*,
			requested.full_name AS requested_by_name,
			submitted.full_name AS submitted_by_name,
			approved.full_name AS approved_by_name,
			wi.title AS work_item_title,
			wfi.title AS workflow_form_title,
			wfpe.file_name AS pdf_file_name,
			COALESCE(item_counts.item_count, 0) AS checklist_count
		FROM acceptance_records ar
		LEFT JOIN users requested ON requested.id=ar.requested_by
		LEFT JOIN users submitted ON submitted.id=ar.submitted_by
		LEFT JOIN users approved ON approved.id=ar.approved_by
		LEFT JOIN work_items wi ON wi.id=ar.work_item_id
		LEFT JOIN workflow_form_instances wfi ON wfi.id=ar.form_instance_id
		LEFT JOIN workflow_form_pdf_exports wfpe ON wfpe.id=ar.pdf_export_id
		LEFT JOIN (
			SELECT acceptance_id, COUNT(*) AS item_count
			FROM acceptance_checklist_items
			GROUP BY acceptance_id
		) item_counts ON item_counts.acceptance_id=ar.id
		WHERE ar.work_item_id=$1
		  AND ar.status <> 'deleted'
		  AND ar.status <> 'cancelled'
		ORDER BY ar.updated_at DESC, ar.created_at DESC, ar.id
	`, workItemID)
}

func (r *Repository) GetAcceptanceDetail(ctx context.Context, acceptanceID string) (Record, error) {
	record, err := r.QueryOne(ctx, `
		SELECT
			ar.*,
			requested.full_name AS requested_by_name,
			submitted.full_name AS submitted_by_name,
			approved.full_name AS approved_by_name,
			wi.title AS work_item_title,
			wfi.title AS workflow_form_title,
			wfpe.file_name AS pdf_file_name
		FROM acceptance_records ar
		LEFT JOIN users requested ON requested.id=ar.requested_by
		LEFT JOIN users submitted ON submitted.id=ar.submitted_by
		LEFT JOIN users approved ON approved.id=ar.approved_by
		LEFT JOIN work_items wi ON wi.id=ar.work_item_id
		LEFT JOIN workflow_form_instances wfi ON wfi.id=ar.form_instance_id
		LEFT JOIN workflow_form_pdf_exports wfpe ON wfpe.id=ar.pdf_export_id
		WHERE ar.id=$1
		  AND ar.status <> 'deleted'
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT
			aci.*,
			er.name AS evidence_name,
			er.status AS evidence_status
		FROM acceptance_checklist_items aci
		LEFT JOIN evidence_records er ON er.id=aci.evidence_id
		WHERE aci.acceptance_id=$1
		ORDER BY aci.item_index, aci.created_at, aci.id
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	record["checklist"] = items
	record["checklist_count"] = len(items)
	return record, nil
}

func (r *Repository) SaveAcceptance(ctx context.Context, acceptanceID string, header Record, items []Record, actorID, actorName string) (Record, error) {
	acceptanceID = cleanID(acceptanceID)
	projectID := cleanID(header["project_id"])
	workItemID := cleanID(header["work_item_id"])
	rawSummary, _ := json.Marshal(header["summary_json"])
	activityComment := cleanOptionalText(header["notes"])
	if activityComment == "" {
		activityComment = "Đã lưu nháp nghiệm thu đầu ra"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if acceptanceID != "" {
		existingRows, err := queryTx(ctx, tx, `
			SELECT *
			FROM acceptance_records
			WHERE id=$1
			FOR UPDATE
		`, acceptanceID)
		if err != nil {
			return nil, err
		}
		if len(existingRows) == 0 {
			return nil, sql.ErrNoRows
		}
		if !statusIn(fmt.Sprint(existingRows[0]["status"]), "draft") {
			return nil, validationError("Hồ sơ nghiệm thu đầu ra đã submit/chốt, không thể sửa", Record{"acceptance_id": acceptanceID, "status": existingRows[0]["status"]})
		}
		if cleanID(existingRows[0]["project_id"]) != projectID {
			return nil, validationError("Hồ sơ nghiệm thu đầu ra không thuộc công trình", Record{"acceptance_id": acceptanceID})
		}
		rows, err := queryTx(ctx, tx, `
			UPDATE acceptance_records
			SET work_item_id=NULLIF($2,'')::uuid,
				form_instance_id=NULLIF($3,'')::uuid,
				pdf_export_id=NULLIF($4,'')::uuid,
				type=$5,
				title=$6,
				acceptance_date=NULLIF($7,'')::date,
				location=NULLIF($8,''),
				client_representative=NULLIF($9,''),
				contractor_representative=NULLIF($10,''),
				scope_summary=NULLIF($11,''),
				signature_status=$12,
				document_status=$13,
				notes=NULLIF($14,''),
				summary_json=$15::jsonb,
				updated_at=now()
			WHERE id=$1
			RETURNING id
		`, acceptanceID, workItemID, cleanID(header["form_instance_id"]), cleanID(header["pdf_export_id"]), header["type"],
			header["title"], header["acceptance_date"], header["location"], header["client_representative"], header["contractor_representative"],
			header["scope_summary"], header["signature_status"], header["document_status"], header["notes"], string(rawSummary))
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM acceptance_checklist_items WHERE acceptance_id=$1`, acceptanceID); err != nil {
			return nil, err
		}
	} else {
		code := fmt.Sprintf("OA-%s-%d-%s", projectID[:8], timeNowUnix(), newID()[:8])
		rows, err := queryTx(ctx, tx, `
			INSERT INTO acceptance_records (
				acceptance_code, project_id, work_item_id, form_instance_id, pdf_export_id,
				type, title, requested_by, status, acceptance_date, location,
				client_representative, contractor_representative, scope_summary,
				signature_status, document_status, notes, summary_json
			)
			VALUES (
				$1, $2, NULLIF($3,'')::uuid, NULLIF($4,'')::uuid, NULLIF($5,'')::uuid,
				$6, $7, NULLIF($8,'')::uuid, 'draft', NULLIF($9,'')::date, NULLIF($10,''),
				NULLIF($11,''), NULLIF($12,''), NULLIF($13,''),
				$14, $15, NULLIF($16,''), $17::jsonb
			)
			RETURNING id
		`, code, projectID, workItemID, cleanID(header["form_instance_id"]), cleanID(header["pdf_export_id"]),
			header["type"], header["title"], actorID, header["acceptance_date"], header["location"], header["client_representative"],
			header["contractor_representative"], header["scope_summary"], header["signature_status"], header["document_status"],
			header["notes"], string(rawSummary))
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		acceptanceID = cleanID(rows[0]["id"])
	}

	for _, item := range items {
		rawPayload, _ := json.Marshal(item["payload_json"])
		isPassed := cleanOptionalText(item["decision"]) == "pass"
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO acceptance_checklist_items (
				acceptance_id, item_index, title, description, is_passed, note,
				photo_required, photo_url, evidence_id, decision, severity, required_action, due_date, payload_json
			)
			VALUES (
				$1, $2, $3, NULLIF($4,''), $5, NULLIF($6,''),
				$7, NULLIF($8,''), NULLIF($9,'')::uuid, $10, $11, NULLIF($12,''), NULLIF($13,'')::date, $14::jsonb
			)
		`, acceptanceID, item["item_index"], item["title"], item["description"], isPassed, item["note"],
			item["photo_required"], item["photo_url"], cleanID(item["evidence_id"]), item["decision"], item["severity"],
			item["required_action"], item["due_date"], string(rawPayload)); err != nil {
			return nil, err
		}
	}
	rawMetadata, _ := json.Marshal(Record{
		"acceptance_id": acceptanceID,
		"status":        "draft",
		"type":          header["type"],
		"summary":       header["summary_json"],
	})
	if workItemID != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'project', $5::jsonb)
		`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE work_items
			SET updated_by=NULLIF($2,'')::uuid,
				updated_at=now()
			WHERE id=$1
		`, workItemID, actorID); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "outgoing_acceptance_saved", "acceptance", acceptanceID, activityComment, Record{
		"work_item_id":     workItemID,
		"form_instance_id": cleanID(header["form_instance_id"]),
		"pdf_export_id":    cleanID(header["pdf_export_id"]),
		"type":             header["type"],
		"signature_status": header["signature_status"],
		"document_status":  header["document_status"],
		"summary":          header["summary_json"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetAcceptanceDetail(ctx, acceptanceID)
}

func (r *Repository) SubmitAcceptance(ctx context.Context, acceptanceID, actorID, actorName string) (Record, error) {
	acceptanceID = cleanID(acceptanceID)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		SELECT *
		FROM acceptance_records
		WHERE id=$1
		FOR UPDATE
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	record := rows[0]
	if fmt.Sprint(record["status"]) != "draft" {
		return nil, validationError("Chỉ hồ sơ nghiệm thu nháp mới được submit", Record{"acceptance_id": acceptanceID, "status": record["status"]})
	}
	itemRows, err := queryTx(ctx, tx, `SELECT COUNT(*) AS total FROM acceptance_checklist_items WHERE acceptance_id=$1`, acceptanceID)
	if err != nil {
		return nil, err
	}
	if len(itemRows) == 0 || number(itemRows[0]["total"]) == 0 {
		return nil, validationError("Cần ít nhất một checklist item để submit nghiệm thu đầu ra")
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE acceptance_records
		SET status='submitted',
			submitted_by=NULLIF($2,'')::uuid,
			submitted_at=now(),
			updated_at=now()
		WHERE id=$1
		RETURNING *
	`, acceptanceID, actorID)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, sql.ErrNoRows
	}
	projectID := cleanID(record["project_id"])
	workItemID := cleanID(record["work_item_id"])
	comment := cleanOptionalText(record["notes"])
	if comment == "" {
		comment = "Đã submit nghiệm thu đầu ra"
	}
	rawMetadata, _ := json.Marshal(Record{
		"acceptance_id": acceptanceID,
		"status":        "submitted",
		"type":          record["type"],
		"summary":       record["summary_json"],
	})
	if workItemID != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'project', $5::jsonb)
		`, workItemID, projectID, actorID, comment, string(rawMetadata)); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE work_items
			SET updated_by=NULLIF($2,'')::uuid,
				updated_at=now()
			WHERE id=$1
		`, workItemID, actorID); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "outgoing_acceptance_submitted", "acceptance", acceptanceID, comment, Record{
		"work_item_id":     workItemID,
		"form_instance_id": cleanID(record["form_instance_id"]),
		"pdf_export_id":    cleanID(record["pdf_export_id"]),
		"type":             record["type"],
		"summary":          record["summary_json"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetAcceptanceDetail(ctx, acceptanceID)
}

func (r *Repository) GetOwnerClientReconciliation(ctx context.Context, workItemID string) (Record, error) {
	reconciliation, err := r.QueryOne(ctx, `
		SELECT ocr.*, submitted.full_name AS submitted_by_name, updated.full_name AS updated_by_name
		FROM owner_client_checklist_reconciliations ocr
		LEFT JOIN users submitted ON submitted.id=ocr.submitted_by
		LEFT JOIN users updated ON updated.id=ocr.updated_by
		WHERE ocr.work_item_id=$1
	`, workItemID)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT
			ocri.*,
			ar.title AS current_acceptance_title,
			ar.type AS current_acceptance_type,
			ar.status AS current_acceptance_status,
			ar.result AS current_acceptance_result,
			er.name AS evidence_name,
			er.status AS evidence_status
		FROM owner_client_checklist_reconciliation_items ocri
		LEFT JOIN acceptance_records ar ON ar.id=ocri.acceptance_id
		LEFT JOIN evidence_records er ON er.id=ocri.evidence_id
		WHERE ocri.reconciliation_id=$1
		ORDER BY ocri.item_index, ocri.created_at
	`, reconciliation["id"])
	if err != nil {
		return nil, err
	}
	reconciliation["items"] = items
	return reconciliation, nil
}

func (r *Repository) SaveOwnerClientReconciliation(ctx context.Context, item Record, form Record, comment string, items []Record, summary Record, submit bool, actorID, actorName string) error {
	workItemID := cleanID(item["id"])
	projectID := cleanID(item["project_id"])
	instance, _ := form["instance"].(Record)
	answers, _ := form["answers"].(Record)
	formInstanceID := cleanID(instance["id"])
	formRevisionNo := int(number(instance["current_revision"]))
	status := "draft"
	activityComment := "Đã lưu nháp đối soát checklist chủ đầu tư"
	eventType := "owner_client_reconciliation_saved"
	if submit {
		status = "submitted"
		activityComment = "Đã submit đối soát checklist chủ đầu tư"
		eventType = "owner_client_reconciliation_submitted"
	}
	if strings.TrimSpace(comment) != "" {
		activityComment = strings.TrimSpace(comment)
	}
	rawSummary, _ := json.Marshal(summary)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existingRows, err := queryTx(ctx, tx, `
		SELECT *
		FROM owner_client_checklist_reconciliations
		WHERE work_item_id=$1
		FOR UPDATE
	`, workItemID)
	if err != nil {
		return err
	}
	reconciliationID := ""
	if len(existingRows) > 0 {
		if fmt.Sprint(existingRows[0]["status"]) == "submitted" {
			return validationError("Đối soát checklist chủ đầu tư đã submit, không thể sửa", Record{"work_item_id": workItemID})
		}
		rows, err := queryTx(ctx, tx, `
			UPDATE owner_client_checklist_reconciliations
			SET form_instance_id=$2,
				form_revision_no=$3,
				status=$4,
				client_representative=NULLIF($5,''),
				company_representative=NULLIF($6,''),
				handover_scope=NULLIF($7,''),
				scope_detail=NULLIF($8,''),
				summary_json=$9::jsonb,
				comment=NULLIF($10,''),
				submitted_by=CASE WHEN $11 THEN NULLIF($12,'')::uuid ELSE submitted_by END,
				submitted_at=CASE WHEN $11 THEN now() ELSE submitted_at END,
				updated_by=NULLIF($12,'')::uuid,
				updated_at=now()
			WHERE id=$1
			RETURNING id
		`, existingRows[0]["id"], formInstanceID, formRevisionNo, status, cleanOptionalText(answers["client_representative"]), cleanOptionalText(answers["company_representative"]), cleanOptionalText(answers["handover_scope"]), cleanOptionalText(answers["scope_detail"]), string(rawSummary), comment, submit, actorID)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return sql.ErrNoRows
		}
		reconciliationID = cleanID(rows[0]["id"])
	} else {
		rows, err := queryTx(ctx, tx, `
			INSERT INTO owner_client_checklist_reconciliations (
				project_id, work_item_id, form_instance_id, form_revision_no, status,
				client_representative, company_representative, handover_scope, scope_detail,
				summary_json, comment, created_by, submitted_by, submitted_at, updated_by
			)
			VALUES (
				$1, $2, $3, $4, $5,
				NULLIF($6,''), NULLIF($7,''), NULLIF($8,''), NULLIF($9,''),
				$10::jsonb, NULLIF($11,''), NULLIF($12,'')::uuid,
				CASE WHEN $13 THEN NULLIF($12,'')::uuid ELSE NULL END,
				CASE WHEN $13 THEN now() ELSE NULL END,
				NULLIF($12,'')::uuid
			)
			RETURNING id
		`, projectID, workItemID, formInstanceID, formRevisionNo, status, cleanOptionalText(answers["client_representative"]), cleanOptionalText(answers["company_representative"]), cleanOptionalText(answers["handover_scope"]), cleanOptionalText(answers["scope_detail"]), string(rawSummary), comment, actorID, submit)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return sql.ErrNoRows
		}
		reconciliationID = cleanID(rows[0]["id"])
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM owner_client_checklist_reconciliation_items WHERE reconciliation_id=$1`, reconciliationID); err != nil {
		return err
	}
	for _, row := range items {
		rawPayload, _ := json.Marshal(row["payload_json"])
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO owner_client_checklist_reconciliation_items (
				reconciliation_id, project_id, work_item_id, form_instance_id, item_index,
				checklist_title, output_ref, status, client_decision, severity, required_action, due_date,
				evidence_id, acceptance_id, acceptance_title, acceptance_type, acceptance_status, acceptance_result,
				decision, comment, payload_json
			)
			VALUES (
				$1, $2, $3, $4, $5,
				$6, NULLIF($7,''), NULLIF($8,''), NULLIF($9,''), NULLIF($10,''), NULLIF($11,''), NULLIF($12,'')::date,
				NULLIF($13,'')::uuid, NULLIF($14,'')::uuid, NULLIF($15,''), NULLIF($16,''), NULLIF($17,''), NULLIF($18,''),
				$19, NULLIF($20,''), $21::jsonb
			)
		`, reconciliationID, projectID, workItemID, formInstanceID, row["item_index"],
			row["checklist_title"], row["output_ref"], row["status"], row["client_decision"], row["severity"], row["required_action"], row["due_date"],
			cleanID(row["evidence_id"]), cleanID(row["acceptance_id"]), row["acceptance_title"], row["acceptance_type"], row["acceptance_status"], row["acceptance_result"],
			row["decision"], row["comment"], string(rawPayload)); err != nil {
			return err
		}
	}
	rawMetadata, _ := json.Marshal(Record{
		"owner_client_reconciliation_id": reconciliationID,
		"workflow_form_instance_id":      formInstanceID,
		"status":                         status,
		"summary":                        summary,
	})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'project', $5::jsonb)
	`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE work_items
		SET updated_by=NULLIF($2,'')::uuid,
			updated_at=now()
		WHERE id=$1
	`, workItemID, actorID); err != nil {
		return err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, eventType, "owner_client_checklist_reconciliation", reconciliationID, activityComment, Record{
		"work_item_id":              workItemID,
		"workflow_form_instance_id": formInstanceID,
		"status":                    status,
		"summary":                   summary,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) GetMaterialSnapshot(ctx context.Context, workItemID string) (Record, error) {
	snapshot, err := r.QueryOne(ctx, `
		SELECT mbs.*, submitted.full_name AS submitted_by_name, updated.full_name AS updated_by_name
		FROM material_before_after_snapshots mbs
		LEFT JOIN users submitted ON submitted.id=mbs.submitted_by
		LEFT JOIN users updated ON updated.id=mbs.updated_by
		WHERE mbs.work_item_id=$1
	`, workItemID)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT
			mbsi.*,
			m.code AS current_material_code,
			m.name AS current_material_name,
			m.unit AS current_material_unit,
			m.category AS current_material_category,
			m.active AS current_material_active,
			smb.issued_quantity AS current_issued_quantity,
			smb.used_quantity AS current_used_quantity,
			smb.surplus_quantity AS current_surplus_quantity,
			before_ev.name AS before_evidence_name,
			before_ev.status AS before_evidence_status,
			after_ev.name AS after_evidence_name,
			after_ev.status AS after_evidence_status
		FROM material_before_after_snapshot_items mbsi
		LEFT JOIN materials m ON m.id=mbsi.material_id
		LEFT JOIN site_material_balances smb ON smb.id=mbsi.site_balance_id
		LEFT JOIN evidence_records before_ev ON before_ev.id=mbsi.before_evidence_id
		LEFT JOIN evidence_records after_ev ON after_ev.id=mbsi.after_evidence_id
		WHERE mbsi.snapshot_id=$1
		ORDER BY mbsi.item_index, mbsi.created_at
	`, snapshot["id"])
	if err != nil {
		return nil, err
	}
	snapshot["items"] = items
	return snapshot, nil
}

func (r *Repository) SaveMaterialSnapshot(ctx context.Context, item Record, header Record, items []Record, summary Record, submit bool, actorID, actorName string) error {
	workItemID := cleanID(item["id"])
	projectID := cleanID(item["project_id"])
	snapshotDate := cleanOptionalText(header["snapshot_date"])
	constructionArea := cleanOptionalText(header["construction_area"])
	comment := cleanOptionalText(header["comment"])
	status := "draft"
	activityComment := "Đã lưu nháp snapshot vật tư trước/sau"
	eventType := "material_snapshot_saved"
	if submit {
		status = "submitted"
		activityComment = "Đã submit snapshot vật tư trước/sau"
		eventType = "material_snapshot_submitted"
	}
	if comment != "" {
		activityComment = comment
	}
	rawSummary, _ := json.Marshal(summary)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existingRows, err := queryTx(ctx, tx, `
		SELECT *
		FROM material_before_after_snapshots
		WHERE work_item_id=$1
		FOR UPDATE
	`, workItemID)
	if err != nil {
		return err
	}
	snapshotID := ""
	if len(existingRows) > 0 {
		existing := existingRows[0]
		if fmt.Sprint(existing["status"]) == "submitted" || boolValue(existing["applied_to_site_balance"]) {
			return validationError("Snapshot vật tư đã submit, không thể sửa", Record{"work_item_id": workItemID})
		}
		rows, err := queryTx(ctx, tx, `
			UPDATE material_before_after_snapshots
			SET status=$2,
				snapshot_date=NULLIF($3,'')::date,
				construction_area=NULLIF($4,''),
				summary_json=$5::jsonb,
				comment=NULLIF($6,''),
				submitted_by=CASE WHEN $7 THEN NULLIF($8,'')::uuid ELSE submitted_by END,
				submitted_at=CASE WHEN $7 THEN now() ELSE submitted_at END,
				updated_by=NULLIF($8,'')::uuid,
				updated_at=now()
			WHERE id=$1
			RETURNING id
		`, existing["id"], status, snapshotDate, constructionArea, string(rawSummary), comment, submit, actorID)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return sql.ErrNoRows
		}
		snapshotID = cleanID(rows[0]["id"])
	} else {
		rows, err := queryTx(ctx, tx, `
			INSERT INTO material_before_after_snapshots (
				project_id, work_item_id, status, snapshot_date, construction_area,
				summary_json, comment, created_by, submitted_by, submitted_at, updated_by
			)
			VALUES (
				$1, $2, $3, NULLIF($4,'')::date, NULLIF($5,''),
				$6::jsonb, NULLIF($7,''), NULLIF($8,'')::uuid,
				CASE WHEN $9 THEN NULLIF($8,'')::uuid ELSE NULL END,
				CASE WHEN $9 THEN now() ELSE NULL END,
				NULLIF($8,'')::uuid
			)
			RETURNING id
		`, projectID, workItemID, status, snapshotDate, constructionArea, string(rawSummary), comment, actorID, submit)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return sql.ErrNoRows
		}
		snapshotID = cleanID(rows[0]["id"])
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM material_before_after_snapshot_items WHERE snapshot_id=$1`, snapshotID); err != nil {
		return err
	}
	for _, row := range items {
		rawPayload, _ := json.Marshal(row["payload_json"])
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO material_before_after_snapshot_items (
				snapshot_id, project_id, work_item_id, item_index,
				material_id, material_code, material_name, unit, source,
				before_quantity, added_quantity, after_quantity, waste_quantity, used_quantity,
				affects_site_balance, site_balance_id, before_evidence_id, after_evidence_id,
				construction_area, condition, comment, payload_json
			)
			VALUES (
				$1, $2, $3, $4,
				NULLIF($5,'')::uuid, NULLIF($6,''), $7, $8, $9,
				$10, $11, $12, $13, $14,
				$15, NULLIF($16,'')::uuid, NULLIF($17,'')::uuid, NULLIF($18,'')::uuid,
				NULLIF($19,''), $20, NULLIF($21,''), $22::jsonb
			)
		`, snapshotID, projectID, workItemID, row["item_index"],
			cleanID(row["material_id"]), row["material_code"], row["material_name"], row["unit"], row["source"],
			row["before_quantity"], row["added_quantity"], row["after_quantity"], row["waste_quantity"], row["used_quantity"],
			row["affects_site_balance"], cleanID(row["site_balance_id"]), cleanID(row["before_evidence_id"]), cleanID(row["after_evidence_id"]),
			row["construction_area"], row["condition"], row["comment"], string(rawPayload)); err != nil {
			return err
		}
	}
	if submit {
		if err := r.applyMaterialSnapshotToSiteBalanceTx(ctx, tx, projectID, items); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE material_before_after_snapshots
			SET applied_to_site_balance=true,
				updated_at=now()
			WHERE id=$1
		`, snapshotID); err != nil {
			return err
		}
	}
	rawMetadata, _ := json.Marshal(Record{
		"material_snapshot_id":    snapshotID,
		"status":                  status,
		"summary":                 summary,
		"applied_to_site_balance": submit,
	})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'project', $5::jsonb)
	`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE work_items
		SET updated_by=NULLIF($2,'')::uuid,
			updated_at=now()
		WHERE id=$1
	`, workItemID, actorID); err != nil {
		return err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, eventType, "material_before_after_snapshot", snapshotID, activityComment, Record{
		"work_item_id":            workItemID,
		"status":                  status,
		"summary":                 summary,
		"applied_to_site_balance": submit,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) applyMaterialSnapshotToSiteBalanceTx(ctx context.Context, tx *sql.Tx, projectID string, items []Record) error {
	for _, item := range items {
		if !boolValue(item["affects_site_balance"]) {
			continue
		}
		materialID := cleanID(item["material_id"])
		balanceID := cleanID(item["site_balance_id"])
		if materialID == "" || balanceID == "" {
			return validationError("Dòng snapshot thiếu vật tư hoặc số dư công trình", Record{"item_index": item["item_index"]})
		}
		balanceRows, err := queryTx(ctx, tx, `
			SELECT *
			FROM site_material_balances
			WHERE id=$1 AND project_id=$2 AND material_id=$3
			FOR UPDATE
		`, balanceID, projectID, materialID)
		if err != nil {
			return err
		}
		if len(balanceRows) == 0 {
			return validationError("Không tìm thấy số dư vật tư tại công trình", Record{"item_index": item["item_index"], "material_id": materialID})
		}
		usedQuantity := number(item["used_quantity"])
		if materialSnapshotQuantityExceeds(usedQuantity, materialBalanceAvailable(balanceRows[0])) {
			return validationError("Số lượng sử dụng vượt vật tư còn tại công trình", Record{
				"item_index":  item["item_index"],
				"material_id": materialID,
				"available":   materialBalanceAvailable(balanceRows[0]),
				"quantity":    usedQuantity,
			})
		}
		if usedQuantity <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE site_material_balances
			SET used_quantity = used_quantity + $2,
				surplus_quantity = GREATEST(issued_quantity - (used_quantity + $2), 0),
				updated_at = now()
			WHERE id=$1
		`, balanceID, usedQuantity); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ListSurplusHandlingsForWorkItem(ctx context.Context, workItemID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			ssh.*,
			m.code AS material_code,
			m.name AS material_name,
			m.unit AS material_unit,
			w.name AS warehouse_name,
			er.name AS evidence_name,
			er.status AS evidence_status,
			sm.movement_code AS stock_movement_code,
			sm.movement_type AS stock_movement_type
		FROM site_surplus_handlings ssh
		JOIN materials m ON m.id=ssh.material_id
		LEFT JOIN warehouses w ON w.id=ssh.warehouse_id
		LEFT JOIN evidence_records er ON er.id=ssh.evidence_id
		LEFT JOIN stock_movements sm ON sm.id=ssh.stock_movement_id
		WHERE ssh.work_item_id=$1
		  AND ssh.status <> 'cancelled'
		ORDER BY ssh.created_at DESC, ssh.id
	`, workItemID)
}

func (r *Repository) GetSurplusHandling(ctx context.Context, handlingID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			ssh.*,
			m.code AS material_code,
			m.name AS material_name,
			m.unit AS material_unit,
			w.name AS warehouse_name,
			er.name AS evidence_name,
			er.status AS evidence_status,
			sm.movement_code AS stock_movement_code,
			sm.movement_type AS stock_movement_type
		FROM site_surplus_handlings ssh
		JOIN materials m ON m.id=ssh.material_id
		LEFT JOIN warehouses w ON w.id=ssh.warehouse_id
		LEFT JOIN evidence_records er ON er.id=ssh.evidence_id
		LEFT JOIN stock_movements sm ON sm.id=ssh.stock_movement_id
		WHERE ssh.id=$1
		  AND ssh.status <> 'cancelled'
	`, handlingID)
}

func (r *Repository) SaveSurplusHandling(ctx context.Context, item Record, handlingID string, row Record, actorID, actorName string) (Record, error) {
	workItemID := cleanID(item["id"])
	projectID := cleanID(item["project_id"])
	handlingID = cleanID(handlingID)
	activityComment := cleanOptionalText(row["comment"])
	if activityComment == "" {
		activityComment = "Đã lưu nháp xử lý vật tư dư"
	}
	rawPayload, _ := json.Marshal(Record{
		"material_name":           row["material_name"],
		"material_code":           row["material_code"],
		"surplus_before_handling": row["surplus_before_handling"],
	})

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if handlingID != "" {
		existingRows, err := queryTx(ctx, tx, `
			SELECT *
			FROM site_surplus_handlings
			WHERE id=$1
			FOR UPDATE
		`, handlingID)
		if err != nil {
			return nil, err
		}
		if len(existingRows) == 0 {
			return nil, sql.ErrNoRows
		}
		if fmt.Sprint(existingRows[0]["status"]) == "submitted" {
			return nil, validationError("Hồ sơ xử lý vật tư dư đã submit, không thể sửa", Record{"surplus_handling_id": handlingID})
		}
		if cleanID(existingRows[0]["work_item_id"]) != workItemID {
			return nil, validationError("Hồ sơ xử lý vật tư dư không thuộc work item", Record{"surplus_handling_id": handlingID})
		}
		rows, err := queryTx(ctx, tx, `
			UPDATE site_surplus_handlings
			SET site_balance_id=$2,
				material_id=$3,
				material_snapshot_id=NULLIF($4,'')::uuid,
				material_snapshot_item_id=NULLIF($5,'')::uuid,
				handling_type=$6,
				quantity=$7,
				unit=$8,
				warehouse_id=NULLIF($9,'')::uuid,
				custody_location=NULLIF($10,''),
				condition=$11,
				evidence_id=NULLIF($12,'')::uuid,
				comment=NULLIF($13,''),
				payload_json=$14::jsonb,
				updated_by=NULLIF($15,'')::uuid,
				updated_at=now()
			WHERE id=$1
			RETURNING id
		`, handlingID, row["site_balance_id"], row["material_id"], cleanID(row["material_snapshot_id"]), cleanID(row["material_snapshot_item_id"]),
			row["handling_type"], row["quantity"], row["unit"], cleanID(row["warehouse_id"]), row["custody_location"], row["condition"],
			cleanID(row["evidence_id"]), row["comment"], string(rawPayload), actorID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
	} else {
		code := fmt.Sprintf("SH-%s-%d-%s", workItemID[:8], timeNowUnix(), newID()[:8])
		rows, err := queryTx(ctx, tx, `
			INSERT INTO site_surplus_handlings (
				handling_code, project_id, work_item_id, site_balance_id, material_id,
				material_snapshot_id, material_snapshot_item_id, status, handling_type,
				quantity, unit, warehouse_id, custody_location, condition, evidence_id,
				comment, payload_json, created_by, updated_by
			)
			VALUES (
				$1, $2, $3, $4, $5,
				NULLIF($6,'')::uuid, NULLIF($7,'')::uuid, 'draft', $8,
				$9, $10, NULLIF($11,'')::uuid, NULLIF($12,''), $13, NULLIF($14,'')::uuid,
				NULLIF($15,''), $16::jsonb, NULLIF($17,'')::uuid, NULLIF($17,'')::uuid
			)
			RETURNING id
		`, code, projectID, workItemID, row["site_balance_id"], row["material_id"],
			cleanID(row["material_snapshot_id"]), cleanID(row["material_snapshot_item_id"]), row["handling_type"],
			row["quantity"], row["unit"], cleanID(row["warehouse_id"]), row["custody_location"], row["condition"], cleanID(row["evidence_id"]),
			row["comment"], string(rawPayload), actorID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		handlingID = cleanID(rows[0]["id"])
	}
	rawMetadata, _ := json.Marshal(Record{
		"surplus_handling_id": handlingID,
		"status":              "draft",
		"handling_type":       row["handling_type"],
		"material_id":         row["material_id"],
		"quantity":            row["quantity"],
	})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'project', $5::jsonb)
	`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE work_items
		SET updated_by=NULLIF($2,'')::uuid,
			updated_at=now()
		WHERE id=$1
	`, workItemID, actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "site_surplus_handling_saved", "site_surplus_handling", handlingID, activityComment, Record{
		"work_item_id":  workItemID,
		"handling_type": row["handling_type"],
		"material_id":   row["material_id"],
		"quantity":      row["quantity"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetSurplusHandling(ctx, handlingID)
}

func (r *Repository) SubmitSurplusHandling(ctx context.Context, handlingID string, actorID, actorName string) (Record, error) {
	handlingID = cleanID(handlingID)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		SELECT *
		FROM site_surplus_handlings
		WHERE id=$1
		FOR UPDATE
	`, handlingID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	handling := rows[0]
	if fmt.Sprint(handling["status"]) == "submitted" {
		return nil, validationError("Hồ sơ xử lý vật tư dư đã submit", Record{"surplus_handling_id": handlingID})
	}
	projectID := cleanID(handling["project_id"])
	workItemID := cleanID(handling["work_item_id"])
	materialID := cleanID(handling["material_id"])
	warehouseID := cleanID(handling["warehouse_id"])
	handlingType := cleanOptionalText(handling["handling_type"])
	quantity := number(handling["quantity"])
	unit := cleanOptionalText(handling["unit"])
	if warehouseID == "" || cleanID(handling["evidence_id"]) == "" || cleanOptionalText(handling["custody_location"]) == "" {
		return nil, validationError("Hồ sơ xử lý vật tư dư thiếu kho, evidence hoặc vị trí lưu/trả")
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "stock:"+warehouseID+":"+materialID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "site-material:"+projectID+":"+materialID); err != nil {
		return nil, err
	}
	warehouseRows, err := queryTx(ctx, tx, `SELECT id FROM warehouses WHERE id=$1 AND active`, warehouseID)
	if err != nil {
		return nil, err
	}
	if len(warehouseRows) == 0 {
		return nil, validationError("Kho không tồn tại hoặc đã ngưng hoạt động", Record{"warehouse_id": warehouseID})
	}
	materialRows, err := queryTx(ctx, tx, `SELECT id FROM materials WHERE id=$1 AND active`, materialID)
	if err != nil {
		return nil, err
	}
	if len(materialRows) == 0 {
		return nil, validationError("Vật tư không tồn tại hoặc đã ngưng hoạt động", Record{"material_id": materialID})
	}
	balanceRows, err := queryTx(ctx, tx, `
		SELECT *
		FROM site_material_balances
		WHERE id=$1 AND project_id=$2 AND material_id=$3
		FOR UPDATE
	`, handling["site_balance_id"], projectID, materialID)
	if err != nil {
		return nil, err
	}
	if len(balanceRows) == 0 {
		return nil, validationError("Không tìm thấy số dư vật tư tại công trình", Record{"site_balance_id": handling["site_balance_id"]})
	}
	if quantityExceedsAvailable(quantity, number(balanceRows[0]["surplus_quantity"])) {
		return nil, validationError("Số lượng xử lý vượt vật tư dư", Record{"available": balanceRows[0]["surplus_quantity"], "quantity": quantity})
	}

	code := fmt.Sprintf("SM-%s-%d-%s", handlingType, timeNowUnix(), newID()[:8])
	reason := cleanOptionalText(handling["comment"])
	if reason == "" {
		reason = fmt.Sprintf("Xử lý vật tư dư %s %.3f %s", handlingType, quantity, unit)
	}
	movementRows, err := queryTx(ctx, tx, `
		INSERT INTO stock_movements
			(movement_code, project_id, warehouse_id, material_id, movement_type, quantity, unit, reason, reference_type, reference_id, created_by, approved_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'site_surplus_handling',$9,NULLIF($10,'')::uuid,NULLIF($10,'')::uuid)
		RETURNING *
	`, code, projectID, warehouseID, materialID, handlingType, quantity, unit, reason, handlingID, actorID)
	if err != nil {
		return nil, err
	}
	if len(movementRows) == 0 {
		return nil, sql.ErrNoRows
	}
	movement := movementRows[0]
	updatedBalance, err := queryTx(ctx, tx, `
		UPDATE site_material_balances
		SET issued_quantity = GREATEST(issued_quantity - $3, 0),
			surplus_quantity = GREATEST(GREATEST(issued_quantity - $3, 0) - used_quantity, 0),
			surplus_status = CASE
				WHEN GREATEST(GREATEST(issued_quantity - $3, 0) - used_quantity, 0) > 0.000001 THEN 'partial'
				ELSE 'handled'
			END,
			updated_at = now()
		WHERE id=$1 AND project_id=$2
		RETURNING *
	`, handling["site_balance_id"], projectID, quantity)
	if err != nil {
		return nil, err
	}
	if len(updatedBalance) == 0 {
		return nil, validationError("Không cập nhật được số dư vật tư tại công trình", Record{"site_balance_id": handling["site_balance_id"]})
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE site_surplus_handlings
		SET status='submitted',
			stock_movement_id=$2,
			submitted_by=NULLIF($3,'')::uuid,
			submitted_at=now(),
			updated_by=NULLIF($3,'')::uuid,
			updated_at=now()
		WHERE id=$1
		RETURNING *
	`, handlingID, movement["id"], actorID)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, sql.ErrNoRows
	}
	activityComment := reason
	rawMetadata, _ := json.Marshal(Record{
		"surplus_handling_id": handlingID,
		"stock_movement_id":   movement["id"],
		"stock_movement_code": movement["movement_code"],
		"status":              "submitted",
		"handling_type":       handlingType,
		"material_id":         materialID,
		"quantity":            quantity,
		"surplus_status":      updatedBalance[0]["surplus_status"],
	})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'project', $5::jsonb)
	`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE work_items
		SET updated_by=NULLIF($2,'')::uuid,
			updated_at=now()
		WHERE id=$1
	`, workItemID, actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, stockMovementEvent(handlingType), "stock_movement", cleanID(movement["id"]), stockMovementDescription(StockActionRequest{
		ProjectID:     projectID,
		WarehouseID:   warehouseID,
		MaterialID:    materialID,
		MovementType:  handlingType,
		Quantity:      quantity,
		Unit:          unit,
		Reason:        reason,
		ReferenceType: "site_surplus_handling",
		ReferenceID:   handlingID,
	}), Record{
		"movement_type":       handlingType,
		"quantity":            quantity,
		"material_id":         materialID,
		"warehouse_id":        warehouseID,
		"reference_type":      "site_surplus_handling",
		"reference_id":        handlingID,
		"surplus_handling_id": handlingID,
	}); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "site_surplus_handling_submitted", "site_surplus_handling", handlingID, activityComment, Record{
		"work_item_id":        workItemID,
		"stock_movement_id":   movement["id"],
		"stock_movement_code": movement["movement_code"],
		"handling_type":       handlingType,
		"material_id":         materialID,
		"quantity":            quantity,
		"surplus_status":      updatedBalance[0]["surplus_status"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetSurplusHandling(ctx, handlingID)
}

func (r *Repository) ListIncomingAcceptancesForProject(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			iar.*,
			w.name AS warehouse_name,
			pr.request_code AS procurement_request_code,
			pr.title AS procurement_request_title,
			cb.full_name AS created_by_name,
			sb.full_name AS submitted_by_name,
			COALESCE(item_counts.item_count, 0) AS item_count
		FROM incoming_acceptance_records iar
		LEFT JOIN warehouses w ON w.id=iar.warehouse_id
		LEFT JOIN procurement_requests pr ON pr.id=iar.procurement_request_id
		LEFT JOIN users cb ON cb.id=iar.created_by
		LEFT JOIN users sb ON sb.id=iar.submitted_by
		LEFT JOIN (
			SELECT incoming_acceptance_id, COUNT(*) AS item_count
			FROM incoming_acceptance_items
			GROUP BY incoming_acceptance_id
		) item_counts ON item_counts.incoming_acceptance_id=iar.id
		WHERE iar.project_id=$1
		  AND iar.status <> 'cancelled'
		ORDER BY iar.updated_at DESC, iar.created_at DESC, iar.id
	`, projectID)
}

func (r *Repository) ListIncomingAcceptancesForWorkItem(ctx context.Context, workItemID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			iar.*,
			w.name AS warehouse_name,
			pr.request_code AS procurement_request_code,
			pr.title AS procurement_request_title,
			cb.full_name AS created_by_name,
			sb.full_name AS submitted_by_name,
			COALESCE(item_counts.item_count, 0) AS item_count
		FROM incoming_acceptance_records iar
		LEFT JOIN warehouses w ON w.id=iar.warehouse_id
		LEFT JOIN procurement_requests pr ON pr.id=iar.procurement_request_id
		LEFT JOIN users cb ON cb.id=iar.created_by
		LEFT JOIN users sb ON sb.id=iar.submitted_by
		LEFT JOIN (
			SELECT incoming_acceptance_id, COUNT(*) AS item_count
			FROM incoming_acceptance_items
			GROUP BY incoming_acceptance_id
		) item_counts ON item_counts.incoming_acceptance_id=iar.id
		WHERE iar.work_item_id=$1
		  AND iar.status <> 'cancelled'
		ORDER BY iar.updated_at DESC, iar.created_at DESC, iar.id
	`, workItemID)
}

func (r *Repository) GetIncomingAcceptance(ctx context.Context, acceptanceID string) (Record, error) {
	record, err := r.QueryOne(ctx, `
		SELECT
			iar.*,
			w.name AS warehouse_name,
			pr.request_code AS procurement_request_code,
			pr.title AS procurement_request_title,
			cb.full_name AS created_by_name,
			sb.full_name AS submitted_by_name
		FROM incoming_acceptance_records iar
		LEFT JOIN warehouses w ON w.id=iar.warehouse_id
		LEFT JOIN procurement_requests pr ON pr.id=iar.procurement_request_id
		LEFT JOIN users cb ON cb.id=iar.created_by
		LEFT JOIN users sb ON sb.id=iar.submitted_by
		WHERE iar.id=$1
		  AND iar.status <> 'cancelled'
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT
			iai.*,
			er.name AS evidence_name,
			er.status AS evidence_status,
			pri.quantity_requested AS procurement_quantity_requested,
			pri.delivery_status AS procurement_delivery_status,
			pri.approval_status AS procurement_approval_status
		FROM incoming_acceptance_items iai
		LEFT JOIN evidence_records er ON er.id=iai.evidence_id
		LEFT JOIN procurement_request_items pri ON pri.id=iai.procurement_request_item_id
		WHERE iai.incoming_acceptance_id=$1
		ORDER BY iai.item_index, iai.created_at
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	record["items"] = items
	record["item_count"] = len(items)
	return record, nil
}

func (r *Repository) SaveIncomingAcceptance(ctx context.Context, acceptanceID string, header Record, items []Record, actorID, actorName string) (Record, error) {
	acceptanceID = cleanID(acceptanceID)
	projectID := cleanID(header["project_id"])
	workItemID := cleanID(header["work_item_id"])
	rawSummary, _ := json.Marshal(header["summary_json"])
	activityComment := cleanOptionalText(header["comment"])
	if activityComment == "" {
		activityComment = "Đã lưu nháp nghiệm thu đầu vào"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if acceptanceID != "" {
		existingRows, err := queryTx(ctx, tx, `
			SELECT *
			FROM incoming_acceptance_records
			WHERE id=$1
			FOR UPDATE
		`, acceptanceID)
		if err != nil {
			return nil, err
		}
		if len(existingRows) == 0 {
			return nil, sql.ErrNoRows
		}
		if fmt.Sprint(existingRows[0]["status"]) == "submitted" {
			return nil, validationError("Hồ sơ nghiệm thu đầu vào đã submit, không thể sửa", Record{"incoming_acceptance_id": acceptanceID})
		}
		if cleanID(existingRows[0]["project_id"]) != projectID {
			return nil, validationError("Hồ sơ nghiệm thu đầu vào không thuộc công trình", Record{"incoming_acceptance_id": acceptanceID})
		}
		rows, err := queryTx(ctx, tx, `
			UPDATE incoming_acceptance_records
			SET work_item_id=NULLIF($2,'')::uuid,
				procurement_request_id=NULLIF($3,'')::uuid,
				warehouse_id=NULLIF($4,'')::uuid,
				source_type=$5,
				supplier_partner_id=NULLIF($6,'')::uuid,
				supplier_name=NULLIF($7,''),
				subcontractor_partner_id=NULLIF($8,'')::uuid,
				subcontractor_name=NULLIF($9,''),
				delivery_reference=NULLIF($10,''),
				result=NULLIF($11,''),
				received_at=NULLIF($12,'')::date,
				summary_json=$13::jsonb,
				comment=NULLIF($14,''),
				updated_by=NULLIF($15,'')::uuid,
				updated_at=now()
			WHERE id=$1
			RETURNING id
		`, acceptanceID, workItemID, cleanID(header["procurement_request_id"]), cleanID(header["warehouse_id"]), header["source_type"],
			cleanID(header["supplier_partner_id"]), header["supplier_name"], cleanID(header["subcontractor_partner_id"]), header["subcontractor_name"],
			header["delivery_reference"], header["result"], header["received_at"],
			string(rawSummary), header["comment"], actorID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM incoming_acceptance_items WHERE incoming_acceptance_id=$1`, acceptanceID); err != nil {
			return nil, err
		}
	} else {
		code := fmt.Sprintf("IA-%s-%d-%s", projectID[:8], timeNowUnix(), newID()[:8])
		rows, err := queryTx(ctx, tx, `
			INSERT INTO incoming_acceptance_records (
				acceptance_code, project_id, work_item_id, procurement_request_id, warehouse_id,
				source_type, supplier_partner_id, supplier_name, subcontractor_partner_id, subcontractor_name, delivery_reference,
				status, result, received_at, summary_json, comment, created_by, updated_by
			)
			VALUES (
				$1, $2, NULLIF($3,'')::uuid, NULLIF($4,'')::uuid, NULLIF($5,'')::uuid,
				$6, NULLIF($7,'')::uuid, NULLIF($8,''), NULLIF($9,'')::uuid, NULLIF($10,''), NULLIF($11,''),
				'draft', NULLIF($12,''), NULLIF($13,'')::date, $14::jsonb, NULLIF($15,''), NULLIF($16,'')::uuid, NULLIF($16,'')::uuid
			)
			RETURNING id
		`, code, projectID, workItemID, cleanID(header["procurement_request_id"]), cleanID(header["warehouse_id"]), header["source_type"],
			cleanID(header["supplier_partner_id"]), header["supplier_name"], cleanID(header["subcontractor_partner_id"]), header["subcontractor_name"],
			header["delivery_reference"], header["result"], header["received_at"],
			string(rawSummary), header["comment"], actorID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		acceptanceID = cleanID(rows[0]["id"])
	}

	for _, item := range items {
		rawPayload, _ := json.Marshal(item["payload_json"])
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO incoming_acceptance_items (
				incoming_acceptance_id, project_id, work_item_id, item_index,
				material_id, material_code, material_name, procurement_request_item_id,
				delivered_quantity, accepted_quantity, rejected_quantity, unit,
				condition, defect_note, evidence_id, decision, comment, payload_json
			)
			VALUES (
				$1, $2, NULLIF($3,'')::uuid, $4,
				NULLIF($5,'')::uuid, NULLIF($6,''), $7, NULLIF($8,'')::uuid,
				$9, $10, $11, $12,
				$13, NULLIF($14,''), NULLIF($15,'')::uuid, $16, NULLIF($17,''), $18::jsonb
			)
		`, acceptanceID, projectID, workItemID, item["item_index"], cleanID(item["material_id"]), item["material_code"], item["material_name"],
			cleanID(item["procurement_request_item_id"]), item["delivered_quantity"], item["accepted_quantity"], item["rejected_quantity"],
			item["unit"], item["condition"], item["defect_note"], cleanID(item["evidence_id"]), item["decision"], item["comment"], string(rawPayload)); err != nil {
			return nil, err
		}
	}
	rawMetadata, _ := json.Marshal(Record{
		"incoming_acceptance_id": acceptanceID,
		"status":                 "draft",
		"source_type":            header["source_type"],
		"result":                 header["result"],
		"summary":                header["summary_json"],
	})
	if workItemID != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'project', $5::jsonb)
		`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE work_items
			SET updated_by=NULLIF($2,'')::uuid,
				updated_at=now()
			WHERE id=$1
		`, workItemID, actorID); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "incoming_acceptance_saved", "incoming_acceptance", acceptanceID, activityComment, Record{
		"work_item_id":           workItemID,
		"procurement_request_id": cleanID(header["procurement_request_id"]),
		"warehouse_id":           cleanID(header["warehouse_id"]),
		"source_type":            header["source_type"],
		"result":                 header["result"],
		"summary":                header["summary_json"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetIncomingAcceptance(ctx, acceptanceID)
}

func (r *Repository) SubmitIncomingAcceptance(ctx context.Context, acceptanceID string, result string, actorID, actorName string) (Record, error) {
	acceptanceID = cleanID(acceptanceID)
	result = cleanOptionalText(result)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		SELECT *
		FROM incoming_acceptance_records
		WHERE id=$1
		FOR UPDATE
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	record := rows[0]
	if fmt.Sprint(record["status"]) == "submitted" {
		return nil, validationError("Hồ sơ nghiệm thu đầu vào đã submit", Record{"incoming_acceptance_id": acceptanceID})
	}
	if fmt.Sprint(record["status"]) == "cancelled" {
		return nil, validationError("Hồ sơ nghiệm thu đầu vào đã hủy", Record{"incoming_acceptance_id": acceptanceID})
	}
	itemRows, err := queryTx(ctx, tx, `
		SELECT COUNT(*) AS total
		FROM incoming_acceptance_items
		WHERE incoming_acceptance_id=$1
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	if len(itemRows) == 0 || number(itemRows[0]["total"]) == 0 {
		return nil, validationError("Cần ít nhất một dòng vật tư để submit nghiệm thu đầu vào")
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE incoming_acceptance_records
		SET status='submitted',
			submitted_by=NULLIF($2,'')::uuid,
			submitted_at=now(),
			result=NULLIF($3,''),
			updated_by=NULLIF($2,'')::uuid,
			updated_at=now()
		WHERE id=$1
		RETURNING *
	`, acceptanceID, actorID, result)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, sql.ErrNoRows
	}
	projectID := cleanID(record["project_id"])
	workItemID := cleanID(record["work_item_id"])
	if result == "" {
		result = cleanOptionalText(record["result"])
	}
	comment := cleanOptionalText(record["comment"])
	if comment == "" {
		comment = fmt.Sprintf("Submit nghiệm thu đầu vào: %s", result)
	}
	rawMetadata, _ := json.Marshal(Record{
		"incoming_acceptance_id": acceptanceID,
		"status":                 "submitted",
		"result":                 result,
		"summary":                record["summary_json"],
	})
	if workItemID != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'project', $5::jsonb)
		`, workItemID, projectID, actorID, comment, string(rawMetadata)); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE work_items
			SET updated_by=NULLIF($2,'')::uuid,
				updated_at=now()
			WHERE id=$1
		`, workItemID, actorID); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "incoming_acceptance_submitted", "incoming_acceptance", acceptanceID, comment, Record{
		"work_item_id":           workItemID,
		"procurement_request_id": cleanID(record["procurement_request_id"]),
		"warehouse_id":           cleanID(record["warehouse_id"]),
		"source_type":            record["source_type"],
		"result":                 result,
		"summary":                record["summary_json"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetIncomingAcceptance(ctx, acceptanceID)
}

func (r *Repository) HasSubmittedWorkflowFormForWorkItem(ctx context.Context, workItemID string) (bool, error) {
	rows, err := r.Query(ctx, `
		SELECT COUNT(*) AS total
		FROM workflow_form_instances
		WHERE work_item_id=$1 AND status='submitted'
	`, workItemID)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && number(rows[0]["total"]) > 0, nil
}

func (r *Repository) WorkflowFormCatalog(ctx context.Context) ([]Record, error) {
	return r.Query(ctx, `
		WITH latest_published AS (
			SELECT DISTINCT ON (template_id)
				id,
				template_id
			FROM workflow_form_versions
			WHERE status='published'
			ORDER BY template_id, version_no DESC, published_at DESC NULLS LAST, created_at DESC
		),
		step_usage AS (
			SELECT form_version_id, COUNT(*)::int AS step_usage_count
			FROM pipeline_template_steps
			WHERE form_version_id IS NOT NULL
			GROUP BY form_version_id
		)
		SELECT
			t.id AS template_id,
			t.code AS template_code,
			t.name AS template_name,
			t.description AS template_description,
			t.form_type,
			t.status AS template_status,
			v.id AS version_id,
			v.version_no,
			v.status AS version_status,
			v.published_at,
			(v.id=lp.id) AS latest_published,
			COALESCE(su.step_usage_count, 0) AS step_usage_count,
			COUNT(f.id)::int AS field_count,
			COALESCE(
				jsonb_agg(f.field_key ORDER BY f.order_index, f.created_at, f.id) FILTER (WHERE f.id IS NOT NULL AND f.required),
				'[]'::jsonb
			) AS required_field_keys,
			COALESCE(
				jsonb_agg(
					jsonb_build_object(
						'field_key', f.field_key,
						'label', f.label,
						'field_type', f.field_type,
						'required', f.required,
						'order_index', f.order_index
					)
					ORDER BY f.order_index, f.created_at, f.id
				) FILTER (WHERE f.id IS NOT NULL),
				'[]'::jsonb
			) AS fields
		FROM workflow_form_templates t
		JOIN workflow_form_versions v ON v.template_id=t.id AND v.status='published'
		JOIN latest_published lp ON lp.template_id=t.id
		LEFT JOIN workflow_form_fields f ON f.form_version_id=v.id
		LEFT JOIN step_usage su ON su.form_version_id=v.id
		WHERE t.status='active'
		GROUP BY t.id, t.code, t.name, t.description, t.form_type, t.status, v.id, v.version_no, v.status, v.published_at, lp.id, su.step_usage_count
		ORDER BY t.form_type, t.name, v.version_no DESC
	`)
}

func (r *Repository) WorkflowFormTemplateLibrary(ctx context.Context) ([]Record, error) {
	return r.Query(ctx, `
		WITH latest_published AS (
			SELECT DISTINCT ON (template_id)
				id,
				template_id,
				version_no,
				status,
				published_at
			FROM workflow_form_versions
			WHERE status='published'
			ORDER BY template_id, version_no DESC, published_at DESC NULLS LAST, created_at DESC
		),
		version_counts AS (
			SELECT
				template_id,
				COUNT(*)::int AS version_count,
				COUNT(*) FILTER (WHERE status='published')::int AS published_version_count,
				COUNT(*) FILTER (WHERE status='draft')::int AS draft_version_count
			FROM workflow_form_versions
			GROUP BY template_id
		),
		field_counts AS (
			SELECT v.template_id, COUNT(f.id)::int AS field_count
			FROM workflow_form_versions v
			LEFT JOIN workflow_form_fields f ON f.form_version_id=v.id
			GROUP BY v.template_id
		),
		step_usage AS (
			SELECT v.template_id, COUNT(pts.id)::int AS step_usage_count
			FROM workflow_form_versions v
			LEFT JOIN pipeline_template_steps pts ON pts.form_version_id=v.id
			GROUP BY v.template_id
		),
		instance_usage AS (
			SELECT form_template_id AS template_id, COUNT(*)::int AS instance_count
			FROM workflow_form_instances
			WHERE status <> 'cancelled'
			GROUP BY form_template_id
		)
		SELECT
			t.id AS template_id,
			t.code AS template_code,
			t.name AS template_name,
			t.description AS template_description,
			t.form_type,
			t.status AS template_status,
			t.created_at,
			t.updated_at,
			lp.id AS latest_version_id,
			lp.version_no AS latest_version_no,
			lp.status AS latest_version_status,
			lp.published_at AS latest_published_at,
			COALESCE(vc.version_count, 0) AS version_count,
			COALESCE(vc.published_version_count, 0) AS published_version_count,
			COALESCE(vc.draft_version_count, 0) AS draft_version_count,
			COALESCE(fc.field_count, 0) AS field_count,
			COALESCE(su.step_usage_count, 0) AS step_usage_count,
			COALESCE(iu.instance_count, 0) AS instance_count
		FROM workflow_form_templates t
		LEFT JOIN latest_published lp ON lp.template_id=t.id
		LEFT JOIN version_counts vc ON vc.template_id=t.id
		LEFT JOIN field_counts fc ON fc.template_id=t.id
		LEFT JOIN step_usage su ON su.template_id=t.id
		LEFT JOIN instance_usage iu ON iu.template_id=t.id
		ORDER BY CASE WHEN t.status='active' THEN 0 ELSE 1 END, t.form_type, t.name
	`)
}

func (r *Repository) WorkflowFormTemplateDetail(ctx context.Context, templateID string) (Record, error) {
	template, err := r.QueryOne(ctx, `
		WITH step_usage AS (
			SELECT v.template_id, COUNT(pts.id)::int AS step_usage_count
			FROM workflow_form_versions v
			LEFT JOIN pipeline_template_steps pts ON pts.form_version_id=v.id
			GROUP BY v.template_id
		),
		instance_usage AS (
			SELECT form_template_id AS template_id, COUNT(*)::int AS instance_count
			FROM workflow_form_instances
			WHERE status <> 'cancelled'
			GROUP BY form_template_id
		)
		SELECT
			t.id AS template_id,
			t.code AS template_code,
			t.name AS template_name,
			t.description AS template_description,
			t.form_type,
			t.status AS template_status,
			t.created_by,
			t.created_at,
			t.updated_at,
			COALESCE(su.step_usage_count, 0) AS step_usage_count,
			COALESCE(iu.instance_count, 0) AS instance_count
		FROM workflow_form_templates t
		LEFT JOIN step_usage su ON su.template_id=t.id
		LEFT JOIN instance_usage iu ON iu.template_id=t.id
		WHERE t.id=$1
	`, templateID)
	if err != nil {
		return nil, err
	}
	versions, err := r.workflowFormVersions(ctx, templateID)
	if err != nil {
		return nil, err
	}
	template["versions"] = versions
	return template, nil
}

func (r *Repository) WorkflowFormVersionDetail(ctx context.Context, versionID string) (Record, error) {
	rows, err := r.Query(ctx, workflowFormVersionDetailSQL("WHERE v.id=$1"), versionID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	return rows[0], nil
}

func (r *Repository) workflowFormVersions(ctx context.Context, templateID string) ([]Record, error) {
	return r.Query(ctx, workflowFormVersionDetailSQL("WHERE v.template_id=$1"), templateID)
}

func workflowFormVersionDetailSQL(where string) string {
	return fmt.Sprintf(`
		WITH latest_published AS (
			SELECT DISTINCT ON (template_id)
				id,
				template_id
			FROM workflow_form_versions
			WHERE status='published'
			ORDER BY template_id, version_no DESC, published_at DESC NULLS LAST, created_at DESC
		),
		step_usage AS (
			SELECT form_version_id, COUNT(*)::int AS step_usage_count
			FROM pipeline_template_steps
			WHERE form_version_id IS NOT NULL
			GROUP BY form_version_id
		),
		instance_usage AS (
			SELECT form_version_id, COUNT(*)::int AS instance_count
			FROM workflow_form_instances
			WHERE status <> 'cancelled'
			GROUP BY form_version_id
		)
		SELECT
			v.id AS version_id,
			v.template_id,
			t.code AS template_code,
			t.name AS template_name,
			t.form_type,
			t.status AS template_status,
			v.version_no,
			v.status AS version_status,
			v.schema_json,
			v.validation_json,
			v.published_at,
			v.created_by,
			v.created_at,
			v.updated_at,
			(v.id=lp.id) AS latest_published,
			COALESCE(su.step_usage_count, 0) AS step_usage_count,
			COALESCE(iu.instance_count, 0) AS instance_count,
			COUNT(f.id)::int AS field_count,
			COALESCE(
				jsonb_agg(
					jsonb_build_object(
						'id', f.id,
						'form_version_id', f.form_version_id,
						'field_key', f.field_key,
						'label', f.label,
						'field_type', f.field_type,
						'required', f.required,
						'options_json', f.options_json,
						'validation_json', f.validation_json,
						'help_text', f.help_text,
						'order_index', f.order_index,
						'visibility_condition_json', f.visibility_condition_json,
						'created_at', f.created_at
					)
					ORDER BY f.order_index, f.created_at, f.id
				) FILTER (WHERE f.id IS NOT NULL),
				'[]'::jsonb
			) AS fields
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		LEFT JOIN latest_published lp ON lp.template_id=v.template_id
		LEFT JOIN step_usage su ON su.form_version_id=v.id
		LEFT JOIN instance_usage iu ON iu.form_version_id=v.id
		LEFT JOIN workflow_form_fields f ON f.form_version_id=v.id
		%s
		GROUP BY v.id, t.id, t.code, t.name, t.form_type, t.status, lp.id, su.step_usage_count, iu.instance_count
		ORDER BY v.version_no DESC
	`, where)
}

func (r *Repository) CreateWorkflowFormTemplate(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	row, err := r.QueryOne(ctx, `
		INSERT INTO workflow_form_templates (code, name, description, form_type, status, created_by)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6,'')::uuid)
		RETURNING id
	`, body["code"], body["name"], body["description"], body["form_type"], body["status"], actorID)
	if err != nil {
		return nil, err
	}
	templateID := cleanID(row["id"])
	_ = r.AddAudit(ctx, "", actorID, actorName, "workflow_form_template_created", "workflow_form_template", templateID, fmt.Sprintf("Tạo workflow form template %s", body["name"]), Record{"code": body["code"], "form_type": body["form_type"]})
	return r.WorkflowFormTemplateDetail(ctx, templateID)
}

func (r *Repository) UpdateWorkflowFormTemplate(ctx context.Context, templateID string, body Record, actorID, actorName string) (Record, error) {
	if fmt.Sprint(body["status"]) == "archived" {
		usageRows, err := r.Query(ctx, `
			SELECT COUNT(*) AS total
			FROM pipeline_template_steps pts
			JOIN workflow_form_versions v ON v.id=pts.form_version_id
			WHERE v.template_id=$1
		`, templateID)
		if err != nil {
			return nil, err
		}
		if len(usageRows) > 0 && number(usageRows[0]["total"]) > 0 {
			return nil, validationError("Không thể archive workflow form template đang được pipeline step sử dụng", Record{"step_usage_count": usageRows[0]["total"]})
		}
	}
	row, err := r.QueryOne(ctx, `
		UPDATE workflow_form_templates
		SET name=$2,
			description=$3,
			status=$4,
			updated_at=now()
		WHERE id=$1
		RETURNING id, name
	`, templateID, body["name"], body["description"], body["status"])
	if err != nil {
		return nil, err
	}
	_ = r.AddAudit(ctx, "", actorID, actorName, "workflow_form_template_updated", "workflow_form_template", templateID, fmt.Sprintf("Cập nhật workflow form template %s", row["name"]), Record{"status": body["status"]})
	return r.WorkflowFormTemplateDetail(ctx, templateID)
}

func (r *Repository) ArchiveWorkflowFormTemplate(ctx context.Context, templateID string, actorID, actorName string) (Record, error) {
	usageRows, err := r.Query(ctx, `
		SELECT COUNT(*) AS total
		FROM pipeline_template_steps pts
		JOIN workflow_form_versions v ON v.id=pts.form_version_id
		WHERE v.template_id=$1
	`, templateID)
	if err != nil {
		return nil, err
	}
	if len(usageRows) > 0 && number(usageRows[0]["total"]) > 0 {
		return nil, validationError("Không thể archive workflow form template đang được pipeline step sử dụng", Record{"step_usage_count": usageRows[0]["total"]})
	}
	row, err := r.QueryOne(ctx, `
		UPDATE workflow_form_templates
		SET status='archived', updated_at=now()
		WHERE id=$1
		RETURNING name
	`, templateID)
	if err != nil {
		return nil, err
	}
	_ = r.AddAudit(ctx, "", actorID, actorName, "workflow_form_template_archived", "workflow_form_template", templateID, fmt.Sprintf("Archive workflow form template %s", row["name"]), Record{})
	return r.WorkflowFormTemplateDetail(ctx, templateID)
}

func (r *Repository) CreateWorkflowFormVersion(ctx context.Context, templateID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var templateName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM workflow_form_templates WHERE id=$1 AND status='active' FOR UPDATE`, templateID).Scan(&templateName); err != nil {
		return nil, err
	}
	sourceVersionID := cleanID(body["source_version_id"])
	schemaJSON := body["schema_json"]
	validationJSON := body["validation_json"]
	fields := workflowFieldRecords(body["fields"])
	if sourceVersionID != "" {
		sourceRows, err := tx.QueryContext(ctx, `
			SELECT schema_json, validation_json
			FROM workflow_form_versions
			WHERE id=$1 AND template_id=$2
		`, sourceVersionID, templateID)
		if err != nil {
			return nil, err
		}
		source, err := scanRows(sourceRows)
		sourceRows.Close()
		if err != nil {
			return nil, err
		}
		if len(source) == 0 {
			return nil, sql.ErrNoRows
		}
		if recordEmpty(schemaJSON) {
			schemaJSON = source[0]["schema_json"]
		}
		if recordEmpty(validationJSON) {
			validationJSON = source[0]["validation_json"]
		}
		if len(fields) == 0 {
			copied, err := workflowFormFieldRowsTx(ctx, tx, sourceVersionID)
			if err != nil {
				return nil, err
			}
			fields = copied
		}
	}
	nextVersionNo := 1
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_no),0)+1 FROM workflow_form_versions WHERE template_id=$1`, templateID).Scan(&nextVersionNo); err != nil {
		return nil, err
	}
	rawSchema, _ := json.Marshal(defaultRecord(schemaJSON))
	rawValidation, _ := json.Marshal(defaultRecord(validationJSON))
	var versionID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO workflow_form_versions (template_id, version_no, status, schema_json, validation_json, created_by)
		VALUES ($1, $2, 'draft', $3::jsonb, $4::jsonb, NULLIF($5,'')::uuid)
		RETURNING id
	`, templateID, nextVersionNo, string(rawSchema), string(rawValidation), actorID).Scan(&versionID); err != nil {
		return nil, err
	}
	if err := insertWorkflowFormFieldsTx(ctx, tx, versionID, fields); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "workflow_form_version_created", "workflow_form_version", versionID, fmt.Sprintf("Tạo draft workflow form %s v%d", templateName, nextVersionNo), Record{"template_id": templateID, "source_version_id": sourceVersionID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.WorkflowFormVersionDetail(ctx, versionID)
}

func (r *Repository) UpdateWorkflowFormVersion(ctx context.Context, versionID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var templateID, templateName, status string
	var versionNo int
	if err := tx.QueryRowContext(ctx, `
		SELECT v.template_id::text, t.name, v.version_no, v.status
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$1
		FOR UPDATE
	`, versionID).Scan(&templateID, &templateName, &versionNo, &status); err != nil {
		return nil, err
	}
	if status != "draft" {
		return nil, validationError("Chỉ draft workflow form version mới được chỉnh sửa", Record{"status": status})
	}
	rawSchema, _ := json.Marshal(defaultRecord(body["schema_json"]))
	rawValidation, _ := json.Marshal(defaultRecord(body["validation_json"]))
	if _, err := tx.ExecContext(ctx, `
		UPDATE workflow_form_versions
		SET schema_json=$2::jsonb,
			validation_json=$3::jsonb,
			updated_at=now()
		WHERE id=$1
	`, versionID, string(rawSchema), string(rawValidation)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM workflow_form_fields WHERE form_version_id=$1`, versionID); err != nil {
		return nil, err
	}
	if err := insertWorkflowFormFieldsTx(ctx, tx, versionID, workflowFieldRecords(body["fields"])); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "workflow_form_version_updated", "workflow_form_version", versionID, fmt.Sprintf("Cập nhật draft workflow form %s v%d", templateName, versionNo), Record{"template_id": templateID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.WorkflowFormVersionDetail(ctx, versionID)
}

func (r *Repository) PublishWorkflowFormVersion(ctx context.Context, versionID string, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var templateID, templateName, status string
	var versionNo int
	if err := tx.QueryRowContext(ctx, `
		SELECT v.template_id::text, t.name, v.version_no, v.status
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$1
		FOR UPDATE
	`, versionID).Scan(&templateID, &templateName, &versionNo, &status); err != nil {
		return nil, err
	}
	if status != "draft" {
		return nil, validationError("Chỉ draft workflow form version mới được publish", Record{"status": status})
	}
	var fieldCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_form_fields WHERE form_version_id=$1`, versionID).Scan(&fieldCount); err != nil {
		return nil, err
	}
	if fieldCount == 0 {
		return nil, validationError("Workflow form version cần ít nhất một field trước khi publish")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE workflow_form_versions
		SET status='published',
			published_at=COALESCE(published_at, now()),
			updated_at=now()
		WHERE id=$1
	`, versionID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "workflow_form_version_published", "workflow_form_version", versionID, fmt.Sprintf("Publish workflow form %s v%d", templateName, versionNo), Record{"template_id": templateID, "field_count": fieldCount}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.WorkflowFormVersionDetail(ctx, versionID)
}

func (r *Repository) ArchiveWorkflowFormVersion(ctx context.Context, versionID string, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var templateID, templateName, status string
	var versionNo int
	if err := tx.QueryRowContext(ctx, `
		SELECT v.template_id::text, t.name, v.version_no, v.status
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$1
		FOR UPDATE
	`, versionID).Scan(&templateID, &templateName, &versionNo, &status); err != nil {
		return nil, err
	}
	if status == "archived" {
		return r.WorkflowFormVersionDetail(ctx, versionID)
	}
	var stepUsage int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM pipeline_template_steps WHERE form_version_id=$1`, versionID).Scan(&stepUsage); err != nil {
		return nil, err
	}
	if stepUsage > 0 {
		return nil, validationError("Không thể archive workflow form version đang được pipeline step sử dụng", Record{"step_usage_count": stepUsage})
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workflow_form_versions SET status='archived', updated_at=now() WHERE id=$1`, versionID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "workflow_form_version_archived", "workflow_form_version", versionID, fmt.Sprintf("Archive workflow form %s v%d", templateName, versionNo), Record{"template_id": templateID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.WorkflowFormVersionDetail(ctx, versionID)
}

func workflowFieldRecords(value any) []Record {
	fields, ok := value.([]Record)
	if ok {
		return fields
	}
	rawFields, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]Record, 0, len(rawFields))
	for _, raw := range rawFields {
		if record, ok := raw.(Record); ok {
			out = append(out, record)
			continue
		}
		if record, ok := raw.(map[string]any); ok {
			out = append(out, Record(record))
		}
	}
	return out
}

func workflowFormFieldRowsTx(ctx context.Context, tx *sql.Tx, versionID string) ([]Record, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT field_key, label, field_type, required, options_json, validation_json, help_text, order_index, visibility_condition_json
		FROM workflow_form_fields
		WHERE form_version_id=$1
		ORDER BY order_index, created_at, id
	`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func insertWorkflowFormFieldsTx(ctx context.Context, tx *sql.Tx, versionID string, fields []Record) error {
	for index, field := range fields {
		orderIndex := int(number(field["order_index"]))
		if orderIndex <= 0 {
			orderIndex = (index + 1) * 10
		}
		rawOptions, _ := json.Marshal(defaultArray(field["options_json"]))
		rawValidation, _ := json.Marshal(defaultRecord(field["validation_json"]))
		rawVisibility, _ := json.Marshal(defaultRecord(field["visibility_condition_json"]))
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workflow_form_fields
				(form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index, visibility_condition_json)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, NULLIF($8,''), $9, $10::jsonb)
		`, versionID, field["field_key"], field["label"], field["field_type"], boolValue(field["required"]), string(rawOptions), string(rawValidation), cleanOptionalText(field["help_text"]), orderIndex, string(rawVisibility)); err != nil {
			return err
		}
	}
	return nil
}

func defaultRecord(value any) any {
	if value == nil {
		return Record{}
	}
	return value
}

func defaultArray(value any) any {
	if value == nil {
		return []any{}
	}
	return value
}

func recordEmpty(value any) bool {
	if value == nil {
		return true
	}
	if record, ok := value.(Record); ok {
		return len(record) == 0
	}
	if record, ok := value.(map[string]any); ok {
		return len(record) == 0
	}
	return false
}

func (r *Repository) DefaultWorkflowFormVersionID(ctx context.Context, stepType string, checklistRequired bool, stepContext ...string) (string, error) {
	stepName := ""
	if len(stepContext) > 0 {
		stepName = stepContext[0]
	}
	code := defaultWorkflowFormCode(stepType, checklistRequired, stepName)
	row, err := r.QueryOne(ctx, `
		SELECT v.id
		FROM workflow_form_templates t
		JOIN workflow_form_versions v ON v.template_id=t.id
		WHERE t.code=$1 AND t.status='active' AND v.status='published'
		ORDER BY v.version_no DESC
		LIMIT 1
	`, code)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return cleanID(row["id"]), nil
}

func defaultWorkflowFormCode(stepType string, checklistRequired bool, stepName string) string {
	normalizedName := normalizedWorkflowStepName(stepName)
	if stepType == "daily_report" {
		if isConstructionDailyReportStep(normalizedName) {
			return "builtin_construction_daily_report"
		}
		return "builtin_daily_report"
	}
	if stepType == "meeting_minutes" {
		return "builtin_meeting_minutes"
	}
	if statusIn(stepType, "checklist", "acceptance_check", "handover_check") || checklistRequired {
		if strings.Contains(normalizedName, "chu dau tu") || strings.Contains(normalizedName, "client") || strings.Contains(normalizedName, "owner") {
			return "builtin_owner_client_checklist"
		}
		if strings.Contains(normalizedName, "thau phu") || strings.Contains(normalizedName, "subcontractor") {
			return "builtin_subcontractor_checklist"
		}
		return "builtin_checklist"
	}
	return "builtin_generic_submission"
}

func normalizedWorkflowStepName(stepName string) string {
	return strings.ToLower(strings.TrimSpace(asciiFold(stepName)))
}

func isConstructionDailyReportStep(normalizedName string) bool {
	return strings.Contains(normalizedName, "thi cong") ||
		strings.Contains(normalizedName, "construction") ||
		strings.Contains(normalizedName, "cong truong") ||
		strings.Contains(normalizedName, "site report")
}

func (r *Repository) TemplatePublishFormIssues(ctx context.Context, templateID string) ([]Record, error) {
	return r.Query(ctx, templatePublishFormIssuesSQL(), templateID)
}

func templatePublishFormIssuesSQL() string {
	return `
		SELECT
			n.id AS node_id,
			n.name AS node_name,
			pts.id AS step_id,
			pts.name AS step_name,
			pts.step_type,
			pts.form_required,
			pts.form_version_id,
			COALESCE(wfv.status, 'missing') AS form_version_status,
			wfv.version_no AS form_version_no,
			wft.name AS form_name,
			wft.status AS form_template_status
		FROM pipeline_template_steps pts
		JOIN pipeline_template_nodes n ON n.id=pts.node_id
		LEFT JOIN workflow_form_versions wfv ON wfv.id=pts.form_version_id
		LEFT JOIN workflow_form_templates wft ON wft.id=wfv.template_id
		WHERE n.template_id=$1
		  AND COALESCE(pts.form_required, false)
		  AND (
			pts.form_version_id IS NULL
			OR wfv.id IS NULL
			OR wfv.status <> 'published'
			OR wft.status <> 'active'
		  )
		ORDER BY n.order_index, pts.order_index, pts.created_at, pts.id
	`
}

func (r *Repository) SaveWorkflowForm(ctx context.Context, instanceID string, answers Record, comment string, submit bool, actorID, actorName string) (Record, error) {
	if answers == nil {
		answers = Record{}
	}
	rawSnapshot, _ := json.Marshal(answers)
	eventType := "draft_saved"
	toStatus := "draft"
	if submit {
		eventType = "submitted"
		toStatus = "submitted"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		SELECT wfi.*, wft.code AS form_code
		FROM workflow_form_instances wfi
		JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		WHERE wfi.id=$1
		FOR UPDATE OF wfi
	`, instanceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	instance := rows[0]
	fromStatus := fmt.Sprint(instance["status"])
	if statusIn(fromStatus, "cancelled") || (!submit && fromStatus == "submitted") || (submit && fromStatus == "submitted") {
		return nil, validationError("Trạng thái workflow form đã thay đổi, vui lòng tải lại", Record{"status": fromStatus})
	}
	revisionNo := int(number(instance["current_revision"]))
	if submit {
		revisionNo++
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM workflow_form_answers wfa
		WHERE wfa.instance_id=$1
		  AND NOT EXISTS (
			SELECT 1
			FROM jsonb_object_keys($2::jsonb) AS submitted(field_key)
			WHERE submitted.field_key=wfa.field_key
		  )
	`, instanceID, string(rawSnapshot)); err != nil {
		return nil, err
	}
	for key, value := range answers {
		rawValue, _ := json.Marshal(value)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workflow_form_answers (instance_id, field_key, field_type, value_json, revision_no, updated_by)
			SELECT $1, wff.field_key, wff.field_type, $3::jsonb, $4, NULLIF($5,'')::uuid
			FROM workflow_form_fields wff
			WHERE wff.form_version_id=$2 AND wff.field_key=$6
			ON CONFLICT (instance_id, field_key)
			DO UPDATE SET value_json=EXCLUDED.value_json, field_type=EXCLUDED.field_type, revision_no=EXCLUDED.revision_no, updated_by=EXCLUDED.updated_by, updated_at=now()
		`, instanceID, instance["form_version_id"], string(rawValue), revisionNo, actorID, key); err != nil {
			return nil, err
		}
	}
	if submit {
		if _, err := tx.ExecContext(ctx, `
			UPDATE workflow_form_instances
			SET status='submitted',
				current_revision=$2,
				answers_snapshot_json=$3::jsonb,
				submitted_by=NULLIF($4,'')::uuid,
				submitted_at=now(),
				updated_at=now()
			WHERE id=$1
		`, instanceID, revisionNo, string(rawSnapshot), actorID); err != nil {
			return nil, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE workflow_form_instances
			SET answers_snapshot_json=$2::jsonb,
				updated_at=now()
			WHERE id=$1
		`, instanceID, string(rawSnapshot)); err != nil {
			return nil, err
		}
	}
	rawMetadata, _ := json.Marshal(Record{
		"form_type":       instance["form_type"],
		"form_version_id": instance["form_version_id"],
		"revision_no":     revisionNo,
	})
	workItemID := cleanID(instance["work_item_id"])
	projectID := cleanID(instance["project_id"])
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_form_events (instance_id, project_id, work_item_id, actor_id, event_type, from_status, to_status, comment, metadata_json)
		VALUES ($1, $2, NULLIF($3,'')::uuid, NULLIF($4,'')::uuid, $5, $6, $7, NULLIF($8,''), $9::jsonb)
	`, instanceID, projectID, workItemID, actorID, eventType, fromStatus, toStatus, comment, string(rawMetadata)); err != nil {
		return nil, err
	}
	if workItemID != "" {
		activityComment := comment
		if activityComment == "" {
			if submit {
				activityComment = "Workflow form submitted"
			} else {
				activityComment = "Workflow form draft saved"
			}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, $5::jsonb)
		`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
			return nil, err
		}
		if submit {
			generatedWorkItemIDs := []string{}
			if fmt.Sprint(instance["form_type"]) == "meeting_minutes" {
				var err error
				generatedWorkItemIDs, err = createMeetingMinutesActionItemsTx(ctx, tx, instance, answers, actorID)
				if err != nil {
					return nil, err
				}
			} else if fmt.Sprint(instance["form_type"]) == "daily_report" {
				var err error
				generatedWorkItemIDs, err = createDailyReportNextItemsTx(ctx, tx, instance, answers, actorID)
				if err != nil {
					return nil, err
				}
			}
			payload := Record{
				"workflow_form_instance_id": instanceID,
				"workflow_form_version_id":  instance["form_version_id"],
				"workflow_form_type":        instance["form_type"],
				"workflow_form_answers":     answers,
			}
			if len(generatedWorkItemIDs) > 0 {
				payload["generated_work_item_ids"] = generatedWorkItemIDs
			}
			if fmt.Sprint(instance["form_type"]) == "checklist" {
				if checklistItems, ok := answers["checklist_items"]; ok {
					payload["checklist_items"] = checklistItems
					payload["checklist"] = checklistItems
				} else {
					payload["checklist"] = []Record{{
						"label":    "Submitted workflow form",
						"checked":  true,
						"source":   "workflow_form",
						"form_id":  instanceID,
						"revision": revisionNo,
					}}
				}
			}
			rawPayload, _ := json.Marshal(payload)
			if _, err := tx.ExecContext(ctx, `
				UPDATE work_items
				SET completion_payload_json=COALESCE(completion_payload_json, '{}'::jsonb) || $2::jsonb,
					updated_by=NULLIF($3,'')::uuid,
					updated_at=now()
				WHERE id=$1
			`, workItemID, string(rawPayload), actorID); err != nil {
				return nil, err
			}
		}
		auditDescription := "Lưu nháp workflow form"
		if submit {
			auditDescription = "Submit workflow form"
		}
		if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "workflow_form_"+eventType, "workflow_form_instance", instanceID, auditDescription, Record{"work_item_id": workItemID, "form_type": instance["form_type"], "revision_no": revisionNo}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetWorkflowFormInstanceDetail(ctx, instanceID)
}

func (r *Repository) SignWorkflowForm(ctx context.Context, instanceID, purpose, comment string, visualFile StoredFileMetadata, strokes []WorkflowFormSignatureStroke, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		SELECT
			wfi.*,
			wft.code AS form_code,
			wft.name AS form_name,
			wfv.version_no AS form_version_no
		FROM workflow_form_instances wfi
		JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
		JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		WHERE wfi.id=$1
		FOR UPDATE
	`, instanceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	instance := rows[0]
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Workflow form phải được submit trước khi ký nội bộ", Record{"status": instance["status"]})
	}
	projectID := cleanID(instance["project_id"])
	workItemID := cleanID(instance["work_item_id"])
	formVersionID := cleanID(instance["form_version_id"])
	revisionNo := int(number(instance["current_revision"]))
	if workItemID == "" || projectID == "" || formVersionID == "" {
		return nil, validationError("Workflow form thiếu liên kết để ký", Record{"workflow_form_instance_id": instanceID})
	}

	duplicateRows, err := queryTx(ctx, tx, `
		SELECT COUNT(*) AS total
		FROM workflow_form_signatures
		WHERE form_instance_id=$1 AND signer_user_id=$2 AND purpose=$3
	`, instanceID, actorID, purpose)
	if err != nil {
		return nil, err
	}
	if len(duplicateRows) > 0 && number(duplicateRows[0]["total"]) > 0 {
		return nil, validationError("Bạn đã ký workflow form này cho mục đích này", Record{"purpose": purpose})
	}

	roleRows, err := queryTx(ctx, tx, `
		SELECT COALESCE(jsonb_agg(role_name ORDER BY role_name), '[]'::jsonb) AS roles
		FROM (
			SELECT DISTINCT r.display_name AS role_name
			FROM user_roles ur
			JOIN roles r ON r.id=ur.role_id
			WHERE ur.user_id=$1
		) signer_roles
	`, actorID)
	if err != nil {
		return nil, err
	}
	signerRoles := any([]any{})
	if len(roleRows) > 0 && roleRows[0]["roles"] != nil {
		signerRoles = roleRows[0]["roles"]
	}
	signerName := strings.TrimSpace(actorName)
	if signerName == "" {
		signerName = actorID
	}
	payload := Record{
		"source":                    "workflow_form_signature",
		"workflow_form_instance_id": instanceID,
		"workflow_form_version_id":  formVersionID,
		"workflow_form_revision_no": revisionNo,
		"workflow_form_type":        instance["form_type"],
		"workflow_form_code":        instance["form_code"],
		"workflow_form_title":       instance["title"],
		"answers_snapshot":          instance["answers_snapshot_json"],
		"project_id":                projectID,
		"work_item_id":              workItemID,
		"purpose":                   purpose,
		"comment":                   comment,
		"signer_user_id":            actorID,
		"signer_name":               signerName,
		"signer_roles":              signerRoles,
		"visual_signature": Record{
			"storage_provider": visualFile.StorageProvider,
			"storage_key":      visualFile.StorageKey,
			"mime_type":        visualFile.MIMEType,
			"file_size":        visualFile.FileSize,
			"checksum_sha256":  visualFile.ChecksumSHA256,
			"strokes":          strokes,
		},
	}
	rawPayload, _ := json.Marshal(payload)
	hashBytes := sha256.Sum256(rawPayload)
	payloadHash := hex.EncodeToString(hashBytes[:])
	rawRoles, _ := json.Marshal(signerRoles)

	signatureRows, err := queryTx(ctx, tx, `
		INSERT INTO workflow_form_signatures
			(project_id, work_item_id, form_instance_id, form_version_id, form_revision_no, purpose, signer_user_id, signer_name, signer_roles_json, comment, payload_json, payload_hash, visual_storage_provider, visual_storage_key, visual_mime_type, visual_file_size, visual_checksum_sha256)
		VALUES
			($1, $2, $3, $4, $5, $6, NULLIF($7,'')::uuid, $8, $9::jsonb, NULLIF($10,''), $11::jsonb, $12, $13, $14, $15, $16, $17)
		RETURNING *
	`, projectID, workItemID, instanceID, formVersionID, revisionNo, purpose, actorID, signerName, string(rawRoles), comment, string(rawPayload), payloadHash, visualFile.StorageProvider, visualFile.StorageKey, visualFile.MIMEType, visualFile.FileSize, visualFile.ChecksumSHA256)
	if err != nil {
		return nil, err
	}
	if len(signatureRows) == 0 {
		return nil, sql.ErrNoRows
	}
	signatureID := cleanID(signatureRows[0]["id"])
	rawMetadata, _ := json.Marshal(Record{
		"workflow_form_instance_id": instanceID,
		"signature_id":              signatureID,
		"purpose":                   purpose,
		"payload_hash":              payloadHash,
		"form_revision_no":          revisionNo,
		"visual_checksum_sha256":    visualFile.ChecksumSHA256,
	})
	eventComment := comment
	if eventComment == "" {
		eventComment = "Workflow form signed"
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_form_events (instance_id, project_id, work_item_id, actor_id, event_type, from_status, to_status, comment, metadata_json)
		VALUES ($1, $2, $3, NULLIF($4,'')::uuid, 'signed', 'submitted', 'submitted', $5, $6::jsonb)
	`, instanceID, projectID, workItemID, actorID, eventComment, string(rawMetadata)); err != nil {
		return nil, err
	}
	activityComment := fmt.Sprintf("Workflow form signed: %s", purpose)
	if comment != "" {
		activityComment = comment
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'internal', $5::jsonb)
	`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "workflow_form_signed", "workflow_form_signature", signatureID, "Ký nội bộ workflow form", Record{
		"work_item_id":              workItemID,
		"workflow_form_instance_id": instanceID,
		"purpose":                   purpose,
		"payload_hash":              payloadHash,
		"form_revision_no":          revisionNo,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetWorkflowFormInstanceDetail(ctx, instanceID)
}

func (r *Repository) CreateWorkflowFormPDFExport(ctx context.Context, instanceID, fileName, title string, pdfFile StoredFileMetadata, payloadHash, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		SELECT
			wfi.*,
			wft.code AS form_code,
			wft.name AS form_name,
			wfv.version_no AS form_version_no
		FROM workflow_form_instances wfi
		JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
		JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		WHERE wfi.id=$1
		FOR UPDATE
	`, instanceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	instance := rows[0]
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Workflow form phải được submit trước khi export PDF", Record{"status": instance["status"]})
	}
	projectID := cleanID(instance["project_id"])
	workItemID := cleanID(instance["work_item_id"])
	formVersionID := cleanID(instance["form_version_id"])
	revisionNo := int(number(instance["current_revision"]))
	if workItemID == "" || projectID == "" || formVersionID == "" {
		return nil, validationError("Workflow form thiếu liên kết để export PDF", Record{"workflow_form_instance_id": instanceID})
	}
	if strings.TrimSpace(title) == "" {
		title = fmt.Sprint(instance["title"])
	}
	if strings.TrimSpace(fileName) == "" {
		fileName = fmt.Sprintf("workflow-form-rev-%d.pdf", revisionNo)
	}
	metadata := Record{
		"workflow_form_type":       instance["form_type"],
		"workflow_form_code":       instance["form_code"],
		"workflow_form_title":      instance["title"],
		"workflow_form_version_no": instance["form_version_no"],
		"form_revision_no":         revisionNo,
	}
	rawMetadata, _ := json.Marshal(metadata)

	exportRows, err := queryTx(ctx, tx, `
		INSERT INTO workflow_form_pdf_exports
			(project_id, work_item_id, form_instance_id, form_version_id, form_revision_no, title, file_name, storage_provider, storage_key, mime_type, file_size, checksum_sha256, payload_hash, metadata_json, generated_by, generated_by_name)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, NULLIF($15,'')::uuid, NULLIF($16,''))
		RETURNING *
	`, projectID, workItemID, instanceID, formVersionID, revisionNo, title, fileName, pdfFile.StorageProvider, pdfFile.StorageKey, pdfFile.MIMEType, pdfFile.FileSize, pdfFile.ChecksumSHA256, payloadHash, string(rawMetadata), actorID, actorName)
	if err != nil {
		return nil, err
	}
	if len(exportRows) == 0 {
		return nil, sql.ErrNoRows
	}
	exportID := cleanID(exportRows[0]["id"])
	eventMetadata := Record{
		"workflow_form_instance_id": instanceID,
		"pdf_export_id":             exportID,
		"payload_hash":              payloadHash,
		"checksum_sha256":           pdfFile.ChecksumSHA256,
		"form_revision_no":          revisionNo,
	}
	rawEventMetadata, _ := json.Marshal(eventMetadata)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_form_events (instance_id, project_id, work_item_id, actor_id, event_type, from_status, to_status, comment, metadata_json)
		VALUES ($1, $2, $3, NULLIF($4,'')::uuid, 'pdf_exported', 'submitted', 'submitted', $5, $6::jsonb)
	`, instanceID, projectID, workItemID, actorID, fmt.Sprintf("Export PDF workflow form: %s", fileName), string(rawEventMetadata)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'internal', $5::jsonb)
	`, workItemID, projectID, actorID, fmt.Sprintf("Workflow form PDF exported: %s", fileName), string(rawEventMetadata)); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "workflow_form_pdf_exported", "workflow_form_pdf_export", exportID, "Export PDF workflow form", eventMetadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetWorkflowFormInstanceDetail(ctx, instanceID)
}

func (r *Repository) GetWorkflowFormSignatureDetail(ctx context.Context, signatureID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT wfs.*, wi.status AS work_item_status
		FROM workflow_form_signatures wfs
		JOIN work_items wi ON wi.id=wfs.work_item_id AND wi.project_id=wfs.project_id
		JOIN projects p ON p.id=wfs.project_id
		WHERE wfs.id=$1 AND wi.status <> 'cancelled' AND p.status <> 'cancelled'
	`, signatureID)
}

func (r *Repository) GetWorkflowFormPDFExportDetail(ctx context.Context, exportID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT wfpe.*, wi.status AS work_item_status
		FROM workflow_form_pdf_exports wfpe
		JOIN work_items wi ON wi.id=wfpe.work_item_id AND wi.project_id=wfpe.project_id
		JOIN projects p ON p.id=wfpe.project_id
		WHERE wfpe.id=$1 AND wi.status <> 'cancelled' AND p.status <> 'cancelled'
	`, exportID)
}

func createMeetingMinutesActionItemsTx(ctx context.Context, tx *sql.Tx, instance Record, answers Record, actorID string) ([]string, error) {
	rawActionItems, ok := answers["action_items"]
	if !ok || workflowFormValueEmpty(Record{"field_type": "structured_list"}, rawActionItems) {
		return []string{}, nil
	}
	actionItems, err := workflowRecordList(rawActionItems)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(instance["project_id"])
	instanceID := cleanID(instance["id"])
	parentWorkItemID := cleanID(instance["work_item_id"])
	meetingTitle := strings.TrimSpace(fmt.Sprint(answers["meeting_title"]))
	createdIDs := make([]string, 0, len(actionItems))
	for index, item := range actionItems {
		title := strings.TrimSpace(fmt.Sprint(item["title"]))
		if title == "" || title == "<nil>" {
			continue
		}
		ownerRole := strings.TrimSpace(fmt.Sprint(item["owner_role"]))
		ownerName := strings.TrimSpace(fmt.Sprint(item["owner_name"]))
		dueDate := strings.TrimSpace(fmt.Sprint(item["due_date"]))
		priority := strings.TrimSpace(fmt.Sprint(item["priority"]))
		if !statusIn(priority, "low", "normal", "high", "urgent") {
			priority = "normal"
		}
		descriptionParts := []string{}
		if meetingTitle != "" && meetingTitle != "<nil>" {
			descriptionParts = append(descriptionParts, "Biên bản họp: "+meetingTitle)
		}
		if ownerName != "" && ownerName != "<nil>" {
			descriptionParts = append(descriptionParts, "Owner: "+ownerName)
		}
		if note := strings.TrimSpace(fmt.Sprint(item["description"])); note != "" && note != "<nil>" {
			descriptionParts = append(descriptionParts, note)
		}
		description := strings.Join(descriptionParts, "\n")
		roleIDs := []string{}
		if ownerRole != "" && ownerRole != "<nil>" {
			roleIDs = append(roleIDs, ownerRole)
		}
		rawRoles, _ := json.Marshal(roleIDs)
		rawMetadata, _ := json.Marshal(Record{
			"source":                    "meeting_minutes",
			"workflow_form_instance_id": instanceID,
			"parent_work_item_id":       parentWorkItemID,
			"action_item_index":         index,
			"action_item":               item,
		})
		workItemID := newID()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_items
				(id, project_id, source_type, source_id, title, description, work_type, status, priority, role_ids, due_at, evidence_requirement_json, created_by, updated_by)
			VALUES
				($1, $2, 'workflow_form_action_item', $3, $4, NULLIF($5,''), 'task', 'ready', $6, $7::jsonb, NULLIF($8,'')::timestamptz, '{}'::jsonb, NULLIF($9,'')::uuid, NULLIF($9,'')::uuid)
		`, workItemID, projectID, instanceID, title, description, priority, string(rawRoles), dueDate, actorID); err != nil {
			return nil, err
		}
		if ownerRole != "" && ownerRole != "<nil>" {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO work_item_assignments (work_item_id, assignment_type, role_name, created_by)
				VALUES ($1, 'role', $2, NULLIF($3,'')::uuid)
			`, workItemID, ownerRole, actorID); err != nil {
				return nil, err
			}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, to_status, comment, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'created', 'ready', 'Action item generated from meeting minutes', $4::jsonb)
		`, workItemID, projectID, actorID, string(rawMetadata)); err != nil {
			return nil, err
		}
		createdIDs = append(createdIDs, workItemID)
	}
	if len(createdIDs) > 0 && parentWorkItemID != "" {
		rawMetadata, _ := json.Marshal(Record{
			"source":                    "meeting_minutes",
			"workflow_form_instance_id": instanceID,
			"generated_work_item_ids":   createdIDs,
		})
		comment := fmt.Sprintf("Meeting minutes generated %d action item work items", len(createdIDs))
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, $5::jsonb)
		`, parentWorkItemID, projectID, actorID, comment, string(rawMetadata)); err != nil {
			return nil, err
		}
	}
	return createdIDs, nil
}

func createDailyReportNextItemsTx(ctx context.Context, tx *sql.Tx, instance Record, answers Record, actorID string) ([]string, error) {
	rawNextItems, ok := answers["next_plan_items"]
	if !ok || workflowFormValueEmpty(Record{"field_type": "structured_list"}, rawNextItems) {
		return []string{}, nil
	}
	nextItems, err := workflowRecordList(rawNextItems)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(instance["project_id"])
	instanceID := cleanID(instance["id"])
	parentWorkItemID := cleanID(instance["work_item_id"])
	reportDate := strings.TrimSpace(fmt.Sprint(answers["report_date"]))
	reportedBy := strings.TrimSpace(fmt.Sprint(answers["reported_by"]))
	reportSource, reportLabel, scopeKey, scopeValue := dailyReportNextItemContext(instance, answers)
	createdIDs := make([]string, 0, len(nextItems))
	for index, item := range nextItems {
		title := strings.TrimSpace(fmt.Sprint(item["title"]))
		if title == "" || title == "<nil>" {
			continue
		}
		ownerRole := strings.TrimSpace(fmt.Sprint(item["owner_role"]))
		ownerName := strings.TrimSpace(fmt.Sprint(item["owner_name"]))
		dueDate := strings.TrimSpace(fmt.Sprint(item["due_date"]))
		priority := strings.TrimSpace(fmt.Sprint(item["priority"]))
		if !statusIn(priority, "low", "normal", "high", "urgent") {
			priority = "normal"
		}
		descriptionParts := []string{}
		if reportDate != "" && reportDate != "<nil>" {
			descriptionParts = append(descriptionParts, reportLabel+": "+reportDate)
		}
		if scopeValue != "" && scopeValue != "<nil>" {
			descriptionParts = append(descriptionParts, "Scope: "+scopeValue)
		}
		if reportedBy != "" && reportedBy != "<nil>" {
			descriptionParts = append(descriptionParts, "Reporter: "+reportedBy)
		}
		if ownerName != "" && ownerName != "<nil>" {
			descriptionParts = append(descriptionParts, "Owner: "+ownerName)
		}
		if note := strings.TrimSpace(fmt.Sprint(item["description"])); note != "" && note != "<nil>" {
			descriptionParts = append(descriptionParts, note)
		}
		description := strings.Join(descriptionParts, "\n")
		roleIDs := []string{}
		if ownerRole != "" && ownerRole != "<nil>" {
			roleIDs = append(roleIDs, ownerRole)
		}
		rawRoles, _ := json.Marshal(roleIDs)
		metadata := Record{
			"source":                    reportSource,
			"workflow_form_instance_id": instanceID,
			"parent_work_item_id":       parentWorkItemID,
			"next_plan_item_index":      index,
			"next_plan_item":            item,
			"report_date":               reportDate,
		}
		if scopeKey != "" {
			metadata[scopeKey] = scopeValue
		}
		rawMetadata, _ := json.Marshal(metadata)
		workItemID := newID()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_items
				(id, project_id, source_type, source_id, title, description, work_type, status, priority, role_ids, due_at, evidence_requirement_json, created_by, updated_by)
			VALUES
				($1, $2, 'workflow_form_action_item', $3, $4, NULLIF($5,''), 'task', 'ready', $6, $7::jsonb, NULLIF($8,'')::timestamptz, '{}'::jsonb, NULLIF($9,'')::uuid, NULLIF($9,'')::uuid)
		`, workItemID, projectID, instanceID, title, description, priority, string(rawRoles), dueDate, actorID); err != nil {
			return nil, err
		}
		if ownerRole != "" && ownerRole != "<nil>" {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO work_item_assignments (work_item_id, assignment_type, role_name, created_by)
				VALUES ($1, 'role', $2, NULLIF($3,'')::uuid)
			`, workItemID, ownerRole, actorID); err != nil {
				return nil, err
			}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, to_status, comment, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'created', 'ready', $4, $5::jsonb)
		`, workItemID, projectID, actorID, "Next plan item generated from "+strings.ToLower(reportLabel), string(rawMetadata)); err != nil {
			return nil, err
		}
		createdIDs = append(createdIDs, workItemID)
	}
	if len(createdIDs) > 0 && parentWorkItemID != "" {
		rawMetadata, _ := json.Marshal(Record{
			"source":                    reportSource,
			"workflow_form_instance_id": instanceID,
			"generated_work_item_ids":   createdIDs,
		})
		comment := fmt.Sprintf("%s generated %d next plan work items", reportLabel, len(createdIDs))
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, $5::jsonb)
		`, parentWorkItemID, projectID, actorID, comment, string(rawMetadata)); err != nil {
			return nil, err
		}
	}
	return createdIDs, nil
}

func dailyReportNextItemContext(instance Record, answers Record) (string, string, string, string) {
	formCode := strings.TrimSpace(fmt.Sprint(instance["form_code"]))
	if formCode == "builtin_construction_daily_report" {
		return "construction_daily_report", "Construction daily report", "construction_scope", strings.TrimSpace(fmt.Sprint(answers["construction_scope"]))
	}
	if rawScope, ok := answers["construction_scope"]; ok {
		return "construction_daily_report", "Construction daily report", "construction_scope", strings.TrimSpace(fmt.Sprint(rawScope))
	}
	return "planning_daily_report", "Planning daily report", "planning_scope", strings.TrimSpace(fmt.Sprint(answers["planning_scope"]))
}

func (r *Repository) GetDocumentAttachment(ctx context.Context, documentID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT *
		FROM documents
		WHERE id=$1 AND status <> 'deleted'
	`, documentID)
}

func (r *Repository) RequestWorkItemApproval(ctx context.Context, item Record, req WorkItemActionRequest, actorID, actorName string) (Record, error) {
	workItemID := fmt.Sprint(item["id"])
	projectID := fmt.Sprint(item["project_id"])
	fromStatus := fmt.Sprint(item["status"])
	comment := workItemActionComment(req)
	if comment == "" {
		comment = "Yêu cầu duyệt work item"
	}
	requiredRoles := normalizedRoleList(req.RequiredRoles)
	if len(requiredRoles) == 0 {
		requiredRoles = normalizedRoleList(stringSlice(item["role_ids"]))
	}
	if len(requiredRoles) == 0 {
		requiredRoles = []string{"PM"}
	}
	rawRequiredRoles, _ := json.Marshal(requiredRoles)
	rawMetadata, _ := json.Marshal(Record{"action": "approval_requested", "required_roles": requiredRoles})

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	approvalID := cleanID(item["approval_request_id"])
	var approval Record
	if approvalID == "" {
		rows, err := queryTx(ctx, tx, `
			INSERT INTO approval_requests (target_type, target_id, project_id, requested_by, status, reason, required_roles)
			VALUES ('work_item', $1, $2, NULLIF($3,'')::uuid, 'pending', $4, $5::jsonb)
			RETURNING *
		`, workItemID, projectID, actorID, comment, string(rawRequiredRoles))
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		approval = rows[0]
		approvalID = fmt.Sprint(approval["id"])
	} else {
		rows, err := queryTx(ctx, tx, `
			UPDATE approval_requests
			SET status='pending', reason=$2, required_roles=$3::jsonb, resolved_at=NULL
			WHERE id=$1 AND project_id=$4
			RETURNING *
		`, approvalID, comment, string(rawRequiredRoles), projectID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		approval = rows[0]
	}

	updated, err := queryTx(ctx, tx, `
		UPDATE work_items
		SET status='waiting_approval',
			approval_request_id=$2,
			updated_by=NULLIF($3,'')::uuid,
			updated_at=now()
		WHERE id=$1 AND status=$4
		RETURNING id
	`, workItemID, approvalID, actorID, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái work item đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	if err := applyRunStepStatusTx(ctx, tx, item, "waiting_approval", actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, from_status, to_status, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'approval_requested', $4, $5, 'waiting_approval', $6::jsonb)
	`, workItemID, projectID, actorID, comment, fromStatus, string(rawMetadata)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json)
		VALUES ($1, NULLIF($2,'')::uuid, $3, 'work_item_approval_requested', 'work_item', $4, $5, $6::jsonb)
	`, projectID, actorID, actorName, workItemID, workItemAuditDescription("approval_requested", item, comment), string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	out, err := r.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	out["approval_request"] = approval
	return out, nil
}

func (r *Repository) ApplyWorkItemTransition(ctx context.Context, item Record, toStatus, activityType string, req WorkItemActionRequest, actorID, actorName string) (Record, error) {
	workItemID := fmt.Sprint(item["id"])
	projectID := fmt.Sprint(item["project_id"])
	fromStatus := fmt.Sprint(item["status"])
	comment := workItemActionComment(req)
	payload := req.CompletionPayload
	if payload == nil {
		payload = Record{}
	}
	rawPayload, _ := json.Marshal(payload)
	rawMetadata, _ := json.Marshal(Record{"action": activityType})

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	updated, err := queryTx(ctx, tx, `
			UPDATE work_items
			SET status=$2,
				started_at=CASE WHEN $3 THEN COALESCE(started_at, now()) ELSE started_at END,
				completed_at=CASE WHEN $4 THEN now() ELSE completed_at END,
				blocked_at=CASE WHEN $5 THEN now() WHEN $6 THEN NULL ELSE blocked_at END,
			blocked_reason=CASE WHEN $5 THEN $7 WHEN $6 THEN NULL ELSE blocked_reason END,
				completion_payload_json=CASE WHEN $4 THEN $8::jsonb ELSE completion_payload_json END,
				updated_by=NULLIF($9,'')::uuid,
				updated_at=now()
			WHERE id=$1 AND status=$10
			RETURNING id
		`, workItemID, toStatus, activityType == "started", activityType == "completed", activityType == "blocked", activityType == "blocker_resolved", strings.TrimSpace(req.Reason), string(rawPayload), actorID, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái work item đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}

	if err := applyRunStepStatusTx(ctx, tx, item, toStatus, actorID); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, from_status, to_status, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, $4, $5, $6, $7, $8::jsonb)
	`, workItemID, projectID, actorID, activityType, comment, fromStatus, toStatus, string(rawMetadata)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, 'work_item', $5, $6, $7::jsonb)
	`, projectID, actorID, actorName, "work_item_"+activityType, workItemID, workItemAuditDescription(activityType, item, comment), string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetWorkItemDetail(ctx, workItemID)
}

func (r *Repository) ApplyWorkItemDecision(ctx context.Context, item Record, action, toStatus, activityType string, req WorkItemActionRequest, actorID, actorName string) (Record, error) {
	workItemID := fmt.Sprint(item["id"])
	projectID := fmt.Sprint(item["project_id"])
	fromStatus := fmt.Sprint(item["status"])
	comment := workItemActionComment(req)
	payload := req.CompletionPayload
	if payload == nil {
		payload = Record{}
	}
	rawPayload, _ := json.Marshal(payload)
	rawMetadata, _ := json.Marshal(Record{"action": activityType})

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	approvalID := cleanID(item["approval_request_id"])
	if approvalID != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO approval_actions (approval_request_id, user_id, action, comment)
			VALUES ($1, NULLIF($2,'')::uuid, $3, $4)
		`, approvalID, actorID, action, comment); err != nil {
			return nil, err
		}
		approvalStatus := "pending"
		switch action {
		case "approve":
			approvalStatus = "approved"
		case "reject":
			approvalStatus = "rejected"
		case "request-change":
			approvalStatus = "changes_requested"
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE approval_requests
			SET status=$2, resolved_at=now()
			WHERE id=$1 AND project_id=$3
		`, approvalID, approvalStatus, projectID); err != nil {
			return nil, err
		}
	}

	updated, err := queryTx(ctx, tx, `
		UPDATE work_items
		SET status=$2,
			completed_at=CASE WHEN $3 THEN now() ELSE completed_at END,
			completion_payload_json=CASE WHEN $3 THEN $4::jsonb ELSE completion_payload_json END,
			updated_by=NULLIF($5,'')::uuid,
			updated_at=now()
		WHERE id=$1 AND status=$6
		RETURNING id
	`, workItemID, toStatus, toStatus == "completed", string(rawPayload), actorID, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái work item đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	if err := applyRunStepStatusTx(ctx, tx, item, toStatus, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, from_status, to_status, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, $4, $5, $6, $7, $8::jsonb)
	`, workItemID, projectID, actorID, activityType, comment, fromStatus, toStatus, string(rawMetadata)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, 'work_item', $5, $6, $7::jsonb)
	`, projectID, actorID, actorName, "work_item_"+activityType, workItemID, workItemAuditDescription(activityType, item, comment), string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetWorkItemDetail(ctx, workItemID)
}

func (r *Repository) AddWorkItemEvidence(ctx context.Context, item Record, req WorkItemEvidenceRequest, actorID, actorName string) (Record, error) {
	workItemID := fmt.Sprint(item["id"])
	projectID := fmt.Sprint(item["project_id"])
	payload := req.Payload
	if payload == nil {
		payload = Record{}
	}
	rawPayload, _ := json.Marshal(payload)
	rawMetadata, _ := json.Marshal(Record{"evidence_type": req.EvidenceType})
	comment := req.Caption
	if comment == "" {
		comment = req.Description
	}
	if comment == "" {
		comment = req.Name
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO evidence_records
			(work_item_id, project_id, target_type, target_id, evidence_type, document_type, name, document_id, title, description, file_url, mime_type, file_size, storage_provider, storage_key, checksum_sha256, caption, revision_no, visibility, payload_json, status, uploaded_by, submitted_by)
		VALUES
			($1, $2, 'work_item', $1, $3, NULLIF($4,''), $5, NULLIF($6,'')::uuid, NULLIF($7,''), NULLIF($8,''), NULLIF($9,''), NULLIF($10,''), NULLIF($11,0), $12, NULLIF($13,''), NULLIF($14,''), NULLIF($15,''), NULLIF($16,''), $17, $18::jsonb, 'submitted', NULLIF($19,'')::uuid, NULLIF($19,'')::uuid)
		RETURNING *
	`, workItemID, projectID, req.EvidenceType, req.DocumentType, req.Name, req.DocumentID, req.Title, req.Description, req.FileURL, req.MIMEType, req.FileSize, req.StorageProvider, req.StorageKey, req.ChecksumSHA256, req.Caption, req.RevisionNo, req.Visibility, string(rawPayload), actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	evidence := rows[0]
	if _, err := tx.ExecContext(ctx, `UPDATE work_items SET updated_by=NULLIF($2,'')::uuid, updated_at=now() WHERE id=$1`, workItemID, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'evidence_added', $4, $5::jsonb)
	`, workItemID, projectID, actorID, comment, string(rawMetadata)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json)
		VALUES ($1, NULLIF($2,'')::uuid, $3, 'work_item_evidence_added', 'work_item', $4, $5, $6::jsonb)
	`, projectID, actorID, actorName, workItemID, fmt.Sprintf("Thêm evidence %s cho work item %s", req.Name, item["title"]), string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetEvidenceDetail(ctx, fmt.Sprint(evidence["id"]))
}

func (r *Repository) AddWorkItemActivity(ctx context.Context, item Record, req WorkItemActivityRequest, actorID, actorName string) (Record, error) {
	workItemID := fmt.Sprint(item["id"])
	projectID := fmt.Sprint(item["project_id"])
	rawMetadata, _ := json.Marshal(req.Metadata)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, $4, $5, $6, $7::jsonb)
		RETURNING *
	`, workItemID, projectID, actorID, req.ActivityType, req.Comment, req.Visibility, string(rawMetadata))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := r.insertActivityAttentionTx(ctx, tx, rows[0], req, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE work_items SET updated_by=NULLIF($2,'')::uuid, updated_at=now() WHERE id=$1`, workItemID, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, 'work_item', $5, $6, $7::jsonb)
	`, projectID, actorID, actorName, workItemActivityEvent(req.ActivityType), workItemID, workItemActivityDescription(req.ActivityType, item, req.Comment), string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) AddProjectActivity(ctx context.Context, projectID string, req WorkItemActivityRequest, actorID, actorName string) (Record, error) {
	rawMetadata, _ := json.Marshal(req.Metadata)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES (NULLIF($2,'')::uuid, $1, $3, $4, NULLIF($5,'')::uuid, $6, $7, $8, $9::jsonb)
		RETURNING *
	`, projectID, req.WorkItemID, req.ScopeType, req.ScopeID, actorID, req.ActivityType, req.Comment, req.Visibility, string(rawMetadata))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := r.insertActivityAttentionTx(ctx, tx, rows[0], req, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7, $8::jsonb)
	`, projectID, actorID, actorName, "project_activity_"+req.ActivityType, req.ScopeType, req.ScopeID, projectActivityDescription(req.ActivityType, req.Comment), string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) insertActivityAttentionTx(ctx context.Context, tx *sql.Tx, activity Record, req WorkItemActivityRequest, actorID string) error {
	activityID := cleanID(activity["id"])
	projectID := cleanID(activity["project_id"])
	if activityID == "" || projectID == "" || len(req.Mentions) == 0 {
		return nil
	}
	for _, mention := range req.Mentions {
		switch mention.Type {
		case "user":
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO activity_mentions (activity_id, project_id, mention_type, mention_key, recipient_user_id, recipient_staff_id)
				SELECT $1, $2, 'user', $3, u.id, s.id
				FROM users u
				LEFT JOIN staff s ON s.user_id=u.id AND s.status='active'
				WHERE u.id=$3::uuid
				  AND u.status='active'
				  AND EXISTS (
					SELECT 1
					FROM project_members pm
					WHERE pm.project_id=$2
					  AND pm.active
					  AND pm.staff_id=s.id
				  )
				ON CONFLICT DO NOTHING
			`, activityID, projectID, mention.ID); err != nil {
				return err
			}
		case "staff":
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO activity_mentions (activity_id, project_id, mention_type, mention_key, recipient_user_id, recipient_staff_id)
				SELECT $1, $2, 'staff', $3, s.user_id, s.id
				FROM staff s
				JOIN project_members pm ON pm.staff_id=s.id AND pm.project_id=$2 AND pm.active
				WHERE s.id=$3::uuid
				  AND s.status='active'
				  AND s.user_id IS NOT NULL
				ON CONFLICT DO NOTHING
			`, activityID, projectID, mention.ID); err != nil {
				return err
			}
		case "role":
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO activity_mentions (activity_id, project_id, mention_type, mention_key, recipient_user_id, recipient_staff_id)
				SELECT DISTINCT $1, $2, 'role', $3, s.user_id, s.id
				FROM project_members pm
				JOIN staff s ON s.id=pm.staff_id
				WHERE pm.project_id=$2
				  AND pm.active
				  AND s.status='active'
				  AND s.user_id IS NOT NULL
				  AND (
					pm.role_ids ? $3
					OR (NULLIF($4, '') IS NOT NULL AND pm.role_ids ? $4)
				  )
				ON CONFLICT DO NOTHING
			`, activityID, projectID, mention.ID, mention.Label); err != nil {
				return err
			}
		}
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO activity_read_states (activity_id, project_id, user_id, read_at)
		SELECT DISTINCT activity_id, project_id, recipient_user_id,
			CASE WHEN recipient_user_id=NULLIF($2,'')::uuid THEN now() ELSE NULL END
		FROM activity_mentions
		WHERE activity_id=$1
		ON CONFLICT DO NOTHING
	`, activityID, actorID)
	return err
}

func (r *Repository) ProjectActivityTargetExists(ctx context.Context, projectID, scopeType, scopeID string) (bool, error) {
	table, statusColumn, ok := activityTargetTable(scopeType)
	if !ok {
		return false, nil
	}
	statusSQL := ""
	if statusColumn != "" {
		statusSQL = fmt.Sprintf(" AND COALESCE(%s, '') <> 'cancelled'", statusColumn)
	}
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT COUNT(*) AS total
		FROM %s
		WHERE id=$1 AND project_id=$2%s
	`, table, statusSQL), scopeID, projectID)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && number(rows[0]["total"]) > 0, nil
}

func activityTargetTable(scopeType string) (string, string, bool) {
	switch scopeType {
	case "boq_item":
		return "boq_items", "status", true
	case "procurement_request":
		return "procurement_requests", "status", true
	case "stock_movement":
		return "stock_movements", "", true
	case "acceptance":
		return "acceptance_records", "status", true
	case "rework":
		return "rework_requests", "status", true
	case "handover":
		return "handover_records", "status", true
	case "payable":
		return "payables", "status", true
	case "receivable":
		return "receivables", "status", true
	case "payment":
		return "payments", "status", true
	default:
		return "", "", false
	}
}

func workItemActivityEvent(activityType string) string {
	if activityType == "comment" {
		return "work_item_comment"
	}
	return "work_item_activity_" + activityType
}

func workItemActivityDescription(activityType string, item Record, comment string) string {
	title := fmt.Sprint(item["title"])
	if activityType == "comment" {
		return fmt.Sprintf("Ghi chú work item %s", title)
	}
	return fmt.Sprintf("Tạo activity %s cho work item %s: %s", activityType, title, comment)
}

func projectActivityDescription(activityType string, comment string) string {
	if activityType == "comment" {
		return "Ghi chú công trình"
	}
	return fmt.Sprintf("Tạo activity %s cho công trình: %s", activityType, comment)
}

func (r *Repository) EvidenceAction(ctx context.Context, evidence Record, item Record, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	evidenceID := fmt.Sprint(evidence["id"])
	workItemID := fmt.Sprint(item["id"])
	projectID := fmt.Sprint(item["project_id"])
	status := "accepted"
	activityType := "evidence_accepted"
	if action == "reject" {
		status = "rejected"
		activityType = "evidence_rejected"
	}
	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		comment = strings.TrimSpace(req.Reason)
	}
	rawMetadata, _ := json.Marshal(Record{"action": action, "evidence_id": evidenceID})
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	updated, err := queryTx(ctx, tx, `
		UPDATE evidence_records
		SET status=$2,
			reviewed_by=NULLIF($3,'')::uuid,
			reviewed_at=now(),
			updated_at=now()
		WHERE id=$1 AND status IN ('draft','submitted')
		RETURNING id
	`, evidenceID, status, actorID)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Evidence đã được review hoặc không còn hợp lệ", Record{"status": evidence["status"]})
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, $4, $5, $6::jsonb)
	`, workItemID, projectID, actorID, activityType, comment, string(rawMetadata)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, 'evidence_record', $5, $6, $7::jsonb)
	`, projectID, actorID, actorName, "work_item_"+activityType, evidenceID, fmt.Sprintf("%s evidence %s", action, evidence["name"]), string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetEvidenceDetail(ctx, evidenceID)
}

func applyRunStepStatusTx(ctx context.Context, tx *sql.Tx, item Record, toStatus, actorID string) error {
	runStepID := cleanID(item["run_step_id"])
	if runStepID == "" {
		return nil
	}
	stepStatus := toStatus
	if toStatus == "ready" {
		stepStatus = "pending"
	}
	if toStatus == "completed" {
		_, err := tx.ExecContext(ctx, `
			UPDATE pipeline_run_steps
			SET status='completed', submitted_by=NULLIF($2,'')::uuid, submitted_at=now()
			WHERE id=$1
		`, runStepID, actorID)
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE pipeline_run_steps SET status=$2 WHERE id=$1`, runStepID, stepStatus)
	return err
}

func workItemAuditDescription(activityType string, item Record, comment string) string {
	title := fmt.Sprint(item["title"])
	switch activityType {
	case "started":
		return fmt.Sprintf("Bắt đầu work item %s", title)
	case "completed":
		return fmt.Sprintf("Hoàn tất work item %s", title)
	case "blocked":
		return fmt.Sprintf("Block work item %s: %s", title, comment)
	case "blocker_resolved":
		return fmt.Sprintf("Resolve blocker work item %s: %s", title, comment)
	case "review_requested":
		return fmt.Sprintf("Yêu cầu review work item %s", title)
	case "approval_requested":
		return fmt.Sprintf("Yêu cầu duyệt work item %s", title)
	case "approved":
		return fmt.Sprintf("Duyệt work item %s", title)
	case "rejected":
		return fmt.Sprintf("Reject work item %s: %s", title, comment)
	case "changes_requested":
		return fmt.Sprintf("Yêu cầu chỉnh sửa work item %s: %s", title, comment)
	default:
		return fmt.Sprintf("Cập nhật work item %s", title)
	}
}

func cleanID(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	return text
}

func (r *Repository) ProjectOperations(ctx context.Context, projectID, userID string) (Record, error) {
	project, err := r.GetProjectDetail(ctx, projectID)
	if err != nil {
		return nil, err
	}
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	permissions, err := r.UserProjectPermissions(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}
	if !profile.IsManager {
		delete(project, "members")
	}
	workItems, err := r.ListProjectWorkItems(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	summary, blockers, activeWork := summarizeWorkItems(workItems)

	latestActivity, err := r.ProjectOperationsActivity(ctx, projectID, profile)
	if err != nil {
		return nil, err
	}
	approvals, err := r.ProjectOperationsApprovals(ctx, projectID, profile)
	if err != nil {
		return nil, err
	}
	latestEvidence, err := r.ProjectOperationsEvidence(ctx, projectID, profile)
	if err != nil {
		return nil, err
	}
	auditRows, err := r.ProjectOperationsAudit(ctx, projectID, profile)
	if err != nil {
		return nil, err
	}
	domainCounts := Record{}
	if profile.IsManager {
		domainCounts, err = r.ProjectOperationsDomainCounts(ctx, projectID)
		if err != nil {
			return nil, err
		}
	}
	run, err := r.GetPipelineRun(ctx, projectID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if run != nil && !profile.IsManager {
		run = scopePipelineRunToWorkItems(run, workItems)
	}
	visibility := projectOperationsVisibility(profile, permissions)
	healthCounts := projectOperationsHealthCounts(summary, domainCounts)
	return Record{
		"project":             project,
		"project_summary":     projectOperationsProjectSummary(project),
		"visibility":          visibility,
		"health_counts":       healthCounts,
		"workflow_timeline":   workflowTimeline(run),
		"active_work":         activeWork,
		"blockers":            blockers,
		"approvals":           approvals,
		"latest_evidence":     latestEvidence,
		"activity_summary":    activitySummary(latestActivity),
		"latest_activity":     latestActivity,
		"audit_summary":       auditSummary(auditRows),
		"department_surfaces": departmentSurfaces(workItems),
		"pipeline_run":        run,
		"summary":             summary,
		"work_items":          workItems,
	}, nil
}

func (r *Repository) ProjectPipelineRun(ctx context.Context, projectID, userID string) (Record, error) {
	run, err := r.GetPipelineRun(ctx, projectID)
	if err != nil {
		return nil, err
	}
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if profile.IsManager {
		return run, nil
	}
	workItems, err := r.ListProjectWorkItems(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	return scopePipelineRunToWorkItems(run, workItems), nil
}

func (r *Repository) ProjectOperationsActivity(ctx context.Context, projectID string, profile workItemAccessProfile) ([]Record, error) {
	return r.ProjectActivityThread(ctx, projectID, profile, ProjectActivityFilter{Limit: 50})
}

func (r *Repository) ActivityVisibleToUser(ctx context.Context, activityID string, profile workItemAccessProfile) (bool, error) {
	row, err := r.QueryOne(ctx, `
		SELECT
			wia.project_id,
			wia.scope_type,
			wia.scope_id,
			wia.work_item_id,
			wia.visibility
		FROM work_item_activities wia
		JOIN projects p ON p.id=wia.project_id
		WHERE wia.id=$1
		  AND p.status <> 'cancelled'
	`, activityID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if profileIsClient(profile) && !profile.IsManager && fmt.Sprint(row["visibility"]) != "client" {
		return false, nil
	}
	projectID := cleanID(row["project_id"])
	allowed, err := r.CanViewProjectOperations(ctx, projectID, profile.UserID)
	if err != nil {
		return false, err
	}
	if !allowed {
		return false, nil
	}
	workItemID := cleanID(row["work_item_id"])
	if workItemID == "" && fmt.Sprint(row["scope_type"]) == "work_item" {
		workItemID = cleanID(row["scope_id"])
	}
	if workItemID == "" {
		return true, nil
	}
	return r.CanViewWorkItem(ctx, workItemID, profile.UserID)
}

func (r *Repository) MarkActivityRead(ctx context.Context, activityID, userID string) (Record, error) {
	return r.QueryOne(ctx, `
		INSERT INTO activity_read_states (activity_id, project_id, user_id, read_at)
		SELECT id, project_id, $2::uuid, now()
		FROM work_item_activities
		WHERE id=$1
		ON CONFLICT (activity_id, user_id)
		DO UPDATE SET read_at=EXCLUDED.read_at
		RETURNING *
	`, activityID, userID)
}

func (r *Repository) ProjectActivityThread(ctx context.Context, projectID string, profile workItemAccessProfile, filter ProjectActivityFilter) ([]Record, error) {
	args := append([]any{projectID, profile.UserID}, workItemScopeArgs(profile, profile.IsManager, true)...)
	visibilitySQL := "TRUE"
	if profileIsClient(profile) && !profile.IsManager {
		visibilitySQL = "wia.visibility='client'"
	}
	conditions := []string{
		"wia.project_id=$1",
		"(wi.id IS NULL OR wi.status <> 'cancelled')",
		fmt.Sprintf("(%s)", visibilitySQL),
		fmt.Sprintf("(wia.scope_type <> 'work_item' OR (wi.id IS NOT NULL AND (%s)))", workItemScopeSQL("wi", 3)),
	}
	if filter.ScopeType != "" {
		args = append(args, filter.ScopeType)
		conditions = append(conditions, fmt.Sprintf("wia.scope_type=$%d", len(args)))
	}
	if filter.ScopeID != "" {
		args = append(args, filter.ScopeID)
		conditions = append(conditions, fmt.Sprintf("wia.scope_id=$%d::uuid", len(args)))
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 80
	}
	args = append(args, filter.Limit)
	limitPlaceholder := len(args)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT
			wia.*,
			wi.title AS work_item_title,
			u.full_name AS actor_name,
			COALESCE(mention_state.mentions, '[]'::jsonb) AS mentions,
			COALESCE(mention_state.mentioned_me, false) AS mentioned_me,
			%s,
			ars.read_at,
			(ars.user_id IS NOT NULL AND ars.read_at IS NULL) AS is_unread,
			COALESCE(
				CASE
					WHEN wia.scope_type='project' THEN p.code || ' · ' || p.name
					WHEN wia.scope_type='work_item' THEN wi.title
					WHEN wia.scope_type='boq_item' THEN bi.work_item_name
					WHEN wia.scope_type='procurement_request' THEN pr.request_code || ' · ' || pr.title
					WHEN wia.scope_type='stock_movement' THEN sm.movement_code
					WHEN wia.scope_type='acceptance' THEN ar.title
					WHEN wia.scope_type='rework' THEN rw.title
					WHEN wia.scope_type='handover' THEN 'Bàn giao · ' || hr.status
					WHEN wia.scope_type='payable' THEN pay.payable_code || ' · ' || pay.vendor_name
					WHEN wia.scope_type='receivable' THEN rec.receivable_code || ' · ' || rec.client_name
					WHEN wia.scope_type='payment' THEN pm.payment_code || ' · ' || pm.payment_type
					ELSE wi.title
				END,
				'Project'
			) AS target_label
		FROM work_item_activities wia
		JOIN projects p ON p.id=wia.project_id
		LEFT JOIN work_items wi ON wi.id=wia.work_item_id
		LEFT JOIN users u ON u.id=wia.actor_id
		%s
		LEFT JOIN activity_read_states ars ON ars.activity_id=wia.id AND ars.user_id=$2
		LEFT JOIN LATERAL (
			SELECT
				jsonb_agg(jsonb_build_object(
					'type', am.mention_type,
					'key', am.mention_key,
					'user_id', am.recipient_user_id,
					'staff_id', am.recipient_staff_id
				) ORDER BY am.created_at) AS mentions,
				bool_or(am.recipient_user_id=$2) AS mentioned_me
			FROM activity_mentions am
			WHERE am.activity_id=wia.id
		) mention_state ON true
		LEFT JOIN boq_items bi ON bi.id=wia.scope_id AND wia.scope_type='boq_item'
		LEFT JOIN procurement_requests pr ON pr.id=wia.scope_id AND wia.scope_type='procurement_request'
		LEFT JOIN stock_movements sm ON sm.id=wia.scope_id AND wia.scope_type='stock_movement'
		LEFT JOIN acceptance_records ar ON ar.id=wia.scope_id AND wia.scope_type='acceptance'
		LEFT JOIN rework_requests rw ON rw.id=wia.scope_id AND wia.scope_type='rework'
		LEFT JOIN handover_records hr ON hr.id=wia.scope_id AND wia.scope_type='handover'
		LEFT JOIN payables pay ON pay.id=wia.scope_id AND wia.scope_type='payable'
		LEFT JOIN receivables rec ON rec.id=wia.scope_id AND wia.scope_type='receivable'
		LEFT JOIN payments pm ON pm.id=wia.scope_id AND wia.scope_type='payment'
		WHERE %s
		ORDER BY wia.created_at DESC
		LIMIT $%d
	`, activityAttachmentSelectSQL(), activityAttachmentJoinSQL(), strings.Join(conditions, " AND "), limitPlaceholder), args...)
}

func profileIsClient(profile workItemAccessProfile) bool {
	for _, label := range profile.RoleLabels {
		normalized := strings.ToLower(strings.TrimSpace(label))
		if normalized == "client" || normalized == "chủ đầu tư" {
			return true
		}
	}
	return false
}

func (r *Repository) ProjectOperationsApprovals(ctx context.Context, projectID string, profile workItemAccessProfile) ([]Record, error) {
	if profile.IsManager {
		return r.Query(ctx, `
			SELECT ar.*, u.full_name AS requested_by_name
			FROM approval_requests ar
			LEFT JOIN users u ON u.id=ar.requested_by
			WHERE ar.project_id=$1
			ORDER BY
				CASE ar.status WHEN 'pending' THEN 1 WHEN 'changes_requested' THEN 2 ELSE 3 END,
				ar.created_at DESC
			LIMIT 50
		`, projectID)
	}
	args := append([]any{projectID, profile.UserID}, workItemScopeArgs(profile, false, true)...)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT ar.*, u.full_name AS requested_by_name
		FROM approval_requests ar
		LEFT JOIN users u ON u.id=ar.requested_by
		LEFT JOIN work_items wi ON wi.approval_request_id=ar.id AND wi.status <> 'cancelled'
		WHERE ar.project_id=$1
		  AND (
			ar.requested_by=NULLIF($2,'')::uuid
			OR (wi.id IS NOT NULL AND (%s))
		  )
		ORDER BY
			CASE ar.status WHEN 'pending' THEN 1 WHEN 'changes_requested' THEN 2 ELSE 3 END,
			ar.created_at DESC
		LIMIT 50
	`, workItemScopeSQL("wi", 3)), args...)
}

func (r *Repository) ProjectOperationsEvidence(ctx context.Context, projectID string, profile workItemAccessProfile) ([]Record, error) {
	args := append([]any{projectID}, workItemScopeArgs(profile, profile.IsManager, true)...)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT er.*, wi.title AS work_item_title, u.full_name AS submitted_by_name, d.name AS document_name
		FROM evidence_records er
		JOIN work_items wi ON wi.id=er.work_item_id
		LEFT JOIN users u ON u.id=er.submitted_by
		LEFT JOIN documents d ON d.id=er.document_id
		WHERE er.project_id=$1
		  AND wi.status <> 'cancelled'
		  AND er.status <> 'rejected'
		  AND er.visibility IN ('internal','project')
		  AND (%s)
		ORDER BY er.created_at DESC
		LIMIT 50
	`, workItemScopeSQL("wi", 2)), args...)
}

func (r *Repository) ProjectOperationsAudit(ctx context.Context, projectID string, profile workItemAccessProfile) ([]Record, error) {
	if profile.IsManager {
		return r.Query(ctx, `
			SELECT al.*
			FROM audit_logs al
			WHERE al.project_id=$1
			ORDER BY al.created_at DESC
			LIMIT 50
		`, projectID)
	}
	args := append([]any{projectID}, workItemScopeArgs(profile, false, true)...)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT al.*
		FROM audit_logs al
		JOIN work_items wi ON wi.id=al.target_id AND al.target_type='work_item'
		WHERE al.project_id=$1
		  AND wi.status <> 'cancelled'
		  AND (%s)
		ORDER BY al.created_at DESC
		LIMIT 50
	`, workItemScopeSQL("wi", 2)), args...)
}

func (r *Repository) ProjectOperationsDomainCounts(ctx context.Context, projectID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			(SELECT COUNT(*) FROM issues WHERE project_id=$1 AND status <> 'closed') AS open_issues,
			(SELECT COUNT(*) FROM acceptance_records WHERE project_id=$1 AND result='fail' AND status <> 'cancelled') AS failed_acceptances,
			(SELECT COUNT(*) FROM procurement_requests WHERE project_id=$1 AND status IN ('draft','submitted')) AS pending_procurement,
			(SELECT COUNT(*) FROM site_material_balances WHERE project_id=$1 AND planned_quantity > issued_quantity) AS material_risks
	`, projectID)
}

func summarizeWorkItems(workItems []Record) (Record, []Record, []Record) {
	summary := Record{
		"total_work_items":      len(workItems),
		"active_work_items":     0,
		"blocked_work_items":    0,
		"completed_work_items":  0,
		"overdue_work_items":    0,
		"pending_approvals":     0,
		"waiting_evidence":      0,
		"visible_scope_applied": true,
	}
	statusCounts := Record{}
	departmentCounts := Record{}
	blockers := []Record{}
	activeWork := []Record{}
	for _, item := range workItems {
		status := fmt.Sprint(item["status"])
		statusCounts[status] = number(statusCounts[status]) + 1
		if status != "completed" && status != "cancelled" {
			summary["active_work_items"] = number(summary["active_work_items"]) + 1
			if len(activeWork) < 20 {
				activeWork = append(activeWork, item)
			}
		}
		if isOverdue(item) {
			summary["overdue_work_items"] = number(summary["overdue_work_items"]) + 1
		}
		if status == "blocked" {
			summary["blocked_work_items"] = number(summary["blocked_work_items"]) + 1
			blockers = append(blockers, item)
		}
		if status == "completed" {
			summary["completed_work_items"] = number(summary["completed_work_items"]) + 1
		}
		if status == "waiting_approval" {
			summary["pending_approvals"] = number(summary["pending_approvals"]) + 1
		}
		if status == "waiting_evidence" {
			summary["waiting_evidence"] = number(summary["waiting_evidence"]) + 1
		}
		department := cleanText(item["department"])
		if department != "" {
			departmentCounts[department] = number(departmentCounts[department]) + 1
		}
	}
	summary["status_counts"] = statusCounts
	summary["department_counts"] = departmentCounts
	return summary, blockers, activeWork
}

func isOverdue(item Record) bool {
	status := fmt.Sprint(item["status"])
	if status == "completed" || status == "cancelled" {
		return false
	}
	dueAt := cleanText(item["due_at"])
	if dueAt == "" {
		return false
	}
	parsed, err := time.Parse(time.RFC3339, dueAt)
	if err != nil {
		return false
	}
	return parsed.Before(time.Now())
}

func projectOperationsVisibility(profile workItemAccessProfile, permissions []string) Record {
	scope := "project_member"
	if profile.IsAdmin {
		scope = "admin"
	} else if profile.IsDirector {
		scope = "director"
	} else if profile.Department != "" {
		scope = "project_member"
	}
	hiddenSections := []string{}
	if !profile.IsManager {
		hiddenSections = append(hiddenSections, "members", "domain_counts", "project_audit", "unscoped_pipeline_steps")
	}
	return Record{
		"scope":                scope,
		"scoped":               !profile.IsManager,
		"can_view_all_project": profile.IsManager,
		"department":           profile.Department,
		"permissions":          permissions,
		"role_labels":          profile.RoleLabels,
		"hidden_sections":      hiddenSections,
	}
}

func projectOperationsHealthCounts(summary Record, domainCounts Record) Record {
	return Record{
		"active":              summary["active_work_items"],
		"blocked":             summary["blocked_work_items"],
		"completed":           summary["completed_work_items"],
		"overdue":             summary["overdue_work_items"],
		"waiting_approval":    summary["pending_approvals"],
		"waiting_evidence":    summary["waiting_evidence"],
		"open_issues":         number(domainCounts["open_issues"]),
		"failed_acceptances":  number(domainCounts["failed_acceptances"]),
		"pending_procurement": number(domainCounts["pending_procurement"]),
		"material_risks":      number(domainCounts["material_risks"]),
	}
}

func projectOperationsProjectSummary(project Record) Record {
	return Record{
		"id":                 project["id"],
		"code":               project["code"],
		"name":               project["name"],
		"client_name":        project["client_name"],
		"pm_name":            project["pm_name"],
		"status":             project["status"],
		"progress_percent":   project["progress_percent"],
		"estimated_budget":   project["estimated_budget"],
		"contract_amount":    project["contract_amount"],
		"planned_start_date": project["planned_start_date"],
		"planned_end_date":   project["planned_end_date"],
	}
}

func workflowTimeline(run Record) []Record {
	if run == nil {
		return []Record{}
	}
	nodes, _ := run["nodes"].([]Record)
	out := make([]Record, 0, len(nodes))
	for _, node := range nodes {
		steps, _ := node["steps"].([]Record)
		completedSteps := 0
		for _, step := range steps {
			if fmt.Sprint(step["status"]) == "completed" {
				completedSteps++
			}
		}
		out = append(out, Record{
			"id":               node["id"],
			"name":             node["name"],
			"type":             node["type"],
			"order_index":      node["order_index"],
			"status":           node["status"],
			"step_count":       len(steps),
			"completed_steps":  completedSteps,
			"progress_percent": percent(completedSteps, len(steps)),
			"started_at":       node["started_at"],
			"completed_at":     node["completed_at"],
		})
	}
	return out
}

func activitySummary(rows []Record) Record {
	counts := Record{}
	for _, row := range rows {
		activityType := fmt.Sprint(row["activity_type"])
		counts[activityType] = number(counts[activityType]) + 1
	}
	return Record{"total_recent": len(rows), "type_counts": counts}
}

func auditSummary(rows []Record) Record {
	counts := Record{}
	for _, row := range rows {
		eventType := fmt.Sprint(row["event_type"])
		counts[eventType] = number(counts[eventType]) + 1
	}
	return Record{"total_recent": len(rows), "event_counts": counts, "recent": rows}
}

func departmentSurfaces(workItems []Record) []Record {
	index := map[string]Record{}
	order := []string{}
	for _, item := range workItems {
		department := cleanText(item["department"])
		if department == "" {
			department = "unassigned"
		}
		surface, ok := index[department]
		if !ok {
			surface = Record{
				"department":       department,
				"total":            0,
				"active":           0,
				"blocked":          0,
				"waiting_approval": 0,
				"work_items":       []Record{},
			}
			index[department] = surface
			order = append(order, department)
		}
		surface["total"] = number(surface["total"]) + 1
		status := fmt.Sprint(item["status"])
		if status != "completed" && status != "cancelled" {
			surface["active"] = number(surface["active"]) + 1
		}
		if status == "blocked" {
			surface["blocked"] = number(surface["blocked"]) + 1
		}
		if status == "waiting_approval" {
			surface["waiting_approval"] = number(surface["waiting_approval"]) + 1
		}
		items, _ := surface["work_items"].([]Record)
		if len(items) < 10 {
			surface["work_items"] = append(items, item)
		}
	}
	out := make([]Record, 0, len(order))
	for _, department := range order {
		out = append(out, index[department])
	}
	return out
}

func cleanText(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	return text
}

func percent(done int, total int) int {
	if total == 0 {
		return 0
	}
	return int(math.Round(float64(done) * 100 / float64(total)))
}

func scopePipelineRunToWorkItems(run Record, workItems []Record) Record {
	visibleNodes := map[string]bool{}
	visibleSteps := map[string]bool{}
	for _, item := range workItems {
		if id := cleanID(item["run_node_id"]); id != "" {
			visibleNodes[id] = true
		}
		if id := cleanID(item["run_step_id"]); id != "" {
			visibleSteps[id] = true
		}
	}
	nodes, _ := run["nodes"].([]Record)
	filteredNodes := []Record{}
	for _, node := range nodes {
		steps, _ := node["steps"].([]Record)
		filteredSteps := []Record{}
		for _, step := range steps {
			if visibleSteps[fmt.Sprint(step["id"])] {
				filteredSteps = append(filteredSteps, step)
			}
		}
		if visibleNodes[fmt.Sprint(node["id"])] || len(filteredSteps) > 0 {
			node["steps"] = filteredSteps
			filteredNodes = append(filteredNodes, node)
		}
	}
	run["nodes"] = filteredNodes
	run["scoped"] = true
	return run
}

func workItemScopeArgs(profile workItemAccessProfile, managerOverride bool, includeWatchers bool) []any {
	return []any{
		managerOverride,
		profile.UserID,
		profile.StaffID,
		profile.Department,
		includeWatchers,
	}
}

func workItemScopeSQL(alias string, start int) string {
	manager := fmt.Sprintf("$%d::boolean", start)
	userID := fmt.Sprintf("NULLIF($%d,'')::uuid", start+1)
	staffID := fmt.Sprintf("NULLIF($%d,'')::uuid", start+2)
	staffText := fmt.Sprintf("$%d", start+2)
	department := fmt.Sprintf("$%d", start+3)
	includeWatchers := fmt.Sprintf("$%d::boolean", start+4)
	return fmt.Sprintf(`
		%s OR (
			(%s IS NOT NULL OR %s IS NOT NULL) AND (
				%s.assignee_staff_id=%s
				OR (%s AND %s.watcher_staff_ids ? %s)
				OR EXISTS (
					SELECT 1
					FROM work_item_assignments wias
					WHERE wias.work_item_id=%s.id
					  AND (
						(wias.assignment_type <> 'watcher' AND wias.staff_id=%s)
						OR (%s AND wias.assignment_type='watcher' AND wias.staff_id=%s)
					  )
				)
				OR EXISTS (
					SELECT 1
					FROM project_members pm
					WHERE pm.project_id=%s.project_id
					  AND pm.staff_id=%s
					  AND pm.active
					  AND (
						pm.role_ids ? 'admin'
						OR pm.role_ids ? 'director'
						OR pm.role_ids ? 'Giám đốc'
						OR pm.role_ids ? 'pm'
						OR pm.role_ids ? 'PM'
						OR pm.role_ids ? 'Project Manager'
						OR (NULLIF(%s,'') IS NOT NULL AND %s.department=%s)
						OR EXISTS (
							SELECT 1
							FROM jsonb_array_elements_text(pm.role_ids) pm_role(role_label)
							JOIN jsonb_array_elements_text(%s.role_ids) wi_role(role_label) ON wi_role.role_label=pm_role.role_label
						)
						OR EXISTS (
							SELECT 1
							FROM work_item_assignments wias
							WHERE wias.work_item_id=%s.id
							  AND wias.assignment_type <> 'watcher'
							  AND (
								(NULLIF(%s,'') IS NOT NULL AND wias.department=%s)
								OR (
									NULLIF(wias.role_name, '') IS NOT NULL
									AND EXISTS (
										SELECT 1
										FROM jsonb_array_elements_text(pm.role_ids) pm_role(role_label)
										WHERE pm_role.role_label=wias.role_name
									)
								)
							  )
						)
					  )
				)
				OR EXISTS (
					SELECT 1
					FROM user_roles ur
					JOIN roles r ON r.id=ur.role_id
					WHERE ur.user_id=%s
					  AND ur.project_id=%s.project_id
					  AND (
						r.name IN ('admin','director','pm')
						OR r.display_name IN ('Admin','Giám đốc','PM','Project Manager')
						OR EXISTS (
							SELECT 1
							FROM jsonb_array_elements_text(%s.role_ids) wi_role(role_label)
							WHERE wi_role.role_label IN (r.name, r.display_name, r.id::text)
						)
						OR EXISTS (
							SELECT 1
							FROM work_item_assignments wias
							WHERE wias.work_item_id=%s.id
							  AND wias.assignment_type <> 'watcher'
							  AND NULLIF(wias.role_name, '') IS NOT NULL
							  AND wias.role_name IN (r.name, r.display_name, r.id::text)
						)
					  )
				)
			)
		)
	`, manager, staffID, userID, alias, staffID, includeWatchers, alias, staffText, alias, staffID, includeWatchers, staffID, alias, staffID, department, alias, department, alias, alias, department, department, userID, alias, alias, alias)
}

func (r *Repository) ListProjects(ctx context.Context, userID string) ([]Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, err
	}
	return r.Query(ctx, fmt.Sprintf(`
		SELECT p.*, s.full_name AS pm_name, t.name AS pipeline_template_name,
			pt.name AS partner_name, pt.tax_code AS partner_tax_code,
			COALESCE((SELECT COUNT(*) FROM issues i WHERE i.project_id=p.id AND i.status IN ('open','in_progress')),0) AS open_issues,
			COALESCE((SELECT COUNT(*) FROM approval_requests ar WHERE ar.project_id=p.id AND ar.status='pending'),0) AS pending_approvals
		FROM projects p
		LEFT JOIN staff s ON s.id=p.pm_staff_id
		LEFT JOIN pipeline_templates t ON t.id=p.pipeline_template_id
		LEFT JOIN partners pt ON pt.id=p.partner_id
		WHERE p.status <> 'cancelled' AND %s
		ORDER BY p.created_at DESC
	`, scopeSQL), scopeArgs...)
}

func (r *Repository) GetProjectDetail(ctx context.Context, id string) (Record, error) {
	project, err := r.QueryOne(ctx, `
		SELECT p.*, s.full_name AS pm_name, t.name AS pipeline_template_name,
			pt.name AS partner_name, pt.tax_code AS partner_tax_code
		FROM projects p
		LEFT JOIN staff s ON s.id=p.pm_staff_id
		LEFT JOIN pipeline_templates t ON t.id=p.pipeline_template_id
		LEFT JOIN partners pt ON pt.id=p.partner_id
		WHERE p.id=$1 AND p.status <> 'cancelled'
	`, id)
	if err != nil {
		return nil, err
	}
	members, _ := r.Query(ctx, `
		SELECT pm.*, s.full_name, s.department, s.position
		FROM project_members pm
		JOIN staff s ON s.id=pm.staff_id
		WHERE pm.project_id=$1
		ORDER BY s.full_name
	`, id)
	project["members"] = members
	return project, nil
}

func (r *Repository) GetTemplateDetail(ctx context.Context, id string) (Record, error) {
	template, err := r.GetByID(ctx, "pipeline_templates", id)
	if err != nil {
		return nil, err
	}
	nodes, err := r.Query(ctx, `
		SELECT * FROM pipeline_template_nodes
		WHERE template_id=$1
		ORDER BY order_index, created_at
	`, id)
	if err != nil {
		return nil, err
	}
	edges, _ := r.Query(ctx, `
		SELECT * FROM pipeline_template_edges
		WHERE template_id=$1
		ORDER BY priority, created_at, id
	`, id)
	for _, node := range nodes {
		steps, _ := r.Query(ctx, `
			SELECT * FROM pipeline_template_steps
			WHERE node_id=$1
			ORDER BY order_index, created_at
		`, node["id"])
		node["steps"] = steps
	}
	template["nodes"] = nodes
	template["edges"] = edges
	return template, nil
}

func (r *Repository) CreateTemplateWithNodes(ctx context.Context, body Record, userID string) (Record, error) {
	if body["created_by"] == nil && userID != "" {
		body["created_by"] = userID
	}
	nodes, _ := body["nodes"].([]any)
	delete(body, "nodes")
	template, err := r.Insert(ctx, "pipeline_templates", templateColumns(), body)
	if err != nil {
		return nil, err
	}
	templateID := fmt.Sprint(template["id"])
	for index, raw := range nodes {
		nodeBody, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		nodeBody["template_id"] = templateID
		if nodeBody["order_index"] == nil {
			nodeBody["order_index"] = index + 1
		}
		steps, _ := nodeBody["steps"].([]any)
		delete(nodeBody, "steps")
		node, err := r.Insert(ctx, "pipeline_template_nodes", templateNodeColumns(), nodeBody)
		if err != nil {
			return nil, err
		}
		nodeID := fmt.Sprint(node["id"])
		for stepIndex, stepRaw := range steps {
			stepBody := Record{}
			if text, ok := stepRaw.(string); ok {
				stepBody["name"] = text
			}
			if obj, ok := stepRaw.(map[string]any); ok {
				stepBody = obj
			}
			stepBody["node_id"] = nodeID
			if stepBody["order_index"] == nil {
				stepBody["order_index"] = stepIndex + 1
			}
			if stepBody["step_type"] == nil {
				stepBody["step_type"] = "task"
			}
			formRequired := workflowFormRequiredForStep(fmt.Sprint(stepBody["step_type"]), stepBody["checklist_required"] == nil || boolValue(stepBody["checklist_required"]))
			if raw, ok := stepBody["form_required"]; ok {
				formRequired = boolValue(raw)
			} else {
				stepBody["form_required"] = workflowFormRequiredForStep(fmt.Sprint(stepBody["step_type"]), stepBody["checklist_required"] == nil || boolValue(stepBody["checklist_required"]))
			}
			if formRequired && cleanID(stepBody["form_version_id"]) == "" {
				formVersionID, err := r.DefaultWorkflowFormVersionID(ctx, fmt.Sprint(stepBody["step_type"]), stepBody["checklist_required"] == nil || boolValue(stepBody["checklist_required"]), fmt.Sprint(stepBody["name"]))
				if err != nil {
					return nil, err
				}
				if formVersionID != "" {
					stepBody["form_version_id"] = formVersionID
				}
			}
			if _, err := r.Insert(ctx, "pipeline_template_steps", templateStepColumns(), stepBody); err != nil {
				return nil, err
			}
		}
	}
	return r.GetTemplateDetail(ctx, templateID)
}

func (r *Repository) CloneTemplate(ctx context.Context, id, userID string) (Record, error) {
	detail, err := r.GetTemplateDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	body := Record{
		"name":         fmt.Sprintf("%s (copy)", detail["name"]),
		"description":  detail["description"],
		"project_type": detail["project_type"],
		"version":      1,
		"status":       "draft",
		"created_by":   userID,
	}
	clone, err := r.Insert(ctx, "pipeline_templates", templateColumns(), body)
	if err != nil {
		return nil, err
	}
	cloneID := fmt.Sprint(clone["id"])
	nodeIDMap := map[string]string{}
	nodes, _ := detail["nodes"].([]Record)
	for _, node := range nodes {
		oldID := fmt.Sprint(node["id"])
		nodeBody := Record{
			"template_id":          cloneID,
			"name":                 node["name"],
			"description":          node["description"],
			"type":                 node["type"],
			"position_x":           node["position_x"],
			"position_y":           node["position_y"],
			"order_index":          node["order_index"],
			"responsible_role_ids": node["responsible_role_ids"],
			"required_permission":  node["required_permission"],
			"permissions":          nodePermissions(node),
			"entry_conditions":     node["entry_conditions"],
			"exit_conditions":      node["exit_conditions"],
			"approval_rule":        node["approval_rule"],
			"rework_enabled":       node["rework_enabled"],
			"replanning_enabled":   node["replanning_enabled"],
			"fail_path":            node["fail_path"],
			"status":               node["status"],
		}
		newNode, err := r.Insert(ctx, "pipeline_template_nodes", templateNodeColumns(), nodeBody)
		if err != nil {
			return nil, err
		}
		newNodeID := fmt.Sprint(newNode["id"])
		nodeIDMap[oldID] = newNodeID
		steps, _ := node["steps"].([]Record)
		for _, step := range steps {
			stepBody := Record{
				"node_id":             newNodeID,
				"name":                step["name"],
				"description":         step["description"],
				"step_type":           step["step_type"],
				"required_role_ids":   step["required_role_ids"],
				"required_permission": step["required_permission"],
				"checklist_required":  step["checklist_required"],
				"document_required":   step["document_required"],
				"photo_required":      step["photo_required"],
				"form_version_id":     step["form_version_id"],
				"form_required":       step["form_required"],
				"order_index":         step["order_index"],
			}
			if _, err := r.Insert(ctx, "pipeline_template_steps", templateStepColumns(), stepBody); err != nil {
				return nil, err
			}
		}
	}
	edges, _ := detail["edges"].([]Record)
	for _, edge := range edges {
		from := nodeIDMap[fmt.Sprint(edge["from_node_id"])]
		to := nodeIDMap[fmt.Sprint(edge["to_node_id"])]
		if from == "" || to == "" {
			continue
		}
		_, _ = r.Insert(ctx, "pipeline_template_edges", edgeColumns(), Record{
			"template_id":      cloneID,
			"from_node_id":     from,
			"to_node_id":       to,
			"condition_type":   edge["condition_type"],
			"condition_config": edge["condition_config"],
			"label":            edge["label"],
			"priority":         edgePriority(edge),
		})
	}
	return r.GetTemplateDetail(ctx, cloneID)
}

func (r *Repository) StartPipeline(ctx context.Context, projectID, templateID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	projectRows, err := queryTx(ctx, tx, `SELECT id FROM projects WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, projectID)
	if err != nil {
		return nil, err
	}
	if len(projectRows) == 0 {
		return nil, sql.ErrNoRows
	}
	activeRuns, err := queryTx(ctx, tx, `
		SELECT id
		FROM pipeline_runs
		WHERE project_id=$1 AND status NOT IN ('completed','cancelled')
		ORDER BY started_at DESC
		LIMIT 1
	`, projectID)
	if err != nil {
		return nil, err
	}
	if len(activeRuns) > 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return r.GetPipelineRun(ctx, projectID)
	}

	if templateID == "" {
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(pipeline_template_id, (SELECT id FROM pipeline_templates WHERE status='published' ORDER BY created_at LIMIT 1)) FROM projects WHERE id=$1`, projectID).Scan(&templateID); err != nil {
			return nil, err
		}
	}
	if templateID == "" {
		return nil, errors.New("project has no pipeline template")
	}

	formIssues, err := queryTx(ctx, tx, templatePublishFormIssuesSQL(), templateID)
	if err != nil {
		return nil, err
	}
	if len(formIssues) > 0 {
		return nil, validationError("Template có step bắt buộc workflow form nhưng chưa map form published", Record{"issues": formIssues})
	}

	runID := newID()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pipeline_runs (id, project_id, template_id, status, started_at)
		VALUES ($1,$2,$3,'in_progress',now())
	`, runID, projectID, templateID); err != nil {
		return nil, err
	}

	nodes, err := queryTx(ctx, tx, `
		SELECT * FROM pipeline_template_nodes
		WHERE template_id=$1
		ORDER BY order_index, created_at
	`, templateID)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, errors.New("template has no nodes")
	}

	currentNodeID := ""
	runNodeIDByTemplateNodeID := map[string]string{}
	for index, node := range nodes {
		runNodeID := newID()
		runNodeIDByTemplateNodeID[fmt.Sprint(node["id"])] = runNodeID
		status := "pending"
		var startedAt any = nil
		if index == 0 {
			status = "in_progress"
			startedAt = "now"
			currentNodeID = runNodeID
		}
		startedSQL := "NULL"
		if startedAt != nil {
			startedSQL = "now()"
		}
		rawRoles, _ := json.Marshal(node["responsible_role_ids"])
		rawPermissions, _ := json.Marshal(nodePermissions(node))
		rawExit, _ := json.Marshal(node["exit_conditions"])
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
				INSERT INTO pipeline_run_nodes
					(id, run_id, project_id, template_node_id, name, description, type, order_index, responsible_role_ids, required_permission, permissions, exit_conditions, approval_rule, fail_path, status, started_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11::jsonb,$12::jsonb,$13,$14,$15,%s)
			`, startedSQL), runNodeID, runID, projectID, node["id"], node["name"], node["description"], node["type"], node["order_index"], string(rawRoles), node["required_permission"], string(rawPermissions), string(rawExit), node["approval_rule"], node["fail_path"], status); err != nil {
			return nil, err
		}
		steps, err := queryTx(ctx, tx, `
			SELECT * FROM pipeline_template_steps
			WHERE node_id=$1
			ORDER BY order_index, created_at
		`, node["id"])
		if err != nil {
			return nil, err
		}
		for _, step := range steps {
			runStepID := newID()
			workItemStatus := "pending"
			if index == 0 {
				workItemStatus = "ready"
			}
			rawStepRoles, _ := json.Marshal(step["required_role_ids"])
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO pipeline_run_steps
					(id, run_node_id, project_id, template_step_id, name, description, step_type, required_role_ids, required_permission, checklist_required, document_required, photo_required, order_index, status)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11,$12,$13,'pending')
			`, runStepID, runNodeID, projectID, step["id"], step["name"], step["description"], step["step_type"], string(rawStepRoles), step["required_permission"], step["checklist_required"], step["document_required"], step["photo_required"], step["order_index"]); err != nil {
				return nil, err
			}
			roleLabels := step["required_role_ids"]
			if len(stringSlice(roleLabels)) == 0 {
				roleLabels = node["responsible_role_ids"]
			}
			rawWorkItemRoles, _ := json.Marshal(roleLabels)
			rawEvidenceRequirement, _ := json.Marshal(Record{
				"checklist_required":  step["checklist_required"],
				"document_required":   step["document_required"],
				"photo_required":      step["photo_required"],
				"required_permission": step["required_permission"],
			})
			workItemID := newID()
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO work_items
					(id, project_id, pipeline_run_id, run_node_id, run_step_id, source_type, source_id, title, description, work_type, status, priority, role_ids, evidence_requirement_json, created_by, updated_by)
				VALUES ($1,$2,$3,$4,$5,'pipeline_run_step',$5,$6,$7,$8,$9,'normal',$10::jsonb,$11::jsonb,NULLIF($12,'')::uuid,NULLIF($12,'')::uuid)
			`, workItemID, projectID, runID, runNodeID, runStepID, step["name"], step["description"], workItemType(step["step_type"]), workItemStatus, string(rawWorkItemRoles), string(rawEvidenceRequirement), actorID); err != nil {
				return nil, err
			}
			for _, roleLabel := range stringSlice(roleLabels) {
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO work_item_assignments (work_item_id, assignment_type, role_name, created_by)
					VALUES ($1, 'role', $2, NULLIF($3,'')::uuid)
				`, workItemID, roleLabel, actorID); err != nil {
					return nil, err
				}
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, to_status, comment, metadata_json)
				VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'created', $4, 'Work item generated from pipeline run step', $5::jsonb)
			`, workItemID, projectID, actorID, workItemStatus, `{"source_type":"pipeline_run_step"}`); err != nil {
				return nil, err
			}
			if formVersionID := cleanID(step["form_version_id"]); formVersionID != "" {
				formRows, err := queryTx(ctx, tx, `
					INSERT INTO workflow_form_instances
						(project_id, work_item_id, pipeline_run_id, run_node_id, run_step_id, template_step_id, form_template_id, form_version_id, form_type, title, required_role_ids, created_by)
					SELECT $1, $2, $3, $4, $5, $6, v.template_id, v.id, t.form_type, $7, $8::jsonb, NULLIF($9,'')::uuid
					FROM workflow_form_versions v
					JOIN workflow_form_templates t ON t.id=v.template_id
					WHERE v.id=$10
					ON CONFLICT DO NOTHING
					RETURNING *
				`, projectID, workItemID, runID, runNodeID, runStepID, step["id"], step["name"], string(rawWorkItemRoles), actorID, formVersionID)
				if err != nil {
					return nil, err
				}
				if len(formRows) > 0 {
					if _, err := tx.ExecContext(ctx, `
						INSERT INTO workflow_form_events (instance_id, project_id, work_item_id, actor_id, event_type, to_status, comment, metadata_json)
						VALUES ($1, $2, $3, NULLIF($4,'')::uuid, 'created', 'draft', 'Workflow form instance created from pipeline start', $5::jsonb)
					`, formRows[0]["id"], projectID, workItemID, actorID, `{"source":"start_pipeline"}`); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	edges, err := queryTx(ctx, tx, `
		SELECT * FROM pipeline_template_edges
		WHERE template_id=$1
		ORDER BY priority, created_at, id
	`, templateID)
	if err != nil {
		return nil, err
	}
	for _, edge := range edges {
		fromRunNodeID := runNodeIDByTemplateNodeID[fmt.Sprint(edge["from_node_id"])]
		toRunNodeID := runNodeIDByTemplateNodeID[fmt.Sprint(edge["to_node_id"])]
		if fromRunNodeID == "" || toRunNodeID == "" {
			continue
		}
		rawConfig, _ := json.Marshal(edge["condition_config"])
		if _, err := tx.ExecContext(ctx, `
		INSERT INTO pipeline_run_edges
			(id, run_id, project_id, template_edge_id, from_run_node_id, to_run_node_id, condition_type, condition_config, label, priority)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10)
	`, newID(), runID, projectID, edge["id"], fromRunNodeID, toRunNodeID, edgeConditionType(edge), string(rawConfig), edge["label"], edgePriority(edge)); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE pipeline_runs SET current_node_id=$1 WHERE id=$2`, currentNodeID, runID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET current_pipeline_run_id=$1, status='planning', progress_percent=0, updated_at=now() WHERE id=$2`, runID, projectID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description)
		VALUES ($1, NULLIF($2,'')::uuid, $3, 'node_started', 'pipeline_run', $4, 'Tạo Pipeline Run và mở node đầu tiên')
	`, projectID, actorID, actorName, runID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPipelineRun(ctx, projectID)
}

func (r *Repository) GetPipelineRun(ctx context.Context, projectID string) (Record, error) {
	run, err := r.QueryOne(ctx, `
		SELECT pr.*, p.name AS project_name, pt.name AS template_name
		FROM pipeline_runs pr
		JOIN projects p ON p.id=pr.project_id
		JOIN pipeline_templates pt ON pt.id=pr.template_id
		WHERE pr.project_id=$1
		ORDER BY pr.started_at DESC
		LIMIT 1
	`, projectID)
	if err != nil {
		return nil, err
	}
	nodes, err := r.Query(ctx, `
		SELECT * FROM pipeline_run_nodes
		WHERE run_id=$1
		ORDER BY order_index, started_at NULLS LAST
	`, run["id"])
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		steps, _ := r.Query(ctx, `
			SELECT
				rs.*,
				wi.id AS work_item_id,
				wi.status AS work_item_status,
				wi.title AS work_item_title,
				wi.role_ids AS work_item_role_ids,
				wi.evidence_requirement_json,
				COALESCE((
					SELECT COUNT(*)
					FROM evidence_records er
					WHERE er.work_item_id=wi.id
					  AND er.status <> 'rejected'
				), 0) AS evidence_count,
				wfi.id AS workflow_form_instance_id,
				wfi.status AS workflow_form_status,
				wfi.form_type AS workflow_form_type,
				wfi.current_revision AS workflow_form_revision,
				wfi.submitted_at AS workflow_form_submitted_at,
				wfi.form_template_id AS workflow_form_template_id,
				COALESCE(wfi.form_version_id, pts.form_version_id) AS workflow_form_version_id,
				wfv.version_no AS workflow_form_version_no,
				wfv.status AS workflow_form_version_status,
				wft.code AS workflow_form_template_code,
				wft.name AS workflow_form_template_name,
				COALESCE(pts.form_required, false) AS workflow_form_required
			FROM pipeline_run_steps rs
			LEFT JOIN work_items wi ON wi.run_step_id=rs.id AND wi.status <> 'cancelled'
			LEFT JOIN workflow_form_instances wfi ON wfi.run_step_id=rs.id AND wfi.status <> 'cancelled'
			LEFT JOIN pipeline_template_steps pts ON pts.id=rs.template_step_id
			LEFT JOIN workflow_form_versions wfv ON wfv.id=COALESCE(wfi.form_version_id, pts.form_version_id)
			LEFT JOIN workflow_form_templates wft ON wft.id=COALESCE(wfi.form_template_id, wfv.template_id)
			WHERE rs.run_node_id=$1
			ORDER BY rs.order_index, rs.created_at
		`, node["id"])
		node["steps"] = steps
	}
	run["nodes"] = nodes
	return run, nil
}

func (r *Repository) SubmitRunStep(ctx context.Context, stepID string, req WorkItemActionRequest, actorID, actorName string) (Record, error) {
	payload := req.CompletionPayload
	if payload == nil {
		payload = Record{}
	}
	rawPayload, _ := json.Marshal(payload)
	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		comment = "Completed through legacy pipeline step submit"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	steps, err := queryTx(ctx, tx, `
		UPDATE pipeline_run_steps
		SET status='completed', submitted_by=NULLIF($2,'')::uuid, submitted_at=now()
		WHERE id=$1
		RETURNING *
	`, stepID, actorID)
	if err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return nil, sql.ErrNoRows
	}
	step := steps[0]
	workItems, err := queryTx(ctx, tx, `
		WITH existing AS (
			SELECT id, status AS previous_status
			FROM work_items
			WHERE run_step_id=$1 AND status <> 'cancelled'
		)
			UPDATE work_items
			SET status='completed',
				completed_at=now(),
				completion_payload_json=$3::jsonb,
				updated_by=NULLIF($2,'')::uuid,
				updated_at=now()
			FROM existing
			WHERE work_items.id=existing.id
			RETURNING work_items.*, existing.previous_status
		`, stepID, actorID, string(rawPayload))
	if err != nil {
		return nil, err
	}
	if len(workItems) > 0 {
		item := workItems[0]
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, from_status, to_status, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'completed', $4, $5, 'completed', $6::jsonb)
		`, item["id"], item["project_id"], actorID, comment, item["previous_status"], `{"source":"pipeline_run_step_submit"}`); err != nil {
			return nil, err
		}
	}
	rawMetadata, _ := json.Marshal(Record{"step": step["name"]})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json)
		VALUES ($1, NULLIF($2,'')::uuid, $3, 'step_completed', 'pipeline_run_step', $4, $5, $6::jsonb)
	`, step["project_id"], actorID, actorName, stepID, fmt.Sprintf("Hoàn tất step %s", step["name"]), string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return step, nil
}

func (r *Repository) RequestNodeApproval(ctx context.Context, nodeID, actorID, actorName, reason string) (Record, error) {
	node, err := r.GetByID(ctx, "pipeline_run_nodes", nodeID)
	if err != nil {
		return nil, err
	}
	request, err := r.Insert(ctx, "approval_requests", []ColumnSpec{
		col("target_type"), col("target_id"), col("project_id"), col("requested_by"), col("status"), col("reason"), jsonCol("required_roles"),
	}, Record{
		"target_type":    "pipeline_run_node",
		"target_id":      nodeID,
		"project_id":     node["project_id"],
		"requested_by":   actorID,
		"status":         "pending",
		"reason":         reason,
		"required_roles": node["responsible_role_ids"],
	})
	if err != nil {
		return nil, err
	}
	_, _ = r.QueryOne(ctx, `UPDATE pipeline_run_nodes SET status='waiting_approval' WHERE id=$1 RETURNING *`, nodeID)
	_ = r.AddAudit(ctx, fmt.Sprint(node["project_id"]), actorID, actorName, "review_requested", "pipeline_run_node", nodeID, fmt.Sprintf("Yêu cầu duyệt node %s", node["name"]), Record{"approval_request_id": request["id"]})
	return request, nil
}

func (r *Repository) AdvanceRunNode(ctx context.Context, nodeID, actorID, actorName string) (Record, error) {
	node, err := r.GetByID(ctx, "pipeline_run_nodes", nodeID)
	if err != nil {
		return nil, err
	}
	if blockers := r.nodeBlockers(ctx, node); len(blockers) > 0 {
		reason := stringsJoin(blockers)
		blocked, err := r.QueryOne(ctx, `UPDATE pipeline_run_nodes SET status='blocked', block_reason=$2 WHERE id=$1 RETURNING *`, nodeID, reason)
		if err != nil {
			return nil, err
		}
		_ = r.AddAudit(ctx, fmt.Sprint(node["project_id"]), actorID, actorName, "node_blocked", "pipeline_run_node", nodeID, reason, Record{"blockers": blockers})
		return blocked, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	nextNode, transition, transitionBlockers, err := r.nextRunNode(ctx, tx, node)
	if err != nil {
		return nil, err
	}
	if len(transitionBlockers) > 0 {
		reason := stringsJoin(transitionBlockers)
		blockedRows, err := queryTx(ctx, tx, `UPDATE pipeline_run_nodes SET status='blocked', block_reason=$2 WHERE id=$1 RETURNING *`, nodeID, reason)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		metadata := Record{"blockers": transitionBlockers}
		if transition != nil {
			metadata["transition"] = transition
		}
		_ = r.AddAudit(ctx, fmt.Sprint(node["project_id"]), actorID, actorName, "node_blocked", "pipeline_run_node", nodeID, reason, metadata)
		if len(blockedRows) > 0 {
			return blockedRows[0], nil
		}
		return node, nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE pipeline_run_nodes SET status='completed', completed_at=now(), block_reason=NULL WHERE id=$1`, nodeID); err != nil {
		return nil, err
	}
	if nextNode != nil {
		nextID := fmt.Sprint(nextNode["id"])
		if _, err := tx.ExecContext(ctx, `UPDATE pipeline_run_nodes SET status='in_progress', started_at=COALESCE(started_at, now()) WHERE id=$1`, nextID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE pipeline_runs SET current_node_id=$1 WHERE id=$2`, nextID, node["run_id"]); err != nil {
			return nil, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE pipeline_runs SET status='completed', completed_at=now(), current_node_id=NULL WHERE id=$1`, node["run_id"]); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE projects p
		SET progress_percent = COALESCE((
			SELECT ROUND(100.0 * COUNT(*) FILTER (WHERE status='completed') / NULLIF(COUNT(*),0))::int
			FROM pipeline_run_nodes rn
			WHERE rn.run_id=$2
		),0),
		status = CASE WHEN EXISTS (SELECT 1 FROM pipeline_runs pr WHERE pr.id=$2 AND pr.status='completed') THEN 'financial_closure' ELSE p.status END,
		updated_at=now()
		WHERE p.id=$1
	`, node["project_id"], node["run_id"]); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	metadata := Record{"node": node["name"]}
	description := fmt.Sprintf("Hoàn tất node %s", node["name"])
	if transition != nil {
		metadata["transition"] = transition
		if targetName := fmt.Sprint(transition["to_node_name"]); targetName != "" {
			description = fmt.Sprintf("Hoàn tất node %s -> %s", node["name"], targetName)
		}
	}
	_ = r.AddAudit(ctx, fmt.Sprint(node["project_id"]), actorID, actorName, "node_completed", "pipeline_run_node", nodeID, description, metadata)
	return r.GetPipelineRun(ctx, fmt.Sprint(node["project_id"]))
}

func (r *Repository) nextRunNode(ctx context.Context, tx *sql.Tx, node Record) (Record, Record, []string, error) {
	edges, err := queryTx(ctx, tx, `
		SELECT e.*, rn.name AS to_node_name, rn.order_index AS to_order_index, rn.status AS to_status
		FROM pipeline_run_edges e
		JOIN pipeline_run_nodes rn ON rn.id=e.to_run_node_id
		WHERE e.run_id=$1 AND e.from_run_node_id=$2
		ORDER BY e.priority, e.created_at, e.id
	`, node["run_id"], node["id"])
	if err != nil {
		return nil, nil, nil, err
	}
	evaluations := []Record{}
	blockers := []string{}
	for _, edge := range edges {
		matched, reason, err := r.runEdgeConditionMet(ctx, tx, edge, node)
		if err != nil {
			return nil, nil, nil, err
		}
		evaluations = append(evaluations, Record{
			"edge_id":        edge["id"],
			"label":          edge["label"],
			"condition_type": edgeConditionType(edge),
			"matched":        matched,
			"reason":         reason,
		})
		if !matched {
			blockers = append(blockers, fmt.Sprintf("%s: %s", edgeDisplayLabel(edge), reason))
			continue
		}
		nextRows, err := queryTx(ctx, tx, `SELECT * FROM pipeline_run_nodes WHERE id=$1 LIMIT 1`, edge["to_run_node_id"])
		if err != nil {
			return nil, nil, nil, err
		}
		if len(nextRows) == 0 {
			continue
		}
		transition := Record{
			"mode":             "edge",
			"edge_id":          edge["id"],
			"template_edge_id": edge["template_edge_id"],
			"condition_type":   edgeConditionType(edge),
			"label":            edge["label"],
			"priority":         edgePriority(edge),
			"reason":           reason,
			"to_node_id":       edge["to_run_node_id"],
			"to_node_name":     edge["to_node_name"],
		}
		return nextRows[0], transition, nil, nil
	}
	if len(edges) > 0 {
		return nil, Record{"mode": "edge", "evaluations": evaluations}, blockers, nil
	}
	return r.nextRunNodeByOrder(ctx, tx, node)
}

func (r *Repository) nextRunNodeByOrder(ctx context.Context, tx *sql.Tx, node Record) (Record, Record, []string, error) {
	nextRows, err := queryTx(ctx, tx, `
		SELECT * FROM pipeline_run_nodes
		WHERE run_id=$1 AND order_index > $2
		ORDER BY order_index
		LIMIT 1
	`, node["run_id"], node["order_index"])
	if err != nil {
		return nil, nil, nil, err
	}
	if len(nextRows) == 0 {
		return nil, Record{"mode": "completed"}, nil, nil
	}
	return nextRows[0], Record{
		"mode":         "fallback_order_index",
		"to_node_id":   nextRows[0]["id"],
		"to_node_name": nextRows[0]["name"],
	}, nil, nil
}

func (r *Repository) runEdgeConditionMet(ctx context.Context, tx *sql.Tx, edge, fromNode Record) (bool, string, error) {
	projectID := fmt.Sprint(fromNode["project_id"])
	nodeID := fmt.Sprint(fromNode["id"])

	switch edgeConditionType(edge) {
	case "", "always":
		return true, "condition always matched", nil
	case "order_index":
		matched := number(edge["to_order_index"]) > number(fromNode["order_index"])
		return matched, boolReason(matched, "target node is after source node", "target node is not after source node"), nil
	case "approval_approved":
		return r.edgeExists(ctx, tx, "node approval has been approved", "node approval is not approved", `
			SELECT COUNT(*) AS total
			FROM approval_requests
			WHERE project_id=$1 AND target_type='pipeline_run_node' AND target_id=$2 AND status='approved'
		`, projectID, nodeID)
	case "approval_rejected":
		return r.edgeExists(ctx, tx, "node approval has been rejected", "node approval is not rejected", `
			SELECT COUNT(*) AS total
			FROM approval_requests
			WHERE project_id=$1 AND target_type='pipeline_run_node' AND target_id=$2 AND status='rejected'
		`, projectID, nodeID)
	case "acceptance_passed":
		return r.edgeExists(ctx, tx, "acceptance passed", "no completed passed acceptance", `
			SELECT COUNT(*) AS total
			FROM acceptance_records
			WHERE project_id=$1 AND result='pass' AND status='completed'
		`, projectID)
	case "acceptance_failed":
		return r.edgeExists(ctx, tx, "acceptance failed", "no failed acceptance", `
			SELECT COUNT(*) AS total
			FROM acceptance_records
			WHERE project_id=$1 AND result='fail'
		`, projectID)
	case "rework_required":
		return r.edgeExists(ctx, tx, "rework is required", "no open rework requirement", `
			SELECT COUNT(*) AS total
			FROM (
				SELECT id FROM rework_requests WHERE project_id=$1 AND status IN ('draft','approved','in_progress')
				UNION ALL
				SELECT id FROM issues WHERE project_id=$1 AND requires_rework AND status IN ('open','in_progress')
			) required_rework
		`, projectID)
	case "rework_completed":
		total, err := r.scalarNumberTx(ctx, tx, `SELECT COUNT(*) AS total FROM rework_requests WHERE project_id=$1`, projectID)
		if err != nil {
			return false, "", err
		}
		open, err := r.scalarNumberTx(ctx, tx, `SELECT COUNT(*) AS total FROM rework_requests WHERE project_id=$1 AND status IN ('draft','approved','in_progress')`, projectID)
		if err != nil {
			return false, "", err
		}
		matched := total > 0 && open == 0
		return matched, boolReason(matched, "all rework requests are done", "rework requests are still open or do not exist"), nil
	case "replanning_required":
		return r.edgeExists(ctx, tx, "replanning is required", "no open replanning requirement", `
			SELECT COUNT(*) AS total
			FROM (
				SELECT id FROM replanning_requests WHERE project_id=$1 AND status IN ('draft','submitted')
				UNION ALL
				SELECT id FROM issues WHERE project_id=$1 AND requires_replanning AND status IN ('open','in_progress')
			) required_replanning
		`, projectID)
	case "materials_issued":
		missing, err := r.scalarNumberTx(ctx, tx, `
			SELECT COUNT(*) AS total
			FROM site_material_balances
			WHERE project_id=$1 AND issued_quantity < planned_quantity
		`, projectID)
		if err != nil {
			return false, "", err
		}
		matched := missing == 0
		return matched, boolReason(matched, "all planned materials have been issued", "planned materials are still missing"), nil
	case "boq_approved":
		total, err := r.scalarNumberTx(ctx, tx, `SELECT COUNT(*) AS total FROM boq_items WHERE project_id=$1 AND status <> 'deleted'`, projectID)
		if err != nil {
			return false, "", err
		}
		notApproved, err := r.scalarNumberTx(ctx, tx, `SELECT COUNT(*) AS total FROM boq_items WHERE project_id=$1 AND status <> 'deleted' AND status <> 'approved'`, projectID)
		if err != nil {
			return false, "", err
		}
		matched := total > 0 && notApproved == 0
		return matched, boolReason(matched, "all BOQ items are approved", "BOQ is missing or has unapproved items"), nil
	case "procurement_approved":
		total, err := r.scalarNumberTx(ctx, tx, `SELECT COUNT(*) AS total FROM procurement_requests WHERE project_id=$1 AND status <> 'cancelled'`, projectID)
		if err != nil {
			return false, "", err
		}
		notApproved, err := r.scalarNumberTx(ctx, tx, `
			SELECT COUNT(*) AS total
			FROM procurement_requests
			WHERE project_id=$1 AND status <> 'cancelled' AND status NOT IN ('approved','ordered','partially_received','received')
		`, projectID)
		if err != nil {
			return false, "", err
		}
		matched := total > 0 && notApproved == 0
		return matched, boolReason(matched, "procurement requests are approved", "procurement is missing or has unapproved requests"), nil
	case "handover_ready":
		return r.handoverReadyCondition(ctx, tx, projectID)
	case "financial_ready":
		openDebt, err := r.scalarNumberTx(ctx, tx, `
			SELECT COUNT(*) AS total
			FROM (
				SELECT id FROM payables WHERE project_id=$1 AND status <> 'cancelled' AND remaining_amount > 0
				UNION ALL
				SELECT id FROM receivables WHERE project_id=$1 AND status <> 'cancelled' AND remaining_amount > 0
			) open_debt
		`, projectID)
		if err != nil {
			return false, "", err
		}
		matched := openDebt == 0
		return matched, boolReason(matched, "no open payable or receivable balance", "payable or receivable balances are still open"), nil
	default:
		return false, fmt.Sprintf("unsupported condition_type %s", edgeConditionType(edge)), nil
	}
}

func (r *Repository) edgeExists(ctx context.Context, tx *sql.Tx, passReason, failReason, query string, args ...any) (bool, string, error) {
	total, err := r.scalarNumberTx(ctx, tx, query, args...)
	if err != nil {
		return false, "", err
	}
	matched := total > 0
	return matched, boolReason(matched, passReason, failReason), nil
}

func (r *Repository) handoverReadyCondition(ctx context.Context, tx *sql.Tx, projectID string) (bool, string, error) {
	handoverReady, err := r.scalarNumberTx(ctx, tx, `
		SELECT COUNT(*) AS total
		FROM handover_records
		WHERE project_id=$1
			AND documents_ready
			AND warehouse_closed
			AND payables_recorded
			AND receivables_recorded
			AND status <> 'blocked'
	`, projectID)
	if err != nil {
		return false, "", err
	}
	failedAcceptance, err := r.scalarNumberTx(ctx, tx, `
		SELECT COUNT(*) AS total
		FROM acceptance_records
		WHERE project_id=$1 AND result='fail' AND status <> 'cancelled'
	`, projectID)
	if err != nil {
		return false, "", err
	}
	openIssues, err := r.scalarNumberTx(ctx, tx, `
		SELECT COUNT(*) AS total
		FROM issues
		WHERE project_id=$1 AND status IN ('open','in_progress')
	`, projectID)
	if err != nil {
		return false, "", err
	}
	pendingSurplus, err := r.scalarNumberTx(ctx, tx, `
		SELECT COUNT(*) AS total
		FROM site_material_balances
		WHERE project_id=$1 AND surplus_quantity > 0 AND surplus_status <> 'handled'
	`, projectID)
	if err != nil {
		return false, "", err
	}
	matched := handoverReady > 0 && failedAcceptance == 0 && openIssues == 0 && pendingSurplus == 0
	return matched, boolReason(matched, "handover checklist has no blockers", "handover still has blockers"), nil
}

func (r *Repository) scalarNumberTx(ctx context.Context, tx *sql.Tx, query string, args ...any) (float64, error) {
	rows, err := queryTx(ctx, tx, query, args...)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return number(rows[0]["total"]), nil
}

func boolReason(matched bool, passReason, failReason string) string {
	if matched {
		return passReason
	}
	return failReason
}

func edgeDisplayLabel(edge Record) string {
	label := strings.TrimSpace(fmt.Sprint(edge["label"]))
	if label != "" && label != "<nil>" {
		return label
	}
	return edgeConditionType(edge)
}

func nodePermissions(node Record) []string {
	permissions := stringSlice(node["permissions"])
	if len(permissions) > 0 {
		return permissions
	}
	legacy := strings.TrimSpace(fmt.Sprint(node["required_permission"]))
	if legacy != "" && legacy != "<nil>" {
		return []string{legacy}
	}
	return []string{}
}

func stringSlice(value any) []string {
	out := []string{}
	switch v := value.(type) {
	case []string:
		out = append(out, v...)
	case []any:
		for _, item := range v {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
	}
	return out
}

func normalizedRoleList(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text == "" || text == "<nil>" || seen[text] {
			continue
		}
		seen[text] = true
		out = append(out, text)
	}
	return out
}

func edgeConditionType(edge Record) string {
	conditionType := strings.TrimSpace(fmt.Sprint(edge["condition_type"]))
	if conditionType == "" || conditionType == "<nil>" {
		return "always"
	}
	return conditionType
}

func edgePriority(edge Record) int {
	priority, err := parseEdgePriority(edge["priority"])
	if err != nil {
		return 100
	}
	return priority
}

func workItemType(value any) string {
	workType := strings.TrimSpace(fmt.Sprint(value))
	switch workType {
	case "task", "checklist", "approval", "document_upload", "photo_upload", "design_review", "budget_check", "material_check", "daily_report", "acceptance_check", "rework_task", "handover_check", "accounting_check":
		return workType
	default:
		return "task"
	}
}

func (r *Repository) nodeBlockers(ctx context.Context, node Record) []string {
	blockers := []string{}
	nodeType := fmt.Sprint(node["type"])
	projectID := fmt.Sprint(node["project_id"])
	if nodeType == "construction" {
		project, err := r.GetByID(ctx, "projects", projectID)
		if err == nil {
			contractAmount := number(project["contract_amount"])
			rows, _ := r.Query(ctx, `
				SELECT COALESCE(SUM(received_amount),0) AS received
				FROM receivables
				WHERE project_id=$1
			`, projectID)
			received := 0.0
			if len(rows) > 0 {
				received = number(rows[0]["received"])
			}
			if contractAmount > 0 && received/contractAmount < 0.30 {
				blockers = append(blockers, "Chưa thu đủ tối thiểu 30% giá trị hợp đồng để bắt đầu thi công")
			}
		}
		missing, _ := r.Query(ctx, `
			SELECT COUNT(*) AS total
			FROM site_material_balances
			WHERE project_id=$1 AND issued_quantity < planned_quantity
		`, projectID)
		if len(missing) > 0 && number(missing[0]["total"]) > 0 {
			blockers = append(blockers, "Vật tư required chưa được xuất đủ ra công trình")
		}
	}
	return blockers
}

func (r *Repository) BlockRunNode(ctx context.Context, nodeID, reason, actorID, actorName string) (Record, error) {
	node, err := r.QueryOne(ctx, `UPDATE pipeline_run_nodes SET status='blocked', block_reason=$2 WHERE id=$1 RETURNING *`, nodeID, reason)
	if err != nil {
		return nil, err
	}
	_ = r.AddAudit(ctx, fmt.Sprint(node["project_id"]), actorID, actorName, "node_blocked", "pipeline_run_node", nodeID, reason, Record{})
	return node, nil
}

func (r *Repository) ApprovalAction(ctx context.Context, approvalID, action, comment, actorID, actorName string) (Record, error) {
	request, err := r.GetByID(ctx, "approval_requests", approvalID)
	if err != nil {
		return nil, err
	}
	status, ok := approvalStatusForAction(action)
	if !ok {
		return nil, validationError("Action approval không hợp lệ", Record{"action": action})
	}
	event := "approval_granted"
	if action == "reject" {
		event = "approval_rejected"
	} else if action == "request-change" {
		event = "approval_changes_requested"
	}
	projectID := cleanID(request["project_id"])
	targetID := cleanID(request["target_id"])
	targetType := strings.TrimSpace(fmt.Sprint(request["target_type"]))
	var projectParam any
	if projectID != "" {
		projectParam = projectID
	}
	var targetParam any
	if targetID != "" {
		targetParam = targetID
	}
	rawMetadata, _ := json.Marshal(Record{"approval_request_id": approvalID, "action": action})
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO approval_actions (approval_request_id, user_id, action, comment)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4)
	`, approvalID, actorID, action, comment); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE approval_requests SET status=$2, resolved_at=now() WHERE id=$1`, approvalID, status); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7, $8::jsonb)
	`, projectParam, actorID, actorName, event, targetType, targetParam, fmt.Sprintf("%s approval: %s", action, comment), string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, "approval_requests", approvalID)
}

func (r *Repository) ListProjectBOQ(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT b.*, m.name AS material_name,
			COALESCE(b.final_unit_price, b.norm_unit_price, b.expected_unit_price, b.unit_price, 0) AS effective_unit_price,
			(b.quantity * COALESCE(b.final_unit_price, b.norm_unit_price, b.expected_unit_price, b.unit_price, 0))::numeric(18,2) AS effective_total_amount,
			CASE
				WHEN b.final_price_locked THEN 'final_locked'
				WHEN b.norm_price_locked THEN 'norm_locked'
				ELSE 'expected'
			END AS price_lifecycle_status,
			COALESCE((
				SELECT jsonb_agg(to_jsonb(price_events) ORDER BY price_events.created_at DESC)
				FROM (
					SELECT id, event_type, price_source, from_unit_price, to_unit_price, revision_no, comment, actor_name, created_at
					FROM boq_unit_price_events
					WHERE boq_item_id=b.id
					ORDER BY created_at DESC
					LIMIT 5
				) price_events
			), '[]'::jsonb) AS price_events
		FROM boq_items b
		LEFT JOIN materials m ON m.id=b.material_id
		WHERE b.project_id=$1 AND b.status <> 'deleted'
		ORDER BY b.category, b.created_at
	`, projectID)
}

func (r *Repository) GetBOQItemDetail(ctx context.Context, itemID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT b.*, m.name AS material_name,
			COALESCE(b.final_unit_price, b.norm_unit_price, b.expected_unit_price, b.unit_price, 0) AS effective_unit_price,
			(b.quantity * COALESCE(b.final_unit_price, b.norm_unit_price, b.expected_unit_price, b.unit_price, 0))::numeric(18,2) AS effective_total_amount,
			CASE
				WHEN b.final_price_locked THEN 'final_locked'
				WHEN b.norm_price_locked THEN 'norm_locked'
				ELSE 'expected'
			END AS price_lifecycle_status,
			COALESCE((
				SELECT jsonb_agg(to_jsonb(price_events) ORDER BY price_events.created_at DESC)
				FROM (
					SELECT id, event_type, price_source, from_unit_price, to_unit_price, revision_no, comment, actor_name, created_at
					FROM boq_unit_price_events
					WHERE boq_item_id=b.id
					ORDER BY created_at DESC
					LIMIT 20
				) price_events
			), '[]'::jsonb) AS price_events
		FROM boq_items b
		LEFT JOIN materials m ON m.id=b.material_id
		WHERE b.id=$1 AND b.status <> 'deleted'
	`, itemID)
}

func (r *Repository) CreateBOQItem(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		INSERT INTO boq_items (
			project_id, category, work_item_name, description, unit, quantity, expected_unit_price,
			material_id, node_id, status, created_by, price_revision_no
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::uuid,NULLIF($9,'')::uuid,$10,NULLIF($11,'')::uuid,1)
		RETURNING *
	`, body["project_id"], body["category"], body["work_item_name"], body["description"], body["unit"], body["quantity"], body["expected_unit_price"], cleanID(body["material_id"]), cleanID(body["node_id"]), body["status"], actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	item := rows[0]
	if err := insertBOQUnitPriceEventTx(ctx, tx, item, "expected_updated", "expected", nil, number(item["expected_unit_price"]), "Initial expected unit price", actorID, actorName); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(item["project_id"]), actorID, actorName, "boq_created", "boq_item", fmt.Sprint(item["id"]), "Tạo dòng BOQ", Record{"price_revision_no": item["price_revision_no"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetBOQItemDetail(ctx, fmt.Sprint(item["id"]))
}

func (r *Repository) UpdateBOQItem(ctx context.Context, itemID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	currentRows, err := queryTx(ctx, tx, `SELECT * FROM boq_items WHERE id=$1 AND status <> 'deleted' FOR UPDATE`, itemID)
	if err != nil {
		return nil, err
	}
	if len(currentRows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := currentRows[0]
	if boolValue(current["final_price_locked"]) && boqBodyChangesAny(current, body, finalLockedBOQProtectedFields...) {
		return nil, validationError("Dòng BOQ đã final lock, không thể sửa khối lượng/vật tư/đơn giá", Record{"boq_item_id": itemID})
	}
	if boolValue(current["norm_price_locked"]) && boqBodyChangesAny(current, body, "expected_unit_price") {
		return nil, validationError("Dòng BOQ đã norm lock, không thể sửa expected unit price", Record{"boq_item_id": itemID})
	}

	specs := []string{"category", "work_item_name", "description", "unit", "quantity", "expected_unit_price", "material_id", "node_id", "status", "approved_by"}
	sets := make([]string, 0, len(specs)+2)
	args := make([]any, 0, len(specs)+1)
	for _, key := range specs {
		value, ok := body[key]
		if !ok {
			continue
		}
		args = append(args, value)
		if key == "material_id" || key == "node_id" || key == "approved_by" {
			sets = append(sets, fmt.Sprintf("%s=NULLIF($%d,'')::uuid", key, len(args)))
		} else {
			sets = append(sets, fmt.Sprintf("%s=$%d", key, len(args)))
		}
	}
	expectedChanged := boqBodyChangesAny(current, body, "expected_unit_price")
	if expectedChanged {
		sets = append(sets, "price_revision_no=price_revision_no+1")
	}
	if len(sets) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return r.GetBOQItemDetail(ctx, itemID)
	}
	sets = append(sets, "updated_at=now()")
	args = append(args, itemID)
	updatedRows, err := queryTx(ctx, tx, fmt.Sprintf(`UPDATE boq_items SET %s WHERE id=$%d RETURNING *`, strings.Join(sets, ","), len(args)), args...)
	if err != nil {
		return nil, err
	}
	if len(updatedRows) == 0 {
		return nil, sql.ErrNoRows
	}
	updated := updatedRows[0]
	if expectedChanged {
		if err := insertBOQUnitPriceEventTx(ctx, tx, updated, "expected_updated", "expected", numberPtr(current["expected_unit_price"]), number(updated["expected_unit_price"]), "Updated expected unit price", actorID, actorName); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(updated["project_id"]), actorID, actorName, "boq_updated", "boq_item", itemID, "Cập nhật dòng BOQ", Record{"price_revision_no": updated["price_revision_no"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetBOQItemDetail(ctx, itemID)
}

func (r *Repository) UpdateBOQExpectedUnitPrice(ctx context.Context, itemID string, unitPrice float64, comment, actorID, actorName string) (Record, error) {
	return r.changeBOQUnitPrice(ctx, itemID, "expected_updated", unitPrice, comment, actorID, actorName)
}

func (r *Repository) LockBOQNormUnitPrice(ctx context.Context, itemID string, unitPrice float64, comment, actorID, actorName string) (Record, error) {
	return r.changeBOQUnitPrice(ctx, itemID, "norm_locked", unitPrice, comment, actorID, actorName)
}

func (r *Repository) LockBOQFinalUnitPrice(ctx context.Context, itemID string, unitPrice float64, comment, actorID, actorName string) (Record, error) {
	return r.changeBOQUnitPrice(ctx, itemID, "final_locked", unitPrice, comment, actorID, actorName)
}

func (r *Repository) changeBOQUnitPrice(ctx context.Context, itemID, eventType string, unitPrice float64, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	currentRows, err := queryTx(ctx, tx, `SELECT * FROM boq_items WHERE id=$1 AND status <> 'deleted' FOR UPDATE`, itemID)
	if err != nil {
		return nil, err
	}
	if len(currentRows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := currentRows[0]
	var updatedRows []Record
	var source string
	var fromPrice *float64
	switch eventType {
	case "expected_updated":
		if boolValue(current["norm_price_locked"]) || boolValue(current["final_price_locked"]) {
			return nil, validationError("Không thể sửa expected price sau khi đã lock norm/final", Record{"boq_item_id": itemID})
		}
		fromPrice = numberPtr(current["expected_unit_price"])
		updatedRows, err = queryTx(ctx, tx, `
			UPDATE boq_items
			SET expected_unit_price=$2, price_revision_no=price_revision_no+1, updated_at=now()
			WHERE id=$1
			RETURNING *
		`, itemID, unitPrice)
		source = "expected"
	case "norm_locked":
		if boolValue(current["norm_price_locked"]) {
			return nil, validationError("Dòng BOQ đã norm lock", Record{"boq_item_id": itemID})
		}
		if boolValue(current["final_price_locked"]) {
			return nil, validationError("Dòng BOQ đã final lock", Record{"boq_item_id": itemID})
		}
		fromPrice = numberPtr(current["expected_unit_price"])
		updatedRows, err = queryTx(ctx, tx, `
			UPDATE boq_items
			SET norm_unit_price=$2,
			    norm_price_locked=true,
			    norm_price_locked_by=NULLIF($3,'')::uuid,
			    norm_price_locked_at=now(),
			    norm_price_comment=$4,
			    price_revision_no=price_revision_no+1,
			    updated_at=now()
			WHERE id=$1
			RETURNING *
		`, itemID, unitPrice, actorID, comment)
		source = "norm"
	case "final_locked":
		if !boolValue(current["norm_price_locked"]) {
			return nil, validationError("Cần lock norm price trước khi lock final price", Record{"boq_item_id": itemID})
		}
		if boolValue(current["final_price_locked"]) {
			return nil, validationError("Dòng BOQ đã final lock", Record{"boq_item_id": itemID})
		}
		fromPrice = numberPtr(current["norm_unit_price"])
		updatedRows, err = queryTx(ctx, tx, `
			UPDATE boq_items
			SET final_unit_price=$2,
			    final_price_locked=true,
			    final_price_locked_by=NULLIF($3,'')::uuid,
			    final_price_locked_at=now(),
			    final_price_comment=$4,
			    price_revision_no=price_revision_no+1,
			    updated_at=now()
			WHERE id=$1
			RETURNING *
		`, itemID, unitPrice, actorID, comment)
		source = "final"
	default:
		return nil, validationError("Action đơn giá không hợp lệ", Record{"event_type": eventType})
	}
	if err != nil {
		return nil, err
	}
	if len(updatedRows) == 0 {
		return nil, sql.ErrNoRows
	}
	updated := updatedRows[0]
	if err := insertBOQUnitPriceEventTx(ctx, tx, updated, eventType, source, fromPrice, unitPrice, comment, actorID, actorName); err != nil {
		return nil, err
	}
	eventName := "boq_unit_price_" + eventType
	if err := addAuditTx(ctx, tx, fmt.Sprint(updated["project_id"]), actorID, actorName, eventName, "boq_item", itemID, "Cập nhật lifecycle đơn giá BOQ", Record{
		"price_source":      source,
		"unit_price":        unitPrice,
		"price_revision_no": updated["price_revision_no"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetBOQItemDetail(ctx, itemID)
}

func insertBOQUnitPriceEventTx(ctx context.Context, tx *sql.Tx, item Record, eventType, source string, fromPrice *float64, toPrice float64, comment, actorID, actorName string) error {
	snapshot := Record{
		"boq_item_id":           item["id"],
		"project_id":            item["project_id"],
		"quantity":              item["quantity"],
		"unit":                  item["unit"],
		"expected_unit_price":   item["expected_unit_price"],
		"norm_unit_price":       item["norm_unit_price"],
		"final_unit_price":      item["final_unit_price"],
		"effective_unit_price":  item["unit_price"],
		"unit_price_source":     item["unit_price_source"],
		"price_revision_no":     item["price_revision_no"],
		"norm_price_locked":     item["norm_price_locked"],
		"norm_price_locked_at":  item["norm_price_locked_at"],
		"final_price_locked":    item["final_price_locked"],
		"final_price_locked_at": item["final_price_locked_at"],
		"work_item_name":        item["work_item_name"],
		"category":              item["category"],
	}
	rawSnapshot, _ := json.Marshal(snapshot)
	var from any
	if fromPrice != nil {
		from = *fromPrice
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO boq_unit_price_events (
			project_id, boq_item_id, event_type, price_source, from_unit_price, to_unit_price,
			revision_no, comment, snapshot_json, actor_id, actor_name
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,NULLIF($10,'')::uuid,$11)
	`, item["project_id"], item["id"], eventType, source, from, toPrice, item["price_revision_no"], strings.TrimSpace(comment), string(rawSnapshot), actorID, actorName)
	return err
}

var finalLockedBOQProtectedFields = []string{
	"category",
	"work_item_name",
	"description",
	"unit",
	"quantity",
	"material_id",
	"node_id",
	"expected_unit_price",
}

func boqBodyChangesAny(current, body Record, fields ...string) bool {
	for _, field := range fields {
		next, ok := body[field]
		if !ok {
			continue
		}
		if boqFieldChanged(current[field], next) {
			return true
		}
	}
	return false
}

func boqFieldChanged(current any, next any) bool {
	switch next.(type) {
	case int, int64, float64, float32:
		return number(current) != number(next)
	}
	if _, ok := current.(float64); ok {
		return number(current) != number(next)
	}
	return strings.TrimSpace(fmt.Sprint(current)) != strings.TrimSpace(fmt.Sprint(next))
}

func numberPtr(value any) *float64 {
	out := number(value)
	return &out
}

func (r *Repository) GenerateProcurementFromBOQ(ctx context.Context, projectID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "generate_procurement:"+projectID); err != nil {
		return nil, err
	}

	blockers, err := queryTx(ctx, tx, `
		SELECT id, work_item_name, status, unit_price_source, expected_unit_price, norm_unit_price, final_unit_price, price_revision_no
		FROM boq_items b
		WHERE b.project_id=$1 AND b.status='approved' AND NOT b.final_price_locked
		  AND NOT EXISTS (
			SELECT 1
			FROM procurement_request_items pri
			JOIN procurement_requests pr ON pr.id=pri.request_id
			WHERE pri.boq_item_id=b.id
			  AND pr.status <> 'cancelled'
		  )
		ORDER BY b.category, b.created_at
	`, projectID)
	if err != nil {
		return nil, err
	}
	if len(blockers) > 0 {
		return nil, validationError("Chỉ sinh yêu cầu mua khi mọi dòng BOQ approved chưa sinh mua sắm đã final lock", Record{"items": blockers})
	}

	rows, err := queryTx(ctx, tx, `
		SELECT b.*, m.name AS material_name, m.standard_price, m.default_supplier, m.default_supplier_partner_id,
			COALESCE(b.final_unit_price, b.unit_price) AS effective_unit_price,
			(
				SELECT e.id
				FROM boq_unit_price_events e
				WHERE e.boq_item_id=b.id AND e.event_type='final_locked'
				ORDER BY e.created_at DESC
				LIMIT 1
			) AS final_price_event_id
		FROM boq_items b
		LEFT JOIN materials m ON m.id=b.material_id
		WHERE b.project_id=$1 AND b.status='approved' AND b.final_price_locked
		  AND NOT EXISTS (
			SELECT 1
			FROM procurement_request_items pri
			JOIN procurement_requests pr ON pr.id=pri.request_id
			WHERE pri.boq_item_id=b.id
			  AND pr.status <> 'cancelled'
		  )
	`, projectID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("no BOQ items available to generate procurement")
	}
	total := 0.0
	for _, row := range rows {
		price := number(row["effective_unit_price"])
		total += number(row["quantity"]) * price
	}
	requestCode := fmt.Sprintf("PR-%s-%d", projectID[:8], timeNowUnix())
	reqRows, err := queryTx(ctx, tx, `
		INSERT INTO procurement_requests
			(project_id, request_code, title, requested_by, status, total_estimated_amount, budget_status)
		VALUES ($1,$2,'Yêu cầu mua vật tư sinh từ BOQ',NULLIF($3,'')::uuid,'waiting_approval',$4,'within_budget')
		RETURNING *
	`, projectID, requestCode, actorID, total)
	if err != nil {
		return nil, err
	}
	if len(reqRows) == 0 {
		return nil, sql.ErrNoRows
	}
	requestID := fmt.Sprint(reqRows[0]["id"])
	for _, row := range rows {
		price := number(row["effective_unit_price"])
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO procurement_request_items
				(request_id, material_id, boq_item_id, quantity_planned, quantity_requested, unit, estimated_unit_price, supplier_partner_id, supplier_name, delivery_status, approval_status, boq_price_source, boq_price_revision_no, boq_price_event_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::uuid,$9,'pending','pending','final',$10,NULLIF($11,'')::uuid)
		`, requestID, row["material_id"], row["id"], row["quantity"], row["quantity"], row["unit"], price, cleanID(row["default_supplier_partner_id"]), row["default_supplier"], row["price_revision_no"], cleanID(row["final_price_event_id"])); err != nil {
			return nil, err
		}
	}
	rawMetadata, _ := json.Marshal(Record{"total": total, "boq_price_source": "final"})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json)
		VALUES ($1, NULLIF($2,'')::uuid, $3, 'material_requested', 'procurement_request', $4, 'Sinh yêu cầu mua vật tư từ BOQ', $5::jsonb)
	`, projectID, actorID, actorName, requestID, string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetProcurementDetail(ctx, requestID)
}

func (r *Repository) GetProcurementDetail(ctx context.Context, requestID string) (Record, error) {
	req, err := r.GetByID(ctx, "procurement_requests", requestID)
	if err != nil {
		return nil, err
	}
	items, _ := r.Query(ctx, `
		SELECT pri.*, m.name AS material_name, b.work_item_name
		FROM procurement_request_items pri
		LEFT JOIN materials m ON m.id=pri.material_id
		LEFT JOIN boq_items b ON b.id=pri.boq_item_id
		WHERE pri.request_id=$1
		ORDER BY pri.created_at
	`, requestID)
	req["items"] = items
	return req, nil
}

func (r *Repository) ListProjectProcurementRequests(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			pr.*,
			u.full_name AS requested_by_name,
			COALESCE(item_counts.item_count, 0) AS item_count
		FROM procurement_requests pr
		LEFT JOIN users u ON u.id=pr.requested_by
		LEFT JOIN (
			SELECT request_id, COUNT(*) AS item_count
			FROM procurement_request_items
			GROUP BY request_id
		) item_counts ON item_counts.request_id=pr.id
		WHERE pr.project_id=$1
		  AND pr.status <> 'cancelled'
		ORDER BY pr.created_at DESC, pr.id
	`, projectID)
}

func (r *Repository) ListProjectProcurementRequestItems(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			pri.*,
			pr.project_id,
			pr.request_code,
			pr.title AS procurement_request_title,
			m.code AS material_code,
			m.name AS material_name,
			b.work_item_name
		FROM procurement_request_items pri
		JOIN procurement_requests pr ON pr.id=pri.request_id
		LEFT JOIN materials m ON m.id=pri.material_id
		LEFT JOIN boq_items b ON b.id=pri.boq_item_id
		WHERE pr.project_id=$1
		  AND pr.status <> 'cancelled'
		ORDER BY pr.created_at DESC, pri.created_at, pri.id
	`, projectID)
}

func (r *Repository) ProcurementAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM procurement_requests WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	toStatus, event, err := procurementTransitionForAction(action, fromStatus)
	if err != nil {
		return nil, err
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE procurement_requests
		SET status=$2, updated_at=now()
		WHERE id=$1 AND status=$3
		RETURNING *
	`, id, toStatus, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái yêu cầu mua đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	if action == "approve" && cleanID(current["approval_request_id"]) != "" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE approval_requests
			SET status='approved', resolved_at=COALESCE(resolved_at, now())
			WHERE id=$1 AND project_id=$2 AND status IN ('pending','changes_requested')
		`, current["approval_request_id"], current["project_id"]); err != nil {
			return nil, err
		}
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": toStatus}
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, event, "procurement_request", id, fmt.Sprintf("Cập nhật yêu cầu mua từ %s sang %s", fromStatus, toStatus), metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetProcurementDetail(ctx, id)
}

func procurementTransitionForAction(action, fromStatus string) (string, string, error) {
	switch action {
	case "submit":
		if statusIn(fromStatus, "draft", "submitted", "changes_requested") {
			return "waiting_approval", "review_requested", nil
		}
	case "approve":
		if statusIn(fromStatus, "waiting_approval", "submitted") {
			return "approved", "material_approved", nil
		}
	case "order":
		if fromStatus == "approved" {
			return "ordered", "material_ordered", nil
		}
	case "receive":
		if fromStatus == "ordered" {
			return "received", "warehouse_received", nil
		}
	default:
		return "", "", validationError("Action yêu cầu mua không hợp lệ", Record{"action": action})
	}
	return "", "", validationError("Trạng thái yêu cầu mua không cho phép thao tác này", Record{"action": action, "status": fromStatus})
}

func actionComment(req ActionRequest) string {
	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		comment = strings.TrimSpace(req.Reason)
	}
	return comment
}

func (r *Repository) CreateStockMovement(ctx context.Context, req StockActionRequest, actorID, actorName string) (Record, error) {
	req.MovementType = strings.TrimSpace(req.MovementType)
	if req.MovementType == "" {
		req.MovementType = "receive"
	}
	if !statusIn(req.MovementType, "receive", "issue", "transfer", "loss", "return_to_warehouse", "return_to_subcontractor") {
		return nil, validationError("Loại nhập xuất kho không hợp lệ", Record{"movement_type": req.MovementType})
	}
	req.ProjectID = cleanID(req.ProjectID)
	req.WarehouseID = cleanID(req.WarehouseID)
	req.MaterialID = cleanID(req.MaterialID)
	req.ReferenceID = cleanID(req.ReferenceID)
	req.Unit = strings.TrimSpace(req.Unit)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.WarehouseID == "" {
		return nil, validationError("Kho là bắt buộc")
	}
	if req.MaterialID == "" {
		return nil, validationError("Vật tư là bắt buộc")
	}
	if req.Quantity <= 0 {
		return nil, validationError("Số lượng nhập xuất kho phải lớn hơn 0", Record{"quantity": req.Quantity})
	}
	if statusIn(req.MovementType, "issue", "return_to_warehouse", "return_to_subcontractor") && req.ProjectID == "" {
		return nil, validationError("Công trình là bắt buộc cho thao tác xuất/hoàn vật tư")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "stock:"+req.WarehouseID+":"+req.MaterialID); err != nil {
		return nil, err
	}
	if req.ProjectID != "" {
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "site-material:"+req.ProjectID+":"+req.MaterialID); err != nil {
			return nil, err
		}
	}

	warehouseRows, err := queryTx(ctx, tx, `SELECT id FROM warehouses WHERE id=$1 AND active`, req.WarehouseID)
	if err != nil {
		return nil, err
	}
	if len(warehouseRows) == 0 {
		return nil, validationError("Kho không tồn tại hoặc đã ngưng hoạt động", Record{"warehouse_id": req.WarehouseID})
	}
	materialRows, err := queryTx(ctx, tx, `SELECT id, unit FROM materials WHERE id=$1 AND active`, req.MaterialID)
	if err != nil {
		return nil, err
	}
	if len(materialRows) == 0 {
		return nil, validationError("Vật tư không tồn tại hoặc đã ngưng hoạt động", Record{"material_id": req.MaterialID})
	}
	if req.Unit == "" {
		req.Unit = strings.TrimSpace(fmt.Sprint(materialRows[0]["unit"]))
	}
	if req.Unit == "" {
		return nil, validationError("Đơn vị vật tư là bắt buộc")
	}

	if statusIn(req.MovementType, "issue", "transfer", "loss") {
		available, err := warehouseMaterialAvailableTx(ctx, tx, req.WarehouseID, req.MaterialID)
		if err != nil {
			return nil, err
		}
		if quantityExceedsAvailable(req.Quantity, available) {
			return nil, validationError("Tồn kho không đủ cho thao tác này", Record{"available": available, "quantity": req.Quantity})
		}
	}
	if statusIn(req.MovementType, "return_to_warehouse", "return_to_subcontractor") {
		available, err := projectMaterialAvailableTx(ctx, tx, req.ProjectID, req.MaterialID)
		if err != nil {
			return nil, err
		}
		if quantityExceedsAvailable(req.Quantity, available) {
			return nil, validationError("Vật tư tại công trình không đủ để hoàn/trả", Record{"available": available, "quantity": req.Quantity})
		}
	}

	code := fmt.Sprintf("SM-%s-%d-%s", req.MovementType, timeNowUnix(), newID()[:8])
	movements, err := queryTx(ctx, tx, `
		INSERT INTO stock_movements
			(movement_code, project_id, warehouse_id, material_id, movement_type, quantity, unit, reason, reference_type, reference_id, created_by, approved_by)
		VALUES ($1,NULLIF($2,'')::uuid,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,'')::uuid,NULLIF($11,'')::uuid,NULLIF($11,'')::uuid)
		RETURNING *
	`, code, req.ProjectID, req.WarehouseID, req.MaterialID, req.MovementType, req.Quantity, req.Unit, req.Reason, req.ReferenceType, req.ReferenceID, actorID)
	if err != nil {
		return nil, err
	}
	if len(movements) == 0 {
		return nil, sql.ErrNoRows
	}
	movement := movements[0]

	if req.ProjectID != "" && statusIn(req.MovementType, "issue", "return_to_warehouse", "return_to_subcontractor") {
		if req.MovementType == "issue" {
			if _, err := tx.ExecContext(ctx, `
			INSERT INTO site_material_balances (project_id, material_id, issued_quantity, surplus_quantity, surplus_status)
			VALUES ($1,$2,$3,GREATEST($3,0),'pending')
			ON CONFLICT (project_id, material_id)
			DO UPDATE SET issued_quantity = site_material_balances.issued_quantity + EXCLUDED.issued_quantity,
				surplus_quantity = GREATEST(site_material_balances.issued_quantity + EXCLUDED.issued_quantity - site_material_balances.used_quantity, 0),
				updated_at=now()
		`, req.ProjectID, req.MaterialID, req.Quantity); err != nil {
				return nil, err
			}
		} else {
			updated, err := queryTx(ctx, tx, `
				UPDATE site_material_balances
				SET issued_quantity = GREATEST(issued_quantity - $3, 0),
					surplus_quantity = GREATEST(GREATEST(issued_quantity - $3, 0) - used_quantity, 0),
					surplus_status = CASE
						WHEN GREATEST(GREATEST(issued_quantity - $3, 0) - used_quantity, 0) > 0.000001 THEN 'partial'
						ELSE 'handled'
					END,
					updated_at = now()
				WHERE project_id=$1 AND material_id=$2
				RETURNING *
			`, req.ProjectID, req.MaterialID, req.Quantity)
			if err != nil {
				return nil, err
			}
			if len(updated) == 0 {
				return nil, validationError("Không tìm thấy số dư vật tư tại công trình", Record{"project_id": req.ProjectID, "material_id": req.MaterialID})
			}
		}
	}

	metadata := Record{
		"movement_type": req.MovementType,
		"quantity":      req.Quantity,
		"material_id":   req.MaterialID,
		"warehouse_id":  req.WarehouseID,
	}
	if req.ReferenceType != "" {
		metadata["reference_type"] = req.ReferenceType
	}
	if req.ReferenceID != "" {
		metadata["reference_id"] = req.ReferenceID
	}
	if err := addAuditTx(ctx, tx, req.ProjectID, actorID, actorName, stockMovementEvent(req.MovementType), "stock_movement", fmt.Sprint(movement["id"]), stockMovementDescription(req), metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return movement, nil
}

func warehouseMaterialAvailableTx(ctx context.Context, tx *sql.Tx, warehouseID, materialID string) (float64, error) {
	rows, err := queryTx(ctx, tx, `
		SELECT COALESCE(SUM(CASE
			WHEN movement_type IN ('receive','return_to_warehouse') THEN quantity
			WHEN movement_type IN ('issue','transfer','loss') THEN -quantity
			ELSE 0
		END),0) AS available
		FROM stock_movements
		WHERE warehouse_id=$1 AND material_id=$2
	`, warehouseID, materialID)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return number(rows[0]["available"]), nil
}

func projectMaterialAvailableTx(ctx context.Context, tx *sql.Tx, projectID, materialID string) (float64, error) {
	rows, err := queryTx(ctx, tx, `
		SELECT GREATEST(issued_quantity - used_quantity, surplus_quantity, 0) AS available
		FROM site_material_balances
		WHERE project_id=$1 AND material_id=$2
		FOR UPDATE
	`, projectID, materialID)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return number(rows[0]["available"]), nil
}

func quantityExceedsAvailable(quantity, available float64) bool {
	return quantity > available+0.000001
}

func stockMovementEvent(movementType string) string {
	switch movementType {
	case "receive":
		return "warehouse_received"
	case "issue":
		return "warehouse_issued"
	case "transfer":
		return "warehouse_transferred"
	case "loss":
		return "warehouse_loss_recorded"
	case "return_to_warehouse":
		return "surplus_stored"
	case "return_to_subcontractor":
		return "surplus_returned_to_subcontractor"
	default:
		return "stock_movement_created"
	}
}

func stockMovementDescription(req StockActionRequest) string {
	if req.Reason != "" {
		return req.Reason
	}
	return fmt.Sprintf("Ghi nhận %s %.3f %s", req.MovementType, req.Quantity, req.Unit)
}

func (r *Repository) ProjectMaterialBalances(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT smb.*, m.code, m.name AS material_name, m.unit, m.category
		FROM site_material_balances smb
		JOIN materials m ON m.id=smb.material_id
		WHERE smb.project_id=$1 AND m.active
		ORDER BY m.category, m.name
	`, projectID)
}

func (r *Repository) ListActiveMaterials(ctx context.Context) ([]Record, error) {
	return r.Query(ctx, `
		SELECT m.id, m.code, m.name, m.unit, m.category, m.default_supplier_partner_id, m.default_supplier,
			p.name AS default_supplier_partner_name, p.tax_code AS default_supplier_tax_code,
			m.standard_price, m.active, m.created_at
		FROM materials m
		LEFT JOIN partners p ON p.id=m.default_supplier_partner_id
		WHERE m.active
		ORDER BY m.category, m.name, m.code
	`)
}

func (r *Repository) ListActiveWarehouses(ctx context.Context) ([]Record, error) {
	return r.Query(ctx, `
		SELECT id, name, location, type, active, created_at
		FROM warehouses
		WHERE active
		ORDER BY type, name
	`)
}

func (r *Repository) CreateWarehouse(ctx context.Context, body Record) (Record, error) {
	return r.Insert(ctx, "warehouses", warehouseColumns(), body)
}

func (r *Repository) ListProjectDailyReports(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT *
		FROM operation_daily_reports
		WHERE project_id=$1
		ORDER BY created_at DESC
	`, projectID)
}

func (r *Repository) CreateDailyReport(ctx context.Context, body Record) (Record, error) {
	return r.Insert(ctx, "operation_daily_reports", dailyReportColumns(), body)
}

func (r *Repository) ListProjectIssues(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT *
		FROM issues
		WHERE project_id=$1 AND status <> 'deleted'
		ORDER BY created_at DESC
	`, projectID)
}

func (r *Repository) CreateIssue(ctx context.Context, body Record) (Record, error) {
	return r.Insert(ctx, "issues", issueColumns(), body)
}

func (r *Repository) UpdateIssue(ctx context.Context, id string, body Record) (Record, error) {
	return r.Update(ctx, "issues", issueColumns(), id, body)
}

func (r *Repository) ListProjectReworks(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT *
		FROM rework_requests
		WHERE project_id=$1 AND status <> 'deleted'
		ORDER BY created_at DESC
	`, projectID)
}

func (r *Repository) CreateRework(ctx context.Context, body Record) (Record, error) {
	return r.Insert(ctx, "rework_requests", reworkColumns(), body)
}

func (r *Repository) UpdateReworkStatus(ctx context.Context, id, status, actorID string) (Record, error) {
	return r.QueryOne(ctx, `
		UPDATE rework_requests
		SET status=$2,
			approved_by=CASE
				WHEN $2='approved' THEN NULLIF($3,'')::uuid
				WHEN $2='done' THEN COALESCE(approved_by, NULLIF($3,'')::uuid)
				ELSE approved_by
			END,
			updated_at=now()
		WHERE id=$1
		RETURNING *
	`, id, status, actorID)
}

func (r *Repository) ListProjectReplannings(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT *
		FROM replanning_requests
		WHERE project_id=$1 AND status <> 'deleted'
		ORDER BY created_at DESC
	`, projectID)
}

func (r *Repository) CreateReplanning(ctx context.Context, body Record) (Record, error) {
	return r.Insert(ctx, "replanning_requests", replanningColumns(), body)
}

func (r *Repository) ApproveReplanning(ctx context.Context, id, actorID string) (Record, error) {
	return r.QueryOne(ctx, `
		UPDATE replanning_requests
		SET status='approved', approved_by=NULLIF($2,'')::uuid, updated_at=now()
		WHERE id=$1
		RETURNING *
	`, id, actorID)
}

func (r *Repository) ListProjectHandover(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT *
		FROM handover_records
		WHERE project_id=$1 AND status <> 'deleted'
		ORDER BY created_at DESC
	`, projectID)
}

func (r *Repository) CreateHandover(ctx context.Context, body Record) (Record, error) {
	return r.Insert(ctx, "handover_records", handoverColumns(), body)
}

func (r *Repository) ListProjectBudget(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT *
		FROM accounting_budgets
		WHERE project_id=$1 AND status <> 'cancelled'
		ORDER BY created_at DESC
	`, projectID)
}

func (r *Repository) CreateBudget(ctx context.Context, body Record) (Record, error) {
	return r.Insert(ctx, "accounting_budgets", budgetColumns(), body)
}

func (r *Repository) ApproveBudget(ctx context.Context, id, actorID string) (Record, error) {
	return r.QueryOne(ctx, `
		UPDATE accounting_budgets
		SET status='approved', approved_by=NULLIF($2,'')::uuid, updated_at=now()
		WHERE id=$1
		RETURNING *
	`, id, actorID)
}

func (r *Repository) CreatePayment(ctx context.Context, body Record) (Record, error) {
	return r.Insert(ctx, "payments", paymentColumns(), body)
}

func (r *Repository) AcceptanceAction(ctx context.Context, id, result string, req ActionRequest, actorID, actorName string) (Record, error) {
	result = strings.TrimSpace(result)
	if !statusIn(result, "pass", "fail", "conditional_pass") {
		return nil, validationError("Kết quả nghiệm thu không hợp lệ", Record{"result": result})
	}
	event := "acceptance_passed"
	if result == "fail" {
		event = "acceptance_failed"
	}
	if result == "conditional_pass" {
		event = "acceptance_conditionally_passed"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM acceptance_records WHERE id=$1 AND status <> 'deleted' FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	if statusIn(fromStatus, "completed", "cancelled") {
		return nil, validationError("Nghiệm thu đã được chốt hoặc không còn hợp lệ", Record{"status": fromStatus})
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE acceptance_records
		SET status='completed', result=$2, approved_by=NULLIF($3,'')::uuid, updated_at=now()
		WHERE id=$1 AND status=$4
		RETURNING *
	`, id, result, actorID, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái nghiệm thu đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	acc := updated[0]
	projectID := fmt.Sprint(acc["project_id"])
	comment := actionComment(req)
	metadata := Record{"result": result, "from_status": fromStatus, "to_status": "completed"}
	if comment != "" {
		metadata["comment"] = comment
	}
	if result == "fail" {
		issueRows, err := queryTx(ctx, tx, `
			INSERT INTO issues
				(project_id, node_id, title, description, severity, status, reported_by, requires_rework, requires_replanning)
			VALUES ($1,$2,$3,$4,'high','open',NULLIF($5,'')::uuid,true,false)
			RETURNING *
		`, projectID, acc["node_id"], fmt.Sprintf("Punch list từ nghiệm thu: %s", acc["title"]), acc["notes"], actorID)
		if err != nil {
			return nil, err
		}
		if len(issueRows) > 0 {
			if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "rework_requested", "issue", fmt.Sprint(issueRows[0]["id"]), "Nghiệm thu fail tạo punch list/rework", Record{"acceptance_id": id}); err != nil {
				return nil, err
			}
			metadata["issue_id"] = issueRows[0]["id"]
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, event, "acceptance", id, fmt.Sprintf("Nghiệm thu %s: %s", result, acc["title"]), metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return acc, nil
}

func (r *Repository) CreateReworkFromAcceptance(ctx context.Context, acceptanceID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	accRows, err := queryTx(ctx, tx, `SELECT * FROM acceptance_records WHERE id=$1 AND status <> 'deleted' FOR UPDATE`, acceptanceID)
	if err != nil {
		return nil, err
	}
	if len(accRows) == 0 {
		return nil, sql.ErrNoRows
	}
	acc := accRows[0]
	if fmt.Sprint(acc["status"]) != "completed" || fmt.Sprint(acc["result"]) != "fail" {
		return nil, validationError("Chỉ tạo rework từ nghiệm thu đã fail", Record{"status": acc["status"], "result": acc["result"]})
	}
	existing, err := queryTx(ctx, tx, `
		SELECT COUNT(*) AS total
		FROM rework_requests
		WHERE acceptance_id=$1 AND status NOT IN ('cancelled','done','deleted')
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 && number(existing[0]["total"]) > 0 {
		return nil, validationError("Nghiệm thu này đã có rework đang mở", Record{"acceptance_id": acceptanceID})
	}
	reworkRows, err := queryTx(ctx, tx, `
		INSERT INTO rework_requests
			(project_id, acceptance_id, node_id, title, description, status, created_by, approved_by)
		VALUES ($1,$2,$3,$4,$5,'approved',NULLIF($6,'')::uuid,NULLIF($6,'')::uuid)
		RETURNING *
	`, acc["project_id"], acceptanceID, acc["node_id"], fmt.Sprintf("Rework sau nghiệm thu: %s", acc["title"]), acc["notes"], actorID)
	if err != nil {
		return nil, err
	}
	if len(reworkRows) == 0 {
		return nil, sql.ErrNoRows
	}
	rework := reworkRows[0]
	if err := addAuditTx(ctx, tx, fmt.Sprint(acc["project_id"]), actorID, actorName, "rework_requested", "rework_request", fmt.Sprint(rework["id"]), "Tạo rework request từ nghiệm thu fail", Record{"acceptance_id": acceptanceID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rework, nil
}

func (r *Repository) HandoverCheck(ctx context.Context, id string) (Record, error) {
	handover, err := r.GetByID(ctx, "handover_records", id)
	if err != nil {
		return nil, err
	}
	projectID := fmt.Sprint(handover["project_id"])
	blockers := []string{}
	finalPass, _ := r.Query(ctx, `
		SELECT COUNT(*) AS total FROM acceptance_records
		WHERE project_id=$1 AND type='final' AND status='completed' AND result IN ('pass','conditional_pass')
	`, projectID)
	if len(finalPass) == 0 || number(finalPass[0]["total"]) == 0 {
		blockers = append(blockers, "Chưa có nghiệm thu toàn phần đạt")
	}
	openIssues, _ := r.Query(ctx, `SELECT COUNT(*) AS total FROM issues WHERE project_id=$1 AND status IN ('open','in_progress')`, projectID)
	if len(openIssues) > 0 && number(openIssues[0]["total"]) > 0 {
		blockers = append(blockers, "Còn punch list/issue chưa đóng")
	}
	pendingSurplus, _ := r.Query(ctx, `SELECT COUNT(*) AS total FROM site_material_balances WHERE project_id=$1 AND surplus_quantity > 0 AND surplus_status <> 'handled'`, projectID)
	if len(pendingSurplus) > 0 && number(pendingSurplus[0]["total"]) > 0 {
		blockers = append(blockers, "Còn vật tư dư chưa xử lý")
	}
	if handover["documents_ready"] != true {
		blockers = append(blockers, "Chứng từ bàn giao chưa đủ")
	}
	if handover["warehouse_closed"] != true {
		blockers = append(blockers, "Kho công trình chưa closure")
	}
	if handover["payables_recorded"] != true {
		blockers = append(blockers, "Công nợ đầu vào chưa ghi nhận")
	}
	if handover["receivables_recorded"] != true {
		blockers = append(blockers, "Công nợ đầu ra chưa ghi nhận")
	}
	openPayables, _ := r.Query(ctx, `SELECT COUNT(*) AS total, COALESCE(SUM(remaining_amount),0) AS amount FROM payables WHERE project_id=$1 AND status <> 'cancelled' AND remaining_amount > 0`, projectID)
	if len(openPayables) > 0 && number(openPayables[0]["total"]) > 0 {
		blockers = append(blockers, "Còn công nợ đầu vào chưa tất toán")
	}
	openReceivables, _ := r.Query(ctx, `SELECT COUNT(*) AS total, COALESCE(SUM(remaining_amount),0) AS amount FROM receivables WHERE project_id=$1 AND status <> 'cancelled' AND remaining_amount > 0`, projectID)
	if len(openReceivables) > 0 && number(openReceivables[0]["total"]) > 0 {
		blockers = append(blockers, "Còn công nợ đầu ra chưa thu")
	}
	pendingPayments, _ := r.Query(ctx, `SELECT COUNT(*) AS total FROM payments WHERE project_id=$1 AND status NOT IN ('approved','paid','received','cancelled')`, projectID)
	if len(pendingPayments) > 0 && number(pendingPayments[0]["total"]) > 0 {
		blockers = append(blockers, "Còn phiếu chi/thu chưa duyệt")
	}
	handover["blockers"] = blockers
	handover["ready"] = len(blockers) == 0
	return handover, nil
}

func (r *Repository) CompleteHandover(ctx context.Context, id string, req ActionRequest, actorID, actorName string) (Record, error) {
	check, err := r.HandoverCheck(ctx, id)
	if err != nil {
		return nil, err
	}
	blockers, _ := check["blockers"].([]string)
	comment := actionComment(req)
	if len(blockers) > 0 {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()
		updated, err := queryTx(ctx, tx, `UPDATE handover_records SET status='blocked', updated_at=now() WHERE id=$1 RETURNING *`, id)
		if err != nil {
			return nil, err
		}
		if len(updated) == 0 {
			return nil, sql.ErrNoRows
		}
		metadata := Record{"blockers": blockers}
		if comment != "" {
			metadata["comment"] = comment
		}
		if err := addAuditTx(ctx, tx, fmt.Sprint(updated[0]["project_id"]), actorID, actorName, "handover_blocked", "handover", id, "Bàn giao bị chặn do còn điều kiện chưa đạt", metadata); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		for key, value := range updated[0] {
			check[key] = value
		}
		check["blockers"] = blockers
		check["ready"] = false
		return check, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	updated, err := queryTx(ctx, tx, `
		UPDATE handover_records
		SET status='handed_over',
			handover_date=COALESCE(handover_date, CURRENT_DATE),
			approved_by=NULLIF($2,'')::uuid,
			updated_at=now()
		WHERE id=$1 AND status NOT IN ('handed_over','cancelled','deleted')
		RETURNING *
	`, id, actorID)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Bàn giao đã hoàn tất hoặc không còn hợp lệ")
	}
	metadata := Record{}
	if comment != "" {
		metadata["comment"] = comment
	}
	done := updated[0]
	if err := addAuditTx(ctx, tx, fmt.Sprint(done["project_id"]), actorID, actorName, "handover_completed", "handover", id, "Hoàn tất bàn giao công trình", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return done, nil
}

func (r *Repository) PayableAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM payables WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	toStatus := ""
	event := ""
	metadata := Record{"action": action, "from_status": fromStatus}
	var updated []Record
	switch action {
	case "approve":
		if !statusIn(fromStatus, "draft", "submitted", "recorded", "overdue") {
			return nil, validationError("Trạng thái phải trả không cho phép duyệt", Record{"status": fromStatus})
		}
		toStatus = "approved"
		event = "payable_approved"
		updated, err = queryTx(ctx, tx, `
			UPDATE payables
			SET status='approved', approved_by=NULLIF($2,'')::uuid, updated_at=now()
			WHERE id=$1 AND status=$3
			RETURNING *
		`, id, actorID, fromStatus)
	case "pay":
		if !statusIn(fromStatus, "approved", "partially_paid", "overdue") {
			return nil, validationError("Phải trả cần được duyệt trước khi tất toán", Record{"status": fromStatus})
		}
		remaining := number(current["remaining_amount"])
		if remaining <= 0 {
			return nil, validationError("Phải trả không còn số tiền cần tất toán")
		}
		toStatus = "paid"
		event = "payable_paid"
		metadata["amount"] = remaining
		updated, err = queryTx(ctx, tx, `
			UPDATE payables
			SET status='paid',
				paid_amount=paid_amount + $2,
				remaining_amount=0,
				approved_by=COALESCE(approved_by,NULLIF($3,'')::uuid),
				updated_at=now()
			WHERE id=$1 AND status=$4
			RETURNING *
		`, id, remaining, actorID, fromStatus)
	default:
		return nil, validationError("Action phải trả không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái phải trả đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	comment := actionComment(req)
	metadata["to_status"] = toStatus
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, event, "payable", id, fmt.Sprintf("Cập nhật phải trả từ %s sang %s", fromStatus, toStatus), metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated[0], nil
}

func (r *Repository) GetPayableDetail(ctx context.Context, payableID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT pay.*, p.name AS project_name
		FROM payables pay
		JOIN projects p ON p.id=pay.project_id
		WHERE pay.id=$1
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, payableID)
}

func (r *Repository) PayableReconciliation(ctx context.Context, payableID string) (Record, error) {
	summary, err := r.QueryOne(ctx, `
		WITH pay AS (
			SELECT pay.*, p.name AS project_name
			FROM payables pay
			JOIN projects p ON p.id=pay.project_id
			WHERE pay.id=$1
			  AND pay.status <> 'cancelled'
			  AND p.status <> 'cancelled'
		),
		inv AS (
			SELECT
				payable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('deleted','cancelled')) AS invoice_count,
				COUNT(*) FILTER (WHERE status='verified') AS verified_invoice_count,
				COUNT(*) FILTER (WHERE status='rejected') AS rejected_invoice_count,
				COALESCE(SUM(total_amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS invoice_total,
				COALESCE(SUM(total_amount) FILTER (WHERE status='verified'), 0) AS verified_invoice_total,
				MAX(invoice_date) FILTER (WHERE status NOT IN ('deleted','cancelled')) AS latest_invoice_date
			FROM input_invoices
			WHERE payable_id=$1
			GROUP BY payable_id
		),
		sch AS (
			SELECT
				payable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('cancelled','deleted')) AS schedule_count,
				COUNT(*) FILTER (WHERE status IN ('approved','partially_paid','overdue') AND remaining_amount > 0 AND due_date < CURRENT_DATE) AS overdue_schedule_count,
				COALESCE(SUM(planned_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS scheduled_total,
				COALESCE(SUM(paid_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS scheduled_paid_total,
				COALESCE(SUM(remaining_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS scheduled_remaining,
				MIN(due_date) FILTER (WHERE status IN ('approved','partially_paid','overdue') AND remaining_amount > 0) AS next_schedule_due_date
			FROM payable_payment_schedules
			WHERE payable_id=$1
			GROUP BY payable_id
		),
		adv AS (
			SELECT
				payable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('cancelled','deleted')) AS advance_request_count,
				COALESCE(SUM(amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_requested_total,
				COALESCE(SUM(amount) FILTER (WHERE status IN ('approved','partially_settled','settled')), 0) AS advance_approved_total,
				COALESCE(SUM(paid_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_paid_total,
				COALESCE(SUM(settled_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_settled_total
			FROM advance_requests
			WHERE payable_id=$1
			GROUP BY payable_id
		),
		alloc AS (
			SELECT
				pa.payable_id,
				COALESCE(SUM(pa.amount) FILTER (WHERE pmt.status='approved' AND pa.allocation_type='payable_payment'), 0) AS approved_payable_payment_total,
				COALESCE(SUM(pa.amount) FILTER (WHERE pmt.status='approved' AND pa.allocation_type='advance_payment'), 0) AS approved_advance_payment_total,
				COALESCE(SUM(pa.amount) FILTER (WHERE pmt.status <> 'cancelled'), 0) AS payment_allocation_total,
				COUNT(*) FILTER (WHERE pmt.status NOT IN ('approved','cancelled')) AS pending_payment_count
			FROM payment_allocations pa
			JOIN payments pmt ON pmt.id=pa.payment_id
			WHERE pa.payable_id=$1
			GROUP BY pa.payable_id
		),
		calc AS (
			SELECT
				pay.*,
				COALESCE(inv.invoice_count, 0) AS invoice_count,
				COALESCE(inv.verified_invoice_count, 0) AS verified_invoice_count,
				COALESCE(inv.rejected_invoice_count, 0) AS rejected_invoice_count,
				COALESCE(inv.invoice_total, 0) AS invoice_total,
				COALESCE(inv.verified_invoice_total, 0) AS verified_invoice_total,
				inv.latest_invoice_date,
				COALESCE(sch.schedule_count, 0) AS schedule_count,
				COALESCE(sch.overdue_schedule_count, 0) AS overdue_schedule_count,
				COALESCE(sch.scheduled_total, 0) AS scheduled_total,
				COALESCE(sch.scheduled_paid_total, 0) AS scheduled_paid_total,
				COALESCE(sch.scheduled_remaining, 0) AS scheduled_remaining,
				sch.next_schedule_due_date,
				COALESCE(adv.advance_request_count, 0) AS advance_request_count,
				COALESCE(adv.advance_requested_total, 0) AS advance_requested_total,
				COALESCE(adv.advance_approved_total, 0) AS advance_approved_total,
				COALESCE(adv.advance_paid_total, 0) AS advance_paid_total,
				COALESCE(adv.advance_settled_total, 0) AS advance_settled_total,
				COALESCE(alloc.approved_payable_payment_total, 0) AS approved_payable_payment_total,
				COALESCE(alloc.approved_advance_payment_total, 0) AS approved_advance_payment_total,
				COALESCE(alloc.payment_allocation_total, 0) AS payment_allocation_total,
				COALESCE(alloc.pending_payment_count, 0) AS pending_payment_count,
				COALESCE(pay.due_date, sch.next_schedule_due_date, inv.latest_invoice_date) AS effective_due_date,
				GREATEST(COALESCE(pay.gross_amount,0) - COALESCE(pay.advance_amount,0) - COALESCE(pay.paid_amount,0) - COALESCE(pay.deduction_amount,0), 0) AS computed_remaining
			FROM pay
			LEFT JOIN inv ON inv.payable_id=pay.id
			LEFT JOIN sch ON sch.payable_id=pay.id
			LEFT JOIN adv ON adv.payable_id=pay.id
			LEFT JOIN alloc ON alloc.payable_id=pay.id
		)
		SELECT
			calc.*,
			CASE
				WHEN COALESCE(remaining_amount,0) <= 0 THEN 0
				WHEN effective_due_date IS NULL THEN 0
				WHEN effective_due_date >= CURRENT_DATE THEN 0
				ELSE CURRENT_DATE - effective_due_date
			END AS overdue_days,
			CASE
				WHEN COALESCE(remaining_amount,0) <= 0 THEN 'settled'
				WHEN effective_due_date IS NULL THEN 'no_due_date'
				WHEN effective_due_date >= CURRENT_DATE THEN 'not_due'
				WHEN CURRENT_DATE - effective_due_date <= 30 THEN '1_30'
				WHEN CURRENT_DATE - effective_due_date <= 60 THEN '31_60'
				WHEN CURRENT_DATE - effective_due_date <= 90 THEN '61_90'
				ELSE '90_plus'
			END AS aging_bucket,
			ABS(COALESCE(remaining_amount,0) - COALESCE(computed_remaining,0)) AS balance_difference,
			GREATEST(COALESCE(gross_amount,0) - COALESCE(verified_invoice_total,0), 0) AS unverified_invoice_gap,
			GREATEST(COALESCE(advance_approved_total,0) - COALESCE(advance_paid_total,0), 0) AS advance_unpaid_total,
			GREATEST(COALESCE(advance_paid_total,0) - COALESCE(advance_settled_total,0), 0) AS advance_unsettled_total
		FROM calc
	`, payableID)
	if err != nil {
		return nil, err
	}
	warnings := payableReconciliationWarnings(summary)
	summary["warning_count"] = len(warnings)
	if len(warnings) > 0 {
		summary["reconciliation_status"] = "needs_review"
	} else {
		summary["reconciliation_status"] = "ok"
	}

	ledger, err := r.Query(ctx, `
		WITH entries AS (
			SELECT
				pay.created_at AS occurred_at,
				0 AS sort_order,
				'payable_recorded'::text AS entry_type,
				pay.id::text AS source_id,
				pay.payable_code::text AS source_code,
				pay.status::text AS status,
				'Ghi nhận phải trả'::text AS description,
				COALESCE(pay.gross_amount,0)::numeric AS debit_amount,
				0::numeric AS credit_amount,
				0::numeric AS document_amount,
				0::numeric AS cash_amount,
				COALESCE(created.full_name, '') AS actor_name
			FROM payables pay
			LEFT JOIN users created ON created.id=pay.created_by
			WHERE pay.id=$1 AND pay.status <> 'cancelled'

			UNION ALL

			SELECT
				COALESCE(ii.verified_at, ii.rejected_at, ii.created_at) AS occurred_at,
				10 AS sort_order,
				('input_invoice_' || ii.status)::text AS entry_type,
				ii.id::text AS source_id,
				ii.invoice_number::text AS source_code,
				ii.status::text AS status,
				CASE
					WHEN ii.status='verified' THEN 'Hoá đơn đầu vào đã xác thực'
					WHEN ii.status='rejected' THEN 'Hoá đơn đầu vào bị từ chối'
					ELSE 'Upload hoá đơn đầu vào'
				END AS description,
				0::numeric AS debit_amount,
				0::numeric AS credit_amount,
				COALESCE(ii.total_amount,0)::numeric AS document_amount,
				0::numeric AS cash_amount,
				COALESCE(actor.full_name, '') AS actor_name
			FROM input_invoices ii
			LEFT JOIN users actor ON actor.id=COALESCE(ii.verified_by, ii.rejected_by, ii.uploaded_by)
			WHERE ii.payable_id=$1 AND ii.status NOT IN ('deleted','cancelled')

			UNION ALL

			SELECT
				ar.updated_at AS occurred_at,
				20 AS sort_order,
				'advance_approved'::text AS entry_type,
				ar.id::text AS source_id,
				ar.request_code::text AS source_code,
				ar.status::text AS status,
				'Ứng trước được duyệt ghi giảm phải trả'::text AS description,
				0::numeric AS debit_amount,
				COALESCE(ar.amount,0)::numeric AS credit_amount,
				0::numeric AS document_amount,
				0::numeric AS cash_amount,
				COALESCE(approved.full_name, '') AS actor_name
			FROM advance_requests ar
			LEFT JOIN users approved ON approved.id=ar.approved_by
			WHERE ar.payable_id=$1 AND ar.status IN ('approved','partially_settled','settled')

			UNION ALL

			SELECT
				COALESCE(pmt.paid_at, pmt.updated_at, pmt.created_at) AS occurred_at,
				30 AS sort_order,
				('payment_' || pa.allocation_type)::text AS entry_type,
				pmt.id::text AS source_id,
				pmt.payment_code::text AS source_code,
				pmt.status::text AS status,
				CASE
					WHEN pa.allocation_type='advance_payment' THEN 'Chi tiền ứng trước'
					WHEN pa.allocation_type='advance_settlement' THEN 'Quyết toán ứng trước'
					ELSE 'Thanh toán phải trả'
				END AS description,
				0::numeric AS debit_amount,
				CASE WHEN pa.allocation_type='payable_payment' THEN COALESCE(pa.amount,0)::numeric ELSE 0::numeric END AS credit_amount,
				0::numeric AS document_amount,
				COALESCE(pa.amount,0)::numeric AS cash_amount,
				COALESCE(approved.full_name, created.full_name, '') AS actor_name
			FROM payment_allocations pa
			JOIN payments pmt ON pmt.id=pa.payment_id
			LEFT JOIN users approved ON approved.id=pmt.approved_by
			LEFT JOIN users created ON created.id=pmt.created_by
			WHERE pa.payable_id=$1 AND pmt.status='approved'

			UNION ALL

			SELECT
				COALESCE(ar.settled_at, ar.updated_at) AS occurred_at,
				40 AS sort_order,
				'advance_settled'::text AS entry_type,
				ar.id::text AS source_id,
				ar.request_code::text AS source_code,
				ar.status::text AS status,
				'Quyết toán ứng trước bằng chứng từ'::text AS description,
				0::numeric AS debit_amount,
				0::numeric AS credit_amount,
				COALESCE(ar.settled_amount,0)::numeric AS document_amount,
				0::numeric AS cash_amount,
				''::text AS actor_name
			FROM advance_requests ar
			WHERE ar.payable_id=$1
			  AND ar.status NOT IN ('cancelled','deleted')
			  AND COALESCE(ar.settled_amount,0) > 0
		),
		running AS (
			SELECT
				entries.*,
				SUM(debit_amount - credit_amount) OVER (
					ORDER BY occurred_at ASC, sort_order ASC, source_id ASC
					ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
				) AS running_balance
			FROM entries
		)
		SELECT *
		FROM running
		ORDER BY occurred_at DESC, sort_order DESC, source_id DESC
	`, payableID)
	if err != nil {
		return nil, err
	}

	return Record{
		"summary":  summary,
		"warnings": warnings,
		"ledger":   ledger,
	}, nil
}

func payableReconciliationWarnings(summary Record) []Record {
	warnings := []Record{}
	if number(summary["remaining_amount"]) > 0 && int(number(summary["overdue_days"])) > 0 {
		severity := "warning"
		if number(summary["overdue_days"]) > 60 {
			severity = "critical"
		}
		warnings = append(warnings, payableWarning("overdue", severity, "Khoản phải trả đã quá hạn", Record{
			"effective_due_date": summary["effective_due_date"],
			"overdue_days":       summary["overdue_days"],
			"remaining_amount":   summary["remaining_amount"],
		}))
	}
	if number(summary["verified_invoice_total"]) > number(summary["gross_amount"])+1 {
		warnings = append(warnings, payableWarning("invoice_over_gross", "critical", "Tổng hoá đơn verified vượt giá trị phải trả", Record{
			"gross_amount":           summary["gross_amount"],
			"verified_invoice_total": summary["verified_invoice_total"],
		}))
	}
	if number(summary["paid_amount"]) > number(summary["verified_invoice_total"])+1 {
		warnings = append(warnings, payableWarning("paid_without_verified_invoice", "warning", "Đã thanh toán nhiều hơn tổng hoá đơn verified", Record{
			"paid_amount":            summary["paid_amount"],
			"verified_invoice_total": summary["verified_invoice_total"],
		}))
	}
	if number(summary["remaining_amount"]) > 0 && int(number(summary["schedule_count"])) == 0 {
		warnings = append(warnings, payableWarning("missing_schedule", "warning", "Khoản phải trả còn dư nhưng chưa có lịch thanh toán", Record{
			"remaining_amount": summary["remaining_amount"],
		}))
	}
	if number(summary["scheduled_remaining"]) > number(summary["remaining_amount"])+1 {
		warnings = append(warnings, payableWarning("schedule_over_remaining", "warning", "Số còn lại theo lịch lớn hơn số còn phải trả", Record{
			"scheduled_remaining": summary["scheduled_remaining"],
			"remaining_amount":    summary["remaining_amount"],
		}))
	}
	if number(summary["advance_unpaid_total"]) > 1 {
		warnings = append(warnings, payableWarning("advance_unpaid", "warning", "Có ứng trước đã duyệt nhưng chưa chi đủ", Record{
			"advance_approved_total": summary["advance_approved_total"],
			"advance_paid_total":     summary["advance_paid_total"],
			"advance_unpaid_total":   summary["advance_unpaid_total"],
		}))
	}
	if number(summary["advance_unsettled_total"]) > 1 {
		warnings = append(warnings, payableWarning("advance_unsettled", "warning", "Có ứng trước đã chi nhưng chưa quyết toán đủ", Record{
			"advance_paid_total":      summary["advance_paid_total"],
			"advance_settled_total":   summary["advance_settled_total"],
			"advance_unsettled_total": summary["advance_unsettled_total"],
		}))
	}
	if number(summary["balance_difference"]) > 1 {
		warnings = append(warnings, payableWarning("balance_difference", "critical", "Số còn phải trả lệch với công thức gross - ứng trước - đã trả - khấu trừ", Record{
			"remaining_amount":   summary["remaining_amount"],
			"computed_remaining": summary["computed_remaining"],
			"balance_difference": summary["balance_difference"],
		}))
	}
	if int(number(summary["pending_payment_count"])) > 0 {
		warnings = append(warnings, payableWarning("pending_payment", "info", "Có phiếu thanh toán đang chờ duyệt", Record{
			"pending_payment_count": summary["pending_payment_count"],
		}))
	}
	return warnings
}

func payableWarning(code, severity, message string, details Record) Record {
	return Record{
		"code":     code,
		"severity": severity,
		"message":  message,
		"details":  details,
	}
}

func (r *Repository) ListPayablePaymentSchedules(ctx context.Context, payableID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			pps.*,
			p.name AS project_name,
			pay.payable_code,
			pay.vendor_name,
			COALESCE(created.full_name, '') AS created_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM payable_payment_schedules pps
		JOIN projects p ON p.id=pps.project_id
		JOIN payables pay ON pay.id=pps.payable_id
		LEFT JOIN users created ON created.id=pps.created_by
		LEFT JOIN users approved ON approved.id=pps.approved_by
		WHERE pps.payable_id=$1
		  AND pps.status NOT IN ('cancelled','deleted')
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY pps.due_date NULLS LAST, pps.created_at DESC
	`, payableID)
}

func (r *Repository) GetPayablePaymentScheduleDetail(ctx context.Context, scheduleID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			pps.*,
			p.name AS project_name,
			pay.payable_code,
			pay.vendor_name,
			COALESCE(created.full_name, '') AS created_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM payable_payment_schedules pps
		JOIN projects p ON p.id=pps.project_id
		JOIN payables pay ON pay.id=pps.payable_id
		LEFT JOIN users created ON created.id=pps.created_by
		LEFT JOIN users approved ON approved.id=pps.approved_by
		WHERE pps.id=$1
		  AND pps.status NOT IN ('cancelled','deleted')
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, scheduleID)
}

func (r *Repository) CreatePayablePaymentSchedule(ctx context.Context, payable Record, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	scheduleID := newID()
	projectID := cleanID(payable["project_id"])
	payableID := cleanID(payable["id"])
	rows, err := queryTx(ctx, tx, `
		INSERT INTO payable_payment_schedules (
			id, project_id, payable_id, schedule_code, milestone_name, schedule_type,
			planned_amount, paid_amount, remaining_amount, due_date, status, notes, created_by
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,0,$7,NULLIF($8,'')::date,'draft',$9,NULLIF($10,'')::uuid)
		RETURNING *
	`, scheduleID, projectID, payableID, body["schedule_code"], body["milestone_name"], body["schedule_type"], number(body["planned_amount"]), fmt.Sprint(body["due_date"]), body["notes"], actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "payable_payment_schedule_created", "payable_payment_schedule", scheduleID, "Tạo kỳ thanh toán đầu vào", Record{
		"payable_id":     payableID,
		"schedule_code":  body["schedule_code"],
		"planned_amount": body["planned_amount"],
		"schedule_type":  body["schedule_type"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPayablePaymentScheduleDetail(ctx, scheduleID)
}

func (r *Repository) PayablePaymentScheduleAction(ctx context.Context, scheduleID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	if action != "approve" {
		return nil, validationError("Action kỳ thanh toán không hợp lệ", Record{"action": action})
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM payable_payment_schedules WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, scheduleID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	if !statusIn(fromStatus, "draft", "overdue") {
		return nil, validationError("Trạng thái kỳ thanh toán không cho phép duyệt", Record{"status": fromStatus})
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE payable_payment_schedules
		SET status='approved',
			approved_by=NULLIF($2,'')::uuid,
			updated_at=now()
		WHERE id=$1 AND status=$3
		RETURNING *
	`, scheduleID, actorID, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái kỳ thanh toán đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": "approved", "payable_id": current["payable_id"]}
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, "payable_payment_schedule_approved", "payable_payment_schedule", scheduleID, "Duyệt kỳ thanh toán đầu vào", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPayablePaymentScheduleDetail(ctx, scheduleID)
}

func (r *Repository) ListPayableAdvanceRequests(ctx context.Context, payableID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			ar.*,
			p.name AS project_name,
			pay.payable_code,
			pay.vendor_type,
			pay.vendor_name AS payable_vendor_name,
			pmt.payment_code,
			pmt.payment_type,
			COALESCE(requested.full_name, '') AS requested_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM advance_requests ar
		JOIN projects p ON p.id=ar.project_id
		JOIN payables pay ON pay.id=ar.payable_id
		LEFT JOIN payments pmt ON pmt.id=ar.payment_id
		LEFT JOIN users requested ON requested.id=ar.requested_by
		LEFT JOIN users approved ON approved.id=ar.approved_by
		WHERE ar.payable_id=$1
		  AND ar.status NOT IN ('cancelled','deleted')
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY ar.due_date NULLS LAST, ar.created_at DESC
	`, payableID)
}

func (r *Repository) GetPayableAdvanceRequestDetail(ctx context.Context, advanceID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			ar.*,
			p.name AS project_name,
			pay.payable_code,
			pay.vendor_type,
			pay.vendor_name AS payable_vendor_name,
			pmt.payment_code,
			pmt.payment_type,
			COALESCE(requested.full_name, '') AS requested_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM advance_requests ar
		JOIN projects p ON p.id=ar.project_id
		JOIN payables pay ON pay.id=ar.payable_id
		LEFT JOIN payments pmt ON pmt.id=ar.payment_id
		LEFT JOIN users requested ON requested.id=ar.requested_by
		LEFT JOIN users approved ON approved.id=ar.approved_by
		WHERE ar.id=$1
		  AND ar.status NOT IN ('cancelled','deleted')
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, advanceID)
}

func (r *Repository) CreatePayableAdvanceRequest(ctx context.Context, payable Record, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	advanceID := newID()
	projectID := cleanID(payable["project_id"])
	payableID := cleanID(payable["id"])
	rows, err := queryTx(ctx, tx, `
		INSERT INTO advance_requests (
			id, project_id, payable_id, request_code, requested_by, amount, reason, status,
			vendor_name, due_date, notes
		)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,$6,$7,'draft',$8,NULLIF($9,'')::date,$10)
		RETURNING *
	`, advanceID, projectID, payableID, body["request_code"], actorID, number(body["amount"]), body["reason"], body["vendor_name"], fmt.Sprint(body["due_date"]), body["notes"])
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "payable_advance_requested", "advance_request", advanceID, "Tạo yêu cầu ứng trước đầu vào", Record{
		"payable_id":   payableID,
		"request_code": body["request_code"],
		"amount":       body["amount"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPayableAdvanceRequestDetail(ctx, advanceID)
}

func (r *Repository) PayableAdvanceRequestAction(ctx context.Context, advanceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	if action != "approve" {
		return nil, validationError("Action ứng trước không hợp lệ", Record{"action": action})
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	advanceRows, err := queryTx(ctx, tx, `SELECT * FROM advance_requests WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, advanceID)
	if err != nil {
		return nil, err
	}
	if len(advanceRows) == 0 {
		return nil, sql.ErrNoRows
	}
	advance := advanceRows[0]
	fromStatus := fmt.Sprint(advance["status"])
	if !statusIn(fromStatus, "draft", "submitted") {
		return nil, validationError("Trạng thái ứng trước không cho phép duyệt", Record{"status": fromStatus})
	}
	payableRows, err := queryTx(ctx, tx, `SELECT * FROM payables WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, advance["payable_id"])
	if err != nil {
		return nil, err
	}
	if len(payableRows) == 0 {
		return nil, sql.ErrNoRows
	}
	payable := payableRows[0]
	if !statusIn(fmt.Sprint(payable["status"]), "approved", "partially_paid", "overdue") {
		return nil, validationError("Phải trả cần được duyệt trước khi duyệt ứng trước", Record{"status": payable["status"]})
	}
	amount := number(advance["amount"])
	if amount <= 0 || amount > number(payable["remaining_amount"]) {
		return nil, validationError("Số tiền ứng trước vượt số còn phải trả", Record{"amount": amount, "remaining_amount": payable["remaining_amount"]})
	}
	updatedAdvance, err := queryTx(ctx, tx, `
		UPDATE advance_requests
		SET status='approved',
			approved_by=NULLIF($2,'')::uuid,
			updated_at=now()
		WHERE id=$1 AND status=$3
		RETURNING *
	`, advanceID, actorID, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updatedAdvance) == 0 {
		return nil, validationError("Trạng thái ứng trước đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	newRemaining := number(payable["remaining_amount"]) - amount
	newStatus := payableSettlementStatus(number(payable["paid_amount"])+number(payable["advance_amount"])+amount, newRemaining)
	if _, err := tx.ExecContext(ctx, `
		UPDATE payables
		SET advance_amount=advance_amount + $2,
			remaining_amount=GREATEST(remaining_amount - $2, 0),
			status=$3,
			approved_by=COALESCE(approved_by,NULLIF($4,'')::uuid),
			updated_at=now()
		WHERE id=$1
	`, advance["payable_id"], amount, newStatus, actorID); err != nil {
		return nil, err
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": "approved", "payable_id": advance["payable_id"], "amount": amount}
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(advance["project_id"]), actorID, actorName, "payable_advance_approved", "advance_request", advanceID, "Duyệt ứng trước đầu vào", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPayableAdvanceRequestDetail(ctx, advanceID)
}

func (r *Repository) SettlePayableAdvanceRequest(ctx context.Context, advanceID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM advance_requests WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, advanceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	if !statusIn(fromStatus, "approved", "partially_settled") {
		return nil, validationError("Trạng thái ứng trước không cho phép quyết toán", Record{"status": fromStatus})
	}
	amount := number(body["amount"])
	remaining := number(current["amount"]) - number(current["settled_amount"])
	if amount <= 0 || amount > remaining {
		return nil, validationError("Số tiền quyết toán vượt số ứng trước còn lại", Record{"amount": amount, "remaining": remaining})
	}
	toStatus := "partially_settled"
	if amount >= remaining {
		toStatus = "settled"
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE advance_requests
		SET settled_amount=settled_amount + $2,
			status=$3,
			settled_at=CASE WHEN $3='settled' THEN now() ELSE settled_at END,
			updated_at=now()
		WHERE id=$1 AND status=$4
		RETURNING *
	`, advanceID, amount, toStatus, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái ứng trước đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	metadata := Record{
		"from_status":      fromStatus,
		"to_status":        toStatus,
		"payable_id":       current["payable_id"],
		"amount":           amount,
		"input_invoice_id": cleanID(body["input_invoice_id"]),
		"schedule_id":      cleanID(body["schedule_id"]),
	}
	if comment := strings.TrimSpace(fmt.Sprint(body["comment"])); comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, "payable_advance_settled", "advance_request", advanceID, "Quyết toán ứng trước đầu vào", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPayableAdvanceRequestDetail(ctx, advanceID)
}

func (r *Repository) ListPayablePayments(ctx context.Context, payableID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			pmt.*,
			p.name AS project_name,
			pa.id AS allocation_id,
			pa.schedule_id,
			pa.advance_request_id,
			pa.input_invoice_id,
			pa.allocation_type,
			pa.amount AS allocation_amount,
			pa.notes AS allocation_notes,
			pps.schedule_code,
			pps.milestone_name,
			ar.request_code AS advance_request_code,
			ii.invoice_number AS input_invoice_number,
			pay.payable_code,
			pay.vendor_name AS payable_vendor_name,
			COALESCE(created.full_name, '') AS created_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM payments pmt
		JOIN projects p ON p.id=pmt.project_id
		JOIN payment_allocations pa ON pa.payment_id=pmt.id
		JOIN payables pay ON pay.id=pa.payable_id
		LEFT JOIN payable_payment_schedules pps ON pps.id=pa.schedule_id
		LEFT JOIN advance_requests ar ON ar.id=pa.advance_request_id
		LEFT JOIN input_invoices ii ON ii.id=pa.input_invoice_id
		LEFT JOIN users created ON created.id=pmt.created_by
		LEFT JOIN users approved ON approved.id=pmt.approved_by
		WHERE pa.payable_id=$1
		  AND pmt.status <> 'cancelled'
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY pmt.created_at DESC
	`, payableID)
}

func (r *Repository) GetPayablePaymentDetail(ctx context.Context, paymentID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			pmt.*,
			p.name AS project_name,
			pa.id AS allocation_id,
			pa.payable_id,
			pa.schedule_id,
			pa.advance_request_id,
			pa.input_invoice_id,
			pa.allocation_type,
			pa.amount AS allocation_amount,
			pa.notes AS allocation_notes,
			pps.schedule_code,
			pps.milestone_name,
			ar.request_code AS advance_request_code,
			ii.invoice_number AS input_invoice_number,
			pay.payable_code,
			pay.vendor_name AS payable_vendor_name,
			COALESCE(created.full_name, '') AS created_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM payments pmt
		JOIN projects p ON p.id=pmt.project_id
		JOIN payment_allocations pa ON pa.payment_id=pmt.id
		JOIN payables pay ON pay.id=pa.payable_id
		LEFT JOIN payable_payment_schedules pps ON pps.id=pa.schedule_id
		LEFT JOIN advance_requests ar ON ar.id=pa.advance_request_id
		LEFT JOIN input_invoices ii ON ii.id=pa.input_invoice_id
		LEFT JOIN users created ON created.id=pmt.created_by
		LEFT JOIN users approved ON approved.id=pmt.approved_by
		WHERE pmt.id=$1
		  AND pmt.status <> 'cancelled'
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, paymentID)
}

func (r *Repository) CreatePayablePayment(ctx context.Context, payable Record, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	projectID := cleanID(payable["project_id"])
	payableID := cleanID(payable["id"])
	payableRows, err := queryTx(ctx, tx, `SELECT * FROM payables WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, payableID)
	if err != nil {
		return nil, err
	}
	if len(payableRows) == 0 {
		return nil, sql.ErrNoRows
	}
	currentPayable := payableRows[0]
	amount := number(body["amount"])
	allocationType := fmt.Sprint(body["allocation_type"])
	scheduleID := cleanID(body["schedule_id"])
	advanceID := cleanID(body["advance_request_id"])
	if allocationType == "payable_payment" && scheduleID == "" && amount > number(currentPayable["remaining_amount"]) {
		return nil, validationError("Số tiền thanh toán vượt số còn phải trả", Record{"amount": amount, "remaining_amount": currentPayable["remaining_amount"]})
	}
	if scheduleID != "" {
		scheduleRows, err := queryTx(ctx, tx, `SELECT * FROM payable_payment_schedules WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, scheduleID)
		if err != nil {
			return nil, err
		}
		if len(scheduleRows) == 0 {
			return nil, sql.ErrNoRows
		}
		schedule := scheduleRows[0]
		if cleanID(schedule["payable_id"]) != payableID || !statusIn(fmt.Sprint(schedule["status"]), "approved", "partially_paid", "overdue") {
			return nil, validationError("Kỳ thanh toán không thuộc phải trả hoặc chưa được duyệt", Record{"schedule_id": scheduleID})
		}
		if amount > number(schedule["remaining_amount"]) {
			return nil, validationError("Số tiền thanh toán vượt số còn lại của kỳ", Record{"amount": amount, "schedule_remaining": schedule["remaining_amount"]})
		}
	}
	if advanceID != "" {
		advanceRows, err := queryTx(ctx, tx, `SELECT * FROM advance_requests WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, advanceID)
		if err != nil {
			return nil, err
		}
		if len(advanceRows) == 0 {
			return nil, sql.ErrNoRows
		}
		advance := advanceRows[0]
		if cleanID(advance["payable_id"]) != payableID || !statusIn(fmt.Sprint(advance["status"]), "approved", "partially_settled") {
			return nil, validationError("Ứng trước không thuộc phải trả hoặc chưa được duyệt", Record{"advance_request_id": advanceID})
		}
		if amount > number(advance["amount"])-number(advance["paid_amount"]) {
			return nil, validationError("Số tiền thanh toán vượt số ứng trước chưa chi", Record{"amount": amount})
		}
	}
	paymentID := newID()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO payments (
			id, project_id, payment_code, payment_type, target_type, target_id,
			amount, payment_method, status, paid_at, created_by
		)
		VALUES ($1,$2,$3,$4,'payable',$5,$6,$7,'draft',NULLIF($8,'')::timestamptz,NULLIF($9,'')::uuid)
		RETURNING *
	`, paymentID, projectID, body["payment_code"], body["payment_type"], payableID, amount, body["payment_method"], fmt.Sprint(body["paid_at"]), actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO payment_allocations (
			project_id, payment_id, payable_id, schedule_id, advance_request_id, input_invoice_id,
			allocation_type, amount, notes, created_by
		)
		VALUES ($1,$2,$3,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,$7,$8,$9,NULLIF($10,'')::uuid)
	`, projectID, paymentID, payableID, scheduleID, advanceID, cleanID(body["input_invoice_id"]), allocationType, amount, body["notes"], actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "payable_payment_created", "payment", paymentID, "Tạo phiếu thanh toán đầu vào", Record{
		"payable_id":         payableID,
		"amount":             amount,
		"allocation_type":    allocationType,
		"schedule_id":        scheduleID,
		"advance_request_id": advanceID,
		"input_invoice_id":   cleanID(body["input_invoice_id"]),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPayablePaymentDetail(ctx, paymentID)
}

func (r *Repository) ListInputInvoicesForPayable(ctx context.Context, payableID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			ii.*,
			p.name AS project_name,
			pay.payable_code,
			pay.vendor_type,
			pay.vendor_name AS payable_vendor_name,
			pay.gross_amount AS payable_gross_amount,
			pay.remaining_amount AS payable_remaining_amount,
			pr.request_code AS procurement_request_code,
			d.name AS document_name,
			COALESCE(uploaded.full_name, '') AS uploaded_by_name,
			COALESCE(verified.full_name, '') AS verified_by_name,
			COALESCE(rejected.full_name, '') AS rejected_by_name
		FROM input_invoices ii
		JOIN projects p ON p.id=ii.project_id
		JOIN payables pay ON pay.id=ii.payable_id
		LEFT JOIN procurement_requests pr ON pr.id=ii.procurement_request_id
		LEFT JOIN documents d ON d.id=ii.document_id
		LEFT JOIN users uploaded ON uploaded.id=ii.uploaded_by
		LEFT JOIN users verified ON verified.id=ii.verified_by
		LEFT JOIN users rejected ON rejected.id=ii.rejected_by
		WHERE ii.payable_id=$1
		  AND ii.status NOT IN ('deleted','cancelled')
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY ii.invoice_date DESC, ii.created_at DESC
	`, payableID)
}

func (r *Repository) ListInputInvoicesForProject(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			ii.*,
			p.name AS project_name,
			pay.payable_code,
			pay.vendor_type,
			pay.vendor_name AS payable_vendor_name,
			pay.gross_amount AS payable_gross_amount,
			pay.remaining_amount AS payable_remaining_amount,
			pr.request_code AS procurement_request_code,
			d.name AS document_name,
			COALESCE(uploaded.full_name, '') AS uploaded_by_name,
			COALESCE(verified.full_name, '') AS verified_by_name,
			COALESCE(rejected.full_name, '') AS rejected_by_name
		FROM input_invoices ii
		JOIN projects p ON p.id=ii.project_id
		JOIN payables pay ON pay.id=ii.payable_id
		LEFT JOIN procurement_requests pr ON pr.id=ii.procurement_request_id
		LEFT JOIN documents d ON d.id=ii.document_id
		LEFT JOIN users uploaded ON uploaded.id=ii.uploaded_by
		LEFT JOIN users verified ON verified.id=ii.verified_by
		LEFT JOIN users rejected ON rejected.id=ii.rejected_by
		WHERE ii.project_id=$1
		  AND ii.status NOT IN ('deleted','cancelled')
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY ii.invoice_date DESC, ii.created_at DESC
	`, projectID)
}

func (r *Repository) GetInputInvoiceDetail(ctx context.Context, invoiceID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			ii.*,
			p.name AS project_name,
			pay.payable_code,
			pay.vendor_type,
			pay.vendor_name AS payable_vendor_name,
			pay.gross_amount AS payable_gross_amount,
			pay.remaining_amount AS payable_remaining_amount,
			pr.request_code AS procurement_request_code,
			d.name AS document_name,
			COALESCE(uploaded.full_name, '') AS uploaded_by_name,
			COALESCE(verified.full_name, '') AS verified_by_name,
			COALESCE(rejected.full_name, '') AS rejected_by_name
		FROM input_invoices ii
		JOIN projects p ON p.id=ii.project_id
		JOIN payables pay ON pay.id=ii.payable_id
		LEFT JOIN procurement_requests pr ON pr.id=ii.procurement_request_id
		LEFT JOIN documents d ON d.id=ii.document_id
		LEFT JOIN users uploaded ON uploaded.id=ii.uploaded_by
		LEFT JOIN users verified ON verified.id=ii.verified_by
		LEFT JOIN users rejected ON rejected.id=ii.rejected_by
		WHERE ii.id=$1
		  AND ii.status NOT IN ('deleted','cancelled')
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, invoiceID)
}

func (r *Repository) CreateInputInvoice(ctx context.Context, payable Record, body Record, file StoredFileMetadata, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	projectID := cleanID(payable["project_id"])
	payableID := cleanID(payable["id"])
	invoiceID := newID()
	documentID := cleanID(body["document_id"])
	if documentID == "" {
		documentID = newID()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO documents (id, project_id, target_type, target_id, name, file_url, document_type, status, uploaded_by)
			VALUES ($1, $2, 'input_invoice', $3, $4, $5, 'input_invoice_pdf', 'active', NULLIF($6,'')::uuid)
		`, documentID, projectID, invoiceID, fmt.Sprintf("Hoá đơn đầu vào %s", fmt.Sprint(body["invoice_number"])), fmt.Sprintf("/api/input-invoices/%s/file", invoiceID), actorID)
		if err != nil {
			return nil, err
		}
	}

	rows, err := queryTx(ctx, tx, `
		INSERT INTO input_invoices (
			id, project_id, payable_id, procurement_request_id, document_id,
			invoice_number, invoice_symbol, invoice_date, vendor_partner_id, vendor_name, vendor_tax_code,
			amount_before_vat, vat_amount, total_amount, currency, status, notes,
			storage_provider, storage_key, mime_type, file_name, file_size, checksum_sha256,
			uploaded_by
		)
		VALUES (
			$1, $2, $3, NULLIF($4,'')::uuid, NULLIF($5,'')::uuid,
			$6, $7, $8::date, NULLIF($9,'')::uuid, $10, $11,
			$12, $13, $14, 'VND', 'draft', $15,
			$16, $17, $18, $19, $20, $21,
			NULLIF($22,'')::uuid
		)
		RETURNING *
	`,
		invoiceID,
		projectID,
		payableID,
		cleanID(body["procurement_request_id"]),
		documentID,
		fmt.Sprint(body["invoice_number"]),
		fmt.Sprint(body["invoice_symbol"]),
		fmt.Sprint(body["invoice_date"]),
		cleanID(body["vendor_partner_id"]),
		fmt.Sprint(body["vendor_name"]),
		fmt.Sprint(body["vendor_tax_code"]),
		number(body["amount_before_vat"]),
		number(body["vat_amount"]),
		number(body["total_amount"]),
		fmt.Sprint(body["notes"]),
		file.StorageProvider,
		file.StorageKey,
		file.MIMEType,
		fmt.Sprint(body["file_name"]),
		file.FileSize,
		file.ChecksumSHA256,
		actorID,
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `UPDATE payables SET updated_at=now() WHERE id=$1`, payableID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "input_invoice_uploaded", "input_invoice", invoiceID, "Upload PDF hoá đơn đầu vào", Record{
		"payable_id":             payableID,
		"document_id":            documentID,
		"procurement_request_id": cleanID(body["procurement_request_id"]),
		"invoice_number":         body["invoice_number"],
		"total_amount":           body["total_amount"],
		"checksum_sha256":        file.ChecksumSHA256,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetInputInvoiceDetail(ctx, invoiceID)
}

func (r *Repository) InputInvoiceAction(ctx context.Context, invoiceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM input_invoices WHERE id=$1 AND status NOT IN ('deleted','cancelled') FOR UPDATE`, invoiceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	toStatus := ""
	event := ""
	var updated []Record
	switch action {
	case "verify":
		if !statusIn(fromStatus, "draft", "rejected") {
			return nil, validationError("Trạng thái hoá đơn không cho phép xác thực", Record{"status": fromStatus})
		}
		toStatus = "verified"
		event = "input_invoice_verified"
		updated, err = queryTx(ctx, tx, `
			UPDATE input_invoices
			SET status='verified',
				verified_by=NULLIF($2,'')::uuid,
				verified_at=now(),
				rejected_by=NULL,
				rejected_at=NULL,
				updated_at=now()
			WHERE id=$1 AND status=$3
			RETURNING *
		`, invoiceID, actorID, fromStatus)
	case "reject":
		if !statusIn(fromStatus, "draft", "verified") {
			return nil, validationError("Trạng thái hoá đơn không cho phép từ chối", Record{"status": fromStatus})
		}
		toStatus = "rejected"
		event = "input_invoice_rejected"
		updated, err = queryTx(ctx, tx, `
			UPDATE input_invoices
			SET status='rejected',
				rejected_by=NULLIF($2,'')::uuid,
				rejected_at=now(),
				verified_by=NULL,
				verified_at=NULL,
				updated_at=now()
			WHERE id=$1 AND status=$3
			RETURNING *
		`, invoiceID, actorID, fromStatus)
	default:
		return nil, validationError("Action hoá đơn đầu vào không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái hoá đơn đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": toStatus, "payable_id": current["payable_id"]}
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, event, "input_invoice", invoiceID, fmt.Sprintf("Cập nhật hoá đơn đầu vào từ %s sang %s", fromStatus, toStatus), metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetInputInvoiceDetail(ctx, invoiceID)
}

func (r *Repository) ListReceivablePaymentSchedules(ctx context.Context, receivableID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			rps.*,
			p.name AS project_name,
			rec.receivable_code,
			rec.client_name,
			COALESCE(created.full_name, '') AS created_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM receivable_payment_schedules rps
		JOIN projects p ON p.id=rps.project_id
		JOIN receivables rec ON rec.id=rps.receivable_id
		LEFT JOIN users created ON created.id=rps.created_by
		LEFT JOIN users approved ON approved.id=rps.approved_by
		WHERE rps.receivable_id=$1
		  AND rps.status NOT IN ('cancelled','deleted')
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY rps.due_date NULLS LAST, rps.created_at DESC
	`, receivableID)
}

func (r *Repository) GetReceivablePaymentScheduleDetail(ctx context.Context, scheduleID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			rps.*,
			p.name AS project_name,
			rec.receivable_code,
			rec.client_name,
			COALESCE(created.full_name, '') AS created_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM receivable_payment_schedules rps
		JOIN projects p ON p.id=rps.project_id
		JOIN receivables rec ON rec.id=rps.receivable_id
		LEFT JOIN users created ON created.id=rps.created_by
		LEFT JOIN users approved ON approved.id=rps.approved_by
		WHERE rps.id=$1
		  AND rps.status NOT IN ('cancelled','deleted')
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, scheduleID)
}

func (r *Repository) CreateReceivablePaymentSchedule(ctx context.Context, receivable Record, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	scheduleID := newID()
	projectID := cleanID(receivable["project_id"])
	receivableID := cleanID(receivable["id"])
	rows, err := queryTx(ctx, tx, `
		INSERT INTO receivable_payment_schedules (
			id, project_id, receivable_id, schedule_code, milestone_name, schedule_type,
			planned_amount, received_amount, remaining_amount, due_date, status, notes, created_by
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,0,$7,NULLIF($8,'')::date,'draft',$9,NULLIF($10,'')::uuid)
		RETURNING *
	`, scheduleID, projectID, receivableID, body["schedule_code"], body["milestone_name"], body["schedule_type"], number(body["planned_amount"]), fmt.Sprint(body["due_date"]), body["notes"], actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "receivable_payment_schedule_created", "receivable_payment_schedule", scheduleID, "Tạo kỳ thu đầu ra", Record{
		"receivable_id":  receivableID,
		"schedule_code":  body["schedule_code"],
		"planned_amount": body["planned_amount"],
		"schedule_type":  body["schedule_type"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetReceivablePaymentScheduleDetail(ctx, scheduleID)
}

func (r *Repository) ReceivablePaymentScheduleAction(ctx context.Context, scheduleID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	if action != "approve" {
		return nil, validationError("Action kỳ thu không hợp lệ", Record{"action": action})
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM receivable_payment_schedules WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, scheduleID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	if !statusIn(fromStatus, "draft", "overdue") {
		return nil, validationError("Trạng thái kỳ thu không cho phép duyệt", Record{"status": fromStatus})
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE receivable_payment_schedules
		SET status='approved',
			approved_by=NULLIF($2,'')::uuid,
			updated_at=now()
		WHERE id=$1 AND status=$3
		RETURNING *
	`, scheduleID, actorID, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái kỳ thu đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": "approved", "receivable_id": current["receivable_id"]}
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, "receivable_payment_schedule_approved", "receivable_payment_schedule", scheduleID, "Duyệt kỳ thu đầu ra", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetReceivablePaymentScheduleDetail(ctx, scheduleID)
}

func (r *Repository) ListReceivableAdvanceReceipts(ctx context.Context, receivableID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			car.*,
			p.name AS project_name,
			rec.receivable_code,
			rec.client_name AS receivable_client_name,
			pmt.payment_code,
			pmt.payment_type,
			COALESCE(requested.full_name, '') AS requested_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM client_advance_receipts car
		JOIN projects p ON p.id=car.project_id
		JOIN receivables rec ON rec.id=car.receivable_id
		LEFT JOIN payments pmt ON pmt.id=car.payment_id
		LEFT JOIN users requested ON requested.id=car.requested_by
		LEFT JOIN users approved ON approved.id=car.approved_by
		WHERE car.receivable_id=$1
		  AND car.status NOT IN ('cancelled','deleted')
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY car.due_date NULLS LAST, car.created_at DESC
	`, receivableID)
}

func (r *Repository) GetReceivableAdvanceReceiptDetail(ctx context.Context, advanceID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			car.*,
			p.name AS project_name,
			rec.receivable_code,
			rec.client_name AS receivable_client_name,
			pmt.payment_code,
			pmt.payment_type,
			COALESCE(requested.full_name, '') AS requested_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM client_advance_receipts car
		JOIN projects p ON p.id=car.project_id
		JOIN receivables rec ON rec.id=car.receivable_id
		LEFT JOIN payments pmt ON pmt.id=car.payment_id
		LEFT JOIN users requested ON requested.id=car.requested_by
		LEFT JOIN users approved ON approved.id=car.approved_by
		WHERE car.id=$1
		  AND car.status NOT IN ('cancelled','deleted')
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, advanceID)
}

func (r *Repository) CreateReceivableAdvanceReceipt(ctx context.Context, receivable Record, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	advanceID := newID()
	projectID := cleanID(receivable["project_id"])
	receivableID := cleanID(receivable["id"])
	rows, err := queryTx(ctx, tx, `
		INSERT INTO client_advance_receipts (
			id, project_id, receivable_id, receipt_code, requested_by, amount, reason, status,
			client_name, due_date, notes
		)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,$6,$7,'draft',$8,NULLIF($9,'')::date,$10)
		RETURNING *
	`, advanceID, projectID, receivableID, body["receipt_code"], actorID, number(body["amount"]), body["reason"], body["client_name"], fmt.Sprint(body["due_date"]), body["notes"])
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "receivable_advance_receipt_created", "client_advance_receipt", advanceID, "Tạo ứng trước đầu ra", Record{
		"receivable_id": receivableID,
		"receipt_code":  body["receipt_code"],
		"amount":        body["amount"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetReceivableAdvanceReceiptDetail(ctx, advanceID)
}

func (r *Repository) ReceivableAdvanceReceiptAction(ctx context.Context, advanceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	if action != "approve" {
		return nil, validationError("Action ứng trước đầu ra không hợp lệ", Record{"action": action})
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	advanceRows, err := queryTx(ctx, tx, `SELECT * FROM client_advance_receipts WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, advanceID)
	if err != nil {
		return nil, err
	}
	if len(advanceRows) == 0 {
		return nil, sql.ErrNoRows
	}
	advance := advanceRows[0]
	fromStatus := fmt.Sprint(advance["status"])
	if !statusIn(fromStatus, "draft", "submitted") {
		return nil, validationError("Trạng thái ứng trước đầu ra không cho phép duyệt", Record{"status": fromStatus})
	}
	receivableRows, err := queryTx(ctx, tx, `SELECT * FROM receivables WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, advance["receivable_id"])
	if err != nil {
		return nil, err
	}
	if len(receivableRows) == 0 {
		return nil, sql.ErrNoRows
	}
	receivable := receivableRows[0]
	if !statusIn(fmt.Sprint(receivable["status"]), "approved", "partially_received", "overdue") {
		return nil, validationError("Phải thu cần được duyệt trước khi duyệt ứng trước", Record{"status": receivable["status"]})
	}
	amount := number(advance["amount"])
	if amount <= 0 || amount > number(receivable["remaining_amount"]) {
		return nil, validationError("Số tiền ứng trước vượt số còn phải thu", Record{"amount": amount, "remaining_amount": receivable["remaining_amount"]})
	}
	updatedAdvance, err := queryTx(ctx, tx, `
		UPDATE client_advance_receipts
		SET status='approved',
			approved_by=NULLIF($2,'')::uuid,
			updated_at=now()
		WHERE id=$1 AND status=$3
		RETURNING *
	`, advanceID, actorID, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updatedAdvance) == 0 {
		return nil, validationError("Trạng thái ứng trước đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	newRemaining := number(receivable["remaining_amount"]) - amount
	newReceived := number(receivable["received_amount"]) + amount
	if _, err := tx.ExecContext(ctx, `
		UPDATE receivables
		SET received_amount=received_amount + $2,
			remaining_amount=GREATEST(remaining_amount - $2, 0),
			status=$3,
			approved_by=COALESCE(approved_by,NULLIF($4,'')::uuid),
			updated_at=now()
		WHERE id=$1
	`, advance["receivable_id"], amount, receivableSettlementStatus(newReceived, newRemaining), actorID); err != nil {
		return nil, err
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": "approved", "receivable_id": advance["receivable_id"], "amount": amount}
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(advance["project_id"]), actorID, actorName, "receivable_advance_receipt_approved", "client_advance_receipt", advanceID, "Duyệt ứng trước đầu ra", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetReceivableAdvanceReceiptDetail(ctx, advanceID)
}

func (r *Repository) SettleReceivableAdvanceReceipt(ctx context.Context, advanceID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM client_advance_receipts WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, advanceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	if !statusIn(fromStatus, "approved", "partially_received", "received", "partially_settled") {
		return nil, validationError("Trạng thái ứng trước không cho phép quyết toán", Record{"status": fromStatus})
	}
	amount := number(body["amount"])
	remaining := number(current["amount"]) - number(current["settled_amount"])
	if amount <= 0 || amount > remaining {
		return nil, validationError("Số tiền quyết toán vượt số ứng trước còn lại", Record{"amount": amount, "remaining": remaining})
	}
	toStatus := "partially_settled"
	if amount >= remaining {
		toStatus = "settled"
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE client_advance_receipts
		SET settled_amount=settled_amount + $2,
			status=$3,
			settled_at=CASE WHEN $3='settled' THEN now() ELSE settled_at END,
			updated_at=now()
		WHERE id=$1 AND status=$4
		RETURNING *
	`, advanceID, amount, toStatus, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái ứng trước đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	metadata := Record{
		"from_status":       fromStatus,
		"to_status":         toStatus,
		"receivable_id":     current["receivable_id"],
		"amount":            amount,
		"output_invoice_id": cleanID(body["output_invoice_id"]),
		"schedule_id":       cleanID(body["schedule_id"]),
	}
	if comment := strings.TrimSpace(fmt.Sprint(body["comment"])); comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, "receivable_advance_receipt_settled", "client_advance_receipt", advanceID, "Quyết toán ứng trước đầu ra", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetReceivableAdvanceReceiptDetail(ctx, advanceID)
}

func (r *Repository) ListReceivablePayments(ctx context.Context, receivableID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			pmt.*,
			p.name AS project_name,
			ra.id AS allocation_id,
			ra.schedule_id,
			ra.advance_receipt_id,
			ra.output_invoice_id,
			ra.allocation_type,
			ra.amount AS allocation_amount,
			ra.notes AS allocation_notes,
			rps.schedule_code,
			rps.milestone_name,
			car.receipt_code AS advance_receipt_code,
			oi.invoice_number AS output_invoice_number,
			rec.receivable_code,
			rec.client_name AS receivable_client_name,
			COALESCE(created.full_name, '') AS created_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM payments pmt
		JOIN projects p ON p.id=pmt.project_id
		JOIN receipt_allocations ra ON ra.payment_id=pmt.id
		JOIN receivables rec ON rec.id=ra.receivable_id
		LEFT JOIN receivable_payment_schedules rps ON rps.id=ra.schedule_id
		LEFT JOIN client_advance_receipts car ON car.id=ra.advance_receipt_id
		LEFT JOIN output_invoices oi ON oi.id=ra.output_invoice_id
		LEFT JOIN users created ON created.id=pmt.created_by
		LEFT JOIN users approved ON approved.id=pmt.approved_by
		WHERE ra.receivable_id=$1
		  AND pmt.status <> 'cancelled'
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY pmt.created_at DESC
	`, receivableID)
}

func (r *Repository) GetReceivablePaymentDetail(ctx context.Context, paymentID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			pmt.*,
			p.name AS project_name,
			ra.id AS allocation_id,
			ra.receivable_id,
			ra.schedule_id,
			ra.advance_receipt_id,
			ra.output_invoice_id,
			ra.allocation_type,
			ra.amount AS allocation_amount,
			ra.notes AS allocation_notes,
			rps.schedule_code,
			rps.milestone_name,
			car.receipt_code AS advance_receipt_code,
			oi.invoice_number AS output_invoice_number,
			rec.receivable_code,
			rec.client_name AS receivable_client_name,
			COALESCE(created.full_name, '') AS created_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM payments pmt
		JOIN projects p ON p.id=pmt.project_id
		JOIN receipt_allocations ra ON ra.payment_id=pmt.id
		JOIN receivables rec ON rec.id=ra.receivable_id
		LEFT JOIN receivable_payment_schedules rps ON rps.id=ra.schedule_id
		LEFT JOIN client_advance_receipts car ON car.id=ra.advance_receipt_id
		LEFT JOIN output_invoices oi ON oi.id=ra.output_invoice_id
		LEFT JOIN users created ON created.id=pmt.created_by
		LEFT JOIN users approved ON approved.id=pmt.approved_by
		WHERE pmt.id=$1
		  AND pmt.status <> 'cancelled'
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, paymentID)
}

func (r *Repository) CreateReceivablePayment(ctx context.Context, receivable Record, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	projectID := cleanID(receivable["project_id"])
	receivableID := cleanID(receivable["id"])
	receivableRows, err := queryTx(ctx, tx, `SELECT * FROM receivables WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, receivableID)
	if err != nil {
		return nil, err
	}
	if len(receivableRows) == 0 {
		return nil, sql.ErrNoRows
	}
	currentReceivable := receivableRows[0]
	amount := number(body["amount"])
	allocationType := fmt.Sprint(body["allocation_type"])
	scheduleID := cleanID(body["schedule_id"])
	advanceID := cleanID(body["advance_receipt_id"])
	if allocationType == "receivable_receipt" && scheduleID == "" && amount > number(currentReceivable["remaining_amount"]) {
		return nil, validationError("Số tiền thu vượt số còn phải thu", Record{"amount": amount, "remaining_amount": currentReceivable["remaining_amount"]})
	}
	if scheduleID != "" {
		scheduleRows, err := queryTx(ctx, tx, `SELECT * FROM receivable_payment_schedules WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, scheduleID)
		if err != nil {
			return nil, err
		}
		if len(scheduleRows) == 0 {
			return nil, sql.ErrNoRows
		}
		schedule := scheduleRows[0]
		if cleanID(schedule["receivable_id"]) != receivableID || !statusIn(fmt.Sprint(schedule["status"]), "approved", "partially_received", "overdue") {
			return nil, validationError("Kỳ thu không thuộc phải thu hoặc chưa được duyệt", Record{"schedule_id": scheduleID})
		}
		if amount > number(schedule["remaining_amount"]) {
			return nil, validationError("Số tiền thu vượt số còn lại của kỳ", Record{"amount": amount, "schedule_remaining": schedule["remaining_amount"]})
		}
	}
	if advanceID != "" {
		advanceRows, err := queryTx(ctx, tx, `SELECT * FROM client_advance_receipts WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, advanceID)
		if err != nil {
			return nil, err
		}
		if len(advanceRows) == 0 {
			return nil, sql.ErrNoRows
		}
		advance := advanceRows[0]
		if cleanID(advance["receivable_id"]) != receivableID || !statusIn(fmt.Sprint(advance["status"]), "approved", "partially_received", "received", "partially_settled") {
			return nil, validationError("Ứng trước không thuộc phải thu hoặc chưa được duyệt", Record{"advance_receipt_id": advanceID})
		}
		if amount > number(advance["amount"])-number(advance["received_amount"]) {
			return nil, validationError("Số tiền thu vượt số ứng trước chưa ghi nhận tiền", Record{"amount": amount})
		}
	}
	paymentID := newID()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO payments (
			id, project_id, payment_code, payment_type, target_type, target_id,
			amount, payment_method, status, paid_at, created_by
		)
		VALUES ($1,$2,$3,$4,'receivable',$5,$6,$7,'draft',NULLIF($8,'')::timestamptz,NULLIF($9,'')::uuid)
		RETURNING *
	`, paymentID, projectID, body["payment_code"], body["payment_type"], receivableID, amount, body["payment_method"], fmt.Sprint(body["paid_at"]), actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO receipt_allocations (
			project_id, payment_id, receivable_id, schedule_id, advance_receipt_id, output_invoice_id,
			allocation_type, amount, notes, created_by
		)
		VALUES ($1,$2,$3,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,$7,$8,$9,NULLIF($10,'')::uuid)
	`, projectID, paymentID, receivableID, scheduleID, advanceID, cleanID(body["output_invoice_id"]), allocationType, amount, body["notes"], actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "receivable_payment_created", "payment", paymentID, "Tạo phiếu thu đầu ra", Record{
		"receivable_id":      receivableID,
		"amount":             amount,
		"allocation_type":    allocationType,
		"schedule_id":        scheduleID,
		"advance_receipt_id": advanceID,
		"output_invoice_id":  cleanID(body["output_invoice_id"]),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetReceivablePaymentDetail(ctx, paymentID)
}

func (r *Repository) GetReceivableDetail(ctx context.Context, receivableID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT rec.*, p.name AS project_name
		FROM receivables rec
		JOIN projects p ON p.id=rec.project_id
		WHERE rec.id=$1
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, receivableID)
}

func (r *Repository) ReceivableReconciliation(ctx context.Context, receivableID string) (Record, error) {
	summary, err := r.QueryOne(ctx, `
		WITH rec AS (
			SELECT rec.*, p.name AS project_name
			FROM receivables rec
			JOIN projects p ON p.id=rec.project_id
			WHERE rec.id=$1
			  AND rec.status <> 'cancelled'
			  AND p.status <> 'cancelled'
		),
		inv AS (
			SELECT
				receivable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('deleted','cancelled')) AS invoice_count,
				COUNT(*) FILTER (WHERE status='issued') AS issued_invoice_count,
				COUNT(*) FILTER (WHERE status='voided') AS voided_invoice_count,
				COALESCE(SUM(total_amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS invoice_total,
				COALESCE(SUM(total_amount) FILTER (WHERE status='issued'), 0) AS issued_invoice_total,
				MAX(invoice_date) FILTER (WHERE status NOT IN ('deleted','cancelled')) AS latest_invoice_date
			FROM output_invoices
			WHERE receivable_id=$1
			GROUP BY receivable_id
		),
		sch AS (
			SELECT
				receivable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('cancelled','deleted')) AS schedule_count,
				COUNT(*) FILTER (WHERE status IN ('approved','partially_received','overdue') AND remaining_amount > 0 AND due_date < CURRENT_DATE) AS overdue_schedule_count,
				COALESCE(SUM(planned_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS scheduled_total,
				COALESCE(SUM(received_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS scheduled_received_total,
				COALESCE(SUM(remaining_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS scheduled_remaining,
				MIN(due_date) FILTER (WHERE status IN ('approved','partially_received','overdue') AND remaining_amount > 0) AS next_schedule_due_date
			FROM receivable_payment_schedules
			WHERE receivable_id=$1
			GROUP BY receivable_id
		),
		adv AS (
			SELECT
				receivable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('cancelled','deleted')) AS advance_receipt_count,
				COALESCE(SUM(amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_requested_total,
				COALESCE(SUM(amount) FILTER (WHERE status IN ('approved','partially_received','received','partially_settled','settled')), 0) AS advance_approved_total,
				COALESCE(SUM(received_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_received_total,
				COALESCE(SUM(settled_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_settled_total
			FROM client_advance_receipts
			WHERE receivable_id=$1
			GROUP BY receivable_id
		),
		alloc AS (
			SELECT
				ra.receivable_id,
				COALESCE(SUM(ra.amount) FILTER (WHERE pmt.status='approved' AND ra.allocation_type='receivable_receipt'), 0) AS approved_receipt_total,
				COALESCE(SUM(ra.amount) FILTER (WHERE pmt.status='approved' AND ra.allocation_type='advance_receipt'), 0) AS approved_advance_receipt_total,
				COALESCE(SUM(ra.amount) FILTER (WHERE pmt.status <> 'cancelled'), 0) AS receipt_allocation_total,
				COUNT(*) FILTER (WHERE pmt.status NOT IN ('approved','cancelled')) AS pending_receipt_count
			FROM receipt_allocations ra
			JOIN payments pmt ON pmt.id=ra.payment_id
			WHERE ra.receivable_id=$1
			GROUP BY ra.receivable_id
		),
		calc AS (
			SELECT
				rec.*,
				COALESCE(inv.invoice_count, 0) AS invoice_count,
				COALESCE(inv.issued_invoice_count, 0) AS issued_invoice_count,
				COALESCE(inv.voided_invoice_count, 0) AS voided_invoice_count,
				COALESCE(inv.invoice_total, 0) AS invoice_total,
				COALESCE(inv.issued_invoice_total, 0) AS issued_invoice_total,
				inv.latest_invoice_date,
				COALESCE(sch.schedule_count, 0) AS schedule_count,
				COALESCE(sch.overdue_schedule_count, 0) AS overdue_schedule_count,
				COALESCE(sch.scheduled_total, 0) AS scheduled_total,
				COALESCE(sch.scheduled_received_total, 0) AS scheduled_received_total,
				COALESCE(sch.scheduled_remaining, 0) AS scheduled_remaining,
				sch.next_schedule_due_date,
				COALESCE(adv.advance_receipt_count, 0) AS advance_receipt_count,
				COALESCE(adv.advance_requested_total, 0) AS advance_requested_total,
				COALESCE(adv.advance_approved_total, 0) AS advance_approved_total,
				COALESCE(adv.advance_received_total, 0) AS advance_received_total,
				COALESCE(adv.advance_settled_total, 0) AS advance_settled_total,
				COALESCE(alloc.approved_receipt_total, 0) AS approved_receipt_total,
				COALESCE(alloc.approved_advance_receipt_total, 0) AS approved_advance_receipt_total,
				COALESCE(alloc.receipt_allocation_total, 0) AS receipt_allocation_total,
				COALESCE(alloc.pending_receipt_count, 0) AS pending_receipt_count,
				COALESCE(rec.due_date, sch.next_schedule_due_date, inv.latest_invoice_date) AS effective_due_date,
				GREATEST(COALESCE(rec.gross_amount,0) - COALESCE(rec.received_amount,0), 0) AS computed_remaining
			FROM rec
			LEFT JOIN inv ON inv.receivable_id=rec.id
			LEFT JOIN sch ON sch.receivable_id=rec.id
			LEFT JOIN adv ON adv.receivable_id=rec.id
			LEFT JOIN alloc ON alloc.receivable_id=rec.id
		)
		SELECT
			calc.*,
			CASE
				WHEN COALESCE(remaining_amount,0) <= 0 THEN 0
				WHEN effective_due_date IS NULL THEN 0
				WHEN effective_due_date >= CURRENT_DATE THEN 0
				ELSE CURRENT_DATE - effective_due_date
			END AS overdue_days,
			CASE
				WHEN COALESCE(remaining_amount,0) <= 0 THEN 'settled'
				WHEN effective_due_date IS NULL THEN 'no_due_date'
				WHEN effective_due_date >= CURRENT_DATE THEN 'not_due'
				WHEN CURRENT_DATE - effective_due_date <= 30 THEN '1_30'
				WHEN CURRENT_DATE - effective_due_date <= 60 THEN '31_60'
				WHEN CURRENT_DATE - effective_due_date <= 90 THEN '61_90'
				ELSE '90_plus'
			END AS aging_bucket,
			ABS(COALESCE(remaining_amount,0) - COALESCE(computed_remaining,0)) AS balance_difference,
			GREATEST(COALESCE(gross_amount,0) - COALESCE(issued_invoice_total,0), 0) AS unissued_invoice_gap,
			GREATEST(COALESCE(advance_approved_total,0) - COALESCE(advance_received_total,0), 0) AS advance_unreceived_total,
			GREATEST(COALESCE(advance_received_total,0) - COALESCE(advance_settled_total,0), 0) AS advance_unsettled_total
		FROM calc
	`, receivableID)
	if err != nil {
		return nil, err
	}
	warnings := receivableReconciliationWarnings(summary)
	summary["warning_count"] = len(warnings)
	if len(warnings) > 0 {
		summary["reconciliation_status"] = "needs_review"
	} else {
		summary["reconciliation_status"] = "ok"
	}

	ledger, err := r.Query(ctx, `
		WITH entries AS (
			SELECT
				rec.created_at AS occurred_at,
				0 AS sort_order,
				'receivable_recorded'::text AS entry_type,
				rec.id::text AS source_id,
				rec.receivable_code::text AS source_code,
				rec.status::text AS status,
				'Ghi nhận phải thu'::text AS description,
				COALESCE(rec.gross_amount,0)::numeric AS debit_amount,
				0::numeric AS credit_amount,
				0::numeric AS document_amount,
				0::numeric AS cash_amount,
				COALESCE(created.full_name, '') AS actor_name
			FROM receivables rec
			LEFT JOIN users created ON created.id=rec.created_by
			WHERE rec.id=$1 AND rec.status <> 'cancelled'

			UNION ALL

			SELECT
				COALESCE(oi.issued_at, oi.voided_at, oi.created_at) AS occurred_at,
				10 AS sort_order,
				('output_invoice_' || oi.status)::text AS entry_type,
				oi.id::text AS source_id,
				oi.invoice_number::text AS source_code,
				oi.status::text AS status,
				CASE
					WHEN oi.status='issued' THEN 'Hoá đơn đầu ra đã phát hành'
					WHEN oi.status='voided' THEN 'Hoá đơn đầu ra đã huỷ'
					ELSE 'Upload hoá đơn đầu ra'
				END AS description,
				0::numeric AS debit_amount,
				0::numeric AS credit_amount,
				COALESCE(oi.total_amount,0)::numeric AS document_amount,
				0::numeric AS cash_amount,
				COALESCE(actor.full_name, '') AS actor_name
			FROM output_invoices oi
			LEFT JOIN users actor ON actor.id=COALESCE(oi.issued_by, oi.voided_by, oi.uploaded_by)
			WHERE oi.receivable_id=$1 AND oi.status NOT IN ('deleted','cancelled')

			UNION ALL

			SELECT
				car.updated_at AS occurred_at,
				20 AS sort_order,
				'client_advance_approved'::text AS entry_type,
				car.id::text AS source_id,
				car.receipt_code::text AS source_code,
				car.status::text AS status,
				'Ứng trước khách hàng được duyệt ghi giảm phải thu'::text AS description,
				0::numeric AS debit_amount,
				COALESCE(car.amount,0)::numeric AS credit_amount,
				0::numeric AS document_amount,
				0::numeric AS cash_amount,
				COALESCE(approved.full_name, '') AS actor_name
			FROM client_advance_receipts car
			LEFT JOIN users approved ON approved.id=car.approved_by
			WHERE car.receivable_id=$1 AND car.status IN ('approved','partially_received','received','partially_settled','settled')

			UNION ALL

			SELECT
				COALESCE(pmt.paid_at, pmt.updated_at, pmt.created_at) AS occurred_at,
				30 AS sort_order,
				('receipt_' || ra.allocation_type)::text AS entry_type,
				pmt.id::text AS source_id,
				pmt.payment_code::text AS source_code,
				pmt.status::text AS status,
				CASE
					WHEN ra.allocation_type='advance_receipt' THEN 'Thu tiền ứng trước khách hàng'
					WHEN ra.allocation_type='advance_settlement' THEN 'Quyết toán ứng trước khách hàng'
					ELSE 'Thu tiền phải thu'
				END AS description,
				0::numeric AS debit_amount,
				CASE WHEN ra.allocation_type='receivable_receipt' THEN COALESCE(ra.amount,0)::numeric ELSE 0::numeric END AS credit_amount,
				0::numeric AS document_amount,
				COALESCE(ra.amount,0)::numeric AS cash_amount,
				COALESCE(approved.full_name, created.full_name, '') AS actor_name
			FROM receipt_allocations ra
			JOIN payments pmt ON pmt.id=ra.payment_id
			LEFT JOIN users approved ON approved.id=pmt.approved_by
			LEFT JOIN users created ON created.id=pmt.created_by
			WHERE ra.receivable_id=$1 AND pmt.status='approved'

			UNION ALL

			SELECT
				COALESCE(car.settled_at, car.updated_at) AS occurred_at,
				40 AS sort_order,
				'client_advance_settled'::text AS entry_type,
				car.id::text AS source_id,
				car.receipt_code::text AS source_code,
				car.status::text AS status,
				'Quyết toán ứng trước bằng chứng từ đầu ra'::text AS description,
				0::numeric AS debit_amount,
				0::numeric AS credit_amount,
				COALESCE(car.settled_amount,0)::numeric AS document_amount,
				0::numeric AS cash_amount,
				''::text AS actor_name
			FROM client_advance_receipts car
			WHERE car.receivable_id=$1
			  AND car.status NOT IN ('cancelled','deleted')
			  AND COALESCE(car.settled_amount,0) > 0
		),
		running AS (
			SELECT
				entries.*,
				SUM(debit_amount - credit_amount) OVER (
					ORDER BY occurred_at ASC, sort_order ASC, source_id ASC
					ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
				) AS running_balance
			FROM entries
		)
		SELECT *
		FROM running
		ORDER BY occurred_at DESC, sort_order DESC, source_id DESC
	`, receivableID)
	if err != nil {
		return nil, err
	}

	return Record{
		"summary":  summary,
		"warnings": warnings,
		"ledger":   ledger,
	}, nil
}

func receivableReconciliationWarnings(summary Record) []Record {
	warnings := []Record{}
	if number(summary["remaining_amount"]) > 0 && int(number(summary["overdue_days"])) > 0 {
		severity := "warning"
		if number(summary["overdue_days"]) > 60 {
			severity = "critical"
		}
		warnings = append(warnings, receivableWarning("overdue", severity, "Khoản phải thu đã quá hạn", Record{
			"effective_due_date": summary["effective_due_date"],
			"overdue_days":       summary["overdue_days"],
			"remaining_amount":   summary["remaining_amount"],
		}))
	}
	if number(summary["issued_invoice_total"]) > number(summary["gross_amount"])+1 {
		warnings = append(warnings, receivableWarning("invoice_over_gross", "critical", "Tổng hoá đơn issued vượt giá trị phải thu", Record{
			"gross_amount":         summary["gross_amount"],
			"issued_invoice_total": summary["issued_invoice_total"],
		}))
	}
	if number(summary["received_amount"]) > number(summary["issued_invoice_total"])+1 {
		warnings = append(warnings, receivableWarning("received_without_issued_invoice", "warning", "Đã thu nhiều hơn tổng hoá đơn issued", Record{
			"received_amount":        summary["received_amount"],
			"issued_invoice_total":   summary["issued_invoice_total"],
			"approved_receipt_total": summary["approved_receipt_total"],
		}))
	}
	if number(summary["remaining_amount"]) > 0 && int(number(summary["schedule_count"])) == 0 {
		warnings = append(warnings, receivableWarning("missing_schedule", "warning", "Khoản phải thu còn dư nhưng chưa có lịch thu", Record{
			"remaining_amount": summary["remaining_amount"],
		}))
	}
	if number(summary["scheduled_remaining"]) > number(summary["remaining_amount"])+1 {
		warnings = append(warnings, receivableWarning("schedule_over_remaining", "warning", "Số còn lại theo lịch lớn hơn số còn phải thu", Record{
			"scheduled_remaining": summary["scheduled_remaining"],
			"remaining_amount":    summary["remaining_amount"],
		}))
	}
	if number(summary["advance_unreceived_total"]) > 1 {
		warnings = append(warnings, receivableWarning("advance_unreceived", "warning", "Có ứng trước đã duyệt nhưng chưa thu đủ tiền", Record{
			"advance_approved_total":   summary["advance_approved_total"],
			"advance_received_total":   summary["advance_received_total"],
			"advance_unreceived_total": summary["advance_unreceived_total"],
		}))
	}
	if number(summary["advance_unsettled_total"]) > 1 {
		warnings = append(warnings, receivableWarning("advance_unsettled", "warning", "Có ứng trước đã thu nhưng chưa quyết toán đủ", Record{
			"advance_received_total":  summary["advance_received_total"],
			"advance_settled_total":   summary["advance_settled_total"],
			"advance_unsettled_total": summary["advance_unsettled_total"],
		}))
	}
	if number(summary["balance_difference"]) > 1 {
		warnings = append(warnings, receivableWarning("balance_difference", "critical", "Số còn phải thu lệch với công thức gross - đã thu", Record{
			"remaining_amount":   summary["remaining_amount"],
			"computed_remaining": summary["computed_remaining"],
			"balance_difference": summary["balance_difference"],
		}))
	}
	if int(number(summary["pending_receipt_count"])) > 0 {
		warnings = append(warnings, receivableWarning("pending_receipt", "info", "Có phiếu thu đang chờ duyệt", Record{
			"pending_receipt_count": summary["pending_receipt_count"],
		}))
	}
	return warnings
}

func receivableWarning(code, severity, message string, details Record) Record {
	return Record{
		"code":     code,
		"severity": severity,
		"message":  message,
		"details":  details,
	}
}

func (r *Repository) ListOutputInvoicesForReceivable(ctx context.Context, receivableID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			oi.*,
			p.name AS project_name,
			rec.receivable_code,
			rec.client_name AS receivable_client_name,
			rec.gross_amount AS receivable_gross_amount,
			rec.remaining_amount AS receivable_remaining_amount,
			pay.payment_code,
			pay.payment_type,
			d.name AS document_name,
			COALESCE(uploaded.full_name, '') AS uploaded_by_name,
			COALESCE(issued.full_name, '') AS issued_by_name,
			COALESCE(voided.full_name, '') AS voided_by_name
		FROM output_invoices oi
		JOIN projects p ON p.id=oi.project_id
		JOIN receivables rec ON rec.id=oi.receivable_id
		LEFT JOIN payments pay ON pay.id=oi.payment_id
		LEFT JOIN documents d ON d.id=oi.document_id
		LEFT JOIN users uploaded ON uploaded.id=oi.uploaded_by
		LEFT JOIN users issued ON issued.id=oi.issued_by
		LEFT JOIN users voided ON voided.id=oi.voided_by
		WHERE oi.receivable_id=$1
		  AND oi.status NOT IN ('deleted','cancelled')
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY oi.invoice_date DESC, oi.created_at DESC
	`, receivableID)
}

func (r *Repository) ListOutputInvoicesForProject(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			oi.*,
			p.name AS project_name,
			rec.receivable_code,
			rec.client_name AS receivable_client_name,
			rec.gross_amount AS receivable_gross_amount,
			rec.remaining_amount AS receivable_remaining_amount,
			pay.payment_code,
			pay.payment_type,
			d.name AS document_name,
			COALESCE(uploaded.full_name, '') AS uploaded_by_name,
			COALESCE(issued.full_name, '') AS issued_by_name,
			COALESCE(voided.full_name, '') AS voided_by_name
		FROM output_invoices oi
		JOIN projects p ON p.id=oi.project_id
		JOIN receivables rec ON rec.id=oi.receivable_id
		LEFT JOIN payments pay ON pay.id=oi.payment_id
		LEFT JOIN documents d ON d.id=oi.document_id
		LEFT JOIN users uploaded ON uploaded.id=oi.uploaded_by
		LEFT JOIN users issued ON issued.id=oi.issued_by
		LEFT JOIN users voided ON voided.id=oi.voided_by
		WHERE oi.project_id=$1
		  AND oi.status NOT IN ('deleted','cancelled')
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY oi.invoice_date DESC, oi.created_at DESC
	`, projectID)
}

func (r *Repository) GetOutputInvoiceDetail(ctx context.Context, invoiceID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			oi.*,
			p.name AS project_name,
			rec.receivable_code,
			rec.client_name AS receivable_client_name,
			rec.gross_amount AS receivable_gross_amount,
			rec.remaining_amount AS receivable_remaining_amount,
			pay.payment_code,
			pay.payment_type,
			d.name AS document_name,
			COALESCE(uploaded.full_name, '') AS uploaded_by_name,
			COALESCE(issued.full_name, '') AS issued_by_name,
			COALESCE(voided.full_name, '') AS voided_by_name
		FROM output_invoices oi
		JOIN projects p ON p.id=oi.project_id
		JOIN receivables rec ON rec.id=oi.receivable_id
		LEFT JOIN payments pay ON pay.id=oi.payment_id
		LEFT JOIN documents d ON d.id=oi.document_id
		LEFT JOIN users uploaded ON uploaded.id=oi.uploaded_by
		LEFT JOIN users issued ON issued.id=oi.issued_by
		LEFT JOIN users voided ON voided.id=oi.voided_by
		WHERE oi.id=$1
		  AND oi.status NOT IN ('deleted','cancelled')
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, invoiceID)
}

func (r *Repository) CreateOutputInvoice(ctx context.Context, receivable Record, body Record, file StoredFileMetadata, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	projectID := cleanID(receivable["project_id"])
	receivableID := cleanID(receivable["id"])
	invoiceID := newID()
	documentID := cleanID(body["document_id"])
	if documentID == "" {
		documentID = newID()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO documents (id, project_id, target_type, target_id, name, file_url, document_type, status, uploaded_by)
			VALUES ($1, $2, 'output_invoice', $3, $4, $5, 'output_invoice_pdf', 'active', NULLIF($6,'')::uuid)
		`, documentID, projectID, invoiceID, fmt.Sprintf("Hoá đơn đầu ra %s", fmt.Sprint(body["invoice_number"])), fmt.Sprintf("/api/output-invoices/%s/file", invoiceID), actorID)
		if err != nil {
			return nil, err
		}
	}

	rows, err := queryTx(ctx, tx, `
		INSERT INTO output_invoices (
			id, project_id, receivable_id, payment_id, document_id,
			invoice_number, invoice_symbol, invoice_date, client_partner_id, client_name, client_tax_code,
			amount_before_vat, vat_amount, total_amount, currency, status, notes,
			storage_provider, storage_key, mime_type, file_name, file_size, checksum_sha256,
			uploaded_by
		)
		VALUES (
			$1, $2, $3, NULLIF($4,'')::uuid, NULLIF($5,'')::uuid,
			$6, $7, $8::date, NULLIF($9,'')::uuid, $10, $11,
			$12, $13, $14, 'VND', 'draft', $15,
			$16, $17, $18, $19, $20, $21,
			NULLIF($22,'')::uuid
		)
		RETURNING *
	`,
		invoiceID,
		projectID,
		receivableID,
		cleanID(body["payment_id"]),
		documentID,
		fmt.Sprint(body["invoice_number"]),
		fmt.Sprint(body["invoice_symbol"]),
		fmt.Sprint(body["invoice_date"]),
		cleanID(body["client_partner_id"]),
		fmt.Sprint(body["client_name"]),
		fmt.Sprint(body["client_tax_code"]),
		number(body["amount_before_vat"]),
		number(body["vat_amount"]),
		number(body["total_amount"]),
		fmt.Sprint(body["notes"]),
		file.StorageProvider,
		file.StorageKey,
		file.MIMEType,
		fmt.Sprint(body["file_name"]),
		file.FileSize,
		file.ChecksumSHA256,
		actorID,
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `UPDATE receivables SET updated_at=now() WHERE id=$1`, receivableID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "output_invoice_uploaded", "output_invoice", invoiceID, "Upload PDF hoá đơn đầu ra", Record{
		"receivable_id":   receivableID,
		"document_id":     documentID,
		"payment_id":      cleanID(body["payment_id"]),
		"invoice_number":  body["invoice_number"],
		"total_amount":    body["total_amount"],
		"checksum_sha256": file.ChecksumSHA256,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetOutputInvoiceDetail(ctx, invoiceID)
}

func (r *Repository) OutputInvoiceAction(ctx context.Context, invoiceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM output_invoices WHERE id=$1 AND status NOT IN ('deleted','cancelled') FOR UPDATE`, invoiceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	toStatus := ""
	event := ""
	var updated []Record
	switch action {
	case "issue":
		if fromStatus != "draft" {
			return nil, validationError("Trạng thái hoá đơn không cho phép phát hành", Record{"status": fromStatus})
		}
		toStatus = "issued"
		event = "output_invoice_issued"
		updated, err = queryTx(ctx, tx, `
			UPDATE output_invoices
			SET status='issued',
				issued_by=NULLIF($2,'')::uuid,
				issued_at=now(),
				voided_by=NULL,
				voided_at=NULL,
				updated_at=now()
			WHERE id=$1 AND status=$3
			RETURNING *
		`, invoiceID, actorID, fromStatus)
	case "void":
		if !statusIn(fromStatus, "draft", "issued") {
			return nil, validationError("Trạng thái hoá đơn không cho phép huỷ", Record{"status": fromStatus})
		}
		toStatus = "voided"
		event = "output_invoice_voided"
		updated, err = queryTx(ctx, tx, `
			UPDATE output_invoices
			SET status='voided',
				voided_by=NULLIF($2,'')::uuid,
				voided_at=now(),
				updated_at=now()
			WHERE id=$1 AND status=$3
			RETURNING *
		`, invoiceID, actorID, fromStatus)
	default:
		return nil, validationError("Action hoá đơn đầu ra không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái hoá đơn đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": toStatus, "receivable_id": current["receivable_id"]}
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, event, "output_invoice", invoiceID, fmt.Sprintf("Cập nhật hoá đơn đầu ra từ %s sang %s", fromStatus, toStatus), metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetOutputInvoiceDetail(ctx, invoiceID)
}

func (r *Repository) ReceivableAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM receivables WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	toStatus := ""
	event := ""
	metadata := Record{"action": action, "from_status": fromStatus}
	var updated []Record
	switch action {
	case "approve":
		if !statusIn(fromStatus, "draft", "submitted", "recorded", "overdue") {
			return nil, validationError("Trạng thái phải thu không cho phép duyệt", Record{"status": fromStatus})
		}
		toStatus = "approved"
		event = "receivable_approved"
		updated, err = queryTx(ctx, tx, `
			UPDATE receivables
			SET status='approved', approved_by=NULLIF($2,'')::uuid, updated_at=now()
			WHERE id=$1 AND status=$3
			RETURNING *
		`, id, actorID, fromStatus)
	case "receive":
		if !statusIn(fromStatus, "approved", "partially_received", "overdue") {
			return nil, validationError("Phải thu cần được duyệt trước khi ghi nhận thu", Record{"status": fromStatus})
		}
		remaining := number(current["remaining_amount"])
		if remaining <= 0 {
			return nil, validationError("Phải thu không còn số tiền cần ghi nhận")
		}
		toStatus = "received"
		event = "receivable_received"
		metadata["amount"] = remaining
		updated, err = queryTx(ctx, tx, `
			UPDATE receivables
			SET status='received',
				received_amount=received_amount + $2,
				remaining_amount=0,
				approved_by=COALESCE(approved_by,NULLIF($3,'')::uuid),
				updated_at=now()
			WHERE id=$1 AND status=$4
			RETURNING *
		`, id, remaining, actorID, fromStatus)
	default:
		return nil, validationError("Action phải thu không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái phải thu đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	comment := actionComment(req)
	metadata["to_status"] = toStatus
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, event, "receivable", id, fmt.Sprintf("Cập nhật phải thu từ %s sang %s", fromStatus, toStatus), metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated[0], nil
}

func (r *Repository) PaymentAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	if action != "approve" {
		return nil, validationError("Action phiếu thanh toán không hợp lệ", Record{"action": action})
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM payments WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	if !statusIn(fromStatus, "draft", "submitted", "pending") {
		return nil, validationError("Trạng thái phiếu thanh toán không cho phép duyệt", Record{"status": fromStatus})
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE payments
		SET status='approved',
			approved_by=NULLIF($2,'')::uuid,
			paid_at=COALESCE(paid_at, now()),
			updated_at=now()
		WHERE id=$1 AND status=$3
		RETURNING *
	`, id, actorID, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái phiếu thanh toán đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	if err := r.applyPaymentAllocationsTx(ctx, tx, updated[0], actorID, actorName); err != nil {
		return nil, err
	}
	if err := r.applyReceiptAllocationsTx(ctx, tx, updated[0], actorID, actorName); err != nil {
		return nil, err
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": "approved"}
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, "payment_approved", "payment", id, "Duyệt phiếu thanh toán", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated[0], nil
}

func (r *Repository) applyPaymentAllocationsTx(ctx context.Context, tx *sql.Tx, payment Record, actorID, actorName string) error {
	allocations, err := queryTx(ctx, tx, `SELECT * FROM payment_allocations WHERE payment_id=$1 FOR UPDATE`, payment["id"])
	if err != nil {
		return err
	}
	for _, allocation := range allocations {
		amount := number(allocation["amount"])
		if amount <= 0 {
			return validationError("Allocation thanh toán không hợp lệ", Record{"allocation_id": allocation["id"]})
		}
		projectID := fmt.Sprint(allocation["project_id"])
		payableID := cleanID(allocation["payable_id"])
		switch fmt.Sprint(allocation["allocation_type"]) {
		case "payable_payment":
			payableRows, err := queryTx(ctx, tx, `SELECT * FROM payables WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, payableID)
			if err != nil {
				return err
			}
			if len(payableRows) == 0 {
				return sql.ErrNoRows
			}
			payable := payableRows[0]
			if amount > number(payable["remaining_amount"]) {
				return validationError("Số tiền thanh toán vượt số còn phải trả", Record{"amount": amount, "remaining_amount": payable["remaining_amount"]})
			}
			scheduleID := cleanID(allocation["schedule_id"])
			if scheduleID != "" {
				scheduleRows, err := queryTx(ctx, tx, `SELECT * FROM payable_payment_schedules WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, scheduleID)
				if err != nil {
					return err
				}
				if len(scheduleRows) == 0 {
					return sql.ErrNoRows
				}
				schedule := scheduleRows[0]
				if cleanID(schedule["payable_id"]) != payableID {
					return validationError("Kỳ thanh toán không thuộc phải trả", Record{"schedule_id": scheduleID})
				}
				if amount > number(schedule["remaining_amount"]) {
					return validationError("Số tiền thanh toán vượt số còn lại của kỳ", Record{"amount": amount, "schedule_remaining": schedule["remaining_amount"]})
				}
				scheduleRemaining := number(schedule["remaining_amount"]) - amount
				if _, err := tx.ExecContext(ctx, `
					UPDATE payable_payment_schedules
					SET paid_amount=paid_amount + $2,
						remaining_amount=GREATEST(remaining_amount - $2, 0),
						status=$3,
						updated_at=now()
					WHERE id=$1
				`, scheduleID, amount, scheduleSettlementStatus(scheduleRemaining)); err != nil {
					return err
				}
			}
			payableRemaining := number(payable["remaining_amount"]) - amount
			applied := number(payable["paid_amount"]) + number(payable["advance_amount"]) + amount
			if _, err := tx.ExecContext(ctx, `
				UPDATE payables
				SET paid_amount=paid_amount + $2,
					remaining_amount=GREATEST(remaining_amount - $2, 0),
					status=$3,
					approved_by=COALESCE(approved_by,NULLIF($4,'')::uuid),
					updated_at=now()
				WHERE id=$1
			`, payableID, amount, payableSettlementStatus(applied, payableRemaining), actorID); err != nil {
				return err
			}
			if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "payable_payment_allocated", "payment", cleanID(payment["id"]), "Ghi nhận thanh toán từng phần đầu vào", Record{
				"payable_id":       payableID,
				"schedule_id":      scheduleID,
				"input_invoice_id": cleanID(allocation["input_invoice_id"]),
				"amount":           amount,
			}); err != nil {
				return err
			}
		case "advance_payment":
			advanceID := cleanID(allocation["advance_request_id"])
			if advanceID == "" {
				return validationError("Thanh toán ứng trước thiếu advance_request_id", Record{"allocation_id": allocation["id"]})
			}
			advanceRows, err := queryTx(ctx, tx, `SELECT * FROM advance_requests WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, advanceID)
			if err != nil {
				return err
			}
			if len(advanceRows) == 0 {
				return sql.ErrNoRows
			}
			advance := advanceRows[0]
			if cleanID(advance["payable_id"]) != payableID {
				return validationError("Ứng trước không thuộc phải trả", Record{"advance_request_id": advanceID})
			}
			if amount > number(advance["amount"])-number(advance["paid_amount"]) {
				return validationError("Số tiền chi ứng trước vượt phần chưa chi", Record{"amount": amount})
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE advance_requests
				SET paid_amount=paid_amount + $2,
					payment_id=COALESCE(payment_id, $3),
					paid_at=COALESCE(paid_at, $4::timestamptz, now()),
					updated_at=now()
				WHERE id=$1
			`, advanceID, amount, payment["id"], payment["paid_at"]); err != nil {
				return err
			}
			if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "payable_advance_payment_recorded", "advance_request", advanceID, "Ghi nhận chi ứng trước đầu vào", Record{
				"payable_id": payableID,
				"payment_id": cleanID(payment["id"]),
				"amount":     amount,
			}); err != nil {
				return err
			}
		case "advance_settlement":
			continue
		default:
			return validationError("Loại allocation thanh toán không hợp lệ", Record{"allocation_type": allocation["allocation_type"]})
		}
	}
	return nil
}

func (r *Repository) applyReceiptAllocationsTx(ctx context.Context, tx *sql.Tx, payment Record, actorID, actorName string) error {
	allocations, err := queryTx(ctx, tx, `SELECT * FROM receipt_allocations WHERE payment_id=$1 FOR UPDATE`, payment["id"])
	if err != nil {
		return err
	}
	for _, allocation := range allocations {
		amount := number(allocation["amount"])
		if amount <= 0 {
			return validationError("Allocation phiếu thu không hợp lệ", Record{"allocation_id": allocation["id"]})
		}
		projectID := fmt.Sprint(allocation["project_id"])
		receivableID := cleanID(allocation["receivable_id"])
		switch fmt.Sprint(allocation["allocation_type"]) {
		case "receivable_receipt":
			receivableRows, err := queryTx(ctx, tx, `SELECT * FROM receivables WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, receivableID)
			if err != nil {
				return err
			}
			if len(receivableRows) == 0 {
				return sql.ErrNoRows
			}
			receivable := receivableRows[0]
			if amount > number(receivable["remaining_amount"]) {
				return validationError("Số tiền thu vượt số còn phải thu", Record{"amount": amount, "remaining_amount": receivable["remaining_amount"]})
			}
			scheduleID := cleanID(allocation["schedule_id"])
			if scheduleID != "" {
				scheduleRows, err := queryTx(ctx, tx, `SELECT * FROM receivable_payment_schedules WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, scheduleID)
				if err != nil {
					return err
				}
				if len(scheduleRows) == 0 {
					return sql.ErrNoRows
				}
				schedule := scheduleRows[0]
				if cleanID(schedule["receivable_id"]) != receivableID {
					return validationError("Kỳ thu không thuộc phải thu", Record{"schedule_id": scheduleID})
				}
				if amount > number(schedule["remaining_amount"]) {
					return validationError("Số tiền thu vượt số còn lại của kỳ", Record{"amount": amount, "schedule_remaining": schedule["remaining_amount"]})
				}
				scheduleRemaining := number(schedule["remaining_amount"]) - amount
				if _, err := tx.ExecContext(ctx, `
					UPDATE receivable_payment_schedules
					SET received_amount=received_amount + $2,
						remaining_amount=GREATEST(remaining_amount - $2, 0),
						status=$3,
						updated_at=now()
					WHERE id=$1
				`, scheduleID, amount, receiptScheduleSettlementStatus(scheduleRemaining)); err != nil {
					return err
				}
			}
			receivableRemaining := number(receivable["remaining_amount"]) - amount
			received := number(receivable["received_amount"]) + amount
			if _, err := tx.ExecContext(ctx, `
				UPDATE receivables
				SET received_amount=received_amount + $2,
					remaining_amount=GREATEST(remaining_amount - $2, 0),
					status=$3,
					approved_by=COALESCE(approved_by,NULLIF($4,'')::uuid),
					updated_at=now()
				WHERE id=$1
			`, receivableID, amount, receivableSettlementStatus(received, receivableRemaining), actorID); err != nil {
				return err
			}
			if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "receivable_payment_allocated", "payment", cleanID(payment["id"]), "Ghi nhận thu từng phần đầu ra", Record{
				"receivable_id":     receivableID,
				"schedule_id":       scheduleID,
				"output_invoice_id": cleanID(allocation["output_invoice_id"]),
				"amount":            amount,
			}); err != nil {
				return err
			}
		case "advance_receipt":
			advanceID := cleanID(allocation["advance_receipt_id"])
			if advanceID == "" {
				return validationError("Thu ứng trước thiếu advance_receipt_id", Record{"allocation_id": allocation["id"]})
			}
			advanceRows, err := queryTx(ctx, tx, `SELECT * FROM client_advance_receipts WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, advanceID)
			if err != nil {
				return err
			}
			if len(advanceRows) == 0 {
				return sql.ErrNoRows
			}
			advance := advanceRows[0]
			if cleanID(advance["receivable_id"]) != receivableID {
				return validationError("Ứng trước không thuộc phải thu", Record{"advance_receipt_id": advanceID})
			}
			if amount > number(advance["amount"])-number(advance["received_amount"]) {
				return validationError("Số tiền thu ứng trước vượt phần chưa ghi nhận tiền", Record{"amount": amount})
			}
			newReceived := number(advance["received_amount"]) + amount
			toStatus := clientAdvanceReceiptStatusAfterReceipt(number(advance["amount"]), newReceived, number(advance["settled_amount"]))
			if _, err := tx.ExecContext(ctx, `
				UPDATE client_advance_receipts
				SET received_amount=received_amount + $2,
					payment_id=COALESCE(payment_id, $3),
					received_at=COALESCE(received_at, $4::timestamptz, now()),
					status=$5,
					updated_at=now()
				WHERE id=$1
			`, advanceID, amount, payment["id"], payment["paid_at"], toStatus); err != nil {
				return err
			}
			if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "receivable_advance_payment_recorded", "client_advance_receipt", advanceID, "Ghi nhận thu ứng trước đầu ra", Record{
				"receivable_id": receivableID,
				"payment_id":    cleanID(payment["id"]),
				"amount":        amount,
			}); err != nil {
				return err
			}
		case "advance_settlement":
			continue
		default:
			return validationError("Loại allocation phiếu thu không hợp lệ", Record{"allocation_type": allocation["allocation_type"]})
		}
	}
	return nil
}

func payableSettlementStatus(applied, remaining float64) string {
	if remaining <= 0.000001 {
		return "paid"
	}
	if applied > 0 {
		return "partially_paid"
	}
	return "approved"
}

func scheduleSettlementStatus(remaining float64) string {
	if remaining <= 0.000001 {
		return "paid"
	}
	return "partially_paid"
}

func receivableSettlementStatus(received, remaining float64) string {
	if remaining <= 0.000001 {
		return "received"
	}
	if received > 0 {
		return "partially_received"
	}
	return "approved"
}

func receiptScheduleSettlementStatus(remaining float64) string {
	if remaining <= 0.000001 {
		return "received"
	}
	return "partially_received"
}

func clientAdvanceReceiptStatusAfterReceipt(total, received, settled float64) string {
	if settled >= total && total > 0 {
		return "settled"
	}
	if settled > 0 {
		return "partially_settled"
	}
	if received >= total && total > 0 {
		return "received"
	}
	return "partially_received"
}

func (r *Repository) DashboardReport(ctx context.Context, userID string) (Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, err
	}
	data := Record{}
	scalars := map[string]string{
		"total_projects":       `SELECT COUNT(*) FROM projects p WHERE p.status <> 'cancelled' AND %s`,
		"active_projects":      `SELECT COUNT(*) FROM projects p WHERE p.status IN ('planning','in_progress','acceptance','handover') AND %s`,
		"blocked_projects":     `SELECT COUNT(*) FROM projects p WHERE p.status='blocked' AND %s`,
		"pending_approvals":    `SELECT COUNT(*) FROM approval_requests ar JOIN projects p ON p.id=ar.project_id WHERE ar.status='pending' AND p.status <> 'cancelled' AND %s`,
		"receivable_remaining": `SELECT COALESCE(SUM(rec.remaining_amount),0) FROM receivables rec JOIN projects p ON p.id=rec.project_id WHERE rec.status <> 'cancelled' AND p.status <> 'cancelled' AND %s`,
		"payable_remaining":    `SELECT COALESCE(SUM(pay.remaining_amount),0) FROM payables pay JOIN projects p ON p.id=pay.project_id WHERE pay.status <> 'cancelled' AND p.status <> 'cancelled' AND %s`,
		"missing_materials":    `SELECT COUNT(*) FROM site_material_balances smb JOIN materials m ON m.id=smb.material_id JOIN projects p ON p.id=smb.project_id WHERE smb.issued_quantity < smb.planned_quantity AND m.active AND p.status <> 'cancelled' AND %s`,
		"failed_acceptances":   `SELECT COUNT(*) FROM acceptance_records ar JOIN projects p ON p.id=ar.project_id WHERE ar.result='fail' AND p.status <> 'cancelled' AND %s`,
		"pending_reworks":      `SELECT COUNT(*) FROM rework_requests rw JOIN projects p ON p.id=rw.project_id WHERE rw.status IN ('draft','approved','in_progress') AND p.status <> 'cancelled' AND %s`,
		"cashflow_month":       `SELECT COALESCE(SUM(CASE WHEN pay.payment_type='client_receipt' THEN pay.amount ELSE -pay.amount END),0) FROM payments pay JOIN projects p ON p.id=pay.project_id WHERE pay.status <> 'cancelled' AND p.status <> 'cancelled' AND date_trunc('month', pay.created_at)=date_trunc('month', now()) AND %s`,
	}
	for key, query := range scalars {
		var value any
		if err := r.db.QueryRowContext(ctx, fmt.Sprintf(query, scopeSQL), scopeArgs...).Scan(&value); err != nil {
			return nil, err
		}
		data[key] = normalizeValue(value)
	}
	charts := Record{}
	statusRows, _ := r.Query(ctx, fmt.Sprintf(`SELECT p.status, COUNT(*) AS total FROM projects p WHERE p.status <> 'cancelled' AND %s GROUP BY p.status ORDER BY p.status`, scopeSQL), scopeArgs...)
	debtRows, _ := r.Query(ctx, fmt.Sprintf(`
		SELECT 'Phải thu' AS type, COALESCE(SUM(rec.remaining_amount),0) AS amount
		FROM receivables rec
		JOIN projects p ON p.id=rec.project_id
		WHERE rec.status <> 'cancelled' AND p.status <> 'cancelled' AND %s
		UNION ALL
		SELECT 'Phải trả' AS type, COALESCE(SUM(pay.remaining_amount),0) AS amount
		FROM payables pay
		JOIN projects p ON p.id=pay.project_id
		WHERE pay.status <> 'cancelled' AND p.status <> 'cancelled' AND %s
	`, scopeSQL, scopeSQL), scopeArgs...)
	costRows, _ := r.Query(ctx, fmt.Sprintf(`
		SELECT p.name, COALESCE(b.used_amount,0) AS used_amount, COALESCE(b.remaining_amount,0) AS remaining_amount
		FROM projects p
		LEFT JOIN accounting_budgets b ON b.project_id=p.id
		WHERE p.status <> 'cancelled' AND %s
		ORDER BY p.created_at
	`, scopeSQL), scopeArgs...)
	approvalRows, _ := r.Query(ctx, fmt.Sprintf(`
		SELECT ar.target_type, COUNT(*) AS total
		FROM approval_requests ar
		JOIN projects p ON p.id=ar.project_id
		WHERE p.status <> 'cancelled' AND %s
		GROUP BY ar.target_type
		ORDER BY ar.target_type
	`, scopeSQL), scopeArgs...)
	charts["project_status"] = statusRows
	charts["debt"] = debtRows
	charts["cost_by_project"] = costRows
	charts["approvals_by_type"] = approvalRows
	data["charts"] = charts
	return data, nil
}

func (r *Repository) Report(ctx context.Context, kind, userID string) ([]Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "projects":
		return r.Query(ctx, fmt.Sprintf(`
			SELECT p.name, p.status, p.progress_percent, COUNT(i.id) FILTER (WHERE i.status IN ('open','in_progress')) AS open_issues,
				COUNT(ar.id) FILTER (WHERE ar.status='pending') AS pending_approvals
			FROM projects p
			LEFT JOIN issues i ON i.project_id=p.id
			LEFT JOIN approval_requests ar ON ar.project_id=p.id
			WHERE p.status <> 'cancelled' AND %s
			GROUP BY p.id
			ORDER BY p.created_at DESC
		`, scopeSQL), scopeArgs...)
	case "warehouse":
		return r.Query(ctx, fmt.Sprintf(`
			SELECT m.name, m.category,
				COALESCE(SUM(CASE
					WHEN sm.movement_type IN ('receive','return_to_warehouse') THEN sm.quantity
					WHEN sm.movement_type IN ('issue','transfer','loss') THEN -sm.quantity
					ELSE 0
				END),0) AS stock_quantity,
				COALESCE(SUM(smb.surplus_quantity),0) AS surplus_quantity
			FROM materials m
			LEFT JOIN stock_movements sm ON sm.material_id=m.id
				AND (
					sm.project_id IS NULL
					OR EXISTS (
						SELECT 1 FROM projects p
						WHERE p.id=sm.project_id AND p.status <> 'cancelled' AND %s
					)
				)
			LEFT JOIN site_material_balances smb ON smb.material_id=m.id
				AND EXISTS (
					SELECT 1 FROM projects p
					WHERE p.id=smb.project_id AND p.status <> 'cancelled' AND %s
				)
			WHERE m.active
			GROUP BY m.id
			ORDER BY m.category, m.name
		`, scopeSQL, scopeSQL), scopeArgs...)
	case "procurement":
		return r.Query(ctx, fmt.Sprintf(`
			SELECT pr.status, pr.budget_status, COUNT(*) AS total, COALESCE(SUM(pr.total_estimated_amount),0) AS amount
			FROM procurement_requests pr
			JOIN projects p ON p.id=pr.project_id
			WHERE pr.status <> 'cancelled' AND p.status <> 'cancelled' AND %s
			GROUP BY pr.status, pr.budget_status
			ORDER BY pr.status
		`, scopeSQL), scopeArgs...)
	case "accounting":
		return r.Query(ctx, fmt.Sprintf(`
			SELECT p.name,
				COALESCE(SUM(DISTINCT pay.remaining_amount),0) AS payable_remaining,
				COALESCE(SUM(DISTINCT rec.remaining_amount),0) AS receivable_remaining,
				COALESCE(MAX(b.used_amount),0) AS used_budget
			FROM projects p
			LEFT JOIN payables pay ON pay.project_id=p.id AND pay.status <> 'cancelled'
			LEFT JOIN receivables rec ON rec.project_id=p.id AND rec.status <> 'cancelled'
			LEFT JOIN accounting_budgets b ON b.project_id=p.id AND b.status <> 'cancelled'
			WHERE p.status <> 'cancelled' AND %s
			GROUP BY p.id
			ORDER BY p.name
		`, scopeSQL), scopeArgs...)
	case "staff":
		return r.Query(ctx, `
			SELECT s.full_name, s.department, s.position, COUNT(pm.project_id) AS project_count
			FROM staff s
			LEFT JOIN project_members pm ON pm.staff_id=s.id AND pm.active
			WHERE s.status <> 'deleted'
			GROUP BY s.id
			ORDER BY s.full_name
		`)
	default:
		return nil, errors.New("unknown report")
	}
}

func (r *Repository) CashflowReport(ctx context.Context, req CashflowReportRequest, userID string) (Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, err
	}
	statusFilter := cashflowStatusFilter(req.Status)

	openingArgs := append([]any{}, scopeArgs...)
	openingFromIdx := len(openingArgs) + 1
	openingArgs = append(openingArgs, req.FromDate)
	openingProjectFilter := cashflowProjectFilter(&openingArgs, req.ProjectID)
	openingRows, err := r.Query(ctx, cashflowRowsCTE(
		scopeSQL,
		openingProjectFilter,
		fmt.Sprintf(" AND %s < $%d::date", cashflowOccurredAtSQL(), openingFromIdx),
		statusFilter,
	)+`
		SELECT COALESCE(SUM(net_amount), 0) AS opening_balance
		FROM cashflow_rows
	`, openingArgs...)
	if err != nil {
		return nil, err
	}
	openingBalance := 0.0
	if len(openingRows) > 0 {
		openingBalance = number(openingRows[0]["opening_balance"])
	}

	rangeArgs := append([]any{}, scopeArgs...)
	fromIdx := len(rangeArgs) + 1
	rangeArgs = append(rangeArgs, req.FromDate)
	toIdx := len(rangeArgs) + 1
	rangeArgs = append(rangeArgs, req.ToDate)
	rangeProjectFilter := cashflowProjectFilter(&rangeArgs, req.ProjectID)
	rangeDateFilter := fmt.Sprintf(" AND %s >= $%d::date AND %s < ($%d::date + INTERVAL '1 day')", cashflowOccurredAtSQL(), fromIdx, cashflowOccurredAtSQL(), toIdx)
	rangeCTE := cashflowRowsCTE(scopeSQL, rangeProjectFilter, rangeDateFilter, statusFilter)

	summaryRows, err := r.Query(ctx, rangeCTE+`
		SELECT
			COUNT(*) AS payment_count,
			COUNT(*) FILTER (WHERE direction='unclassified') AS unclassified_count,
			COUNT(*) FILTER (WHERE direction='non_cash') AS non_cash_count,
			COALESCE(SUM(inflow_amount), 0) AS total_inflow,
			COALESCE(SUM(outflow_amount), 0) AS total_outflow,
			COALESCE(SUM(net_amount), 0) AS net_flow
		FROM cashflow_rows
	`, rangeArgs...)
	if err != nil {
		return nil, err
	}
	summary := Record{
		"from":                  req.FromDate,
		"to":                    req.ToDate,
		"group_by":              req.GroupBy,
		"status":                req.Status,
		"project_id":            req.ProjectID,
		"opening_balance":       openingBalance,
		"payment_count":         0,
		"unclassified_count":    0,
		"non_cash_count":        0,
		"total_inflow":          0,
		"total_outflow":         0,
		"net_flow":              0,
		"closing_balance":       openingBalance,
		"pending_payment_count": 0,
		"pending_payment_total": 0,
	}
	if len(summaryRows) > 0 {
		row := summaryRows[0]
		netFlow := number(row["net_flow"])
		summary["payment_count"] = row["payment_count"]
		summary["unclassified_count"] = row["unclassified_count"]
		summary["non_cash_count"] = row["non_cash_count"]
		summary["total_inflow"] = row["total_inflow"]
		summary["total_outflow"] = row["total_outflow"]
		summary["net_flow"] = row["net_flow"]
		summary["closing_balance"] = openingBalance + netFlow
	}

	pendingArgs := append([]any{}, scopeArgs...)
	pendingFromIdx := len(pendingArgs) + 1
	pendingArgs = append(pendingArgs, req.FromDate)
	pendingToIdx := len(pendingArgs) + 1
	pendingArgs = append(pendingArgs, req.ToDate)
	pendingProjectFilter := cashflowProjectFilter(&pendingArgs, req.ProjectID)
	pendingDateFilter := fmt.Sprintf(" AND %s >= $%d::date AND %s < ($%d::date + INTERVAL '1 day')", cashflowOccurredAtSQL(), pendingFromIdx, cashflowOccurredAtSQL(), pendingToIdx)
	pendingRows, err := r.Query(ctx, cashflowRowsCTE(scopeSQL, pendingProjectFilter, pendingDateFilter, " AND pay.status NOT IN ('approved','cancelled')")+`
		SELECT COUNT(*) AS pending_payment_count, COALESCE(SUM(amount), 0) AS pending_payment_total
		FROM cashflow_rows
	`, pendingArgs...)
	if err != nil {
		return nil, err
	}
	if len(pendingRows) > 0 {
		summary["pending_payment_count"] = pendingRows[0]["pending_payment_count"]
		summary["pending_payment_total"] = pendingRows[0]["pending_payment_total"]
	}

	periodExpr := cashflowPeriodExpr(req.GroupBy)
	seriesRows, err := r.Query(ctx, rangeCTE+fmt.Sprintf(`
		SELECT
			%s AS period_start,
			COALESCE(SUM(inflow_amount), 0) AS inflow_amount,
			COALESCE(SUM(outflow_amount), 0) AS outflow_amount,
			COALESCE(SUM(net_amount), 0) AS net_amount,
			COUNT(*) AS payment_count
		FROM cashflow_rows
		GROUP BY period_start
		ORDER BY period_start
	`, periodExpr), rangeArgs...)
	if err != nil {
		return nil, err
	}
	runningBalance := openingBalance
	for _, row := range seriesRows {
		runningBalance += number(row["net_amount"])
		row["running_balance"] = runningBalance
	}

	categoryRows, err := r.Query(ctx, rangeCTE+`
		SELECT
			category,
			direction,
			COUNT(*) AS payment_count,
			COALESCE(SUM(inflow_amount), 0) AS inflow_amount,
			COALESCE(SUM(outflow_amount), 0) AS outflow_amount,
			COALESCE(SUM(net_amount), 0) AS net_amount
		FROM cashflow_rows
		GROUP BY category, direction
		ORDER BY direction, category
	`, rangeArgs...)
	if err != nil {
		return nil, err
	}

	ledgerRows, err := r.Query(ctx, rangeCTE+`
		SELECT
			occurred_at,
			id,
			project_id,
			project_name,
			payment_code,
			payment_type,
			payment_method,
			status,
			direction,
			category,
			amount,
			inflow_amount,
			outflow_amount,
			net_amount,
			target_type,
			target_id,
			created_by_name,
			approved_by_name
		FROM cashflow_rows
		ORDER BY occurred_at DESC, created_at DESC, id DESC
	`, rangeArgs...)
	if err != nil {
		return nil, err
	}

	return Record{
		"summary":    summary,
		"series":     seriesRows,
		"categories": categoryRows,
		"ledger":     ledgerRows,
	}, nil
}

func (r *Repository) TaxReport(ctx context.Context, req TaxReportRequest, userID string) (Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, err
	}
	args := append([]any{}, scopeArgs...)
	fromIdx := len(args) + 1
	args = append(args, req.FromDate)
	toIdx := len(args) + 1
	args = append(args, req.ToDate)
	projectFilter := taxProjectFilter(&args, req.ProjectID)
	inputDateFilter := fmt.Sprintf(" AND ii.invoice_date >= $%d::date AND ii.invoice_date <= $%d::date", fromIdx, toIdx)
	outputDateFilter := fmt.Sprintf(" AND oi.invoice_date >= $%d::date AND oi.invoice_date <= $%d::date", fromIdx, toIdx)
	sourceFilter := taxSourceFilter(req.Source)
	statusFilter := taxStatusFilter(req.Status)
	baseCTE := taxRowsCTE(scopeSQL, projectFilter, inputDateFilter, outputDateFilter)
	filteredCTE := baseCTE + fmt.Sprintf(`
		, filtered_tax_rows AS (
			SELECT * FROM tax_rows
			WHERE 1=1
			%s
			%s
		)
	`, sourceFilter, statusFilter)

	summaryRows, err := r.Query(ctx, filteredCTE+`
		SELECT
			COUNT(*) AS invoice_count,
			COUNT(*) FILTER (WHERE direction='input') AS input_invoice_count,
			COUNT(*) FILTER (WHERE direction='output') AS output_invoice_count,
			COALESCE(SUM(amount_before_vat) FILTER (WHERE direction='input'), 0) AS input_taxable_amount,
			COALESCE(SUM(vat_amount) FILTER (WHERE direction='input'), 0) AS input_vat_amount,
			COALESCE(SUM(total_amount) FILTER (WHERE direction='input'), 0) AS input_total_amount,
			COALESCE(SUM(amount_before_vat) FILTER (WHERE direction='output'), 0) AS output_taxable_amount,
			COALESCE(SUM(vat_amount) FILTER (WHERE direction='output'), 0) AS output_vat_amount,
			COALESCE(SUM(total_amount) FILTER (WHERE direction='output'), 0) AS output_total_amount,
			COALESCE(SUM(vat_amount) FILTER (WHERE direction='output'), 0) - COALESCE(SUM(vat_amount) FILTER (WHERE direction='input'), 0) AS net_vat_amount,
			COUNT(*) FILTER (WHERE missing_tax_code) AS missing_tax_code_count,
			COUNT(*) FILTER (WHERE vat_math_mismatch) AS vat_math_mismatch_count
		FROM filtered_tax_rows
	`, args...)
	if err != nil {
		return nil, err
	}
	summary := Record{
		"from":                    req.FromDate,
		"to":                      req.ToDate,
		"group_by":                req.GroupBy,
		"source":                  req.Source,
		"status":                  req.Status,
		"project_id":              req.ProjectID,
		"invoice_count":           0,
		"input_invoice_count":     0,
		"output_invoice_count":    0,
		"input_taxable_amount":    0,
		"input_vat_amount":        0,
		"input_total_amount":      0,
		"output_taxable_amount":   0,
		"output_vat_amount":       0,
		"output_total_amount":     0,
		"net_vat_amount":          0,
		"net_vat_status":          "neutral",
		"missing_tax_code_count":  0,
		"vat_math_mismatch_count": 0,
		"excluded_unposted_count": 0,
		"excluded_unposted_total": 0,
		"warning_count":           0,
	}
	if len(summaryRows) > 0 {
		for key, value := range summaryRows[0] {
			summary[key] = value
		}
	}

	excludedRows, err := r.Query(ctx, baseCTE+fmt.Sprintf(`
		SELECT COUNT(*) AS excluded_unposted_count, COALESCE(SUM(total_amount), 0) AS excluded_unposted_total
		FROM tax_rows
		WHERE 1=1
		%s
		AND NOT posted
	`, sourceFilter), args...)
	if err != nil {
		return nil, err
	}
	if len(excludedRows) > 0 {
		summary["excluded_unposted_count"] = excludedRows[0]["excluded_unposted_count"]
		summary["excluded_unposted_total"] = excludedRows[0]["excluded_unposted_total"]
	}

	netVAT := number(summary["net_vat_amount"])
	if netVAT > 1 {
		summary["net_vat_status"] = "payable"
	} else if netVAT < -1 {
		summary["net_vat_status"] = "credit"
	}
	warnings := taxReportWarnings(summary, req.Status)
	summary["warning_count"] = len(warnings)

	periodRows, err := r.Query(ctx, filteredCTE+fmt.Sprintf(`
		SELECT
			%s AS period_start,
			COUNT(*) AS invoice_count,
			COUNT(*) FILTER (WHERE direction='input') AS input_invoice_count,
			COUNT(*) FILTER (WHERE direction='output') AS output_invoice_count,
			COALESCE(SUM(amount_before_vat) FILTER (WHERE direction='input'), 0) AS input_taxable_amount,
			COALESCE(SUM(vat_amount) FILTER (WHERE direction='input'), 0) AS input_vat_amount,
			COALESCE(SUM(total_amount) FILTER (WHERE direction='input'), 0) AS input_total_amount,
			COALESCE(SUM(amount_before_vat) FILTER (WHERE direction='output'), 0) AS output_taxable_amount,
			COALESCE(SUM(vat_amount) FILTER (WHERE direction='output'), 0) AS output_vat_amount,
			COALESCE(SUM(total_amount) FILTER (WHERE direction='output'), 0) AS output_total_amount,
			COALESCE(SUM(vat_amount) FILTER (WHERE direction='output'), 0) - COALESCE(SUM(vat_amount) FILTER (WHERE direction='input'), 0) AS net_vat_amount
		FROM filtered_tax_rows
		GROUP BY period_start
		ORDER BY period_start
	`, taxPeriodExpr(req.GroupBy)), args...)
	if err != nil {
		return nil, err
	}

	counterpartyRows, err := r.Query(ctx, filteredCTE+`
		SELECT
			direction,
			tax_code,
			counterparty_name,
			COUNT(*) AS invoice_count,
			COALESCE(SUM(amount_before_vat), 0) AS taxable_amount,
			COALESCE(SUM(vat_amount), 0) AS vat_amount,
			COALESCE(SUM(total_amount), 0) AS total_amount,
			COUNT(*) FILTER (WHERE missing_tax_code OR vat_math_mismatch) AS warning_count
		FROM filtered_tax_rows
		GROUP BY direction, tax_code, counterparty_name
		ORDER BY direction, vat_amount DESC, counterparty_name
	`, args...)
	if err != nil {
		return nil, err
	}

	ledgerRows, err := r.Query(ctx, filteredCTE+`
		SELECT
			invoice_date,
			id,
			direction,
			project_id,
			project_name,
			invoice_number,
			invoice_symbol,
			counterparty_name,
			tax_code,
			amount_before_vat,
			vat_amount,
			total_amount,
			currency,
			status,
			source_type,
			source_id,
			source_code,
			secondary_source_type,
			secondary_source_id,
			secondary_source_code,
			document_id,
			document_name,
			missing_tax_code,
			vat_math_mismatch,
			CONCAT_WS('; ',
				CASE WHEN missing_tax_code THEN 'missing_tax_code' END,
				CASE WHEN vat_math_mismatch THEN 'vat_math_mismatch' END
			) AS warning_flags
		FROM filtered_tax_rows
		ORDER BY invoice_date DESC, created_at DESC, id DESC
	`, args...)
	if err != nil {
		return nil, err
	}

	return Record{
		"summary":        summary,
		"periods":        periodRows,
		"counterparties": counterpartyRows,
		"ledger":         ledgerRows,
		"warnings":       warnings,
	}, nil
}

func (r *Repository) ProfitReport(ctx context.Context, req ProfitReportRequest, userID string) (Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, err
	}
	args := append([]any{}, scopeArgs...)
	fromIdx := len(args) + 1
	args = append(args, req.FromDate)
	toIdx := len(args) + 1
	args = append(args, req.ToDate)
	projectFilter := profitProjectFilter(&args, req.ProjectID)
	inputDateFilter := fmt.Sprintf(" AND ii.invoice_date >= $%d::date AND ii.invoice_date <= $%d::date", fromIdx, toIdx)
	outputDateFilter := fmt.Sprintf(" AND oi.invoice_date >= $%d::date AND oi.invoice_date <= $%d::date", fromIdx, toIdx)
	statusFilter := profitStatusFilter(req.Status)
	baseCTE := profitRowsCTE(scopeSQL, projectFilter, inputDateFilter, outputDateFilter)
	filteredCTE := baseCTE + fmt.Sprintf(`
		, filtered_profit_rows AS (
			SELECT * FROM profit_rows
			WHERE 1=1
			%s
		)
	`, statusFilter)

	summaryRows, err := r.Query(ctx, filteredCTE+`
		SELECT
			COUNT(*) AS ledger_count,
			COUNT(*) FILTER (WHERE direction='revenue') AS revenue_invoice_count,
			COUNT(*) FILTER (WHERE direction='cost') AS cost_invoice_count,
			COALESCE(SUM(revenue_amount), 0) AS revenue_amount,
			COALESCE(SUM(direct_cost_amount), 0) AS direct_cost_amount,
			COALESCE(SUM(indirect_cost_amount), 0) AS indirect_cost_amount,
			COALESCE(SUM(direct_cost_amount + indirect_cost_amount), 0) AS total_cost_amount,
			COALESCE(SUM(revenue_amount - direct_cost_amount), 0) AS gross_profit,
			COALESCE(SUM(revenue_amount - direct_cost_amount - indirect_cost_amount), 0) AS net_profit,
			COUNT(*) FILTER (WHERE unclassified_cost) AS unclassified_cost_count
		FROM filtered_profit_rows
	`, args...)
	if err != nil {
		return nil, err
	}
	summary := Record{
		"from":                    req.FromDate,
		"to":                      req.ToDate,
		"group_by":                req.GroupBy,
		"status":                  req.Status,
		"project_id":              req.ProjectID,
		"ledger_count":            0,
		"revenue_invoice_count":   0,
		"cost_invoice_count":      0,
		"revenue_amount":          0,
		"direct_cost_amount":      0,
		"indirect_cost_amount":    0,
		"total_cost_amount":       0,
		"gross_profit":            0,
		"net_profit":              0,
		"gross_margin_percent":    0,
		"net_margin_percent":      0,
		"unclassified_cost_count": 0,
		"excluded_unposted_count": 0,
		"excluded_unposted_total": 0,
		"warning_count":           0,
	}
	if len(summaryRows) > 0 {
		for key, value := range summaryRows[0] {
			summary[key] = value
		}
	}
	profitApplyMargins(summary)

	excludedRows, err := r.Query(ctx, baseCTE+`
		SELECT COUNT(*) AS excluded_unposted_count, COALESCE(SUM(amount_before_vat), 0) AS excluded_unposted_total
		FROM profit_rows
		WHERE NOT posted
	`, args...)
	if err != nil {
		return nil, err
	}
	if len(excludedRows) > 0 {
		summary["excluded_unposted_count"] = excludedRows[0]["excluded_unposted_count"]
		summary["excluded_unposted_total"] = excludedRows[0]["excluded_unposted_total"]
	}
	warnings := profitReportWarnings(summary, req.Status)
	summary["warning_count"] = len(warnings)

	periodRows, err := r.Query(ctx, filteredCTE+fmt.Sprintf(`
		SELECT
			%s AS period_start,
			COUNT(*) AS ledger_count,
			COUNT(*) FILTER (WHERE direction='revenue') AS revenue_invoice_count,
			COUNT(*) FILTER (WHERE direction='cost') AS cost_invoice_count,
			COALESCE(SUM(revenue_amount), 0) AS revenue_amount,
			COALESCE(SUM(direct_cost_amount), 0) AS direct_cost_amount,
			COALESCE(SUM(indirect_cost_amount), 0) AS indirect_cost_amount,
			COALESCE(SUM(direct_cost_amount + indirect_cost_amount), 0) AS total_cost_amount,
			COALESCE(SUM(revenue_amount - direct_cost_amount), 0) AS gross_profit,
			COALESCE(SUM(revenue_amount - direct_cost_amount - indirect_cost_amount), 0) AS net_profit
		FROM filtered_profit_rows
		GROUP BY period_start
		ORDER BY period_start
	`, profitPeriodExpr(req.GroupBy)), args...)
	if err != nil {
		return nil, err
	}
	for _, row := range periodRows {
		profitApplyMargins(row)
	}

	projectRows, err := r.Query(ctx, filteredCTE+`
		SELECT
			project_id,
			project_name,
			COUNT(*) AS ledger_count,
			COUNT(*) FILTER (WHERE direction='revenue') AS revenue_invoice_count,
			COUNT(*) FILTER (WHERE direction='cost') AS cost_invoice_count,
			COALESCE(SUM(revenue_amount), 0) AS revenue_amount,
			COALESCE(SUM(direct_cost_amount), 0) AS direct_cost_amount,
			COALESCE(SUM(indirect_cost_amount), 0) AS indirect_cost_amount,
			COALESCE(SUM(direct_cost_amount + indirect_cost_amount), 0) AS total_cost_amount,
			COALESCE(SUM(revenue_amount - direct_cost_amount), 0) AS gross_profit,
			COALESCE(SUM(revenue_amount - direct_cost_amount - indirect_cost_amount), 0) AS net_profit,
			COUNT(*) FILTER (WHERE unclassified_cost) AS warning_count
		FROM filtered_profit_rows
		GROUP BY project_id, project_name
		ORDER BY net_profit DESC, project_name
	`, args...)
	if err != nil {
		return nil, err
	}
	for _, row := range projectRows {
		profitApplyMargins(row)
	}

	breakdownRows, err := r.Query(ctx, filteredCTE+`
		SELECT
			cost_type,
			cost_group,
			classification_reason,
			COUNT(*) AS invoice_count,
			COALESCE(SUM(amount_before_vat), 0) AS cost_amount,
			COALESCE(SUM(vat_amount), 0) AS vat_amount,
			COALESCE(SUM(total_amount), 0) AS total_amount,
			COUNT(*) FILTER (WHERE unclassified_cost) AS warning_count
		FROM filtered_profit_rows
		WHERE direction='cost'
		GROUP BY cost_type, cost_group, classification_reason
		ORDER BY cost_type, cost_amount DESC, cost_group
	`, args...)
	if err != nil {
		return nil, err
	}

	ledgerRows, err := r.Query(ctx, filteredCTE+`
		SELECT
			invoice_date,
			id,
			direction,
			cost_type,
			cost_group,
			classification_reason,
			project_id,
			project_name,
			invoice_number,
			invoice_symbol,
			counterparty_name,
			tax_code,
			amount_before_vat,
			vat_amount,
			total_amount,
			revenue_amount,
			direct_cost_amount,
			indirect_cost_amount,
			currency,
			status,
			source_type,
			source_id,
			source_code,
			secondary_source_type,
			secondary_source_id,
			secondary_source_code,
			document_id,
			document_name,
			unclassified_cost,
			CASE WHEN unclassified_cost THEN 'indirect_or_unclassified_cost' ELSE '' END AS warning_flags
		FROM filtered_profit_rows
		ORDER BY invoice_date DESC, created_at DESC, id DESC
	`, args...)
	if err != nil {
		return nil, err
	}

	return Record{
		"summary":        summary,
		"periods":        periodRows,
		"projects":       projectRows,
		"cost_breakdown": breakdownRows,
		"ledger":         ledgerRows,
		"warnings":       warnings,
	}, nil
}

func profitApplyMargins(row Record) {
	revenue := number(row["revenue_amount"])
	row["gross_margin_percent"] = profitMarginPercent(revenue, number(row["gross_profit"]))
	row["net_margin_percent"] = profitMarginPercent(revenue, number(row["net_profit"]))
}

func profitMarginPercent(revenue, profit float64) float64 {
	if math.Abs(revenue) <= 1 {
		return 0
	}
	return profit / revenue * 100
}

func profitReportWarnings(summary Record, status string) []Record {
	warnings := []Record{}
	if count := number(summary["unclassified_cost_count"]); count > 0 {
		warnings = append(warnings, Record{
			"code":     "indirect_or_unclassified_cost",
			"severity": "warning",
			"message":  "Có chi phí chưa đủ nguồn phân loại direct nên được đưa vào indirect cost",
			"count":    count,
		})
	}
	if status == "posted" {
		if count := number(summary["excluded_unposted_count"]); count > 0 {
			warnings = append(warnings, Record{
				"code":     "unposted_excluded",
				"severity": "info",
				"message":  "Có chứng từ chưa posted bị loại khỏi báo cáo mặc định",
				"count":    count,
				"amount":   summary["excluded_unposted_total"],
			})
		}
	}
	if number(summary["revenue_amount"]) > 0 && number(summary["net_profit"]) < 0 {
		warnings = append(warnings, Record{
			"code":     "negative_net_profit",
			"severity": "warning",
			"message":  "Net profit đang âm trong khoảng lọc",
			"amount":   summary["net_profit"],
		})
	}
	return warnings
}

func taxReportWarnings(summary Record, status string) []Record {
	warnings := []Record{}
	if count := number(summary["missing_tax_code_count"]); count > 0 {
		warnings = append(warnings, Record{
			"code":     "missing_tax_code",
			"severity": "warning",
			"message":  "Có hoá đơn thiếu mã số thuế đối tác",
			"count":    count,
		})
	}
	if count := number(summary["vat_math_mismatch_count"]); count > 0 {
		warnings = append(warnings, Record{
			"code":     "vat_math_mismatch",
			"severity": "warning",
			"message":  "Có hoá đơn lệch phép tính trước VAT + VAT = tổng tiền",
			"count":    count,
		})
	}
	if status == "posted" {
		if count := number(summary["excluded_unposted_count"]); count > 0 {
			warnings = append(warnings, Record{
				"code":     "unposted_excluded",
				"severity": "info",
				"message":  "Có hoá đơn chưa posted bị loại khỏi báo cáo mặc định",
				"count":    count,
				"amount":   summary["excluded_unposted_total"],
			})
		}
	}
	return warnings
}

func taxRowsCTE(scopeSQL, projectFilter, inputDateFilter, outputDateFilter string) string {
	return fmt.Sprintf(`
		WITH tax_rows AS (
			SELECT
				'input'::text AS direction,
				ii.id::text AS id,
				ii.project_id::text AS project_id,
				p.name AS project_name,
				ii.invoice_number,
				COALESCE(ii.invoice_symbol, '') AS invoice_symbol,
				ii.invoice_date,
				ii.vendor_name AS counterparty_name,
				COALESCE(ii.vendor_tax_code, '') AS tax_code,
				ii.amount_before_vat,
				ii.vat_amount,
				ii.total_amount,
				ii.currency,
				ii.status,
				(ii.status='verified') AS posted,
				'payable'::text AS source_type,
				ii.payable_id::text AS source_id,
				COALESCE(payable.payable_code, '') AS source_code,
				'procurement_request'::text AS secondary_source_type,
				ii.procurement_request_id::text AS secondary_source_id,
				COALESCE(procurement.request_code, '') AS secondary_source_code,
				ii.document_id::text AS document_id,
				COALESCE(doc.name, '') AS document_name,
				(COALESCE(ii.vendor_tax_code, '') = '') AS missing_tax_code,
				(ABS((COALESCE(ii.amount_before_vat, 0) + COALESCE(ii.vat_amount, 0)) - COALESCE(ii.total_amount, 0)) > 1) AS vat_math_mismatch,
				ii.created_at
			FROM input_invoices ii
			JOIN projects p ON p.id=ii.project_id
			LEFT JOIN payables payable ON payable.id=ii.payable_id
			LEFT JOIN procurement_requests procurement ON procurement.id=ii.procurement_request_id
			LEFT JOIN documents doc ON doc.id=ii.document_id
			WHERE ii.status <> 'deleted'
			  AND p.status <> 'cancelled'
			  AND %s
			  %s
			  %s
			UNION ALL
			SELECT
				'output'::text AS direction,
				oi.id::text AS id,
				oi.project_id::text AS project_id,
				p.name AS project_name,
				oi.invoice_number,
				COALESCE(oi.invoice_symbol, '') AS invoice_symbol,
				oi.invoice_date,
				oi.client_name AS counterparty_name,
				COALESCE(oi.client_tax_code, '') AS tax_code,
				oi.amount_before_vat,
				oi.vat_amount,
				oi.total_amount,
				oi.currency,
				oi.status,
				(oi.status='issued') AS posted,
				'receivable'::text AS source_type,
				oi.receivable_id::text AS source_id,
				COALESCE(receivable.receivable_code, '') AS source_code,
				'payment'::text AS secondary_source_type,
				oi.payment_id::text AS secondary_source_id,
				COALESCE(payment.payment_code, '') AS secondary_source_code,
				oi.document_id::text AS document_id,
				COALESCE(doc.name, '') AS document_name,
				(COALESCE(oi.client_tax_code, '') = '') AS missing_tax_code,
				(ABS((COALESCE(oi.amount_before_vat, 0) + COALESCE(oi.vat_amount, 0)) - COALESCE(oi.total_amount, 0)) > 1) AS vat_math_mismatch,
				oi.created_at
			FROM output_invoices oi
			JOIN projects p ON p.id=oi.project_id
			LEFT JOIN receivables receivable ON receivable.id=oi.receivable_id
			LEFT JOIN payments payment ON payment.id=oi.payment_id
			LEFT JOIN documents doc ON doc.id=oi.document_id
			WHERE oi.status <> 'deleted'
			  AND p.status <> 'cancelled'
			  AND %s
			  %s
			  %s
		)
	`, scopeSQL, projectFilter, inputDateFilter, scopeSQL, projectFilter, outputDateFilter)
}

func taxSourceFilter(source string) string {
	switch source {
	case "input":
		return " AND direction='input'"
	case "output":
		return " AND direction='output'"
	default:
		return ""
	}
}

func taxStatusFilter(status string) string {
	if status == "posted" {
		return " AND posted"
	}
	return ""
}

func taxProjectFilter(args *[]any, projectID string) string {
	projectID = cleanID(projectID)
	if projectID == "" {
		return ""
	}
	*args = append(*args, projectID)
	return fmt.Sprintf(" AND p.id=$%d::uuid", len(*args))
}

func taxPeriodExpr(groupBy string) string {
	if groupBy == "quarter" {
		return "date_trunc('quarter', invoice_date)::date"
	}
	return "date_trunc('month', invoice_date)::date"
}

func profitRowsCTE(scopeSQL, projectFilter, inputDateFilter, outputDateFilter string) string {
	directExpr := `
		ii.procurement_request_id IS NOT NULL
		OR lower(COALESCE(payable.source_type, '')) IN ('procurement','boq','boq_item','work_item','acceptance','incoming_acceptance','outgoing_acceptance','operation','rework')
		OR lower(COALESCE(payable.vendor_type, '')) IN ('supplier','subcontractor')
	`
	return fmt.Sprintf(`
		WITH input_rows AS (
			SELECT
				ii.id::text AS id,
				ii.project_id::text AS project_id,
				p.name AS project_name,
				ii.invoice_number,
				COALESCE(ii.invoice_symbol, '') AS invoice_symbol,
				ii.invoice_date,
				ii.vendor_name AS counterparty_name,
				COALESCE(ii.vendor_tax_code, '') AS tax_code,
				ii.amount_before_vat,
				ii.vat_amount,
				ii.total_amount,
				ii.currency,
				ii.status,
				(ii.status='verified') AS posted,
				'payable'::text AS source_type,
				ii.payable_id::text AS source_id,
				COALESCE(payable.payable_code, '') AS source_code,
				'procurement_request'::text AS secondary_source_type,
				ii.procurement_request_id::text AS secondary_source_id,
				COALESCE(procurement.request_code, '') AS secondary_source_code,
				ii.document_id::text AS document_id,
				COALESCE(doc.name, '') AS document_name,
				(%s) AS direct_cost,
				CASE
					WHEN ii.procurement_request_id IS NOT NULL OR lower(COALESCE(payable.source_type, '')) IN ('procurement','boq','boq_item') THEN 'Procurement / BOQ'
					WHEN lower(COALESCE(payable.vendor_type, ''))='subcontractor' THEN 'Thầu phụ'
					WHEN lower(COALESCE(payable.vendor_type, ''))='supplier' THEN 'Nhà cung cấp'
					WHEN COALESCE(payable.source_type, '') <> '' THEN 'Source khác'
					ELSE 'Chi phí gián tiếp'
				END AS cost_group,
				CASE
					WHEN ii.procurement_request_id IS NOT NULL THEN 'linked_procurement_invoice'
					WHEN lower(COALESCE(payable.source_type, '')) IN ('procurement','boq','boq_item','work_item','acceptance','incoming_acceptance','outgoing_acceptance','operation','rework') THEN 'payable_source_direct'
					WHEN lower(COALESCE(payable.vendor_type, '')) IN ('supplier','subcontractor') THEN 'vendor_type_direct'
					WHEN COALESCE(payable.source_type, '') <> '' THEN 'payable_source_indirect'
					ELSE 'indirect_fallback'
				END AS classification_reason,
				ii.created_at
			FROM input_invoices ii
			JOIN projects p ON p.id=ii.project_id
			LEFT JOIN payables payable ON payable.id=ii.payable_id
			LEFT JOIN procurement_requests procurement ON procurement.id=ii.procurement_request_id
			LEFT JOIN documents doc ON doc.id=ii.document_id
			WHERE ii.status <> 'deleted'
			  AND p.status <> 'cancelled'
			  AND %s
			  %s
			  %s
		),
		output_rows AS (
			SELECT
				oi.id::text AS id,
				oi.project_id::text AS project_id,
				p.name AS project_name,
				oi.invoice_number,
				COALESCE(oi.invoice_symbol, '') AS invoice_symbol,
				oi.invoice_date,
				oi.client_name AS counterparty_name,
				COALESCE(oi.client_tax_code, '') AS tax_code,
				oi.amount_before_vat,
				oi.vat_amount,
				oi.total_amount,
				oi.currency,
				oi.status,
				(oi.status='issued') AS posted,
				'receivable'::text AS source_type,
				oi.receivable_id::text AS source_id,
				COALESCE(receivable.receivable_code, '') AS source_code,
				'payment'::text AS secondary_source_type,
				oi.payment_id::text AS secondary_source_id,
				COALESCE(payment.payment_code, '') AS secondary_source_code,
				oi.document_id::text AS document_id,
				COALESCE(doc.name, '') AS document_name,
				oi.created_at
			FROM output_invoices oi
			JOIN projects p ON p.id=oi.project_id
			LEFT JOIN receivables receivable ON receivable.id=oi.receivable_id
			LEFT JOIN payments payment ON payment.id=oi.payment_id
			LEFT JOIN documents doc ON doc.id=oi.document_id
			WHERE oi.status <> 'deleted'
			  AND p.status <> 'cancelled'
			  AND %s
			  %s
			  %s
		),
		profit_rows AS (
			SELECT
				'revenue'::text AS direction,
				'revenue'::text AS cost_type,
				'Revenue'::text AS cost_group,
				'issued_output_invoice'::text AS classification_reason,
				id,
				project_id,
				project_name,
				invoice_number,
				invoice_symbol,
				invoice_date,
				counterparty_name,
				tax_code,
				amount_before_vat,
				vat_amount,
				total_amount,
				amount_before_vat AS revenue_amount,
				0::numeric AS direct_cost_amount,
				0::numeric AS indirect_cost_amount,
				currency,
				status,
				posted,
				source_type,
				source_id,
				source_code,
				secondary_source_type,
				secondary_source_id,
				secondary_source_code,
				document_id,
				document_name,
				false AS unclassified_cost,
				created_at
			FROM output_rows
			UNION ALL
			SELECT
				'cost'::text AS direction,
				CASE WHEN direct_cost THEN 'direct' ELSE 'indirect' END AS cost_type,
				cost_group,
				classification_reason,
				id,
				project_id,
				project_name,
				invoice_number,
				invoice_symbol,
				invoice_date,
				counterparty_name,
				tax_code,
				amount_before_vat,
				vat_amount,
				total_amount,
				0::numeric AS revenue_amount,
				CASE WHEN direct_cost THEN amount_before_vat ELSE 0 END AS direct_cost_amount,
				CASE WHEN direct_cost THEN 0 ELSE amount_before_vat END AS indirect_cost_amount,
				currency,
				status,
				posted,
				source_type,
				source_id,
				source_code,
				secondary_source_type,
				secondary_source_id,
				secondary_source_code,
				document_id,
				document_name,
				NOT direct_cost AS unclassified_cost,
				created_at
			FROM input_rows
		)
	`, directExpr, scopeSQL, projectFilter, inputDateFilter, scopeSQL, projectFilter, outputDateFilter)
}

func profitStatusFilter(status string) string {
	if status == "posted" {
		return " AND posted"
	}
	return ""
}

func profitProjectFilter(args *[]any, projectID string) string {
	projectID = cleanID(projectID)
	if projectID == "" {
		return ""
	}
	*args = append(*args, projectID)
	return fmt.Sprintf(" AND p.id=$%d::uuid", len(*args))
}

func profitPeriodExpr(groupBy string) string {
	if groupBy == "quarter" {
		return "date_trunc('quarter', invoice_date)::date"
	}
	return "date_trunc('month', invoice_date)::date"
}

func cashflowRowsCTE(scopeSQL, projectFilter, dateFilter, statusFilter string) string {
	return fmt.Sprintf(`
		WITH classified AS (
			SELECT
				pay.*,
				p.name AS project_name,
				COALESCE(created.full_name, '') AS created_by_name,
				COALESCE(approved.full_name, '') AS approved_by_name,
				%s AS occurred_at,
				CASE
					WHEN pay.payment_type IN ('client_receipt','inbound') OR (pay.payment_type='advance' AND pay.target_type='receivable') THEN 'inflow'
					WHEN pay.payment_type IN ('supplier_payment','subcontractor_payment','outbound') OR (pay.payment_type='advance' AND pay.target_type='payable') THEN 'outflow'
					WHEN pay.payment_type='advance_settlement' THEN 'non_cash'
					ELSE 'unclassified'
				END AS direction,
				CASE
					WHEN pay.payment_type='client_receipt' THEN 'Thu khách hàng'
					WHEN pay.payment_type='inbound' THEN 'Thu khác'
					WHEN pay.payment_type='advance' AND pay.target_type='receivable' THEN 'Thu ứng trước khách hàng'
					WHEN pay.payment_type='supplier_payment' THEN 'Thanh toán nhà cung cấp'
					WHEN pay.payment_type='subcontractor_payment' THEN 'Thanh toán thầu phụ'
					WHEN pay.payment_type='outbound' THEN 'Phiếu chi'
					WHEN pay.payment_type='advance' AND pay.target_type='payable' THEN 'Chi ứng trước'
					WHEN pay.payment_type='advance_settlement' THEN 'Quyết toán tạm ứng'
					ELSE 'Unclassified'
				END AS category
			FROM payments pay
			JOIN projects p ON p.id=pay.project_id
			LEFT JOIN users created ON created.id=pay.created_by
			LEFT JOIN users approved ON approved.id=pay.approved_by
			WHERE pay.status <> 'cancelled'
			  AND p.status <> 'cancelled'
			  AND %s
			  %s
			  %s
			  %s
		),
		cashflow_rows AS (
			SELECT
				classified.*,
				CASE WHEN direction='inflow' THEN COALESCE(amount,0) ELSE 0 END AS inflow_amount,
				CASE WHEN direction='outflow' THEN COALESCE(amount,0) ELSE 0 END AS outflow_amount,
				CASE
					WHEN direction='inflow' THEN COALESCE(amount,0)
					WHEN direction='outflow' THEN -COALESCE(amount,0)
					ELSE 0
				END AS net_amount
			FROM classified
		)
	`, cashflowOccurredAtSQL(), scopeSQL, projectFilter, dateFilter, statusFilter)
}

func cashflowOccurredAtSQL() string {
	return "COALESCE(pay.paid_at, pay.updated_at, pay.created_at)"
}

func cashflowStatusFilter(status string) string {
	if status == "approved" {
		return " AND pay.status='approved'"
	}
	return ""
}

func cashflowProjectFilter(args *[]any, projectID string) string {
	projectID = cleanID(projectID)
	if projectID == "" {
		return ""
	}
	*args = append(*args, projectID)
	return fmt.Sprintf(" AND p.id=$%d::uuid", len(*args))
}

func cashflowPeriodExpr(groupBy string) string {
	switch groupBy {
	case "week":
		return "date_trunc('week', occurred_at)::date"
	case "month":
		return "date_trunc('month', occurred_at)::date"
	default:
		return "date_trunc('day', occurred_at)::date"
	}
}

func queryTx(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]Record, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func number(value any) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	case float32:
		return float64(v)
	case string:
		out, _ := strconv.ParseFloat(v, 64)
		if math.IsNaN(out) {
			return 0
		}
		return out
	default:
		return 0
	}
}

func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		out, _ := strconv.ParseBool(v)
		return out
	default:
		return false
	}
}

func stringsJoin(values []string) string {
	return strings.Join(values, "; ")
}

func timeNowUnix() int64 {
	return time.Now().Unix()
}
