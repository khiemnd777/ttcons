package core

import (
	"context"
	"fmt"
	"strings"
)

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
				COALESCE(u.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,
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
				  AND NOT EXISTS (
					SELECT 1
					FROM workflow_form_pdf_export_deletions deletion
					WHERE deletion.pdf_export_id=wfp.id AND deletion.status='deleted'
				  )
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
				COALESCE(u.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,
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
				  AND NOT EXISTS (
					SELECT 1
					FROM workflow_form_pdf_export_deletions deletion
					WHERE deletion.pdf_export_id=wfp.id AND deletion.status='deleted'
				  )
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
				COALESCE(u.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,
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
				  AND NOT EXISTS (
					SELECT 1
					FROM workflow_form_pdf_export_deletions deletion
					WHERE deletion.pdf_export_id=wfp.id AND deletion.status='deleted'
				  )
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
