package core

import (
	"context"
	"fmt"
	"strings"
)

type myWorkItemsPageData struct {
	Items      []Record
	Summary    Record
	Facets     Record
	Total      int
	HasMore    bool
	NextCursor string
}

func (r *Repository) MyWorkNavigationCount(ctx context.Context, userID string) (int, error) {
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return 0, err
	}
	args := append(
		workItemScopeArgs(profile, false, true),
		workItemScopeArgs(profile, profile.IsAdmin, false)...,
	)
	args = append(args, profile.IsAdmin || profile.IsDirector)
	row, err := r.QueryOne(ctx, myWorkItemsBaseSQL()+fmt.Sprintf(`
		SELECT COUNT(*) AS total
		FROM base_work
		WHERE is_responsible AND %s
	`, myWorkNavigationNeedActionSQL()), args...)
	if err != nil {
		return 0, err
	}
	return int(number(row["total"])), nil
}

func myWorkNavigationNeedActionSQL() string {
	return `(
		status NOT IN ('waiting_review','waiting_approval','completed','cancelled')
		AND can_act
		AND NOT is_future
	)`
}

func (r *Repository) MyWorkItemsPage(ctx context.Context, userID string, filter MyWorkItemsFilter, offset int) (myWorkItemsPageData, error) {
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return myWorkItemsPageData{}, err
	}
	baseArgs := append(
		workItemScopeArgs(profile, false, true),
		workItemScopeArgs(profile, profile.IsAdmin, false)...,
	)
	baseArgs = append(baseArgs, profile.IsAdmin || profile.IsDirector)

	projectConditions := []string{"TRUE"}
	projectArgs := append([]any{}, baseArgs...)
	if filter.ProjectID != "" {
		projectArgs = append(projectArgs, filter.ProjectID)
		projectConditions = append(projectConditions, fmt.Sprintf("project_id=$%d", len(projectArgs)))
	}
	projectWhere := strings.Join(projectConditions, " AND ")

	itemConditions := append([]string{}, projectConditions...)
	itemArgs := append([]any{}, projectArgs...)
	appendValue := func(condition string, value any) {
		itemArgs = append(itemArgs, value)
		itemConditions = append(itemConditions, fmt.Sprintf(condition, len(itemArgs)))
	}
	queueCondition := myWorkItemsQueueCondition(filter.Queue)
	if queueCondition != "TRUE" && filter.Bucket != "waiting_decision" {
		itemConditions = append(itemConditions, queueCondition)
	}
	if filter.Status != "" {
		appendValue("status=$%d", filter.Status)
	}
	switch filter.Deadline {
	case "overdue":
		itemConditions = append(itemConditions, "status NOT IN ('completed','cancelled') AND due_at < now()")
	case "due_7_days":
		itemConditions = append(itemConditions, "status NOT IN ('completed','cancelled') AND due_at >= now() AND due_at <= now() + interval '7 days'")
	case "no_due":
		itemConditions = append(itemConditions, "due_at IS NULL")
	}
	if filter.WorkType != "" {
		appendValue("work_type=$%d", filter.WorkType)
	}
	if filter.Priority != "" {
		appendValue("priority=$%d", filter.Priority)
	}
	if filter.Search != "" {
		itemArgs = append(itemArgs, "%"+filter.Search+"%")
		placeholder := len(itemArgs)
		itemConditions = append(itemConditions, fmt.Sprintf(`(
			title ILIKE $%d OR COALESCE(description,'') ILIKE $%d
			OR project_name ILIKE $%d OR project_code ILIKE $%d
			OR COALESCE(run_node_name,'') ILIKE $%d OR COALESCE(run_step_name,'') ILIKE $%d
			OR COALESCE(assignee_name,'') ILIKE $%d
		)`, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder))
	}
	switch filter.Bucket {
	case "all":
		// Keep the complete actor-scoped queue.
	case "need_action":
		itemConditions = append(itemConditions, myWorkNeedActionSQL())
	case "in_progress":
		itemConditions = append(itemConditions, "status='in_progress'")
	case "overdue":
		itemConditions = append(itemConditions, "status NOT IN ('completed','cancelled') AND due_at < now()")
	case "blocked":
		itemConditions = append(itemConditions, "status='blocked'")
	case "waiting_decision":
		itemConditions = append(itemConditions, "can_decide AND status IN ('waiting_review','waiting_approval')")
	case "upcoming":
		itemConditions = append(itemConditions, "is_future")
	case "completed":
		itemConditions = append(itemConditions, "status='completed'")
	case "done_today":
		itemConditions = append(itemConditions, "status='completed' AND completed_at IS NOT NULL AND (completed_at AT TIME ZONE 'Asia/Ho_Chi_Minh')::date=(now() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date")
	}
	itemWhere := strings.Join(itemConditions, " AND ")

	tx, err := r.beginReadTransactionWithJITDisabled(ctx)
	if err != nil {
		return myWorkItemsPageData{}, err
	}
	defer func() { _ = tx.Rollback() }()

	baseSQL := myWorkItemsBaseSQL()
	totalRow, err := queryOneRecord(ctx, tx, baseSQL+fmt.Sprintf(` SELECT COUNT(*) AS total FROM base_work WHERE %s`, itemWhere), itemArgs...)
	if err != nil {
		return myWorkItemsPageData{}, err
	}
	total := int(number(totalRow["total"]))

	pageArgs := append([]any{}, itemArgs...)
	pageArgs = append(pageArgs, filter.Limit, offset)
	items, err := queryRecords(ctx, tx, baseSQL+fmt.Sprintf(`
		SELECT * FROM base_work
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, itemWhere, myWorkItemsOrderSQL(filter.Sort), len(pageArgs)-1, len(pageArgs)), pageArgs...)
	if err != nil {
		return myWorkItemsPageData{}, err
	}

	summary, err := queryOneRecord(ctx, tx, baseSQL+fmt.Sprintf(`
			SELECT
				COUNT(*) FILTER (WHERE %s) AS total,
				COUNT(*) FILTER (WHERE %s AND %s) AS need_action,
				COUNT(*) FILTER (WHERE %s AND status='in_progress') AS in_progress,
				COUNT(*) FILTER (WHERE %s AND status NOT IN ('completed','cancelled') AND due_at < now()) AS overdue,
				COUNT(*) FILTER (WHERE %s AND status='blocked') AS blocked,
				COUNT(*) FILTER (WHERE can_decide AND status IN ('waiting_review','waiting_approval')) AS waiting_decision,
				COUNT(*) FILTER (WHERE %s AND status IN ('waiting_review','waiting_approval')) AS waiting_approval,
				COUNT(*) FILTER (WHERE %s AND is_future) AS upcoming,
				COUNT(*) FILTER (WHERE %s AND status='completed') AS completed,
				COUNT(*) FILTER (WHERE %s AND status='completed' AND completed_at IS NOT NULL AND (completed_at AT TIME ZONE 'Asia/Ho_Chi_Minh')::date=(now() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date) AS done_today
			FROM base_work
			WHERE %s
		`, queueCondition, queueCondition, myWorkNeedActionSQL(), queueCondition, queueCondition, queueCondition,
		queueCondition, queueCondition, queueCondition, queueCondition, projectWhere), projectArgs...)
	if err != nil {
		return myWorkItemsPageData{}, err
	}

	facets, err := queryOneRecord(ctx, tx, baseSQL+fmt.Sprintf(`
		SELECT
			COALESCE((SELECT jsonb_agg(row_value ORDER BY label) FROM (
				SELECT project_name AS label, jsonb_build_object(
					'value', project_id, 'label', project_name, 'code', project_code, 'count', COUNT(*)
				) AS row_value
				FROM base_work WHERE %s GROUP BY project_id, project_name, project_code
			) project_rows), '[]'::jsonb) AS projects,
			COALESCE((SELECT jsonb_agg(row_value ORDER BY sort_order, value) FROM (
				SELECT status AS value, CASE status
					WHEN 'blocked' THEN 1 WHEN 'waiting_review' THEN 2 WHEN 'waiting_approval' THEN 3
					WHEN 'in_progress' THEN 4 WHEN 'ready' THEN 5 WHEN 'needs_changes' THEN 6
					WHEN 'waiting_evidence' THEN 7 WHEN 'pending' THEN 8 WHEN 'completed' THEN 9 ELSE 10 END AS sort_order,
					jsonb_build_object('value', status, 'count', COUNT(*)) AS row_value
				FROM base_work WHERE %s GROUP BY status
			) status_rows), '[]'::jsonb) AS statuses,
			COALESCE((SELECT jsonb_agg(row_value ORDER BY value) FROM (
				SELECT work_type AS value, jsonb_build_object('value', work_type, 'count', COUNT(*)) AS row_value
				FROM base_work WHERE %s GROUP BY work_type
			) work_type_rows), '[]'::jsonb) AS work_types,
			COALESCE((SELECT jsonb_agg(row_value ORDER BY sort_order) FROM (
				SELECT priority, CASE priority WHEN 'urgent' THEN 1 WHEN 'high' THEN 2 WHEN 'normal' THEN 3 ELSE 4 END AS sort_order,
					jsonb_build_object('value', priority, 'count', COUNT(*)) AS row_value
				FROM base_work WHERE %s GROUP BY priority
			) priority_rows), '[]'::jsonb) AS priorities
	`, projectWhere, projectWhere, projectWhere, projectWhere), projectArgs...)
	if err != nil {
		return myWorkItemsPageData{}, err
	}

	nextOffset := offset + len(items)
	hasMore := nextOffset < total
	nextCursor := ""
	if hasMore {
		nextCursor = encodeMyWorkItemsCursor(nextOffset)
	}
	if err := tx.Commit(); err != nil {
		return myWorkItemsPageData{}, err
	}
	return myWorkItemsPageData{
		Items: items, Summary: summary, Facets: facets, Total: total, HasMore: hasMore, NextCursor: nextCursor,
	}, nil
}

func myWorkItemsQueueCondition(queue string) string {
	switch queue {
	case "responsible":
		return "is_responsible"
	case "mine":
		return "is_mine"
	case "department":
		return "is_department"
	case "pending_approval":
		return "can_decide AND status IN ('waiting_review','waiting_approval')"
	case "watching":
		return "is_watching"
	default:
		return "TRUE"
	}
}

func myWorkNeedActionSQL() string {
	return `(
		(status IN ('waiting_review','waiting_approval') AND can_decide)
		OR (
			status NOT IN ('waiting_review','waiting_approval','completed','cancelled')
			AND can_act
			AND NOT is_future
		)
	)`
}

func myWorkItemsOrderSQL(sortKind string) string {
	switch sortKind {
	case "due_asc":
		return "due_at NULLS LAST, created_at DESC, id"
	case "newest":
		return "created_at DESC, id"
	default:
		return `CASE
			WHEN status='blocked' THEN 1
			WHEN status IN ('waiting_review','waiting_approval') AND can_decide THEN 2
			WHEN can_act THEN 3 ELSE 4 END,
			CASE priority WHEN 'urgent' THEN 1 WHEN 'high' THEN 2 WHEN 'normal' THEN 3 ELSE 4 END,
			due_at NULLS LAST, created_at DESC, id`
	}
}

func myWorkItemsBaseSQL() string {
	return fmt.Sprintf(`
		WITH scoped_work AS (
			SELECT wi.*, p.name AS project_name, p.code AS project_code,
				rn.name AS run_node_name, rn.order_index AS run_node_order_index,
				rn.type AS run_node_type, rn.business_code AS run_node_business_code,
				rs.name AS run_step_name, rs.order_index AS run_step_order_index,
				rs.business_code AS run_step_business_code,
				s.full_name AS assignee_name,
				COALESCE(s.status='deleted', false) AS assignee_account_deleted,
				pr.current_node_id AS pipeline_current_node_id, rn.status AS run_node_status,
				COALESCE(pr.engine_version, 1) AS pipeline_engine_version,
				COALESCE((SELECT COUNT(*) FROM evidence_records er WHERE er.work_item_id=wi.id AND er.status <> 'rejected'),0) AS evidence_count,
				wfi.id AS workflow_form_instance_id, wfi.status AS workflow_form_status,
				wfi.form_type AS workflow_form_type, wfi.current_revision AS workflow_form_revision,
				wfi.submitted_at AS workflow_form_submitted_at, wfi.form_template_id AS workflow_form_template_id,
				wfi.form_version_id AS workflow_form_version_id, wfv.version_no AS workflow_form_version_no,
				wfv.status AS workflow_form_version_status, wft.code AS workflow_form_template_code,
				wft.name AS workflow_form_template_name, COALESCE(rs.form_required, false) AS workflow_form_required,
				rs.form_version_id AS required_workflow_form_version_id, rs.surface_kind, rs.domain_action_key,
				(%s) AS action_scope,
				COALESCE((
					wi.assignee_staff_id=NULLIF($3,'')::uuid OR EXISTS (
						SELECT 1 FROM work_item_assignments wias WHERE wias.work_item_id=wi.id
						  AND wias.assignment_type <> 'watcher' AND wias.staff_id=NULLIF($3,'')::uuid
					)
				), false) AS is_mine,
				COALESCE((
					wi.watcher_staff_ids ? $3 OR EXISTS (
						SELECT 1 FROM work_item_assignments wias WHERE wias.work_item_id=wi.id
						  AND wias.assignment_type='watcher' AND wias.staff_id=NULLIF($3,'')::uuid
					)
				), false) AS is_watching,
				(%s) AS role_responsibility,
				(%s) AS department_scope,
				(%s) AS unassigned_department_responsibility,
				COALESCE(NULLIF(ar.required_roles, '[]'::jsonb), wi.role_ids, '[]'::jsonb) AS decision_required_roles,
				COALESCE(ar.requested_by, review_request.actor_id) AS decision_requested_by
			FROM work_items wi
			JOIN projects p ON p.id=wi.project_id
			LEFT JOIN pipeline_runs pr ON pr.id=wi.pipeline_run_id
			LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
			LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
			LEFT JOIN staff s ON s.id=wi.assignee_staff_id
			LEFT JOIN workflow_form_instances wfi ON wfi.work_item_id=wi.id AND wfi.status <> 'cancelled'
			LEFT JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
			LEFT JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
			LEFT JOIN approval_requests ar ON ar.id=wi.approval_request_id
			LEFT JOIN LATERAL (
				SELECT wia.actor_id FROM work_item_activities wia
				WHERE wia.work_item_id=wi.id AND wia.activity_type='review_requested'
				ORDER BY wia.created_at DESC LIMIT 1
			) review_request ON true
			WHERE wi.status <> 'cancelled' AND p.status <> 'cancelled' AND (%s)
		), base_work AS (
			SELECT sw.*,
				(
					status NOT IN ('completed','cancelled') AND started_at IS NULL AND (
						COALESCE(planned_start_at > now(), false)
						OR (
							pipeline_engine_version >= 2
							AND pipeline_run_id IS NOT NULL
							AND pipeline_current_node_id IS DISTINCT FROM run_node_id
						)
					)
				) AS is_future,
				(
					status NOT IN ('completed','cancelled')
					AND started_at IS NOT NULL
					AND pipeline_engine_version >= 2
					AND pipeline_run_id IS NOT NULL
					AND pipeline_current_node_id IS DISTINCT FROM run_node_id
				) AS is_early_execution,
				(is_mine OR role_responsibility OR unassigned_department_responsibility) AS is_responsible,
				(NOT is_mine AND NOT is_watching AND department_scope) AS is_department,
				(
					action_scope AND status NOT IN ('completed','cancelled')
					AND (
						started_at IS NOT NULL
						OR (
							NOT (
								COALESCE(planned_start_at > now(), false)
								OR (
									pipeline_engine_version >= 2
									AND pipeline_run_id IS NOT NULL
									AND pipeline_current_node_id IS DISTINCT FROM run_node_id
								)
							)
							AND (
								pipeline_engine_version < 2 OR pipeline_run_id IS NULL
								OR (pipeline_current_node_id=run_node_id AND run_node_status IN ('in_progress','waiting_approval','blocked'))
							)
						)
					)
					AND (
						NULLIF(evidence_requirement_json->>'required_permission','') IS NULL
						OR EXISTS (
							SELECT 1 FROM user_roles ur
							JOIN role_permissions rp ON rp.role_id=ur.role_id
							JOIN permissions perm ON perm.id=rp.permission_id
							WHERE ur.user_id=NULLIF($2,'')::uuid AND (ur.project_id IS NULL OR ur.project_id=sw.project_id)
							  AND perm.key=evidence_requirement_json->>'required_permission'
						)
					)
				) AS can_act,
				(
					status IN ('waiting_review','waiting_approval')
					AND (
						started_at IS NOT NULL
						OR NOT (
							COALESCE(planned_start_at > now(), false)
							OR (
								pipeline_engine_version >= 2
								AND pipeline_run_id IS NOT NULL
								AND pipeline_current_node_id IS DISTINCT FROM run_node_id
							)
						)
					)
					AND (decision_requested_by IS NULL OR decision_requested_by <> NULLIF($2,'')::uuid)
					AND (
						$11::boolean
						OR EXISTS (
							SELECT 1 FROM staff actor_staff
							JOIN project_members pm ON pm.staff_id=actor_staff.id AND pm.active
							WHERE actor_staff.user_id=NULLIF($2,'')::uuid AND actor_staff.status='active' AND pm.project_id=sw.project_id
							  AND (pm.role_ids ?| ARRAY['pm','PM','Project Manager','admin','director','Giám đốc'])
						)
						OR EXISTS (
							SELECT 1 FROM user_roles ur JOIN roles actor_role ON actor_role.id=ur.role_id
							WHERE ur.user_id=NULLIF($2,'')::uuid AND ur.project_id=sw.project_id
							  AND (actor_role.name IN ('pm','admin','director') OR actor_role.display_name IN ('PM','Project Manager','Admin','Giám đốc'))
						)
						OR (
							(
								EXISTS (
									SELECT 1 FROM staff actor_staff
									JOIN project_members pm ON pm.staff_id=actor_staff.id AND pm.active
									WHERE actor_staff.user_id=NULLIF($2,'')::uuid AND actor_staff.status='active' AND pm.project_id=sw.project_id
								)
								OR EXISTS (
									SELECT 1 FROM user_roles scoped_role
									WHERE scoped_role.user_id=NULLIF($2,'')::uuid AND scoped_role.project_id=sw.project_id
								)
							)
							AND EXISTS (
								SELECT 1 FROM jsonb_array_elements_text(decision_required_roles) required(role_label)
								WHERE EXISTS (
									SELECT 1 FROM user_roles ur JOIN roles actor_role ON actor_role.id=ur.role_id
									WHERE ur.user_id=NULLIF($2,'')::uuid AND (ur.project_id IS NULL OR ur.project_id=sw.project_id)
									  AND lower(btrim(required.role_label)) IN (
										lower(btrim(actor_role.name)), lower(btrim(actor_role.display_name)), lower(actor_role.id::text)
									  )
								)
								OR EXISTS (
									SELECT 1 FROM staff actor_staff
									JOIN project_members pm ON pm.staff_id=actor_staff.id AND pm.active
									CROSS JOIN LATERAL jsonb_array_elements_text(pm.role_ids) member_role(role_label)
									WHERE actor_staff.user_id=NULLIF($2,'')::uuid AND actor_staff.status='active' AND pm.project_id=sw.project_id
									  AND lower(btrim(member_role.role_label))=lower(btrim(required.role_label))
								)
							)
						)
					)
				) AS can_decide
			FROM scoped_work sw
		)
	`, workItemScopeSQL("wi", 6), myWorkRoleResponsibilitySQL("wi", 2, 3),
		myWorkDepartmentScopeSQL("wi", 4), myWorkUnassignedDepartmentResponsibilitySQL("wi", 4),
		workItemScopeSQL("wi", 1))
}

func myWorkRoleResponsibilitySQL(alias string, userParam, staffParam int) string {
	userID := fmt.Sprintf("NULLIF($%d,'')::uuid", userParam)
	staffID := fmt.Sprintf("NULLIF($%d,'')::uuid", staffParam)
	return fmt.Sprintf(`COALESCE((
		EXISTS (
			SELECT 1
			FROM project_members pm
			WHERE pm.project_id=%s.project_id AND pm.staff_id=%s AND pm.active
			  AND (
				EXISTS (
					SELECT 1
					FROM jsonb_array_elements_text(pm.role_ids) member_role(role_label)
					JOIN jsonb_array_elements_text(%s.role_ids) work_role(role_label)
					  ON work_role.role_label=member_role.role_label
				)
				OR EXISTS (
					SELECT 1
					FROM work_item_assignments assignment
					CROSS JOIN LATERAL jsonb_array_elements_text(pm.role_ids) member_role(role_label)
					WHERE assignment.work_item_id=%s.id
					  AND assignment.assignment_type='role'
					  AND assignment.role_name=member_role.role_label
				)
			  )
		)
		OR EXISTS (
			SELECT 1
			FROM user_roles user_role
			JOIN roles role ON role.id=user_role.role_id
			WHERE user_role.user_id=%s AND user_role.project_id=%s.project_id
			  AND (
				EXISTS (
					SELECT 1 FROM jsonb_array_elements_text(%s.role_ids) work_role(role_label)
					WHERE work_role.role_label IN (role.name, role.display_name, role.id::text)
				)
				OR EXISTS (
					SELECT 1 FROM work_item_assignments assignment
					WHERE assignment.work_item_id=%s.id AND assignment.assignment_type='role'
					  AND assignment.role_name IN (role.name, role.display_name, role.id::text)
				)
			  )
		)
	), false)`, alias, staffID, alias, alias, userID, alias, alias, alias)
}

func myWorkDepartmentScopeSQL(alias string, departmentParam int) string {
	department := fmt.Sprintf("$%d", departmentParam)
	return fmt.Sprintf(`COALESCE((
		NULLIF(%s,'') IS NOT NULL AND (
			%s.department=%s
			OR EXISTS (
				SELECT 1 FROM work_item_assignments assignment
				WHERE assignment.work_item_id=%s.id AND assignment.assignment_type='department'
				  AND assignment.department=%s
			)
		)
	), false)`, department, alias, department, alias, department)
}

func myWorkUnassignedDepartmentResponsibilitySQL(alias string, departmentParam int) string {
	department := fmt.Sprintf("$%d", departmentParam)
	return fmt.Sprintf(`COALESCE((
		NULLIF(%s,'') IS NOT NULL
		AND %s.assignee_staff_id IS NULL
		AND NOT EXISTS (
			SELECT 1 FROM work_item_assignments assignment
			WHERE assignment.work_item_id=%s.id AND assignment.assignment_type='assignee'
			  AND assignment.staff_id IS NOT NULL
		)
		AND (
			EXISTS (
				SELECT 1 FROM work_item_assignments assignment
				WHERE assignment.work_item_id=%s.id AND assignment.assignment_type='department'
				  AND assignment.department=%s
			)
			OR (
				%s.department=%s
				AND COALESCE(jsonb_array_length(%s.role_ids),0)=0
				AND NOT EXISTS (
					SELECT 1 FROM work_item_assignments assignment
					WHERE assignment.work_item_id=%s.id AND assignment.assignment_type='role'
				)
			)
		)
	), false)`, department, alias, alias, alias, department, alias, department, alias, alias)
}
