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
	"sort"
	"strconv"
	"strings"
	"time"
)

const clientOfficialWorkflowTemplateID = "40000000-0000-0000-0000-000000000020"

type workflowAutomationOutcome struct {
	ActionKey     string
	Status        string
	Message       string
	TargetType    string
	TargetID      string
	BlockerReason string
	ErrorMessage  string
	Metadata      Record
}

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
			return nil, validationError("Quyền không tồn tại", Record{"permission_id": permissionID})
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
			return nil, validationError("Vai trò không tồn tại", Record{"role_id": roleID})
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
	`, profile.IsAdmin, profile.IsDirector, profile.UserID, profile.StaffID)
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

func (r *Repository) canAccessWorkItem(ctx context.Context, workItemID, userID string, includeWatchers, includeCancelled bool) (bool, error) {
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

func (r *Repository) ListProjectWorkItems(ctx context.Context, projectID, userID string) ([]Record, error) {
	return r.listProjectWorkItems(ctx, projectID, userID, false)
}

func (r *Repository) ListProjectOperationsWorkItems(ctx context.Context, projectID, userID string) ([]Record, error) {
	return r.listProjectWorkItems(ctx, projectID, userID, true)
}

func (r *Repository) listProjectWorkItems(ctx context.Context, projectID, userID string, includeCancelled bool) ([]Record, error) {
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	args := append([]any{projectID}, workItemScopeArgs(profile, profile.IsManager, true)...)
	statusCondition := "wi.status <> 'cancelled'"
	if includeCancelled {
		statusCondition = "TRUE"
	}
	return r.Query(ctx, fmt.Sprintf(`
		SELECT wi.*, p.name AS project_name, p.code AS project_code,
			rn.name AS run_node_name, rn.order_index AS run_node_order_index,
			rs.name AS run_step_name, rs.order_index AS run_step_order_index,
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
			COALESCE(rs.form_required, false) AS workflow_form_required,
			rs.form_version_id AS required_workflow_form_version_id,
			rs.surface_kind,
			rs.domain_action_key,
			COALESCE(pr.engine_version, 1) AS pipeline_engine_version
		FROM work_items wi
		JOIN projects p ON p.id=wi.project_id
		LEFT JOIN pipeline_runs pr ON pr.id=wi.pipeline_run_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		LEFT JOIN staff s ON s.id=wi.assignee_staff_id
		LEFT JOIN workflow_form_instances wfi ON wfi.work_item_id=wi.id AND wfi.status <> 'cancelled'
		LEFT JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
		LEFT JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		WHERE wi.project_id=$1 AND %s AND p.status <> 'cancelled'
		  AND (%s)
		ORDER BY
			rn.order_index NULLS LAST,
			rs.order_index NULLS LAST,
			wi.due_at NULLS LAST,
			wi.created_at
	`, statusCondition, workItemScopeSQL("wi", 2)), args...)
}

func (r *Repository) ListProjectWorkflowSidebarWorkItems(ctx context.Context, projectID, userID string) ([]Record, error) {
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	args := append([]any{projectID}, workItemScopeArgs(profile, profile.IsManager, true)...)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT
			wi.id,
			wi.project_id,
			wi.run_node_id,
			wi.run_step_id,
			wi.title,
			wi.description,
			wi.work_type,
			wi.status,
			wi.role_ids,
			wi.due_at,
			wi.evidence_requirement_json,
			rn.name AS run_node_name,
			rn.order_index AS run_node_order_index,
			rs.name AS run_step_name,
			rs.order_index AS run_step_order_index,
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
			COALESCE(rs.form_required, false) AS workflow_form_required,
			rs.form_version_id AS required_workflow_form_version_id,
			rs.surface_kind,
			rs.domain_action_key,
			COALESCE(pr.engine_version, 1) AS pipeline_engine_version
		FROM work_items wi
		JOIN projects p ON p.id=wi.project_id
		LEFT JOIN pipeline_runs pr ON pr.id=wi.pipeline_run_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		LEFT JOIN workflow_form_instances wfi ON wfi.work_item_id=wi.id AND wfi.status <> 'cancelled'
		LEFT JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
		LEFT JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		WHERE wi.project_id=$1 AND p.status <> 'cancelled'
		  AND (%s)
		ORDER BY
			rn.order_index NULLS LAST,
			rs.order_index NULLS LAST,
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
			COALESCE(rs.form_required, false) AS workflow_form_required,
			rs.form_version_id AS required_workflow_form_version_id,
			rs.surface_kind,
			rs.domain_action_key,
			COALESCE(pr.engine_version, 1) AS pipeline_engine_version,
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
		LEFT JOIN pipeline_runs pr ON pr.id=wi.pipeline_run_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
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

func activityConversationSelectSQL(userExpression string) string {
	return fmt.Sprintf(`
		CASE WHEN parent_wia.id IS NULL THEN NULL ELSE jsonb_build_object(
			'id', parent_wia.id,
			'actor_id', parent_wia.actor_id,
			'actor_name', COALESCE(parent_actor.full_name, 'Hệ thống'),
			'activity_type', parent_wia.activity_type,
			'comment', CASE WHEN parent_wia.withdrawn_at IS NULL THEN parent_wia.comment ELSE NULL END,
			'withdrawn', parent_wia.withdrawn_at IS NOT NULL,
			'created_at', parent_wia.created_at
		) END AS reply_to,
		COALESCE(reply_state.reply_count, 0) AS reply_count,
		COALESCE(reaction_state.reactions, '[]'::jsonb) AS reactions,
		(wia.activity_type='comment' AND wia.actor_id=%s AND wia.withdrawn_at IS NULL AND wia.created_at >= now() - INTERVAL '15 minutes') AS can_edit,
		(wia.activity_type='comment' AND wia.actor_id=%s AND wia.withdrawn_at IS NULL AND wia.created_at >= now() - INTERVAL '15 minutes') AS can_withdraw`, userExpression, userExpression)
}

func activityConversationJoinSQL(userExpression string) string {
	return fmt.Sprintf(`
		LEFT JOIN work_item_activities parent_wia ON parent_wia.id=wia.parent_activity_id
		LEFT JOIN users parent_actor ON parent_actor.id=parent_wia.actor_id
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS reply_count
			FROM work_item_activities replies
			WHERE replies.parent_activity_id=wia.id
		) reply_state ON true
		LEFT JOIN LATERAL (
			SELECT jsonb_agg(jsonb_build_object(
				'reaction', grouped.reaction,
				'count', grouped.reaction_count,
				'reacted_by_me', grouped.reacted_by_me
			) ORDER BY grouped.reaction) AS reactions
			FROM (
				SELECT ar.reaction,
					COUNT(*) AS reaction_count,
					COALESCE(bool_or(ar.user_id=%s), false) AS reacted_by_me
				FROM activity_reactions ar
				WHERE ar.activity_id=wia.id AND ar.active AND wia.withdrawn_at IS NULL
				GROUP BY ar.reaction
			) grouped
		) reaction_state ON true
	`, userExpression)
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
		"wia.withdrawn_at IS NULL",
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
	`, activityAttachmentSelectSQL(), activityConversationSelectSQL("$1"), unread, mentioned, openQuestion, openBlocker, dueReminder, unread, activityAttachmentJoinSQL(), activityConversationJoinSQL("$1"), strings.Join(conditions, " AND "), unread, mentioned, openQuestion, openBlocker, dueReminder, limitPlaceholder), args...)
}

func (r *Repository) GetWorkItemDetail(ctx context.Context, workItemID string, clientVisibleOnly ...bool) (Record, error) {
	item, err := r.QueryOne(ctx, `
		SELECT wi.*, p.name AS project_name, p.code AS project_code, rn.name AS run_node_name, rs.name AS run_step_name,
			s.full_name AS assignee_name,
			pr.current_node_id AS pipeline_current_node_id,
			rn.status AS run_node_status,
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
			COALESCE(rs.form_required, false) AS workflow_form_required,
			rs.form_version_id AS required_workflow_form_version_id,
			rs.surface_kind,
			rs.domain_action_key,
			COALESCE(pr.engine_version, 1) AS pipeline_engine_version
		FROM work_items wi
		JOIN projects p ON p.id=wi.project_id
		LEFT JOIN pipeline_runs pr ON pr.id=wi.pipeline_run_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		LEFT JOIN staff s ON s.id=wi.assignee_staff_id
		LEFT JOIN workflow_form_instances wfi ON wfi.work_item_id=wi.id AND wfi.status <> 'cancelled'
		LEFT JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
		LEFT JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		WHERE wi.id=$1 AND p.status <> 'cancelled'
	`, workItemID)
	if err != nil {
		return nil, err
	}
	visibilitySQL := "TRUE"
	if len(clientVisibleOnly) > 0 && clientVisibleOnly[0] {
		visibilitySQL = "wia.visibility='client'"
	}
	activities, _ := r.Query(ctx, fmt.Sprintf(`
		SELECT wia.*, u.full_name AS actor_name, %s, %s
		FROM work_item_activities wia
		LEFT JOIN users u ON u.id=wia.actor_id
		%s
		%s
		WHERE wia.work_item_id=$1
		  AND (%s)
		ORDER BY wia.created_at DESC
		LIMIT 100
	`, activityAttachmentSelectSQL(), activityConversationSelectSQL("NULL::uuid"), activityAttachmentJoinSQL(), activityConversationJoinSQL("NULL::uuid"), visibilitySQL), workItemID)
	evidence, _ := r.Query(ctx, `
		SELECT er.*, u.full_name AS submitted_by_name, d.name AS document_name
		FROM evidence_records er
		LEFT JOIN users u ON u.id=er.submitted_by
		LEFT JOIN documents d ON d.id=er.document_id
		WHERE er.work_item_id=$1
		ORDER BY er.created_at DESC
	`, workItemID)
	automationEvents, _ := r.Query(ctx, `
		SELECT *
		FROM workflow_automation_events
		WHERE work_item_id=$1
		ORDER BY updated_at DESC, created_at DESC
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
	item["automation_events"] = automationEvents
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
			COALESCE(rs.form_required, false) AS workflow_form_required,
			rs.form_version_id AS required_workflow_form_version_id,
			rs.surface_kind,
			rs.domain_action_key,
			COALESCE(pr.engine_version, 1) AS pipeline_engine_version
		FROM work_items wi
		JOIN projects p ON p.id=wi.project_id
		LEFT JOIN pipeline_runs pr ON pr.id=wi.pipeline_run_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
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
	return getEvidenceDetail(ctx, r.db, evidenceID)
}

func getEvidenceDetail(ctx context.Context, db queryContext, evidenceID string) (Record, error) {
	return queryOneRecord(ctx, db, `
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
	return getWorkflowFormInstanceDetail(ctx, r.db, instanceID)
}

func getWorkflowFormInstanceDetail(ctx context.Context, db queryContext, instanceID string) (Record, error) {
	instance, err := queryOneRecord(ctx, db, `
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
	fields, err := queryRecords(ctx, db, `
		SELECT id, form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index, visibility_condition_json
		FROM workflow_form_fields
		WHERE form_version_id=$1
		ORDER BY order_index, created_at, id
	`, instance["form_version_id"])
	if err != nil {
		return nil, err
	}
	answerRows, err := queryRecords(ctx, db, `
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
	events, err := queryRecords(ctx, db, `
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
	signatures, err := queryRecords(ctx, db, `
		SELECT *
		FROM workflow_form_signatures
		WHERE form_instance_id=$1
		ORDER BY signed_at DESC, created_at DESC
	`, instanceID)
	if err != nil {
		return nil, err
	}
	pdfExports, err := queryRecords(ctx, db, `
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
		activityComment = "Đã gửi đối soát checklist thầu phụ"
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
			return validationError("Đối soát checklist thầu phụ đã gửi, không thể sửa", Record{"work_item_id": workItemID})
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
	evidenceRows, err := r.Query(ctx, `
		SELECT
			link.checklist_item_id,
			link.sort_order,
			er.id,
			er.work_item_id,
			er.evidence_type,
			er.name,
			er.caption,
			er.file_url,
			er.mime_type,
			er.file_size,
			er.visibility,
			er.status,
			er.created_at,
			wi.title AS work_item_title
		FROM acceptance_checklist_item_evidence link
		JOIN acceptance_checklist_items item ON item.id=link.checklist_item_id
		JOIN evidence_records er ON er.id=link.evidence_id
		LEFT JOIN work_items wi ON wi.id=er.work_item_id
		WHERE item.acceptance_id=$1
		ORDER BY link.checklist_item_id, link.sort_order, link.created_at, er.id
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	evidenceByItem := map[string][]Record{}
	for _, evidence := range evidenceRows {
		itemID := cleanID(evidence["checklist_item_id"])
		delete(evidence, "checklist_item_id")
		evidenceByItem[itemID] = append(evidenceByItem[itemID], evidence)
	}
	for _, item := range items {
		records := evidenceByItem[cleanID(item["id"])]
		ids := make([]string, 0, len(records))
		for _, evidence := range records {
			ids = append(ids, cleanID(evidence["id"]))
		}
		item["evidence_ids"] = ids
		item["evidence_records"] = records
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
			return nil, validationError("Hồ sơ nghiệm thu đầu ra đã gửi/chốt, không thể sửa", Record{"acceptance_id": acceptanceID, "status": existingRows[0]["status"]})
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
		rows, err := queryTx(ctx, tx, `
			INSERT INTO acceptance_checklist_items (
				acceptance_id, item_index, title, description, is_passed, note,
				photo_required, photo_url, evidence_id, decision, severity, required_action, due_date, payload_json
			)
			VALUES (
				$1, $2, $3, NULLIF($4,''), $5, NULLIF($6,''),
				$7, NULLIF($8,''), NULLIF($9,'')::uuid, $10, $11, NULLIF($12,''), NULLIF($13,'')::date, $14::jsonb
			)
			RETURNING id
		`, acceptanceID, item["item_index"], item["title"], item["description"], isPassed, item["note"],
			item["photo_required"], item["photo_url"], cleanID(item["evidence_id"]), item["decision"], item["severity"],
			item["required_action"], item["due_date"], string(rawPayload))
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		checklistItemID := cleanID(rows[0]["id"])
		evidenceIDs := stringSlice(item["evidence_ids"])
		if len(evidenceIDs) == 0 && cleanID(item["evidence_id"]) != "" {
			evidenceIDs = []string{cleanID(item["evidence_id"])}
		}
		for sortOrder, evidenceID := range evidenceIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO acceptance_checklist_item_evidence (
					checklist_item_id, evidence_id, sort_order, created_by
				)
				VALUES ($1, $2, $3, NULLIF($4,'')::uuid)
			`, checklistItemID, evidenceID, sortOrder, actorID); err != nil {
				return nil, err
			}
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
		return nil, validationError("Chỉ hồ sơ nghiệm thu nháp mới được gửi", Record{"acceptance_id": acceptanceID, "status": record["status"]})
	}
	itemRows, err := queryTx(ctx, tx, `SELECT COUNT(*) AS total FROM acceptance_checklist_items WHERE acceptance_id=$1`, acceptanceID)
	if err != nil {
		return nil, err
	}
	if len(itemRows) == 0 || number(itemRows[0]["total"]) == 0 {
		return nil, validationError("Cần ít nhất một dòng checklist để gửi nghiệm thu đầu ra")
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
		comment = "Đã gửi nghiệm thu đầu ra"
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
		activityComment = "Đã gửi đối soát checklist chủ đầu tư"
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
			return validationError("Đối soát checklist chủ đầu tư đã gửi, không thể sửa", Record{"work_item_id": workItemID})
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
	activityComment := "Đã lưu nháp ghi nhận vật tư trước/sau"
	eventType := "material_snapshot_saved"
	if submit {
		status = "submitted"
		activityComment = "Đã gửi ghi nhận vật tư trước/sau"
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
			return validationError("Ghi nhận vật tư đã gửi, không thể sửa", Record{"work_item_id": workItemID})
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
			return nil, validationError("Hồ sơ xử lý vật tư dư đã gửi, không thể sửa", Record{"surplus_handling_id": handlingID})
		}
		if cleanID(existingRows[0]["work_item_id"]) != workItemID {
			return nil, validationError("Hồ sơ xử lý vật tư dư không thuộc đầu việc", Record{"surplus_handling_id": handlingID})
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
		return nil, validationError("Hồ sơ xử lý vật tư dư đã gửi", Record{"surplus_handling_id": handlingID})
	}
	projectID := cleanID(handling["project_id"])
	workItemID := cleanID(handling["work_item_id"])
	materialID := cleanID(handling["material_id"])
	warehouseID := cleanID(handling["warehouse_id"])
	handlingType := cleanOptionalText(handling["handling_type"])
	quantity := number(handling["quantity"])
	unit := cleanOptionalText(handling["unit"])
	if warehouseID == "" || cleanID(handling["evidence_id"]) == "" || cleanOptionalText(handling["custody_location"]) == "" {
		return nil, validationError("Hồ sơ xử lý vật tư dư thiếu kho, minh chứng hoặc vị trí lưu/trả")
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
			return nil, validationError("Hồ sơ nghiệm thu đầu vào đã gửi, không thể sửa", Record{"incoming_acceptance_id": acceptanceID})
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
		return nil, validationError("Hồ sơ nghiệm thu đầu vào đã gửi", Record{"incoming_acceptance_id": acceptanceID})
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
		return nil, validationError("Cần ít nhất một dòng vật tư để gửi nghiệm thu đầu vào")
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
		comment = fmt.Sprintf("Gửi nghiệm thu đầu vào: %s", result)
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
	row, err := r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		rows, queryErr := queryTx(ctx, tx, `
			INSERT INTO workflow_form_templates (code, name, description, form_type, status, created_by)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6,'')::uuid)
			RETURNING id
		`, body["code"], body["name"], body["description"], body["form_type"], body["status"], actorID)
		if queryErr != nil {
			return nil, queryErr
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		return rows[0], nil
	}, func(row Record) auditEvent {
		return auditEvent{
			ActorID: actorID, ActorName: actorName, EventType: "workflow_form_template_created",
			TargetType: "workflow_form_template", Description: fmt.Sprintf("Tạo mẫu biểu mẫu quy trình %s", body["name"]),
			Metadata: Record{"code": body["code"], "form_type": body["form_type"]},
		}
	})
	if err != nil {
		return nil, err
	}
	templateID := cleanID(row["id"])
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
			return nil, validationError("Không thể lưu trữ mẫu biểu mẫu quy trình đang được bước quy trình sử dụng", Record{"step_usage_count": usageRows[0]["total"]})
		}
	}
	_, err := r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		rows, queryErr := queryTx(ctx, tx, `
			UPDATE workflow_form_templates
			SET name=$2,
				description=$3,
				status=$4,
				updated_at=now()
			WHERE id=$1
			RETURNING id, name
		`, templateID, body["name"], body["description"], body["status"])
		if queryErr != nil {
			return nil, queryErr
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		return rows[0], nil
	}, func(row Record) auditEvent {
		return auditEvent{
			ActorID: actorID, ActorName: actorName, EventType: "workflow_form_template_updated",
			TargetType: "workflow_form_template", TargetID: templateID,
			Description: fmt.Sprintf("Cập nhật mẫu biểu mẫu quy trình %s", row["name"]), Metadata: Record{"status": body["status"]},
		}
	})
	if err != nil {
		return nil, err
	}
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
		return nil, validationError("Không thể lưu trữ mẫu biểu mẫu quy trình đang được bước quy trình sử dụng", Record{"step_usage_count": usageRows[0]["total"]})
	}
	_, err = r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		rows, queryErr := queryTx(ctx, tx, `
			UPDATE workflow_form_templates
			SET status='archived', updated_at=now()
			WHERE id=$1
			RETURNING name
		`, templateID)
		if queryErr != nil {
			return nil, queryErr
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		return rows[0], nil
	}, func(row Record) auditEvent {
		return auditEvent{
			ActorID: actorID, ActorName: actorName, EventType: "workflow_form_template_archived",
			TargetType: "workflow_form_template", TargetID: templateID,
			Description: fmt.Sprintf("Lưu trữ mẫu biểu mẫu quy trình %s", row["name"]), Metadata: Record{},
		}
	})
	if err != nil {
		return nil, err
	}
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
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "workflow_form_version_created", "workflow_form_version", versionID, fmt.Sprintf("Tạo nháp biểu mẫu quy trình %s v%d", templateName, nextVersionNo), Record{"template_id": templateID, "source_version_id": sourceVersionID}); err != nil {
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
		return nil, validationError("Chỉ phiên bản nháp biểu mẫu quy trình mới được chỉnh sửa", Record{"status": status})
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
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "workflow_form_version_updated", "workflow_form_version", versionID, fmt.Sprintf("Cập nhật nháp biểu mẫu quy trình %s v%d", templateName, versionNo), Record{"template_id": templateID}); err != nil {
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
		return nil, validationError("Chỉ phiên bản nháp biểu mẫu quy trình mới được phát hành", Record{"status": status})
	}
	var fieldCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_form_fields WHERE form_version_id=$1`, versionID).Scan(&fieldCount); err != nil {
		return nil, err
	}
	if fieldCount == 0 {
		return nil, validationError("Phiên bản biểu mẫu quy trình cần ít nhất một trường trước khi phát hành")
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
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "workflow_form_version_published", "workflow_form_version", versionID, fmt.Sprintf("Phát hành biểu mẫu quy trình %s v%d", templateName, versionNo), Record{"template_id": templateID, "field_count": fieldCount}); err != nil {
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
		return nil, validationError("Không thể lưu trữ phiên bản biểu mẫu quy trình đang được bước quy trình sử dụng", Record{"step_usage_count": stepUsage})
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workflow_form_versions SET status='archived', updated_at=now() WHERE id=$1`, versionID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "workflow_form_version_archived", "workflow_form_version", versionID, fmt.Sprintf("Lưu trữ biểu mẫu quy trình %s v%d", templateName, versionNo), Record{"template_id": templateID}); err != nil {
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
		return nil, validationError("Trạng thái biểu mẫu quy trình đã thay đổi, vui lòng tải lại", Record{"status": fromStatus})
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
				activityComment = "Đã gửi biểu mẫu quy trình"
			} else {
				activityComment = "Đã lưu nháp biểu mẫu quy trình"
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
				if strings.TrimSpace(fmt.Sprint(instance["form_code"])) == "builtin_construction_daily_report" {
					if err := syncConstructionDailyReportTx(ctx, tx, instance, answers, actorID); err != nil {
						return nil, err
					}
				}
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
						"label":    "Đã gửi biểu mẫu quy trình",
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
		auditDescription := "Lưu nháp biểu mẫu quy trình"
		if submit {
			auditDescription = "Gửi biểu mẫu quy trình"
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

func syncConstructionDailyReportTx(ctx context.Context, tx *sql.Tx, instance Record, answers Record, actorID string) error {
	completedItems := workflowAnswerRecordList(answers["completed_items"])
	crewItems := workflowAnswerRecordList(answers["crew_items"])
	materialItems := workflowAnswerRecordList(answers["material_usage_items"])
	blockers := workflowAnswerRecordList(answers["blockers"])
	workTitles := make([]string, 0, len(completedItems))
	for _, item := range completedItems {
		if title := strings.TrimSpace(fmt.Sprint(item["title"])); title != "" && title != "<nil>" {
			workTitles = append(workTitles, title)
		}
	}
	workSummary := strings.Join(workTitles, "; ")
	if workSummary == "" {
		workSummary = fmt.Sprintf("%d hạng mục thi công", len(completedItems))
	}
	manpower := 0.0
	for _, item := range crewItems {
		manpower += number(item["worker_count"])
	}
	materialNames := make([]string, 0, len(materialItems))
	for _, item := range materialItems {
		if name := strings.TrimSpace(fmt.Sprint(item["material_name"])); name != "" && name != "<nil>" {
			materialNames = append(materialNames, name)
		}
	}
	blockerTitles := make([]string, 0, len(blockers))
	for _, item := range blockers {
		title := strings.TrimSpace(fmt.Sprint(item["content"]))
		if title == "" || title == "<nil>" {
			title = strings.TrimSpace(fmt.Sprint(item["title"]))
		}
		if title != "" && title != "<nil>" {
			blockerTitles = append(blockerTitles, title)
		}
	}
	primaryEvidenceID := cleanID(answers["site_photo_evidence_id"])
	photos := []string{}
	if primaryEvidenceID != "" {
		photos = append(photos, primaryEvidenceID)
	}
	structuredRaw, _ := json.Marshal(answers)
	photosRaw, _ := json.Marshal(photos)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO operation_daily_reports (
			project_id,report_date,reported_by,weather,work_summary,manpower_summary,material_usage_summary,
			issues_summary,progress_percent,photos,workflow_form_instance_id,work_item_id,structured_json,primary_evidence_id
		) VALUES ($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7,$8,$9,$10::jsonb,$11,NULLIF($12,'')::uuid,$13::jsonb,NULLIF($14,'')::uuid)
		ON CONFLICT (workflow_form_instance_id) WHERE workflow_form_instance_id IS NOT NULL DO UPDATE SET
			report_date=EXCLUDED.report_date,reported_by=EXCLUDED.reported_by,weather=EXCLUDED.weather,
			work_summary=EXCLUDED.work_summary,manpower_summary=EXCLUDED.manpower_summary,
			material_usage_summary=EXCLUDED.material_usage_summary,issues_summary=EXCLUDED.issues_summary,
			progress_percent=EXCLUDED.progress_percent,photos=EXCLUDED.photos,structured_json=EXCLUDED.structured_json,
			primary_evidence_id=EXCLUDED.primary_evidence_id
	`, instance["project_id"], answers["report_date"], actorID, answers["weather"], workSummary,
		fmt.Sprintf("%.0f nhân sự / %d đội", manpower, len(crewItems)), strings.Join(materialNames, "; "),
		strings.Join(blockerTitles, "; "), int(number(answers["progress_percent"])), string(photosRaw), instance["id"], cleanID(instance["work_item_id"]), string(structuredRaw), primaryEvidenceID)
	return err
}

func workflowAnswerRecordList(value any) []Record {
	switch typed := value.(type) {
	case []Record:
		return typed
	case []any:
		out := make([]Record, 0, len(typed))
		for _, item := range typed {
			if record, ok := item.(map[string]any); ok {
				out = append(out, Record(record))
			}
		}
		return out
	default:
		return nil
	}
}

func (r *Repository) SignWorkflowForm(ctx context.Context, instanceID, purpose, comment string, visualFile StoredFileMetadata, strokes []WorkflowFormSignatureStroke, actorID, actorName string) (result Record, err error) {
	tracker := storedMutationTracker{}
	defer tracker.wrap(&err)
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
		return nil, validationError("Biểu mẫu quy trình phải được gửi trước khi ký nội bộ", Record{"status": instance["status"]})
	}
	projectID := cleanID(instance["project_id"])
	workItemID := cleanID(instance["work_item_id"])
	formVersionID := cleanID(instance["form_version_id"])
	revisionNo := int(number(instance["current_revision"]))
	if workItemID == "" || projectID == "" || formVersionID == "" {
		return nil, validationError("Biểu mẫu quy trình thiếu liên kết để ký", Record{"workflow_form_instance_id": instanceID})
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
		return nil, validationError("Bạn đã ký biểu mẫu quy trình này cho mục đích này", Record{"purpose": purpose})
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
		eventComment = "Đã ký biểu mẫu quy trình"
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_form_events (instance_id, project_id, work_item_id, actor_id, event_type, from_status, to_status, comment, metadata_json)
		VALUES ($1, $2, $3, NULLIF($4,'')::uuid, 'signed', 'submitted', 'submitted', $5, $6::jsonb)
	`, instanceID, projectID, workItemID, actorID, eventComment, string(rawMetadata)); err != nil {
		return nil, err
	}
	activityComment := fmt.Sprintf("Đã ký biểu mẫu quy trình: %s", purpose)
	if comment != "" {
		activityComment = comment
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'internal', $5::jsonb)
	`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "workflow_form_signed", "workflow_form_signature", signatureID, "Ký nội bộ biểu mẫu quy trình", Record{
		"work_item_id":              workItemID,
		"workflow_form_instance_id": instanceID,
		"purpose":                   purpose,
		"payload_hash":              payloadHash,
		"form_revision_no":          revisionNo,
	}); err != nil {
		return nil, err
	}
	return finalizeStoredMutation(&tracker, func() (Record, error) {
		return getWorkflowFormInstanceDetail(ctx, tx, instanceID)
	}, tx.Commit)
}

func (r *Repository) CreateWorkflowFormPDFExport(ctx context.Context, instanceID, fileName, title string, pdfFile StoredFileMetadata, payloadHash, actorID, actorName string) (result Record, err error) {
	tracker := storedMutationTracker{}
	defer tracker.wrap(&err)
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
		return nil, validationError("Biểu mẫu quy trình phải được gửi trước khi xuất PDF", Record{"status": instance["status"]})
	}
	projectID := cleanID(instance["project_id"])
	workItemID := cleanID(instance["work_item_id"])
	formVersionID := cleanID(instance["form_version_id"])
	revisionNo := int(number(instance["current_revision"]))
	if workItemID == "" || projectID == "" || formVersionID == "" {
		return nil, validationError("Biểu mẫu quy trình thiếu liên kết để xuất PDF", Record{"workflow_form_instance_id": instanceID})
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
	`, instanceID, projectID, workItemID, actorID, fmt.Sprintf("Xuất PDF biểu mẫu quy trình: %s", fileName), string(rawEventMetadata)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'internal', $5::jsonb)
	`, workItemID, projectID, actorID, fmt.Sprintf("Đã xuất PDF biểu mẫu quy trình: %s", fileName), string(rawEventMetadata)); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "workflow_form_pdf_exported", "workflow_form_pdf_export", exportID, "Xuất PDF biểu mẫu quy trình", eventMetadata); err != nil {
		return nil, err
	}
	return finalizeStoredMutation(&tracker, func() (Record, error) {
		return getWorkflowFormInstanceDetail(ctx, tx, instanceID)
	}, tx.Commit)
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
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'created', 'ready', 'Đầu việc được sinh từ biên bản họp', $4::jsonb)
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
		comment := fmt.Sprintf("Biên bản họp đã sinh %d đầu việc", len(createdIDs))
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
			descriptionParts = append(descriptionParts, "Phạm vi: "+scopeValue)
		}
		if reportedBy != "" && reportedBy != "<nil>" {
			descriptionParts = append(descriptionParts, "Người báo cáo: "+reportedBy)
		}
		if ownerName != "" && ownerName != "<nil>" {
			descriptionParts = append(descriptionParts, "Người phụ trách: "+ownerName)
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
		`, workItemID, projectID, actorID, "Đầu việc kế hoạch tiếp theo được sinh từ "+strings.ToLower(reportLabel), string(rawMetadata)); err != nil {
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
		comment := fmt.Sprintf("%s đã sinh %d đầu việc kế hoạch tiếp theo", reportLabel, len(createdIDs))
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
		return "construction_daily_report", "Nhật ký thi công", "construction_scope", strings.TrimSpace(fmt.Sprint(answers["construction_scope"]))
	}
	if rawScope, ok := answers["construction_scope"]; ok {
		return "construction_daily_report", "Nhật ký thi công", "construction_scope", strings.TrimSpace(fmt.Sprint(rawScope))
	}
	return "planning_daily_report", "Nhật ký kế hoạch", "planning_scope", strings.TrimSpace(fmt.Sprint(answers["planning_scope"]))
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
		comment = "Yêu cầu duyệt đầu việc"
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
		return nil, validationError("Trạng thái đầu việc đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
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
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "work_item_approval_requested", "work_item", workItemID, workItemAuditDescription("approval_requested", item, comment), json.RawMessage(rawMetadata)); err != nil {
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
		return nil, validationError("Trạng thái đầu việc đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
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
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "work_item_"+activityType, "work_item", workItemID, workItemAuditDescription(activityType, item, comment), json.RawMessage(rawMetadata)); err != nil {
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
		return nil, validationError("Trạng thái đầu việc đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
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
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "work_item_"+activityType, "work_item", workItemID, workItemAuditDescription(activityType, item, comment), json.RawMessage(rawMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetWorkItemDetail(ctx, workItemID)
}

func (r *Repository) AddWorkItemEvidence(ctx context.Context, item Record, req WorkItemEvidenceRequest, actorID, actorName string) (result Record, err error) {
	tracker := storedMutationTracker{}
	defer tracker.wrap(&err)
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
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "work_item_evidence_added", "work_item", workItemID, fmt.Sprintf("Thêm minh chứng %s cho đầu việc %s", req.Name, item["title"]), json.RawMessage(rawMetadata)); err != nil {
		return nil, err
	}
	return finalizeStoredMutation(&tracker, func() (Record, error) {
		return getEvidenceDetail(ctx, tx, fmt.Sprint(evidence["id"]))
	}, tx.Commit)
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
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, workItemActivityEvent(req.ActivityType), "work_item", workItemID, workItemActivityDescription(req.ActivityType, item, req.Comment), json.RawMessage(rawMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) AddProjectActivity(ctx context.Context, projectID string, req WorkItemActivityRequest, actorID, actorName string) (Record, error) {
	rawMetadata, _ := json.Marshal(req.Metadata)
	auditMetadata := Record{}
	for key, value := range req.Metadata {
		auditMetadata[key] = value
	}
	if req.ParentActivityID != "" {
		auditMetadata["parent_activity_id"] = req.ParentActivityID
	}
	rawAuditMetadata, _ := json.Marshal(auditMetadata)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json, parent_activity_id)
		VALUES (NULLIF($2,'')::uuid, $1, $3, $4, NULLIF($5,'')::uuid, $6, $7, $8, $9::jsonb, NULLIF($10,'')::uuid)
		RETURNING *
	`, projectID, req.WorkItemID, req.ScopeType, req.ScopeID, actorID, req.ActivityType, req.Comment, req.Visibility, string(rawMetadata), req.ParentActivityID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := r.insertActivityAttentionTx(ctx, tx, rows[0], req, actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "project_activity_"+req.ActivityType, req.ScopeType, req.ScopeID, projectActivityDescription(req.ActivityType, req.Comment), json.RawMessage(rawAuditMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) SetActivityReaction(ctx context.Context, activity Record, reaction string, active bool, userID string) (Record, error) {
	activityID := cleanID(activity["id"])
	projectID := cleanID(activity["project_id"])
	if err := r.Exec(ctx, `
		INSERT INTO activity_reactions (activity_id, project_id, user_id, reaction, active)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (activity_id, user_id, reaction)
		DO UPDATE SET active=EXCLUDED.active, updated_at=now()
	`, activityID, projectID, userID, reaction, active); err != nil {
		return nil, err
	}
	rows, err := r.Query(ctx, `
		SELECT reaction, COUNT(*) AS count,
			COALESCE(bool_or(user_id=$2), false) AS reacted_by_me
		FROM activity_reactions
		WHERE activity_id=$1 AND active
		GROUP BY reaction
		ORDER BY reaction
	`, activityID, userID)
	if err != nil {
		return nil, err
	}
	return Record{"activity_id": activityID, "project_id": projectID, "reactions": rows}, nil
}

func (r *Repository) UpdateActivityMessage(ctx context.Context, activity Record, comment, actorID, actorName string) (Record, error) {
	activityID := cleanID(activity["id"])
	projectID := cleanID(activity["project_id"])
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	currentRows, err := queryTx(ctx, tx, `
		SELECT *
		FROM work_item_activities
		WHERE id=$1 AND actor_id=$2 AND activity_type='comment'
		  AND withdrawn_at IS NULL
		  AND created_at >= now() - INTERVAL '15 minutes'
		FOR UPDATE
	`, activityID, actorID)
	if err != nil {
		return nil, err
	}
	if len(currentRows) == 0 {
		return nil, validationError("Tin nhắn không còn đủ điều kiện chỉnh sửa")
	}
	current := currentRows[0]
	revisionNo := int(number(current["message_revision_no"])) + 1
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO activity_message_revisions
			(activity_id, project_id, revision_no, action, previous_comment, new_comment, actor_id, actor_name)
		VALUES ($1, $2, $3, 'edited', $4, $5, $6, $7)
	`, activityID, projectID, revisionNo, current["comment"], comment, actorID, actorName); err != nil {
		return nil, err
	}
	updatedRows, err := queryTx(ctx, tx, `
		UPDATE work_item_activities
		SET comment=$2, edited_at=now(), edited_by=$3, message_revision_no=$4
		WHERE id=$1
		RETURNING *
	`, activityID, comment, actorID, revisionNo)
	if err != nil {
		return nil, err
	}
	if len(updatedRows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "project_activity_comment_edited", "activity", activityID, "Đã chỉnh sửa tin nhắn trao đổi", Record{"activity_id": activityID, "revision_no": revisionNo, "action": "edited"}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updatedRows[0], nil
}

func (r *Repository) WithdrawActivityMessage(ctx context.Context, activity Record, actorID, actorName string) (Record, error) {
	activityID := cleanID(activity["id"])
	projectID := cleanID(activity["project_id"])
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	currentRows, err := queryTx(ctx, tx, `
		SELECT *
		FROM work_item_activities
		WHERE id=$1 AND actor_id=$2 AND activity_type='comment'
		  AND withdrawn_at IS NULL
		  AND created_at >= now() - INTERVAL '15 minutes'
		FOR UPDATE
	`, activityID, actorID)
	if err != nil {
		return nil, err
	}
	if len(currentRows) == 0 {
		return nil, validationError("Tin nhắn không còn đủ điều kiện thu hồi")
	}
	current := currentRows[0]
	revisionNo := int(number(current["message_revision_no"])) + 1
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO activity_message_revisions
			(activity_id, project_id, revision_no, action, previous_comment, new_comment, actor_id, actor_name)
		VALUES ($1, $2, $3, 'withdrawn', $4, NULL, $5, $6)
	`, activityID, projectID, revisionNo, current["comment"], actorID, actorName); err != nil {
		return nil, err
	}
	updatedRows, err := queryTx(ctx, tx, `
		UPDATE work_item_activities
		SET comment=NULL, withdrawn_at=now(), withdrawn_by=$2, message_revision_no=$3
		WHERE id=$1
		RETURNING *
	`, activityID, actorID, revisionNo)
	if err != nil {
		return nil, err
	}
	if len(updatedRows) == 0 {
		return nil, sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `UPDATE activity_reactions SET active=false, updated_at=now() WHERE activity_id=$1 AND active`, activityID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE activity_read_states SET read_at=COALESCE(read_at, now()) WHERE activity_id=$1`, activityID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "project_activity_comment_withdrawn", "activity", activityID, "Đã thu hồi tin nhắn trao đổi", Record{"activity_id": activityID, "revision_no": revisionNo, "action": "withdrawn"}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updatedRows[0], nil
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
				SELECT $1::uuid, $2::uuid, 'user', $3::text, u.id, s.id
				FROM users u
				LEFT JOIN staff s ON s.user_id=u.id AND s.status='active'
				WHERE u.id=$3::uuid
				  AND u.status='active'
				  AND EXISTS (
					SELECT 1
					FROM project_members pm
					WHERE pm.project_id=$2::uuid
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
				SELECT $1::uuid, $2::uuid, 'staff', $3::text, s.user_id, s.id
				FROM staff s
				JOIN project_members pm ON pm.staff_id=s.id AND pm.project_id=$2::uuid AND pm.active
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
				SELECT DISTINCT $1::uuid, $2::uuid, 'role', $3::text, s.user_id, s.id
				FROM project_members pm
				JOIN staff s ON s.id=pm.staff_id
				WHERE pm.project_id=$2::uuid
				  AND pm.active
				  AND s.status='active'
				  AND s.user_id IS NOT NULL
				  AND (
					pm.role_ids ? $3::text
					OR (NULLIF($4::text, '') IS NOT NULL AND pm.role_ids ? $4::text)
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
		return fmt.Sprintf("Ghi chú đầu việc %s", title)
	}
	return fmt.Sprintf("Tạo hoạt động %s cho đầu việc %s: %s", activityType, title, comment)
}

func projectActivityDescription(activityType string, comment string) string {
	if activityType == "comment" {
		return "Ghi chú công trình"
	}
	return fmt.Sprintf("Tạo hoạt động %s cho công trình: %s", activityType, comment)
}

func evidenceReviewActionLabel(action string) string {
	switch action {
	case "accept":
		return "Chấp nhận"
	case "reject":
		return "Từ chối"
	default:
		return action
	}
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
		return nil, validationError("Minh chứng đã được rà soát hoặc không còn hợp lệ", Record{"status": evidence["status"]})
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, $4, $5, $6::jsonb)
	`, workItemID, projectID, actorID, activityType, comment, string(rawMetadata)); err != nil {
		return nil, err
	}
	actionLabel := evidenceReviewActionLabel(action)
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "work_item_"+activityType, "evidence_record", evidenceID, fmt.Sprintf("%s minh chứng %s", actionLabel, evidence["name"]), json.RawMessage(rawMetadata)); err != nil {
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
		return fmt.Sprintf("Bắt đầu đầu việc %s", title)
	case "completed":
		return fmt.Sprintf("Hoàn tất đầu việc %s", title)
	case "blocked":
		return fmt.Sprintf("Chặn đầu việc %s: %s", title, comment)
	case "blocker_resolved":
		return fmt.Sprintf("Gỡ điểm chặn đầu việc %s: %s", title, comment)
	case "review_requested":
		return fmt.Sprintf("Yêu cầu rà soát đầu việc %s", title)
	case "approval_requested":
		return fmt.Sprintf("Yêu cầu duyệt đầu việc %s", title)
	case "approved":
		return fmt.Sprintf("Duyệt đầu việc %s", title)
	case "rejected":
		return fmt.Sprintf("Từ chối đầu việc %s: %s", title, comment)
	case "changes_requested":
		return fmt.Sprintf("Yêu cầu chỉnh sửa đầu việc %s: %s", title, comment)
	default:
		return fmt.Sprintf("Cập nhật đầu việc %s", title)
	}
}

func cleanID(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	return text
}

func (r *Repository) ProjectOperations(ctx context.Context, projectID, userID string, filter ProjectOperationsFilter) (Record, error) {
	if filter.View == projectOperationsViewWorkflowSidebar {
		return r.ProjectOperationsWorkflowSidebar(ctx, projectID, userID)
	}
	project, err := r.GetProjectSummary(ctx, projectID)
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
	workItems, err := r.ListProjectOperationsWorkItems(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	summary, blockers, activeWork := summarizeWorkItems(workItems)

	latestActivity, err := r.ProjectOperationsActivity(ctx, projectID, profile)
	if err != nil {
		return nil, err
	}
	projectActivitySummary, err := r.ProjectActivitySummary(ctx, projectID, profile)
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
	evidenceSummary, err := r.ProjectEvidenceSummary(ctx, projectID, profile)
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
		"evidence_summary":    evidenceSummary,
		"activity_summary":    projectActivitySummary,
		"latest_activity":     latestActivity,
		"audit_summary":       auditSummary(auditRows),
		"department_surfaces": departmentSurfaces(workItems),
		"pipeline_run":        run,
		"summary":             summary,
		"work_items":          workItems,
	}, nil
}

func (r *Repository) ProjectOperationsWorkflowSidebar(ctx context.Context, projectID, userID string) (Record, error) {
	project, err := r.GetProjectSummary(ctx, projectID)
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
	workItems, err := r.ListProjectWorkflowSidebarWorkItems(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	summary, _, _ := summarizeWorkItems(workItems)
	run, err := r.GetPipelineRun(ctx, projectID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if run != nil && !profile.IsManager {
		run = scopePipelineRunToWorkItems(run, workItems)
	}
	visibility := projectOperationsVisibility(profile, permissions)
	healthCounts := projectOperationsHealthCounts(summary, Record{})
	projectSummary := projectOperationsProjectSummary(project)
	return Record{
		"project":           projectSummary,
		"project_summary":   projectSummary,
		"visibility":        visibility,
		"health_counts":     healthCounts,
		"workflow_timeline": workflowTimeline(run),
		"pipeline_run":      run,
		"summary":           summary,
		"work_items":        workItems,
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

func (r *Repository) ActivityRecord(ctx context.Context, activityID string) (Record, error) {
	return r.QueryOne(ctx, `SELECT * FROM work_item_activities WHERE id=$1`, activityID)
}

func (r *Repository) ActivityMentionedUser(ctx context.Context, activityID, userID string) (bool, error) {
	row, err := r.QueryOne(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM activity_mentions WHERE activity_id=$1 AND recipient_user_id=$2
		) AS allowed
	`, activityID, userID)
	if err != nil {
		return false, err
	}
	return boolValue(row["allowed"]), nil
}

func (r *Repository) ResolveActivity(ctx context.Context, activity Record, comment, actorID, actorName string) (Record, error) {
	activityID := cleanID(activity["id"])
	projectID := cleanID(activity["project_id"])
	activityType := strings.TrimSpace(fmt.Sprint(activity["activity_type"]))
	timestampKey := ""
	actorKey := ""
	actionLabel := ""
	switch activityType {
	case "question":
		timestampKey = "answered_at"
		actorKey = "answered_by"
		actionLabel = "Đã trả lời câu hỏi"
	case "blocker":
		timestampKey = "resolved_at"
		actorKey = "resolved_by"
		actionLabel = "Đã xử lý điểm chặn"
	case "reminder":
		timestampKey = "completed_at"
		actorKey = "completed_by"
		actionLabel = "Đã hoàn tất nhắc việc"
	default:
		return nil, validationError("Loại hoạt động này không có hành động hoàn tất", Record{"activity_type": activityType})
	}
	if comment == "" {
		comment = actionLabel
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		UPDATE work_item_activities
		SET metadata_json=COALESCE(metadata_json, '{}'::jsonb) || jsonb_build_object(
			$2::text, now(),
			$3::text, NULLIF($4,'')::uuid,
			'resolution_comment', $5::text
		)
		WHERE id=$1 AND COALESCE(metadata_json->>$2, metadata_json->>'closed_at', '')=''
		RETURNING *
	`, activityID, timestampKey, actorKey, actorID, comment)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, validationError("Hoạt động đã được xử lý trước đó", Record{"activity_id": activityID})
	}
	resolutionMetadata := Record{
		"source_activity_id":   activityID,
		"source_activity_type": activityType,
		"resolution_action":    timestampKey,
	}
	rawMetadata, _ := json.Marshal(resolutionMetadata)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities
			(work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES
			(NULLIF($1,'')::uuid, $2, $3, $4, NULLIF($5,'')::uuid, 'system_event', $6, $7, $8::jsonb)
	`, cleanID(activity["work_item_id"]), projectID, activity["scope_type"], activity["scope_id"], actorID, comment, activity["visibility"], string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "activity_resolved", "work_item_activity", activityID, comment, resolutionMetadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
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

func (r *Repository) MarkProjectActivityRead(ctx context.Context, projectID, userID string) (Record, error) {
	rows, err := r.Query(ctx, `
		UPDATE activity_read_states
		SET read_at=now()
		WHERE project_id=$1 AND user_id=$2 AND read_at IS NULL
		RETURNING activity_id
	`, projectID, userID)
	if err != nil {
		return nil, err
	}
	return Record{"project_id": projectID, "marked_count": len(rows)}, nil
}

func (r *Repository) ProjectActivitySummary(ctx context.Context, projectID string, profile workItemAccessProfile) (Record, error) {
	args := append([]any{projectID, profile.UserID}, workItemScopeArgs(profile, profile.IsManager, true)...)
	visibilitySQL := "TRUE"
	if profileIsClient(profile) && !profile.IsManager {
		visibilitySQL = "wia.visibility='client'"
	}
	unread := "(wia.withdrawn_at IS NULL AND ars.user_id IS NOT NULL AND ars.read_at IS NULL)"
	mentioned := "(wia.withdrawn_at IS NULL AND COALESCE(mention_state.mentioned_me, false))"
	openQuestion := "(wia.activity_type='question' AND COALESCE(wia.metadata_json->>'answered_at', wia.metadata_json->>'resolved_at', wia.metadata_json->>'closed_at', '')='')"
	openBlocker := "(wia.activity_type='blocker' AND COALESCE(wia.metadata_json->>'resolved_at', wia.metadata_json->>'closed_at', '')='')"
	dueReminder := "(wia.activity_type='reminder' AND COALESCE(wia.metadata_json->>'completed_at', wia.metadata_json->>'done_at', wia.metadata_json->>'closed_at', '')='' AND COALESCE(wia.metadata_json->>'due_at', '') ~ '^\\d{4}-\\d{2}-\\d{2}' AND left(wia.metadata_json->>'due_at', 10)::date <= current_date + INTERVAL '7 days')"
	return r.QueryOne(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE wia.activity_type IN ('comment','status','blocker','question','decision','reminder','file_note')) AS discussion,
			COUNT(*) FILTER (WHERE wia.activity_type NOT IN ('comment','status','blocker','question','decision','reminder','file_note')) AS system,
			COUNT(*) FILTER (WHERE wia.activity_type='decision') AS decisions,
			COUNT(*) FILTER (WHERE %s) AS unread,
			COUNT(*) FILTER (WHERE %s) AS mentioned,
			COUNT(*) FILTER (WHERE %s) AS questions,
			COUNT(*) FILTER (WHERE %s) AS blockers,
			COUNT(*) FILTER (WHERE %s) AS reminders,
			COUNT(*) FILTER (WHERE %s OR %s OR %s OR %s) AS attention
		FROM work_item_activities wia
		LEFT JOIN work_items wi ON wi.id=wia.work_item_id
		LEFT JOIN activity_read_states ars ON ars.activity_id=wia.id AND ars.user_id=$2
		LEFT JOIN LATERAL (
			SELECT bool_or(am.recipient_user_id=$2) AS mentioned_me
			FROM activity_mentions am
			WHERE am.activity_id=wia.id
		) mention_state ON true
		WHERE wia.project_id=$1
		  AND (wi.id IS NULL OR wi.status <> 'cancelled')
		  AND (%s)
		  AND (wia.scope_type <> 'work_item' OR (wi.id IS NOT NULL AND (%s)))
	`, unread, mentioned, openQuestion, openBlocker, dueReminder, unread, openQuestion, openBlocker, dueReminder, visibilitySQL, workItemScopeSQL("wi", 3)), args...)
}

func (r *Repository) ProjectActivityOptions(ctx context.Context, projectID string, profile workItemAccessProfile) (Record, error) {
	members, err := r.Query(ctx, `
		SELECT s.id, s.user_id, s.full_name AS label, s.department, s.position
		FROM project_members pm
		JOIN staff s ON s.id=pm.staff_id
		WHERE pm.project_id=$1 AND pm.active AND s.status='active' AND s.user_id IS NOT NULL
		ORDER BY lower(s.full_name)
	`, projectID)
	if err != nil {
		return nil, err
	}
	roles, err := r.Query(ctx, `
		SELECT role_name AS id, role_name AS label
		FROM (
			SELECT DISTINCT role_name
			FROM project_members pm
			CROSS JOIN LATERAL jsonb_array_elements_text(COALESCE(pm.role_ids, '[]'::jsonb)) AS roles(role_name)
			WHERE pm.project_id=$1 AND pm.active AND role_name <> ''
		) role_options
		ORDER BY lower(role_name), role_name
	`, projectID)
	if err != nil {
		return nil, err
	}
	workArgs := append([]any{projectID}, workItemScopeArgs(profile, profile.IsManager, true)...)
	workTargets, err := r.Query(ctx, fmt.Sprintf(`
		SELECT 'work_item' AS scope_type, wi.id AS scope_id,
			COALESCE(NULLIF(wi.title, ''), wi.id::text) AS label,
			wi.id AS work_item_id, wi.status
		FROM work_items wi
		WHERE wi.project_id=$1 AND wi.status <> 'cancelled' AND (%s)
		ORDER BY wi.created_at DESC
		LIMIT 300
	`, workItemScopeSQL("wi", 2)), workArgs...)
	if err != nil {
		return nil, err
	}
	domainTargets, err := r.Query(ctx, `
		SELECT * FROM (
			SELECT 'boq_item' AS scope_type, id AS scope_id, work_item_name AS label, NULL::uuid AS work_item_id, status FROM boq_items WHERE project_id=$1 AND status NOT IN ('cancelled','deleted')
			UNION ALL
			SELECT 'procurement_request', id, request_code || ' · ' || title, NULL::uuid, status FROM procurement_requests WHERE project_id=$1 AND status NOT IN ('cancelled','deleted')
			UNION ALL
			SELECT 'stock_movement', id, movement_code, NULL::uuid, NULL::text FROM stock_movements WHERE project_id=$1
			UNION ALL
			SELECT 'acceptance', id, title, NULL::uuid, status FROM acceptance_records WHERE project_id=$1 AND status NOT IN ('cancelled','deleted')
			UNION ALL
			SELECT 'rework', id, title, NULL::uuid, status FROM rework_requests WHERE project_id=$1 AND status NOT IN ('cancelled','deleted')
			UNION ALL
			SELECT 'handover', id, 'Bàn giao · ' || status, NULL::uuid, status FROM handover_records WHERE project_id=$1 AND status NOT IN ('cancelled','deleted')
			UNION ALL
			SELECT 'payable', id, payable_code || ' · ' || vendor_name, NULL::uuid, status FROM payables WHERE project_id=$1 AND status NOT IN ('cancelled','deleted')
			UNION ALL
			SELECT 'receivable', id, receivable_code || ' · ' || client_name, NULL::uuid, status FROM receivables WHERE project_id=$1 AND status NOT IN ('cancelled','deleted')
			UNION ALL
			SELECT 'payment', id, payment_code || ' · ' || payment_type, NULL::uuid, status FROM payments WHERE project_id=$1 AND status NOT IN ('cancelled','deleted')
		) options
		ORDER BY scope_type, lower(label)
	`, projectID)
	if err != nil {
		return nil, err
	}
	targets := append(workTargets, domainTargets...)
	evidenceVisibility := "er.visibility IN ('internal','project','client')"
	if profileIsClient(profile) && !profile.IsManager {
		evidenceVisibility = "er.visibility='client'"
	}
	attachmentArgs := append([]any{projectID}, workItemScopeArgs(profile, profile.IsManager, true)...)
	attachments, err := r.Query(ctx, fmt.Sprintf(`
		SELECT 'evidence' AS type, er.id, COALESCE(NULLIF(er.name,''), NULLIF(er.title,''), er.id::text) AS label,
			er.status, er.visibility, er.work_item_id
		FROM evidence_records er
		JOIN work_items wi ON wi.id=er.work_item_id
		WHERE er.project_id=$1 AND er.status <> 'rejected' AND wi.status <> 'cancelled'
		  AND %s AND (%s)
		ORDER BY er.created_at DESC
		LIMIT 200
	`, evidenceVisibility, workItemScopeSQL("wi", 2)), attachmentArgs...)
	if err != nil {
		return nil, err
	}
	if !profileIsClient(profile) || profile.IsManager {
		documents, err := r.Query(ctx, `
			SELECT 'document' AS type, d.id, d.name AS label, d.status, 'project' AS visibility, NULL::uuid AS work_item_id
			FROM documents d
			WHERE d.project_id=$1 AND d.status NOT IN ('cancelled','deleted')
			ORDER BY d.created_at DESC
			LIMIT 200
		`, projectID)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, documents...)
	}
	return Record{
		"members":     members,
		"roles":       roles,
		"targets":     targets,
		"attachments": attachments,
	}, nil
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
	if filter.ActivityType != "" {
		args = append(args, filter.ActivityType)
		conditions = append(conditions, fmt.Sprintf("wia.activity_type=$%d", len(args)))
	}
	if filter.ActorID != "" {
		args = append(args, filter.ActorID)
		conditions = append(conditions, fmt.Sprintf("wia.actor_id=$%d::uuid", len(args)))
	}
	if filter.Visibility != "" {
		args = append(args, filter.Visibility)
		conditions = append(conditions, fmt.Sprintf("wia.visibility=$%d", len(args)))
	}
	switch filter.Kind {
	case "discussion":
		conditions = append(conditions, "wia.activity_type IN ('comment','status','blocker','question','decision','reminder','file_note')")
	case "system":
		conditions = append(conditions, "wia.activity_type NOT IN ('comment','status','blocker','question','decision','reminder','file_note')")
	case "decision":
		conditions = append(conditions, "wia.activity_type='decision'")
	}
	unread := "(wia.withdrawn_at IS NULL AND ars.user_id IS NOT NULL AND ars.read_at IS NULL)"
	mentioned := "(wia.withdrawn_at IS NULL AND COALESCE(mention_state.mentioned_me, false))"
	openQuestion := "(wia.activity_type='question' AND COALESCE(wia.metadata_json->>'answered_at', wia.metadata_json->>'resolved_at', wia.metadata_json->>'closed_at', '')='')"
	openBlocker := "(wia.activity_type='blocker' AND COALESCE(wia.metadata_json->>'resolved_at', wia.metadata_json->>'closed_at', '')='')"
	dueReminder := "(wia.activity_type='reminder' AND COALESCE(wia.metadata_json->>'completed_at', wia.metadata_json->>'done_at', wia.metadata_json->>'closed_at', '')='' AND COALESCE(wia.metadata_json->>'due_at', '') ~ '^\\d{4}-\\d{2}-\\d{2}' AND left(wia.metadata_json->>'due_at', 10)::date <= current_date + INTERVAL '7 days')"
	switch filter.Attention {
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
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		placeholder := len(args)
		conditions = append(conditions, fmt.Sprintf(`(
			COALESCE(wia.comment, '') ILIKE $%d
			OR COALESCE(u.full_name, '') ILIKE $%d
			OR COALESCE(wi.title, '') ILIKE $%d
			OR COALESCE(p.code, '') ILIKE $%d
			OR COALESCE(p.name, '') ILIKE $%d
			OR COALESCE(bi.work_item_name, '') ILIKE $%d
			OR COALESCE(pr.request_code, '') ILIKE $%d
			OR COALESCE(pr.title, '') ILIKE $%d
			OR COALESCE(sm.movement_code, '') ILIKE $%d
			OR COALESCE(ar.title, '') ILIKE $%d
			OR COALESCE(rw.title, '') ILIKE $%d
			OR COALESCE(pay.payable_code, '') ILIKE $%d
			OR COALESCE(rec.receivable_code, '') ILIKE $%d
			OR COALESCE(pm.payment_code, '') ILIKE $%d
		)`, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder))
	}
	if filter.CursorCreatedAt != "" && filter.CursorID != "" {
		args = append(args, filter.CursorCreatedAt, filter.CursorID)
		conditions = append(conditions, fmt.Sprintf("(wia.created_at, wia.id) < ($%d::timestamptz, $%d::uuid)", len(args)-1, len(args)))
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 80
	}
	args = append(args, filter.Limit)
	limitPlaceholder := len(args)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT
			wia.*,
			to_char(wia.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"') AS cursor_created_at,
			wi.title AS work_item_title,
			u.full_name AS actor_name,
			COALESCE(mention_state.mentions, '[]'::jsonb) AS mentions,
				(wia.withdrawn_at IS NULL AND COALESCE(mention_state.mentioned_me, false)) AS mentioned_me,
			%s,
			%s,
			ars.read_at,
				(wia.withdrawn_at IS NULL AND ars.user_id IS NOT NULL AND ars.read_at IS NULL) AS is_unread,
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
		ORDER BY wia.created_at DESC, wia.id DESC
		LIMIT $%d
		`, activityAttachmentSelectSQL(), activityConversationSelectSQL("$2"), activityAttachmentJoinSQL(), activityConversationJoinSQL("$2"), strings.Join(conditions, " AND "), limitPlaceholder), args...)
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

func projectEvidenceWhere(projectID string, profile workItemAccessProfile, filter ProjectEvidenceFilter, includeCursor bool) ([]string, []any) {
	args := append([]any{projectID}, workItemScopeArgs(profile, profile.IsManager, true)...)
	conditions := []string{
		"er.project_id=$1",
		"wi.status <> 'cancelled'",
		fmt.Sprintf("(%s)", workItemScopeSQL("wi", 2)),
	}
	if profileIsClient(profile) && !profile.IsManager {
		conditions = append(conditions, "er.visibility='client'")
	} else {
		conditions = append(conditions, "er.visibility IN ('internal','project','client')")
	}
	if filter.EvidenceType != "" {
		args = append(args, filter.EvidenceType)
		conditions = append(conditions, fmt.Sprintf("er.evidence_type=$%d", len(args)))
	}
	switch filter.Status {
	case "waiting":
		conditions = append(conditions, "er.status IN ('draft','submitted')")
	case "accepted", "rejected":
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("er.status=$%d", len(args)))
	}
	if filter.WorkItemID != "" {
		args = append(args, filter.WorkItemID)
		conditions = append(conditions, fmt.Sprintf("er.work_item_id=$%d::uuid", len(args)))
	}
	if filter.SubmittedBy != "" {
		args = append(args, filter.SubmittedBy)
		conditions = append(conditions, fmt.Sprintf("er.submitted_by=$%d::uuid", len(args)))
	}
	if filter.Visibility != "" {
		args = append(args, filter.Visibility)
		conditions = append(conditions, fmt.Sprintf("er.visibility=$%d", len(args)))
	}
	if filter.DateFrom != "" {
		args = append(args, filter.DateFrom)
		conditions = append(conditions, fmt.Sprintf("er.created_at >= $%d::date", len(args)))
	}
	if filter.DateTo != "" {
		args = append(args, filter.DateTo)
		conditions = append(conditions, fmt.Sprintf("er.created_at < $%d::date + INTERVAL '1 day'", len(args)))
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		placeholder := len(args)
		conditions = append(conditions, fmt.Sprintf(`(
			er.name ILIKE $%d
			OR COALESCE(er.title, '') ILIKE $%d
			OR COALESCE(er.description, '') ILIKE $%d
			OR COALESCE(er.caption, '') ILIKE $%d
			OR COALESCE(er.file_url, '') ILIKE $%d
			OR COALESCE(er.checksum_sha256, '') ILIKE $%d
			OR COALESCE(er.payload_json->>'original_filename', '') ILIKE $%d
			OR wi.title ILIKE $%d
			OR COALESCE(d.name, '') ILIKE $%d
			OR COALESCE(u.full_name, '') ILIKE $%d
		)`, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder))
	}
	if includeCursor && filter.CursorCreatedAt != "" && filter.CursorID != "" {
		priorityExpression := "CASE WHEN er.status IN ('draft','submitted') THEN 0 ELSE 1 END"
		args = append(args, filter.CursorPriority, filter.CursorCreatedAt, filter.CursorID)
		priorityPlaceholder := len(args) - 2
		createdPlaceholder := len(args) - 1
		idPlaceholder := len(args)
		conditions = append(conditions, fmt.Sprintf(`(
			%s > $%d
			OR (%s=$%d AND (er.created_at, er.id) < ($%d::timestamptz, $%d::uuid))
		)`, priorityExpression, priorityPlaceholder, priorityExpression, priorityPlaceholder, createdPlaceholder, idPlaceholder))
	}
	return conditions, args
}

func (r *Repository) ListProjectEvidence(ctx context.Context, projectID string, profile workItemAccessProfile, filter ProjectEvidenceFilter) ([]Record, error) {
	conditions, args := projectEvidenceWhere(projectID, profile, filter, true)
	args = append(args, filter.Limit)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT er.*,
			wi.title AS work_item_title,
			wi.status AS work_item_status,
			wi.role_ids AS work_item_role_ids,
			wi.approval_request_id AS work_item_approval_request_id,
			rn.name AS run_node_name,
			u.full_name AS submitted_by_name,
			reviewer.full_name AS reviewed_by_name,
			d.name AS document_name,
			CASE WHEN er.status IN ('draft','submitted') THEN 0 ELSE 1 END AS review_priority
		FROM evidence_records er
		JOIN work_items wi ON wi.id=er.work_item_id AND wi.project_id=er.project_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN users u ON u.id=er.submitted_by
		LEFT JOIN users reviewer ON reviewer.id=er.reviewed_by
		LEFT JOIN documents d ON d.id=er.document_id
		WHERE %s
		ORDER BY review_priority, er.created_at DESC, er.id DESC
		LIMIT $%d
	`, strings.Join(conditions, " AND "), len(args)), args...)
}

func (r *Repository) ProjectEvidenceCount(ctx context.Context, projectID string, profile workItemAccessProfile, filter ProjectEvidenceFilter) (int, error) {
	conditions, args := projectEvidenceWhere(projectID, profile, filter, false)
	row, err := r.QueryOne(ctx, fmt.Sprintf(`
		SELECT COUNT(*) AS total
		FROM evidence_records er
		JOIN work_items wi ON wi.id=er.work_item_id AND wi.project_id=er.project_id
		LEFT JOIN users u ON u.id=er.submitted_by
		LEFT JOIN documents d ON d.id=er.document_id
		WHERE %s
	`, strings.Join(conditions, " AND ")), args...)
	if err != nil {
		return 0, err
	}
	return int(number(row["total"])), nil
}

func (r *Repository) ProjectEvidenceSummary(ctx context.Context, projectID string, profile workItemAccessProfile) (Record, error) {
	conditions, args := projectEvidenceWhere(projectID, profile, ProjectEvidenceFilter{}, false)
	return r.QueryOne(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE er.status IN ('draft','submitted')) AS waiting,
			COUNT(*) FILTER (WHERE er.status='accepted') AS accepted,
			COUNT(*) FILTER (WHERE er.status='rejected') AS rejected,
			COUNT(*) FILTER (WHERE er.evidence_type='photo' OR COALESCE(er.mime_type, '') LIKE 'image/%%') AS photos,
			COUNT(*) FILTER (WHERE er.evidence_type='document' OR COALESCE(er.mime_type, '') ILIKE '%%pdf%%') AS documents
		FROM evidence_records er
		JOIN work_items wi ON wi.id=er.work_item_id AND wi.project_id=er.project_id
		LEFT JOIN users u ON u.id=er.submitted_by
		LEFT JOIN documents d ON d.id=er.document_id
		WHERE %s
	`, strings.Join(conditions, " AND ")), args...)
}

func (r *Repository) ProjectEvidenceOptions(ctx context.Context, projectID string, profile workItemAccessProfile) (Record, error) {
	conditions, args := projectEvidenceWhere(projectID, profile, ProjectEvidenceFilter{}, false)
	where := strings.Join(conditions, " AND ")
	workItems, err := r.Query(ctx, fmt.Sprintf(`
		SELECT wi.id, wi.title AS label
		FROM evidence_records er
		JOIN work_items wi ON wi.id=er.work_item_id AND wi.project_id=er.project_id
		LEFT JOIN users u ON u.id=er.submitted_by
		LEFT JOIN documents d ON d.id=er.document_id
		WHERE %s
		GROUP BY wi.id, wi.title
		ORDER BY lower(wi.title), wi.id
	`, where), args...)
	if err != nil {
		return nil, err
	}
	submitters, err := r.Query(ctx, fmt.Sprintf(`
		SELECT u.id, u.full_name AS label
		FROM evidence_records er
		JOIN work_items wi ON wi.id=er.work_item_id AND wi.project_id=er.project_id
		JOIN users u ON u.id=er.submitted_by
		LEFT JOIN documents d ON d.id=er.document_id
		WHERE %s
		GROUP BY u.id, u.full_name
		ORDER BY lower(u.full_name), u.id
	`, where), args...)
	if err != nil {
		return nil, err
	}
	return Record{"work_items": workItems, "submitters": submitters}, nil
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
		hiddenSections = append(hiddenSections, "domain_counts", "project_audit", "unscoped_pipeline_steps")
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
		skippedSteps := 0
		for _, step := range steps {
			switch fmt.Sprint(step["status"]) {
			case "completed", "submitted":
				completedSteps++
			case "skipped", "cancelled":
				skippedSteps++
			}
		}
		progressSteps := completedSteps
		if fmt.Sprint(node["status"]) == "skipped" {
			progressSteps = 0
		}
		out = append(out, Record{
			"id":               node["id"],
			"name":             node["name"],
			"type":             node["type"],
			"order_index":      node["order_index"],
			"status":           node["status"],
			"step_count":       len(steps),
			"completed_steps":  completedSteps,
			"skipped_steps":    skippedSteps,
			"progress_percent": percent(progressSteps, len(steps)),
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

func (r *Repository) ProjectProgressBoard(ctx context.Context, userID string) ([]Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, err
	}
	return r.Query(ctx, fmt.Sprintf(`
		SELECT
			p.id::text AS id,
			p.code,
			p.name,
			p.client_name,
			p.status AS project_status,
			p.progress_percent,
			p.planned_start_date,
			p.planned_end_date,
			COALESCE(s.full_name, '') AS pm_name,
			COALESCE(pr.id::text, '') AS pipeline_run_id,
			COALESCE(pr.status, '') AS pipeline_run_status,
			COALESCE(rn.id::text, '') AS current_node_id,
			COALESCE(rn.name, '') AS current_node_name,
			COALESCE(rn.type, '') AS current_node_type,
			COALESCE(rn.status, '') AS current_node_status,
			COALESCE(rn.order_index, 0) AS current_node_order,
			COALESCE(rn.block_reason, '') AS current_node_block_reason,
			COALESCE((
				SELECT COUNT(*)
				FROM issues i
				WHERE i.project_id=p.id AND i.status IN ('open','in_progress')
			), 0) AS open_issues,
			COALESCE((
				SELECT COUNT(*)
				FROM approval_requests ar
				WHERE ar.project_id=p.id AND ar.status='pending'
			), 0) AS pending_approvals,
			COALESCE((
				SELECT COUNT(*)
				FROM work_items wi
				WHERE wi.project_id=p.id AND wi.status='blocked'
			), 0) AS blocked_items,
			COALESCE((
				SELECT COUNT(*)
				FROM work_items wi
				WHERE wi.project_id=p.id
				  AND wi.status NOT IN ('completed','cancelled')
				  AND wi.due_at IS NOT NULL
				  AND wi.due_at < now()
			), 0) AS overdue_items
		FROM projects p
		LEFT JOIN staff s ON s.id=p.pm_staff_id
		LEFT JOIN pipeline_runs pr ON pr.id=p.current_pipeline_run_id AND pr.project_id=p.id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=pr.current_node_id AND rn.project_id=p.id
		WHERE p.status <> 'cancelled' AND %s
		ORDER BY p.created_at DESC
	`, scopeSQL), scopeArgs...)
}

func (r *Repository) ProjectVisibleProjectIDs(ctx context.Context, userID string) (map[string]struct{}, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, err
	}
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT p.id::text AS id
		FROM projects p
		WHERE p.status <> 'cancelled' AND %s
	`, scopeSQL), scopeArgs...)
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

func (r *Repository) ListProjectMembers(ctx context.Context, projectID string, includeInactive bool) ([]Record, error) {
	activeFilter := "AND pm.active"
	if includeInactive {
		activeFilter = ""
	}
	return r.Query(ctx, fmt.Sprintf(`
		SELECT pm.id::text, pm.project_id::text, pm.staff_id::text,
			COALESCE(s.user_id::text, '') AS user_id,
			s.full_name, s.email, s.phone, s.department, s.position,
			pm.role_ids, pm.active, pm.notes, pm.created_at, pm.updated_at,
			(pm.staff_id=p.pm_staff_id) AS is_project_manager,
			COALESCE((
				SELECT jsonb_agg(jsonb_build_object(
					'id', r.id::text,
					'name', r.name,
					'display_name', r.display_name,
					'scope', 'global',
					'grants_project_view', EXISTS (
						SELECT 1 FROM role_permissions rp
						JOIN permissions p ON p.id=rp.permission_id
						WHERE rp.role_id=r.id AND p.key='project.view'
					)
				) ORDER BY r.display_name)
				FROM user_roles ur
				JOIN roles r ON r.id=ur.role_id
				WHERE ur.user_id=s.user_id AND ur.project_id IS NULL
			), '[]'::jsonb) AS global_roles,
			COALESCE((
				SELECT jsonb_agg(jsonb_build_object(
					'id', r.id::text,
					'name', r.name,
					'display_name', r.display_name,
					'scope', 'project',
					'grants_project_view', EXISTS (
						SELECT 1 FROM role_permissions rp
						JOIN permissions p ON p.id=rp.permission_id
						WHERE rp.role_id=r.id AND p.key='project.view'
					)
				) ORDER BY r.display_name)
				FROM user_roles ur
				JOIN roles r ON r.id=ur.role_id
				WHERE ur.user_id=s.user_id AND ur.project_id=pm.project_id
			), '[]'::jsonb) AS project_roles
		FROM project_members pm
		JOIN projects p ON p.id=pm.project_id AND p.status <> 'cancelled'
		JOIN staff s ON s.id=pm.staff_id AND s.status <> 'deleted'
		WHERE pm.project_id=$1 %s
		ORDER BY pm.active DESC, lower(s.full_name), pm.created_at DESC
	`, activeFilter), projectID)
}

func (r *Repository) ProjectMemberOptions(ctx context.Context, projectID string) (Record, error) {
	staff, err := r.Query(ctx, `
		SELECT s.id::text, s.user_id::text, s.full_name, s.email, s.department, s.position,
			latest_member.id AS member_id, COALESCE(latest_member.active, false) AS member_active,
			COALESCE((
				SELECT jsonb_agg(jsonb_build_object(
					'id', r.id::text,
					'name', r.name,
					'display_name', r.display_name,
					'scope', 'global',
					'grants_project_view', EXISTS (
						SELECT 1 FROM role_permissions rp
						JOIN permissions p ON p.id=rp.permission_id
						WHERE rp.role_id=r.id AND p.key='project.view'
					)
				) ORDER BY r.display_name)
				FROM user_roles ur
				JOIN roles r ON r.id=ur.role_id
				WHERE ur.user_id=s.user_id AND ur.project_id IS NULL
			), '[]'::jsonb) AS global_roles
		FROM staff s
		JOIN users u ON u.id=s.user_id AND u.status='active'
		LEFT JOIN LATERAL (
			SELECT pm.id::text AS id, pm.active
			FROM project_members pm
			WHERE pm.project_id=$1 AND pm.staff_id=s.id
			ORDER BY pm.active DESC, pm.created_at DESC, pm.id DESC
			LIMIT 1
		) latest_member ON true
		WHERE s.status='active'
		ORDER BY lower(s.full_name)
	`, projectID)
	if err != nil {
		return nil, err
	}
	roles, err := r.Query(ctx, `
		SELECT r.id::text, r.name, r.display_name, r.description,
			EXISTS (
				SELECT 1
				FROM role_permissions rp
				JOIN permissions p ON p.id=rp.permission_id
				WHERE rp.role_id=r.id AND p.key='project.view'
			) AS grants_project_view
		FROM roles r
		WHERE r.name NOT IN ('admin','director')
		ORDER BY lower(r.display_name)
	`)
	if err != nil {
		return nil, err
	}
	roleLabelRows, err := r.Query(ctx, `
		SELECT label
		FROM (
			SELECT DISTINCT label
			FROM (
				SELECT r.display_name AS label FROM roles r
				UNION ALL
				SELECT jsonb_array_elements_text(COALESCE(pm.role_ids, '[]'::jsonb)) AS label
				FROM project_members pm
				WHERE pm.project_id=$1
			) labels
			WHERE NULLIF(trim(label), '') IS NOT NULL
		) distinct_labels
		ORDER BY lower(label)
	`, projectID)
	if err != nil {
		return nil, err
	}
	roleLabels := make([]string, 0, len(roleLabelRows))
	for _, row := range roleLabelRows {
		roleLabels = append(roleLabels, strings.TrimSpace(fmt.Sprint(row["label"])))
	}
	return Record{"staff": staff, "access_roles": roles, "role_labels": roleLabels}, nil
}

func (r *Repository) SaveProjectMember(ctx context.Context, projectID, memberID string, req ProjectMemberMutationRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	projectRows, err := queryTx(ctx, tx, `
		SELECT id::text, COALESCE(pm_staff_id::text, '') AS pm_staff_id
		FROM projects
		WHERE id=$1 AND status <> 'cancelled'
		FOR UPDATE
	`, projectID)
	if err != nil {
		return nil, err
	}
	if len(projectRows) == 0 {
		return nil, validationError("Công trình không tồn tại hoặc đã huỷ", Record{"project_id": projectID})
	}
	staffRows, err := queryTx(ctx, tx, `
		SELECT s.id::text, s.full_name, COALESCE(s.user_id::text, '') AS user_id
		FROM staff s
		JOIN users u ON u.id=s.user_id AND u.status='active'
		WHERE s.id=$1 AND s.status='active'
		FOR UPDATE OF s
	`, req.StaffID)
	if err != nil {
		return nil, err
	}
	if len(staffRows) == 0 || cleanID(staffRows[0]["user_id"]) == "" {
		return nil, validationError("Nhân sự phải đang hoạt động và có tài khoản đăng nhập", Record{"staff_id": req.StaffID})
	}
	staffName := strings.TrimSpace(fmt.Sprint(staffRows[0]["full_name"]))
	userID := cleanID(staffRows[0]["user_id"])

	for _, roleID := range req.ProjectRoleIDs {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM roles WHERE id=$1 AND name NOT IN ('admin','director')`, roleID).Scan(&exists); err != nil {
			return nil, err
		}
		if exists == 0 {
			return nil, validationError("Role phân quyền không tồn tại", Record{"role_id": roleID})
		}
	}

	current := Record(nil)
	if memberID != "" {
		rows, err := queryTx(ctx, tx, `
			SELECT * FROM project_members
			WHERE id=$1 AND project_id=$2
			FOR UPDATE
		`, memberID, projectID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, validationError("Thành viên không thuộc công trình", Record{"member_id": memberID})
		}
		current = rows[0]
		if current["active"] != true {
			return nil, validationError("Thành viên đã ngừng tham gia; hãy dùng luồng thêm để kích hoạt lại", Record{"member_id": memberID})
		}
		if cleanID(current["staff_id"]) != req.StaffID {
			return nil, validationError("Không thể đổi nhân sự của một membership đã tồn tại", Record{"member_id": memberID})
		}
	} else {
		rows, err := queryTx(ctx, tx, `
			SELECT * FROM project_members
			WHERE project_id=$1 AND staff_id=$2
			ORDER BY active DESC, created_at DESC, id DESC
			FOR UPDATE
		`, projectID, req.StaffID)
		if err != nil {
			return nil, err
		}
		if len(rows) > 0 {
			current = rows[0]
			if current["active"] == true {
				return nil, validationError("Nhân sự đã là thành viên đang hoạt động của công trình", Record{"staff_id": req.StaffID})
			}
			memberID = cleanID(current["id"])
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1 AND project_id=$2`, userID, projectID); err != nil {
		return nil, err
	}
	for _, roleID := range req.ProjectRoleIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_roles (user_id, role_id, project_id)
			VALUES ($1,$2,$3)
			ON CONFLICT DO NOTHING
		`, userID, roleID, projectID); err != nil {
			return nil, err
		}
	}
	var canViewProject bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM user_roles ur
			JOIN role_permissions rp ON rp.role_id=ur.role_id
			JOIN permissions p ON p.id=rp.permission_id
			WHERE ur.user_id=$1
			  AND (ur.project_id IS NULL OR ur.project_id=$2)
			  AND p.key='project.view'
		)
	`, userID, projectID).Scan(&canViewProject); err != nil {
		return nil, err
	}
	if !canViewProject {
		return nil, validationError("Role hiện tại không cấp quyền xem công trình; hãy chọn role có project.view", Record{"field": "project_role_ids"})
	}

	rawRoles, _ := json.Marshal(req.RoleIDs)
	eventType := "project_member_updated"
	description := fmt.Sprintf("Cập nhật thành viên %s", staffName)
	if memberID == "" {
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO project_members (project_id, staff_id, role_ids, notes, active, updated_at)
			VALUES ($1,$2,$3::jsonb,NULLIF($4,''),true,now())
			RETURNING id::text
		`, projectID, req.StaffID, string(rawRoles), req.Notes).Scan(&memberID); err != nil {
			return nil, err
		}
		eventType = "project_member_added"
		description = fmt.Sprintf("Thêm thành viên %s", staffName)
	} else if _, err := tx.ExecContext(ctx, `
		UPDATE project_members
		SET role_ids=$3::jsonb, notes=NULLIF($4,''), active=true, updated_at=now()
		WHERE id=$1 AND project_id=$2
	`, memberID, projectID, string(rawRoles), req.Notes); err != nil {
		return nil, err
	} else if current != nil && current["active"] != true {
		eventType = "project_member_reactivated"
		description = fmt.Sprintf("Kích hoạt lại thành viên %s", staffName)
	}

	metadata := Record{
		"staff_id":         req.StaffID,
		"role_ids":         req.RoleIDs,
		"project_role_ids": req.ProjectRoleIDs,
		"notes":            req.Notes,
	}
	if current != nil {
		metadata["previous_role_ids"] = current["role_ids"]
		metadata["previous_active"] = current["active"]
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, eventType, "project_member", memberID, description, metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetProjectMember(ctx, projectID, memberID)
}

func (r *Repository) DeactivateProjectMember(ctx context.Context, projectID, memberID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT pm.*, s.full_name, COALESCE(s.user_id::text, '') AS user_id,
			(pm.staff_id=p.pm_staff_id) AS is_project_manager
		FROM project_members pm
		JOIN projects p ON p.id=pm.project_id AND p.status <> 'cancelled'
		JOIN staff s ON s.id=pm.staff_id
		WHERE pm.id=$1 AND pm.project_id=$2
		FOR UPDATE OF pm, p
	`, memberID, projectID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, validationError("Thành viên không thuộc công trình", Record{"member_id": memberID})
	}
	member := rows[0]
	if member["active"] != true {
		return nil, validationError("Thành viên đã ngừng tham gia", Record{"member_id": memberID})
	}
	if member["is_project_manager"] == true {
		return nil, validationError("Không thể ngừng tham gia PM hiện tại; hãy cập nhật PM thay thế trước", Record{"member_id": memberID})
	}
	if _, err := tx.ExecContext(ctx, `UPDATE project_members SET active=false, updated_at=now() WHERE id=$1`, memberID); err != nil {
		return nil, err
	}
	userID := cleanID(member["user_id"])
	if userID != "" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1 AND project_id=$2`, userID, projectID); err != nil {
			return nil, err
		}
	}
	metadata := Record{
		"staff_id":          cleanID(member["staff_id"]),
		"previous_role_ids": member["role_ids"],
		"active":            false,
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "project_member_deactivated", "project_member", memberID, fmt.Sprintf("Ngừng tham gia thành viên %s", member["full_name"]), metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetProjectMember(ctx, projectID, memberID)
}

func (r *Repository) GetProjectMember(ctx context.Context, projectID, memberID string) (Record, error) {
	rows, err := r.ListProjectMembers(ctx, projectID, true)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if cleanID(row["id"]) == memberID {
			return row, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *Repository) GetProjectDetail(ctx context.Context, id string) (Record, error) {
	project, err := r.GetProjectSummary(ctx, id)
	if err != nil {
		return nil, err
	}
	members, _ := r.Query(ctx, `
		SELECT pm.*, s.full_name, s.department, s.position
		FROM project_members pm
		JOIN staff s ON s.id=pm.staff_id
		WHERE pm.project_id=$1 AND pm.active
		ORDER BY s.full_name
	`, id)
	project["members"] = members
	return project, nil
}

func (r *Repository) GetProjectSummary(ctx context.Context, id string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT p.*, s.full_name AS pm_name, t.name AS pipeline_template_name,
			pt.name AS partner_name, pt.tax_code AS partner_tax_code
		FROM projects p
		LEFT JOIN staff s ON s.id=p.pm_staff_id
		LEFT JOIN pipeline_templates t ON t.id=p.pipeline_template_id
		LEFT JOIN partners pt ON pt.id=p.partner_id
		WHERE p.id=$1 AND p.status <> 'cancelled'
	`, id)
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
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	templateID := newID()
	body["id"] = templateID
	body["template_family_id"] = templateID
	body["revision_no"] = 1
	body["engine_version"] = 2
	body["status"] = "draft"
	if body["created_by"] == nil && userID != "" {
		body["created_by"] = userID
	}
	nodes, _ := body["nodes"].([]any)
	delete(body, "nodes")
	_, err = insertRecord(ctx, tx, "pipeline_templates", templateColumns(), body)
	if err != nil {
		return nil, err
	}
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
		node, err := insertRecord(ctx, tx, "pipeline_template_nodes", templateNodeColumns(), nodeBody)
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
			if _, provided := stepBody["surface_kind"]; !provided {
				stepBody["surface_kind"] = defaultStepSurfaceKind(fmt.Sprint(stepBody["step_type"]), formRequired)
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
			if _, err := insertRecord(ctx, tx, "pipeline_template_steps", templateStepColumns(), stepBody); err != nil {
				return nil, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetTemplateDetail(ctx, templateID)
}

func (r *Repository) CloneTemplate(ctx context.Context, id, userID string) (Record, error) {
	detail, err := r.GetTemplateDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	familyID := cleanID(detail["template_family_id"])
	if familyID == "" {
		familyID = id
	}
	if _, err := queryTx(ctx, tx, `SELECT id FROM pipeline_templates WHERE id=$1 FOR UPDATE`, familyID); err != nil {
		return nil, err
	}
	var revisionNo int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision_no),0)+1 FROM pipeline_templates WHERE template_family_id=$1 AND status <> 'deleted'`, familyID).Scan(&revisionNo); err != nil {
		return nil, err
	}
	cloneID := newID()
	body := Record{
		"id":                 cloneID,
		"name":               detail["name"],
		"description":        detail["description"],
		"project_type":       detail["project_type"],
		"version":            revisionNo,
		"status":             "draft",
		"created_by":         userID,
		"template_family_id": familyID,
		"revision_no":        revisionNo,
		"source_template_id": id,
		"engine_version":     2,
	}
	_, err = insertRecord(ctx, tx, "pipeline_templates", templateColumns(), body)
	if err != nil {
		return nil, err
	}
	nodeIDMap := map[string]string{}
	nodes, _ := detail["nodes"].([]Record)
	for _, node := range nodes {
		oldID := fmt.Sprint(node["id"])
		nodeBody := Record{
			"template_id":          cloneID,
			"business_code":        node["business_code"],
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
			"approval_policy_json": node["approval_policy_json"],
			"rework_enabled":       node["rework_enabled"],
			"replanning_enabled":   node["replanning_enabled"],
			"fail_path":            node["fail_path"],
			"status":               node["status"],
		}
		newNode, err := insertRecord(ctx, tx, "pipeline_template_nodes", templateNodeColumns(), nodeBody)
		if err != nil {
			return nil, err
		}
		newNodeID := fmt.Sprint(newNode["id"])
		nodeIDMap[oldID] = newNodeID
		steps, _ := node["steps"].([]Record)
		for _, step := range steps {
			stepBody := Record{
				"node_id":             newNodeID,
				"business_code":       step["business_code"],
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
				"surface_kind":        step["surface_kind"],
				"domain_action_key":   step["domain_action_key"],
				"order_index":         step["order_index"],
			}
			if _, err := insertRecord(ctx, tx, "pipeline_template_steps", templateStepColumns(), stepBody); err != nil {
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
		if _, err := insertRecord(ctx, tx, "pipeline_template_edges", edgeColumns(), Record{
			"template_id":      cloneID,
			"from_node_id":     from,
			"to_node_id":       to,
			"condition_type":   edge["condition_type"],
			"condition_config": edge["condition_config"],
			"label":            edge["label"],
			"priority":         edgePriority(edge),
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetTemplateDetail(ctx, cloneID)
}

func (r *Repository) StartPipeline(ctx context.Context, projectID, templateID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	projectRows, err := queryTx(ctx, tx, `SELECT id,workflow_entry_mode FROM projects WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, projectID)
	if err != nil {
		return nil, err
	}
	if len(projectRows) == 0 {
		return nil, sql.ErrNoRows
	}
	if fmt.Sprint(projectRows[0]["workflow_entry_mode"]) == "client_boq" {
		var hasClientBOQ bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM documents
				WHERE project_id=$1 AND target_type='project_intake' AND document_type='client_boq' AND status='active'
			)
		`, projectID).Scan(&hasClientBOQ); err != nil {
			return nil, err
		}
		if !hasClientBOQ {
			return nil, validationError("Công trình chọn nhánh BOQ có sẵn nhưng chưa có hồ sơ BOQ đầu vào")
		}
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
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(pipeline_template_id, (SELECT id FROM pipeline_templates WHERE status='published' ORDER BY revision_no DESC, published_at DESC NULLS LAST, created_at DESC LIMIT 1)) FROM projects WHERE id=$1`, projectID).Scan(&templateID); err != nil {
			return nil, err
		}
	}
	if templateID == "" {
		return nil, errors.New("Công trình chưa có mẫu quy trình")
	}
	templateRows, err := queryTx(ctx, tx, `
		SELECT id, status, engine_version, definition_hash
		FROM pipeline_templates
		WHERE id=$1
		FOR SHARE
	`, templateID)
	if err != nil {
		return nil, err
	}
	if len(templateRows) == 0 {
		return nil, sql.ErrNoRows
	}
	if strings.TrimSpace(fmt.Sprint(templateRows[0]["status"])) != "published" {
		return nil, validationError("Chỉ phiên bản quy trình đã phát hành mới được áp dụng cho công trình", Record{
			"template_id": templateID,
			"status":      templateRows[0]["status"],
		})
	}

	formIssues, err := queryTx(ctx, tx, templatePublishFormIssuesSQL(), templateID)
	if err != nil {
		return nil, err
	}
	if len(formIssues) > 0 {
		return nil, validationError("Mẫu quy trình có bước bắt buộc biểu mẫu quy trình nhưng chưa gắn phiên bản biểu mẫu đã phát hành", Record{"issues": formIssues})
	}

	runID := newID()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pipeline_runs (id, project_id, template_id, engine_version, template_definition_hash, status, started_at)
		VALUES ($1,$2,$3,$4,$5,'in_progress',now())
	`, runID, projectID, templateID, templateRows[0]["engine_version"], templateRows[0]["definition_hash"]); err != nil {
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
		return nil, errors.New("Mẫu quy trình chưa có node")
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
		rawApprovalPolicy, _ := json.Marshal(node["approval_policy_json"])
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
				INSERT INTO pipeline_run_nodes
					(id, run_id, project_id, template_node_id, business_code, name, description, type, order_index, responsible_role_ids, required_permission, permissions, exit_conditions, approval_rule, approval_policy_json, fail_path, status, started_at)
				VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,$10::jsonb,$11,$12::jsonb,$13::jsonb,$14,$15::jsonb,$16,$17,%s)
			`, startedSQL), runNodeID, runID, projectID, node["id"], cleanOptionalText(node["business_code"]), node["name"], node["description"], node["type"], node["order_index"], string(rawRoles), node["required_permission"], string(rawPermissions), string(rawExit), node["approval_rule"], string(rawApprovalPolicy), node["fail_path"], status); err != nil {
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
					(id, run_node_id, project_id, template_step_id, business_code, name, description, step_type, required_role_ids, required_permission, checklist_required, document_required, photo_required, form_required, form_version_id, surface_kind, domain_action_key, order_index, status)
				VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9::jsonb,$10,$11,$12,$13,$14,NULLIF($15,'')::uuid,NULLIF($16,''),NULLIF($17,''),$18,'pending')
			`, runStepID, runNodeID, projectID, step["id"], cleanOptionalText(step["business_code"]), step["name"], step["description"], step["step_type"], string(rawStepRoles), step["required_permission"], step["checklist_required"], step["document_required"], step["photo_required"], step["form_required"], cleanID(step["form_version_id"]), cleanOptionalText(step["surface_kind"]), cleanOptionalText(step["domain_action_key"]), step["order_index"]); err != nil {
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
				"form_required":       step["form_required"],
				"form_version_id":     cleanID(step["form_version_id"]),
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
				VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'created', $4, 'Đầu việc được tạo từ bước chạy quy trình', $5::jsonb)
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
						VALUES ($1, $2, $3, NULLIF($4,'')::uuid, 'created', 'draft', 'Biểu mẫu quy trình được tạo khi khởi động quy trình', $5::jsonb)
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
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "node_started", "pipeline_run", runID, "Tạo Pipeline Run và mở node đầu tiên", Record{}); err != nil {
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
	stepRows, err := r.Query(ctx, `
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
			COALESCE(wfi.form_version_id, rs.form_version_id) AS workflow_form_version_id,
			wfv.version_no AS workflow_form_version_no,
			wfv.status AS workflow_form_version_status,
			wft.code AS workflow_form_template_code,
			wft.name AS workflow_form_template_name,
			COALESCE(rs.form_required, false) AS workflow_form_required,
			rs.form_version_id AS required_workflow_form_version_id,
			rs.surface_kind,
			rs.domain_action_key
		FROM pipeline_run_steps rs
		JOIN pipeline_run_nodes rn ON rn.id=rs.run_node_id
		LEFT JOIN work_items wi ON wi.run_step_id=rs.id AND wi.status <> 'cancelled'
		LEFT JOIN workflow_form_instances wfi ON wfi.run_step_id=rs.id AND wfi.status <> 'cancelled'
		LEFT JOIN workflow_form_versions wfv ON wfv.id=COALESCE(wfi.form_version_id, rs.form_version_id)
		LEFT JOIN workflow_form_templates wft ON wft.id=COALESCE(wfi.form_template_id, wfv.template_id)
		WHERE rn.run_id=$1
		ORDER BY rn.order_index, rs.order_index, rs.id
	`, run["id"])
	if err != nil {
		return nil, err
	}
	stepsByNodeID := map[string][]Record{}
	for _, step := range stepRows {
		nodeID := cleanID(step["run_node_id"])
		stepsByNodeID[nodeID] = append(stepsByNodeID[nodeID], step)
	}
	for _, node := range nodes {
		nodeID := cleanID(node["id"])
		node["steps"] = stepsByNodeID[nodeID]
		if nodeID == cleanID(run["current_node_id"]) {
			issues, issueErr := r.NodeCompletionIssues(ctx, node)
			if issueErr != nil {
				return nil, issueErr
			}
			node["completion_issues"] = issues
			run["completion_issues"] = issues
		}
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
		comment = "Hoàn tất qua thao tác gửi bước quy trình cũ"
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
	if err := addAuditTx(ctx, tx, cleanID(step["project_id"]), actorID, actorName, "step_completed", "pipeline_run_step", stepID, fmt.Sprintf("Hoàn tất step %s", step["name"]), Record{"step": step["name"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return step, nil
}

func (r *Repository) RunClientWorkflowAutomationForWorkItem(ctx context.Context, workItemID, actorID, actorName string) error {
	workItemID = cleanID(workItemID)
	if workItemID == "" {
		return nil
	}
	contextRow, err := r.clientWorkflowAutomationContext(ctx, workItemID)
	if err != nil {
		return err
	}
	if cleanID(contextRow["template_id"]) != clientOfficialWorkflowTemplateID {
		return nil
	}
	if int(number(contextRow["engine_version"])) >= 2 {
		return nil
	}
	if fmt.Sprint(contextRow["status"]) != "completed" {
		return nil
	}
	actionKey := clientWorkflowAutomationAction(contextRow)
	if actionKey == "" {
		return nil
	}
	outcome, err := r.executeClientWorkflowAutomation(ctx, contextRow, actionKey, actorID, actorName)
	if err != nil {
		outcome = workflowAutomationOutcome{
			ActionKey:    actionKey,
			Status:       "failed",
			Message:      "Tự động hóa quy trình lỗi",
			ErrorMessage: err.Error(),
			Metadata:     Record{"error": err.Error()},
		}
	}
	return r.recordClientWorkflowAutomationOutcome(ctx, contextRow, outcome, actorID, actorName)
}

func (r *Repository) RunRequiredWorkflowAutomationForWorkItem(ctx context.Context, workItemID, actorID, actorName string) error {
	workItemID = cleanID(workItemID)
	if workItemID == "" {
		return nil
	}
	contextRow, err := r.clientWorkflowAutomationContext(ctx, workItemID)
	if err != nil {
		return err
	}
	if int(number(contextRow["engine_version"])) < 2 || cleanID(contextRow["template_id"]) != clientOfficialWorkflowTemplateID {
		return nil
	}
	actionKey := clientWorkflowAutomationAction(contextRow)
	if actionKey == "" {
		return nil
	}
	outcome, executeErr := r.executeClientWorkflowAutomation(ctx, contextRow, actionKey, actorID, actorName)
	if executeErr != nil {
		outcome = workflowAutomationOutcome{
			ActionKey:    actionKey,
			Status:       "failed",
			Message:      "Tự động hóa bắt buộc bị lỗi",
			ErrorMessage: executeErr.Error(),
			Metadata:     Record{"error": executeErr.Error(), "hard_gate": true},
		}
	}
	if outcome.ActionKey == "" {
		outcome.ActionKey = actionKey
	}
	if outcome.Metadata == nil {
		outcome.Metadata = Record{}
	}
	outcome.Metadata["hard_gate"] = true
	if err := r.recordClientWorkflowAutomationOutcome(ctx, contextRow, outcome, actorID, actorName); err != nil {
		return err
	}
	if outcome.Status != "succeeded" && outcome.Status != "skipped" {
		message := outcome.BlockerReason
		if message == "" {
			message = outcome.ErrorMessage
		}
		if message == "" {
			message = outcome.Message
		}
		return validationError("Tự động hóa bắt buộc chưa hoàn tất", Record{
			"action_key":  actionKey,
			"status":      outcome.Status,
			"reason":      message,
			"target_type": outcome.TargetType,
			"target_id":   outcome.TargetID,
		})
	}
	return nil
}

func (r *Repository) clientWorkflowAutomationContext(ctx context.Context, workItemID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			wi.*,
			pr.id AS pipeline_run_id,
			pr.template_id,
			COALESCE(pr.engine_version, 1) AS engine_version,
			pt.name AS template_name,
			rn.id AS context_run_node_id,
			rn.name AS run_node_name,
			rn.type AS run_node_type,
			rn.order_index AS run_node_order_index,
			rs.id AS context_run_step_id,
			rs.name AS run_step_name,
			rs.step_type AS run_step_type,
			rs.domain_action_key
		FROM work_items wi
		LEFT JOIN pipeline_runs pr ON pr.id=wi.pipeline_run_id
		LEFT JOIN pipeline_templates pt ON pt.id=pr.template_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		WHERE wi.id=$1
	`, workItemID)
}

func clientWorkflowAutomationAction(contextRow Record) string {
	domainActionKey := strings.TrimSpace(fmt.Sprint(contextRow["domain_action_key"]))
	switch domainActionKey {
	case "procurement.generate_from_boq", "procurement.require_approved", "contract.require_execution_ready", "finance.scaffold_incoming_acceptance", "finance.scaffold_outgoing_acceptance", "handover.evaluate_readiness":
		return domainActionKey
	}
	if int(number(contextRow["engine_version"])) >= 2 {
		return ""
	}
	nodeName := strings.TrimSpace(fmt.Sprint(contextRow["run_node_name"]))
	switch {
	case strings.HasPrefix(nodeName, "BOQ ("):
		return "generate_procurement_from_boq"
	case strings.HasPrefix(nodeName, "Đầu Vào:"):
		return "input_procurement_gate"
	case nodeName == "Nghiệm thu Đầu Vào":
		return "incoming_acceptance_finance_scaffold"
	case nodeName == "Nghiệm thu Đầu Ra":
		return "outgoing_acceptance_finance_scaffold"
	case strings.HasPrefix(nodeName, "Kết thúc"):
		return "handover_financial_closure_check"
	default:
		return ""
	}
}

func (r *Repository) executeClientWorkflowAutomation(ctx context.Context, contextRow Record, actionKey, actorID, actorName string) (workflowAutomationOutcome, error) {
	var outcome workflowAutomationOutcome
	var err error
	switch actionKey {
	case "generate_procurement_from_boq", "procurement.generate_from_boq":
		outcome, err = r.automateClientBOQProcurement(ctx, contextRow, actorID, actorName)
	case "input_procurement_gate", "procurement.require_approved":
		outcome, err = r.automateClientInputProcurementGate(ctx, contextRow)
	case "contract.require_execution_ready":
		outcome, err = r.automateClientContractExecutionGate(ctx, contextRow)
	case "incoming_acceptance_finance_scaffold", "finance.scaffold_incoming_acceptance":
		outcome, err = r.automateClientIncomingAcceptanceFinance(ctx, contextRow, actorID, actorName)
	case "outgoing_acceptance_finance_scaffold", "finance.scaffold_outgoing_acceptance":
		outcome, err = r.automateClientOutgoingAcceptanceFinance(ctx, contextRow, actorID, actorName)
	case "handover_financial_closure_check", "handover.evaluate_readiness":
		outcome, err = r.automateClientHandoverClosure(ctx, contextRow, actorID, actorName)
	default:
		return workflowAutomationOutcome{}, nil
	}
	if outcome.ActionKey != "" {
		outcome.ActionKey = actionKey
	}
	return outcome, err
}

func (r *Repository) automateClientContractExecutionGate(ctx context.Context, contextRow Record) (workflowAutomationOutcome, error) {
	projectID := cleanID(contextRow["project_id"])
	rows, err := r.Query(ctx, `
		SELECT c.id AS contract_id,c.status AS contract_status,cv.id AS contract_version_id,cv.status AS version_status,
			EXISTS (
				SELECT 1 FROM contract_version_signature_links link
				WHERE link.contract_version_id=cv.id
			) AS app_signed,
			EXISTS (
				SELECT 1 FROM contract_dispatches dispatch
				WHERE dispatch.contract_id=c.id
				  AND dispatch.contract_version_id=cv.id
				  AND dispatch.status='received'
				  AND dispatch.returned_scan_document_id IS NOT NULL
			) AS returned_scan_received
		FROM contracts c
		LEFT JOIN contract_versions cv ON cv.id=c.current_version_id
		WHERE c.project_id=$1
		  AND c.contract_type='main_client'
		  AND c.direction='receivable'
		  AND c.status <> 'terminated'
		ORDER BY (c.status='active') DESC,c.updated_at DESC,c.id
		LIMIT 1
	`, projectID)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	metadata := Record{"project_id": projectID}
	if len(rows) == 0 {
		return automationBlocked("contract.require_execution_ready", "Chưa có hợp đồng đầu ra với chủ đầu tư", metadata), nil
	}
	contract := rows[0]
	metadata["contract_id"] = contract["contract_id"]
	metadata["contract_status"] = contract["contract_status"]
	metadata["contract_version_id"] = contract["contract_version_id"]
	metadata["version_status"] = contract["version_status"]
	metadata["app_signed"] = contract["app_signed"]
	metadata["returned_scan_received"] = contract["returned_scan_received"]
	if fmt.Sprint(contract["version_status"]) != "signed" || !boolValue(contract["app_signed"]) {
		return automationBlockedWithTarget("contract.require_execution_ready", "Version hợp đồng chưa được ký trên App bằng chữ ký workflow", "contract", cleanID(contract["contract_id"]), metadata), nil
	}
	if !boolValue(contract["returned_scan_received"]) {
		return automationBlockedWithTarget("contract.require_execution_ready", "Chưa nhận lại và liên kết bản scan hợp đồng giấy đã ký", "contract", cleanID(contract["contract_id"]), metadata), nil
	}
	if fmt.Sprint(contract["contract_status"]) != "active" {
		return automationBlockedWithTarget("contract.require_execution_ready", "Hợp đồng chưa được kích hoạt sau khi hoàn tất ký và hồ sơ giấy", "contract", cleanID(contract["contract_id"]), metadata), nil
	}
	return automationSucceeded("contract.require_execution_ready", "Hợp đồng đầu ra đã đủ điều kiện mở triển khai", "contract", cleanID(contract["contract_id"]), metadata), nil
}

func (r *Repository) automateClientBOQProcurement(ctx context.Context, contextRow Record, actorID, actorName string) (workflowAutomationOutcome, error) {
	projectID := cleanID(contextRow["project_id"])
	summary, err := r.QueryOne(ctx, `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status <> 'approved' OR NOT final_price_locked) AS not_ready,
			COUNT(*) FILTER (
				WHERE status='approved'
				  AND final_price_locked
				  AND NOT EXISTS (
					SELECT 1
					FROM procurement_request_items pri
					JOIN procurement_requests pr ON pr.id=pri.request_id
					WHERE pri.boq_item_id=boq_items.id
					  AND pr.status <> 'cancelled'
				  )
			) AS ready_without_procurement
		FROM boq_items
		WHERE project_id=$1 AND status <> 'deleted'
	`, projectID)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	metadata := Record{
		"node":                      contextRow["run_node_name"],
		"boq_total":                 number(summary["total"]),
		"boq_not_ready":             number(summary["not_ready"]),
		"ready_without_procurement": number(summary["ready_without_procurement"]),
	}
	if number(summary["total"]) == 0 {
		return automationBlocked("generate_procurement_from_boq", "Chưa có dòng BOQ để sinh yêu cầu mua", metadata), nil
	}
	if number(summary["not_ready"]) > 0 {
		return automationBlocked("generate_procurement_from_boq", "BOQ còn dòng chưa duyệt hoặc chưa khóa đơn giá cuối", metadata), nil
	}
	if number(summary["ready_without_procurement"]) > 0 {
		procurement, err := r.GenerateProcurementFromBOQ(ctx, projectID, actorID, actorName)
		if err != nil {
			var statusErr StatusError
			if errors.As(err, &statusErr) {
				metadata["validation"] = statusErr.Message
				if statusErr.Details != nil {
					metadata["details"] = statusErr.Details
				}
				return automationBlocked("generate_procurement_from_boq", statusErr.Message, metadata), nil
			}
			return workflowAutomationOutcome{}, err
		}
		return automationSucceeded("generate_procurement_from_boq", "Đã sinh yêu cầu mua từ BOQ", "procurement_request", cleanID(procurement["id"]), metadata), nil
	}
	procurement, err := r.firstProjectProcurement(ctx, projectID)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	if procurement == nil {
		return automationBlocked("generate_procurement_from_boq", "Không còn dòng BOQ nào cần sinh mua sắm nhưng cũng chưa có yêu cầu mua", metadata), nil
	}
	return automationSucceeded("generate_procurement_from_boq", "Yêu cầu mua từ BOQ đã tồn tại", "procurement_request", cleanID(procurement["id"]), metadata), nil
}

func (r *Repository) automateClientInputProcurementGate(ctx context.Context, contextRow Record) (workflowAutomationOutcome, error) {
	projectID := cleanID(contextRow["project_id"])
	summary, err := r.QueryOne(ctx, `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status NOT IN ('approved','ordered','partially_received','received')) AS not_approved
		FROM procurement_requests
		WHERE project_id=$1 AND status <> 'cancelled'
	`, projectID)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	metadata := Record{
		"node":                     contextRow["run_node_name"],
		"procurement_total":        number(summary["total"]),
		"procurement_not_approved": number(summary["not_approved"]),
	}
	if number(summary["total"]) == 0 {
		return automationBlocked("input_procurement_gate", "Chưa có yêu cầu mua/đầu vào bắt buộc", metadata), nil
	}
	if number(summary["not_approved"]) > 0 {
		return automationBlocked("input_procurement_gate", "Còn yêu cầu mua/đầu vào chưa được duyệt", metadata), nil
	}
	procurement, err := r.firstProjectProcurement(ctx, projectID)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	targetID := ""
	if procurement != nil {
		targetID = cleanID(procurement["id"])
	}
	return automationSucceeded("input_procurement_gate", "Đầu vào bắt buộc đã có procurement được duyệt", "procurement_request", targetID, metadata), nil
}

func (r *Repository) automateClientIncomingAcceptanceFinance(ctx context.Context, contextRow Record, actorID, actorName string) (workflowAutomationOutcome, error) {
	projectID := cleanID(contextRow["project_id"])
	procurement, err := r.firstApprovedProjectProcurement(ctx, projectID)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	if procurement == nil {
		return automationBlocked("incoming_acceptance_finance_scaffold", "Chưa có yêu cầu mua/đầu vào đã duyệt để mở nghiệm thu đầu vào", Record{"node": contextRow["run_node_name"]}), nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	defer tx.Rollback()
	acceptanceID, payableID, metadata, err := r.ensureIncomingAcceptanceFinanceScaffoldTx(ctx, tx, contextRow, procurement, actorID, actorName)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	if err := tx.Commit(); err != nil {
		return workflowAutomationOutcome{}, err
	}
	if payableID != "" {
		metadata["payable_id"] = payableID
		ready, err := r.clientInputInvoiceVerified(ctx, payableID)
		if err != nil {
			return workflowAutomationOutcome{}, err
		}
		if !ready {
			metadata["input_invoice_required"] = "Cần tải lên và xác minh hóa đơn đầu vào thật"
			return automationBlockedWithTarget("incoming_acceptance_finance_scaffold", "Cần tải lên và xác minh hóa đơn đầu vào thật", "payable", payableID, metadata), nil
		}
	} else {
		return automationBlockedWithTarget("incoming_acceptance_finance_scaffold", "Cần mở công nợ phải trả trước khi lập lịch thanh toán và hóa đơn thật", "incoming_acceptance", acceptanceID, metadata), nil
	}
	return automationSucceeded("incoming_acceptance_finance_scaffold", "Đã mở hồ sơ nghiệm thu đầu vào, công nợ và lịch thanh toán dạng nháp", "incoming_acceptance", acceptanceID, metadata), nil
}

func (r *Repository) ensureIncomingAcceptanceFinanceScaffoldTx(ctx context.Context, tx *sql.Tx, contextRow, procurement Record, actorID, actorName string) (string, string, Record, error) {
	projectID := cleanID(contextRow["project_id"])
	workItemID := cleanID(contextRow["id"])
	procurementID := cleanID(procurement["id"])
	supplierName := cleanOptionalText(procurement["supplier_name"])
	if supplierName == "" {
		supplierName = "Nhà cung cấp / Thầu phụ"
	}
	metadata := Record{
		"node":                   contextRow["run_node_name"],
		"procurement_request_id": procurementID,
	}
	acceptanceRows, err := queryTx(ctx, tx, `
		SELECT id
		FROM incoming_acceptance_records
		WHERE project_id=$1
			AND procurement_request_id=$2
			AND source_type='procurement'
			AND status <> 'cancelled'
		ORDER BY created_at DESC
		LIMIT 1
	`, projectID, procurementID)
	if err != nil {
		return "", "", nil, err
	}
	acceptanceID := ""
	if len(acceptanceRows) > 0 {
		acceptanceID = cleanID(acceptanceRows[0]["id"])
		metadata["incoming_acceptance_existing"] = true
	} else {
		code := fmt.Sprintf("IA-%s-%d-%s", projectID[:8], timeNowUnix(), newID()[:8])
		rawSummary, _ := json.Marshal(Record{"automation_scaffold": true, "procurement_request_id": procurementID})
		rows, err := queryTx(ctx, tx, `
			INSERT INTO incoming_acceptance_records (
				acceptance_code, project_id, work_item_id, procurement_request_id,
				source_type, supplier_name, delivery_reference, status, summary_json,
				created_by, updated_by
			)
			VALUES ($1,$2,$3,$4,'procurement',$5,$6,'draft',$7::jsonb,NULLIF($8,'')::uuid,NULLIF($8,'')::uuid)
			RETURNING id
		`, code, projectID, workItemID, procurementID, supplierName, procurement["request_code"], string(rawSummary), actorID)
		if err != nil {
			return "", "", nil, err
		}
		if len(rows) == 0 {
			return "", "", nil, sql.ErrNoRows
		}
		acceptanceID = cleanID(rows[0]["id"])
		metadata["incoming_acceptance_created"] = true
		if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "incoming_acceptance_saved", "incoming_acceptance", acceptanceID, "Tự động mở hồ sơ nghiệm thu đầu vào dạng nháp", metadata); err != nil {
			return "", "", nil, err
		}
	}
	payableRows, err := queryTx(ctx, tx, `
		SELECT id
		FROM payables
		WHERE project_id=$1 AND source_type='procurement' AND source_id=$2 AND status <> 'cancelled'
		ORDER BY created_at DESC
		LIMIT 1
	`, projectID, procurementID)
	if err != nil {
		return "", "", nil, err
	}
	payableID := ""
	amount := number(procurement["total_estimated_amount"])
	if len(payableRows) > 0 {
		payableID = cleanID(payableRows[0]["id"])
		metadata["payable_existing"] = true
	} else if amount > 0 {
		code := fmt.Sprintf("AP-AUTO-%s-%d-%s", projectID[:8], timeNowUnix(), newID()[:6])
		rows, err := queryTx(ctx, tx, `
			INSERT INTO payables (
				project_id, payable_code, vendor_type, vendor_name, source_type, source_id,
				gross_amount, paid_amount, remaining_amount, status, created_by
			)
			VALUES ($1,$2,'supplier',$3,'procurement',$4,$5,0,$5,'draft',NULLIF($6,'')::uuid)
			RETURNING id
		`, projectID, code, supplierName, procurementID, amount, actorID)
		if err != nil {
			return "", "", nil, err
		}
		if len(rows) == 0 {
			return "", "", nil, sql.ErrNoRows
		}
		payableID = cleanID(rows[0]["id"])
		metadata["payable_created"] = true
		metadata["payable_amount"] = amount
		if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "payable_created", "payable", payableID, "Tự động mở công nợ phải trả dạng nháp từ yêu cầu mua", metadata); err != nil {
			return "", "", nil, err
		}
	} else {
		metadata["payable_blocker"] = "Yêu cầu mua chưa có giá trị dự toán để mở công nợ phải trả"
	}
	if payableID != "" {
		if err := r.ensurePayablePaymentScheduleScaffoldTx(ctx, tx, projectID, payableID, actorID, actorName, metadata); err != nil {
			return "", "", nil, err
		}
	}
	return acceptanceID, payableID, metadata, nil
}

func (r *Repository) automateClientOutgoingAcceptanceFinance(ctx context.Context, contextRow Record, actorID, actorName string) (workflowAutomationOutcome, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	defer tx.Rollback()
	acceptanceID, receivableID, metadata, err := r.ensureOutgoingAcceptanceFinanceScaffoldTx(ctx, tx, contextRow, actorID, actorName)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	if err := tx.Commit(); err != nil {
		return workflowAutomationOutcome{}, err
	}
	if receivableID != "" {
		metadata["receivable_id"] = receivableID
		ready, err := r.clientOutputInvoiceIssued(ctx, receivableID)
		if err != nil {
			return workflowAutomationOutcome{}, err
		}
		if !ready {
			metadata["output_invoice_required"] = "Cần tải lên và phát hành hóa đơn đầu ra thật"
			return automationBlockedWithTarget("outgoing_acceptance_finance_scaffold", "Cần tải lên và phát hành hóa đơn đầu ra thật", "receivable", receivableID, metadata), nil
		}
	} else {
		return automationBlockedWithTarget("outgoing_acceptance_finance_scaffold", "Cần mở công nợ phải thu trước khi lập lịch thu tiền và hóa đơn thật", "acceptance", acceptanceID, metadata), nil
	}
	return automationSucceeded("outgoing_acceptance_finance_scaffold", "Đã mở hồ sơ nghiệm thu đầu ra, công nợ và lịch thu tiền dạng nháp", "acceptance", acceptanceID, metadata), nil
}

func (r *Repository) ensureOutgoingAcceptanceFinanceScaffoldTx(ctx context.Context, tx *sql.Tx, contextRow Record, actorID, actorName string) (string, string, Record, error) {
	projectID := cleanID(contextRow["project_id"])
	workItemID := cleanID(contextRow["id"])
	runNodeID := cleanID(contextRow["run_node_id"])
	metadata := Record{"node": contextRow["run_node_name"]}
	var acceptanceRows []Record
	var err error
	if runNodeID != "" {
		acceptanceRows, err = queryTx(ctx, tx, `
			SELECT id
			FROM acceptance_records
			WHERE project_id=$1
				AND node_id=$2
				AND type='final'
				AND status <> 'cancelled'
				AND summary_json->>'automation_scaffold'='true'
			ORDER BY created_at DESC
			LIMIT 1
		`, projectID, runNodeID)
	} else {
		acceptanceRows, err = queryTx(ctx, tx, `
			SELECT id
			FROM acceptance_records
			WHERE work_item_id=$1 AND status <> 'cancelled'
			ORDER BY created_at DESC
			LIMIT 1
		`, workItemID)
	}
	if err != nil {
		return "", "", nil, err
	}
	acceptanceID := ""
	if len(acceptanceRows) > 0 {
		acceptanceID = cleanID(acceptanceRows[0]["id"])
		metadata["acceptance_existing"] = true
	} else {
		code := fmt.Sprintf("OA-%s-%d-%s", projectID[:8], timeNowUnix(), newID()[:8])
		rawSummary, _ := json.Marshal(Record{"automation_scaffold": true})
		rows, err := queryTx(ctx, tx, `
			INSERT INTO acceptance_records (
				acceptance_code, project_id, node_id, work_item_id, type, title,
				requested_by, status, signature_status, document_status, summary_json, notes
			)
			VALUES ($1,$2,NULLIF($3,'')::uuid,$4,'final','Nghiệm thu đầu ra chính thức',NULLIF($5,'')::uuid,'draft','pending','pending',$6::jsonb,'Tự động mở từ quy trình chính thức')
			RETURNING id
		`, code, projectID, runNodeID, workItemID, actorID, string(rawSummary))
		if err != nil {
			return "", "", nil, err
		}
		if len(rows) == 0 {
			return "", "", nil, sql.ErrNoRows
		}
		acceptanceID = cleanID(rows[0]["id"])
		metadata["acceptance_created"] = true
		if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "outgoing_acceptance_saved", "acceptance", acceptanceID, "Tự động mở hồ sơ nghiệm thu đầu ra dạng nháp", metadata); err != nil {
			return "", "", nil, err
		}
	}
	projectRows, err := queryTx(ctx, tx, `SELECT client_name, contract_amount FROM projects WHERE id=$1`, projectID)
	if err != nil {
		return "", "", nil, err
	}
	clientName := "Chủ đầu tư"
	contractAmount := 0.0
	if len(projectRows) > 0 {
		clientName = cleanOptionalText(projectRows[0]["client_name"])
		contractAmount = number(projectRows[0]["contract_amount"])
	}
	if clientName == "" {
		clientName = "Chủ đầu tư"
	}
	receivableRows, err := queryTx(ctx, tx, `
		SELECT id
		FROM receivables
		WHERE project_id=$1 AND source_type='acceptance' AND source_id=$2 AND status <> 'cancelled'
		ORDER BY created_at DESC
		LIMIT 1
	`, projectID, acceptanceID)
	if err != nil {
		return "", "", nil, err
	}
	receivableID := ""
	if len(receivableRows) > 0 {
		receivableID = cleanID(receivableRows[0]["id"])
		metadata["receivable_existing"] = true
	} else if contractAmount > 0 {
		code := fmt.Sprintf("AR-AUTO-%s-%d-%s", projectID[:8], timeNowUnix(), newID()[:6])
		rows, err := queryTx(ctx, tx, `
			INSERT INTO receivables (
				project_id, receivable_code, client_name, source_type, source_id,
				contract_amount, milestone_name, gross_amount, received_amount, remaining_amount,
				status, created_by
			)
			VALUES ($1,$2,$3,'acceptance',$4,$5,'Nghiệm thu đầu ra',$5,0,$5,'draft',NULLIF($6,'')::uuid)
			RETURNING id
		`, projectID, code, clientName, acceptanceID, contractAmount, actorID)
		if err != nil {
			return "", "", nil, err
		}
		if len(rows) == 0 {
			return "", "", nil, sql.ErrNoRows
		}
		receivableID = cleanID(rows[0]["id"])
		metadata["receivable_created"] = true
		metadata["receivable_amount"] = contractAmount
		if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "receivable_created", "receivable", receivableID, "Tự động mở công nợ phải thu dạng nháp từ nghiệm thu", metadata); err != nil {
			return "", "", nil, err
		}
	} else {
		metadata["receivable_blocker"] = "Công trình chưa có giá trị hợp đồng để mở công nợ phải thu"
	}
	if receivableID != "" {
		if err := r.ensureReceivablePaymentScheduleScaffoldTx(ctx, tx, projectID, receivableID, actorID, actorName, metadata); err != nil {
			return "", "", nil, err
		}
	}
	return acceptanceID, receivableID, metadata, nil
}

func (r *Repository) automateClientHandoverClosure(ctx context.Context, contextRow Record, actorID, actorName string) (workflowAutomationOutcome, error) {
	projectID := cleanID(contextRow["project_id"])
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	defer tx.Rollback()
	handoverID, metadata, err := r.ensureHandoverScaffoldTx(ctx, tx, contextRow, actorID, actorName)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	if err := tx.Commit(); err != nil {
		return workflowAutomationOutcome{}, err
	}
	if handoverID == "" {
		return automationBlocked("handover_financial_closure_check", "Chưa có hồ sơ nghiệm thu đầu ra để mở bàn giao", metadata), nil
	}
	handover, err := r.GetHandoverDetail(ctx, handoverID)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	if fmt.Sprint(handover["status"]) == "draft" {
		metadata["handover_blockers"] = []string{"Cần hoàn thiện và gửi hồ sơ bàn giao trước khi kiểm tra readiness"}
		return automationBlockedWithTarget("handover_financial_closure_check", "Hồ sơ bàn giao chưa được gửi kiểm tra", "handover", handoverID, metadata), nil
	}
	check, err := r.HandoverCheck(ctx, handoverID, actorID, actorName)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	blockers, _ := check["blockers"].([]string)
	if len(blockers) > 0 {
		metadata["handover_blockers"] = blockers
		return automationBlockedWithTarget("handover_financial_closure_check", "Bàn giao/đóng tài chính còn điều kiện chưa hoàn tất", "handover", handoverID, metadata), nil
	}
	metadata["project_id"] = projectID
	return automationSucceeded("handover_financial_closure_check", "Bàn giao và đóng tài chính đã sẵn sàng", "handover", handoverID, metadata), nil
}

func (r *Repository) ensurePayablePaymentScheduleScaffoldTx(ctx context.Context, tx *sql.Tx, projectID, payableID, actorID, actorName string, metadata Record) error {
	rows, err := queryTx(ctx, tx, `
		SELECT gross_amount, remaining_amount
		FROM payables
		WHERE id=$1 AND project_id=$2 AND status <> 'cancelled'
		FOR UPDATE
	`, payableID, projectID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		metadata["payment_schedule_blocker"] = "Không tìm thấy công nợ phải trả để lập lịch thanh toán"
		return nil
	}
	amount := number(rows[0]["remaining_amount"])
	if amount <= 0 {
		amount = number(rows[0]["gross_amount"])
	}
	if amount <= 0 {
		metadata["payment_schedule_blocker"] = "Công nợ phải trả không có số tiền để lập lịch thanh toán"
		return nil
	}
	const scheduleCode = "LICH-THANH-TOAN-001"
	existing, err := queryTx(ctx, tx, `
		SELECT id
		FROM payable_payment_schedules
		WHERE payable_id=$1
			AND schedule_code=$2
			AND status NOT IN ('cancelled','deleted')
		ORDER BY created_at DESC
		LIMIT 1
	`, payableID, scheduleCode)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		metadata["payment_schedule_id"] = cleanID(existing[0]["id"])
		metadata["payment_schedule_existing"] = true
		return nil
	}
	scheduleID := newID()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO payable_payment_schedules (
			id, project_id, payable_id, schedule_code, milestone_name, schedule_type,
			planned_amount, paid_amount, remaining_amount, status, notes, created_by
		)
		VALUES ($1,$2,$3,$4,'Thanh toán đầu vào sau nghiệm thu','final',$5,0,$5,'draft','Tự động tạo từ quy trình chính thức; kế toán cần kiểm tra trước khi duyệt.',NULLIF($6,'')::uuid)
	`, scheduleID, projectID, payableID, scheduleCode, amount, actorID); err != nil {
		return err
	}
	metadata["payment_schedule_id"] = scheduleID
	metadata["payment_schedule_created"] = true
	metadata["payment_schedule_amount"] = amount
	return addAuditTx(ctx, tx, projectID, actorID, actorName, "payable_payment_schedule_created", "payable_payment_schedule", scheduleID, "Tự động tạo lịch thanh toán đầu vào dạng nháp", Record{
		"payable_id":     payableID,
		"schedule_code":  scheduleCode,
		"planned_amount": amount,
		"schedule_type":  "final",
	})
}

func (r *Repository) ensureReceivablePaymentScheduleScaffoldTx(ctx context.Context, tx *sql.Tx, projectID, receivableID, actorID, actorName string, metadata Record) error {
	rows, err := queryTx(ctx, tx, `
		SELECT gross_amount, remaining_amount
		FROM receivables
		WHERE id=$1 AND project_id=$2 AND status <> 'cancelled'
		FOR UPDATE
	`, receivableID, projectID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		metadata["billing_schedule_blocker"] = "Không tìm thấy công nợ phải thu để lập lịch thu tiền"
		return nil
	}
	amount := number(rows[0]["remaining_amount"])
	if amount <= 0 {
		amount = number(rows[0]["gross_amount"])
	}
	if amount <= 0 {
		metadata["billing_schedule_blocker"] = "Công nợ phải thu không có số tiền để lập lịch thu tiền"
		return nil
	}
	const scheduleCode = "LICH-THU-TIEN-001"
	existing, err := queryTx(ctx, tx, `
		SELECT id
		FROM receivable_payment_schedules
		WHERE receivable_id=$1
			AND schedule_code=$2
			AND status NOT IN ('cancelled','deleted')
		ORDER BY created_at DESC
		LIMIT 1
	`, receivableID, scheduleCode)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		metadata["billing_schedule_id"] = cleanID(existing[0]["id"])
		metadata["billing_schedule_existing"] = true
		return nil
	}
	scheduleID := newID()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO receivable_payment_schedules (
			id, project_id, receivable_id, schedule_code, milestone_name, schedule_type,
			planned_amount, received_amount, remaining_amount, status, notes, created_by
		)
		VALUES ($1,$2,$3,$4,'Thu tiền đầu ra sau nghiệm thu','final',$5,0,$5,'draft','Tự động tạo từ quy trình chính thức; kế toán cần kiểm tra trước khi duyệt.',NULLIF($6,'')::uuid)
	`, scheduleID, projectID, receivableID, scheduleCode, amount, actorID); err != nil {
		return err
	}
	metadata["billing_schedule_id"] = scheduleID
	metadata["billing_schedule_created"] = true
	metadata["billing_schedule_amount"] = amount
	return addAuditTx(ctx, tx, projectID, actorID, actorName, "receivable_payment_schedule_created", "receivable_payment_schedule", scheduleID, "Tự động tạo lịch thu tiền đầu ra dạng nháp", Record{
		"receivable_id":  receivableID,
		"schedule_code":  scheduleCode,
		"planned_amount": amount,
		"schedule_type":  "final",
	})
}

func (r *Repository) clientInputInvoiceVerified(ctx context.Context, payableID string) (bool, error) {
	rows, err := r.Query(ctx, `
		SELECT COUNT(*) AS total
		FROM input_invoices
		WHERE payable_id=$1 AND status='verified'
	`, payableID)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && number(rows[0]["total"]) > 0, nil
}

func (r *Repository) clientOutputInvoiceIssued(ctx context.Context, receivableID string) (bool, error) {
	rows, err := r.Query(ctx, `
		SELECT COUNT(*) AS total
		FROM output_invoices
		WHERE receivable_id=$1 AND status='issued'
	`, receivableID)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && number(rows[0]["total"]) > 0, nil
}

func (r *Repository) ensureHandoverScaffoldTx(ctx context.Context, tx *sql.Tx, contextRow Record, actorID, actorName string) (string, Record, error) {
	projectID := cleanID(contextRow["project_id"])
	metadata := Record{"node": contextRow["run_node_name"]}
	acceptanceRows, err := queryTx(ctx, tx, `
		SELECT id, status, result
		FROM acceptance_records
		WHERE project_id=$1 AND type='final' AND status <> 'cancelled'
		ORDER BY updated_at DESC, created_at DESC
		LIMIT 1
	`, projectID)
	if err != nil {
		return "", nil, err
	}
	if len(acceptanceRows) == 0 {
		metadata["handover_blocker"] = "missing_final_acceptance"
		return "", metadata, nil
	}
	finalAcceptanceID := cleanID(acceptanceRows[0]["id"])
	metadata["final_acceptance_id"] = finalAcceptanceID
	handoverRows, err := queryTx(ctx, tx, `
		SELECT id
		FROM handover_records
		WHERE project_id=$1 AND status <> 'deleted'
		ORDER BY created_at DESC
		LIMIT 1
	`, projectID)
	if err != nil {
		return "", nil, err
	}
	if len(handoverRows) > 0 {
		metadata["handover_existing"] = true
		return cleanID(handoverRows[0]["id"]), metadata, nil
	}
	handoverCode := "HO-" + strings.ToUpper(strings.ReplaceAll(newID(), "-", ""))[:10]
	rows, err := queryTx(ctx, tx, `
		INSERT INTO handover_records (project_id,handover_code,status,final_acceptance_id,client_representative,company_representative,notes,created_by)
		SELECT $1,$2,'draft',$3,COALESCE(NULLIF(p.client_name,''),'Chủ đầu tư'),'PM phụ trách','Tự động mở từ quy trình chính thức',NULLIF($4,'')::uuid
		FROM projects p WHERE p.id=$1
		RETURNING id
	`, projectID, handoverCode, finalAcceptanceID, actorID)
	if err != nil {
		return "", nil, err
	}
	if len(rows) == 0 {
		return "", nil, sql.ErrNoRows
	}
	handoverID := cleanID(rows[0]["id"])
	items := []Record{
		{"item_code": "final_acceptance", "category": "quality", "title": "Biên bản nghiệm thu cuối đạt", "source_type": "acceptance", "source_id": finalAcceptanceID},
		{"item_code": "warehouse_closure", "category": "operation", "title": "Kho công trình và vật tư dư đã đóng", "source_type": "system_check", "source_id": handoverID},
		{"item_code": "project_material_costs", "category": "finance", "title": "Chi phí vật tư công trình đã được xác nhận", "source_type": "system_check", "source_id": handoverID},
		{"item_code": "project_cash_requests", "category": "finance", "title": "Đề nghị thu/chi nội bộ đã được xử lý", "source_type": "system_check", "source_id": handoverID},
		{"item_code": "as_built_documents", "category": "as_built", "title": "Bản vẽ và hồ sơ hoàn công"},
		{"item_code": "operation_manuals", "category": "operation", "title": "Hướng dẫn vận hành/bảo trì"},
		{"item_code": "warranty_documents", "category": "warranty", "title": "Cam kết và hồ sơ bảo hành"},
		{"item_code": "signed_handover_minutes", "category": "legal", "title": "Biên bản bàn giao có xác nhận hai bên"},
	}
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO handover_checklist_items (handover_id,project_id,item_code,category,title,required,status,source_type,source_id)
			VALUES ($1,$2,$3,$4,$5,true,'pending',NULLIF($6,''),NULLIF($7,'')::uuid)
		`, handoverID, projectID, item["item_code"], item["category"], item["title"], item["source_type"], cleanID(item["source_id"])); err != nil {
			return "", nil, err
		}
	}
	metadata["handover_created"] = true
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "handover_created", "handover", handoverID, "Tự động mở hồ sơ bàn giao dạng nháp", metadata); err != nil {
		return "", nil, err
	}
	return handoverID, metadata, nil
}

func (r *Repository) firstProjectProcurement(ctx context.Context, projectID string) (Record, error) {
	rows, err := r.Query(ctx, `
		SELECT pr.*
		FROM procurement_requests pr
		WHERE pr.project_id=$1 AND pr.status <> 'cancelled'
		ORDER BY pr.created_at DESC
		LIMIT 1
	`, projectID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (r *Repository) firstApprovedProjectProcurement(ctx context.Context, projectID string) (Record, error) {
	rows, err := r.Query(ctx, `
		SELECT
			pr.*,
			COALESCE(NULLIF(string_agg(DISTINCT NULLIF(btrim(pri.supplier_name), ''), ', '), ''), 'Nhà cung cấp / Thầu phụ') AS supplier_name
		FROM procurement_requests pr
		LEFT JOIN procurement_request_items pri ON pri.request_id=pr.id
		WHERE pr.project_id=$1
		  AND pr.status IN ('approved','ordered','partially_received','received')
		GROUP BY pr.id
		ORDER BY pr.created_at DESC
		LIMIT 1
	`, projectID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func automationSucceeded(actionKey, message, targetType, targetID string, metadata Record) workflowAutomationOutcome {
	return workflowAutomationOutcome{
		ActionKey:  actionKey,
		Status:     "succeeded",
		Message:    message,
		TargetType: targetType,
		TargetID:   cleanID(targetID),
		Metadata:   metadata,
	}
}

func automationBlocked(actionKey, reason string, metadata Record) workflowAutomationOutcome {
	return automationBlockedWithTarget(actionKey, reason, "", "", metadata)
}

func automationBlockedWithTarget(actionKey, reason, targetType, targetID string, metadata Record) workflowAutomationOutcome {
	return workflowAutomationOutcome{
		ActionKey:     actionKey,
		Status:        "blocked",
		Message:       reason,
		TargetType:    targetType,
		TargetID:      cleanID(targetID),
		BlockerReason: reason,
		Metadata:      metadata,
	}
}

func clientWorkflowAutomationSource(contextRow Record, actionKey string) (string, string) {
	workItemID := cleanID(contextRow["id"])
	switch actionKey {
	case "incoming_acceptance_finance_scaffold", "outgoing_acceptance_finance_scaffold", "handover_financial_closure_check", "finance.scaffold_incoming_acceptance", "finance.scaffold_outgoing_acceptance", "handover.evaluate_readiness":
		if runNodeID := cleanID(contextRow["run_node_id"]); runNodeID != "" {
			return "pipeline_run_node", runNodeID
		}
	}
	return "work_item", workItemID
}

func (r *Repository) recordClientWorkflowAutomationOutcome(ctx context.Context, contextRow Record, outcome workflowAutomationOutcome, actorID, actorName string) error {
	if outcome.ActionKey == "" || outcome.Status == "" {
		return nil
	}
	projectID := cleanID(contextRow["project_id"])
	workItemID := cleanID(contextRow["id"])
	sourceType, sourceID := clientWorkflowAutomationSource(contextRow, outcome.ActionKey)
	idempotencyKey := strings.Join([]string{cleanID(contextRow["template_id"]), sourceType, sourceID, outcome.ActionKey}, ":")
	metadata := outcome.Metadata
	if metadata == nil {
		metadata = Record{}
	}
	metadata["template_id"] = cleanID(contextRow["template_id"])
	metadata["run_node_id"] = cleanID(contextRow["run_node_id"])
	metadata["run_step_id"] = cleanID(contextRow["run_step_id"])
	metadata["work_item_id"] = workItemID
	metadata["automation_source_type"] = sourceType
	metadata["automation_source_id"] = sourceID
	rawMetadata, _ := json.Marshal(metadata)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existing, err := queryTx(ctx, tx, `
		SELECT id, status, target_type, target_id, blocker_reason, error_message
		FROM workflow_automation_events
		WHERE idempotency_key=$1
		LIMIT 1
	`, idempotencyKey)
	if err != nil {
		return err
	}
	changed := len(existing) == 0
	if len(existing) > 0 {
		prev := existing[0]
		changed = fmt.Sprint(prev["status"]) != outcome.Status ||
			cleanOptionalText(prev["target_type"]) != outcome.TargetType ||
			cleanID(prev["target_id"]) != cleanID(outcome.TargetID) ||
			cleanOptionalText(prev["blocker_reason"]) != outcome.BlockerReason ||
			cleanOptionalText(prev["error_message"]) != outcome.ErrorMessage
	}
	rows, err := queryTx(ctx, tx, `
		INSERT INTO workflow_automation_events (
			project_id, pipeline_run_id, run_node_id, run_step_id, work_item_id,
			source_type, source_id, action_key, idempotency_key, status,
			target_type, target_id, blocker_reason, error_message, metadata_json,
			actor_id, actor_name
		)
		VALUES (
			$1, NULLIF($2,'')::uuid, NULLIF($3,'')::uuid, NULLIF($4,'')::uuid, NULLIF($5,'')::uuid,
			$6, $7, $8, $9, $10,
			NULLIF($11,''), NULLIF($12,'')::uuid, NULLIF($13,''), NULLIF($14,''), $15::jsonb,
			NULLIF($16,'')::uuid, NULLIF($17,'')
		)
		ON CONFLICT (idempotency_key)
		DO UPDATE SET
			status=EXCLUDED.status,
			target_type=EXCLUDED.target_type,
			target_id=EXCLUDED.target_id,
			blocker_reason=EXCLUDED.blocker_reason,
			error_message=EXCLUDED.error_message,
			metadata_json=EXCLUDED.metadata_json,
			actor_id=EXCLUDED.actor_id,
			actor_name=EXCLUDED.actor_name,
			run_count=workflow_automation_events.run_count + 1,
			updated_at=now()
		RETURNING id
	`, projectID, cleanID(contextRow["pipeline_run_id"]), cleanID(contextRow["run_node_id"]), cleanID(contextRow["run_step_id"]), workItemID,
		sourceType, sourceID, outcome.ActionKey, idempotencyKey, outcome.Status,
		outcome.TargetType, cleanID(outcome.TargetID), outcome.BlockerReason, outcome.ErrorMessage, string(rawMetadata),
		actorID, actorName)
	if err != nil {
		return err
	}
	eventID := ""
	if len(rows) > 0 {
		eventID = cleanID(rows[0]["id"])
	}
	if changed && workItemID != "" {
		activityType := "system_event"
		if outcome.Status == "blocked" {
			activityType = "blocker"
		}
		activityMetadata := Record{
			"automation_event_id": eventID,
			"automation_action":   outcome.ActionKey,
			"automation_status":   outcome.Status,
			"target_type":         outcome.TargetType,
			"target_id":           cleanID(outcome.TargetID),
			"metadata":            metadata,
		}
		if outcome.BlockerReason != "" {
			activityMetadata["blocker_reason"] = outcome.BlockerReason
		}
		if outcome.ErrorMessage != "" {
			activityMetadata["error_message"] = outcome.ErrorMessage
		}
		rawActivityMetadata, _ := json.Marshal(activityMetadata)
		comment := outcome.Message
		if comment == "" {
			comment = "Tự động hóa quy trình đã cập nhật trạng thái"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, $4, $5, 'project', $6::jsonb)
		`, workItemID, projectID, actorID, activityType, comment, string(rawActivityMetadata)); err != nil {
			return err
		}
		if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "workflow_automation_"+outcome.Status, "work_item", workItemID, comment, activityMetadata); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) RequestNodeApproval(ctx context.Context, nodeID, actorID, actorName, reason string) (Record, error) {
	node, err := r.GetByID(ctx, "pipeline_run_nodes", nodeID)
	if err != nil {
		return nil, err
	}
	if existing, err := r.Query(ctx, `
		SELECT * FROM approval_requests
		WHERE target_type='pipeline_run_node' AND target_id=$1 AND status='pending'
		ORDER BY created_at DESC LIMIT 1
	`, nodeID); err != nil {
		return nil, err
	} else if len(existing) > 0 {
		return existing[0], nil
	}
	policyRecord, err := normalizeApprovalPolicyValue(node["approval_policy_json"])
	if err != nil {
		return nil, err
	}
	policy := approvalPolicyFromValue(policyRecord)
	runRows, err := r.Query(ctx, `SELECT engine_version FROM pipeline_runs WHERE id=$1`, node["run_id"])
	if err != nil {
		return nil, err
	}
	engineVersion := 1
	if len(runRows) > 0 {
		engineVersion = int(number(runRows[0]["engine_version"]))
	}
	if engineVersion < 2 {
		policyRecord = Record{}
		policy = ApprovalPolicy{Roles: stringSlice(node["responsible_role_ids"])}
	} else if !policy.enabled() {
		roles := stringSlice(node["responsible_role_ids"])
		policyRecord = Record{"mode": "any", "roles": roles, "quorum": 1, "requester_cannot_approve": true, "comment_required": true, "signature_required": false}
		policy = approvalPolicyFromValue(policyRecord)
	}
	request, err := r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		created, mutationErr := insertRecord(ctx, tx, "approval_requests", []ColumnSpec{
			col("target_type"), col("target_id"), col("project_id"), col("requested_by"), col("status"), col("reason"), jsonCol("required_roles"), jsonCol("policy_json"), col("current_stage"),
		}, Record{
			"target_type":    "pipeline_run_node",
			"target_id":      nodeID,
			"project_id":     node["project_id"],
			"requested_by":   actorID,
			"status":         "pending",
			"reason":         reason,
			"required_roles": policy.Roles,
			"policy_json":    policyRecord,
			"current_stage":  0,
		})
		if mutationErr != nil {
			return nil, mutationErr
		}
		result, mutationErr := tx.ExecContext(ctx, `UPDATE pipeline_run_nodes SET status='waiting_approval' WHERE id=$1`, nodeID)
		if mutationErr != nil {
			return nil, mutationErr
		}
		if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
			return nil, rowsErr
		} else if affected != 1 {
			return nil, sql.ErrNoRows
		}
		return created, nil
	}, func(request Record) auditEvent {
		return auditEvent{
			ProjectID: fmt.Sprint(node["project_id"]), ActorID: actorID, ActorName: actorName,
			EventType: "review_requested", TargetType: "pipeline_run_node", TargetID: nodeID,
			Description: fmt.Sprintf("Yêu cầu duyệt node %s", node["name"]), Metadata: Record{"approval_request_id": request["id"]},
		}
	})
	if err != nil {
		return nil, err
	}
	return request, nil
}

func (r *Repository) AdvanceRunNode(ctx context.Context, nodeID, actorID, actorName string) (Record, error) {
	node, err := r.GetByID(ctx, "pipeline_run_nodes", nodeID)
	if err != nil {
		return nil, err
	}
	completionIssues, err := r.NodeCompletionIssues(ctx, node)
	if err != nil {
		return nil, err
	}
	if len(completionIssues) > 0 {
		return nil, validationError("Node chưa đạt điều kiện hoàn tất", Record{"issues": completionIssues})
	}
	if blockers := r.nodeBlockers(ctx, node); len(blockers) > 0 {
		reason := stringsJoin(blockers)
		blocked, err := r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
			rows, mutationErr := queryTx(ctx, tx, `UPDATE pipeline_run_nodes SET status='blocked', block_reason=$2 WHERE id=$1 RETURNING *`, nodeID, reason)
			if mutationErr != nil {
				return nil, mutationErr
			}
			if len(rows) == 0 {
				return nil, sql.ErrNoRows
			}
			return rows[0], nil
		}, func(Record) auditEvent {
			return auditEvent{
				ProjectID: fmt.Sprint(node["project_id"]), ActorID: actorID, ActorName: actorName,
				EventType: "node_blocked", TargetType: "pipeline_run_node", TargetID: nodeID,
				Description: reason, Metadata: Record{"blockers": blockers},
			}
		})
		if err != nil {
			return nil, err
		}
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
		metadata := Record{"blockers": transitionBlockers}
		if transition != nil {
			metadata["transition"] = transition
		}
		if err := addAuditTx(ctx, tx, fmt.Sprint(node["project_id"]), actorID, actorName, "node_blocked", "pipeline_run_node", nodeID, reason, metadata); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
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
		if err := skipUnselectedDirectBranchesTx(ctx, tx, node, nextID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE pipeline_run_nodes SET status='in_progress', started_at=COALESCE(started_at, now()) WHERE id=$1`, nextID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE work_items SET status='ready', updated_at=now() WHERE run_node_id=$1 AND status='pending'`, nextID); err != nil {
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
			WHERE rn.run_id=$2 AND rn.status <> 'skipped'
		),0),
		status = CASE WHEN EXISTS (SELECT 1 FROM pipeline_runs pr WHERE pr.id=$2 AND pr.status='completed') THEN 'financial_closure' ELSE p.status END,
		updated_at=now()
		WHERE p.id=$1
	`, node["project_id"], node["run_id"]); err != nil {
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
	if err := addAuditTx(ctx, tx, fmt.Sprint(node["project_id"]), actorID, actorName, "node_completed", "pipeline_run_node", nodeID, description, metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPipelineRun(ctx, fmt.Sprint(node["project_id"]))
}

func skipUnselectedDirectBranchesTx(ctx context.Context, tx *sql.Tx, fromNode Record, selectedNodeID string) error {
	skippedRows, err := queryTx(ctx, tx, `
		WITH RECURSIVE reachable(id) AS (
			SELECT $3::uuid
			UNION
			SELECT e.to_run_node_id
			FROM pipeline_run_edges e
			JOIN reachable r ON r.id=e.from_run_node_id
			WHERE e.run_id=$1
		), alternatives AS (
			SELECT DISTINCT e.to_run_node_id AS id
			FROM pipeline_run_edges e
			WHERE e.run_id=$1
			  AND e.from_run_node_id=$2
			  AND e.to_run_node_id <> $3
			  AND NOT EXISTS (SELECT 1 FROM reachable r WHERE r.id=e.to_run_node_id)
		)
		UPDATE pipeline_run_nodes rn
		SET status='skipped', block_reason='Nhánh không được chọn bởi điều kiện chuyển bước'
		FROM alternatives a
		WHERE rn.id=a.id AND rn.status='pending'
		RETURNING rn.id
	`, fromNode["run_id"], fromNode["id"], selectedNodeID)
	if err != nil {
		return err
	}
	for _, skipped := range skippedRows {
		skippedNodeID := skipped["id"]
		if _, err := tx.ExecContext(ctx, `UPDATE pipeline_run_steps SET status='skipped' WHERE run_node_id=$1 AND status='pending'`, skippedNodeID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE work_items SET status='cancelled', updated_at=now() WHERE run_node_id=$1 AND status='pending'`, skippedNodeID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE workflow_form_instances SET status='cancelled', updated_at=now() WHERE run_node_id=$1 AND status='draft'`, skippedNodeID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) nextRunNode(ctx context.Context, tx *sql.Tx, node Record) (Record, Record, []string, error) {
	edges, err := queryTx(ctx, tx, `
		SELECT e.*, rn.name AS to_node_name, rn.order_index AS to_order_index, rn.status AS to_status
		FROM pipeline_run_edges e
		JOIN pipeline_run_nodes rn ON rn.id=e.to_run_node_id
		WHERE e.run_id=$1 AND e.from_run_node_id=$2 AND rn.status <> 'skipped'
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
		WHERE run_id=$1 AND order_index > $2 AND status <> 'skipped'
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
		return true, "Điều kiện luôn khớp", nil
	case "order_index":
		matched := number(edge["to_order_index"]) > number(fromNode["order_index"])
		return matched, boolReason(matched, "Node đích đứng sau node nguồn", "Node đích không đứng sau node nguồn"), nil
	case "approval_approved":
		return r.edgeExists(ctx, tx, "Phê duyệt node đã được duyệt", "Phê duyệt node chưa được duyệt", `
			SELECT COUNT(*) AS total
			FROM approval_requests
			WHERE project_id=$1 AND target_type='pipeline_run_node' AND target_id=$2 AND status='approved'
		`, projectID, nodeID)
	case "approval_rejected":
		return r.edgeExists(ctx, tx, "Phê duyệt node đã bị từ chối", "Phê duyệt node chưa bị từ chối", `
			SELECT COUNT(*) AS total
			FROM approval_requests
			WHERE project_id=$1 AND target_type='pipeline_run_node' AND target_id=$2 AND status='rejected'
		`, projectID, nodeID)
	case "acceptance_passed":
		return r.edgeExists(ctx, tx, "Nghiệm thu đã đạt", "Chưa có nghiệm thu hoàn tất đạt", `
			SELECT COUNT(*) AS total
			FROM acceptance_records
			WHERE project_id=$1 AND result='pass' AND status='completed'
		`, projectID)
	case "acceptance_failed":
		return r.edgeExists(ctx, tx, "Nghiệm thu không đạt", "Chưa có nghiệm thu không đạt", `
			SELECT COUNT(*) AS total
			FROM acceptance_records
			WHERE project_id=$1 AND result='fail'
		`, projectID)
	case "rework_required":
		return r.edgeExists(ctx, tx, "Cần sửa lỗi", "Không có yêu cầu sửa lỗi đang mở", `
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
		return matched, boolReason(matched, "Tất cả yêu cầu sửa lỗi đã hoàn tất", "Yêu cầu sửa lỗi vẫn đang mở hoặc chưa tồn tại"), nil
	case "replanning_required":
		return r.edgeExists(ctx, tx, "Cần lập kế hoạch lại", "Không có yêu cầu lập kế hoạch lại đang mở", `
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
		return matched, boolReason(matched, "Vật tư theo kế hoạch đã xuất đủ", "Vật tư theo kế hoạch vẫn còn thiếu"), nil
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
		return matched, boolReason(matched, "Tất cả dòng BOQ đã được duyệt", "BOQ đang thiếu dữ liệu hoặc còn dòng chưa duyệt"), nil
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
		return matched, boolReason(matched, "Yêu cầu mua/đầu vào đã được duyệt", "Thiếu yêu cầu mua/đầu vào hoặc còn yêu cầu chưa được duyệt"), nil
	case "workflow_form_answer_equals":
		return r.workflowFormAnswerEqualsCondition(ctx, tx, edge, projectID, nodeID)
	case "project_entry_mode":
		config := edgeConditionConfig(edge)
		expected := strings.TrimSpace(fmt.Sprint(config["value"]))
		var actual string
		if err := tx.QueryRowContext(ctx, `SELECT workflow_entry_mode FROM projects WHERE id=$1`, projectID).Scan(&actual); err != nil {
			return false, "", err
		}
		matched := actual == expected
		return matched, boolReason(matched, fmt.Sprintf("Điểm bắt đầu công trình là %s", expected), fmt.Sprintf("Điểm bắt đầu công trình là %s, không phải %s", actual, expected)), nil
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
		return matched, boolReason(matched, "Không còn công nợ phải thu hoặc phải trả đang mở", "Công nợ phải thu hoặc phải trả vẫn còn mở"), nil
	default:
		return false, fmt.Sprintf("Loại điều kiện chuyển bước chưa được hỗ trợ: %s", edgeConditionType(edge)), nil
	}
}

func (r *Repository) workflowFormAnswerEqualsCondition(ctx context.Context, tx *sql.Tx, edge Record, projectID, runNodeID string) (bool, string, error) {
	config := edgeConditionConfig(edge)
	fieldKey := strings.TrimSpace(fmt.Sprint(config["field_key"]))
	if fieldKey == "" || fieldKey == "<nil>" {
		return false, "Điều kiện biểu mẫu quy trình thiếu field_key", nil
	}
	expected, hasExpected := config["value"]
	if !hasExpected {
		expected, hasExpected = config["expected_value"]
	}
	if !hasExpected {
		return false, fmt.Sprintf("Điều kiện biểu mẫu quy trình %s thiếu giá trị mong đợi", fieldKey), nil
	}
	rows, err := queryTx(ctx, tx, `
		SELECT answers_snapshot_json
		FROM workflow_form_instances
		WHERE project_id=$1
			AND run_node_id=$2
			AND status='submitted'
			AND answers_snapshot_json ? $3
		ORDER BY submitted_at DESC NULLS LAST, updated_at DESC, id DESC
		LIMIT 1
	`, projectID, runNodeID, fieldKey)
	if err != nil {
		return false, "", err
	}
	if len(rows) == 0 {
		return false, fmt.Sprintf("Câu trả lời biểu mẫu quy trình %s chưa được gửi", fieldKey), nil
	}
	answers := recordFromAny(rows[0]["answers_snapshot_json"])
	actual, ok := answers[fieldKey]
	if !ok {
		return false, fmt.Sprintf("Câu trả lời biểu mẫu quy trình %s đang thiếu", fieldKey), nil
	}
	matched := workflowFormAnswerValueMatches(actual, expected)
	return matched, boolReason(
		matched,
		fmt.Sprintf("Câu trả lời biểu mẫu quy trình %s khớp %s", fieldKey, edgeExpectedDisplay(expected)),
		fmt.Sprintf("Câu trả lời biểu mẫu quy trình %s không khớp %s", fieldKey, edgeExpectedDisplay(expected)),
	), nil
}

func edgeConditionConfig(edge Record) Record {
	return recordFromAny(edge["condition_config"])
}

func recordFromAny(value any) Record {
	switch typed := value.(type) {
	case Record:
		return typed
	case map[string]any:
		return Record(typed)
	default:
		return Record{}
	}
}

func workflowFormAnswerValueMatches(actual any, expected any) bool {
	switch expectedValues := expected.(type) {
	case []any:
		for _, expectedValue := range expectedValues {
			if workflowFormAnswerValueMatches(actual, expectedValue) {
				return true
			}
		}
		return false
	case []string:
		for _, expectedValue := range expectedValues {
			if workflowFormAnswerValueMatches(actual, expectedValue) {
				return true
			}
		}
		return false
	}
	switch actualValues := actual.(type) {
	case []any:
		for _, actualValue := range actualValues {
			if workflowFormAnswerValueMatches(actualValue, expected) {
				return true
			}
		}
		return false
	case []string:
		for _, actualValue := range actualValues {
			if workflowFormAnswerValueMatches(actualValue, expected) {
				return true
			}
		}
		return false
	}
	if actualBool, ok := actual.(bool); ok {
		expectedBool, err := strconv.ParseBool(strings.TrimSpace(fmt.Sprint(expected)))
		return err == nil && actualBool == expectedBool
	}
	if expectedBool, ok := expected.(bool); ok {
		actualBool, err := strconv.ParseBool(strings.TrimSpace(fmt.Sprint(actual)))
		return err == nil && actualBool == expectedBool
	}
	if actualNumber, ok := strictFloat(actual); ok {
		expectedNumber, expectedOK := strictFloat(expected)
		return expectedOK && actualNumber == expectedNumber
	}
	if expectedNumber, ok := strictFloat(expected); ok {
		actualNumber, actualOK := strictFloat(actual)
		return actualOK && actualNumber == expectedNumber
	}
	return strings.EqualFold(strings.TrimSpace(fmt.Sprint(actual)), strings.TrimSpace(fmt.Sprint(expected)))
}

func strictFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(text, 64)
		return parsed, err == nil && !math.IsNaN(parsed)
	default:
		return 0, false
	}
}

func edgeExpectedDisplay(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return string(raw)
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
	return matched, boolReason(matched, "Hồ sơ bàn giao không còn điểm chặn", "Hồ sơ bàn giao còn điểm chặn"), nil
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
			blockers = append(blockers, "Vật tư bắt buộc chưa được xuất đủ ra công trình")
		}
	}
	return blockers
}

func (r *Repository) BlockRunNode(ctx context.Context, nodeID, reason, actorID, actorName string) (Record, error) {
	node, err := r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		rows, mutationErr := queryTx(ctx, tx, `UPDATE pipeline_run_nodes SET status='blocked', block_reason=$2 WHERE id=$1 RETURNING *`, nodeID, reason)
		if mutationErr != nil {
			return nil, mutationErr
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		return rows[0], nil
	}, func(node Record) auditEvent {
		return auditEvent{
			ProjectID: fmt.Sprint(node["project_id"]), ActorID: actorID, ActorName: actorName,
			EventType: "node_blocked", TargetType: "pipeline_run_node", TargetID: nodeID,
			Description: reason, Metadata: Record{},
		}
	})
	if err != nil {
		return nil, err
	}
	return node, nil
}

func (r *Repository) ApprovalAction(ctx context.Context, approvalID, action, comment, actorID, actorName string) (Record, error) {
	request, err := r.GetByID(ctx, "approval_requests", approvalID)
	if err != nil {
		return nil, err
	}
	status, ok := approvalStatusForAction(action)
	if !ok {
		return nil, validationError("Hành động phê duyệt không hợp lệ", Record{"action": action})
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
	actionLabel := approvalAuditActionLabel(action)
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, event, targetType, targetID, fmt.Sprintf("%s phê duyệt: %s", actionLabel, comment), Record{"approval_request_id": approvalID, "action": action}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, "approval_requests", approvalID)
}

func (r *Repository) ListProjectBOQ(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT b.*, m.name AS material_name, selected_quote.quotation_code AS selected_quotation_code,
			selected_quote.supplier_partner_id AS selected_supplier_partner_id,
			selected_quote.supplier_name AS selected_supplier_name,
			selected_quote.quoted_unit_price,
			active_procurement.request_id AS procurement_request_id,
			active_procurement.request_code AS procurement_request_code,
			active_procurement.status AS procurement_request_status,
			COALESCE(active_procurement.request_count, 0) AS active_procurement_count,
			(active_procurement.request_id IS NOT NULL) AS has_active_procurement,
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
			), '[]'::jsonb) AS price_events,
			(SELECT COUNT(*) FROM boq_unit_price_events WHERE boq_item_id=b.id) AS price_event_count,
			((SELECT COUNT(*) FROM boq_unit_price_events WHERE boq_item_id=b.id) > 5) AS price_events_truncated
		FROM boq_items b
		LEFT JOIN materials m ON m.id=b.material_id
		LEFT JOIN LATERAL (
			SELECT q.quotation_code, q.supplier_partner_id, q.supplier_name, qi.unit_price AS quoted_unit_price
			FROM supplier_quotation_items qi
			JOIN supplier_quotations q ON q.id=qi.quotation_id
			WHERE qi.id=b.selected_quotation_item_id
		) selected_quote ON true
		LEFT JOIN LATERAL (
			SELECT linked.request_id, linked.request_code, linked.status, COUNT(*) OVER() AS request_count
			FROM (
				SELECT DISTINCT pr.id AS request_id, pr.request_code, pr.status, pr.created_at
				FROM procurement_request_items pri
				JOIN procurement_requests pr ON pr.id=pri.request_id
				WHERE pri.boq_item_id=b.id AND pr.status <> 'cancelled'
			) linked
			ORDER BY linked.created_at DESC, linked.request_id DESC
			LIMIT 1
		) active_procurement ON true
		WHERE b.project_id=$1 AND b.status <> 'deleted'
		ORDER BY b.category, b.created_at
	`, projectID)
}

func (r *Repository) GetBOQItemDetail(ctx context.Context, itemID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT b.*, m.name AS material_name, selected_quote.quotation_code AS selected_quotation_code,
			selected_quote.supplier_partner_id AS selected_supplier_partner_id,
			selected_quote.supplier_name AS selected_supplier_name,
			selected_quote.quoted_unit_price,
			active_procurement.request_id AS procurement_request_id,
			active_procurement.request_code AS procurement_request_code,
			active_procurement.status AS procurement_request_status,
			COALESCE(active_procurement.request_count, 0) AS active_procurement_count,
			(active_procurement.request_id IS NOT NULL) AS has_active_procurement,
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
			), '[]'::jsonb) AS price_events,
			(SELECT COUNT(*) FROM boq_unit_price_events WHERE boq_item_id=b.id) AS price_event_count,
			((SELECT COUNT(*) FROM boq_unit_price_events WHERE boq_item_id=b.id) > 20) AS price_events_truncated
		FROM boq_items b
		LEFT JOIN materials m ON m.id=b.material_id
		LEFT JOIN LATERAL (
			SELECT q.quotation_code, q.supplier_partner_id, q.supplier_name, qi.unit_price AS quoted_unit_price
			FROM supplier_quotation_items qi
			JOIN supplier_quotations q ON q.id=qi.quotation_id
			WHERE qi.id=b.selected_quotation_item_id
		) selected_quote ON true
		LEFT JOIN LATERAL (
			SELECT linked.request_id, linked.request_code, linked.status, COUNT(*) OVER() AS request_count
			FROM (
				SELECT DISTINCT pr.id AS request_id, pr.request_code, pr.status, pr.created_at
				FROM procurement_request_items pri
				JOIN procurement_requests pr ON pr.id=pri.request_id
				WHERE pri.boq_item_id=b.id AND pr.status <> 'cancelled'
			) linked
			ORDER BY linked.created_at DESC, linked.request_id DESC
			LIMIT 1
		) active_procurement ON true
		WHERE b.id=$1 AND b.status <> 'deleted'
	`, itemID)
}

func (r *Repository) BOQProjectAction(ctx context.Context, projectID, action, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "boq_lifecycle:"+projectID); err != nil {
		return nil, err
	}
	fromStatuses := []string{}
	toStatus := ""
	eventType := ""
	switch action {
	case "submit":
		fromStatuses, toStatus, eventType = []string{"draft", "revised"}, "submitted", "boq_submitted"
	case "approve":
		fromStatuses, toStatus, eventType = []string{"submitted"}, "approved", "boq_approved"
	default:
		return nil, validationError("Hành động BOQ không hợp lệ", Record{"action": action})
	}
	if action == "approve" {
		blockers, err := queryTx(ctx, tx, `
			SELECT id, work_item_name, status, norm_price_locked
			FROM boq_items
			WHERE project_id=$1 AND status='submitted' AND NOT norm_price_locked
			ORDER BY category, created_at
		`, projectID)
		if err != nil {
			return nil, err
		}
		if len(blockers) > 0 {
			return nil, validationError("Chỉ phê duyệt BOQ sau khi mọi dòng đã khóa đơn giá định mức", Record{"items": blockers})
		}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE boq_items SET status=$3,
			approved_by=CASE WHEN $4='approve' THEN NULLIF($5,'')::uuid ELSE approved_by END,
			updated_at=now()
		WHERE project_id=$1
		  AND (($2='submit' AND status IN ('draft','revised')) OR ($2='approve' AND status='submitted'))
	`, projectID, action, toStatus, action, actorID)
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, validationError("Không có dòng BOQ ở trạng thái phù hợp để thực hiện", Record{"action": action, "expected_statuses": fromStatuses})
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, eventType, "project_boq", projectID, fmt.Sprintf("Chuyển %d dòng BOQ sang %s", count, toStatus), Record{"action": action, "from_statuses": fromStatuses, "to_status": toStatus, "item_count": count}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return Record{"action": action, "from_status": fromStatuses[0], "from_statuses": fromStatuses, "to_status": toStatus, "item_count": count}, nil
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
	if err := insertBOQUnitPriceEventTx(ctx, tx, item, "expected_updated", "expected", nil, number(item["expected_unit_price"]), "Thiết lập đơn giá dự kiến ban đầu", actorID, actorName); err != nil {
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
	if !statusIn(fmt.Sprint(current["status"]), "draft", "revised") {
		return nil, validationError("Chỉ có thể cập nhật nội dung BOQ ở trạng thái nháp hoặc cần chỉnh sửa", Record{"boq_item_id": itemID, "status": current["status"]})
	}
	if boolValue(current["final_price_locked"]) && boqBodyChangesAny(current, body, finalLockedBOQProtectedFields...) {
		return nil, validationError("Dòng BOQ đã khóa đơn giá cuối, không thể sửa khối lượng/vật tư/đơn giá", Record{"boq_item_id": itemID})
	}
	if boolValue(current["norm_price_locked"]) && boqBodyChangesAny(current, body, "expected_unit_price", "unit", "quantity", "material_id") {
		return nil, validationError("Dòng BOQ đã khóa đơn giá định mức, không thể sửa đơn giá dự kiến, đơn vị, khối lượng hoặc vật tư", Record{"boq_item_id": itemID})
	}

	specs := []string{"category", "work_item_name", "description", "unit", "quantity", "expected_unit_price", "material_id", "node_id"}
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
		if err := insertBOQUnitPriceEventTx(ctx, tx, updated, "expected_updated", "expected", numberPtr(current["expected_unit_price"]), number(updated["expected_unit_price"]), "Cập nhật đơn giá dự kiến", actorID, actorName); err != nil {
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

func (r *Repository) ReviseBOQItem(ctx context.Context, itemID, reason, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var projectID string
	if err := tx.QueryRowContext(ctx, `SELECT project_id::text FROM boq_items WHERE id=$1 AND status <> 'deleted'`, itemID).Scan(&projectID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "boq_lifecycle:"+projectID); err != nil {
		return nil, err
	}
	rows, err := queryTx(ctx, tx, `SELECT * FROM boq_items WHERE id=$1 AND status <> 'deleted' FOR UPDATE`, itemID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	if !statusIn(fromStatus, "submitted", "approved") {
		return nil, validationError("Chỉ BOQ đã gửi hoặc đã phê duyệt mới có thể yêu cầu chỉnh sửa", Record{"boq_item_id": itemID, "status": fromStatus})
	}
	if boolValue(current["final_price_locked"]) {
		return nil, validationError("Dòng BOQ đã khóa đơn giá cuối, không thể yêu cầu chỉnh sửa", Record{"boq_item_id": itemID})
	}
	var hasActiveProcurement bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM procurement_request_items pri
			JOIN procurement_requests pr ON pr.id=pri.request_id
			WHERE pri.boq_item_id=$1 AND pr.status <> 'cancelled'
		)
	`, itemID).Scan(&hasActiveProcurement); err != nil {
		return nil, err
	}
	if hasActiveProcurement {
		return nil, validationError("Dòng BOQ đã sinh yêu cầu mua, không thể yêu cầu chỉnh sửa", Record{"boq_item_id": itemID})
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE boq_items
		SET status='revised', approved_by=NULL, updated_at=now()
		WHERE id=$1
	`, itemID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "boq_revision_requested", "boq_item", itemID, "Yêu cầu chỉnh sửa dòng BOQ", Record{"from_status": fromStatus, "to_status": "revised", "reason": reason}); err != nil {
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
	if eventType == "expected_updated" && !statusIn(fmt.Sprint(current["status"]), "draft", "revised") {
		return nil, validationError("Chỉ có thể cập nhật đơn giá dự kiến ở trạng thái nháp hoặc cần chỉnh sửa", Record{"boq_item_id": itemID, "status": current["status"]})
	}
	if eventType == "norm_locked" && fmt.Sprint(current["status"]) != "submitted" {
		return nil, validationError("Chỉ có thể khóa đơn giá định mức khi dòng BOQ đã được gửi duyệt", Record{"boq_item_id": itemID, "status": current["status"]})
	}
	if eventType == "final_locked" && fmt.Sprint(current["status"]) != "approved" {
		return nil, validationError("Chỉ có thể khóa đơn giá cuối khi dòng BOQ đã được phê duyệt", Record{"boq_item_id": itemID, "status": current["status"]})
	}
	var updatedRows []Record
	var source string
	var fromPrice *float64
	switch eventType {
	case "expected_updated":
		if boolValue(current["norm_price_locked"]) || boolValue(current["final_price_locked"]) {
			return nil, validationError("Không thể sửa đơn giá dự kiến sau khi đã khóa đơn giá định mức/cuối", Record{"boq_item_id": itemID})
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
			return nil, validationError("Dòng BOQ đã khóa đơn giá định mức", Record{"boq_item_id": itemID})
		}
		if boolValue(current["final_price_locked"]) {
			return nil, validationError("Dòng BOQ đã khóa đơn giá cuối", Record{"boq_item_id": itemID})
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
			return nil, validationError("Cần khóa đơn giá định mức trước khi khóa đơn giá cuối", Record{"boq_item_id": itemID})
		}
		if boolValue(current["final_price_locked"]) {
			return nil, validationError("Dòng BOQ đã khóa đơn giá cuối", Record{"boq_item_id": itemID})
		}
		fromPrice = numberPtr(current["norm_unit_price"])
		updatedRows, err = queryTx(ctx, tx, `
			UPDATE boq_items
			SET final_unit_price=$2,
			    selected_quotation_item_id=NULL,
			    price_waiver_reason=$4,
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
		return nil, validationError("Hành động đơn giá không hợp lệ", Record{"event_type": eventType})
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

func (r *Repository) ListProjectSupplierQuotations(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT q.*, p.tax_code AS supplier_tax_code,
			COALESCE(summary.item_count, 0) AS item_count,
			COALESCE(summary.amount_before_tax, 0) AS amount_before_tax,
			COALESCE(summary.total_amount, 0) AS total_amount
		FROM supplier_quotations q
		JOIN partners p ON p.id=q.supplier_partner_id
		LEFT JOIN (
			SELECT quotation_id, COUNT(*) AS item_count, SUM(amount_before_tax) AS amount_before_tax, SUM(total_amount) AS total_amount
			FROM supplier_quotation_items
			GROUP BY quotation_id
		) summary ON summary.quotation_id=q.id
		WHERE q.project_id=$1 AND q.status <> 'cancelled'
		ORDER BY q.quotation_date DESC, q.created_at DESC
	`, projectID)
}

func (r *Repository) GetSupplierQuotationDetail(ctx context.Context, quotationID string) (Record, error) {
	quotation, err := r.QueryOne(ctx, `
		SELECT q.*, p.tax_code AS supplier_tax_code
		FROM supplier_quotations q
		JOIN partners p ON p.id=q.supplier_partner_id
		WHERE q.id=$1 AND q.status <> 'cancelled'
	`, quotationID)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT qi.*, b.category, b.work_item_name, b.norm_unit_price, b.final_unit_price,
			m.code AS material_code, m.name AS material_name,
			(qi.unit_price - COALESCE(b.norm_unit_price, b.expected_unit_price, 0)) AS variance_to_norm
		FROM supplier_quotation_items qi
		JOIN boq_items b ON b.id=qi.boq_item_id
		LEFT JOIN materials m ON m.id=qi.material_id
		WHERE qi.quotation_id=$1
		ORDER BY b.category, b.created_at, qi.created_at
	`, quotationID)
	if err != nil {
		return nil, err
	}
	quotation["items"] = items
	return quotation, nil
}

func (r *Repository) SaveSupplierQuotation(ctx context.Context, projectID, quotationID string, header Record, items []Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	isUpdate := quotationID != ""
	if quotationID == "" {
		rows, err := queryTx(ctx, tx, `
			INSERT INTO supplier_quotations (
				project_id, quotation_code, supplier_partner_id, supplier_name, quotation_date, valid_until, currency,
				payment_terms_days, lead_time_days, delivery_terms, warranty_terms, notes, status, created_by
			) VALUES ($1,$2,$3,$4,$5,NULLIF($6,'')::date,$7,$8,$9,$10,$11,$12,'draft',NULLIF($13,'')::uuid)
			RETURNING *
		`, projectID, header["quotation_code"], header["supplier_partner_id"], header["supplier_name"], header["quotation_date"], header["valid_until"], header["currency"], header["payment_terms_days"], header["lead_time_days"], header["delivery_terms"], header["warranty_terms"], header["notes"], actorID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		quotationID = cleanID(rows[0]["id"])
	} else {
		current, err := queryTx(ctx, tx, `SELECT * FROM supplier_quotations WHERE id=$1 AND project_id=$2 FOR UPDATE`, quotationID, projectID)
		if err != nil {
			return nil, err
		}
		if len(current) == 0 {
			return nil, sql.ErrNoRows
		}
		if fmt.Sprint(current[0]["status"]) != "draft" {
			return nil, validationError("Chỉ báo giá nháp mới được chỉnh sửa", Record{"quotation_id": quotationID, "status": current[0]["status"]})
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE supplier_quotations SET
				quotation_code=$3, supplier_partner_id=$4, supplier_name=$5, quotation_date=$6,
				valid_until=NULLIF($7,'')::date, currency=$8, payment_terms_days=$9, lead_time_days=$10,
				delivery_terms=$11, warranty_terms=$12, notes=$13, updated_at=now()
			WHERE id=$1 AND project_id=$2
		`, quotationID, projectID, header["quotation_code"], header["supplier_partner_id"], header["supplier_name"], header["quotation_date"], header["valid_until"], header["currency"], header["payment_terms_days"], header["lead_time_days"], header["delivery_terms"], header["warranty_terms"], header["notes"]); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM supplier_quotation_items WHERE quotation_id=$1`, quotationID); err != nil {
			return nil, err
		}
	}
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO supplier_quotation_items (
				quotation_id, boq_item_id, material_id, description, unit, quantity, unit_price, discount_amount,
				tax_rate, lead_time_days, compliance_status, deviation_note
			) VALUES ($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		`, quotationID, item["boq_item_id"], cleanID(item["material_id"]), item["description"], item["unit"], item["quantity"], item["unit_price"], item["discount_amount"], item["tax_rate"], item["lead_time_days"], item["compliance_status"], item["deviation_note"]); err != nil {
			return nil, err
		}
	}
	eventType := "supplier_quotation_created"
	if isUpdate {
		eventType = "supplier_quotation_updated"
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, eventType, "supplier_quotation", quotationID, "Lưu báo giá nhà cung cấp", Record{"quotation_code": header["quotation_code"], "item_count": len(items), "supplier_partner_id": header["supplier_partner_id"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetSupplierQuotationDetail(ctx, quotationID)
}

func (r *Repository) SupplierQuotationAction(ctx context.Context, quotationID, action, technicalStatus, commercialStatus, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM supplier_quotations WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, quotationID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	projectID := cleanID(current["project_id"])
	fromStatus := fmt.Sprint(current["status"])
	toStatus := ""
	eventType := ""
	switch action {
	case "submit":
		if fromStatus != "draft" {
			return nil, validationError("Chỉ báo giá nháp mới được gửi", Record{"status": fromStatus})
		}
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM supplier_quotation_items WHERE quotation_id=$1`, quotationID).Scan(&count); err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, validationError("Báo giá cần ít nhất một dòng trước khi gửi")
		}
		if validUntil := dateOnly(current["valid_until"]); validUntil != "" && validUntil < time.Now().Format("2006-01-02") {
			return nil, validationError("Báo giá đã hết hiệu lực", Record{"valid_until": validUntil})
		}
		toStatus = "submitted"
		eventType = "supplier_quotation_submitted"
		_, err = tx.ExecContext(ctx, `UPDATE supplier_quotations SET status=$2, submitted_by=NULLIF($3,'')::uuid, submitted_at=now(), updated_at=now() WHERE id=$1`, quotationID, toStatus, actorID)
	case "qualify":
		if fromStatus != "submitted" {
			return nil, validationError("Chỉ báo giá đã gửi mới được đánh giá", Record{"status": fromStatus})
		}
		var invalidItems int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM supplier_quotation_items WHERE quotation_id=$1 AND compliance_status IN ('pending','non_compliant')`, quotationID).Scan(&invalidItems); err != nil {
			return nil, err
		}
		qualified := statusIn(technicalStatus, "compliant", "conditional") && commercialStatus == "compliant" && invalidItems == 0
		if qualified {
			toStatus = "qualified"
		} else {
			toStatus = "rejected"
		}
		eventType = "supplier_quotation_qualified"
		_, err = tx.ExecContext(ctx, `
			UPDATE supplier_quotations SET status=$2, technical_status=$3, commercial_status=$4,
				qualified_by=NULLIF($5,'')::uuid, qualified_at=now(), notes=concat_ws(E'\n', NULLIF(notes,''), NULLIF($6,'')), updated_at=now()
			WHERE id=$1
		`, quotationID, toStatus, technicalStatus, commercialStatus, actorID, comment)
	default:
		return nil, validationError("Hành động báo giá không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, eventType, "supplier_quotation", quotationID, "Cập nhật trạng thái báo giá", Record{"from_status": fromStatus, "to_status": toStatus, "technical_status": technicalStatus, "commercial_status": commercialStatus, "comment": comment}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetSupplierQuotationDetail(ctx, quotationID)
}

func (r *Repository) SelectSupplierQuote(ctx context.Context, quotationItemID, reason, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT qi.*, q.project_id, q.status AS quotation_status, q.valid_until, q.supplier_partner_id, q.supplier_name,
			q.technical_status, q.commercial_status, b.status AS boq_status, b.norm_price_locked, b.final_price_locked,
			b.norm_unit_price, b.price_revision_no, b.work_item_name
		FROM supplier_quotation_items qi
		JOIN supplier_quotations q ON q.id=qi.quotation_id
		JOIN boq_items b ON b.id=qi.boq_item_id
		WHERE qi.id=$1
		FOR UPDATE OF qi, q, b
	`, quotationItemID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	item := rows[0]
	if !statusIn(fmt.Sprint(item["quotation_status"]), "qualified", "selected") || !statusIn(fmt.Sprint(item["technical_status"]), "compliant", "conditional") || fmt.Sprint(item["commercial_status"]) != "compliant" || !statusIn(fmt.Sprint(item["compliance_status"]), "compliant", "conditional") {
		return nil, validationError("Chỉ được chọn dòng báo giá đã đạt đánh giá kỹ thuật và thương mại")
	}
	if validUntil := dateOnly(item["valid_until"]); validUntil != "" && validUntil < time.Now().Format("2006-01-02") {
		return nil, validationError("Báo giá đã hết hiệu lực", Record{"valid_until": validUntil})
	}
	if fmt.Sprint(item["boq_status"]) != "approved" || !boolValue(item["norm_price_locked"]) || boolValue(item["final_price_locked"]) {
		return nil, validationError("Dòng BOQ phải được duyệt, khóa giá định mức và chưa khóa giá cuối")
	}
	var procurementCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM procurement_request_items pri
		JOIN procurement_requests pr ON pr.id=pri.request_id
		WHERE pri.boq_item_id=$1 AND pr.status <> 'cancelled'
	`, item["boq_item_id"]).Scan(&procurementCount); err != nil {
		return nil, err
	}
	if procurementCount > 0 {
		return nil, validationError("Dòng BOQ đã phát sinh yêu cầu mua, không thể đổi lựa chọn báo giá")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE supplier_quote_selections SET status='superseded', superseded_at=now() WHERE boq_item_id=$1 AND status='active'`, item["boq_item_id"]); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO supplier_quote_selections (project_id, boq_item_id, quotation_item_id, decision_reason, selected_by)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid)
	`, item["project_id"], item["boq_item_id"], quotationItemID, reason, actorID); err != nil {
		return nil, err
	}
	updatedRows, err := queryTx(ctx, tx, `
		UPDATE boq_items SET selected_quotation_item_id=$2, price_waiver_reason=NULL, final_unit_price=$3,
			final_price_locked=true, final_price_locked_by=NULLIF($4,'')::uuid, final_price_locked_at=now(),
			final_price_comment=$5, price_revision_no=price_revision_no+1, updated_at=now()
		WHERE id=$1 RETURNING *
	`, item["boq_item_id"], quotationItemID, item["unit_price"], actorID, reason)
	if err != nil {
		return nil, err
	}
	if len(updatedRows) == 0 {
		return nil, sql.ErrNoRows
	}
	updated := updatedRows[0]
	if err := insertBOQUnitPriceEventTx(ctx, tx, updated, "final_locked", "final", numberPtr(item["norm_unit_price"]), number(item["unit_price"]), reason, actorID, actorName); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE supplier_quotations SET status='selected', selected_at=COALESCE(selected_at,now()), updated_at=now() WHERE id=$1`, item["quotation_id"]); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(item["project_id"]), actorID, actorName, "supplier_quote_selected", "boq_item", cleanID(item["boq_item_id"]), "Chọn báo giá và khóa đơn giá cuối BOQ", Record{"quotation_item_id": quotationItemID, "quotation_id": item["quotation_id"], "supplier_partner_id": item["supplier_partner_id"], "unit_price": item["unit_price"], "decision_reason": reason}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetBOQItemDetail(ctx, cleanID(item["boq_item_id"]))
}

func (r *Repository) SupplierQuoteComparison(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT b.id AS boq_item_id, b.category, b.work_item_name, b.unit, b.quantity, b.status, b.expected_unit_price,
			b.norm_unit_price, b.final_unit_price, b.norm_price_locked, b.final_price_locked,
			b.selected_quotation_item_id, b.price_waiver_reason,
			COALESCE((
				SELECT jsonb_agg(jsonb_build_object(
					'quotation_item_id', qi.id, 'quotation_id', q.id, 'quotation_code', q.quotation_code,
					'supplier_partner_id', q.supplier_partner_id, 'supplier_name', q.supplier_name,
					'quotation_status', q.status, 'valid_until', q.valid_until, 'unit_price', qi.unit_price,
					'discount_amount', qi.discount_amount, 'tax_rate', qi.tax_rate, 'total_amount', qi.total_amount,
					'lead_time_days', qi.lead_time_days, 'compliance_status', qi.compliance_status,
					'variance_to_norm', qi.unit_price - COALESCE(b.norm_unit_price, b.expected_unit_price, 0),
					'is_selected', qi.id=b.selected_quotation_item_id
				) ORDER BY qi.unit_price, q.supplier_name)
				FROM supplier_quotation_items qi
				JOIN supplier_quotations q ON q.id=qi.quotation_id
				WHERE qi.boq_item_id=b.id AND q.status <> 'cancelled'
			), '[]'::jsonb) AS quotations
		FROM boq_items b
		WHERE b.project_id=$1 AND b.status <> 'deleted'
		ORDER BY b.category, b.created_at
	`, projectID)
}

func (r *Repository) GenerateProcurementFromBOQ(ctx context.Context, projectID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "boq_lifecycle:"+projectID); err != nil {
		return nil, err
	}

	blockers, err := queryTx(ctx, tx, `
		SELECT id, work_item_name, status, unit_price_source, expected_unit_price, norm_unit_price, final_unit_price,
			price_revision_no, selected_quotation_item_id, price_waiver_reason
		FROM boq_items b
		WHERE b.project_id=$1 AND b.status='approved'
		  AND (NOT b.final_price_locked OR (b.selected_quotation_item_id IS NULL AND NULLIF(b.price_waiver_reason, '') IS NULL))
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
		return nil, validationError("Chỉ sinh yêu cầu mua khi mọi dòng BOQ đã duyệt, khóa giá cuối và có báo giá được chọn hoặc lý do miễn báo giá", Record{"items": blockers})
	}

	rows, err := queryTx(ctx, tx, `
		SELECT b.*, m.name AS material_name, m.standard_price,
			COALESCE(q.supplier_name, m.default_supplier) AS selected_supplier_name,
			COALESCE(q.supplier_partner_id, m.default_supplier_partner_id) AS selected_supplier_partner_id,
			qi.id AS supplier_quotation_item_id, qi.unit_price AS quoted_unit_price,
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
		LEFT JOIN supplier_quotation_items qi ON qi.id=b.selected_quotation_item_id
		LEFT JOIN supplier_quotations q ON q.id=qi.quotation_id
		WHERE b.project_id=$1 AND b.status='approved' AND b.final_price_locked
		  AND (b.selected_quotation_item_id IS NOT NULL OR NULLIF(b.price_waiver_reason, '') IS NOT NULL)
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
		return nil, errors.New("Không có dòng BOQ đủ điều kiện để sinh yêu cầu mua")
	}
	groups := map[string][]Record{}
	groupOrder := []string{}
	for _, row := range rows {
		key := cleanID(row["selected_supplier_partner_id"])
		if key == "" {
			key = "unassigned"
		}
		if _, ok := groups[key]; !ok {
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], row)
	}
	requestIDs := make([]string, 0, len(groupOrder))
	for groupIndex, key := range groupOrder {
		groupRows := groups[key]
		total := 0.0
		for _, row := range groupRows {
			total += number(row["quantity"]) * number(row["effective_unit_price"])
		}
		supplierName := cleanOptionalText(groupRows[0]["selected_supplier_name"])
		title := "Yêu cầu mua sinh từ BOQ"
		if supplierName != "" {
			title += " - " + supplierName
		}
		requestCode := fmt.Sprintf("PR-%s-%d-%02d", projectID[:8], timeNowUnix(), groupIndex+1)
		reqRows, err := queryTx(ctx, tx, `
			INSERT INTO procurement_requests
				(project_id, request_code, title, requested_by, status, total_estimated_amount, budget_status, supplier_partner_id, supplier_name)
			VALUES ($1,$2,$3,NULLIF($4,'')::uuid,'waiting_approval',$5,'within_budget',NULLIF($6,'')::uuid,$7)
			RETURNING *
		`, projectID, requestCode, title, actorID, total, func() string {
			if key == "unassigned" {
				return ""
			}
			return key
		}(), supplierName)
		if err != nil {
			return nil, err
		}
		if len(reqRows) == 0 {
			return nil, sql.ErrNoRows
		}
		requestID := cleanID(reqRows[0]["id"])
		requestIDs = append(requestIDs, requestID)
		for _, row := range groupRows {
			price := number(row["effective_unit_price"])
			quotedPrice := row["quoted_unit_price"]
			var variance any
			if row["supplier_quotation_item_id"] != nil {
				variance = number(quotedPrice) - price
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO procurement_request_items
					(request_id, material_id, boq_item_id, quantity_planned, quantity_requested, unit, estimated_unit_price,
					 supplier_partner_id, supplier_name, delivery_status, approval_status, boq_price_source, boq_price_revision_no,
					 boq_price_event_id, supplier_quotation_item_id, quoted_unit_price, price_variance_amount)
				VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::uuid,$9,'pending','pending','final',$10,NULLIF($11,'')::uuid,NULLIF($12,'')::uuid,$13,$14)
			`, requestID, row["material_id"], row["id"], row["quantity"], row["quantity"], row["unit"], price,
				cleanID(row["selected_supplier_partner_id"]), row["selected_supplier_name"], row["price_revision_no"], cleanID(row["final_price_event_id"]), cleanID(row["supplier_quotation_item_id"]), quotedPrice, variance); err != nil {
				return nil, err
			}
		}
		if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "material_requested", "procurement_request", requestID, "Sinh yêu cầu mua từ BOQ theo nhà cung cấp", Record{"total": total, "boq_price_source": "final", "supplier_partner_id": key, "item_count": len(groupRows)}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	requests := make([]Record, 0, len(requestIDs))
	for _, requestID := range requestIDs {
		request, err := r.GetProcurementDetail(ctx, requestID)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	result := requests[0]
	result["requests"] = requests
	result["request_count"] = len(requests)
	return result, nil
}

func (r *Repository) GetProcurementDetail(ctx context.Context, requestID string) (Record, error) {
	req, err := r.GetByID(ctx, "procurement_requests", requestID)
	if err != nil {
		return nil, err
	}
	items, _ := r.Query(ctx, `
		SELECT pri.*, m.name AS material_name, b.work_item_name, b.price_waiver_reason,
			q.quotation_code, q.quotation_date, q.valid_until, q.currency,
			COALESCE(received.accepted_quantity,0) AS accepted_quantity,
			LEAST(COALESCE(received.accepted_quantity,0), COALESCE(received.stock_received_quantity,0)+COALESCE(legacy_stock.received_quantity,0)) AS stock_received_quantity,
			GREATEST(pri.quantity_requested - COALESCE(received.accepted_quantity,0),0) AS remaining_quantity,
			GREATEST(COALESCE(received.accepted_quantity,0)-COALESCE(received.stock_received_quantity,0)-COALESCE(legacy_stock.received_quantity,0),0) AS awaiting_stock_quantity
		FROM procurement_request_items pri
		LEFT JOIN materials m ON m.id=pri.material_id
		LEFT JOIN boq_items b ON b.id=pri.boq_item_id
		LEFT JOIN supplier_quotation_items qi ON qi.id=pri.supplier_quotation_item_id
		LEFT JOIN supplier_quotations q ON q.id=qi.quotation_id
		LEFT JOIN LATERAL (
			SELECT SUM(iai.accepted_quantity) AS accepted_quantity,
				SUM(COALESCE(stock.received_quantity,0)) AS stock_received_quantity
			FROM incoming_acceptance_items iai
			JOIN incoming_acceptance_records iar ON iar.id=iai.incoming_acceptance_id AND iar.status='submitted'
			LEFT JOIN LATERAL (
				SELECT SUM(sm.quantity) AS received_quantity
				FROM stock_movements sm
				WHERE sm.movement_type='receive'
				  AND sm.reference_type='incoming_acceptance_item'
				  AND sm.reference_id=iai.id
			) stock ON true
			WHERE iai.procurement_request_item_id=pri.id
		) received ON true
		LEFT JOIN LATERAL (
			SELECT SUM(sm.quantity) AS received_quantity
			FROM stock_movements sm
			WHERE sm.movement_type='receive'
			  AND sm.reference_type='boq_item'
			  AND sm.reference_id=pri.boq_item_id
		) legacy_stock ON true
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
	toStatus := ""
	event := ""
	if action == "submit" {
		var itemCount, incompleteCount int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*), COUNT(*) FILTER (
				WHERE pri.supplier_partner_id IS NULL
				   OR (pri.supplier_quotation_item_id IS NULL AND NULLIF(b.price_waiver_reason,'') IS NULL)
			)
			FROM procurement_request_items pri
			LEFT JOIN boq_items b ON b.id=pri.boq_item_id
			WHERE pri.request_id=$1
		`, id).Scan(&itemCount, &incompleteCount); err != nil {
			return nil, err
		}
		if itemCount == 0 || incompleteCount > 0 {
			return nil, validationError("Yêu cầu mua cần có dòng hàng, nhà cung cấp và nguồn giá đã duyệt trước khi gửi", Record{"item_count": itemCount, "incomplete_item_count": incompleteCount})
		}
	}
	if action == "order" {
		if cleanID(current["supplier_partner_id"]) == "" || strings.TrimSpace(fmt.Sprint(current["order_reference"])) == "" || dateOnly(current["expected_delivery_date"]) == "" {
			return nil, validationError("Đặt hàng cần nhà cung cấp, số đơn đặt hàng và ngày giao dự kiến")
		}
	}
	if action == "approve" && cleanID(current["approval_request_id"]) != "" {
		var approvalStatus string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM approval_requests WHERE id=$1 AND project_id=$2`, current["approval_request_id"], current["project_id"]).Scan(&approvalStatus); err != nil {
			return nil, err
		}
		if approvalStatus != "approved" {
			return nil, validationError("Yêu cầu phê duyệt cấu trúc chưa hoàn tất", Record{"approval_request_id": current["approval_request_id"], "approval_status": approvalStatus})
		}
	}
	if action == "receive" {
		if !statusIn(fromStatus, "ordered", "partially_received") {
			return nil, validationError("Chỉ yêu cầu mua đã đặt hàng mới được đối soát nhận hàng", Record{"status": fromStatus})
		}
		receipts, err := queryTx(ctx, tx, `
			SELECT pri.id, pri.quantity_requested,
				COALESCE(SUM(iai.accepted_quantity) FILTER (WHERE iar.id IS NOT NULL),0) AS accepted_quantity
			FROM procurement_request_items pri
			LEFT JOIN incoming_acceptance_items iai ON iai.procurement_request_item_id=pri.id
			LEFT JOIN incoming_acceptance_records iar ON iar.id=iai.incoming_acceptance_id AND iar.status='submitted'
			WHERE pri.request_id=$1
			GROUP BY pri.id, pri.quantity_requested
		`, id)
		if err != nil {
			return nil, err
		}
		acceptedAny := false
		allReceived := len(receipts) > 0
		for _, receipt := range receipts {
			accepted := number(receipt["accepted_quantity"])
			requested := number(receipt["quantity_requested"])
			acceptedAny = acceptedAny || accepted > 0
			allReceived = allReceived && accepted+0.000001 >= requested
			deliveryStatus := "pending"
			if accepted > 0 && accepted+0.000001 < requested {
				deliveryStatus = "partial"
			} else if accepted+0.000001 >= requested {
				deliveryStatus = "received"
			}
			if _, err := tx.ExecContext(ctx, `UPDATE procurement_request_items SET delivery_status=$2 WHERE id=$1`, receipt["id"], deliveryStatus); err != nil {
				return nil, err
			}
		}
		if !acceptedAny {
			return nil, validationError("Chưa có khối lượng đạt nghiệm thu đầu vào để ghi nhận nhận hàng")
		}
		toStatus = "partially_received"
		event = "procurement_acceptance_partially_reconciled"
		if allReceived {
			toStatus = "received"
			event = "procurement_acceptance_reconciled"
		}
	} else {
		toStatus, event, err = procurementTransitionForAction(action, fromStatus)
		if err != nil {
			return nil, err
		}
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE procurement_requests
		SET status=$2, ordered_at=CASE WHEN $4='order' THEN COALESCE(ordered_at,now()) ELSE ordered_at END, updated_at=now()
		WHERE id=$1 AND status=$3
		RETURNING *
	`, id, toStatus, fromStatus, action)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái yêu cầu mua đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
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
			return "received", "procurement_acceptance_reconciled", nil
		}
	default:
		return "", "", validationError("Hành động yêu cầu mua không hợp lệ", Record{"action": action})
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

func (r *Repository) ListProjectContracts(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT c.*, p.tax_code AS counterparty_tax_code,
			cv.version_no AS current_version_no, cv.version_label AS current_version_label, cv.status AS current_version_status,
			COALESCE(summary.milestone_count,0) AS milestone_count,
			COALESCE(summary.variation_count,0) AS variation_count,
			COALESCE(summary.retention_outstanding,0) AS retention_outstanding,
			COALESCE(summary.signed_scan_received,false) AS signed_scan_received
		FROM contracts c
		LEFT JOIN partners p ON p.id=c.counterparty_partner_id
		LEFT JOIN contract_versions cv ON cv.id=c.current_version_id
		LEFT JOIN LATERAL (
			SELECT
				(SELECT COUNT(*) FROM contract_milestones cm WHERE cm.contract_id=c.id AND cm.status <> 'cancelled') AS milestone_count,
				(SELECT COUNT(*) FROM contract_variations v WHERE v.contract_id=c.id AND v.status <> 'rejected') AS variation_count,
				(SELECT COALESCE(SUM(cr.outstanding_amount),0) FROM contract_retentions cr WHERE cr.contract_id=c.id AND cr.status <> 'forfeited') AS retention_outstanding,
				EXISTS (
					SELECT 1
					FROM contract_dispatches dispatch
					WHERE dispatch.contract_id=c.id
					  AND dispatch.contract_version_id=c.current_version_id
					  AND dispatch.status='received'
					  AND dispatch.returned_scan_document_id IS NOT NULL
				) AS signed_scan_received
		) summary ON true
		WHERE c.project_id=$1 AND c.status <> 'terminated'
		ORDER BY c.created_at DESC
	`, projectID)
}

func (r *Repository) GetContractDetail(ctx context.Context, contractID string) (Record, error) {
	contract, err := r.QueryOne(ctx, `
		SELECT c.*, p.tax_code AS counterparty_tax_code, p.partner_code AS counterparty_code,
			cv.version_no AS current_version_no, cv.version_label AS current_version_label, cv.status AS current_version_status
		FROM contracts c
		LEFT JOIN partners p ON p.id=c.counterparty_partner_id
		LEFT JOIN contract_versions cv ON cv.id=c.current_version_id
		WHERE c.id=$1 AND c.status <> 'terminated'
	`, contractID)
	if err != nil {
		return nil, err
	}
	versions, err := r.Query(ctx, `SELECT * FROM contract_versions WHERE contract_id=$1 ORDER BY version_no DESC`, contractID)
	if err != nil {
		return nil, err
	}
	signatures, err := r.Query(ctx, `
		SELECT link.id AS link_id,link.contract_version_id,link.purpose,link.linked_by,link.linked_at,
			signature.id AS workflow_form_signature_id,signature.form_instance_id,signature.signer_user_id,
			signature.signer_name,signature.signer_roles_json,signature.comment,signature.payload_hash,
			signature.signed_at,signature.visual_mime_type,signature.visual_file_size,
			wfi.title AS workflow_form_title
		FROM contract_version_signature_links link
		JOIN workflow_form_signatures signature ON signature.id=link.workflow_form_signature_id
		JOIN workflow_form_instances wfi ON wfi.id=signature.form_instance_id
		JOIN contract_versions version ON version.id=link.contract_version_id
		WHERE version.contract_id=$1
		ORDER BY signature.signed_at DESC,signature.id
	`, contractID)
	if err != nil {
		return nil, err
	}
	signingForms, err := r.Query(ctx, `
		SELECT wfi.id,wfi.title,wfi.status,wfi.current_revision,wfi.submitted_at,
			wi.id AS work_item_id,wi.title AS work_item_title,rs.business_code AS step_business_code,
			COALESCE((
				SELECT jsonb_agg(jsonb_build_object(
					'id',signature.id,
					'purpose',signature.purpose,
					'signer_user_id',signature.signer_user_id,
					'signer_name',signature.signer_name,
					'signed_at',signature.signed_at,
					'linked_contract_version_id',link.contract_version_id
				) ORDER BY signature.signed_at DESC)
				FROM workflow_form_signatures signature
				LEFT JOIN contract_version_signature_links link ON link.workflow_form_signature_id=signature.id
				WHERE signature.form_instance_id=wfi.id
			),'[]'::jsonb) AS signatures
		FROM contracts contract
		JOIN workflow_form_instances wfi ON wfi.project_id=contract.project_id AND wfi.status='submitted'
		JOIN work_items wi ON wi.id=wfi.work_item_id
		JOIN pipeline_run_steps rs ON rs.id=wfi.run_step_id
		WHERE contract.id=$1 AND rs.business_code='outbound_contract.app_sign'
		ORDER BY wfi.submitted_at DESC NULLS LAST,wfi.updated_at DESC,wfi.id
	`, contractID)
	if err != nil {
		return nil, err
	}
	dispatches, err := r.Query(ctx, `
		SELECT dispatch.*,
			sent.name AS sent_document_name,sent.mime_type AS sent_document_mime_type,
			returned.name AS returned_scan_document_name,returned.mime_type AS returned_scan_document_mime_type
		FROM contract_dispatches dispatch
		LEFT JOIN documents sent ON sent.id=dispatch.sent_document_id
		LEFT JOIN documents returned ON returned.id=dispatch.returned_scan_document_id
		WHERE dispatch.contract_id=$1
		ORDER BY dispatch.dispatch_no DESC
	`, contractID)
	if err != nil {
		return nil, err
	}
	milestones, err := r.Query(ctx, `
		SELECT cm.*, ar.title AS acceptance_title, ar.status AS acceptance_status
		FROM contract_milestones cm
		LEFT JOIN acceptance_records ar ON ar.id=cm.acceptance_id
		WHERE cm.contract_id=$1 AND cm.status <> 'cancelled'
		ORDER BY cm.sequence_no
	`, contractID)
	if err != nil {
		return nil, err
	}
	variations, err := r.Query(ctx, `
		SELECT variation.*,
			COALESCE((
				SELECT jsonb_agg(to_jsonb(event) ORDER BY event.created_at,event.id)
				FROM contract_variation_events event WHERE event.variation_id=variation.id
			),'[]'::jsonb) AS events
		FROM contract_variations variation
		WHERE variation.contract_id=$1
		ORDER BY variation.created_at DESC
	`, contractID)
	if err != nil {
		return nil, err
	}
	retentions, err := r.Query(ctx, `
		SELECT cr.*, cm.milestone_code, cm.milestone_name,
			COALESCE((SELECT jsonb_agg(to_jsonb(rr) ORDER BY rr.release_no) FROM contract_retention_releases rr WHERE rr.retention_id=cr.id),'[]'::jsonb) AS releases
		FROM contract_retentions cr
		JOIN contract_milestones cm ON cm.id=cr.source_milestone_id
		WHERE cr.contract_id=$1
		ORDER BY cr.created_at
	`, contractID)
	if err != nil {
		return nil, err
	}
	contract["versions"] = versions
	contract["workflow_signatures"] = signatures
	contract["signing_forms"] = signingForms
	contract["dispatches"] = dispatches
	currentVersionID := cleanID(contract["current_version_id"])
	appSigned := false
	for _, signature := range signatures {
		if cleanID(signature["contract_version_id"]) == currentVersionID {
			appSigned = true
			break
		}
	}
	paperDispatched := false
	returnedScanReceived := false
	for _, dispatch := range dispatches {
		if cleanID(dispatch["contract_version_id"]) != currentVersionID {
			continue
		}
		paperDispatched = true
		if fmt.Sprint(dispatch["status"]) == "received" && cleanID(dispatch["returned_scan_document_id"]) != "" {
			returnedScanReceived = true
		}
	}
	contract["execution_readiness"] = Record{
		"app_signed":             appSigned,
		"paper_dispatched":       paperDispatched,
		"returned_scan_received": returnedScanReceived,
		"contract_active":        fmt.Sprint(contract["status"]) == "active",
	}
	contract["milestones"] = milestones
	contract["variations"] = variations
	contract["retentions"] = retentions
	return contract, nil
}

func (r *Repository) CreateProjectDocument(ctx context.Context, projectID, targetType, targetID, documentType, name, originalName string, file StoredFileMetadata, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO documents (
			project_id,target_type,target_id,name,document_type,status,uploaded_by,
			mime_type,file_size,storage_provider,storage_key,checksum_sha256
		)
		VALUES ($1,$2,NULLIF($3,'')::uuid,$4,NULLIF($5,''),'active',NULLIF($6,'')::uuid,$7,$8,$9,$10,$11)
		RETURNING *
	`, projectID, targetType, targetID, name, documentType, actorID, file.MIMEType, file.FileSize, file.StorageProvider, file.StorageKey, file.ChecksumSHA256)
	if err != nil {
		return nil, err
	}
	documentID := cleanID(rows[0]["id"])
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "project_document_uploaded", "document", documentID, "Tải hồ sơ công trình lên kho bảo vệ", Record{
		"target_type": targetType, "target_id": targetID, "document_type": documentType,
		"name": name, "original_filename": originalName, "checksum_sha256": file.ChecksumSHA256,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) SaveContract(ctx context.Context, contractID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	projectID := cleanID(body["project_id"])
	eventType := "contract_created"
	if contractID == "" {
		rows, err := queryTx(ctx, tx, `
			INSERT INTO contracts (
				project_id, contract_code, title, contract_type, direction, counterparty_partner_id, counterparty_name,
				currency, original_amount, current_amount, advance_percent, retention_percent, warranty_days,
				signed_date, effective_date, expiry_date, status, created_by
			) VALUES ($1,$2,$3,$4,$5,NULLIF($6,'')::uuid,$7,$8,$9,$9,$10,$11,$12,NULLIF($13,'')::date,NULLIF($14,'')::date,NULLIF($15,'')::date,'draft',NULLIF($16,'')::uuid)
			RETURNING *
		`, projectID, body["contract_code"], body["title"], body["contract_type"], body["direction"], cleanID(body["counterparty_partner_id"]), body["counterparty_name"], body["currency"], body["original_amount"], body["advance_percent"], body["retention_percent"], body["warranty_days"], body["signed_date"], body["effective_date"], body["expiry_date"], actorID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		contractID = cleanID(rows[0]["id"])
	} else {
		currentRows, err := queryTx(ctx, tx, `SELECT * FROM contracts WHERE id=$1 AND project_id=$2 FOR UPDATE`, contractID, projectID)
		if err != nil {
			return nil, err
		}
		if len(currentRows) == 0 {
			return nil, sql.ErrNoRows
		}
		if !statusIn(fmt.Sprint(currentRows[0]["status"]), "draft", "review") {
			return nil, validationError("Chỉ hợp đồng nháp/review mới được sửa thông tin gốc", Record{"status": currentRows[0]["status"]})
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE contracts SET contract_code=$3,title=$4,contract_type=$5,direction=$6,
				counterparty_partner_id=NULLIF($7,'')::uuid,counterparty_name=$8,currency=$9,
				original_amount=$10,current_amount=CASE WHEN current_version_id IS NULL THEN $10 ELSE current_amount END,
				advance_percent=$11,retention_percent=$12,warranty_days=$13,signed_date=NULLIF($14,'')::date,
				effective_date=NULLIF($15,'')::date,expiry_date=NULLIF($16,'')::date,updated_at=now()
			WHERE id=$1 AND project_id=$2
		`, contractID, projectID, body["contract_code"], body["title"], body["contract_type"], body["direction"], cleanID(body["counterparty_partner_id"]), body["counterparty_name"], body["currency"], body["original_amount"], body["advance_percent"], body["retention_percent"], body["warranty_days"], body["signed_date"], body["effective_date"], body["expiry_date"]); err != nil {
			return nil, err
		}
		eventType = "contract_updated"
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, eventType, "contract", contractID, "Lưu hồ sơ hợp đồng", Record{"contract_code": body["contract_code"], "contract_type": body["contract_type"], "direction": body["direction"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, contractID)
}

func (r *Repository) CreateContractVersion(ctx context.Context, contractID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	contracts, err := queryTx(ctx, tx, `SELECT * FROM contracts WHERE id=$1 AND status <> 'terminated' FOR UPDATE`, contractID)
	if err != nil {
		return nil, err
	}
	if len(contracts) == 0 {
		return nil, sql.ErrNoRows
	}
	contract := contracts[0]
	if statusIn(fmt.Sprint(contract["status"]), "active", "completed", "suspended") {
		return nil, validationError("Hợp đồng đang hiệu lực chỉ tạo version mới thông qua variation đã duyệt")
	}
	var nextVersion int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_no),0)+1 FROM contract_versions WHERE contract_id=$1`, contractID).Scan(&nextVersion); err != nil {
		return nil, err
	}
	label := strings.TrimSpace(fmt.Sprint(body["version_label"]))
	if label == "" {
		label = fmt.Sprintf("V%d", nextVersion)
	}
	hashPayload, _ := json.Marshal(Record{"contract_id": contractID, "version_no": nextVersion, "amount": body["amount"], "scope_summary": body["scope_summary"], "commercial_terms": body["commercial_terms_json"], "payment_terms": body["payment_terms_json"], "planned_start_date": body["planned_start_date"], "planned_end_date": body["planned_end_date"], "document_id": body["document_id"]})
	hash := sha256.Sum256(hashPayload)
	rows, err := queryTx(ctx, tx, `
		INSERT INTO contract_versions (
			contract_id,version_no,version_label,amount,scope_summary,commercial_terms_json,payment_terms_json,
			planned_start_date,planned_end_date,document_id,definition_hash,status,review_comment,created_by
		) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,NULLIF($8,'')::date,NULLIF($9,'')::date,NULLIF($10,'')::uuid,$11,'draft',$12,NULLIF($13,'')::uuid)
		RETURNING *
	`, contractID, nextVersion, label, body["amount"], body["scope_summary"], mustJSON(body["commercial_terms_json"]), mustJSON(body["payment_terms_json"]), body["planned_start_date"], body["planned_end_date"], cleanID(body["document_id"]), hex.EncodeToString(hash[:]), body["comment"], actorID)
	if err != nil {
		return nil, err
	}
	versionID := cleanID(rows[0]["id"])
	if err := addAuditTx(ctx, tx, cleanID(contract["project_id"]), actorID, actorName, "contract_version_created", "contract_version", versionID, "Tạo phiên bản hợp đồng", Record{"contract_id": contractID, "version_no": nextVersion, "definition_hash": hex.EncodeToString(hash[:])}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, contractID)
}

func mustJSON(value any) string {
	raw, _ := json.Marshal(value)
	if string(raw) == "null" {
		return "{}"
	}
	return string(raw)
}

func (r *Repository) ContractVersionAction(ctx context.Context, versionID, action, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT cv.*, c.project_id, c.status AS contract_status, c.approval_request_id, c.contract_type
		FROM contract_versions cv JOIN contracts c ON c.id=cv.contract_id
		WHERE cv.id=$1 FOR UPDATE OF cv,c
	`, versionID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	version := rows[0]
	fromStatus := fmt.Sprint(version["status"])
	toStatus := ""
	eventType := ""
	switch action {
	case "submit":
		if fromStatus != "draft" {
			return nil, validationError("Chỉ version nháp mới được gửi review")
		}
		toStatus, eventType = "review", "contract_version_submitted"
	case "approve":
		if fromStatus != "review" {
			return nil, validationError("Chỉ version đang review mới được duyệt")
		}
		if cleanID(version["approval_request_id"]) != "" {
			var approvalStatus string
			if err := tx.QueryRowContext(ctx, `SELECT status FROM approval_requests WHERE id=$1`, version["approval_request_id"]).Scan(&approvalStatus); err != nil {
				return nil, err
			}
			if approvalStatus != "approved" {
				return nil, validationError("Approval cấu trúc của hợp đồng chưa hoàn tất", Record{"approval_status": approvalStatus})
			}
		}
		toStatus, eventType = "approved", "contract_version_approved"
	case "sign":
		if fromStatus != "approved" {
			return nil, validationError("Chỉ version đã duyệt mới được xác nhận ký")
		}
		toStatus, eventType = "signed", "contract_version_signed"
	default:
		return nil, validationError("Hành động version hợp đồng không hợp lệ", Record{"action": action})
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE contract_versions SET status=$2,review_comment=CASE WHEN $3 IN ('submit','approve') THEN concat_ws(E'\n',NULLIF(review_comment,''),NULLIF($4,'')) ELSE review_comment END,
			signature_comment=CASE WHEN $3='sign' THEN $4 ELSE signature_comment END,
			submitted_by=CASE WHEN $3='submit' THEN NULLIF($5,'')::uuid ELSE submitted_by END,submitted_at=CASE WHEN $3='submit' THEN now() ELSE submitted_at END,
			approved_by=CASE WHEN $3='approve' THEN NULLIF($5,'')::uuid ELSE approved_by END,approved_at=CASE WHEN $3='approve' THEN now() ELSE approved_at END,
			signed_by=CASE WHEN $3='sign' THEN NULLIF($5,'')::uuid ELSE signed_by END,signed_at=CASE WHEN $3='sign' THEN now() ELSE signed_at END,updated_at=now()
		WHERE id=$1
	`, versionID, toStatus, action, comment, actorID)
	if err != nil {
		return nil, err
	}
	contractStatus := map[string]string{"submit": "review", "approve": "approved", "sign": "signed"}[action]
	_, err = tx.ExecContext(ctx, `
		UPDATE contracts SET status=$2,
			current_version_id=CASE WHEN $3='sign' THEN $4::uuid ELSE current_version_id END,
			current_amount=CASE WHEN $3='sign' THEN $5 ELSE current_amount END,
			approved_by=CASE WHEN $3='approve' THEN NULLIF($6,'')::uuid ELSE approved_by END,
			signed_by=CASE WHEN $3='sign' THEN NULLIF($6,'')::uuid ELSE signed_by END,
			signed_date=CASE WHEN $3='sign' THEN COALESCE(signed_date,CURRENT_DATE) ELSE signed_date END,
			updated_at=now()
		WHERE id=$1
	`, version["contract_id"], contractStatus, action, versionID, version["amount"], actorID)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(version["project_id"]), actorID, actorName, eventType, "contract_version", versionID, fmt.Sprintf("Chuyển version hợp đồng từ %s sang %s", fromStatus, toStatus), Record{"contract_id": version["contract_id"], "version_no": version["version_no"], "comment": comment}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, cleanID(version["contract_id"]))
}

func (r *Repository) SignContractVersionWithWorkflowSignature(ctx context.Context, versionID, signatureID, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	versions, err := queryTx(ctx, tx, `
		SELECT cv.*,c.project_id,c.status AS contract_status
		FROM contract_versions cv
		JOIN contracts c ON c.id=cv.contract_id
		WHERE cv.id=$1
		FOR UPDATE OF cv,c
	`, versionID)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, sql.ErrNoRows
	}
	version := versions[0]
	if fmt.Sprint(version["status"]) != "approved" {
		return nil, validationError("Chỉ version đã duyệt mới được ký trên App", Record{"status": version["status"]})
	}
	signatures, err := queryTx(ctx, tx, `
		SELECT signature.*,wfi.project_id AS form_project_id,rs.business_code AS step_business_code
		FROM workflow_form_signatures signature
		JOIN workflow_form_instances wfi ON wfi.id=signature.form_instance_id
		JOIN pipeline_run_steps rs ON rs.id=wfi.run_step_id
		WHERE signature.id=$1
		FOR SHARE OF signature,wfi,rs
	`, signatureID)
	if err != nil {
		return nil, err
	}
	if len(signatures) == 0 {
		return nil, validationError("Không tìm thấy chữ ký workflow đã chọn")
	}
	signature := signatures[0]
	if cleanID(signature["form_project_id"]) != cleanID(version["project_id"]) {
		return nil, validationError("Chữ ký workflow không thuộc cùng công trình với hợp đồng")
	}
	if fmt.Sprint(signature["purpose"]) != "approved_by" || fmt.Sprint(signature["step_business_code"]) != "outbound_contract.app_sign" {
		return nil, validationError("Chữ ký không thuộc bước ký hợp đồng trên App", Record{"purpose": signature["purpose"], "step_business_code": signature["step_business_code"]})
	}
	if cleanID(signature["signer_user_id"]) != actorID {
		return nil, forbiddenError("Chỉ người vừa ký workflow mới được xác nhận ký version hợp đồng")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO contract_version_signature_links (contract_version_id,workflow_form_signature_id,purpose,linked_by)
		VALUES ($1,$2,'approved_by',NULLIF($3,'')::uuid)
	`, versionID, signatureID, actorID); err != nil {
		return nil, validationError("Chữ ký workflow đã được liên kết hoặc version đã có chữ ký", Record{"cause": err.Error()})
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE contract_versions
		SET status='signed',signature_comment=$2,signed_by=NULLIF($3,'')::uuid,signed_at=now(),updated_at=now()
		WHERE id=$1
	`, versionID, comment, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE contracts
		SET status='signed',current_version_id=$2,current_amount=$3,signed_by=NULLIF($4,'')::uuid,
			signed_date=COALESCE(signed_date,CURRENT_DATE),updated_at=now()
		WHERE id=$1
	`, version["contract_id"], versionID, version["amount"], actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(version["project_id"]), actorID, actorName, "contract_version_signed_on_app", "contract_version", versionID, "Ký version hợp đồng trên App", Record{
		"contract_id": version["contract_id"], "workflow_form_signature_id": signatureID, "signature_payload_hash": signature["payload_hash"], "comment": comment,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, cleanID(version["contract_id"]))
}

func (r *Repository) CreateContractDispatch(ctx context.Context, contractID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	contracts, err := queryTx(ctx, tx, `
		SELECT c.*,cv.status AS version_status,
			EXISTS (SELECT 1 FROM contract_version_signature_links link WHERE link.contract_version_id=cv.id) AS app_signed
		FROM contracts c
		JOIN contract_versions cv ON cv.id=c.current_version_id
		WHERE c.id=$1 FOR UPDATE OF c
	`, contractID)
	if err != nil {
		return nil, err
	}
	if len(contracts) == 0 {
		return nil, sql.ErrNoRows
	}
	contract := contracts[0]
	versionID := cleanID(body["contract_version_id"])
	if versionID == "" {
		versionID = cleanID(contract["current_version_id"])
	}
	if versionID != cleanID(contract["current_version_id"]) || fmt.Sprint(contract["version_status"]) != "signed" || !boolValue(contract["app_signed"]) {
		return nil, validationError("Chỉ version hiện hành đã ký trên App mới được gửi hồ sơ giấy")
	}
	if sentDocumentID := cleanID(body["sent_document_id"]); sentDocumentID != "" {
		if err := validateProjectDocumentTx(ctx, tx, sentDocumentID, cleanID(contract["project_id"])); err != nil {
			return nil, err
		}
	}
	var dispatchNo int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(dispatch_no),0)+1 FROM contract_dispatches WHERE contract_id=$1`, contractID).Scan(&dispatchNo); err != nil {
		return nil, err
	}
	rows, err := queryTx(ctx, tx, `
		INSERT INTO contract_dispatches (
			contract_id,contract_version_id,dispatch_no,recipient_name,delivery_method,
			tracking_reference,sent_document_id,status,sent_by,note
		)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,'')::uuid,'sent',NULLIF($8,'')::uuid,NULLIF($9,''))
		RETURNING *
	`, contractID, versionID, dispatchNo, body["recipient_name"], body["delivery_method"], body["tracking_reference"], cleanID(body["sent_document_id"]), actorID, body["note"])
	if err != nil {
		return nil, err
	}
	dispatchID := cleanID(rows[0]["id"])
	if err := addAuditTx(ctx, tx, cleanID(contract["project_id"]), actorID, actorName, "contract_paper_dispatched", "contract_dispatch", dispatchID, "Gửi hồ sơ hợp đồng giấy cho chủ đầu tư", Record{
		"contract_id": contractID, "contract_version_id": versionID, "dispatch_no": dispatchNo, "recipient_name": body["recipient_name"], "tracking_reference": body["tracking_reference"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, contractID)
}

func validateProjectDocumentTx(ctx context.Context, tx *sql.Tx, documentID, projectID string) error {
	var total int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE id=$1 AND project_id=$2 AND status='active'`, documentID, projectID).Scan(&total); err != nil {
		return err
	}
	if total == 0 {
		return validationError("Hồ sơ đính kèm không thuộc công trình hoặc không còn hiệu lực", Record{"document_id": documentID})
	}
	return nil
}

func (r *Repository) ReceiveContractDispatch(ctx context.Context, dispatchID, scanDocumentID, note, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT dispatch.*,contract.project_id
		FROM contract_dispatches dispatch
		JOIN contracts contract ON contract.id=dispatch.contract_id
		WHERE dispatch.id=$1 FOR UPDATE OF dispatch
	`, dispatchID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	dispatch := rows[0]
	if fmt.Sprint(dispatch["status"]) != "sent" {
		return nil, validationError("Hồ sơ giấy đã được ghi nhận nhận lại", Record{"status": dispatch["status"]})
	}
	if err := validateProjectDocumentTx(ctx, tx, scanDocumentID, cleanID(dispatch["project_id"])); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE contract_dispatches
		SET status='received',returned_scan_document_id=$2,received_by=NULLIF($3,'')::uuid,
			received_at=now(),note=concat_ws(E'\n',NULLIF(note,''),NULLIF($4,'')),updated_at=now()
		WHERE id=$1
	`, dispatchID, scanDocumentID, actorID, note); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(dispatch["project_id"]), actorID, actorName, "contract_signed_scan_received", "contract_dispatch", dispatchID, "Nhận lại và lưu bản scan hợp đồng giấy đã ký", Record{
		"contract_id": dispatch["contract_id"], "contract_version_id": dispatch["contract_version_id"], "returned_scan_document_id": scanDocumentID, "note": note,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, cleanID(dispatch["contract_id"]))
}

func (r *Repository) ActivateContract(ctx context.Context, contractID, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT c.*, cv.status AS version_status, cv.amount AS version_amount,
			EXISTS (
				SELECT 1 FROM contract_version_signature_links link
				WHERE link.contract_version_id=cv.id
			) AS app_signed,
			EXISTS (
				SELECT 1 FROM contract_dispatches dispatch
				WHERE dispatch.contract_id=c.id AND dispatch.contract_version_id=cv.id
				  AND dispatch.status='received' AND dispatch.returned_scan_document_id IS NOT NULL
			) AS returned_scan_received
		FROM contracts c LEFT JOIN contract_versions cv ON cv.id=c.current_version_id
		WHERE c.id=$1 FOR UPDATE OF c
	`, contractID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	contract := rows[0]
	if fmt.Sprint(contract["status"]) != "signed" || fmt.Sprint(contract["version_status"]) != "signed" {
		return nil, validationError("Chỉ hợp đồng có version đã ký mới được kích hoạt")
	}
	if !boolValue(contract["app_signed"]) {
		return nil, validationError("Cần chữ ký approved_by trên App trước khi kích hoạt hợp đồng")
	}
	if !boolValue(contract["returned_scan_received"]) {
		return nil, validationError("Cần nhận lại và lưu bản scan hợp đồng giấy đã ký trước khi kích hoạt")
	}
	if cleanID(contract["counterparty_partner_id"]) == "" || dateOnly(contract["effective_date"]) == "" {
		return nil, validationError("Kích hoạt hợp đồng cần đối tác catalog và ngày hiệu lực")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE contracts SET status='active',activated_by=NULLIF($2,'')::uuid,activated_at=now(),updated_at=now() WHERE id=$1`, contractID, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE contract_milestones SET status='active',updated_at=now() WHERE contract_id=$1 AND status='draft'`, contractID); err != nil {
		return nil, err
	}
	if fmt.Sprint(contract["contract_type"]) == "main_client" {
		if _, err := tx.ExecContext(ctx, `UPDATE projects SET contract_amount=$2,updated_at=now() WHERE id=$1`, contract["project_id"], contract["version_amount"]); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, cleanID(contract["project_id"]), actorID, actorName, "contract_activated", "contract", contractID, "Kích hoạt hợp đồng đã ký", Record{"current_version_id": contract["current_version_id"], "current_amount": contract["version_amount"], "comment": comment}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, contractID)
}

func (r *Repository) CreateContractMilestone(ctx context.Context, contractID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM contracts WHERE id=$1 AND status IN ('signed','active') FOR UPDATE`, contractID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, validationError("Cần ký hợp đồng trước khi lập milestone")
	}
	contract := rows[0]
	if cleanID(contract["current_version_id"]) == "" {
		return nil, validationError("Hợp đồng chưa có version hiện hành")
	}
	gross := number(body["gross_amount"])
	percentage := number(body["percentage"])
	if fmt.Sprint(body["amount_basis"]) == "percentage" {
		if percentage <= 0 {
			return nil, validationError("Milestone theo tỷ lệ cần phần trăm lớn hơn 0")
		}
		gross = math.Round(number(contract["current_amount"])*percentage) / 100
	}
	if gross <= 0 {
		return nil, validationError("Giá trị milestone phải lớn hơn 0")
	}
	retentionPercent := number(body["retention_percent"])
	retentionAmount := math.Round(gross*retentionPercent) / 100
	netAmount := gross - retentionAmount
	status := "draft"
	if fmt.Sprint(contract["status"]) == "active" {
		status = "active"
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO contract_milestones (
			contract_id,contract_version_id,milestone_code,milestone_name,sequence_no,milestone_type,amount_basis,
			percentage,gross_amount,retention_percent,retention_amount,net_amount,planned_date,due_days,
			acceptance_required,acceptance_id,status,created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,0),$9,$10,$11,$12,NULLIF($13,'')::date,$14,$15,NULLIF($16,'')::uuid,$17,NULLIF($18,'')::uuid)
	`, contractID, contract["current_version_id"], body["milestone_code"], body["milestone_name"], body["sequence_no"], body["milestone_type"], body["amount_basis"], percentage, gross, retentionPercent, retentionAmount, netAmount, body["planned_date"], body["due_days"], body["acceptance_required"], cleanID(body["acceptance_id"]), status, actorID)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(contract["project_id"]), actorID, actorName, "contract_milestone_created", "contract", contractID, "Tạo milestone hợp đồng", Record{"milestone_code": body["milestone_code"], "gross_amount": gross, "retention_amount": retentionAmount}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, contractID)
}

func (r *Repository) ContractMilestoneAction(ctx context.Context, milestoneID, action, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT cm.*,c.project_id,c.status AS contract_status,c.direction,c.contract_code,c.contract_type,
			c.counterparty_partner_id,c.counterparty_name,c.current_amount,c.warranty_days
		FROM contract_milestones cm JOIN contracts c ON c.id=cm.contract_id
		WHERE cm.id=$1 FOR UPDATE OF cm,c
	`, milestoneID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	milestone := rows[0]
	if fmt.Sprint(milestone["contract_status"]) != "active" {
		return nil, validationError("Milestone chỉ thao tác khi hợp đồng đang hiệu lực")
	}
	if action == "eligible" {
		if fmt.Sprint(milestone["status"]) != "active" {
			return nil, validationError("Chỉ milestone active mới được xác nhận đủ điều kiện")
		}
		if boolValue(milestone["acceptance_required"]) {
			if cleanID(milestone["acceptance_id"]) == "" {
				return nil, validationError("Milestone yêu cầu nghiệm thu nhưng chưa liên kết hồ sơ nghiệm thu")
			}
			var acceptanceStatus string
			var result any
			if err := tx.QueryRowContext(ctx, `SELECT status,result FROM acceptance_records WHERE id=$1 AND project_id=$2`, milestone["acceptance_id"], milestone["project_id"]).Scan(&acceptanceStatus, &result); err != nil {
				return nil, err
			}
			if acceptanceStatus != "approved" || !statusIn(fmt.Sprint(result), "pass", "conditional_pass") {
				return nil, validationError("Hồ sơ nghiệm thu chưa đạt điều kiện milestone", Record{"acceptance_status": acceptanceStatus, "result": result})
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE contract_milestones SET status='eligible',eligible_by=NULLIF($2,'')::uuid,eligible_at=now(),updated_at=now() WHERE id=$1`, milestoneID, actorID); err != nil {
			return nil, err
		}
		if err := addAuditTx(ctx, tx, cleanID(milestone["project_id"]), actorID, actorName, "contract_milestone_eligible", "contract_milestone", milestoneID, "Xác nhận milestone đủ điều kiện", Record{"contract_id": milestone["contract_id"], "comment": comment}); err != nil {
			return nil, err
		}
	} else if action == "open-obligation" {
		if fmt.Sprint(milestone["status"]) == "obligation_opened" && cleanID(milestone["obligation_id"]) != "" {
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			return r.GetContractDetail(ctx, cleanID(milestone["contract_id"]))
		}
		if fmt.Sprint(milestone["status"]) != "eligible" {
			return nil, validationError("Milestone phải đủ điều kiện trước khi mở công nợ")
		}
		dueDate := time.Now().AddDate(0, 0, int(number(milestone["due_days"]))).Format("2006-01-02")
		if planned := dateOnly(milestone["planned_date"]); planned != "" {
			base, _ := time.Parse("2006-01-02", planned)
			dueDate = base.AddDate(0, 0, int(number(milestone["due_days"]))).Format("2006-01-02")
		}
		obligationType := fmt.Sprint(milestone["direction"])
		obligationID := ""
		codeSuffix := strings.ToUpper(strings.ReplaceAll(fmt.Sprint(milestone["milestone_code"]), " ", "-"))
		if obligationType == "receivable" {
			obligationRows, err := queryTx(ctx, tx, `
				INSERT INTO receivables (project_id,receivable_code,client_partner_id,client_name,source_type,source_id,contract_amount,milestone_name,gross_amount,received_amount,remaining_amount,due_date,status,created_by)
				VALUES ($1,$2,NULLIF($3,'')::uuid,$4,'contract_milestone',$5,$6,$7,$8,0,$8,$9,'draft',NULLIF($10,'')::uuid)
				RETURNING id
			`, milestone["project_id"], fmt.Sprintf("AR-%s-%s", cleanID(milestone["contract_id"])[:8], codeSuffix), cleanID(milestone["counterparty_partner_id"]), milestone["counterparty_name"], milestoneID, milestone["current_amount"], milestone["milestone_name"], milestone["net_amount"], dueDate, actorID)
			if err != nil {
				return nil, err
			}
			obligationID = cleanID(obligationRows[0]["id"])
		} else {
			vendorType := fmt.Sprint(milestone["contract_type"])
			if vendorType == "main_client" {
				vendorType = "other"
			}
			obligationRows, err := queryTx(ctx, tx, `
				INSERT INTO payables (project_id,payable_code,vendor_type,vendor_partner_id,vendor_name,source_type,source_id,gross_amount,advance_amount,retention_amount,deduction_amount,paid_amount,remaining_amount,due_date,status,created_by)
				VALUES ($1,$2,$3,NULLIF($4,'')::uuid,$5,'contract_milestone',$6,$7,0,$8,0,0,$9,$10,'draft',NULLIF($11,'')::uuid)
				RETURNING id
			`, milestone["project_id"], fmt.Sprintf("AP-%s-%s", cleanID(milestone["contract_id"])[:8], codeSuffix), vendorType, cleanID(milestone["counterparty_partner_id"]), milestone["counterparty_name"], milestoneID, milestone["gross_amount"], milestone["retention_amount"], milestone["net_amount"], dueDate, actorID)
			if err != nil {
				return nil, err
			}
			obligationID = cleanID(obligationRows[0]["id"])
		}
		if _, err := tx.ExecContext(ctx, `UPDATE contract_milestones SET status='obligation_opened',obligation_type=$2,obligation_id=$3,updated_at=now() WHERE id=$1`, milestoneID, obligationType, obligationID); err != nil {
			return nil, err
		}
		if number(milestone["retention_amount"]) > 0 {
			warrantyEnd := ""
			if dateOnly(milestone["planned_date"]) != "" {
				base, _ := time.Parse("2006-01-02", dateOnly(milestone["planned_date"]))
				warrantyEnd = base.AddDate(0, 0, int(number(milestone["warranty_days"]))).Format("2006-01-02")
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO contract_retentions (contract_id,source_milestone_id,direction,held_amount,warranty_end_date,status)
				VALUES ($1,$2,$3,$4,NULLIF($5,'')::date,'held') ON CONFLICT (source_milestone_id) DO NOTHING
			`, milestone["contract_id"], milestoneID, obligationType, milestone["retention_amount"], warrantyEnd); err != nil {
				return nil, err
			}
		}
		if err := addAuditTx(ctx, tx, cleanID(milestone["project_id"]), actorID, actorName, "contract_milestone_obligation_opened", "contract_milestone", milestoneID, "Mở công nợ từ milestone hợp đồng", Record{"contract_id": milestone["contract_id"], "obligation_type": obligationType, "obligation_id": obligationID, "net_amount": milestone["net_amount"], "retention_amount": milestone["retention_amount"], "comment": comment}); err != nil {
			return nil, err
		}
	} else {
		return nil, validationError("Hành động milestone không hợp lệ", Record{"action": action})
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, cleanID(milestone["contract_id"]))
}

func (r *Repository) CreateContractVariation(ctx context.Context, contractID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM contracts WHERE id=$1 AND status IN ('active','suspended') FOR UPDATE`, contractID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, validationError("Chỉ hợp đồng đang hiệu lực/tạm dừng mới được lập variation")
	}
	contract := rows[0]
	if sourceWorkItemID := cleanID(body["source_work_item_id"]); sourceWorkItemID != "" {
		var validSource bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM work_items item
				LEFT JOIN pipeline_run_steps step ON step.id=item.run_step_id
				LEFT JOIN pipeline_run_nodes node ON node.id=item.run_node_id
				WHERE item.id=$1 AND item.project_id=$2
				  AND (step.business_code='construction.item_reporting' OR node.business_code='construction_execution')
			)
		`, sourceWorkItemID, contract["project_id"]).Scan(&validSource); err != nil {
			return nil, err
		}
		if !validSource {
			return nil, validationError("Đầu việc nguồn không thuộc bước thi công của công trình")
		}
	}
	variationRows, err := queryTx(ctx, tx, `
		INSERT INTO contract_variations (
			contract_id,variation_code,title,variation_type,reason,amount_delta,days_delta,scope_delta,
			document_id,status,priority,source_work_item_id,created_by
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,'')::uuid,'draft',$10,NULLIF($11,'')::uuid,NULLIF($12,'')::uuid)
		RETURNING *
	`, contractID, body["variation_code"], body["title"], body["variation_type"], body["reason"], body["amount_delta"], body["days_delta"], body["scope_delta"], cleanID(body["document_id"]), body["priority"], cleanID(body["source_work_item_id"]), actorID)
	if err != nil {
		return nil, err
	}
	if err := r.recordContractVariationEventTx(ctx, tx, variationRows[0], "contract_variation_created", "", "draft", body["comment"], []string{"QS"}, actorID, actorName); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(contract["project_id"]), actorID, actorName, "contract_variation_created", "contract", contractID, "Tạo phụ lục/variation", Record{"variation_code": body["variation_code"], "amount_delta": body["amount_delta"], "days_delta": body["days_delta"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, contractID)
}

func (r *Repository) ContractVariationAction(ctx context.Context, variationID, action, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT v.*,c.project_id,c.current_version_id,c.current_amount,c.status AS contract_status,
			cv.scope_summary,cv.commercial_terms_json,cv.payment_terms_json,cv.planned_start_date,cv.planned_end_date,cv.document_id
		FROM contract_variations v JOIN contracts c ON c.id=v.contract_id
		LEFT JOIN contract_versions cv ON cv.id=c.current_version_id
		WHERE v.id=$1 FOR UPDATE OF v,c
	`, variationID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	variation := rows[0]
	fromStatus := fmt.Sprint(variation["status"])
	toStatus := ""
	eventType := ""
	switch action {
	case "submit":
		if fromStatus != "draft" {
			return nil, validationError("Chỉ phát sinh nháp mới được gửi QS rà soát")
		}
		toStatus, eventType = "qs_review", "contract_variation_submitted"
		_, err = tx.ExecContext(ctx, `UPDATE contract_variations SET status='qs_review',submitted_by=NULLIF($2,'')::uuid,submitted_at=now(),updated_at=now() WHERE id=$1`, variationID, actorID)
	case "qs-review":
		if fromStatus != "qs_review" {
			return nil, validationError("Phát sinh chưa ở bước QS rà soát")
		}
		toStatus, eventType = "purchasing_sourcing", "contract_variation_qs_reviewed"
		_, err = tx.ExecContext(ctx, `UPDATE contract_variations SET status='purchasing_sourcing',qs_reviewed_by=NULLIF($2,'')::uuid,qs_reviewed_at=now(),qs_comment=$3,updated_at=now() WHERE id=$1`, variationID, actorID, comment)
	case "purchase":
		if fromStatus != "purchasing_sourcing" {
			return nil, validationError("Phát sinh chưa ở bước Mua hàng lấy giá")
		}
		toStatus, eventType = "pending_approval", "contract_variation_purchasing_completed"
		_, err = tx.ExecContext(ctx, `UPDATE contract_variations SET status='pending_approval',purchasing_reviewed_by=NULLIF($2,'')::uuid,purchasing_reviewed_at=now(),purchasing_comment=$3,updated_at=now() WHERE id=$1`, variationID, actorID, comment)
	case "approve":
		if !statusIn(fromStatus, "pending_approval", "review") {
			return nil, validationError("Phát sinh phải qua QS và Mua hàng trước khi duyệt")
		}
		if cleanID(variation["approval_request_id"]) != "" {
			var status string
			if err := tx.QueryRowContext(ctx, `SELECT status FROM approval_requests WHERE id=$1`, variation["approval_request_id"]).Scan(&status); err != nil {
				return nil, err
			}
			if status != "approved" {
				return nil, validationError("Approval cấu trúc của variation chưa hoàn tất", Record{"approval_status": status})
			}
		}
		toStatus, eventType = "approved", "contract_variation_approved"
		_, err = tx.ExecContext(ctx, `UPDATE contract_variations SET status='approved',approved_by=NULLIF($2,'')::uuid,approved_at=now(),updated_at=now() WHERE id=$1`, variationID, actorID)
	case "reject":
		if !statusIn(fromStatus, "qs_review", "purchasing_sourcing", "pending_approval", "review") {
			return nil, validationError("Trạng thái phát sinh không cho phép từ chối")
		}
		toStatus, eventType = "rejected", "contract_variation_rejected"
		_, err = tx.ExecContext(ctx, `UPDATE contract_variations SET status='rejected',rejected_by=NULLIF($2,'')::uuid,rejected_at=now(),updated_at=now() WHERE id=$1`, variationID, actorID)
	case "incorporate":
		if !statusIn(fromStatus, "approved", "procurement_created") || cleanID(variation["current_version_id"]) == "" {
			return nil, validationError("Chỉ variation đã duyệt của hợp đồng có version hiện hành mới được tạo revision")
		}
		var nextVersion int
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_no),0)+1 FROM contract_versions WHERE contract_id=$1`, variation["contract_id"]).Scan(&nextVersion); err != nil {
			return nil, err
		}
		newAmount := number(variation["current_amount"]) + number(variation["amount_delta"])
		if newAmount < 0 {
			return nil, validationError("Variation làm giá trị hợp đồng âm")
		}
		endDate := dateOnly(variation["planned_end_date"])
		if endDate != "" && int(number(variation["days_delta"])) != 0 {
			parsed, _ := time.Parse("2006-01-02", endDate)
			endDate = parsed.AddDate(0, 0, int(number(variation["days_delta"]))).Format("2006-01-02")
		}
		scopeSummary := strings.TrimSpace(fmt.Sprint(variation["scope_summary"]))
		if delta := strings.TrimSpace(fmt.Sprint(variation["scope_delta"])); delta != "" {
			scopeSummary = strings.TrimSpace(scopeSummary + "\nPhụ lục " + fmt.Sprint(variation["variation_code"]) + ": " + delta)
		}
		hashPayload, _ := json.Marshal(Record{"contract_id": variation["contract_id"], "version_no": nextVersion, "source_variation_id": variationID, "amount": newAmount, "scope_summary": scopeSummary, "commercial_terms": variation["commercial_terms_json"], "payment_terms": variation["payment_terms_json"], "planned_start_date": variation["planned_start_date"], "planned_end_date": endDate})
		hash := sha256.Sum256(hashPayload)
		versionRows, err := queryTx(ctx, tx, `
			INSERT INTO contract_versions (contract_id,version_no,version_label,source_variation_id,amount,scope_summary,commercial_terms_json,payment_terms_json,planned_start_date,planned_end_date,document_id,definition_hash,status,review_comment,created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,'')::date,NULLIF($10,'')::date,NULLIF($11,'')::uuid,$12,'draft',$13,NULLIF($14,'')::uuid)
			RETURNING id
		`, variation["contract_id"], nextVersion, fmt.Sprintf("V%d", nextVersion), variationID, newAmount, scopeSummary, variation["commercial_terms_json"], variation["payment_terms_json"], dateOnly(variation["planned_start_date"]), endDate, cleanID(variation["document_id"]), hex.EncodeToString(hash[:]), comment, actorID)
		if err != nil {
			return nil, err
		}
		toStatus, eventType = "incorporated", "contract_variation_incorporated"
		_, err = tx.ExecContext(ctx, `UPDATE contract_variations SET status='incorporated',resulting_version_id=$2,incorporated_by=NULLIF($3,'')::uuid,incorporated_at=now(),updated_at=now() WHERE id=$1`, variationID, versionRows[0]["id"], actorID)
	default:
		return nil, validationError("Hành động variation không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(variation["project_id"]), actorID, actorName, eventType, "contract_variation", variationID, fmt.Sprintf("Chuyển variation từ %s sang %s", fromStatus, toStatus), Record{"contract_id": variation["contract_id"], "amount_delta": variation["amount_delta"], "days_delta": variation["days_delta"], "comment": comment}); err != nil {
		return nil, err
	}
	nextRoles := map[string][]string{
		"qs_review":           {"QS"},
		"purchasing_sourcing": {"Mua hàng"},
		"pending_approval":    {"PM", "Giám đốc"},
	}[toStatus]
	variation["status"] = toStatus
	if err := r.recordContractVariationEventTx(ctx, tx, variation, eventType, fromStatus, toStatus, comment, nextRoles, actorID, actorName); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, cleanID(variation["contract_id"]))
}

func (r *Repository) recordContractVariationEventTx(ctx context.Context, tx *sql.Tx, variation Record, eventType, fromStatus, toStatus string, comment any, nextRoles []string, actorID, actorName string) error {
	projectID := cleanID(variation["project_id"])
	if projectID == "" {
		var err error
		projectID, err = projectIDForContractTx(ctx, tx, cleanID(variation["contract_id"]))
		if err != nil {
			return err
		}
	}
	metadata := Record{"contract_id": variation["contract_id"], "priority": variation["priority"], "next_roles": nextRoles}
	rawMetadata, _ := json.Marshal(metadata)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO contract_variation_events (
			variation_id,project_id,event_type,from_status,to_status,actor_id,actor_name,comment,metadata_json
		)
		VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),NULLIF($6,'')::uuid,NULLIF($7,''),NULLIF($8,''),$9::jsonb)
	`, variation["id"], projectID, eventType, fromStatus, toStatus, actorID, actorName, strings.TrimSpace(fmt.Sprint(comment)), string(rawMetadata)); err != nil {
		return err
	}
	workItemID := cleanID(variation["source_work_item_id"])
	if workItemID == "" || len(nextRoles) == 0 {
		return nil
	}
	activityRows, err := queryTx(ctx, tx, `
		INSERT INTO work_item_activities (
			work_item_id,project_id,scope_type,scope_id,actor_id,activity_type,comment,visibility,metadata_json
		)
		VALUES ($1,$2,'work_item',$1,NULLIF($3,'')::uuid,'system_event',$4,'project',$5::jsonb)
		RETURNING *
	`, workItemID, projectID, actorID, fmt.Sprintf("Phát sinh %s chuyển sang %s", variation["variation_code"], toStatus), string(rawMetadata))
	if err != nil {
		return err
	}
	mentions := make([]ActivityMentionRequest, 0, len(nextRoles))
	for _, role := range nextRoles {
		mentions = append(mentions, ActivityMentionRequest{Type: "role", ID: role, Label: role})
	}
	return r.insertActivityAttentionTx(ctx, tx, activityRows[0], WorkItemActivityRequest{Mentions: mentions}, actorID)
}

func projectIDForContractTx(ctx context.Context, tx *sql.Tx, contractID string) (string, error) {
	var projectID string
	if err := tx.QueryRowContext(ctx, `SELECT project_id FROM contracts WHERE id=$1`, contractID).Scan(&projectID); err != nil {
		return "", err
	}
	return projectID, nil
}

func (r *Repository) GenerateProcurementFromContractVariation(ctx context.Context, variationID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT variation.*,contract.project_id
		FROM contract_variations variation
		JOIN contracts contract ON contract.id=variation.contract_id
		WHERE variation.id=$1 FOR UPDATE OF variation
	`, variationID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	variation := rows[0]
	if cleanID(variation["procurement_request_id"]) != "" {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return r.GetContractDetail(ctx, cleanID(variation["contract_id"]))
	}
	if fmt.Sprint(variation["status"]) != "approved" {
		return nil, validationError("Chỉ phát sinh đã duyệt mới được tạo yêu cầu mua")
	}
	requestCode := fmt.Sprintf("VAR-%s-%d", strings.ToUpper(strings.ReplaceAll(fmt.Sprint(variation["variation_code"]), " ", "-")), time.Now().Unix())
	amount := math.Max(number(variation["amount_delta"]), 0)
	procurementRows, err := queryTx(ctx, tx, `
		INSERT INTO procurement_requests (project_id,request_code,title,requested_by,status,total_estimated_amount,budget_status)
		VALUES ($1,$2,$3,NULLIF($4,'')::uuid,'draft',$5,'within_budget')
		RETURNING id
	`, variation["project_id"], requestCode, fmt.Sprintf("Mua hàng cho phát sinh %s - %s", variation["variation_code"], variation["title"]), actorID, amount)
	if err != nil {
		return nil, err
	}
	procurementID := cleanID(procurementRows[0]["id"])
	if _, err := tx.ExecContext(ctx, `UPDATE contract_variations SET status='procurement_created',procurement_request_id=$2,updated_at=now() WHERE id=$1`, variationID, procurementID); err != nil {
		return nil, err
	}
	variation["status"] = "procurement_created"
	if err := r.recordContractVariationEventTx(ctx, tx, variation, "contract_variation_procurement_created", "approved", "procurement_created", "Tạo yêu cầu mua từ phát sinh", []string{"Mua hàng"}, actorID, actorName); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(variation["project_id"]), actorID, actorName, "contract_variation_procurement_created", "contract_variation", variationID, "Tạo yêu cầu mua từ phát sinh đã duyệt", Record{"procurement_request_id": procurementID, "request_code": requestCode}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, cleanID(variation["contract_id"]))
}

func (r *Repository) ReleaseContractRetention(ctx context.Context, retentionID string, amount float64, dueDate, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT cr.*,c.project_id,c.status AS contract_status,c.counterparty_partner_id,c.counterparty_name,c.contract_type
		FROM contract_retentions cr JOIN contracts c ON c.id=cr.contract_id
		WHERE cr.id=$1 FOR UPDATE OF cr,c
	`, retentionID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	retention := rows[0]
	if !statusIn(fmt.Sprint(retention["contract_status"]), "active", "completed") || fmt.Sprint(retention["status"]) == "forfeited" {
		return nil, validationError("Khoản giữ lại không ở trạng thái cho phép giải phóng")
	}
	if warrantyEnd := dateOnly(retention["warranty_end_date"]); warrantyEnd != "" && warrantyEnd > time.Now().Format("2006-01-02") {
		return nil, validationError("Chưa hết thời hạn bảo hành để giải phóng giữ lại", Record{"warranty_end_date": warrantyEnd})
	}
	if amount > number(retention["outstanding_amount"])+0.000001 {
		return nil, validationError("Số tiền giải phóng vượt số dư giữ lại", Record{"outstanding_amount": retention["outstanding_amount"]})
	}
	if dueDate == "" {
		dueDate = time.Now().Format("2006-01-02")
	}
	var releaseNo int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(release_no),0)+1 FROM contract_retention_releases WHERE retention_id=$1`, retentionID).Scan(&releaseNo); err != nil {
		return nil, err
	}
	obligationType := fmt.Sprint(retention["direction"])
	obligationID := ""
	if obligationType == "receivable" {
		obligationRows, err := queryTx(ctx, tx, `
			INSERT INTO receivables (project_id,receivable_code,client_partner_id,client_name,source_type,source_id,contract_amount,milestone_name,gross_amount,received_amount,remaining_amount,due_date,status,created_by)
			VALUES ($1,$2,NULLIF($3,'')::uuid,$4,'contract_retention',$5,$6,'Giải phóng giữ lại',$7,0,$7,$8,'draft',NULLIF($9,'')::uuid) RETURNING id
		`, retention["project_id"], fmt.Sprintf("AR-RET-%s-%02d", retentionID[:8], releaseNo), cleanID(retention["counterparty_partner_id"]), retention["counterparty_name"], retentionID, retention["held_amount"], amount, dueDate, actorID)
		if err != nil {
			return nil, err
		}
		obligationID = cleanID(obligationRows[0]["id"])
	} else {
		vendorType := fmt.Sprint(retention["contract_type"])
		if vendorType == "service" {
			vendorType = "service_provider"
		}
		obligationRows, err := queryTx(ctx, tx, `
			INSERT INTO payables (project_id,payable_code,vendor_type,vendor_partner_id,vendor_name,source_type,source_id,gross_amount,advance_amount,retention_amount,deduction_amount,paid_amount,remaining_amount,due_date,status,created_by)
			VALUES ($1,$2,$3,NULLIF($4,'')::uuid,$5,'contract_retention',$6,$7,0,0,0,0,$7,$8,'draft',NULLIF($9,'')::uuid) RETURNING id
		`, retention["project_id"], fmt.Sprintf("AP-RET-%s-%02d", retentionID[:8], releaseNo), vendorType, cleanID(retention["counterparty_partner_id"]), retention["counterparty_name"], retentionID, amount, dueDate, actorID)
		if err != nil {
			return nil, err
		}
		obligationID = cleanID(obligationRows[0]["id"])
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO contract_retention_releases (retention_id,release_no,amount,due_date,comment,obligation_type,obligation_id,released_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::uuid)
	`, retentionID, releaseNo, amount, dueDate, comment, obligationType, obligationID, actorID); err != nil {
		return nil, err
	}
	newReleased := number(retention["released_amount"]) + amount
	newStatus := "partially_released"
	if newReleased+0.000001 >= number(retention["held_amount"]) {
		newStatus = "released"
	}
	if _, err := tx.ExecContext(ctx, `UPDATE contract_retentions SET released_amount=$2,status=$3,release_obligation_type=$4,release_obligation_id=$5,released_by=NULLIF($6,'')::uuid,released_at=now(),release_comment=$7,updated_at=now() WHERE id=$1`, retentionID, newReleased, newStatus, obligationType, obligationID, actorID, comment); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(retention["project_id"]), actorID, actorName, "contract_retention_released", "contract_retention", retentionID, "Giải phóng khoản giữ lại", Record{"contract_id": retention["contract_id"], "amount": amount, "released_amount": newReleased, "outstanding_amount": number(retention["held_amount"]) - newReleased, "obligation_type": obligationType, "obligation_id": obligationID, "comment": comment}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, cleanID(retention["contract_id"]))
}

func (r *Repository) ProjectConstructionWorkspace(ctx context.Context, projectID string) (Record, error) {
	templates, err := r.Query(ctx, `
		SELECT t.*,u.full_name AS created_by_name
		FROM construction_check_templates t
		LEFT JOIN users u ON u.id=t.created_by
		WHERE t.project_id=$1
		ORDER BY CASE WHEN t.status='active' THEN 0 ELSE 1 END,t.updated_at DESC,t.name
	`, projectID)
	if err != nil {
		return nil, err
	}
	checks, err := r.Query(ctx, `
		SELECT c.*,t.name AS template_name,w.title AS work_item_title,
			e.name AS evidence_name,e.evidence_type,
			rq.full_name AS requested_by_name,ck.full_name AS checked_by_name,
			dr.report_date AS source_report_date,dr.weather AS source_report_weather,
			hi.handoff_id,hi.source_field_key AS handoff_source_field_key,hi.source_item_index AS handoff_source_item_index,
			h.title AS handoff_title,h.revision_no AS handoff_revision_no,h.source_run_node_id,h.target_run_node_id,
			source_node.name AS handoff_source_node_name
		FROM construction_checks c
		LEFT JOIN construction_check_templates t ON t.id=c.template_id
		LEFT JOIN work_items w ON w.id=c.work_item_id
		LEFT JOIN evidence_records e ON e.id=c.evidence_id
		LEFT JOIN users rq ON rq.id=c.requested_by
		LEFT JOIN users ck ON ck.id=c.checked_by
		LEFT JOIN operation_daily_reports dr ON dr.id=c.source_daily_report_id
		LEFT JOIN construction_handoff_items hi ON hi.id=c.source_handoff_item_id
		LEFT JOIN construction_handoffs h ON h.id=hi.handoff_id
		LEFT JOIN pipeline_run_nodes source_node ON source_node.id=h.source_run_node_id
		WHERE c.project_id=$1 AND c.status <> 'cancelled'
		ORDER BY CASE WHEN c.status='pending' THEN 0 ELSE 1 END,c.scheduled_at NULLS LAST,c.created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	evidenceRows, err := r.Query(ctx, `
		SELECT e.*,ce.check_id,ce.evidence_role,ce.sort_order
		FROM construction_check_evidence ce
		JOIN construction_checks c ON c.id=ce.check_id
		JOIN evidence_records e ON e.id=ce.evidence_id
		WHERE c.project_id=$1 AND c.status <> 'cancelled'
		ORDER BY ce.check_id,ce.evidence_role,ce.sort_order,ce.created_at,ce.id
	`, projectID)
	if err != nil {
		return nil, err
	}
	contextEvidenceByCheck := map[string][]Record{}
	resultEvidenceByCheck := map[string][]Record{}
	for _, evidence := range evidenceRows {
		checkID := cleanID(evidence["check_id"])
		if fmt.Sprint(evidence["evidence_role"]) == "context" {
			contextEvidenceByCheck[checkID] = append(contextEvidenceByCheck[checkID], evidence)
		} else {
			resultEvidenceByCheck[checkID] = append(resultEvidenceByCheck[checkID], evidence)
		}
	}
	for _, check := range checks {
		checkID := cleanID(check["id"])
		contextEvidence := contextEvidenceByCheck[checkID]
		if contextEvidence == nil {
			contextEvidence = []Record{}
		}
		resultEvidence := resultEvidenceByCheck[checkID]
		if resultEvidence == nil {
			resultEvidence = []Record{}
		}
		check["context_evidence"] = contextEvidence
		check["result_evidence"] = resultEvidence
	}
	actions, err := r.Query(ctx, `
		SELECT a.*,w.title AS work_item_title,c.title AS source_check_title,
			u.full_name AS assigned_user_name,b.name AS before_evidence_name,re.name AS resolution_evidence_name,
			rp.status AS impact_status,
			COALESCE(a.source_daily_report_id,c.source_daily_report_id) AS source_daily_report_id,
			CASE WHEN a.source_daily_report_id IS NOT NULL THEN 'direct' WHEN c.source_daily_report_id IS NOT NULL THEN 'check' ELSE NULL END AS source_report_link_kind,
			dr.report_date AS source_report_date,dr.weather AS source_report_weather
		FROM construction_action_items a
		LEFT JOIN work_items w ON w.id=a.work_item_id
		LEFT JOIN construction_checks c ON c.id=a.source_check_id
		LEFT JOIN users u ON u.id=a.assigned_user_id
		LEFT JOIN evidence_records b ON b.id=a.before_evidence_id
		LEFT JOIN evidence_records re ON re.id=a.resolution_evidence_id
		LEFT JOIN replanning_requests rp ON rp.id=a.technical_replanning_id
		LEFT JOIN operation_daily_reports dr ON dr.id=COALESCE(a.source_daily_report_id,c.source_daily_report_id)
		WHERE a.project_id=$1 AND a.status <> 'cancelled'
		ORDER BY CASE a.status WHEN 'open' THEN 0 WHEN 'waiting_confirmation' THEN 1 ELSE 2 END,
			a.due_at NULLS LAST,a.created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	for _, action := range actions {
		events, err := r.Query(ctx, `
			SELECT ev.*,u.full_name AS actor_name,e.name AS evidence_name
			FROM construction_action_events ev
			LEFT JOIN users u ON u.id=ev.actor_id
			LEFT JOIN evidence_records e ON e.id=ev.evidence_id
			WHERE ev.action_id=$1 ORDER BY ev.created_at,ev.id
		`, action["id"])
		if err != nil {
			return nil, err
		}
		action["events"] = events
	}
	workItems, err := r.Query(ctx, `
		SELECT id,title,status,work_type,source_type,source_id,run_node_id
		FROM work_items
		WHERE project_id=$1 AND status <> 'cancelled'
		ORDER BY CASE WHEN status IN ('ready','in_progress','needs_changes','blocked') THEN 0 ELSE 1 END,updated_at DESC
		LIMIT 300
	`, projectID)
	if err != nil {
		return nil, err
	}
	participants, err := r.Query(ctx, `
		SELECT DISTINCT u.id,u.full_name,u.email
		FROM project_members pm
		JOIN staff s ON s.id=pm.staff_id
		JOIN users u ON u.id=s.user_id
		WHERE pm.project_id=$1 AND pm.active AND s.status='active' AND u.status='active'
		ORDER BY u.full_name
	`, projectID)
	if err != nil {
		return nil, err
	}
	return Record{"templates": templates, "checks": checks, "actions": actions, "work_items": workItems, "participants": participants}, nil
}

func (r *Repository) CreateConstructionCheckTemplate(ctx context.Context, projectID, name, description string, criteria []ConstructionCheckCriterion, actorID, actorName string) (Record, error) {
	rawCriteria, _ := json.Marshal(criteria)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO construction_check_templates (project_id,name,description,criteria_jsonb,created_by)
		VALUES ($1,$2,NULLIF($3,''),$4::jsonb,NULLIF($5,'')::uuid)
		RETURNING *
	`, projectID, name, description, string(rawCriteria), actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	row := rows[0]
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "construction_check_template_created", "construction_check_template", cleanID(row["id"]), "Tạo mẫu kiểm tra "+name, Record{"criteria_count": len(criteria)}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return row, nil
}

func (r *Repository) CreateConstructionCheck(ctx context.Context, projectID, workItemID, sourceDailyReportID, templateID, title, location, scheduledAt string, criteria []ConstructionCheckCriterion, criteriaProvided bool, contextEvidenceIDs []string, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	workItems, err := queryTx(ctx, tx, `SELECT id FROM work_items WHERE id=$1 AND project_id=$2 AND status <> 'cancelled'`, workItemID, projectID)
	if err != nil {
		return nil, err
	}
	if len(workItems) == 0 {
		return nil, validationError("Công việc liên quan không thuộc công trình")
	}
	if err := validateConstructionSourceReportTx(ctx, tx, projectID, workItemID, sourceDailyReportID); err != nil {
		return nil, err
	}
	criteriaJSON, err := constructionCheckCriteriaJSONTx(ctx, tx, projectID, templateID, criteria, criteriaProvided)
	if err != nil {
		return nil, err
	}
	for _, evidenceID := range contextEvidenceIDs {
		if err := validateConstructionEvidenceTx(ctx, tx, projectID, workItemID, evidenceID); err != nil {
			return nil, err
		}
	}
	rows, err := queryTx(ctx, tx, `
		INSERT INTO construction_checks (project_id,work_item_id,source_daily_report_id,template_id,title,location,criteria_jsonb,scheduled_at,requested_by)
		VALUES ($1,$2,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,$5,NULLIF($6,''),$7::jsonb,NULLIF($8,'')::timestamptz,NULLIF($9,'')::uuid)
		RETURNING *
	`, projectID, workItemID, sourceDailyReportID, templateID, title, location, criteriaJSON, scheduledAt, actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	row := rows[0]
	if err := replaceConstructionCheckEvidenceTx(ctx, tx, cleanID(row["id"]), "context", contextEvidenceIDs, actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "construction_check_created", "construction_check", cleanID(row["id"]), "Tạo kiểm tra "+title, Record{"work_item_id": workItemID, "source_daily_report_id": sourceDailyReportID, "template_id": templateID, "scheduled_at": scheduledAt, "criteria_count": len(criteria), "context_evidence_count": len(contextEvidenceIDs)}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return row, nil
}

func (r *Repository) UpdateConstructionCheck(ctx context.Context, checkID, workItemID, templateID, title, location, scheduledAt string, criteria []ConstructionCheckCriterion, criteriaProvided bool, contextEvidenceIDs []string, contextEvidenceProvided bool, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	checks, err := queryTx(ctx, tx, `SELECT * FROM construction_checks WHERE id=$1 FOR UPDATE`, checkID)
	if err != nil {
		return nil, err
	}
	if len(checks) == 0 {
		return nil, sql.ErrNoRows
	}
	check := checks[0]
	if fmt.Sprint(check["status"]) != "pending" {
		return nil, validationError("Chỉ được chỉnh sửa kiểm tra đang chờ", Record{"status": check["status"]})
	}
	if cleanID(check["technical_rfi_id"]) != "" {
		return nil, validationError("Kiểm tra đồng bộ từ RFI chỉ được chỉnh sửa tại hồ sơ QA/QC")
	}
	if cleanID(check["source_handoff_item_id"]) != "" {
		return nil, validationError("Kiểm tra từ kế hoạch chỉ được thay đổi bằng một revision bàn giao mới")
	}
	projectID := cleanID(check["project_id"])
	workItems, err := queryTx(ctx, tx, `SELECT id FROM work_items WHERE id=$1 AND project_id=$2 AND status <> 'cancelled'`, workItemID, projectID)
	if err != nil {
		return nil, err
	}
	if len(workItems) == 0 {
		return nil, validationError("Công việc liên quan không thuộc công trình")
	}
	criteriaJSON, err := constructionCheckCriteriaJSONTx(ctx, tx, projectID, templateID, criteria, criteriaProvided)
	if err != nil {
		return nil, err
	}
	if !criteriaProvided {
		raw, marshalErr := json.Marshal(check["criteria_jsonb"])
		if marshalErr != nil {
			return nil, marshalErr
		}
		criteriaJSON = string(raw)
	}
	if contextEvidenceProvided {
		for _, evidenceID := range contextEvidenceIDs {
			if err := validateConstructionEvidenceTx(ctx, tx, projectID, workItemID, evidenceID); err != nil {
				return nil, err
			}
		}
	} else if cleanID(check["work_item_id"]) != workItemID {
		existingEvidence, err := queryTx(ctx, tx, `SELECT evidence_id FROM construction_check_evidence WHERE check_id=$1 AND evidence_role='context'`, checkID)
		if err != nil {
			return nil, err
		}
		for _, evidence := range existingEvidence {
			if err := validateConstructionEvidenceTx(ctx, tx, projectID, workItemID, cleanID(evidence["evidence_id"])); err != nil {
				return nil, validationError("Khi đổi công việc liên quan, cần cập nhật lại ảnh hiện trạng")
			}
		}
	}
	rows, err := queryTx(ctx, tx, `
		UPDATE construction_checks
		SET work_item_id=$2,template_id=NULLIF($3,'')::uuid,title=$4,location=NULLIF($5,''),
			criteria_jsonb=$6::jsonb,scheduled_at=NULLIF($7,'')::timestamptz,updated_at=now()
		WHERE id=$1 RETURNING *
	`, checkID, workItemID, templateID, title, location, criteriaJSON, scheduledAt)
	if err != nil {
		return nil, err
	}
	if contextEvidenceProvided {
		if err := replaceConstructionCheckEvidenceTx(ctx, tx, checkID, "context", contextEvidenceIDs, actorID); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "construction_check_updated", "construction_check", checkID, "Cập nhật kiểm tra "+title, Record{
		"before": Record{"work_item_id": check["work_item_id"], "template_id": check["template_id"], "title": check["title"], "location": check["location"], "scheduled_at": check["scheduled_at"], "criteria": check["criteria_jsonb"]},
		"after":  Record{"work_item_id": workItemID, "template_id": templateID, "title": title, "location": location, "scheduled_at": scheduledAt, "criteria": rows[0]["criteria_jsonb"], "context_evidence_updated": contextEvidenceProvided, "context_evidence_count": len(contextEvidenceIDs)},
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) CompleteConstructionCheck(ctx context.Context, checkID, result, note string, evidenceIDs []string, actionTitle, assignedUserID, dueAt, impactScope, priority, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	checks, err := queryTx(ctx, tx, `SELECT * FROM construction_checks WHERE id=$1 FOR UPDATE`, checkID)
	if err != nil {
		return nil, err
	}
	if len(checks) == 0 {
		return nil, sql.ErrNoRows
	}
	check := checks[0]
	if fmt.Sprint(check["status"]) != "pending" {
		return nil, validationError("Kiểm tra này đã có kết quả", Record{"status": check["status"]})
	}
	if cleanID(check["source_handoff_item_id"]) != "" {
		workRows, queryErr := queryTx(ctx, tx, `SELECT status FROM work_items WHERE id=$1 AND project_id=$2`, check["work_item_id"], check["project_id"])
		if queryErr != nil {
			return nil, queryErr
		}
		if len(workRows) == 0 || !statusIn(fmt.Sprint(workRows[0]["status"]), "ready", "in_progress", "blocked", "needs_changes") {
			return nil, validationError("Hạng mục này chưa tới giai đoạn thi công", Record{"work_item_status": firstRecordValue(workRows, "status")})
		}
	}
	for _, evidenceID := range evidenceIDs {
		if err := validateConstructionEvidenceTx(ctx, tx, cleanID(check["project_id"]), cleanID(check["work_item_id"]), evidenceID); err != nil {
			return nil, err
		}
	}
	primaryEvidenceID := evidenceIDs[0]
	updated, err := queryTx(ctx, tx, `
		UPDATE construction_checks
		SET status='completed',result=$2,note=NULLIF($3,''),evidence_id=$4,checked_by=NULLIF($5,'')::uuid,checked_at=now(),updated_at=now()
		WHERE id=$1 RETURNING *
	`, checkID, result, note, primaryEvidenceID, actorID)
	if err != nil {
		return nil, err
	}
	if err := replaceConstructionCheckEvidenceTx(ctx, tx, checkID, "result", evidenceIDs, actorID); err != nil {
		return nil, err
	}
	responseRow := Record{"check": updated[0]}
	if result == "fail" {
		if actionTitle == "" {
			actionTitle = "Khắc phục: " + fmt.Sprint(check["title"])
		}
		action, err := createConstructionActionTx(ctx, tx, cleanID(check["project_id"]), cleanID(check["work_item_id"]), "", checkID, actionTitle, note, priority, assignedUserID, dueAt, impactScope, primaryEvidenceID, actorID, actorName)
		if err != nil {
			return nil, err
		}
		responseRow["action"] = action
	}
	if cleanID(check["source_handoff_item_id"]) != "" {
		workStatus := "completed"
		if result == "fail" {
			workStatus = "blocked"
		}
		if err := handoffWorkItemStatusTx(ctx, tx, cleanID(check["work_item_id"]), workStatus, actorID); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, cleanID(check["project_id"]), actorID, actorName, "construction_check_completed", "construction_check", checkID, "Ghi nhận kiểm tra "+result, Record{"result": result, "evidence_id": primaryEvidenceID, "evidence_ids": evidenceIDs, "action_created": result == "fail"}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return responseRow, nil
}

func (r *Repository) CreateConstructionAction(ctx context.Context, projectID, workItemID, sourceDailyReportID, title, description, priority, assignedUserID, dueAt, impactScope, evidenceID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	action, err := createConstructionActionTx(ctx, tx, projectID, workItemID, sourceDailyReportID, "", title, description, priority, assignedUserID, dueAt, impactScope, evidenceID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return action, nil
}

func (r *Repository) UpdateConstructionAction(ctx context.Context, actionID, title, description, priority, assignedUserID, dueAt, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	actions, err := queryTx(ctx, tx, `SELECT * FROM construction_action_items WHERE id=$1 FOR UPDATE`, actionID)
	if err != nil {
		return nil, err
	}
	if len(actions) == 0 {
		return nil, sql.ErrNoRows
	}
	action := actions[0]
	if fmt.Sprint(action["status"]) != "open" {
		return nil, validationError("Chỉ được chỉnh sửa việc đang cần xử lý", Record{"status": action["status"]})
	}
	if !constructionActionEditable(action) {
		return nil, validationError("Việc đồng bộ từ hồ sơ kỹ thuật chỉ được chỉnh sửa tại nguồn QA/QC")
	}
	projectID := cleanID(action["project_id"])
	staffID, err := constructionProjectParticipantStaffIDTx(ctx, tx, projectID, assignedUserID)
	if err != nil {
		return nil, err
	}
	severity := "medium"
	if priority == "urgent" {
		severity = "high"
	}
	rows, err := queryTx(ctx, tx, `
		UPDATE construction_action_items
		SET title=$2,description=NULLIF($3,''),priority=$4,assigned_user_id=$5,due_at=$6::timestamptz,updated_at=now()
		WHERE id=$1 RETURNING *
	`, actionID, title, description, priority, assignedUserID, dueAt)
	if err != nil {
		return nil, err
	}
	if issueID := cleanID(action["technical_issue_id"]); issueID != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET title=$2,description=NULLIF($3,''),severity=$4,assigned_to=$5,updated_at=now() WHERE id=$1`, issueID, title, description, severity, staffID); err != nil {
			return nil, err
		}
	}
	if reworkID := cleanID(action["technical_rework_id"]); reworkID != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE rework_requests SET title=$2,description=NULLIF($3,''),assigned_to=$4,due_date=$5::timestamptz::date,updated_at=now() WHERE id=$1`, reworkID, title, description, staffID, dueAt); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "construction_action_updated", "construction_action", actionID, "Cập nhật việc cần xử lý "+title, Record{
		"before": Record{"title": action["title"], "description": action["description"], "priority": action["priority"], "assigned_user_id": action["assigned_user_id"], "due_at": action["due_at"]},
		"after":  Record{"title": title, "description": description, "priority": priority, "assigned_user_id": assignedUserID, "due_at": dueAt},
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func constructionActionEditable(action Record) bool {
	if cleanID(action["technical_ncr_id"]) != "" {
		return false
	}
	issueID := cleanID(action["technical_issue_id"])
	return issueID == "" || cleanID(action["id"]) != issueID
}

func createConstructionActionTx(ctx context.Context, tx *sql.Tx, projectID, workItemID, sourceDailyReportID, sourceCheckID, title, description, priority, assignedUserID, dueAt, impactScope, evidenceID, actorID, actorName string) (Record, error) {
	workItems, err := queryTx(ctx, tx, `SELECT id,run_node_id FROM work_items WHERE id=$1 AND project_id=$2 AND status <> 'cancelled'`, workItemID, projectID)
	if err != nil {
		return nil, err
	}
	if len(workItems) == 0 {
		return nil, validationError("Công việc liên quan không thuộc công trình")
	}
	if err := validateConstructionSourceReportTx(ctx, tx, projectID, workItemID, sourceDailyReportID); err != nil {
		return nil, err
	}
	if err := validateConstructionEvidenceTx(ctx, tx, projectID, workItemID, evidenceID); err != nil {
		return nil, err
	}
	staffID, err := constructionProjectParticipantStaffIDTx(ctx, tx, projectID, assignedUserID)
	if err != nil {
		return nil, err
	}
	severity := "medium"
	if priority == "urgent" {
		severity = "high"
	}
	issues, err := queryTx(ctx, tx, `
		INSERT INTO issues (project_id,node_id,title,description,severity,status,reported_by,assigned_to,requires_rework,requires_replanning)
		VALUES ($1,NULLIF($2,'')::uuid,$3,NULLIF($4,''),$5,'open',NULLIF($6,'')::uuid,$7,true,$8)
		RETURNING *
	`, projectID, cleanID(workItems[0]["run_node_id"]), title, description, severity, actorID, staffID, impactScope != "none")
	if err != nil {
		return nil, err
	}
	issue := issues[0]
	reworks, err := queryTx(ctx, tx, `
		INSERT INTO rework_requests (project_id,issue_id,node_id,title,description,assigned_to,status,due_date,created_by)
		VALUES ($1,$2,NULLIF($3,'')::uuid,$4,NULLIF($5,''),$6,'pending',$7::timestamptz::date,NULLIF($8,'')::uuid)
		RETURNING *
	`, projectID, issue["id"], cleanID(workItems[0]["run_node_id"]), title, description, staffID, dueAt, actorID)
	if err != nil {
		return nil, err
	}
	replanningID := ""
	if impactScope != "none" {
		replannings, err := queryTx(ctx, tx, `
			INSERT INTO replanning_requests (project_id,issue_id,node_id,reason,impact_scope,budget_impact,timeline_impact,status,created_by)
			VALUES ($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7,'pending',NULLIF($8,'')::uuid)
			RETURNING *
		`, projectID, issue["id"], cleanID(workItems[0]["run_node_id"]), title, impactScope, 0, 0, actorID)
		if err != nil {
			return nil, err
		}
		replanningID = cleanID(replannings[0]["id"])
	}
	actions, err := queryTx(ctx, tx, `
		INSERT INTO construction_action_items (
			project_id,work_item_id,source_daily_report_id,source_check_id,title,description,priority,assigned_user_id,due_at,status,
			impact_scope,before_evidence_id,technical_issue_id,technical_rework_id,technical_replanning_id,created_by
		) VALUES ($1,$2,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,$5,NULLIF($6,''),$7,$8,$9::timestamptz,'open',$10,$11,$12,$13,NULLIF($14,'')::uuid,NULLIF($15,'')::uuid)
		RETURNING *
	`, projectID, workItemID, sourceDailyReportID, sourceCheckID, title, description, priority, assignedUserID, dueAt, impactScope, evidenceID, issue["id"], reworks[0]["id"], replanningID, actorID)
	if err != nil {
		return nil, err
	}
	action := actions[0]
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO construction_action_events (action_id,event_type,comment,evidence_id,actor_id)
		VALUES ($1,'created',NULLIF($2,''),$3,NULLIF($4,'')::uuid)
	`, action["id"], description, evidenceID, actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "construction_action_created", "construction_action", cleanID(action["id"]), "Tạo việc cần xử lý "+title, Record{"work_item_id": workItemID, "source_daily_report_id": sourceDailyReportID, "source_check_id": sourceCheckID, "impact_scope": impactScope, "technical_issue_id": issue["id"], "technical_rework_id": reworks[0]["id"], "technical_replanning_id": replanningID}); err != nil {
		return nil, err
	}
	return action, nil
}

func validateConstructionSourceReportTx(ctx context.Context, tx *sql.Tx, projectID, workItemID, sourceDailyReportID string) error {
	if sourceDailyReportID == "" {
		return nil
	}
	reports, err := queryTx(ctx, tx, `
		SELECT id,work_item_id
		FROM operation_daily_reports
		WHERE id=$1 AND project_id=$2
	`, sourceDailyReportID, projectID)
	if err != nil {
		return err
	}
	if len(reports) == 0 {
		return validationError("Báo cáo nguồn không thuộc công trình")
	}
	reportWorkItemID := cleanID(reports[0]["work_item_id"])
	if reportWorkItemID != "" && reportWorkItemID != workItemID {
		return validationError("Báo cáo nguồn và bản ghi hiện trường phải cùng công việc")
	}
	return nil
}

func constructionProjectParticipantStaffIDTx(ctx context.Context, tx *sql.Tx, projectID, assignedUserID string) (string, error) {
	participants, err := queryTx(ctx, tx, `
		SELECT s.id AS staff_id
		FROM project_members pm JOIN staff s ON s.id=pm.staff_id JOIN users u ON u.id=s.user_id
		WHERE pm.project_id=$1 AND pm.active AND s.status='active' AND u.status='active' AND u.id=$2
		LIMIT 1
	`, projectID, assignedUserID)
	if err != nil {
		return "", err
	}
	if len(participants) == 0 {
		return "", validationError("Người phụ trách không phải thành viên đang hoạt động của công trình")
	}
	return cleanID(participants[0]["staff_id"]), nil
}

func constructionCheckCriteriaJSONTx(ctx context.Context, tx *sql.Tx, projectID, templateID string, criteria []ConstructionCheckCriterion, criteriaProvided bool) (string, error) {
	templateCriteria := any([]any{})
	if templateID != "" {
		templates, err := queryTx(ctx, tx, `SELECT criteria_jsonb FROM construction_check_templates WHERE id=$1 AND project_id=$2 AND status='active'`, templateID, projectID)
		if err != nil {
			return "", err
		}
		if len(templates) == 0 {
			return "", validationError("Mẫu kiểm tra không còn hoạt động trong công trình")
		}
		templateCriteria = templates[0]["criteria_jsonb"]
	}
	value := templateCriteria
	if criteriaProvided {
		value = criteria
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func replaceConstructionCheckEvidenceTx(ctx context.Context, tx *sql.Tx, checkID, role string, evidenceIDs []string, actorID string) error {
	if !statusIn(role, "context", "result") {
		return validationError("Vai trò minh chứng kiểm tra không hợp lệ")
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM construction_check_evidence WHERE check_id=$1 AND evidence_role=$2`, checkID, role); err != nil {
		return err
	}
	for index, evidenceID := range evidenceIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO construction_check_evidence (check_id,evidence_id,evidence_role,sort_order,created_by)
			VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid)
		`, checkID, evidenceID, role, index, actorID); err != nil {
			return err
		}
	}
	return nil
}

func validateConstructionEvidenceTx(ctx context.Context, tx *sql.Tx, projectID, workItemID, evidenceID string) error {
	rows, err := queryTx(ctx, tx, `
		SELECT id FROM evidence_records
		WHERE id=$1 AND project_id=$2 AND work_item_id=$3 AND status <> 'rejected'
	`, evidenceID, projectID, workItemID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return validationError("Minh chứng phải thuộc cùng công việc và công trình")
	}
	return nil
}

func (r *Repository) SubmitConstructionActionResolution(ctx context.Context, actionID, evidenceID, note, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	actions, err := queryTx(ctx, tx, `SELECT * FROM construction_action_items WHERE id=$1 FOR UPDATE`, actionID)
	if err != nil {
		return nil, err
	}
	if len(actions) == 0 {
		return nil, sql.ErrNoRows
	}
	action := actions[0]
	if fmt.Sprint(action["status"]) != "open" {
		return nil, validationError("Việc này không ở trạng thái cần xử lý", Record{"status": action["status"]})
	}
	if err := validateConstructionEvidenceTx(ctx, tx, cleanID(action["project_id"]), cleanID(action["work_item_id"]), evidenceID); err != nil {
		return nil, err
	}
	rows, err := queryTx(ctx, tx, `
		UPDATE construction_action_items
		SET status='waiting_confirmation',resolution_evidence_id=$2,resolution_note=NULLIF($3,''),
			submitted_by=NULLIF($4,'')::uuid,submitted_at=now(),updated_at=now()
		WHERE id=$1 RETURNING *
	`, actionID, evidenceID, note, actorID)
	if err != nil {
		return nil, err
	}
	if cleanID(action["technical_issue_id"]) != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET status='in_progress',updated_at=now() WHERE id=$1`, action["technical_issue_id"]); err != nil {
			return nil, err
		}
	}
	if cleanID(action["technical_rework_id"]) != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE rework_requests SET status='in_progress',updated_at=now() WHERE id=$1`, action["technical_rework_id"]); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO construction_action_events (action_id,event_type,comment,evidence_id,actor_id) VALUES ($1,'resolution_submitted',NULLIF($2,''),$3,NULLIF($4,'')::uuid)`, actionID, note, evidenceID, actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(action["project_id"]), actorID, actorName, "construction_action_resolution_submitted", "construction_action", actionID, "Gửi kết quả khắc phục", Record{"evidence_id": evidenceID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) VerifyConstructionAction(ctx context.Context, actionID, result, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	actions, err := queryTx(ctx, tx, `SELECT * FROM construction_action_items WHERE id=$1 FOR UPDATE`, actionID)
	if err != nil {
		return nil, err
	}
	if len(actions) == 0 {
		return nil, sql.ErrNoRows
	}
	action := actions[0]
	if fmt.Sprint(action["status"]) != "waiting_confirmation" {
		return nil, validationError("Việc này chưa chờ xác nhận", Record{"status": action["status"]})
	}
	nextStatus := "done"
	eventType := "verified_pass"
	issueStatus := "closed"
	reworkStatus := "completed"
	if result == "fail" {
		nextStatus = "open"
		eventType = "verified_fail"
		issueStatus = "in_progress"
		reworkStatus = "in_progress"
	}
	rows, err := queryTx(ctx, tx, `
		UPDATE construction_action_items
		SET status=$2,verified_by=NULLIF($3,'')::uuid,verified_at=now(),retry_count=retry_count+CASE WHEN $2='open' THEN 1 ELSE 0 END,updated_at=now()
		WHERE id=$1 RETURNING *
	`, actionID, nextStatus, actorID)
	if err != nil {
		return nil, err
	}
	if cleanID(action["technical_issue_id"]) != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET status=$2,updated_at=now() WHERE id=$1`, action["technical_issue_id"], issueStatus); err != nil {
			return nil, err
		}
	}
	if cleanID(action["technical_rework_id"]) != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE rework_requests SET status=$2,approved_by=COALESCE(approved_by,NULLIF($3,'')::uuid),updated_at=now() WHERE id=$1`, action["technical_rework_id"], reworkStatus, actorID); err != nil {
			return nil, err
		}
	}
	if sourceCheckID := cleanID(action["source_check_id"]); sourceCheckID != "" {
		handoffChecks, queryErr := queryTx(ctx, tx, `SELECT source_handoff_item_id FROM construction_checks WHERE id=$1`, sourceCheckID)
		if queryErr != nil {
			return nil, queryErr
		}
		if len(handoffChecks) > 0 && cleanID(handoffChecks[0]["source_handoff_item_id"]) != "" {
			workStatus := "blocked"
			if result == "pass" {
				workStatus = "completed"
			}
			if err := handoffWorkItemStatusTx(ctx, tx, cleanID(action["work_item_id"]), workStatus, actorID); err != nil {
				return nil, err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO construction_action_events (action_id,event_type,comment,evidence_id,actor_id) VALUES ($1,$2,NULLIF($3,''),$4,NULLIF($5,'')::uuid)`, actionID, eventType, comment, action["resolution_evidence_id"], actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(action["project_id"]), actorID, actorName, "construction_action_"+eventType, "construction_action", actionID, "Xác nhận kết quả khắc phục: "+result, Record{"result": result, "comment": comment, "retry_count": rows[0]["retry_count"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) ApproveConstructionActionImpact(ctx context.Context, actionID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	actions, err := queryTx(ctx, tx, `SELECT * FROM construction_action_items WHERE id=$1 FOR UPDATE`, actionID)
	if err != nil {
		return nil, err
	}
	if len(actions) == 0 {
		return nil, sql.ErrNoRows
	}
	action := actions[0]
	replanningID := cleanID(action["technical_replanning_id"])
	if replanningID == "" {
		return nil, validationError("Việc này không có ảnh hưởng cần duyệt")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE replanning_requests SET status='approved',approved_by=NULLIF($2,'')::uuid,updated_at=now() WHERE id=$1 AND status <> 'approved'`, replanningID, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO construction_action_events (action_id,event_type,comment,actor_id) VALUES ($1,'impact_approved','Đã xác nhận ảnh hưởng',NULLIF($2,'')::uuid)`, actionID, actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(action["project_id"]), actorID, actorName, "construction_action_impact_approved", "construction_action", actionID, "Xác nhận ảnh hưởng của việc cần xử lý", Record{"impact_scope": action["impact_scope"], "technical_replanning_id": replanningID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.QueryOne(ctx, `SELECT a.*,rp.status AS impact_status FROM construction_action_items a LEFT JOIN replanning_requests rp ON rp.id=a.technical_replanning_id WHERE a.id=$1`, actionID)
}

func (r *Repository) ProjectQualityWorkspace(ctx context.Context, projectID string) (Record, error) {
	itps, err := r.Query(ctx, `
		SELECT p.*,COALESCE(items.item_count,0) AS item_count
		FROM quality_itp_plans p
		LEFT JOIN (SELECT itp_plan_id,COUNT(*) item_count FROM quality_itp_items GROUP BY itp_plan_id) items ON items.itp_plan_id=p.id
		WHERE p.project_id=$1 AND p.status <> 'rejected'
		ORDER BY p.itp_code,p.revision_no DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	for _, plan := range itps {
		items, err := r.Query(ctx, `SELECT * FROM quality_itp_items WHERE itp_plan_id=$1 ORDER BY sequence_no`, plan["id"])
		if err != nil {
			return nil, err
		}
		plan["items"] = items
	}
	rfis, err := r.Query(ctx, `
		SELECT rfi.*,p.itp_code,p.revision_no,i.sequence_no,i.activity_name,i.control_point,i.evidence_required,
			w.title AS work_item_title
		FROM quality_inspection_requests rfi
		JOIN quality_itp_plans p ON p.id=rfi.itp_plan_id
		JOIN quality_itp_items i ON i.id=rfi.itp_item_id
		LEFT JOIN work_items w ON w.id=rfi.work_item_id
		WHERE rfi.project_id=$1 AND rfi.status <> 'cancelled'
		ORDER BY rfi.requested_inspection_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	ncrs, err := r.Query(ctx, `
		SELECT n.*,rfi.rfi_code,rw.status AS rework_status
		FROM quality_ncrs n
		LEFT JOIN quality_inspection_requests rfi ON rfi.id=n.rfi_id
		LEFT JOIN rework_requests rw ON rw.id=n.rework_request_id
		WHERE n.project_id=$1 AND n.status <> 'cancelled'
		ORDER BY n.created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	plans, err := r.Query(ctx, `
		SELECT rp.*,u.full_name AS assigned_user_name,
			COALESCE(stats.pending_count,0) AS pending_count,COALESCE(stats.overdue_count,0) AS overdue_count,COALESCE(stats.completed_count,0) AS completed_count
		FROM construction_recurring_plans rp
		LEFT JOIN users u ON u.id=rp.assigned_user_id
		LEFT JOIN (
			SELECT plan_id,COUNT(*) FILTER (WHERE status IN ('pending','in_progress')) pending_count,
				COUNT(*) FILTER (WHERE status='overdue' OR (status='pending' AND occurrence_date<CURRENT_DATE)) overdue_count,
				COUNT(*) FILTER (WHERE status='completed') completed_count
			FROM construction_recurring_occurrences GROUP BY plan_id
		) stats ON stats.plan_id=rp.id
		WHERE rp.project_id=$1 AND rp.status <> 'cancelled'
		ORDER BY rp.next_due_date,rp.title
	`, projectID)
	if err != nil {
		return nil, err
	}
	occurrences, err := r.Query(ctx, `
		SELECT o.*,p.plan_code,p.title AS plan_title,p.evidence_required,p.responsible_role,u.full_name AS assigned_user_name,
			CASE WHEN o.status='pending' AND o.occurrence_date<CURRENT_DATE THEN 'overdue' ELSE o.status END AS effective_status
		FROM construction_recurring_occurrences o
		JOIN construction_recurring_plans p ON p.id=o.plan_id
		LEFT JOIN users u ON u.id=o.assigned_user_id
		WHERE o.project_id=$1
		ORDER BY o.occurrence_date DESC,o.created_at DESC
		LIMIT 200
	`, projectID)
	if err != nil {
		return nil, err
	}
	evidenceOptions, err := r.Query(ctx, `
		SELECT e.id,e.work_item_id,e.name,e.evidence_type,e.status,e.created_at,w.title AS work_item_title
		FROM evidence_records e LEFT JOIN work_items w ON w.id=e.work_item_id
		WHERE e.project_id=$1 AND e.status <> 'rejected'
		ORDER BY e.created_at DESC LIMIT 200
	`, projectID)
	if err != nil {
		return nil, err
	}
	workItems, err := r.Query(ctx, `SELECT id,title,status,work_type FROM work_items WHERE project_id=$1 AND status <> 'cancelled' ORDER BY updated_at DESC LIMIT 200`, projectID)
	if err != nil {
		return nil, err
	}
	participants, err := r.Query(ctx, `
		SELECT DISTINCT u.id,u.full_name,u.email
		FROM project_members pm JOIN staff s ON s.id=pm.staff_id JOIN users u ON u.id=s.user_id
		WHERE pm.project_id=$1 AND pm.active AND s.status='active' AND u.status='active'
		ORDER BY u.full_name
	`, projectID)
	if err != nil {
		return nil, err
	}
	return Record{"itp_plans": itps, "inspection_requests": rfis, "ncrs": ncrs, "recurring_plans": plans, "recurring_occurrences": occurrences, "evidence_options": evidenceOptions, "work_items": workItems, "participants": participants}, nil
}

func (r *Repository) CreateQualityITP(ctx context.Context, projectID string, header Record, items []Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	hashPayload, _ := json.Marshal(Record{"project_id": projectID, "itp_code": header["itp_code"], "title": header["title"], "discipline": header["discipline"], "work_package": header["work_package"], "items": items})
	hash := sha256.Sum256(hashPayload)
	rows, err := queryTx(ctx, tx, `
		INSERT INTO quality_itp_plans (project_id,family_id,itp_code,title,discipline,work_package,revision_no,definition_hash,status,review_comment,document_id,created_by)
		VALUES ($1,gen_random_uuid(),$2,$3,$4,$5,1,$6,'draft',$7,NULLIF($8,'')::uuid,NULLIF($9,'')::uuid) RETURNING *
	`, projectID, header["itp_code"], header["title"], header["discipline"], header["work_package"], hex.EncodeToString(hash[:]), header["review_comment"], cleanID(header["document_id"]), actorID)
	if err != nil {
		return nil, err
	}
	planID := cleanID(rows[0]["id"])
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO quality_itp_items (itp_plan_id,sequence_no,activity_name,inspection_stage,acceptance_criteria,inspection_method,control_point,frequency_type,frequency_value,responsible_role,evidence_required,client_witness_required,notes)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		`, planID, item["sequence_no"], item["activity_name"], item["inspection_stage"], item["acceptance_criteria"], item["inspection_method"], item["control_point"], item["frequency_type"], item["frequency_value"], item["responsible_role"], item["evidence_required"], item["client_witness_required"], item["notes"]); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "quality_itp_created", "quality_itp", planID, "Tạo ITP revision 1", Record{"itp_code": header["itp_code"], "item_count": len(items), "definition_hash": hex.EncodeToString(hash[:])}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, projectID)
}

func (r *Repository) ReviseQualityITP(ctx context.Context, itpID, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	plans, err := queryTx(ctx, tx, `SELECT * FROM quality_itp_plans WHERE id=$1 AND status IN ('active','approved') FOR UPDATE`, itpID)
	if err != nil {
		return nil, err
	}
	if len(plans) == 0 {
		return nil, validationError("Chỉ ITP approved/active mới được tạo revision")
	}
	plan := plans[0]
	items, err := queryTx(ctx, tx, `SELECT * FROM quality_itp_items WHERE itp_plan_id=$1 ORDER BY sequence_no`, itpID)
	if err != nil {
		return nil, err
	}
	nextRevision := int(number(plan["revision_no"])) + 1
	hashPayload, _ := json.Marshal(Record{"itp_code": plan["itp_code"], "revision_no": nextRevision, "title": plan["title"], "discipline": plan["discipline"], "work_package": plan["work_package"], "items": items})
	hash := sha256.Sum256(hashPayload)
	rows, err := queryTx(ctx, tx, `
		INSERT INTO quality_itp_plans (project_id,family_id,itp_code,title,discipline,work_package,revision_no,supersedes_id,definition_hash,status,review_comment,document_id,created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'draft',$10,$11,NULLIF($12,'')::uuid) RETURNING id
	`, plan["project_id"], plan["family_id"], plan["itp_code"], plan["title"], plan["discipline"], plan["work_package"], nextRevision, itpID, hex.EncodeToString(hash[:]), comment, plan["document_id"], actorID)
	if err != nil {
		return nil, err
	}
	newID := cleanID(rows[0]["id"])
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO quality_itp_items (itp_plan_id,sequence_no,activity_name,inspection_stage,acceptance_criteria,inspection_method,control_point,frequency_type,frequency_value,responsible_role,evidence_required,client_witness_required,notes)
		SELECT $2,sequence_no,activity_name,inspection_stage,acceptance_criteria,inspection_method,control_point,frequency_type,frequency_value,responsible_role,evidence_required,client_witness_required,notes FROM quality_itp_items WHERE itp_plan_id=$1
	`, itpID, newID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(plan["project_id"]), actorID, actorName, "quality_itp_revised", "quality_itp", newID, "Tạo revision ITP mới", Record{"supersedes_id": itpID, "revision_no": nextRevision, "comment": comment}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, cleanID(plan["project_id"]))
}

func (r *Repository) QualityITPAction(ctx context.Context, itpID, action, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM quality_itp_plans WHERE id=$1 FOR UPDATE`, itpID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	plan := rows[0]
	fromStatus := fmt.Sprint(plan["status"])
	toStatus := ""
	eventType := ""
	switch action {
	case "submit":
		if fromStatus != "draft" {
			return nil, validationError("Chỉ ITP nháp mới được gửi review")
		}
		var itemCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM quality_itp_items WHERE itp_plan_id=$1`, itpID).Scan(&itemCount); err != nil {
			return nil, err
		}
		if itemCount == 0 {
			return nil, validationError("ITP cần ít nhất một inspection item")
		}
		toStatus, eventType = "review", "quality_itp_submitted"
	case "approve":
		if fromStatus != "review" {
			return nil, validationError("Chỉ ITP đang review mới được duyệt")
		}
		toStatus, eventType = "approved", "quality_itp_approved"
	case "activate":
		if fromStatus != "approved" {
			return nil, validationError("Chỉ ITP đã duyệt mới được kích hoạt")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE quality_itp_plans SET status='superseded',updated_at=now() WHERE family_id=$1 AND status='active' AND id<>$2`, plan["family_id"], itpID); err != nil {
			return nil, err
		}
		toStatus, eventType = "active", "quality_itp_activated"
	default:
		return nil, validationError("Hành động ITP không hợp lệ", Record{"action": action})
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE quality_itp_plans SET status=$2,review_comment=concat_ws(E'\n',NULLIF(review_comment,''),NULLIF($3,'')),
			submitted_by=CASE WHEN $4='submit' THEN NULLIF($5,'')::uuid ELSE submitted_by END,submitted_at=CASE WHEN $4='submit' THEN now() ELSE submitted_at END,
			approved_by=CASE WHEN $4='approve' THEN NULLIF($5,'')::uuid ELSE approved_by END,approved_at=CASE WHEN $4='approve' THEN now() ELSE approved_at END,
			activated_by=CASE WHEN $4='activate' THEN NULLIF($5,'')::uuid ELSE activated_by END,activated_at=CASE WHEN $4='activate' THEN now() ELSE activated_at END,updated_at=now()
		WHERE id=$1
	`, itpID, toStatus, comment, action, actorID)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(plan["project_id"]), actorID, actorName, eventType, "quality_itp", itpID, fmt.Sprintf("Chuyển ITP từ %s sang %s", fromStatus, toStatus), Record{"revision_no": plan["revision_no"], "definition_hash": plan["definition_hash"], "comment": comment}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, cleanID(plan["project_id"]))
}

func (r *Repository) CreateQualityInspectionRequest(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT i.*,p.project_id,p.status AS itp_status,p.id AS itp_plan_id
		FROM quality_itp_items i JOIN quality_itp_plans p ON p.id=i.itp_plan_id
		WHERE i.id=$1
	`, body["itp_item_id"])
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 || cleanID(rows[0]["project_id"]) != projectID || fmt.Sprint(rows[0]["itp_status"]) != "active" {
		return nil, validationError("RFI phải liên kết inspection item của ITP active trong cùng công trình")
	}
	itpItem := rows[0]
	if workItemID := cleanID(body["work_item_id"]); workItemID != "" {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND project_id=$2 AND status <> 'cancelled'`, workItemID, projectID).Scan(&count); err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, validationError("Work item RFI không thuộc công trình")
		}
	}
	if evidenceID := cleanID(body["request_evidence_id"]); evidenceID != "" {
		if err := requireQualityEvidenceTx(ctx, tx, evidenceID, projectID); err != nil {
			return nil, err
		}
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO quality_inspection_requests (project_id,rfi_code,itp_plan_id,itp_item_id,work_item_id,location,lot_reference,requested_inspection_at,status,request_comment,request_evidence_id,requested_by)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,$6,$7,$8::timestamptz,'draft',$9,NULLIF($10,'')::uuid,NULLIF($11,'')::uuid)
	`, projectID, body["rfi_code"], itpItem["itp_plan_id"], body["itp_item_id"], cleanID(body["work_item_id"]), body["location"], body["lot_reference"], body["requested_inspection_at"], body["request_comment"], cleanID(body["request_evidence_id"]), actorID)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "quality_rfi_created", "quality_rfi", "", "Tạo Request for Inspection", Record{"rfi_code": body["rfi_code"], "itp_item_id": body["itp_item_id"], "location": body["location"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, projectID)
}

func requireQualityEvidenceTx(ctx context.Context, tx *sql.Tx, evidenceID, projectID string) error {
	var status string
	var evidenceProjectID string
	if err := tx.QueryRowContext(ctx, `SELECT project_id::text,status FROM evidence_records WHERE id=$1`, evidenceID).Scan(&evidenceProjectID, &status); err != nil {
		return validationError("Minh chứng QA/QC không tồn tại", Record{"evidence_id": evidenceID})
	}
	if evidenceProjectID != projectID || status == "rejected" {
		return validationError("Minh chứng QA/QC không thuộc công trình hoặc đã bị từ chối", Record{"evidence_id": evidenceID})
	}
	return nil
}

func (r *Repository) QualityInspectionAction(ctx context.Context, rfiID, action, result, comment, inspectionEvidenceID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT rfi.*,i.activity_name,i.control_point,i.evidence_required,p.itp_code
		FROM quality_inspection_requests rfi
		JOIN quality_itp_items i ON i.id=rfi.itp_item_id
		JOIN quality_itp_plans p ON p.id=rfi.itp_plan_id
		WHERE rfi.id=$1 FOR UPDATE OF rfi
	`, rfiID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	rfi := rows[0]
	fromStatus := fmt.Sprint(rfi["status"])
	toStatus := ""
	eventType := ""
	if action == "submit" {
		if fromStatus != "draft" {
			return nil, validationError("Chỉ RFI nháp mới được gửi")
		}
		if boolValue(rfi["evidence_required"]) && cleanID(rfi["request_evidence_id"]) == "" {
			return nil, validationError("Inspection item yêu cầu minh chứng khi gửi RFI")
		}
		toStatus, eventType = "submitted", "quality_rfi_submitted"
		_, err = tx.ExecContext(ctx, `UPDATE quality_inspection_requests SET status='submitted',submitted_at=now(),updated_at=now() WHERE id=$1`, rfiID)
	} else if action == "schedule" {
		if fromStatus != "submitted" {
			return nil, validationError("Chỉ RFI đã gửi mới được xếp lịch")
		}
		toStatus, eventType = "scheduled", "quality_rfi_scheduled"
		_, err = tx.ExecContext(ctx, `UPDATE quality_inspection_requests SET status='scheduled',updated_at=now() WHERE id=$1`, rfiID)
	} else if action == "inspect" {
		if !statusIn(fromStatus, "submitted", "scheduled") {
			return nil, validationError("RFI chưa ở trạng thái sẵn sàng inspection")
		}
		if inspectionEvidenceID != "" {
			if err := requireQualityEvidenceTx(ctx, tx, inspectionEvidenceID, cleanID(rfi["project_id"])); err != nil {
				return nil, err
			}
		}
		if (boolValue(rfi["evidence_required"]) || result != "pass") && inspectionEvidenceID == "" {
			return nil, validationError("Inspection bắt buộc có minh chứng theo ITP hoặc khi kết quả không đạt")
		}
		toStatus = map[string]string{"pass": "accepted", "conditional_pass": "conditional", "fail": "rejected"}[result]
		eventType = "quality_rfi_inspected"
		_, err = tx.ExecContext(ctx, `
			UPDATE quality_inspection_requests SET status=$2,result=$3,inspection_comment=$4,inspection_evidence_id=NULLIF($5,'')::uuid,
				inspector_id=NULLIF($6,'')::uuid,inspected_at=now(),closed_at=now(),updated_at=now()
			WHERE id=$1
		`, rfiID, toStatus, result, comment, inspectionEvidenceID, actorID)
		if err == nil && result == "fail" {
			_, err = r.createQualityNCRTx(ctx, tx, cleanID(rfi["project_id"]), Record{
				"ncr_code": fmt.Sprintf("NCR-%s", fmt.Sprint(rfi["rfi_code"])), "source_type": "rfi", "source_id": rfiID,
				"rfi_id": rfiID, "work_item_id": cleanID(rfi["work_item_id"]), "title": fmt.Sprintf("Không phù hợp từ %s", rfi["rfi_code"]),
				"severity": "high", "nonconformity": comment, "immediate_action": "Tạm dừng hạng mục tại vị trí kiểm tra", "evidence_id": inspectionEvidenceID,
			}, actorID, actorName)
		}
	} else {
		return nil, validationError("Hành động RFI không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(rfi["project_id"]), actorID, actorName, eventType, "quality_rfi", rfiID, fmt.Sprintf("Chuyển RFI từ %s sang %s", fromStatus, toStatus), Record{"result": result, "itp_item_id": rfi["itp_item_id"], "control_point": rfi["control_point"], "comment": comment}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, cleanID(rfi["project_id"]))
}

func (r *Repository) CreateQualityNCR(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := r.createQualityNCRTx(ctx, tx, projectID, body, actorID, actorName); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, projectID)
}

func (r *Repository) createQualityNCRTx(ctx context.Context, tx *sql.Tx, projectID string, body Record, actorID, actorName string) (string, error) {
	if evidenceID := cleanID(body["evidence_id"]); evidenceID != "" {
		if err := requireQualityEvidenceTx(ctx, tx, evidenceID, projectID); err != nil {
			return "", err
		}
	}
	if rfiID := cleanID(body["rfi_id"]); rfiID != "" {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM quality_inspection_requests WHERE id=$1 AND project_id=$2`, rfiID, projectID).Scan(&count); err != nil || count == 0 {
			return "", validationError("RFI nguồn NCR không thuộc công trình")
		}
	}
	if workItemID := cleanID(body["work_item_id"]); workItemID != "" {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND project_id=$2`, workItemID, projectID).Scan(&count); err != nil || count == 0 {
			return "", validationError("Work item nguồn NCR không thuộc công trình")
		}
	}
	if issueID := cleanID(body["issue_id"]); issueID != "" {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues WHERE id=$1 AND project_id=$2`, issueID, projectID).Scan(&count); err != nil || count == 0 {
			return "", validationError("Issue nguồn NCR không thuộc công trình")
		}
	}
	reworkRows, err := queryTx(ctx, tx, `
		INSERT INTO rework_requests (project_id,issue_id,title,description,status,due_date,created_by)
		VALUES ($1,NULLIF($2,'')::uuid,$3,$4,'draft',NULLIF($5,'')::date,NULLIF($6,'')::uuid) RETURNING id
	`, projectID, cleanID(body["issue_id"]), fmt.Sprintf("Rework %s", body["ncr_code"]), body["nonconformity"], body["due_date"], actorID)
	if err != nil {
		return "", err
	}
	reworkID := cleanID(reworkRows[0]["id"])
	rows, err := queryTx(ctx, tx, `
		INSERT INTO quality_ncrs (project_id,ncr_code,source_type,source_id,rfi_id,work_item_id,issue_id,rework_request_id,title,severity,nonconformity,immediate_action,due_date,status,evidence_id,owner_id,created_by)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,NULLIF($7,'')::uuid,$8,$9,$10,$11,$12,NULLIF($13,'')::date,'open',NULLIF($14,'')::uuid,NULLIF($15,'')::uuid,NULLIF($16,'')::uuid) RETURNING id
	`, projectID, body["ncr_code"], body["source_type"], body["source_id"], cleanID(body["rfi_id"]), cleanID(body["work_item_id"]), cleanID(body["issue_id"]), reworkID, body["title"], body["severity"], body["nonconformity"], body["immediate_action"], body["due_date"], cleanID(body["evidence_id"]), cleanID(body["owner_id"]), actorID)
	if err != nil {
		return "", err
	}
	ncrID := cleanID(rows[0]["id"])
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "quality_ncr_created", "quality_ncr", ncrID, "Tạo NCR và rework", Record{"ncr_code": body["ncr_code"], "source_type": body["source_type"], "source_id": body["source_id"], "rework_request_id": reworkID}); err != nil {
		return "", err
	}
	return ncrID, nil
}

func (r *Repository) QualityNCRAction(ctx context.Context, ncrID, action string, req QualityNCRActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM quality_ncrs WHERE id=$1 FOR UPDATE`, ncrID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	ncr := rows[0]
	fromStatus := fmt.Sprint(ncr["status"])
	toStatus := ""
	eventType := ""
	switch action {
	case "root-cause":
		if fromStatus != "open" {
			return nil, validationError("NCR không ở trạng thái nhận root cause")
		}
		toStatus, eventType = "root_cause", "quality_ncr_root_cause_submitted"
		_, err = tx.ExecContext(ctx, `UPDATE quality_ncrs SET status=$2,root_cause=$3,preventive_action=$4,root_cause_submitted_by=NULLIF($5,'')::uuid,updated_at=now() WHERE id=$1`, ncrID, toStatus, strings.TrimSpace(req.RootCause), strings.TrimSpace(req.PreventiveAction), actorID)
	case "corrective":
		if fromStatus != "root_cause" {
			return nil, validationError("NCR cần root cause trước corrective action")
		}
		if err := requireQualityEvidenceTx(ctx, tx, cleanID(req.CorrectiveEvidenceID), cleanID(ncr["project_id"])); err != nil {
			return nil, err
		}
		toStatus, eventType = "corrective_action", "quality_ncr_corrective_submitted"
		_, err = tx.ExecContext(ctx, `UPDATE quality_ncrs SET status=$2,corrective_action=$3,preventive_action=COALESCE(NULLIF($4,''),preventive_action),corrective_evidence_id=$5,corrective_submitted_by=NULLIF($6,'')::uuid,updated_at=now() WHERE id=$1`, ncrID, toStatus, strings.TrimSpace(req.CorrectiveAction), strings.TrimSpace(req.PreventiveAction), cleanID(req.CorrectiveEvidenceID), actorID)
	case "reinspection":
		if fromStatus != "corrective_action" {
			return nil, validationError("NCR cần corrective action trước tái kiểm tra")
		}
		var projectID, rfiStatus, rfiResult string
		if err := tx.QueryRowContext(ctx, `SELECT project_id::text,status,COALESCE(result,'') FROM quality_inspection_requests WHERE id=$1`, cleanID(req.ReinspectionRFIID)).Scan(&projectID, &rfiStatus, &rfiResult); err != nil || projectID != cleanID(ncr["project_id"]) {
			return nil, validationError("RFI tái kiểm tra không thuộc công trình")
		}
		toStatus, eventType = "reinspection", "quality_ncr_reinspection_linked"
		_, err = tx.ExecContext(ctx, `UPDATE quality_ncrs SET status=$2,reinspection_rfi_id=$3,updated_at=now() WHERE id=$1`, ncrID, toStatus, cleanID(req.ReinspectionRFIID))
	case "close":
		if fromStatus != "reinspection" || cleanID(ncr["reinspection_rfi_id"]) == "" {
			return nil, validationError("NCR cần RFI tái kiểm tra trước khi đóng")
		}
		var rfiStatus, rfiResult string
		if err := tx.QueryRowContext(ctx, `SELECT status,COALESCE(result,'') FROM quality_inspection_requests WHERE id=$1`, ncr["reinspection_rfi_id"]).Scan(&rfiStatus, &rfiResult); err != nil {
			return nil, err
		}
		if !statusIn(rfiStatus, "accepted", "conditional") || !statusIn(rfiResult, "pass", "conditional_pass") {
			return nil, validationError("RFI tái kiểm tra chưa đạt", Record{"rfi_status": rfiStatus, "rfi_result": rfiResult})
		}
		if reworkID := cleanID(ncr["rework_request_id"]); reworkID != "" {
			var reworkStatus string
			if err := tx.QueryRowContext(ctx, `SELECT status FROM rework_requests WHERE id=$1`, reworkID).Scan(&reworkStatus); err != nil {
				return nil, err
			}
			if !statusIn(reworkStatus, "done", "completed") {
				return nil, validationError("Rework liên kết chưa hoàn tất", Record{"rework_status": reworkStatus})
			}
		}
		toStatus, eventType = "closed", "quality_ncr_closed"
		_, err = tx.ExecContext(ctx, `UPDATE quality_ncrs SET status='closed',closed_by=NULLIF($2,'')::uuid,closed_at=now(),closure_comment=$3,updated_at=now() WHERE id=$1`, ncrID, actorID, strings.TrimSpace(req.Comment))
	default:
		return nil, validationError("Hành động NCR không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(ncr["project_id"]), actorID, actorName, eventType, "quality_ncr", ncrID, fmt.Sprintf("Chuyển NCR từ %s sang %s", fromStatus, toStatus), Record{"rework_request_id": ncr["rework_request_id"], "reinspection_rfi_id": req.ReinspectionRFIID, "comment": req.Comment}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, cleanID(ncr["project_id"]))
}

func (r *Repository) CreateRecurringPlan(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	if sourceWorkItemID := cleanID(body["source_work_item_id"]); sourceWorkItemID != "" {
		var count int
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND project_id=$2`, sourceWorkItemID, projectID).Scan(&count); err != nil || count == 0 {
			return nil, validationError("Work item nguồn lịch lặp không thuộc công trình")
		}
	}
	rows, err := r.Query(ctx, `
		INSERT INTO construction_recurring_plans (project_id,plan_code,title,description,recurrence_type,interval_value,start_date,end_date,next_due_date,due_time,responsible_role,assigned_user_id,evidence_required,source_work_item_id,status,created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::date,$7,NULLIF($9,'')::time,$10,NULLIF($11,'')::uuid,$12,NULLIF($13,'')::uuid,'active',NULLIF($14,'')::uuid) RETURNING *
	`, projectID, body["plan_code"], body["title"], body["description"], body["recurrence_type"], body["interval_value"], body["start_date"], body["end_date"], body["due_time"], body["responsible_role"], cleanID(body["assigned_user_id"]), body["evidence_required"], cleanID(body["source_work_item_id"]), actorID)
	if err != nil {
		return nil, err
	}
	planID := cleanID(rows[0]["id"])
	r.AddAuditBestEffort(ctx, projectID, actorID, actorName, "recurring_plan_created", "recurring_plan", planID, "Tạo lịch QA/QC lặp", Record{"plan_code": body["plan_code"], "recurrence_type": body["recurrence_type"], "interval_value": body["interval_value"]})
	return r.ProjectQualityWorkspace(ctx, projectID)
}

func (r *Repository) GenerateRecurringOccurrences(ctx context.Context, planID, throughDate, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM construction_recurring_plans WHERE id=$1 AND status='active' FOR UPDATE`, planID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, validationError("Lịch lặp không hoạt động")
	}
	plan := rows[0]
	nextDate, _ := time.Parse("2006-01-02", dateOnly(plan["next_due_date"]))
	through, _ := time.Parse("2006-01-02", throughDate)
	if end := dateOnly(plan["end_date"]); end != "" {
		endDate, _ := time.Parse("2006-01-02", end)
		if through.After(endDate) {
			through = endDate
		}
	}
	generated := 0
	for !nextDate.After(through) {
		dueAt := any(nil)
		if dueTime := strings.TrimSpace(fmt.Sprint(plan["due_time"])); dueTime != "" && dueTime != "<nil>" {
			dueAt = nextDate.Format("2006-01-02") + "T" + strings.TrimSuffix(dueTime, ":00+00")
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO construction_recurring_occurrences (plan_id,project_id,occurrence_date,due_at,assigned_user_id,status)
			VALUES ($1,$2,$3,$4::timestamptz,NULLIF($5,'')::uuid,'pending') ON CONFLICT (plan_id,occurrence_date) DO NOTHING
		`, planID, plan["project_id"], nextDate.Format("2006-01-02"), dueAt, cleanID(plan["assigned_user_id"]))
		if err != nil {
			return nil, err
		}
		if count, _ := result.RowsAffected(); count > 0 {
			generated++
		}
		nextDate = nextRecurringDate(nextDate, fmt.Sprint(plan["recurrence_type"]), int(number(plan["interval_value"])))
	}
	if _, err := tx.ExecContext(ctx, `UPDATE construction_recurring_occurrences SET status='overdue',updated_at=now() WHERE plan_id=$1 AND status='pending' AND occurrence_date<CURRENT_DATE`, planID); err != nil {
		return nil, err
	}
	newPlanStatus := "active"
	if end := dateOnly(plan["end_date"]); end != "" && nextDate.Format("2006-01-02") > end {
		newPlanStatus = "completed"
	}
	if _, err := tx.ExecContext(ctx, `UPDATE construction_recurring_plans SET next_due_date=$2,status=$3,updated_at=now() WHERE id=$1`, planID, nextDate.Format("2006-01-02"), newPlanStatus); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(plan["project_id"]), actorID, actorName, "recurring_occurrences_generated", "recurring_plan", planID, "Sinh occurrence QA/QC lặp", Record{"through_date": throughDate, "generated_count": generated, "next_due_date": nextDate.Format("2006-01-02")}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, cleanID(plan["project_id"]))
}

func nextRecurringDate(current time.Time, recurrenceType string, interval int) time.Time {
	switch recurrenceType {
	case "weekly":
		return current.AddDate(0, 0, 7*interval)
	case "monthly":
		return current.AddDate(0, interval, 0)
	default:
		return current.AddDate(0, 0, interval)
	}
}

func (r *Repository) RecurringOccurrenceAction(ctx context.Context, occurrenceID, action, comment, evidenceID, reason, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT o.*,p.evidence_required FROM construction_recurring_occurrences o JOIN construction_recurring_plans p ON p.id=o.plan_id WHERE o.id=$1 FOR UPDATE OF o`, occurrenceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	occurrence := rows[0]
	if !statusIn(fmt.Sprint(occurrence["status"]), "pending", "in_progress", "overdue") {
		return nil, validationError("Occurrence đã kết thúc")
	}
	toStatus := ""
	if action == "complete" {
		if boolValue(occurrence["evidence_required"]) && evidenceID == "" {
			return nil, validationError("Occurrence yêu cầu minh chứng hoàn tất")
		}
		if evidenceID != "" {
			if err := requireQualityEvidenceTx(ctx, tx, evidenceID, cleanID(occurrence["project_id"])); err != nil {
				return nil, err
			}
		}
		toStatus = "completed"
		_, err = tx.ExecContext(ctx, `UPDATE construction_recurring_occurrences SET status='completed',completion_comment=$2,evidence_id=NULLIF($3,'')::uuid,completed_by=NULLIF($4,'')::uuid,completed_at=now(),updated_at=now() WHERE id=$1`, occurrenceID, comment, evidenceID, actorID)
	} else if action == "skip" {
		toStatus = "skipped"
		_, err = tx.ExecContext(ctx, `UPDATE construction_recurring_occurrences SET status='skipped',skip_reason=$2,skipped_by=NULLIF($3,'')::uuid,skipped_at=now(),updated_at=now() WHERE id=$1`, occurrenceID, reason, actorID)
	} else {
		return nil, validationError("Hành động occurrence không hợp lệ")
	}
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(occurrence["project_id"]), actorID, actorName, "recurring_occurrence_"+toStatus, "recurring_occurrence", occurrenceID, "Cập nhật occurrence QA/QC", Record{"plan_id": occurrence["plan_id"], "occurrence_date": occurrence["occurrence_date"], "comment": comment, "reason": reason}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, cleanID(occurrence["project_id"]))
}

func (r *Repository) CreateStockMovement(ctx context.Context, req StockActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	movement, err := createStockMovementTx(ctx, tx, req, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return movement, nil
}

func createStockMovementTx(ctx context.Context, tx *sql.Tx, req StockActionRequest, actorID, actorName string) (Record, error) {
	req.MovementType = strings.TrimSpace(req.MovementType)
	if req.MovementType == "" {
		req.MovementType = "receive"
	}
	if !statusIn(req.MovementType, "receive", "issue", "transfer", "loss", "return_to_warehouse", "return_to_subcontractor") {
		return nil, validationError("Loại nhập xuất kho không hợp lệ", Record{"movement_type": req.MovementType})
	}
	req.ProjectID = cleanID(req.ProjectID)
	req.WarehouseID = cleanID(req.WarehouseID)
	req.TargetWarehouseID = cleanID(req.TargetWarehouseID)
	req.MaterialID = cleanID(req.MaterialID)
	req.ReferenceID = cleanID(req.ReferenceID)
	req.StockDocumentID = cleanID(req.StockDocumentID)
	req.ReferenceType = strings.TrimSpace(req.ReferenceType)
	req.Unit = strings.TrimSpace(req.Unit)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.WarehouseID == "" {
		return nil, validationError("Kho là bắt buộc")
	}
	if req.MovementType == "transfer" && (req.TargetWarehouseID == "" || req.TargetWarehouseID == req.WarehouseID) {
		return nil, validationError("Điều chuyển cần kho đích khác kho nguồn")
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

	lockKeys := []string{"stock:" + req.WarehouseID + ":" + req.MaterialID}
	if req.TargetWarehouseID != "" {
		lockKeys = append(lockKeys, "stock:"+req.TargetWarehouseID+":"+req.MaterialID)
	}
	if req.ProjectID != "" {
		lockKeys = append(lockKeys, "site-material:"+req.ProjectID+":"+req.MaterialID)
	}
	sort.Strings(lockKeys)
	for _, lockKey := range lockKeys {
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, lockKey); err != nil {
			return nil, err
		}
	}

	warehouseIDs := []string{req.WarehouseID}
	if req.TargetWarehouseID != "" {
		warehouseIDs = append(warehouseIDs, req.TargetWarehouseID)
	}
	for _, warehouseID := range warehouseIDs {
		warehouseRows, err := queryTx(ctx, tx, `SELECT id FROM warehouses WHERE id=$1 AND active`, warehouseID)
		if err != nil {
			return nil, err
		}
		if len(warehouseRows) == 0 {
			return nil, validationError("Kho không tồn tại hoặc đã ngưng hoạt động", Record{"warehouse_id": warehouseID})
		}
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
	if req.MovementType == "receive" && req.ReferenceType == "incoming_acceptance_item" && req.ReferenceID != "" {
		sourceRows, err := queryTx(ctx, tx, `
			SELECT iai.material_id, iai.project_id, iai.accepted_quantity,
				COALESCE(received.received_quantity,0)+COALESCE(legacy_stock.received_quantity,0) AS received_quantity
			FROM incoming_acceptance_items iai
			JOIN incoming_acceptance_records iar ON iar.id=iai.incoming_acceptance_id AND iar.status='submitted'
			LEFT JOIN procurement_request_items pri ON pri.id=iai.procurement_request_item_id
			LEFT JOIN LATERAL (
				SELECT SUM(sm.quantity) AS received_quantity
				FROM stock_movements sm
				WHERE sm.movement_type='receive'
				  AND sm.reference_type='incoming_acceptance_item'
				  AND sm.reference_id=iai.id
			) received ON true
			LEFT JOIN LATERAL (
				SELECT SUM(sm.quantity) AS received_quantity
				FROM stock_movements sm
				WHERE sm.movement_type='receive'
				  AND sm.reference_type='boq_item'
				  AND sm.reference_id=pri.boq_item_id
			) legacy_stock ON true
			WHERE iai.id=$1
			FOR UPDATE OF iai
		`, req.ReferenceID)
		if err != nil {
			return nil, err
		}
		if len(sourceRows) == 0 || cleanID(sourceRows[0]["material_id"]) != req.MaterialID {
			return nil, validationError("Dòng nghiệm thu đầu vào không tồn tại hoặc không khớp vật tư")
		}
		if req.ProjectID != "" && cleanID(sourceRows[0]["project_id"]) != req.ProjectID {
			return nil, validationError("Dòng nghiệm thu đầu vào không thuộc công trình của phiếu nhập")
		}
		remaining := number(sourceRows[0]["accepted_quantity"]) - number(sourceRows[0]["received_quantity"])
		if quantityExceedsAvailable(req.Quantity, remaining) {
			return nil, validationError("Số lượng nhập vượt khối lượng đã đạt nghiệm thu còn lại", Record{"remaining": remaining, "quantity": req.Quantity})
		}
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
			(movement_code, project_id, warehouse_id, target_warehouse_id, material_id, movement_type, quantity, unit, reason, reference_type, reference_id, stock_document_id, created_by, approved_by)
		VALUES ($1,NULLIF($2,'')::uuid,$3,NULLIF($4,'')::uuid,$5,$6,$7,$8,$9,$10,NULLIF($11,'')::uuid,NULLIF($12,'')::uuid,NULLIF($13,'')::uuid,NULLIF($13,'')::uuid)
		RETURNING *
	`, code, req.ProjectID, req.WarehouseID, req.TargetWarehouseID, req.MaterialID, req.MovementType, req.Quantity, req.Unit, req.Reason, req.ReferenceType, req.ReferenceID, req.StockDocumentID, actorID)
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
				VALUES ($1,$2,$3::numeric,GREATEST($3::numeric,0::numeric),'pending')
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
				SET issued_quantity = GREATEST(issued_quantity - $3::numeric, 0::numeric),
					surplus_quantity = GREATEST(GREATEST(issued_quantity - $3::numeric, 0::numeric) - used_quantity, 0::numeric),
					surplus_status = CASE
						WHEN GREATEST(GREATEST(issued_quantity - $3::numeric, 0::numeric) - used_quantity, 0::numeric) > 0.000001 THEN 'partial'
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

	metadata := Record{"movement_type": req.MovementType, "quantity": req.Quantity, "material_id": req.MaterialID, "warehouse_id": req.WarehouseID}
	if req.TargetWarehouseID != "" {
		metadata["target_warehouse_id"] = req.TargetWarehouseID
	}
	if req.ReferenceType != "" {
		metadata["reference_type"] = req.ReferenceType
	}
	if req.ReferenceID != "" {
		metadata["reference_id"] = req.ReferenceID
	}
	if req.StockDocumentID != "" {
		metadata["stock_document_id"] = req.StockDocumentID
	}
	if err := addAuditTx(ctx, tx, req.ProjectID, actorID, actorName, stockMovementEvent(req.MovementType), "stock_movement", fmt.Sprint(movement["id"]), stockMovementDescription(req), metadata); err != nil {
		return nil, err
	}
	return movement, nil
}

func warehouseMaterialAvailableTx(ctx context.Context, tx *sql.Tx, warehouseID, materialID string) (float64, error) {
	rows, err := queryTx(ctx, tx, `
		SELECT COALESCE(SUM(CASE
			WHEN warehouse_id=$1 AND movement_type IN ('receive','return_to_warehouse') THEN quantity
			WHEN warehouse_id=$1 AND movement_type IN ('issue','transfer','loss') THEN -quantity
			WHEN target_warehouse_id=$1 AND movement_type='transfer' THEN quantity
			ELSE 0
		END),0) AS available
		FROM stock_movements
		WHERE material_id=$2 AND (warehouse_id=$1 OR target_warehouse_id=$1)
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

func (r *Repository) CreateStockDocument(ctx context.Context, req StockDocumentRequest, actorID, actorName string) (Record, error) {
	req.ProjectID = cleanID(req.ProjectID)
	req.MovementType = strings.TrimSpace(req.MovementType)
	req.SourceWarehouseID = cleanID(req.SourceWarehouseID)
	req.TargetWarehouseID = cleanID(req.TargetWarehouseID)
	req.ReferenceType = strings.TrimSpace(req.ReferenceType)
	req.ReferenceID = cleanID(req.ReferenceID)
	req.DocumentDate = strings.TrimSpace(req.DocumentDate)
	req.Note = strings.TrimSpace(req.Note)
	if !statusIn(req.MovementType, "receive", "issue", "transfer", "return_to_warehouse", "return_to_subcontractor") {
		return nil, validationError("Loại phiếu kho không hợp lệ", Record{"movement_type": req.MovementType})
	}
	if len(req.Items) == 0 {
		return nil, validationError("Phiếu kho cần ít nhất một dòng vật tư")
	}
	if req.DocumentDate == "" {
		req.DocumentDate = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", req.DocumentDate); err != nil {
		return nil, validationError("Ngày phiếu kho không hợp lệ")
	}
	if statusIn(req.MovementType, "issue", "transfer") && req.SourceWarehouseID == "" {
		return nil, validationError("Kho nguồn là bắt buộc")
	}
	if statusIn(req.MovementType, "receive", "return_to_warehouse") && req.TargetWarehouseID == "" {
		return nil, validationError("Kho nhận là bắt buộc")
	}
	if req.MovementType == "transfer" && (req.TargetWarehouseID == "" || req.TargetWarehouseID == req.SourceWarehouseID) {
		return nil, validationError("Điều chuyển cần kho đích khác kho nguồn")
	}
	if statusIn(req.MovementType, "issue", "return_to_warehouse", "return_to_subcontractor") && req.ProjectID == "" {
		return nil, validationError("Công trình là bắt buộc cho phiếu xuất/hoàn trả")
	}
	if req.MovementType == "return_to_subcontractor" && req.SourceWarehouseID == "" && req.TargetWarehouseID == "" {
		return nil, validationError("Kho quản lý vật tư hoàn trả là bắt buộc")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if req.ReferenceType == "incoming_acceptance" {
		rows, err := queryTx(ctx, tx, `
			SELECT id, project_id, status
			FROM incoming_acceptance_records
			WHERE id=$1
			FOR UPDATE
		`, req.ReferenceID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 || fmt.Sprint(rows[0]["status"]) != "submitted" {
			return nil, validationError("Nghiệm thu đầu vào chưa được gửi hoặc không tồn tại")
		}
		if req.ProjectID == "" {
			req.ProjectID = cleanID(rows[0]["project_id"])
		}
		if req.ProjectID != cleanID(rows[0]["project_id"]) {
			return nil, validationError("Nghiệm thu đầu vào không thuộc công trình của phiếu kho")
		}
	}

	prefix := map[string]string{"receive": "PNK", "issue": "PXK", "transfer": "PDC", "return_to_warehouse": "PHT", "return_to_subcontractor": "PTN"}[req.MovementType]
	documentCode := fmt.Sprintf("%s-%d-%s", prefix, timeNowUnix(), newID()[:6])
	documents, err := queryTx(ctx, tx, `
		INSERT INTO stock_documents (
			document_code, project_id, movement_type, source_warehouse_id, target_warehouse_id,
			reference_type, reference_id, document_date, note, status, created_by, posted_by
		) VALUES (
			$1,NULLIF($2,'')::uuid,$3,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,
			$6,NULLIF($7,'')::uuid,$8::date,$9,'posted',NULLIF($10,'')::uuid,NULLIF($10,'')::uuid
		)
		RETURNING *
	`, documentCode, req.ProjectID, req.MovementType, req.SourceWarehouseID, req.TargetWarehouseID, req.ReferenceType, req.ReferenceID, req.DocumentDate, req.Note, actorID)
	if err != nil {
		return nil, err
	}
	if len(documents) == 0 {
		return nil, sql.ErrNoRows
	}
	documentID := cleanID(documents[0]["id"])

	for itemIndex, item := range req.Items {
		item.MaterialID = cleanID(item.MaterialID)
		item.Unit = strings.TrimSpace(item.Unit)
		item.Reason = strings.TrimSpace(item.Reason)
		item.SourceLineType = strings.TrimSpace(item.SourceLineType)
		item.SourceLineID = cleanID(item.SourceLineID)
		if item.MaterialID == "" || item.Quantity <= 0 {
			return nil, validationError("Dòng vật tư phiếu kho không hợp lệ", Record{"item_index": itemIndex})
		}
		if req.ReferenceType == "incoming_acceptance" {
			if item.SourceLineType != "incoming_acceptance_item" || item.SourceLineID == "" {
				return nil, validationError("Dòng nhập kho cần liên kết dòng nghiệm thu đầu vào", Record{"item_index": itemIndex})
			}
			rows, err := queryTx(ctx, tx, `
				SELECT iai.id, iai.incoming_acceptance_id, iai.material_id, iai.accepted_quantity,
					COALESCE(received.received_quantity,0)+COALESCE(legacy_stock.received_quantity,0) AS received_quantity
				FROM incoming_acceptance_items iai
				LEFT JOIN procurement_request_items pri ON pri.id=iai.procurement_request_item_id
				LEFT JOIN LATERAL (
					SELECT SUM(sm.quantity) AS received_quantity
					FROM stock_movements sm
					WHERE sm.movement_type='receive'
					  AND sm.reference_type='incoming_acceptance_item'
					  AND sm.reference_id=iai.id
				) received ON true
				LEFT JOIN LATERAL (
					SELECT SUM(sm.quantity) AS received_quantity
					FROM stock_movements sm
					WHERE sm.movement_type='receive'
					  AND sm.reference_type='boq_item'
					  AND sm.reference_id=pri.boq_item_id
				) legacy_stock ON true
				WHERE iai.id=$1
				FOR UPDATE OF iai
			`, item.SourceLineID)
			if err != nil {
				return nil, err
			}
			if len(rows) == 0 || cleanID(rows[0]["incoming_acceptance_id"]) != req.ReferenceID || cleanID(rows[0]["material_id"]) != item.MaterialID {
				return nil, validationError("Dòng nghiệm thu đầu vào không khớp vật tư hoặc chứng từ nguồn", Record{"item_index": itemIndex})
			}
			remaining := number(rows[0]["accepted_quantity"]) - number(rows[0]["received_quantity"])
			if quantityExceedsAvailable(item.Quantity, remaining) {
				return nil, validationError("Số lượng nhập vượt khối lượng đã đạt nghiệm thu còn lại", Record{"item_index": itemIndex, "remaining": remaining, "quantity": item.Quantity})
			}
		}
		itemRows, err := queryTx(ctx, tx, `
			INSERT INTO stock_document_items (document_id, material_id, quantity, unit, reason, source_line_type, source_line_id)
			VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,'')::uuid)
			RETURNING id
		`, documentID, item.MaterialID, item.Quantity, item.Unit, item.Reason, item.SourceLineType, item.SourceLineID)
		if err != nil {
			return nil, err
		}
		if len(itemRows) == 0 {
			return nil, sql.ErrNoRows
		}
		warehouseID := req.SourceWarehouseID
		if statusIn(req.MovementType, "receive", "return_to_warehouse") {
			warehouseID = req.TargetWarehouseID
		}
		if req.MovementType == "return_to_subcontractor" && warehouseID == "" {
			warehouseID = req.TargetWarehouseID
		}
		movementReferenceType := item.SourceLineType
		movementReferenceID := item.SourceLineID
		if movementReferenceType == "" {
			movementReferenceType = "stock_document_item"
			movementReferenceID = cleanID(itemRows[0]["id"])
		}
		movementTargetWarehouseID := ""
		if req.MovementType == "transfer" {
			movementTargetWarehouseID = req.TargetWarehouseID
		}
		if _, err := createStockMovementTx(ctx, tx, StockActionRequest{
			ProjectID: req.ProjectID, WarehouseID: warehouseID, TargetWarehouseID: movementTargetWarehouseID,
			MaterialID: item.MaterialID, MovementType: req.MovementType, Quantity: item.Quantity,
			Unit: item.Unit, Reason: item.Reason, ReferenceType: movementReferenceType,
			ReferenceID: movementReferenceID, StockDocumentID: documentID,
		}, actorID, actorName); err != nil {
			return nil, err
		}
	}

	if err := addAuditTx(ctx, tx, req.ProjectID, actorID, actorName, "stock_document_posted", "stock_document", documentID, "Ghi nhận phiếu kho "+documentCode, Record{
		"document_code": documentCode, "movement_type": req.MovementType, "item_count": len(req.Items),
		"source_warehouse_id": req.SourceWarehouseID, "target_warehouse_id": req.TargetWarehouseID,
		"reference_type": req.ReferenceType, "reference_id": req.ReferenceID,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetStockDocument(ctx, documentID)
}

func (r *Repository) StockPositions(ctx context.Context, userID, warehouseID string) ([]Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 2)
	if err != nil {
		return nil, err
	}
	args := append([]any{cleanID(warehouseID)}, scopeArgs...)
	return r.Query(ctx, fmt.Sprintf(`
		WITH visible_movements AS (
			SELECT sm.*
			FROM stock_movements sm
			WHERE sm.project_id IS NULL OR EXISTS (
				SELECT 1 FROM projects p
				WHERE p.id=sm.project_id AND p.status <> 'cancelled' AND %s
			)
		), ledger AS (
			SELECT warehouse_id, material_id,
				CASE
					WHEN movement_type IN ('receive','return_to_warehouse') THEN quantity
					WHEN movement_type IN ('issue','transfer','loss') THEN -quantity
					ELSE 0
				END AS signed_quantity
			FROM visible_movements
			UNION ALL
			SELECT target_warehouse_id, material_id, quantity
			FROM visible_movements
			WHERE movement_type='transfer' AND target_warehouse_id IS NOT NULL
		)
		SELECT w.id AS warehouse_id, w.name AS warehouse_name, w.type AS warehouse_type, w.location,
			m.id AS material_id, m.code, m.name, m.unit, m.category,
			COALESCE(SUM(ledger.signed_quantity),0) AS stock_quantity
		FROM ledger
		JOIN warehouses w ON w.id=ledger.warehouse_id AND w.active
		JOIN materials m ON m.id=ledger.material_id AND m.active
		WHERE ($1::text='' OR w.id=$1::uuid)
		GROUP BY w.id, m.id
		ORDER BY w.type, w.name, m.category, m.name
	`, scopeSQL), args...)
}

func (r *Repository) StockMovements(ctx context.Context, userID, projectID, warehouseID string) ([]Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 3)
	if err != nil {
		return nil, err
	}
	args := append([]any{cleanID(projectID), cleanID(warehouseID)}, scopeArgs...)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT sm.*, m.code AS material_code, m.name AS material_name,
			w.name AS warehouse_name, tw.name AS target_warehouse_name,
			p.name AS project_name, u.full_name AS created_by_name,
			sd.document_code
		FROM stock_movements sm
		JOIN materials m ON m.id=sm.material_id AND m.active
		JOIN warehouses w ON w.id=sm.warehouse_id
		LEFT JOIN warehouses tw ON tw.id=sm.target_warehouse_id
		LEFT JOIN projects p ON p.id=sm.project_id
		LEFT JOIN users u ON u.id=sm.created_by
		LEFT JOIN stock_documents sd ON sd.id=sm.stock_document_id
		WHERE ($1::text='' OR sm.project_id=$1::uuid)
		  AND ($2::text='' OR sm.warehouse_id=$2::uuid OR sm.target_warehouse_id=$2::uuid)
		  AND (sm.project_id IS NULL OR (p.status <> 'cancelled' AND %s))
		ORDER BY sm.created_at DESC, sm.id
	`, scopeSQL), args...)
}

func (r *Repository) ListStockDocuments(ctx context.Context, userID, projectID string) ([]Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 2)
	if err != nil {
		return nil, err
	}
	args := append([]any{cleanID(projectID)}, scopeArgs...)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT sd.*, sw.name AS source_warehouse_name, tw.name AS target_warehouse_name,
			p.name AS project_name, u.full_name AS created_by_name,
			COALESCE(items.item_count,0) AS item_count,
			COALESCE(items.total_quantity,0) AS total_quantity
		FROM stock_documents sd
		LEFT JOIN warehouses sw ON sw.id=sd.source_warehouse_id
		LEFT JOIN warehouses tw ON tw.id=sd.target_warehouse_id
		LEFT JOIN projects p ON p.id=sd.project_id
		LEFT JOIN users u ON u.id=sd.created_by
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS item_count, SUM(quantity) AS total_quantity
			FROM stock_document_items sdi WHERE sdi.document_id=sd.id
		) items ON true
		WHERE sd.status <> 'cancelled'
		  AND ($1::text='' OR sd.project_id=$1::uuid)
		  AND (sd.project_id IS NULL OR (p.status <> 'cancelled' AND %s))
		ORDER BY sd.document_date DESC, sd.created_at DESC, sd.id
	`, scopeSQL), args...)
}

func (r *Repository) GetStockDocument(ctx context.Context, documentID string) (Record, error) {
	document, err := r.QueryOne(ctx, `
		SELECT sd.*, sw.name AS source_warehouse_name, tw.name AS target_warehouse_name,
			p.name AS project_name, u.full_name AS created_by_name
		FROM stock_documents sd
		LEFT JOIN warehouses sw ON sw.id=sd.source_warehouse_id
		LEFT JOIN warehouses tw ON tw.id=sd.target_warehouse_id
		LEFT JOIN projects p ON p.id=sd.project_id
		LEFT JOIN users u ON u.id=sd.created_by
		WHERE sd.id=$1 AND sd.status <> 'cancelled'
	`, documentID)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT sdi.*, m.code AS material_code, m.name AS material_name,
			sm.id AS movement_id, sm.movement_code
		FROM stock_document_items sdi
		JOIN materials m ON m.id=sdi.material_id
		LEFT JOIN LATERAL (
			SELECT candidate.id, candidate.movement_code
			FROM stock_movements candidate
			WHERE candidate.stock_document_id=sdi.document_id
			  AND candidate.material_id=sdi.material_id
			  AND (
				(candidate.reference_type='stock_document_item' AND candidate.reference_id=sdi.id)
				OR (sdi.source_line_id IS NOT NULL AND candidate.reference_id=sdi.source_line_id)
			  )
			ORDER BY candidate.created_at, candidate.id
			LIMIT 1
		) sm ON true
		WHERE sdi.document_id=$1
		ORDER BY sdi.created_at, sdi.id
	`, documentID)
	if err != nil {
		return nil, err
	}
	document["items"] = items
	return document, nil
}

func (r *Repository) WarehouseReceiptOptions(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT iar.id AS incoming_acceptance_id, iar.acceptance_code, iar.procurement_request_id,
			pr.request_code, iar.supplier_name, iar.received_at, iar.result, iar.warehouse_id,
			iai.id AS incoming_acceptance_item_id, iai.material_id, iai.material_code, iai.material_name,
			iai.accepted_quantity, iai.unit,
			LEAST(iai.accepted_quantity,COALESCE(received.received_quantity,0)+COALESCE(legacy_stock.received_quantity,0)) AS stock_received_quantity,
			GREATEST(iai.accepted_quantity-COALESCE(received.received_quantity,0)-COALESCE(legacy_stock.received_quantity,0),0) AS remaining_quantity
		FROM incoming_acceptance_records iar
		JOIN incoming_acceptance_items iai ON iai.incoming_acceptance_id=iar.id
		LEFT JOIN procurement_request_items pri ON pri.id=iai.procurement_request_item_id
		LEFT JOIN procurement_requests pr ON pr.id=iar.procurement_request_id
		LEFT JOIN LATERAL (
			SELECT SUM(sm.quantity) AS received_quantity
			FROM stock_movements sm
			WHERE sm.movement_type='receive'
			  AND sm.reference_type='incoming_acceptance_item'
			  AND sm.reference_id=iai.id
		) received ON true
		LEFT JOIN LATERAL (
			SELECT SUM(sm.quantity) AS received_quantity
			FROM stock_movements sm
			WHERE sm.movement_type='receive'
			  AND sm.reference_type='boq_item'
			  AND sm.reference_id=pri.boq_item_id
		) legacy_stock ON true
		WHERE iar.project_id=$1 AND iar.status='submitted' AND iai.material_id IS NOT NULL
		  AND iai.accepted_quantity-COALESCE(received.received_quantity,0)-COALESCE(legacy_stock.received_quantity,0) > 0.000001
		ORDER BY iar.received_at DESC NULLS LAST, iar.created_at DESC, iai.item_index
	`, projectID)
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

func (r *Repository) UpdateDailyReport(ctx context.Context, id string, body Record) (Record, error) {
	return r.Update(ctx, "operation_daily_reports", dailyReportColumns(), id, body)
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

func (r *Repository) UpdateIssue(ctx context.Context, id string, body Record, actorID, actorName string) (Record, error) {
	eventFor := func(row Record) auditEvent {
		return auditEvent{
			ProjectID: cleanID(row["project_id"]), ActorID: actorID, ActorName: actorName,
			EventType: "issue_updated", TargetType: "issue", TargetID: cleanID(row["id"]),
			Description: fmt.Sprintf("Cập nhật vấn đề %s", row["title"]),
			Metadata:    Record{"status": row["status"], "severity": row["severity"]},
		}
	}
	if _, changesStatus := body["status"]; changesStatus {
		return r.updateWithAudit(ctx, "issues", issueColumns(), id, body, eventFor)
	}
	row, err := r.Update(ctx, "issues", issueColumns(), id, body)
	if err != nil {
		return nil, err
	}
	event := eventFor(row)
	r.AddAuditBestEffort(ctx, event.ProjectID, event.ActorID, event.ActorName, event.EventType, event.TargetType, event.TargetID, event.Description, event.Metadata)
	return row, nil
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

func (r *Repository) UpdateReworkStatus(ctx context.Context, id, status, actorID, actorName string) (Record, error) {
	eventType := "rework_approved"
	if status == "done" {
		eventType = "rework_completed"
	}
	return r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		rows, err := queryTx(ctx, tx, `
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
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		return rows[0], nil
	}, func(row Record) auditEvent {
		return auditEvent{
			ProjectID: cleanID(row["project_id"]), ActorID: actorID, ActorName: actorName,
			EventType: eventType, TargetType: "rework",
			Description: fmt.Sprintf("Cập nhật sửa lỗi sang %s", status), Metadata: Record{"status": status},
		}
	})
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

func (r *Repository) ApproveReplanning(ctx context.Context, id, actorID, actorName string) (Record, error) {
	return r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		rows, err := queryTx(ctx, tx, `
			UPDATE replanning_requests
			SET status='approved', approved_by=NULLIF($2,'')::uuid, updated_at=now()
			WHERE id=$1
			RETURNING *
		`, id, actorID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		return rows[0], nil
	}, func(row Record) auditEvent {
		return auditEvent{
			ProjectID: cleanID(row["project_id"]), ActorID: actorID, ActorName: actorName,
			EventType: "replanning_approved", TargetType: "replanning",
			Description: "Duyệt yêu cầu lập kế hoạch lại", Metadata: Record{"status": row["status"]},
		}
	})
}

func (r *Repository) ListProjectHandover(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT h.*,
			COALESCE(items.total_items,0) AS checklist_total,
			COALESCE(items.verified_items,0) AS checklist_verified,
			COALESCE(items.pending_required_items,0) AS checklist_pending_required
		FROM handover_records h
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS total_items,
				COUNT(*) FILTER (WHERE status IN ('verified','waived')) AS verified_items,
				COUNT(*) FILTER (WHERE required AND status <> 'verified') AS pending_required_items
			FROM handover_checklist_items i WHERE i.handover_id=h.id
		) items ON true
		WHERE h.project_id=$1 AND h.status <> 'deleted'
		ORDER BY h.created_at DESC
	`, projectID)
}

func (r *Repository) HandoverOptions(ctx context.Context, projectID string) (Record, error) {
	acceptances, err := r.Query(ctx, `
		SELECT id,title,result,acceptance_date,client_representative,contractor_representative
		FROM acceptance_records
		WHERE project_id=$1 AND type='final' AND status='completed' AND result IN ('pass','conditional_pass')
		ORDER BY acceptance_date DESC NULLS LAST,updated_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	return Record{"final_acceptances": acceptances}, nil
}

func (r *Repository) CreateHandover(ctx context.Context, body Record) (Record, error) {
	return r.Insert(ctx, "handover_records", handoverColumns(), body)
}

func (r *Repository) CreateHandoverPackage(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO handover_records (
			project_id,handover_code,status,final_acceptance_id,workflow_form_instance_id,handover_date,
			client_representative,company_representative,documents_ready,warehouse_closed,
			payables_recorded,receivables_recorded,notes,created_by
		) VALUES ($1,$2,'draft',NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,NULLIF($5,'')::date,$6,$7,false,false,false,false,NULLIF($8,''),NULLIF($9,'')::uuid)
		RETURNING *
	`, body["project_id"], body["handover_code"], cleanID(body["final_acceptance_id"]), cleanID(body["workflow_form_instance_id"]), body["handover_date"], body["client_representative"], body["company_representative"], body["notes"], actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	handover := rows[0]
	items := []Record{
		{"item_code": "final_acceptance", "category": "quality", "title": "Biên bản nghiệm thu cuối đạt", "source_type": "acceptance", "source_id": cleanID(body["final_acceptance_id"])},
		{"item_code": "warehouse_closure", "category": "operation", "title": "Kho công trình và vật tư dư đã đóng", "source_type": "system_check", "source_id": handover["id"]},
		{"item_code": "project_material_costs", "category": "finance", "title": "Chi phí vật tư công trình đã được xác nhận", "source_type": "system_check", "source_id": handover["id"]},
		{"item_code": "project_cash_requests", "category": "finance", "title": "Đề nghị thu/chi nội bộ đã được xử lý", "source_type": "system_check", "source_id": handover["id"]},
		{"item_code": "as_built_documents", "category": "as_built", "title": "Bản vẽ và hồ sơ hoàn công"},
		{"item_code": "operation_manuals", "category": "operation", "title": "Hướng dẫn vận hành/bảo trì"},
		{"item_code": "warranty_documents", "category": "warranty", "title": "Cam kết và hồ sơ bảo hành"},
		{"item_code": "signed_handover_minutes", "category": "legal", "title": "Biên bản bàn giao có xác nhận hai bên"},
	}
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO handover_checklist_items (handover_id,project_id,item_code,category,title,required,status,source_type,source_id)
			VALUES ($1,$2,$3,$4,$5,true,'pending',NULLIF($6,''),NULLIF($7,'')::uuid)
		`, handover["id"], handover["project_id"], item["item_code"], item["category"], item["title"], item["source_type"], cleanID(item["source_id"])); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, cleanID(handover["project_id"]), actorID, actorName, "handover_created", "handover", cleanID(handover["id"]), "Tạo hồ sơ bàn giao", Record{"handover_code": handover["handover_code"], "final_acceptance_id": handover["final_acceptance_id"], "checklist_items": len(items)}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetHandoverDetail(ctx, cleanID(handover["id"]))
}

func (r *Repository) GetHandoverDetail(ctx context.Context, id string) (Record, error) {
	handover, err := r.QueryOne(ctx, `
		SELECT h.*, a.title AS final_acceptance_title, a.status AS final_acceptance_status, a.result AS final_acceptance_result
		FROM handover_records h
		LEFT JOIN acceptance_records a ON a.id=h.final_acceptance_id AND a.project_id=h.project_id
		WHERE h.id=$1 AND h.status <> 'deleted'
	`, id)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT i.*,
			e.name AS evidence_name,
			e.status AS evidence_status,
			e.evidence_type,
			e.mime_type AS evidence_mime_type,
			e.file_size AS evidence_file_size,
			e.storage_provider AS evidence_storage_provider,
			d.name AS document_name,
			d.status AS document_status,
			d.document_type,
			d.file_url AS document_file_url
		FROM handover_checklist_items i
		LEFT JOIN evidence_records e ON e.id=i.evidence_id
		LEFT JOIN documents d ON d.id=i.document_id
		WHERE i.handover_id=$1
		ORDER BY CASE i.category WHEN 'quality' THEN 10 WHEN 'as_built' THEN 20 WHEN 'operation' THEN 30 WHEN 'warranty' THEN 40 WHEN 'finance' THEN 50 WHEN 'legal' THEN 60 ELSE 70 END, i.created_at
	`, id)
	if err != nil {
		return nil, err
	}
	evidenceOptions, err := r.Query(ctx, `
		SELECT id,name,evidence_type,status,work_item_id,created_at
		FROM evidence_records
		WHERE project_id=$1 AND status='accepted'
		ORDER BY created_at DESC LIMIT 100
	`, handover["project_id"])
	if err != nil {
		return nil, err
	}
	documentOptions, err := r.Query(ctx, `
		SELECT id,name,document_type,file_url,created_at
		FROM documents
		WHERE project_id=$1 AND status='active'
		ORDER BY created_at DESC LIMIT 100
	`, handover["project_id"])
	if err != nil {
		return nil, err
	}
	handover["checklist_items"] = items
	handover["evidence_options"] = evidenceOptions
	handover["document_options"] = documentOptions
	return handover, nil
}

func (r *Repository) ListProjectBudget(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT *
		FROM accounting_budget_balances
		WHERE project_id=$1 AND status <> 'cancelled'
		ORDER BY created_at DESC
	`, projectID)
}

func (r *Repository) CreateBudget(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	return r.insertWithAudit(ctx, "accounting_budgets", budgetColumns(), body, func(row Record) auditEvent {
		return auditEvent{
			ProjectID: cleanID(row["project_id"]), ActorID: actorID, ActorName: actorName,
			EventType: "budget_created", TargetType: "budget",
			Description: fmt.Sprintf("Tạo ngân sách %s", row["budget_code"]),
			Metadata:    Record{"status": row["status"], "planned_amount": row["planned_amount"]},
		}
	})
}

func (r *Repository) ApproveBudget(ctx context.Context, id, actorID, actorName string) (Record, error) {
	return r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		rows, err := queryTx(ctx, tx, `
			UPDATE accounting_budgets
			SET status='approved',
				approved_amount=planned_amount,
				remaining_amount=GREATEST(planned_amount-used_amount, 0),
				approved_by=NULLIF($2,'')::uuid,
				updated_at=now()
			WHERE id=$1 AND status IN ('draft','submitted')
			RETURNING *
		`, id, actorID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		return rows[0], nil
	}, func(row Record) auditEvent {
		return auditEvent{
			ProjectID: cleanID(row["project_id"]), ActorID: actorID, ActorName: actorName,
			EventType: "budget_approved", TargetType: "budget",
			Description: fmt.Sprintf("Duyệt ngân sách %s", row["budget_code"]),
			Metadata:    Record{"status": row["status"], "approved_amount": row["approved_amount"]},
		}
	})
}

func (r *Repository) CreatePayment(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	return r.insertWithAudit(ctx, "payments", paymentColumns(), body, func(row Record) auditEvent {
		return auditEvent{
			ProjectID: cleanID(row["project_id"]), ActorID: actorID, ActorName: actorName,
			EventType: "payment_created", TargetType: "payment",
			Description: fmt.Sprintf("Tạo phiếu thanh toán %s", row["payment_code"]),
			Metadata:    Record{"status": row["status"], "payment_type": row["payment_type"], "amount": row["amount"]},
		}
	})
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
			if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "rework_requested", "issue", fmt.Sprint(issueRows[0]["id"]), "Nghiệm thu không đạt tạo danh sách lỗi/sửa lỗi", Record{"acceptance_id": id}); err != nil {
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
		return nil, validationError("Chỉ tạo sửa lỗi từ nghiệm thu không đạt", Record{"status": acc["status"], "result": acc["result"]})
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
		return nil, validationError("Nghiệm thu này đã có yêu cầu sửa lỗi đang mở", Record{"acceptance_id": acceptanceID})
	}
	reworkRows, err := queryTx(ctx, tx, `
		INSERT INTO rework_requests
			(project_id, acceptance_id, node_id, title, description, status, created_by, approved_by)
		VALUES ($1,$2,$3,$4,$5,'approved',NULLIF($6,'')::uuid,NULLIF($6,'')::uuid)
		RETURNING *
	`, acc["project_id"], acceptanceID, acc["node_id"], fmt.Sprintf("Sửa lỗi sau nghiệm thu: %s", acc["title"]), acc["notes"], actorID)
	if err != nil {
		return nil, err
	}
	if len(reworkRows) == 0 {
		return nil, sql.ErrNoRows
	}
	rework := reworkRows[0]
	if err := addAuditTx(ctx, tx, fmt.Sprint(acc["project_id"]), actorID, actorName, "rework_requested", "rework_request", fmt.Sprint(rework["id"]), "Tạo yêu cầu sửa lỗi từ nghiệm thu không đạt", Record{"acceptance_id": acceptanceID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rework, nil
}

func (r *Repository) SubmitHandover(ctx context.Context, id string, req ActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM handover_records WHERE id=$1 AND status <> 'deleted' FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	handover := rows[0]
	if !statusIn(fmt.Sprint(handover["status"]), "draft", "blocked") {
		return nil, validationError("Hồ sơ bàn giao không ở trạng thái có thể gửi", Record{"status": handover["status"]})
	}
	if cleanID(handover["final_acceptance_id"]) == "" || strings.TrimSpace(fmt.Sprint(handover["client_representative"])) == "" || strings.TrimSpace(fmt.Sprint(handover["company_representative"])) == "" {
		return nil, validationError("Cần khóa nghiệm thu cuối và đại diện hai bên trước khi gửi bàn giao")
	}
	validAcceptance, err := queryTx(ctx, tx, `
		SELECT COUNT(*) AS total FROM acceptance_records
		WHERE id=$1 AND project_id=$2 AND type='final' AND status='completed' AND result IN ('pass','conditional_pass')
	`, handover["final_acceptance_id"], handover["project_id"])
	if err != nil {
		return nil, err
	}
	if len(validAcceptance) == 0 || number(validAcceptance[0]["total"]) == 0 {
		return nil, validationError("Nghiệm thu cuối được liên kết chưa đạt điều kiện bàn giao")
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE handover_records SET status='submitted',submitted_by=NULLIF($2,'')::uuid,submitted_at=now(),updated_at=now()
		WHERE id=$1 RETURNING *
	`, id, actorID)
	if err != nil {
		return nil, err
	}
	comment := actionComment(req)
	if err := addAuditTx(ctx, tx, cleanID(handover["project_id"]), actorID, actorName, "handover_submitted", "handover", id, "Gửi hồ sơ bàn giao để kiểm tra", Record{"comment": comment, "final_acceptance_id": handover["final_acceptance_id"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetHandoverDetail(ctx, cleanID(updated[0]["id"]))
}

func (r *Repository) UpdateHandoverChecklistItem(ctx context.Context, id string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT i.*,h.status AS handover_status FROM handover_checklist_items i
		JOIN handover_records h ON h.id=i.handover_id AND h.project_id=i.project_id
		WHERE i.id=$1 FOR UPDATE
	`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	item := rows[0]
	if statusIn(fmt.Sprint(item["handover_status"]), "handed_over", "cancelled", "deleted") {
		return nil, validationError("Hồ sơ bàn giao đã khóa", Record{"status": item["handover_status"]})
	}
	if strings.TrimSpace(fmt.Sprint(item["source_type"])) == "system_check" || fmt.Sprint(item["item_code"]) == "final_acceptance" {
		return nil, validationError("Mục checklist này được xác minh tự động từ nguồn nghiệp vụ")
	}
	status := strings.TrimSpace(fmt.Sprint(body["status"]))
	if status == "waived" && item["required"] == true {
		return nil, validationError("Mục bắt buộc không thể miễn")
	}
	evidenceID := cleanID(body["evidence_id"])
	documentID := cleanID(body["document_id"])
	if status == "verified" && evidenceID == "" && documentID == "" {
		return nil, validationError("Mục đã xác minh cần evidence hoặc tài liệu")
	}
	if evidenceID != "" {
		valid, err := queryTx(ctx, tx, `SELECT COUNT(*) AS total FROM evidence_records WHERE id=$1 AND project_id=$2 AND status='accepted'`, evidenceID, item["project_id"])
		if err != nil || len(valid) == 0 || number(valid[0]["total"]) == 0 {
			return nil, validationError("Evidence chưa được chấp nhận hoặc không thuộc công trình", Record{"evidence_id": evidenceID})
		}
	}
	if documentID != "" {
		valid, err := queryTx(ctx, tx, `SELECT COUNT(*) AS total FROM documents WHERE id=$1 AND project_id=$2 AND status='active'`, documentID, item["project_id"])
		if err != nil || len(valid) == 0 || number(valid[0]["total"]) == 0 {
			return nil, validationError("Tài liệu không hoạt động hoặc không thuộc công trình", Record{"document_id": documentID})
		}
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE handover_checklist_items
		SET status=$2,evidence_id=NULLIF($3,'')::uuid,document_id=NULLIF($4,'')::uuid,notes=NULLIF($5,''),
			verified_by=CASE WHEN $2 IN ('verified','waived') THEN NULLIF($6,'')::uuid ELSE NULL END,
			verified_at=CASE WHEN $2 IN ('verified','waived') THEN now() ELSE NULL END,updated_at=now()
		WHERE id=$1 RETURNING *
	`, id, status, evidenceID, documentID, body["notes"], actorID)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(item["project_id"]), actorID, actorName, "handover_checklist_updated", "handover_checklist_item", id, "Cập nhật checklist bàn giao", Record{"handover_id": item["handover_id"], "item_code": item["item_code"], "from_status": item["status"], "to_status": status, "evidence_id": evidenceID, "document_id": documentID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated[0], nil
}

func (r *Repository) HandoverCheck(ctx context.Context, id, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM handover_records WHERE id=$1 AND status <> 'deleted' FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	handover := rows[0]
	if !statusIn(fmt.Sprint(handover["status"]), "submitted", "ready", "blocked") {
		return nil, validationError("Cần gửi hồ sơ bàn giao trước khi kiểm tra readiness", Record{"status": handover["status"]})
	}
	projectID := cleanID(handover["project_id"])
	counts := Record{}
	checkCount := func(key, query string, args ...any) error {
		result, err := queryTx(ctx, tx, query, args...)
		if err != nil {
			return err
		}
		if len(result) > 0 {
			counts[key] = number(result[0]["total"])
			if amount, ok := result[0]["amount"]; ok {
				counts[key+"_amount"] = number(amount)
			}
		}
		return nil
	}
	if err := checkCount("final_acceptance", `SELECT COUNT(*) AS total FROM acceptance_records WHERE id=$1 AND project_id=$2 AND type='final' AND status='completed' AND result IN ('pass','conditional_pass')`, handover["final_acceptance_id"], projectID); err != nil {
		return nil, err
	}
	if err := checkCount("open_issues", `SELECT COUNT(*) AS total FROM issues WHERE project_id=$1 AND status IN ('open','in_progress')`, projectID); err != nil {
		return nil, err
	}
	if err := checkCount("open_ncrs", `SELECT COUNT(*) AS total FROM quality_ncrs WHERE project_id=$1 AND status <> 'closed'`, projectID); err != nil {
		return nil, err
	}
	if err := checkCount("open_reworks", `SELECT COUNT(*) AS total FROM rework_requests WHERE project_id=$1 AND status NOT IN ('done','completed','cancelled','deleted')`, projectID); err != nil {
		return nil, err
	}
	if err := checkCount("open_construction_actions", `SELECT COUNT(*) AS total FROM construction_action_items WHERE project_id=$1 AND status IN ('open','waiting_confirmation')`, projectID); err != nil {
		return nil, err
	}
	if err := checkCount("pending_surplus", `SELECT COUNT(*) AS total FROM site_material_balances WHERE project_id=$1 AND surplus_quantity > 0 AND surplus_status <> 'handled'`, projectID); err != nil {
		return nil, err
	}
	if err := checkCount("unconfirmed_material_costs", `SELECT COUNT(*) AS total,COALESCE(SUM(total_amount),0) AS amount FROM project_material_costs WHERE project_id=$1 AND status IN ('draft','submitted','needs_changes')`, projectID); err != nil {
		return nil, err
	}
	if err := checkCount("pending_cash_requests", `SELECT COUNT(*) AS total,COALESCE(SUM(amount),0) AS amount FROM project_cash_requests WHERE project_id=$1 AND status NOT IN ('processed','cancelled')`, projectID); err != nil {
		return nil, err
	}

	costsReady := number(counts["unconfirmed_material_costs"]) == 0
	cashRequestsReady := number(counts["pending_cash_requests"]) == 0
	systemStatuses := map[string]bool{
		"final_acceptance":       number(counts["final_acceptance"]) > 0,
		"warehouse_closure":      number(counts["pending_surplus"]) == 0,
		"project_material_costs": costsReady,
		"project_cash_requests":  cashRequestsReady,
		// Compatibility for handover packages created before the project-finance boundary changed.
		"ap_reconciliation": costsReady,
		"ar_reconciliation": cashRequestsReady,
	}
	for itemCode, passed := range systemStatuses {
		status := "pending"
		if passed {
			status = "verified"
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE handover_checklist_items SET status=$2,verified_by=CASE WHEN $2='verified' THEN NULLIF($3,'')::uuid ELSE NULL END,
				verified_at=CASE WHEN $2='verified' THEN now() ELSE NULL END,updated_at=now()
			WHERE handover_id=$1 AND item_code=$4
		`, id, status, actorID, itemCode); err != nil {
			return nil, err
		}
	}

	blockers := []string{}
	if number(counts["final_acceptance"]) == 0 {
		blockers = append(blockers, "Nghiệm thu cuối được liên kết chưa đạt")
	}
	if number(counts["open_issues"]) > 0 {
		blockers = append(blockers, "Còn danh sách lỗi/vấn đề chưa đóng")
	}
	if number(counts["open_ncrs"]) > 0 {
		blockers = append(blockers, "Còn NCR chưa đóng")
	}
	if number(counts["open_reworks"]) > 0 {
		blockers = append(blockers, "Còn rework chưa hoàn tất")
	}
	if number(counts["open_construction_actions"]) > 0 {
		blockers = append(blockers, "Còn việc cần xử lý tại công trường chưa hoàn tất")
	}
	if number(counts["pending_surplus"]) > 0 {
		blockers = append(blockers, "Còn vật tư dư chưa xử lý")
	}
	if !costsReady {
		blockers = append(blockers, "Còn chi phí vật tư chưa được xác nhận")
	}
	if !cashRequestsReady {
		blockers = append(blockers, "Còn đề nghị thu/chi nội bộ chưa được xử lý")
	}
	pendingItems, err := queryTx(ctx, tx, `SELECT item_code,title FROM handover_checklist_items WHERE handover_id=$1 AND required AND status <> 'verified' ORDER BY created_at`, id)
	if err != nil {
		return nil, err
	}
	for _, item := range pendingItems {
		blockers = append(blockers, "Thiếu/xác minh chưa đạt: "+fmt.Sprint(item["title"]))
	}
	ready := len(blockers) == 0
	nextStatus := "blocked"
	if ready {
		nextStatus = "ready"
	}
	snapshot := Record{"ready": ready, "blockers": blockers, "counts": counts, "checked_at": time.Now().UTC().Format(time.RFC3339)}
	rawSnapshot, _ := json.Marshal(snapshot)
	updated, err := queryTx(ctx, tx, `
		UPDATE handover_records SET status=$2,documents_ready=$3,warehouse_closed=$4,payables_recorded=$5,receivables_recorded=$6,
			readiness_snapshot_json=$7::jsonb,checked_by=NULLIF($8,'')::uuid,checked_at=now(),updated_at=now()
		WHERE id=$1 RETURNING *
	`, id, nextStatus, len(pendingItems) == 0, systemStatuses["warehouse_closure"], costsReady, cashRequestsReady, string(rawSnapshot), actorID)
	if err != nil {
		return nil, err
	}
	event := "handover_ready"
	if !ready {
		event = "handover_blocked"
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, event, "handover", id, "Kiểm tra readiness hồ sơ bàn giao", snapshot); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result := updated[0]
	result["ready"] = ready
	result["blockers"] = blockers
	result["counts"] = counts
	return result, nil
}

func (r *Repository) CompleteHandover(ctx context.Context, id string, req ActionRequest, actorID, actorName string) (Record, error) {
	check, err := r.HandoverCheck(ctx, id, actorID, actorName)
	if err != nil {
		return nil, err
	}
	blockers, _ := check["blockers"].([]string)
	comment := actionComment(req)
	if len(blockers) > 0 {
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
			completed_at=now(),
			updated_at=now()
		WHERE id=$1 AND status='ready'
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
		return nil, validationError("Không được tất toán trực tiếp khoản phải trả; hãy lập và duyệt Phiếu chi để bảo toàn phân bổ công nợ, dòng tiền và bút toán")
	default:
		return nil, validationError("Hành động phải trả không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái phải trả đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	if action == "approve" {
		if err := r.recordPayableCommitmentTx(ctx, tx, updated[0], actorID); err != nil {
			return nil, err
		}
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
					ELSE 'Tải lên hoá đơn đầu vào'
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
		return nil, validationError("Hành động kỳ thanh toán không hợp lệ", Record{"action": action})
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
		return nil, validationError("Hành động ứng trước không hợp lệ", Record{"action": action})
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
			amount, payment_method, status, paid_at, created_by,
			accounting_book_id, budget_id, cash_account_id
		)
		VALUES (
			$1,$2,$3,$4,'payable',$5,$6,$7,'draft',NULLIF($8,'')::timestamptz,NULLIF($9,'')::uuid,
			NULLIF($10,'')::uuid,NULLIF($11,'')::uuid,NULLIF($12,'')::uuid
		)
		RETURNING *
	`, paymentID, projectID, body["payment_code"], body["payment_type"], payableID, amount, body["payment_method"], fmt.Sprint(body["paid_at"]), actorID,
		body["accounting_book_id"], body["budget_id"], body["cash_account_id"])
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
	return getInputInvoiceDetail(ctx, r.db, invoiceID)
}

func getInputInvoiceDetail(ctx context.Context, db queryContext, invoiceID string) (Record, error) {
	return queryOneRecord(ctx, db, `
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

func (r *Repository) CreateInputInvoice(ctx context.Context, payable Record, body Record, file StoredFileMetadata, actorID, actorName string) (result Record, err error) {
	tracker := storedMutationTracker{}
	defer tracker.wrap(&err)
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
			uploaded_by, accounting_book_id, tax_code_id, expense_account_id
		)
		VALUES (
			$1, $2, $3, NULLIF($4,'')::uuid, NULLIF($5,'')::uuid,
			$6, $7, $8::date, NULLIF($9,'')::uuid, $10, $11,
			$12, $13, $14, 'VND', 'draft', $15,
			$16, $17, $18, $19, $20, $21,
			NULLIF($22,'')::uuid,$23::uuid,$24::uuid,$25::uuid
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
		body["accounting_book_id"],
		body["tax_code_id"],
		body["expense_account_id"],
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := insertInputInvoiceTaxLineTx(ctx, tx, rows[0], actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE payables SET updated_at=now() WHERE id=$1`, payableID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "input_invoice_uploaded", "input_invoice", invoiceID, "Tải PDF hóa đơn đầu vào", Record{
		"payable_id":             payableID,
		"document_id":            documentID,
		"procurement_request_id": cleanID(body["procurement_request_id"]),
		"invoice_number":         body["invoice_number"],
		"total_amount":           body["total_amount"],
		"checksum_sha256":        file.ChecksumSHA256,
	}); err != nil {
		return nil, err
	}
	return finalizeStoredMutation(&tracker, func() (Record, error) {
		return getInputInvoiceDetail(ctx, tx, invoiceID)
	}, tx.Commit)
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
	if err := ensureVATSourcePeriodMutableTx(ctx, tx, cleanID(current["accounting_book_id"]), current["invoice_date"]); err != nil {
		return nil, err
	}
	fromStatus := fmt.Sprint(current["status"])
	toStatus := ""
	event := ""
	var updated []Record
	switch action {
	case "verify":
		if fromStatus != "draft" {
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
		if fromStatus == "verified" && actionComment(req) == "" {
			return nil, validationError("Lý do điều chỉnh hóa đơn đã ghi sổ là bắt buộc")
		}
		toStatus = "rejected"
		event = "input_invoice_rejected"
		updated, err = queryTx(ctx, tx, `
			UPDATE input_invoices
			SET status='rejected',
				rejected_by=NULLIF($2,'')::uuid,
				rejected_at=now(),
				verified_by=CASE WHEN $3='verified' THEN verified_by ELSE NULL END,
				verified_at=CASE WHEN $3='verified' THEN verified_at ELSE NULL END,
				updated_at=now()
			WHERE id=$1 AND status=$3
			RETURNING *
		`, invoiceID, actorID, fromStatus)
	default:
		return nil, validationError("Hành động hoá đơn đầu vào không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái hoá đơn đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	var journalEntry Record
	if action == "verify" {
		journalEntry, err = r.postInputInvoiceTx(ctx, tx, updated[0], actorID)
		if err != nil {
			return nil, err
		}
	}
	if action == "reject" && fromStatus == "verified" {
		journalEntry, err = r.reverseSourceJournalTx(
			ctx, tx, cleanID(current["project_id"]), "input_invoice", invoiceID,
			"input_invoice_verified", "input_invoice_rejected", actionComment(req), actorID,
		)
		if err != nil {
			return nil, err
		}
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": toStatus, "payable_id": current["payable_id"]}
	if comment != "" {
		metadata["comment"] = comment
	}
	if journalEntry != nil {
		metadata["journal_entry_id"] = journalEntry["id"]
		metadata["journal_entry_no"] = journalEntry["entry_no"]
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
		return nil, validationError("Hành động kỳ thu không hợp lệ", Record{"action": action})
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
		return nil, validationError("Hành động ứng trước đầu ra không hợp lệ", Record{"action": action})
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
			amount, payment_method, status, paid_at, created_by,
			accounting_book_id, cash_account_id
		)
		VALUES (
			$1,$2,$3,$4,'receivable',$5,$6,$7,'draft',NULLIF($8,'')::timestamptz,NULLIF($9,'')::uuid,
			NULLIF($10,'')::uuid,NULLIF($11,'')::uuid
		)
		RETURNING *
	`, paymentID, projectID, body["payment_code"], body["payment_type"], receivableID, amount, body["payment_method"], fmt.Sprint(body["paid_at"]), actorID,
		body["accounting_book_id"], body["cash_account_id"])
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
					ELSE 'Tải lên hoá đơn đầu ra'
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
	return getOutputInvoiceDetail(ctx, r.db, invoiceID)
}

func getOutputInvoiceDetail(ctx context.Context, db queryContext, invoiceID string) (Record, error) {
	return queryOneRecord(ctx, db, `
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

func (r *Repository) CreateOutputInvoice(ctx context.Context, receivable Record, body Record, file StoredFileMetadata, actorID, actorName string) (result Record, err error) {
	tracker := storedMutationTracker{}
	defer tracker.wrap(&err)
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
			uploaded_by, accounting_book_id, tax_code_id, revenue_account_id
		)
		VALUES (
			$1, $2, $3, NULLIF($4,'')::uuid, NULLIF($5,'')::uuid,
			$6, $7, $8::date, NULLIF($9,'')::uuid, $10, $11,
			$12, $13, $14, 'VND', 'draft', $15,
			$16, $17, $18, $19, $20, $21,
			NULLIF($22,'')::uuid,$23::uuid,$24::uuid,$25::uuid
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
		body["accounting_book_id"],
		body["tax_code_id"],
		body["revenue_account_id"],
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := insertOutputInvoiceTaxLineTx(ctx, tx, rows[0], actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE receivables SET updated_at=now() WHERE id=$1`, receivableID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "output_invoice_uploaded", "output_invoice", invoiceID, "Tải PDF hóa đơn đầu ra", Record{
		"receivable_id":   receivableID,
		"document_id":     documentID,
		"payment_id":      cleanID(body["payment_id"]),
		"invoice_number":  body["invoice_number"],
		"total_amount":    body["total_amount"],
		"checksum_sha256": file.ChecksumSHA256,
	}); err != nil {
		return nil, err
	}
	return finalizeStoredMutation(&tracker, func() (Record, error) {
		return getOutputInvoiceDetail(ctx, tx, invoiceID)
	}, tx.Commit)
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
	if err := ensureVATSourcePeriodMutableTx(ctx, tx, cleanID(current["accounting_book_id"]), current["invoice_date"]); err != nil {
		return nil, err
	}
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
		if fromStatus == "issued" && actionComment(req) == "" {
			return nil, validationError("Lý do hủy hóa đơn đã ghi sổ là bắt buộc")
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
		return nil, validationError("Hành động hoá đơn đầu ra không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái hoá đơn đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	var journalEntry Record
	if action == "issue" {
		journalEntry, err = r.postOutputInvoiceTx(ctx, tx, updated[0], actorID)
		if err != nil {
			return nil, err
		}
	}
	if action == "void" && fromStatus == "issued" {
		journalEntry, err = r.reverseSourceJournalTx(
			ctx, tx, cleanID(current["project_id"]), "output_invoice", invoiceID,
			"output_invoice_issued", "output_invoice_voided", actionComment(req), actorID,
		)
		if err != nil {
			return nil, err
		}
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": toStatus, "receivable_id": current["receivable_id"]}
	if comment != "" {
		metadata["comment"] = comment
	}
	if journalEntry != nil {
		metadata["journal_entry_id"] = journalEntry["id"]
		metadata["journal_entry_no"] = journalEntry["entry_no"]
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
		return nil, validationError("Không được tất toán trực tiếp khoản phải thu; hãy lập và duyệt Phiếu thu để bảo toàn phân bổ công nợ, dòng tiền và bút toán")
	default:
		return nil, validationError("Hành động phải thu không hợp lệ", Record{"action": action})
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
	if action == "reverse" {
		return r.ReverseApprovedPayment(ctx, id, req, actorID, actorName)
	}
	if action != "approve" {
		return nil, validationError("Hành động phiếu thanh toán không hợp lệ", Record{"action": action})
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
	if !statusIn(fromStatus, "draft", "submitted", "pending", "waiting_approval") {
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
	journalEntry, err := r.postPaymentTx(ctx, tx, updated[0], actorID)
	if err != nil {
		return nil, err
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": "approved"}
	if comment != "" {
		metadata["comment"] = comment
	}
	if journalEntry != nil {
		metadata["journal_entry_id"] = journalEntry["id"]
		metadata["journal_entry_no"] = journalEntry["entry_no"]
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
			return validationError("Phân bổ thanh toán không hợp lệ", Record{"allocation_id": allocation["id"]})
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
			return validationError("Loại phân bổ thanh toán không hợp lệ", Record{"allocation_type": allocation["allocation_type"]})
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
			return validationError("Phân bổ phiếu thu không hợp lệ", Record{"allocation_id": allocation["id"]})
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
			return validationError("Loại phân bổ phiếu thu không hợp lệ", Record{"allocation_type": allocation["allocation_type"]})
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
