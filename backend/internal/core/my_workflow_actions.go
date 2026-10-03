package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// MyWorkflowActions returns runtime transitions that are ready now for the
// authenticated actor. These are presentation-only queue entries, not
// synthetic Work Items; AdvanceRunNode remains the mutation authority.
func (r *Repository) MyWorkflowActions(ctx context.Context, actorID string, filter MyWorkflowActionsFilter) ([]Record, error) {
	if !statusIn(filter.Queue, "responsible", "all") {
		return []Record{}, nil
	}

	scopeSQL, args, err := r.ProjectAccessFilter(ctx, actorID, "p", 1)
	if err != nil {
		return nil, err
	}
	conditions := []string{
		"p.status <> 'cancelled'",
		"pr.status='in_progress'",
		"pr.current_node_id=rn.id",
		"rn.status IN ('in_progress','waiting_approval')",
		fmt.Sprintf("(%s)", scopeSQL),
		myWorkflowActionPermissionSQL("rn", "p", 2),
		myWorkflowActionReadySQL("rn"),
	}
	if filter.Queue == "responsible" {
		conditions = append(conditions, myWorkflowActionResponsibilitySQL("rn", "p", 2, 3))
	}
	if filter.ProjectID != "" {
		args = append(args, filter.ProjectID)
		conditions = append(conditions, fmt.Sprintf("p.id=$%d", len(args)))
	}
	candidates, err := r.Query(ctx, fmt.Sprintf(`
		SELECT rn.*, p.name AS project_name, p.code AS project_code,
			pr.engine_version AS pipeline_engine_version,
			%s AS is_responsible
		FROM pipeline_runs pr
		JOIN projects p ON p.id=pr.project_id
		JOIN pipeline_run_nodes rn ON rn.id=pr.current_node_id AND rn.run_id=pr.id
		WHERE %s
		ORDER BY p.name,p.code,rn.order_index,rn.id
	`, myWorkflowActionResponsibilitySQL("rn", "p", 2, 3), strings.Join(conditions, " AND ")), args...)
	if err != nil {
		return nil, err
	}

	actions := make([]Record, 0, len(candidates))
	for _, node := range candidates {
		issues, issueErr := r.NodeCompletionIssues(ctx, node)
		if issueErr != nil {
			return nil, issueErr
		}
		if len(issues) > 0 || len(r.nodeBlockers(ctx, node)) > 0 {
			continue
		}

		nextNode, transition, transitionBlockers, transitionErr := r.previewNextRunNode(ctx, node)
		if transitionErr != nil {
			return nil, transitionErr
		}
		if len(transitionBlockers) > 0 || transition == nil {
			continue
		}
		action := myWorkflowAdvanceAction(node, nextNode, transition)
		action["is_responsible"] = boolValue(node["is_responsible"])
		actions = append(actions, action)
	}
	return actions, nil
}

func (r *Repository) previewNextRunNode(ctx context.Context, node Record) (Record, Record, []string, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	nextNode, transition, blockers, err := r.nextRunNode(ctx, tx, node)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, nil, err
	}
	return nextNode, transition, blockers, nil
}

func myWorkflowAdvanceAction(node, nextNode, transition Record) Record {
	nextName := strings.TrimSpace(fmt.Sprint(nextNode["name"]))
	nextType := strings.TrimSpace(fmt.Sprint(nextNode["type"]))
	terminal := nextNode == nil || cleanID(nextNode["id"]) == ""
	title, actionLabel := myWorkflowAdvanceLabels(nextName, nextType, terminal)
	description := fmt.Sprintf("Giai đoạn %s đã đủ điều kiện. Chọn hành động để tiếp tục quy trình.", strings.TrimSpace(fmt.Sprint(node["name"])))
	if terminal {
		description = fmt.Sprintf("Giai đoạn %s đã đủ điều kiện. Chọn hành động để hoàn tất quy trình.", strings.TrimSpace(fmt.Sprint(node["name"])))
	}
	return Record{
		"id":                     "advance_run_node:" + cleanID(node["id"]),
		"action_type":            "advance_run_node",
		"title":                  title,
		"action_label":           actionLabel,
		"description":            description,
		"project_id":             cleanID(node["project_id"]),
		"project_name":           node["project_name"],
		"project_code":           node["project_code"],
		"pipeline_run_id":        cleanID(node["run_id"]),
		"run_node_id":            cleanID(node["id"]),
		"run_node_name":          node["name"],
		"run_node_type":          node["type"],
		"run_node_business_code": node["business_code"],
		"next_run_node_id":       cleanID(nextNode["id"]),
		"next_run_node_name":     nextName,
		"next_run_node_type":     nextType,
		"terminal":               terminal,
		"transition":             transition,
		"readiness":              "actionable",
	}
}

func myWorkflowAdvanceLabels(nextName, nextType string, terminal bool) (string, string) {
	if terminal {
		return "Hoàn tất quy trình", "Hoàn tất quy trình"
	}
	switch nextType {
	case "construction_execution", "construction":
		return "Bắt đầu Thi công", "Chuyển sang Thi công"
	case "handover_finance", "handover":
		return "Mở giai đoạn Bàn giao", "Chuyển sang Bàn giao"
	default:
		label := strings.TrimSpace(nextName)
		if label == "" || label == "<nil>" {
			label = "giai đoạn tiếp theo"
		}
		return "Chuyển sang " + label, "Chuyển sang " + label
	}
}

func myWorkflowActionReadySQL(nodeAlias string) string {
	return fmt.Sprintf(`(
		NOT EXISTS (
			SELECT 1 FROM pipeline_run_steps step
			WHERE step.run_node_id=%[1]s.id AND step.status <> 'completed'
		)
		AND NOT EXISTS (
			SELECT 1 FROM work_items item
			WHERE item.run_node_id=%[1]s.id AND item.status NOT IN ('completed','cancelled')
		)
		AND (
			%[1]s.type <> 'construction_planning'
			OR EXISTS (
				SELECT 1 FROM construction_handoffs handoff
				WHERE handoff.pipeline_run_id=%[1]s.run_id
				  AND handoff.source_run_node_id=%[1]s.id
				  AND handoff.status='activated'
			)
		)
		AND NOT EXISTS (
			SELECT 1
			FROM pipeline_run_steps step
			LEFT JOIN workflow_form_instances form
			  ON form.run_step_id=step.id
			 AND form.form_version_id=step.form_version_id
			 AND form.status <> 'cancelled'
			WHERE step.run_node_id=%[1]s.id
			  AND step.form_required
			  AND (form.id IS NULL OR form.status <> 'submitted')
		)
		AND (
			NOT (
				COALESCE(%[1]s.approval_policy_json->>'mode','') IN ('any','all','quorum','ordered')
				OR NULLIF(BTRIM(COALESCE(%[1]s.approval_rule,'')),'') IS NOT NULL
			)
			OR EXISTS (
				SELECT 1 FROM approval_requests approval
				WHERE approval.target_type='pipeline_run_node'
				  AND approval.target_id=%[1]s.id
				  AND approval.status='approved'
			)
		)
		AND NOT EXISTS (
			SELECT 1 FROM workflow_automation_events automation
			WHERE automation.run_node_id=%[1]s.id AND automation.status IN ('blocked','failed')
		)
	)`, nodeAlias)
}

func myWorkflowActionPermissionSQL(nodeAlias, projectAlias string, userParam int) string {
	return fmt.Sprintf(`(
		(
			jsonb_array_length(COALESCE(%[1]s.permissions,'[]'::jsonb))=0
			AND NULLIF(BTRIM(COALESCE(%[1]s.required_permission,'')),'') IS NULL
		)
		OR EXISTS (
			SELECT 1
			FROM user_roles user_role
			JOIN role_permissions role_permission ON role_permission.role_id=user_role.role_id
			JOIN permissions permission ON permission.id=role_permission.permission_id
			WHERE user_role.user_id=NULLIF($%[3]d,'')::uuid
			  AND (user_role.project_id IS NULL OR user_role.project_id=%[2]s.id)
			  AND permission.key IN (
				SELECT required.permission_key
				FROM jsonb_array_elements_text(
					CASE
						WHEN jsonb_array_length(COALESCE(%[1]s.permissions,'[]'::jsonb))>0 THEN %[1]s.permissions
						ELSE jsonb_build_array(%[1]s.required_permission)
					END
				) required(permission_key)
			  )
		)
	)`, nodeAlias, projectAlias, userParam)
}

func myWorkflowActionResponsibilitySQL(nodeAlias, projectAlias string, userParam, staffParam int) string {
	return fmt.Sprintf(`(
		jsonb_array_length(COALESCE(%[1]s.responsible_role_ids,'[]'::jsonb))=0
		OR EXISTS (
			SELECT 1
			FROM project_members member
			CROSS JOIN LATERAL jsonb_array_elements_text(member.role_ids) member_role(role_label)
			CROSS JOIN LATERAL jsonb_array_elements_text(COALESCE(%[1]s.responsible_role_ids,'[]'::jsonb)) required_role(role_label)
			WHERE member.project_id=%[2]s.id
			  AND member.staff_id=NULLIF($%[4]d,'')::uuid
			  AND member.active
			  AND lower(btrim(consflow_role_label_key(member_role.role_label)))=lower(btrim(consflow_role_label_key(required_role.role_label)))
		)
		OR EXISTS (
			SELECT 1
			FROM user_roles user_role
			JOIN roles role ON role.id=user_role.role_id
			CROSS JOIN LATERAL jsonb_array_elements_text(COALESCE(%[1]s.responsible_role_ids,'[]'::jsonb)) required_role(role_label)
			WHERE user_role.user_id=NULLIF($%[3]d,'')::uuid
			  AND (user_role.project_id IS NULL OR user_role.project_id=%[2]s.id)
			  AND lower(btrim(consflow_role_label_key(required_role.role_label))) IN (
				lower(btrim(consflow_role_label_key(role.name))),lower(btrim(consflow_role_label_key(role.display_name))),lower(role.id::text)
			  )
		)
	)`, nodeAlias, projectAlias, userParam, staffParam)
}
