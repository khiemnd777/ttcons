package core

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

const projectStartupThreadPrefix = "project_setup:"

func (r *Repository) CreateProjectWithStartup(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	body["created_by"] = actorID
	project, err := insertRecord(ctx, tx, "projects", projectColumns(), body)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(project["id"])
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "project_created", "project", projectID,
		fmt.Sprintf("Tạo công trình %s", strings.TrimSpace(fmt.Sprint(project["name"]))),
		Record{"project_code": project["code"], "startup_owner_user_id": actorID}); err != nil {
		return nil, err
	}
	if _, err := emitNotificationEventTx(ctx, tx, projectSetupRequiredEvent(project, actorID, actorID, nil, "created")); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return project, nil
}

func projectSetupRequiredEvent(project Record, ownerID, actorID string, resolveUserIDs []string, suffix string) NotificationDomainEvent {
	projectID := cleanID(project["id"])
	audience := Record{"user_ids": []string{ownerID}}
	if len(resolveUserIDs) > 0 {
		audience["resolve_user_ids"] = resolveUserIDs
	}
	return NotificationDomainEvent{
		Key: "project.setup_required", AggregateType: "project", AggregateID: projectID,
		ProjectID: projectID, ActorUserID: actorID,
		Payload: Record{
			"project_id": projectID, "project_code": project["code"], "project_name": project["name"],
		},
		Audience: audience, CorrelationKey: projectStartupThreadPrefix + projectID,
		IdempotencyKey: fmt.Sprintf("project_setup:%s:%s:%s", projectID, suffix, ownerID),
	}
}

func workItemAssignedNotificationEvent(item, project Record, actorID, actorName, suffix string) NotificationDomainEvent {
	workItemID := cleanID(item["id"])
	projectID := cleanID(project["id"])
	return NotificationDomainEvent{
		Key: "work_item.assigned", AggregateType: "work_item", AggregateID: workItemID, ProjectID: projectID, ActorUserID: actorID,
		Payload: Record{
			"actor_name": actorName, "work_item_id": workItemID, "work_item_title": item["title"],
			"project_id": projectID, "project_name": project["name"],
		},
		Audience: Record{"work_item_id": workItemID}, CorrelationKey: "work_item:" + workItemID,
		IdempotencyKey: fmt.Sprintf("work_item:%s:assigned:%s", workItemID, suffix),
	}
}

func (s *Service) ProjectStartupReadiness(ctx context.Context, projectID, actorID string) (Record, error) {
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"project.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Anh/chị không có quyền xem thiết lập của công trình này")
	}
	readiness, err := projectStartupReadiness(ctx, s.repo.db, projectID)
	if err != nil {
		return nil, err
	}
	canManage, err := s.repo.UserHasAnyProjectPermission(ctx, actorID, projectID, []string{"project.member.manage"})
	if err != nil {
		return nil, err
	}
	canStart, err := s.repo.UserHasAnyProjectPermission(ctx, actorID, projectID, []string{"pipeline.run"})
	if err != nil {
		return nil, err
	}
	readiness["capabilities"] = Record{
		"can_manage_project_manager": canManage,
		"can_start_pipeline":         canStart && boolValue(readiness["ready_to_start"]),
		"is_startup_owner":           cleanID(readiness["owner_user_id"]) == actorID,
	}
	return readiness, nil
}

func (s *Service) MyProjectStartupActions(ctx context.Context, actorID, projectID, search string) (Record, error) {
	conditions := []string{
		"p.created_by IS NOT NULL", "NOT p.is_sample", "p.status <> 'cancelled'",
		"NOT EXISTS (SELECT 1 FROM pipeline_runs run WHERE run.project_id=p.id)",
		`(
			p.created_by=$1::uuid
			OR EXISTS (
				SELECT 1 FROM project_members member
				JOIN staff person ON person.id=member.staff_id AND person.status='active'
				WHERE member.project_id=p.id AND member.active AND person.user_id=$1::uuid
			)
		)`,
	}
	args := []any{actorID}
	if cleanID(projectID) != "" {
		args = append(args, projectID)
		conditions = append(conditions, fmt.Sprintf("p.id=$%d::uuid", len(args)))
	}
	if search = strings.TrimSpace(search); search != "" {
		args = append(args, search)
		conditions = append(conditions, fmt.Sprintf("(p.code ILIKE '%%'||$%d||'%%' OR p.name ILIKE '%%'||$%d||'%%')", len(args), len(args)))
	}
	rows, err := s.repo.Query(ctx, `SELECT p.id::text AS id FROM projects p WHERE `+strings.Join(conditions, " AND ")+` ORDER BY p.created_at DESC,p.id DESC LIMIT 100`, args...)
	if err != nil {
		return nil, err
	}
	items := make([]Record, 0, len(rows))
	needAction := 0
	upcoming := 0
	for _, row := range rows {
		readiness, err := projectStartupReadiness(ctx, s.repo.db, cleanID(row["id"]))
		if err != nil {
			return nil, err
		}
		actionKind := ""
		if cleanID(readiness["owner_user_id"]) == actorID {
			actionKind = "setup_required"
			needAction++
		} else if startupActorEligible(readiness, actorID) {
			actionKind = "upcoming"
			upcoming++
		}
		if actionKind == "" {
			continue
		}
		items = append(items, Record{
			"id": "project-startup:" + cleanID(readiness["project_id"]), "action_kind": actionKind,
			"project_id": readiness["project_id"], "project_code": readiness["project_code"], "project_name": readiness["project_name"],
			"owner_user_id": readiness["owner_user_id"], "owner_name": readiness["owner_name"],
			"ready_to_start": readiness["ready_to_start"], "blockers": readiness["blockers"], "first_steps": readiness["first_steps"],
			"created_at": readiness["created_at"],
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return fmt.Sprint(items[i]["action_kind"]) == "setup_required" && fmt.Sprint(items[j]["action_kind"]) != "setup_required"
	})
	return Record{"items": items, "total": len(items), "summary": Record{"need_action": needAction, "upcoming": upcoming, "total": len(items)}}, nil
}

func startupActorEligible(readiness Record, actorID string) bool {
	for _, step := range startupRecordSlice(readiness["first_steps"]) {
		for _, candidate := range startupRecordSlice(step["eligible_users"]) {
			if cleanID(candidate["user_id"]) == actorID {
				return true
			}
		}
	}
	return false
}

func projectStartupReadiness(ctx context.Context, db queryContext, projectID string) (Record, error) {
	project, err := queryOneRecord(ctx, db, `
		SELECT p.id::text AS project_id,p.code AS project_code,p.name AS project_name,p.created_at,
			COALESCE(p.created_by::text,'') AS created_by,p.is_sample,p.status,
			COALESCE(p.pm_staff_id::text,'') AS pm_staff_id,
			COALESCE(pm_user.id::text,'') AS pm_user_id,COALESCE(pm_user.full_name,'') AS pm_name,
			COALESCE(creator.full_name,creator.email,'') AS creator_name,
			COALESCE(effective_template.id::text,'') AS template_id,
			COALESCE(effective_template.name,'') AS template_name,
			COALESCE(effective_template.status,'') AS template_status,
			COALESCE(pm_member.active,false) AS pm_is_active_member
		FROM projects p
		LEFT JOIN users creator ON creator.id=p.created_by
		LEFT JOIN staff pm_staff ON pm_staff.id=p.pm_staff_id AND pm_staff.status='active'
		LEFT JOIN users pm_user ON pm_user.id=pm_staff.user_id AND pm_user.status='active' AND pm_user.deleted_at IS NULL
		LEFT JOIN project_members pm_member ON pm_member.project_id=p.id AND pm_member.staff_id=p.pm_staff_id AND pm_member.active
		LEFT JOIN LATERAL (
			SELECT template.id,template.name,template.status
			FROM pipeline_templates template
			WHERE template.id=p.pipeline_template_id
			   OR (p.pipeline_template_id IS NULL AND template.status='published')
			ORDER BY (template.id=p.pipeline_template_id) DESC,template.revision_no DESC,template.published_at DESC NULLS LAST,template.created_at DESC
			LIMIT 1
		) effective_template ON true
		WHERE p.id=$1::uuid AND p.status <> 'cancelled'
	`, projectID)
	if err != nil {
		return nil, err
	}
	if boolValue(project["is_sample"]) || cleanID(project["created_by"]) == "" {
		return nil, validationError("Công trình này không thuộc luồng điều phối khởi tạo mới", Record{"project_id": projectID})
	}
	ownerID := cleanID(project["created_by"])
	ownerName := strings.TrimSpace(fmt.Sprint(project["creator_name"]))
	pmReady := cleanID(project["pm_user_id"]) != "" && boolValue(project["pm_is_active_member"])
	if pmReady {
		ownerID = cleanID(project["pm_user_id"])
		ownerName = strings.TrimSpace(fmt.Sprint(project["pm_name"]))
	}
	templateID := cleanID(project["template_id"])
	firstSteps := []Record{}
	blockers := []Record{}
	if templateID == "" || fmt.Sprint(project["template_status"]) != "published" {
		blockers = append(blockers, Record{"key": "pipeline_template", "label": "Chọn mẫu quy trình đã phát hành"})
	} else {
		steps, err := queryRecords(ctx, db, `
			WITH first_node AS (
				SELECT id,responsible_role_ids FROM pipeline_template_nodes
				WHERE template_id=$1::uuid ORDER BY order_index,created_at,id LIMIT 1
			)
			SELECT step.id::text,step.name,step.required_role_ids,step.required_permission,
				first_node.responsible_role_ids AS node_role_ids,step.order_index
			FROM first_node
			JOIN pipeline_template_steps step ON step.node_id=first_node.id
			ORDER BY step.order_index,step.created_at,step.id
		`, templateID)
		if err != nil {
			return nil, err
		}
		members, err := projectStartupMembers(ctx, db, projectID)
		if err != nil {
			return nil, err
		}
		for _, step := range steps {
			roles := stringSlice(step["required_role_ids"])
			if len(roles) == 0 {
				roles = stringSlice(step["node_role_ids"])
			}
			requiredPermission := strings.TrimSpace(stringValue(step["required_permission"]))
			eligible := make([]Record, 0)
			for _, member := range members {
				if len(roles) > 0 && !roleLabelsIntersect(stringSlice(member["role_ids"]), roles) {
					continue
				}
				if requiredPermission != "" && !hasAnyString(stringSlice(member["permissions"]), []string{requiredPermission}) {
					continue
				}
				eligible = append(eligible, Record{"user_id": member["user_id"], "staff_id": member["staff_id"], "full_name": member["full_name"]})
			}
			covered := len(eligible) > 0
			firstSteps = append(firstSteps, Record{
				"id": step["id"], "name": step["name"], "role_ids": roles, "required_permission": requiredPermission,
				"eligible_users": eligible, "covered": covered,
			})
			if !covered {
				blockers = append(blockers, Record{"key": "first_step_coverage", "step_id": step["id"], "label": fmt.Sprintf("Phân công người đủ vai trò cho bước %s", step["name"])})
			}
		}
	}
	if !pmReady {
		blockers = append(blockers, Record{"key": "project_manager", "label": "Chỉ định PM đang là thành viên hoạt động"})
	}
	return Record{
		"project_id": project["project_id"], "project_code": project["project_code"], "project_name": project["project_name"], "created_at": project["created_at"],
		"created_by": project["created_by"], "owner_user_id": ownerID, "owner_name": ownerName,
		"pm_staff_id": project["pm_staff_id"], "pm_user_id": project["pm_user_id"], "pm_name": project["pm_name"],
		"template_id": templateID, "template_name": project["template_name"], "template_status": project["template_status"],
		"first_steps": firstSteps, "blockers": blockers, "ready_to_start": len(blockers) == 0,
	}, nil
}

func projectStartupMembers(ctx context.Context, db queryContext, projectID string) ([]Record, error) {
	return queryRecords(ctx, db, `
		SELECT account.id::text AS user_id,person.id::text AS staff_id,person.full_name,member.role_ids,
			COALESCE(jsonb_agg(DISTINCT permission.key) FILTER (WHERE permission.key IS NOT NULL),'[]'::jsonb) AS permissions
		FROM project_members member
		JOIN staff person ON person.id=member.staff_id AND person.status='active'
		JOIN users account ON account.id=person.user_id AND account.status='active' AND account.deleted_at IS NULL
		LEFT JOIN user_roles assignment ON assignment.user_id=account.id AND (assignment.project_id IS NULL OR assignment.project_id=member.project_id)
		LEFT JOIN role_permissions grant_row ON grant_row.role_id=assignment.role_id
		LEFT JOIN permissions permission ON permission.id=grant_row.permission_id
		WHERE member.project_id=$1::uuid AND member.active
		  AND NOT EXISTS (
			SELECT 1 FROM user_roles external_assignment
			JOIN roles external_role ON external_role.id=external_assignment.role_id
			WHERE external_assignment.user_id=account.id AND external_role.external_only
		  )
		GROUP BY account.id,person.id,person.full_name,member.role_ids
		ORDER BY person.full_name,person.id
	`, projectID)
}

func (s *Service) SetProjectManager(ctx context.Context, projectID, staffID, actorID, actorName string) (Record, error) {
	if cleanID(projectID) == "" || cleanID(staffID) == "" {
		return nil, validationError("Công trình và nhân sự PM là bắt buộc")
	}
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"project.member.manage"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Anh/chị không có quyền chỉ định PM cho công trình này")
	}
	project, err := s.repo.SetProjectManager(ctx, projectID, staffID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishProjectProgressChanged(projectID)
	s.publishWorkQueueChanged(projectID)
	return project, nil
}

func (r *Repository) SetProjectManager(ctx context.Context, projectID, staffID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	project, err := queryOneRecord(ctx, tx, `
		SELECT p.*,COALESCE(p.created_by::text,'') AS created_by_text,
			COALESCE(previous_user.id::text,'') AS previous_pm_user_id,
			NOT EXISTS (SELECT 1 FROM pipeline_runs run WHERE run.project_id=p.id) AS startup_open
		FROM projects p
		LEFT JOIN staff previous_staff ON previous_staff.id=p.pm_staff_id
		LEFT JOIN users previous_user ON previous_user.id=previous_staff.user_id
		WHERE p.id=$1::uuid AND p.status <> 'cancelled' FOR UPDATE OF p
	`, projectID)
	if err != nil {
		return nil, err
	}
	target, err := queryOneRecord(ctx, tx, `
		SELECT person.id::text AS staff_id,account.id::text AS user_id,person.full_name,
			COALESCE(jsonb_agg(DISTINCT permission.key) FILTER (WHERE permission.key IS NOT NULL),'[]'::jsonb) AS permissions
		FROM project_members member
		JOIN staff person ON person.id=member.staff_id AND person.status='active'
		JOIN users account ON account.id=person.user_id AND account.status='active' AND account.deleted_at IS NULL
		LEFT JOIN user_roles assignment ON assignment.user_id=account.id AND (assignment.project_id IS NULL OR assignment.project_id=member.project_id)
		LEFT JOIN role_permissions grant_row ON grant_row.role_id=assignment.role_id
		LEFT JOIN permissions permission ON permission.id=grant_row.permission_id
		WHERE member.project_id=$1::uuid AND member.staff_id=$2::uuid AND member.active
		  AND NOT EXISTS (
			SELECT 1 FROM user_roles external_assignment
			JOIN roles external_role ON external_role.id=external_assignment.role_id
			WHERE external_assignment.user_id=account.id AND external_role.external_only
		  )
		GROUP BY person.id,account.id,person.full_name
	`, projectID, staffID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, validationError("PM phải là thành viên đang hoạt động và có tài khoản đăng nhập", Record{"staff_id": staffID})
		}
		return nil, err
	}
	required := []string{"project.view", "project.member.manage", "pipeline.run"}
	missing := make([]string, 0)
	for _, permission := range required {
		if !hasAnyString(stringSlice(target["permissions"]), []string{permission}) {
			missing = append(missing, permission)
		}
	}
	if len(missing) > 0 {
		return nil, validationError("PM chưa có đủ quyền để hoàn tất khởi tạo công trình", Record{"missing_permissions": missing})
	}
	if cleanID(project["pm_staff_id"]) == staffID {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			return nil, err
		}
		return r.GetProjectDetail(ctx, projectID)
	}
	previousOwnerID := cleanID(project["previous_pm_user_id"])
	if previousOwnerID == "" {
		previousOwnerID = cleanID(project["created_by_text"])
	}
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET pm_staff_id=$2::uuid,updated_at=now() WHERE id=$1::uuid`, projectID, staffID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "project_pm_changed", "project", projectID,
		fmt.Sprintf("Chỉ định %s làm PM công trình", target["full_name"]),
		Record{"previous_owner_user_id": previousOwnerID, "pm_staff_id": staffID, "pm_user_id": target["user_id"]}); err != nil {
		return nil, err
	}
	if boolValue(project["startup_open"]) && cleanID(project["created_by_text"]) != "" && !boolValue(project["is_sample"]) {
		updated := Record{"id": projectID, "code": project["code"], "name": project["name"]}
		handoverVersion := strings.ReplaceAll(fmt.Sprint(project["updated_at"]), " ", "-")
		if _, err := emitNotificationEventTx(ctx, tx, projectSetupRequiredEvent(updated, cleanID(target["user_id"]), actorID, []string{previousOwnerID}, "pm-"+staffID+"-"+handoverVersion)); err != nil {
			return nil, err
		}
		creatorID := cleanID(project["created_by_text"])
		if creatorID != cleanID(target["user_id"]) {
			if _, err := emitNotificationEventTx(ctx, tx, NotificationDomainEvent{
				Key: "project.setup_handed_over", AggregateType: "project", AggregateID: projectID, ProjectID: projectID, ActorUserID: actorID,
				Payload:  Record{"project_id": projectID, "project_code": project["code"], "project_name": project["name"], "owner_name": target["full_name"]},
				Audience: Record{"user_ids": []string{creatorID}}, CorrelationKey: projectStartupThreadPrefix + projectID,
				IdempotencyKey: "project_setup:" + projectID + ":handover:" + cleanID(target["user_id"]),
			}); err != nil {
				return nil, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetProjectDetail(ctx, projectID)
}

func startupRecordSlice(value any) []Record {
	items := []Record{}
	switch rows := value.(type) {
	case []Record:
		return rows
	case []any:
		for _, row := range rows {
			if record, ok := row.(map[string]any); ok {
				items = append(items, Record(record))
			}
		}
	}
	return items
}
