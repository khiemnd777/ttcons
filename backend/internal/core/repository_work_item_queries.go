package core

import (
	"context"
	"fmt"
	"strings"
)

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
	args := append([]any{projectID}, workItemScopeArgs(profile, profileCanObserveAllProjectWork(profile), true)...)
	statusCondition := "wi.status <> 'cancelled'"
	if includeCancelled {
		statusCondition = "TRUE"
	}
	return r.Query(ctx, fmt.Sprintf(`
		SELECT wi.*, p.name AS project_name, p.code AS project_code,
			rn.name AS run_node_name, rn.order_index AS run_node_order_index,
			rn.type AS run_node_type, rn.business_code AS run_node_business_code,
			rs.name AS run_step_name, rs.order_index AS run_step_order_index,
			rs.business_code AS run_step_business_code,
			s.full_name AS assignee_name,
			COALESCE(s.status='deleted', false) AS assignee_account_deleted,
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
			CASE
				WHEN wi.status IN ('completed','done') THEN false
				WHEN wi.pipeline_run_id IS NULL OR COALESCE(pr.engine_version,1) < 2 THEN true
				ELSE pr.current_node_id=wi.run_node_id
					AND rn.status IN ('in_progress','waiting_approval','blocked')
					AND (
						rs.business_code='construction.item_reporting'
						OR rn.business_code='construction_execution'
						OR rn.type='construction'
					)
			END AS report_eligible
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
	args := append([]any{projectID}, workItemScopeArgs(profile, profileCanObserveAllProjectWork(profile), true)...)
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
			'actor_account_deleted', COALESCE(parent_actor.deleted_at IS NOT NULL, false),
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

func activityAttentionCandidatesSQL(userID string, profile workItemAccessProfile, filter ActivityAttentionFilter) (string, []any) {
	projectAccess := projectAccessSQL("p", 2)
	workItemAccess := workItemScopeSQL("wi", 5)
	args := []any{userID, profile.IsAdmin || profile.IsDirector, profile.UserID, profile.StaffID}
	args = append(args, workItemScopeArgs(profile, profile.IsManager, true)...)

	mentioned := `EXISTS (
		SELECT 1 FROM activity_mentions candidate_mention
		WHERE candidate_mention.activity_id=wia.id AND candidate_mention.recipient_user_id=$1
	)`
	unread := `EXISTS (
		SELECT 1 FROM activity_read_states candidate_read
		WHERE candidate_read.activity_id=wia.id AND candidate_read.user_id=$1 AND candidate_read.read_at IS NULL
	)`
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
	queueCondition := "(candidate.mentioned_me OR candidate.is_question OR candidate.is_blocker OR candidate.is_reminder)"
	switch filter.Queue {
	case "mentions":
		queueCondition = "candidate.mentioned_me"
	case "questions":
		queueCondition = "candidate.is_question"
	case "blockers":
		queueCondition = "candidate.is_blocker"
	case "reminders":
		queueCondition = "candidate.is_reminder"
	case "unread":
		queueCondition = "candidate.is_unread"
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 80
	}
	args = append(args, filter.Limit)
	limitPlaceholder := len(args)

	return fmt.Sprintf(`
		WITH attention_candidates AS MATERIALIZED (
			SELECT
				wia.id,
				wia.created_at,
				%s AS mentioned_me,
				%s AS is_unread,
				%s AS is_question,
				%s AS is_blocker,
				%s AS is_reminder
			FROM work_item_activities wia
			JOIN projects p ON p.id=wia.project_id
			LEFT JOIN work_items wi ON wi.id=wia.work_item_id
			WHERE %s
		),
		selected_attention AS MATERIALIZED (
			SELECT candidate.*
			FROM attention_candidates candidate
			WHERE %s
			ORDER BY
				candidate.is_unread DESC,
				CASE
					WHEN candidate.mentioned_me THEN 1
					WHEN candidate.is_question THEN 2
					WHEN candidate.is_blocker THEN 3
					WHEN candidate.is_reminder THEN 4
					ELSE 5
				END,
				candidate.created_at DESC
			LIMIT $%d
		)
	`, mentioned, unread, openQuestion, openBlocker, dueReminder, strings.Join(conditions, " AND "), queueCondition, limitPlaceholder), args
}

func (r *Repository) ListMyActivityAttention(ctx context.Context, userID string, profile workItemAccessProfile, filter ActivityAttentionFilter) ([]Record, error) {
	candidatesSQL, args := activityAttentionCandidatesSQL(userID, profile, filter)
	tx, err := r.beginReadTransactionWithJITDisabled(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	items, err := queryRecords(ctx, tx, candidatesSQL+fmt.Sprintf(`
		SELECT
			wia.*,
			wi.title AS work_item_title,
			u.full_name AS actor_name,
			COALESCE(u.deleted_at IS NOT NULL, false) AS actor_account_deleted,
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
			candidate.mentioned_me,
			%s,
			%s,
			ars.read_at,
			candidate.is_unread,
			COALESCE((
				SELECT jsonb_agg(queue_name)
				FROM (
					VALUES
						('mentions', candidate.mentioned_me),
						('questions', candidate.is_question),
						('blockers', candidate.is_blocker),
						('reminders', candidate.is_reminder),
						('unread', candidate.is_unread)
				) AS queues(queue_name, matched)
				WHERE matched
			), '[]'::jsonb) AS queue_types
		FROM selected_attention candidate
		JOIN work_item_activities wia ON wia.id=candidate.id
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
		ORDER BY
			candidate.is_unread DESC,
			CASE
				WHEN candidate.mentioned_me THEN 1
				WHEN candidate.is_question THEN 2
				WHEN candidate.is_blocker THEN 3
				WHEN candidate.is_reminder THEN 4
				ELSE 5
			END,
			candidate.created_at DESC
	`, activityAttachmentSelectSQL(), activityConversationSelectSQL("$1"), activityAttachmentJoinSQL(), activityConversationJoinSQL("$1")), args...)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) MyActivityAttentionSummary(ctx context.Context, userID string, profile workItemAccessProfile, projectID string) (ActivityAttentionSummary, error) {
	const detailLimit = 120
	candidatesSQL, args := activityAttentionCandidatesSQL(userID, profile, ActivityAttentionFilter{
		Queue:     "all",
		ProjectID: projectID,
		Limit:     detailLimit + 1,
	})
	tx, err := r.beginReadTransactionWithJITDisabled(ctx)
	if err != nil {
		return ActivityAttentionSummary{}, err
	}
	defer func() { _ = tx.Rollback() }()

	row, err := queryOneRecord(ctx, tx, candidatesSQL+`
		SELECT LEAST(COUNT(*), 120) AS count, COUNT(*) > 120 AS has_more
		FROM selected_attention
	`, args...)
	if err != nil {
		return ActivityAttentionSummary{}, err
	}
	if err := tx.Commit(); err != nil {
		return ActivityAttentionSummary{}, err
	}
	hasMore, _ := row["has_more"].(bool)
	return ActivityAttentionSummary{Count: int(number(row["count"])), HasMore: hasMore}, nil
}

func (r *Repository) GetWorkItemDetail(ctx context.Context, workItemID string, clientVisibleOnly ...bool) (Record, error) {
	item, err := r.QueryOne(ctx, `
		SELECT wi.*, p.name AS project_name, p.code AS project_code,
			rn.name AS run_node_name, rn.order_index AS run_node_order_index,
			rn.type AS run_node_type, rn.business_code AS run_node_business_code,
			rs.name AS run_step_name, rs.order_index AS run_step_order_index,
			rs.business_code AS run_step_business_code,
			s.full_name AS assignee_name,
			COALESCE(s.status='deleted', false) AS assignee_account_deleted,
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
			required_permission.name AS required_permission_name,
			required_permission.description AS required_permission_description,
			COALESCE(pr.engine_version, 1) AS pipeline_engine_version,
			(
				wi.status NOT IN ('completed','cancelled') AND wi.started_at IS NULL AND (
					COALESCE(wi.planned_start_at > now(), false)
					OR (
						COALESCE(pr.engine_version, 1) >= 2
						AND wi.pipeline_run_id IS NOT NULL
						AND pr.current_node_id IS DISTINCT FROM wi.run_node_id
					)
				)
			) AS is_future,
			(
				wi.status NOT IN ('completed','cancelled')
				AND wi.started_at IS NOT NULL
				AND COALESCE(pr.engine_version, 1) >= 2
				AND wi.pipeline_run_id IS NOT NULL
				AND pr.current_node_id IS DISTINCT FROM wi.run_node_id
			) AS is_early_execution
		FROM work_items wi
		JOIN projects p ON p.id=wi.project_id
		LEFT JOIN pipeline_runs pr ON pr.id=wi.pipeline_run_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		LEFT JOIN staff s ON s.id=wi.assignee_staff_id
		LEFT JOIN workflow_form_instances wfi ON wfi.work_item_id=wi.id AND wfi.status <> 'cancelled'
		LEFT JOIN workflow_form_versions wfv ON wfv.id=wfi.form_version_id
		LEFT JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
		LEFT JOIN permissions required_permission
			ON required_permission.key=NULLIF(wi.evidence_requirement_json->>'required_permission','')
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
		SELECT wia.*, u.full_name AS actor_name, COALESCE(u.deleted_at IS NOT NULL, false) AS actor_account_deleted, %s, %s
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
		SELECT er.*, u.full_name AS submitted_by_name, COALESCE(u.deleted_at IS NOT NULL, false) AS submitted_by_account_deleted, d.name AS document_name
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
		SELECT wia.*, s.full_name AS staff_name, COALESCE(s.status='deleted', false) AS staff_account_deleted, r.display_name AS role_display_name
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
			COALESCE(s.status='deleted', false) AS assignee_account_deleted,
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
		SELECT er.*, wi.title AS work_item_title, wi.status AS work_item_status,
			u.full_name AS submitted_by_name,COALESCE(u.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,
			reviewer.full_name AS reviewed_by_name,COALESCE(reviewer.deleted_at IS NOT NULL,false) AS reviewed_by_account_deleted,
			d.name AS document_name
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
