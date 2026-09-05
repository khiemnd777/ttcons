package core

import (
	"context"
	"fmt"
	"strings"
)

func dashboardActivitiesSQL(profile workItemAccessProfile, q dashboardActivityQuery) (string, []any) {
	args := projectOperationsScopeArgs(profile)
	args = append(args, workItemScopeArgs(profile, profileCanObserveAllProjectWork(profile), true)...)
	conditions := []string{
		"wia.withdrawn_at IS NULL",
		"wia.activity_type NOT IN (" + projectDiscussionActivityTypesSQL + ")",
		fmt.Sprintf("(wia.work_item_id IS NULL OR (wi.id IS NOT NULL AND wi.project_id=p.id AND wi.status <> 'cancelled' AND (%s)))", workItemScopeSQL("wi", 5)),
		"(wia.scope_type <> 'work_item' OR wi.id IS NOT NULL)",
	}
	if profileIsClient(profile) && !profile.IsManager {
		conditions = append(conditions, "wia.visibility='client'")
	}
	args = append(args, q.AsOf)
	conditions = append(conditions, fmt.Sprintf("wia.created_at <= $%d", len(args)))
	if !q.From.IsZero() {
		args = append(args, q.From)
		conditions = append(conditions, fmt.Sprintf("wia.created_at >= $%d", len(args)))
	}
	if q.ActivityType != "" {
		args = append(args, q.ActivityType)
		conditions = append(conditions, fmt.Sprintf("wia.activity_type=$%d", len(args)))
	}
	if q.Search != "" {
		args = append(args, "%"+q.Search+"%")
		conditions = append(conditions, fmt.Sprintf(`(p.code ILIKE $%[1]d OR p.name ILIKE $%[1]d OR wia.comment ILIKE $%[1]d OR wi.title ILIKE $%[1]d OR u.full_name ILIKE $%[1]d)`, len(args)))
	}
	if q.BeforeID != "" {
		args = append(args, q.Before, q.BeforeID)
		conditions = append(conditions, fmt.Sprintf("(wia.created_at,wia.id) < ($%d::timestamptz,$%d::uuid)", len(args)-1, len(args)))
	}
	args = append(args, q.Limit+1)
	return fmt.Sprintf(`WITH scoped_projects AS MATERIALIZED (
		SELECT p.id,p.code,p.name FROM projects p WHERE %s
	) SELECT wia.id::text, p.id::text, p.code, p.name,
		COALESCE(wia.work_item_id::text,''), COALESCE(wi.title,p.name),
		COALESCE(u.full_name,''), COALESCE(u.deleted_at IS NOT NULL,false),
		wia.activity_type, COALESCE(wia.comment,''), COALESCE(wia.from_status,''), COALESCE(wia.to_status,''),
		to_char(wia.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
		COALESCE(wia.metadata_json->>'form_version_id','')
		FROM work_item_activities wia
		JOIN scoped_projects p ON p.id=wia.project_id
		LEFT JOIN work_items wi ON wi.id=wia.work_item_id
		LEFT JOIN users u ON u.id=wia.actor_id
		WHERE %s ORDER BY wia.created_at DESC,wia.id DESC LIMIT $%d`, projectOperationsScopeSQL("p", 1), strings.Join(conditions, " AND "), len(args)), args
}

func (r *Repository) DashboardActivities(ctx context.Context, profile workItemAccessProfile, q dashboardActivityQuery) (DashboardActivitiesPage, error) {
	page := DashboardActivitiesPage{Items: []DashboardActivityItem{}, CanViewActivity: true}
	tx, err := r.beginReadTransactionWithJITDisabled(ctx)
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	query, args := dashboardActivitiesSQL(profile, q)
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var item DashboardActivityItem
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.ProjectCode, &item.ProjectName, &item.WorkItemID, &item.TargetLabel,
			&item.ActorName, &item.ActorAccountDeleted, &item.ActivityType, &item.Comment, &item.FromStatus, &item.ToStatus, &item.CreatedAt, &item.FormVersionID); err != nil {
			return page, err
		}
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	return page, tx.Commit()
}
