package core

import (
	"context"
	"fmt"
)

// Source precedence belongs before LIMIT/OFFSET. Explicit chronological sorts
// retain their existing semantics; the legacy WorkItem endpoint is unchanged.
func myWorkQueueOrderSQL(sortKind string) string {
	order := myWorkItemsOrderSQL(sortKind) + ", entry_kind"
	if sortKind == "priority" || sortKind == "" {
		return "CASE WHEN entry_kind='accounting_handoff' THEN 0 ELSE 1 END, " + order
	}
	return order
}

// accountingWorkSelectSQL is shared by both views of the canonical handoff.
func accountingWorkSelectSQL() string {
	return `SELECT handoff.id AS handoff_id, handoff.id, handoff.project_id,
		project.code AS project_code, project.name AS project_name,
		handoff.direction, handoff.source_type, handoff.source_id,
		COALESCE(handoff.source_snapshot_json->>'source_code','') AS source_code,
		COALESCE(handoff.source_snapshot_json->>'source_title','') AS source_title,
		COALESCE(handoff.source_snapshot_json->>'counterparty_name','') AS counterparty_name,
		NULLIF(handoff.source_snapshot_json->>'reference_amount','')::numeric AS reference_amount,
		COALESCE(NULLIF(handoff.source_snapshot_json->>'currency',''),'VND') AS currency,
		COALESCE(handoff.source_snapshot_json->>'source_status','') AS source_status,
		CASE handoff.source_type
			WHEN 'procurement_request' THEN '/projects/' || handoff.project_id || '/procurement'
			WHEN 'contract_milestone' THEN '/projects/' || handoff.project_id || '/contracts'
			ELSE '/projects/' || handoff.project_id END AS source_route,
		handoff.status, handoff.note, handoff.external_reference, handoff.return_reason,
		handoff.handed_off_at, handoff.acknowledged_at, handoff.returned_at, handoff.updated_at,
		handed_off.full_name AS handed_off_by_name,
		acknowledged.full_name AS acknowledged_by_name, returned.full_name AS returned_by_name
	FROM accounting_handoffs handoff
	JOIN projects project ON project.id=handoff.project_id
	LEFT JOIN users handed_off ON handed_off.id=handoff.handed_off_by
	LEFT JOIN users acknowledged ON acknowledged.id=handoff.acknowledged_by
	LEFT JOIN users returned ON returned.id=handoff.returned_by`
}

// Only index columns are normalized. Payloads remain their original domain type;
// accounting records never become work_items and never acquire synthetic deadlines.
func (r *Repository) myWorkQueueSQL(ctx context.Context, actorID string, includeAccounting bool, args []any) (string, []any, error) {
	scope := "FALSE"
	if includeAccounting {
		var scopeArgs []any
		var err error
		scope, scopeArgs, err = r.ProjectAccessFilter(ctx, actorID, "project", len(args)+1)
		if err != nil {
			return "", nil, err
		}
		args = append(args, scopeArgs...)
	}
	return myWorkItemsBaseSQL() + fmt.Sprintf(`,
		accounting_work AS (%s WHERE project.status <> 'cancelled'
			AND handoff.status IN ('handed_off','returned','acknowledged') AND (%s)),
		queue_work AS (
			SELECT 'work_item'::text AS entry_kind, to_jsonb(w) AS payload,
				id, project_id, project_name, project_code, title, description,
				run_node_name, run_step_name, assignee_name, status, work_type, priority,
				due_at, created_at, completed_at, can_act, can_decide, is_future,
				is_responsible, is_mine, is_department, is_watching
			FROM base_work w
			UNION ALL
			SELECT 'accounting_handoff', to_jsonb(a),
				id, project_id, project_name, project_code, source_title,
				concat_ws(' ',source_code,counterparty_name,external_reference),
				NULL::text, NULL::text, NULL::text, status, NULL::text, NULL::text,
				NULL::timestamptz, handed_off_at, acknowledged_at,
				status='handed_off', false, false, true, false, false, false
			FROM accounting_work a
		)`, accountingWorkSelectSQL(), scope), args, nil
}
