package core

import (
	"context"
	"fmt"
)

func (r *Repository) nativeDailyReports(ctx context.Context, projectID, actorID string, filter NativeDailyReportFilter, limit int) ([]Record, error) {
	profile, err := r.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	args := []any{projectID, filter.DateFrom, filter.DateTo}
	args = append(args, workItemScopeArgs(profile, profileCanObserveAllProjectWork(profile), true)...)
	pagination := ""
	if limit > 0 {
		args = append(args, limit)
		pagination = fmt.Sprintf(" LIMIT $%d", len(args))
	}
	return r.Query(ctx, `SELECT source.*,source.id::text AS source_id,source.project_id::text AS project_id,
		project.code AS project_code,project.name AS project_name,project.address AS project_address,
		author.full_name AS author_name,source.reported_by::text AS author_id,wi.title AS work_item_title,
		delivery.id::text AS work_report_id,delivery.status AS delivery_status,
		delivery.recipient_id::text AS delivery_recipient_id,recipient.full_name AS delivery_recipient_name,
		delivery.current_revision_no AS delivery_revision_no,delivery.review_comment AS delivery_review_comment,
		revision.snapshot_sha256 AS delivery_snapshot_sha256,revision.snapshot_json AS delivery_snapshot_json,
		(source.work_item_id IS NOT NULL AND EXISTS(SELECT 1 FROM evidence_records evidence
		 WHERE evidence.id::text IN (SELECT jsonb_array_elements_text(source.photos))
		 AND evidence.project_id=source.project_id AND evidence.work_item_id=source.work_item_id
		 AND evidence.evidence_type='photo' AND evidence.status NOT IN ('rejected','cancelled','deleted'))) AS runtime_compliant
		FROM operation_daily_reports source JOIN projects project ON project.id=source.project_id
		LEFT JOIN users author ON author.id=source.reported_by LEFT JOIN work_items wi ON wi.id=source.work_item_id
		LEFT JOIN work_reports delivery ON delivery.source_kind='operation_daily_report' AND delivery.source_id=source.id
		LEFT JOIN work_report_revisions revision ON revision.report_id=delivery.id AND revision.revision_no=delivery.current_revision_no
		LEFT JOIN users recipient ON recipient.id=delivery.recipient_id
		WHERE source.project_id=$1::uuid AND project.status <> 'cancelled'
		AND ($2='' OR source.report_date >= NULLIF($2,'')::date) AND ($3='' OR source.report_date <= NULLIF($3,'')::date)
		AND (source.work_item_id IS NULL OR (wi.status <> 'cancelled' AND (`+workItemScopeSQL("wi", 4)+`)))
		ORDER BY source.report_date DESC,source.created_at DESC,source.id`+pagination, args...)
}

func (r *Repository) nativeConstructionActions(ctx context.Context, projectID, actorID string, filter NativeConstructionActionFilter) ([]Record, error) {
	profile, err := r.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	args := []any{projectID, filter.Queue, filter.Search, filter.AssigneeID, filter.Priority, filter.ImpactScope}
	args = append(args, workItemScopeArgs(profile, profileCanObserveAllProjectWork(profile), true)...)
	return r.Query(ctx, `SELECT action.id::text FROM construction_action_items action
	 LEFT JOIN construction_checks check_source ON check_source.id=action.source_check_id AND check_source.project_id=action.project_id
	 LEFT JOIN work_items wi ON wi.id=COALESCE(action.work_item_id,check_source.work_item_id)
	 LEFT JOIN users assignee ON assignee.id=action.assigned_user_id
	 WHERE action.project_id=$1::uuid AND action.status <> 'cancelled'
	 AND ($2='all' OR action.status=$2) AND ($3='' OR position(lower($3) IN lower(concat_ws(' ',action.title,action.description,wi.title,check_source.title,assignee.full_name)))>0)
	 AND ($4='' OR action.assigned_user_id=NULLIF($4,'')::uuid) AND ($5='' OR action.priority=$5) AND ($6='' OR action.impact_scope=$6)
	 AND (COALESCE(action.work_item_id,check_source.work_item_id) IS NULL OR (wi.status <> 'cancelled' AND (`+workItemScopeSQL("wi", 7)+`)))
	 ORDER BY CASE action.status WHEN 'open' THEN 0 WHEN 'waiting_confirmation' THEN 1 ELSE 2 END,
	 CASE WHEN action.status <> 'done' AND action.priority='urgent' THEN 0 ELSE 1 END,
	 action.due_at ASC NULLS LAST,action.created_at DESC,action.id LIMIT 5001`, args...)
}
