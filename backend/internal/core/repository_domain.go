package core

import (
	"context"
	"database/sql"
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
		SELECT wi.*, p.name AS project_name, rn.name AS run_node_name, rs.name AS run_step_name,
			s.full_name AS assignee_name,
			COALESCE((SELECT COUNT(*) FROM evidence_records er WHERE er.work_item_id=wi.id AND er.status <> 'rejected'),0) AS evidence_count
		FROM work_items wi
		JOIN projects p ON p.id=wi.project_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		LEFT JOIN staff s ON s.id=wi.assignee_staff_id
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
		SELECT wi.*, p.name AS project_name, rn.name AS run_node_name, rs.name AS run_step_name,
			s.full_name AS assignee_name,
			COALESCE((SELECT COUNT(*) FROM evidence_records er WHERE er.work_item_id=wi.id AND er.status <> 'rejected'),0) AS evidence_count
		FROM work_items wi
		JOIN projects p ON p.id=wi.project_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		LEFT JOIN staff s ON s.id=wi.assignee_staff_id
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

func (r *Repository) GetWorkItemDetail(ctx context.Context, workItemID string) (Record, error) {
	item, err := r.QueryOne(ctx, `
		SELECT wi.*, p.name AS project_name, rn.name AS run_node_name, rs.name AS run_step_name,
			s.full_name AS assignee_name
		FROM work_items wi
		JOIN projects p ON p.id=wi.project_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		LEFT JOIN staff s ON s.id=wi.assignee_staff_id
		WHERE wi.id=$1 AND wi.status <> 'cancelled' AND p.status <> 'cancelled'
	`, workItemID)
	if err != nil {
		return nil, err
	}
	activities, _ := r.Query(ctx, `
		SELECT wia.*, u.full_name AS actor_name
		FROM work_item_activities wia
		LEFT JOIN users u ON u.id=wia.actor_id
		WHERE wia.work_item_id=$1
		ORDER BY wia.created_at DESC
		LIMIT 100
	`, workItemID)
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
		SELECT wi.*, p.name AS project_name, rn.name AS run_node_name, rs.name AS run_step_name,
			s.full_name AS assignee_name
		FROM work_items wi
		JOIN projects p ON p.id=wi.project_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		LEFT JOIN staff s ON s.id=wi.assignee_staff_id
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
			(work_item_id, project_id, target_type, target_id, evidence_type, document_type, name, document_id, title, description, file_url, mime_type, file_size, caption, revision_no, visibility, payload_json, status, uploaded_by, submitted_by)
		VALUES
			($1, $2, 'work_item', $1, $3, NULLIF($4,''), $5, NULLIF($6,'')::uuid, NULLIF($7,''), NULLIF($8,''), NULLIF($9,''), NULLIF($10,''), NULLIF($11,0), NULLIF($12,''), NULLIF($13,''), $14, $15::jsonb, 'submitted', NULLIF($16,'')::uuid, NULLIF($16,'')::uuid)
		RETURNING *
	`, workItemID, projectID, req.EvidenceType, req.DocumentType, req.Name, req.DocumentID, req.Title, req.Description, req.FileURL, req.MIMEType, req.FileSize, req.Caption, req.RevisionNo, req.Visibility, string(rawPayload), actorID)
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
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'comment', $4, $5::jsonb)
		RETURNING *
	`, workItemID, projectID, actorID, req.Comment, string(rawMetadata))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `UPDATE work_items SET updated_by=NULLIF($2,'')::uuid, updated_at=now() WHERE id=$1`, workItemID, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json)
		VALUES ($1, NULLIF($2,'')::uuid, $3, 'work_item_comment', 'work_item', $4, $5, $6::jsonb)
	`, projectID, actorID, actorName, workItemID, fmt.Sprintf("Ghi chú work item %s", item["title"]), string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
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
	args := append([]any{projectID}, workItemScopeArgs(profile, profile.IsManager, true)...)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT wia.*, wi.title AS work_item_title, u.full_name AS actor_name
		FROM work_item_activities wia
		JOIN work_items wi ON wi.id=wia.work_item_id
		LEFT JOIN users u ON u.id=wia.actor_id
		WHERE wi.project_id=$1 AND wi.status <> 'cancelled'
		  AND (%s)
		ORDER BY wia.created_at DESC
		LIMIT 50
	`, workItemScopeSQL("wi", 2)), args...)
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

func (r *Repository) ListProjects(ctx context.Context) ([]Record, error) {
	return r.Query(ctx, `
		SELECT p.*, s.full_name AS pm_name, t.name AS pipeline_template_name,
			COALESCE((SELECT COUNT(*) FROM issues i WHERE i.project_id=p.id AND i.status IN ('open','in_progress')),0) AS open_issues,
			COALESCE((SELECT COUNT(*) FROM approval_requests ar WHERE ar.project_id=p.id AND ar.status='pending'),0) AS pending_approvals
		FROM projects p
		LEFT JOIN staff s ON s.id=p.pm_staff_id
		LEFT JOIN pipeline_templates t ON t.id=p.pipeline_template_id
		WHERE p.status <> 'cancelled'
		ORDER BY p.created_at DESC
	`)
}

func (r *Repository) GetProjectDetail(ctx context.Context, id string) (Record, error) {
	project, err := r.QueryOne(ctx, `
		SELECT p.*, s.full_name AS pm_name, t.name AS pipeline_template_name
		FROM projects p
		LEFT JOIN staff s ON s.id=p.pm_staff_id
		LEFT JOIN pipeline_templates t ON t.id=p.pipeline_template_id
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
			SELECT * FROM pipeline_run_steps
			WHERE run_node_id=$1
			ORDER BY order_index, created_at
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

func (r *Repository) GenerateProcurementFromBOQ(ctx context.Context, projectID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "generate_procurement:"+projectID); err != nil {
		return nil, err
	}

	rows, err := queryTx(ctx, tx, `
		SELECT b.*, m.name AS material_name, m.standard_price
		FROM boq_items b
		LEFT JOIN materials m ON m.id=b.material_id
		WHERE b.project_id=$1 AND b.status='approved'
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
		price := number(row["unit_price"])
		if price == 0 {
			price = number(row["standard_price"])
		}
		total += number(row["quantity"]) * price
	}
	requestCode := fmt.Sprintf("PR-%s-%d", projectID[:8], timeNowUnix())
	reqRows, err := queryTx(ctx, tx, `
		INSERT INTO procurement_requests
			(project_id, request_code, title, requested_by, status, total_estimated_amount, budget_status)
		VALUES ($1,$2,'Yêu cầu mua vật tư sinh từ BOQ',NULLIF($3,'')::uuid,'submitted',$4,'within_budget')
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
		price := number(row["unit_price"])
		if price == 0 {
			price = number(row["standard_price"])
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO procurement_request_items
				(request_id, material_id, boq_item_id, quantity_planned, quantity_requested, unit, estimated_unit_price, supplier_name, delivery_status, approval_status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'pending','pending')
		`, requestID, row["material_id"], row["id"], row["quantity"], row["quantity"], row["unit"], price, row["default_supplier"]); err != nil {
			return nil, err
		}
	}
	rawMetadata, _ := json.Marshal(Record{"total": total})
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

func (r *Repository) ProcurementAction(ctx context.Context, id, action, actorID, actorName string) (Record, error) {
	status := map[string]string{
		"submit":  "waiting_approval",
		"approve": "approved",
		"order":   "ordered",
		"receive": "received",
	}[action]
	if status == "" {
		return nil, errors.New("invalid procurement action")
	}
	req, err := r.QueryOne(ctx, `UPDATE procurement_requests SET status=$2, updated_at=now() WHERE id=$1 RETURNING *`, id, status)
	if err != nil {
		return nil, err
	}
	event := map[string]string{"submit": "review_requested", "approve": "material_approved", "order": "material_ordered", "receive": "warehouse_received"}[action]
	_ = r.AddAudit(ctx, fmt.Sprint(req["project_id"]), actorID, actorName, event, "procurement_request", id, fmt.Sprintf("Cập nhật yêu cầu mua sang trạng thái %s", status), Record{})
	return r.GetProcurementDetail(ctx, id)
}

func (r *Repository) CreateStockMovement(ctx context.Context, req StockActionRequest, actorID, actorName string) (Record, error) {
	if req.MovementType == "" {
		req.MovementType = "receive"
	}
	code := fmt.Sprintf("SM-%s-%d", req.MovementType, timeNowUnix())
	body := Record{
		"movement_code":  code,
		"project_id":     req.ProjectID,
		"warehouse_id":   req.WarehouseID,
		"material_id":    req.MaterialID,
		"movement_type":  req.MovementType,
		"quantity":       req.Quantity,
		"unit":           req.Unit,
		"reason":         req.Reason,
		"reference_type": req.ReferenceType,
		"reference_id":   req.ReferenceID,
		"created_by":     actorID,
		"approved_by":    actorID,
	}
	movement, err := r.Insert(ctx, "stock_movements", []ColumnSpec{
		col("movement_code"), col("project_id"), col("warehouse_id"), col("material_id"), col("movement_type"), col("quantity"), col("unit"), col("reason"), col("reference_type"), col("reference_id"), col("created_by"), col("approved_by"),
	}, body)
	if err != nil {
		return nil, err
	}
	if req.ProjectID != "" && (req.MovementType == "issue" || req.MovementType == "return_to_warehouse" || req.MovementType == "return_to_subcontractor") {
		issuedDelta := req.Quantity
		if req.MovementType != "issue" {
			issuedDelta = -req.Quantity
		}
		_, _ = r.db.ExecContext(ctx, `
			INSERT INTO site_material_balances (project_id, material_id, issued_quantity, surplus_quantity, surplus_status)
			VALUES ($1,$2,$3,GREATEST($3,0),'pending')
			ON CONFLICT (project_id, material_id)
			DO UPDATE SET issued_quantity = site_material_balances.issued_quantity + EXCLUDED.issued_quantity,
				surplus_quantity = GREATEST(site_material_balances.issued_quantity + EXCLUDED.issued_quantity - site_material_balances.used_quantity, 0),
				surplus_status = CASE WHEN $4 IN ('return_to_warehouse','return_to_subcontractor') THEN 'handled' ELSE site_material_balances.surplus_status END,
				updated_at=now()
		`, req.ProjectID, req.MaterialID, issuedDelta, req.MovementType)
	}
	event := map[string]string{
		"receive":                 "warehouse_received",
		"issue":                   "warehouse_issued",
		"transfer":                "warehouse_transferred",
		"return_to_warehouse":     "surplus_stored",
		"return_to_subcontractor": "surplus_returned_to_subcontractor",
	}[req.MovementType]
	if event == "" {
		event = "stock_movement_created"
	}
	_ = r.AddAudit(ctx, req.ProjectID, actorID, actorName, event, "stock_movement", fmt.Sprint(movement["id"]), req.Reason, Record{"quantity": req.Quantity, "material_id": req.MaterialID})
	return movement, nil
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

func (r *Repository) AcceptanceAction(ctx context.Context, id, result, actorID, actorName string) (Record, error) {
	status := "completed"
	event := "acceptance_passed"
	if result == "fail" {
		event = "acceptance_failed"
	}
	acc, err := r.QueryOne(ctx, `UPDATE acceptance_records SET status=$2, result=$3, approved_by=NULLIF($4,'')::uuid, updated_at=now() WHERE id=$1 RETURNING *`, id, status, result, actorID)
	if err != nil {
		return nil, err
	}
	projectID := fmt.Sprint(acc["project_id"])
	if result == "fail" {
		issue, _ := r.Insert(ctx, "issues", issueColumns(), Record{
			"project_id":          projectID,
			"node_id":             acc["node_id"],
			"title":               fmt.Sprintf("Punch list từ nghiệm thu: %s", acc["title"]),
			"description":         acc["notes"],
			"severity":            "high",
			"status":              "open",
			"reported_by":         actorID,
			"requires_rework":     true,
			"requires_replanning": false,
		})
		_ = r.AddAudit(ctx, projectID, actorID, actorName, "rework_requested", "issue", fmt.Sprint(issue["id"]), "Nghiệm thu fail tạo punch list/rework", Record{"acceptance_id": id})
	}
	_ = r.AddAudit(ctx, projectID, actorID, actorName, event, "acceptance", id, fmt.Sprintf("Nghiệm thu %s: %s", result, acc["title"]), Record{})
	return acc, nil
}

func (r *Repository) CreateReworkFromAcceptance(ctx context.Context, acceptanceID, actorID, actorName string) (Record, error) {
	acc, err := r.GetByID(ctx, "acceptance_records", acceptanceID)
	if err != nil {
		return nil, err
	}
	rework, err := r.Insert(ctx, "rework_requests", reworkColumns(), Record{
		"project_id":    acc["project_id"],
		"acceptance_id": acceptanceID,
		"node_id":       acc["node_id"],
		"title":         fmt.Sprintf("Rework sau nghiệm thu: %s", acc["title"]),
		"description":   acc["notes"],
		"status":        "approved",
		"created_by":    actorID,
		"approved_by":   actorID,
	})
	if err != nil {
		return nil, err
	}
	_ = r.AddAudit(ctx, fmt.Sprint(acc["project_id"]), actorID, actorName, "rework_requested", "rework_request", fmt.Sprint(rework["id"]), "Tạo rework request từ nghiệm thu fail", Record{"acceptance_id": acceptanceID})
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
		WHERE project_id=$1 AND type='final' AND result IN ('pass','conditional_pass')
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
	handover["blockers"] = blockers
	handover["ready"] = len(blockers) == 0
	return handover, nil
}

func (r *Repository) CompleteHandover(ctx context.Context, id, actorID, actorName string) (Record, error) {
	check, err := r.HandoverCheck(ctx, id)
	if err != nil {
		return nil, err
	}
	blockers, _ := check["blockers"].([]string)
	if len(blockers) > 0 {
		_, _ = r.QueryOne(ctx, `UPDATE handover_records SET status='blocked', updated_at=now() WHERE id=$1 RETURNING *`, id)
		return check, nil
	}
	done, err := r.QueryOne(ctx, `UPDATE handover_records SET status='handed_over', handover_date=COALESCE(handover_date, CURRENT_DATE), approved_by=NULLIF($2,'')::uuid, updated_at=now() WHERE id=$1 RETURNING *`, id, actorID)
	if err != nil {
		return nil, err
	}
	_ = r.AddAudit(ctx, fmt.Sprint(done["project_id"]), actorID, actorName, "handover_completed", "handover", id, "Hoàn tất bàn giao công trình", Record{})
	return done, nil
}

func (r *Repository) DashboardReport(ctx context.Context) (Record, error) {
	data := Record{}
	scalars := map[string]string{
		"total_projects":       `SELECT COUNT(*) FROM projects WHERE status <> 'cancelled'`,
		"active_projects":      `SELECT COUNT(*) FROM projects WHERE status IN ('planning','in_progress','acceptance','handover')`,
		"blocked_projects":     `SELECT COUNT(*) FROM projects WHERE status='blocked'`,
		"pending_approvals":    `SELECT COUNT(*) FROM approval_requests ar LEFT JOIN projects p ON p.id=ar.project_id WHERE ar.status='pending' AND (p.id IS NULL OR p.status <> 'cancelled')`,
		"receivable_remaining": `SELECT COALESCE(SUM(remaining_amount),0) FROM receivables WHERE status <> 'cancelled'`,
		"payable_remaining":    `SELECT COALESCE(SUM(remaining_amount),0) FROM payables WHERE status <> 'cancelled'`,
		"missing_materials":    `SELECT COUNT(*) FROM site_material_balances smb JOIN materials m ON m.id=smb.material_id WHERE smb.issued_quantity < smb.planned_quantity AND m.active`,
		"failed_acceptances":   `SELECT COUNT(*) FROM acceptance_records ar JOIN projects p ON p.id=ar.project_id WHERE ar.result='fail' AND p.status <> 'cancelled'`,
		"pending_reworks":      `SELECT COUNT(*) FROM rework_requests rw JOIN projects p ON p.id=rw.project_id WHERE rw.status IN ('draft','approved','in_progress') AND p.status <> 'cancelled'`,
		"cashflow_month":       `SELECT COALESCE(SUM(CASE WHEN payment_type='client_receipt' THEN amount ELSE -amount END),0) FROM payments WHERE status <> 'cancelled' AND date_trunc('month', created_at)=date_trunc('month', now())`,
	}
	for key, query := range scalars {
		var value any
		if err := r.db.QueryRowContext(ctx, query).Scan(&value); err != nil {
			return nil, err
		}
		data[key] = normalizeValue(value)
	}
	charts := Record{}
	statusRows, _ := r.Query(ctx, `SELECT status, COUNT(*) AS total FROM projects WHERE status <> 'cancelled' GROUP BY status ORDER BY status`)
	debtRows, _ := r.Query(ctx, `
		SELECT 'Phải thu' AS type, COALESCE(SUM(remaining_amount),0) AS amount FROM receivables
		WHERE status <> 'cancelled'
		UNION ALL
		SELECT 'Phải trả' AS type, COALESCE(SUM(remaining_amount),0) AS amount FROM payables
		WHERE status <> 'cancelled'
	`)
	costRows, _ := r.Query(ctx, `
		SELECT p.name, COALESCE(b.used_amount,0) AS used_amount, COALESCE(b.remaining_amount,0) AS remaining_amount
		FROM projects p
		LEFT JOIN accounting_budgets b ON b.project_id=p.id
		WHERE p.status <> 'cancelled'
		ORDER BY p.created_at
	`)
	approvalRows, _ := r.Query(ctx, `SELECT target_type, COUNT(*) AS total FROM approval_requests GROUP BY target_type ORDER BY target_type`)
	charts["project_status"] = statusRows
	charts["debt"] = debtRows
	charts["cost_by_project"] = costRows
	charts["approvals_by_type"] = approvalRows
	data["charts"] = charts
	return data, nil
}

func (r *Repository) Report(ctx context.Context, kind string) ([]Record, error) {
	switch kind {
	case "projects":
		return r.Query(ctx, `
			SELECT p.name, p.status, p.progress_percent, COUNT(i.id) FILTER (WHERE i.status IN ('open','in_progress')) AS open_issues,
				COUNT(ar.id) FILTER (WHERE ar.status='pending') AS pending_approvals
			FROM projects p
			LEFT JOIN issues i ON i.project_id=p.id
			LEFT JOIN approval_requests ar ON ar.project_id=p.id
			WHERE p.status <> 'cancelled'
			GROUP BY p.id
			ORDER BY p.created_at DESC
		`)
	case "warehouse":
		return r.Query(ctx, `
			SELECT m.name, m.category,
				COALESCE(SUM(CASE WHEN sm.movement_type='receive' THEN sm.quantity WHEN sm.movement_type IN ('issue','loss') THEN -sm.quantity ELSE 0 END),0) AS stock_quantity,
				COALESCE(SUM(smb.surplus_quantity),0) AS surplus_quantity
			FROM materials m
			LEFT JOIN stock_movements sm ON sm.material_id=m.id
			LEFT JOIN site_material_balances smb ON smb.material_id=m.id
			WHERE m.active
			GROUP BY m.id
			ORDER BY m.category, m.name
		`)
	case "procurement":
		return r.Query(ctx, `SELECT status, budget_status, COUNT(*) AS total, COALESCE(SUM(total_estimated_amount),0) AS amount FROM procurement_requests WHERE status <> 'cancelled' GROUP BY status, budget_status ORDER BY status`)
	case "accounting":
		return r.Query(ctx, `
			SELECT p.name,
				COALESCE(SUM(DISTINCT pay.remaining_amount),0) AS payable_remaining,
				COALESCE(SUM(DISTINCT rec.remaining_amount),0) AS receivable_remaining,
				COALESCE(MAX(b.used_amount),0) AS used_budget
			FROM projects p
			LEFT JOIN payables pay ON pay.project_id=p.id AND pay.status <> 'cancelled'
			LEFT JOIN receivables rec ON rec.project_id=p.id AND rec.status <> 'cancelled'
			LEFT JOIN accounting_budgets b ON b.project_id=p.id AND b.status <> 'cancelled'
			WHERE p.status <> 'cancelled'
			GROUP BY p.id
			ORDER BY p.name
		`)
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
