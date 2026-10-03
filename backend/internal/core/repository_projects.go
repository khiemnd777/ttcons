package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

func (r *Repository) ListProjects(ctx context.Context, userID string) ([]Record, error) {
	scopeSQL, scopeArgs, err := r.SupervisionProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, err
	}
	return r.Query(ctx, fmt.Sprintf(`
		SELECT p.*, s.full_name AS pm_name, COALESCE(s.status='deleted', false) AS pm_account_deleted, t.name AS pipeline_template_name,
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
	scopeSQL, scopeArgs, err := r.SupervisionProjectAccessFilter(ctx, userID, "p", 1)
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
			COALESCE(s.status='deleted', false) AS pm_account_deleted,
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
	scopeSQL, scopeArgs, err := r.SupervisionProjectAccessFilter(ctx, userID, "p", 1)
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
	activeFilter := "AND pm.active AND s.status <> 'deleted'"
	if includeInactive {
		activeFilter = ""
	}
	return r.Query(ctx, fmt.Sprintf(`
		SELECT pm.id::text, pm.project_id::text, pm.staff_id::text,
			COALESCE(s.user_id::text, '') AS user_id,
			s.full_name, s.email, s.phone, s.department, s.position,
			COALESCE(u.is_sample,false) AS is_sample,
			COALESCE(u.status='active' AND u.deleted_at IS NULL AND s.status='active', false) AS account_active,
			COALESCE(s.status='deleted', false) AS account_deleted,
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
					),
					'grants_project_member_manage', EXISTS (
						SELECT 1 FROM role_permissions rp
						JOIN permissions p ON p.id=rp.permission_id
						WHERE rp.role_id=r.id AND p.key='project.member.manage'
					),
					'grants_pipeline_run', EXISTS (
						SELECT 1 FROM role_permissions rp
						JOIN permissions p ON p.id=rp.permission_id
						WHERE rp.role_id=r.id AND p.key='pipeline.run'
					)
				) ORDER BY r.display_name)
				FROM user_roles ur
				JOIN roles r ON r.id=ur.role_id
				WHERE ur.user_id=s.user_id
				  AND ur.project_id IS NULL
				  AND NOT r.external_only
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
					),
					'grants_project_member_manage', EXISTS (
						SELECT 1 FROM role_permissions rp
						JOIN permissions p ON p.id=rp.permission_id
						WHERE rp.role_id=r.id AND p.key='project.member.manage'
					),
					'grants_pipeline_run', EXISTS (
						SELECT 1 FROM role_permissions rp
						JOIN permissions p ON p.id=rp.permission_id
						WHERE rp.role_id=r.id AND p.key='pipeline.run'
					)
				) ORDER BY r.display_name)
				FROM user_roles ur
				JOIN roles r ON r.id=ur.role_id
				WHERE ur.user_id=s.user_id AND ur.project_id=pm.project_id
			), '[]'::jsonb) AS project_roles
		FROM project_members pm
		JOIN projects p ON p.id=pm.project_id AND p.status <> 'cancelled'
		JOIN staff s ON s.id=pm.staff_id
		LEFT JOIN users u ON u.id=s.user_id
		WHERE pm.project_id=$1 %s
		ORDER BY pm.active DESC, lower(s.full_name), pm.created_at DESC
	`, activeFilter), projectID)
}

func (r *Repository) ProjectMemberOptions(ctx context.Context, projectID string) (Record, error) {
	staff, err := r.Query(ctx, `
		SELECT s.id::text, s.user_id::text, s.full_name, s.email, s.department, s.position,
			u.is_sample,
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
					),
					'grants_project_member_manage', EXISTS (
						SELECT 1 FROM role_permissions rp
						JOIN permissions p ON p.id=rp.permission_id
						WHERE rp.role_id=r.id AND p.key='project.member.manage'
					),
					'grants_pipeline_run', EXISTS (
						SELECT 1 FROM role_permissions rp
						JOIN permissions p ON p.id=rp.permission_id
						WHERE rp.role_id=r.id AND p.key='pipeline.run'
					)
				) ORDER BY r.display_name)
				FROM user_roles ur
				JOIN roles r ON r.id=ur.role_id
				WHERE ur.user_id=s.user_id AND ur.project_id IS NULL
			), '[]'::jsonb) AS global_roles
		FROM projects target_project
		JOIN staff s ON true
		JOIN users u ON u.id=s.user_id AND u.status='active'
		LEFT JOIN LATERAL (
			SELECT pm.id::text AS id, pm.active
			FROM project_members pm
			WHERE pm.project_id=$1 AND pm.staff_id=s.id
			ORDER BY pm.active DESC, pm.created_at DESC, pm.id DESC
			LIMIT 1
		) latest_member ON true
		WHERE target_project.id=$1 AND target_project.status <> 'cancelled'
		  AND s.status='active'
		  AND u.is_sample=target_project.is_sample
		  AND NOT EXISTS (
			SELECT 1
			FROM user_roles assignment
			JOIN roles role ON role.id=assignment.role_id
			WHERE assignment.user_id=s.user_id
			  AND role.external_only
		  )
		ORDER BY lower(s.full_name)
	`, projectID)
	if err != nil {
		return nil, err
	}
	roles, err := r.Query(ctx, `
		SELECT r.id::text, r.name, r.display_name, r.description, 'project' AS scope,
			EXISTS (
				SELECT 1
				FROM role_permissions rp
				JOIN permissions p ON p.id=rp.permission_id
				WHERE rp.role_id=r.id AND p.key='project.view'
			) AS grants_project_view,
			EXISTS (
				SELECT 1
				FROM role_permissions rp
				JOIN permissions p ON p.id=rp.permission_id
				WHERE rp.role_id=r.id AND p.key='project.member.manage'
			) AS grants_project_member_manage,
			EXISTS (
				SELECT 1
				FROM role_permissions rp
				JOIN permissions p ON p.id=rp.permission_id
				WHERE rp.role_id=r.id AND p.key='pipeline.run'
			) AS grants_pipeline_run
		FROM projects target_project
		JOIN roles r ON true
		WHERE target_project.id=$1 AND target_project.status <> 'cancelled'
		  AND r.name NOT IN ('admin','director')
		  AND NOT r.external_only
		  AND (NOT target_project.is_sample OR r.name <> 'developer')
		  AND (
			not target_project.is_sample
			OR EXISTS (
				SELECT 1 FROM users sample_account
				WHERE sample_account.is_sample AND sample_account.sample_role_id=r.id
				  AND sample_account.status='active' AND sample_account.deleted_at IS NULL
			)
		  )
		ORDER BY lower(r.display_name)
	`, projectID)
	if err != nil {
		return nil, err
	}
	roleLabelRows, err := r.Query(ctx, `
		SELECT label
		FROM (
			SELECT DISTINCT label
			FROM (
				SELECT r.display_name AS label
				FROM projects target_project
				JOIN roles r ON true
				WHERE target_project.id=$1 AND target_project.status <> 'cancelled'
				  AND NOT r.external_only
				  AND (
					not target_project.is_sample
					OR EXISTS (
						SELECT 1 FROM users sample_account
						WHERE sample_account.is_sample AND sample_account.sample_role_id=r.id
						  AND sample_account.status='active' AND sample_account.deleted_at IS NULL
					)
				  )
				UNION ALL
				SELECT project_label.label
				FROM project_members pm
				CROSS JOIN LATERAL jsonb_array_elements_text(COALESCE(pm.role_ids, '[]'::jsonb)) project_label(label)
				WHERE pm.project_id=$1
				  AND NOT EXISTS (
					SELECT 1
					FROM roles external_role
					WHERE external_role.external_only
					  AND lower(btrim(external_role.display_name))=lower(btrim(project_label.label))
				  )
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
	memberID, err = r.saveProjectMemberTx(ctx, tx, projectID, memberID, req, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetProjectMember(ctx, projectID, memberID)
}

func (r *Repository) SaveProjectMembersBulk(ctx context.Context, projectID string, requests []ProjectMemberMutationRequest, actorID, actorName string) ([]Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	memberIDs := make([]string, 0, len(requests))
	for _, req := range requests {
		memberID, err := r.saveProjectMemberTx(ctx, tx, projectID, "", req, actorID, actorName)
		if err != nil {
			return nil, err
		}
		memberIDs = append(memberIDs, memberID)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	rows, err := r.ListProjectMembers(ctx, projectID, true)
	if err != nil {
		return nil, err
	}
	memberByID := make(map[string]Record, len(rows))
	for _, row := range rows {
		memberByID[cleanID(row["id"])] = row
	}
	members := make([]Record, 0, len(memberIDs))
	for _, memberID := range memberIDs {
		member, ok := memberByID[memberID]
		if !ok {
			return nil, sql.ErrNoRows
		}
		members = append(members, member)
	}
	return members, nil
}

func (r *Repository) saveProjectMemberTx(ctx context.Context, tx *sql.Tx, projectID, memberID string, req ProjectMemberMutationRequest, actorID, actorName string) (string, error) {

	projectRows, err := queryTx(ctx, tx, `
		SELECT id::text, COALESCE(pm_staff_id::text, '') AS pm_staff_id,is_sample
		FROM projects
		WHERE id=$1 AND status <> 'cancelled'
		FOR UPDATE
	`, projectID)
	if err != nil {
		return "", err
	}
	if len(projectRows) == 0 {
		return "", validationError("Công trình không tồn tại hoặc đã huỷ", Record{"project_id": projectID})
	}
	staffRows, err := queryTx(ctx, tx, `
		SELECT s.id::text, s.full_name, COALESCE(s.user_id::text, '') AS user_id,
			u.is_sample,COALESCE(u.sample_role_id::text,'') AS sample_role_id
		FROM staff s
		JOIN users u ON u.id=s.user_id AND u.status='active'
		WHERE s.id=$1 AND s.status='active'
		  AND NOT EXISTS (
			SELECT 1
			FROM user_roles assignment
			JOIN roles role ON role.id=assignment.role_id
			WHERE assignment.user_id=s.user_id
			  AND role.external_only
		  )
		FOR UPDATE OF s
	`, req.StaffID)
	if err != nil {
		return "", err
	}
	if len(staffRows) == 0 || cleanID(staffRows[0]["user_id"]) == "" {
		return "", validationError("Nhân sự phải đang hoạt động và có tài khoản đăng nhập", Record{"staff_id": req.StaffID})
	}
	staffName := strings.TrimSpace(fmt.Sprint(staffRows[0]["full_name"]))
	userID := cleanID(staffRows[0]["user_id"])
	projectIsSample := boolValue(projectRows[0]["is_sample"])
	userIsSample := boolValue(staffRows[0]["is_sample"])
	sampleRoleID := cleanID(staffRows[0]["sample_role_id"])
	if projectIsSample != userIsSample {
		if projectIsSample {
			return "", validationError("Tài khoản thật không được gán vào công trình mẫu", Record{"staff_id": req.StaffID})
		}
		return "", validationError("Tài khoản mẫu không được gán vào công trình thật", Record{"staff_id": req.StaffID})
	}
	if userIsSample {
		if len(req.ProjectRoleIDs) != 1 || cleanID(req.ProjectRoleIDs[0]) != sampleRoleID {
			return "", validationError("Tài khoản mẫu chỉ được dùng đúng vai trò mẫu của tài khoản", Record{"staff_id": req.StaffID, "sample_role_id": sampleRoleID})
		}
		var sampleRoleName string
		if err := tx.QueryRowContext(ctx, `SELECT display_name FROM roles WHERE id=$1`, sampleRoleID).Scan(&sampleRoleName); err != nil {
			return "", err
		}
		if len(req.RoleIDs) != 1 || !strings.EqualFold(strings.TrimSpace(req.RoleIDs[0]), strings.TrimSpace(sampleRoleName)) {
			return "", validationError("Vai trò vận hành của tài khoản mẫu phải khớp vai trò hệ thống", Record{"staff_id": req.StaffID, "role": sampleRoleName})
		}
	}
	for _, roleLabel := range req.RoleIDs {
		var externalOnly bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM roles
				WHERE external_only
				  AND lower(btrim(display_name))=lower(btrim($1))
			)
		`, roleLabel).Scan(&externalOnly); err != nil {
			return "", err
		}
		if externalOnly {
			return "", validationError("Vai trò vận hành của đối tác ngoài App không thể gán cho thành viên công trình", Record{"role": roleLabel})
		}
	}

	for _, roleID := range req.ProjectRoleIDs {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM roles WHERE id=$1 AND name NOT IN ('admin','director') AND NOT external_only`, roleID).Scan(&exists); err != nil {
			return "", err
		}
		if exists == 0 {
			return "", validationError("Role phân quyền không tồn tại", Record{"role_id": roleID})
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
			return "", err
		}
		if len(rows) == 0 {
			return "", validationError("Thành viên không thuộc công trình", Record{"member_id": memberID})
		}
		current = rows[0]
		if current["active"] != true {
			return "", validationError("Thành viên đã ngừng tham gia; hãy dùng luồng thêm để kích hoạt lại", Record{"member_id": memberID})
		}
		if cleanID(current["staff_id"]) != req.StaffID {
			return "", validationError("Không thể đổi nhân sự của một membership đã tồn tại", Record{"member_id": memberID})
		}
	} else {
		rows, err := queryTx(ctx, tx, `
			SELECT * FROM project_members
			WHERE project_id=$1 AND staff_id=$2
			ORDER BY active DESC, created_at DESC, id DESC
			FOR UPDATE
		`, projectID, req.StaffID)
		if err != nil {
			return "", err
		}
		if len(rows) > 0 {
			current = rows[0]
			if current["active"] == true {
				return "", validationError("Nhân sự đã là thành viên đang hoạt động của công trình", Record{"staff_id": req.StaffID})
			}
			memberID = cleanID(current["id"])
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1 AND project_id=$2`, userID, projectID); err != nil {
		return "", err
	}
	for _, roleID := range req.ProjectRoleIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_roles (user_id, role_id, project_id)
			VALUES ($1,$2,$3)
			ON CONFLICT DO NOTHING
		`, userID, roleID, projectID); err != nil {
			return "", err
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
		return "", err
	}
	if !canViewProject {
		return "", validationError("Role hiện tại không cấp quyền xem công trình; hãy chọn role có project.view", Record{"field": "project_role_ids"})
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
			return "", err
		}
		eventType = "project_member_added"
		description = fmt.Sprintf("Thêm thành viên %s", staffName)
	} else if _, err := tx.ExecContext(ctx, `
		UPDATE project_members
		SET role_ids=$3::jsonb, notes=NULLIF($4,''), active=true, updated_at=now()
		WHERE id=$1 AND project_id=$2
	`, memberID, projectID, string(rawRoles), req.Notes); err != nil {
		return "", err
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
		return "", err
	}
	return memberID, nil
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
		return nil, validationError("Không thể ngừng tham gia Quản lý dự án hiện tại; hãy cập nhật Quản lý dự án thay thế trước", Record{"member_id": memberID})
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
		SELECT p.*, s.full_name AS pm_name, COALESCE(s.status='deleted', false) AS pm_account_deleted, t.name AS pipeline_template_name,
			pt.name AS partner_name, pt.tax_code AS partner_tax_code
		FROM projects p
		LEFT JOIN staff s ON s.id=p.pm_staff_id
		LEFT JOIN pipeline_templates t ON t.id=p.pipeline_template_id
		LEFT JOIN partners pt ON pt.id=p.partner_id
		WHERE p.id=$1 AND p.status <> 'cancelled'
	`, id)
}
