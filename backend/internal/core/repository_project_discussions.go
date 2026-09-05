package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const projectDiscussionActivityTypesSQL = "'comment','status','blocker','question','decision','reminder','file_note'"

func (r *Repository) MyProjectDiscussions(ctx context.Context, profile workItemAccessProfile, filter ProjectDiscussionFilter, offset int) ([]Record, int, int, error) {
	args := projectOperationsScopeArgs(profile)
	actorPlaceholder := 3
	staffPlaceholder := 4
	args = append(args, workItemScopeArgs(profile, profileCanObserveAllProjectWork(profile), true)...)
	workScopeStart := 5
	args = append(args, profile.CanReconcileAllProjects, profile.IsAdmin || profile.IsDirector)
	canReconcilePlaceholder := len(args) - 1
	adminOrDirectorPlaceholder := len(args)

	projectConditions := []string{projectOperationsScopeSQL("p", 1)}
	if filter.ProjectID != "" {
		args = append(args, filter.ProjectID)
		projectConditions = append(projectConditions, fmt.Sprintf("p.id=$%d::uuid", len(args)))
	}
	if len(filter.ProjectIDs) > 0 {
		args = append(args, filter.ProjectIDs)
		projectConditions = append(projectConditions, fmt.Sprintf("p.id=ANY($%d::uuid[])", len(args)))
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		searchPlaceholder := len(args)
		projectConditions = append(projectConditions, fmt.Sprintf("(p.code ILIKE $%d OR p.name ILIKE $%d)", searchPlaceholder, searchPlaceholder))
	}
	args = append(args, offset, filter.Limit)
	offsetPlaceholder := len(args) - 1
	limitPlaceholder := len(args)

	visibilitySQL := "TRUE"
	if profileIsClient(profile) && !profile.IsManager {
		visibilitySQL = "wia.visibility='client'"
	}
	discussionUnreadSQL := fmt.Sprintf(`(
		wia.withdrawn_at IS NULL
		AND wia.actor_id IS DISTINCT FROM $%d::uuid
		AND (
			cursor.last_read_created_at IS NULL
			OR (wia.created_at, wia.id) > (cursor.last_read_created_at, COALESCE(cursor.last_read_activity_id, '00000000-0000-0000-0000-000000000000'::uuid))
		)
	)`, actorPlaceholder)

	rows, err := r.Query(ctx, fmt.Sprintf(`
		WITH scoped_projects AS (
			SELECT p.id, p.code, p.name, p.created_by
			FROM projects p
			WHERE %s
		), visible_discussion AS (
			SELECT
				wia.id,
				wia.project_id,
				wia.actor_id,
				wia.comment,
				wia.created_at,
				u.full_name AS actor_name,
				%s AS is_discussion_unread
			FROM work_item_activities wia
			JOIN scoped_projects project ON project.id=wia.project_id
			LEFT JOIN work_items wi ON wi.id=wia.work_item_id
			LEFT JOIN users u ON u.id=wia.actor_id
			LEFT JOIN project_discussion_read_cursors cursor ON cursor.project_id=wia.project_id AND cursor.user_id=$%d
			WHERE wia.activity_type IN (%s)
			  AND wia.withdrawn_at IS NULL
			  AND (wi.id IS NULL OR wi.status <> 'cancelled')
			  AND (%s)
			  AND (wia.scope_type <> 'work_item' OR (wi.id IS NOT NULL AND (%s)))
		), metrics AS (
			SELECT
				project_id,
				COUNT(*) FILTER (WHERE is_discussion_unread) AS unread_count,
				MAX(created_at) AS latest_activity_at
			FROM visible_discussion
			GROUP BY project_id
		), latest AS (
			SELECT DISTINCT ON (project_id)
				project_id,
				id AS latest_activity_id,
				comment AS latest_comment,
				actor_name AS latest_actor_name
			FROM visible_discussion
			ORDER BY project_id, created_at DESC, id DESC
		), project_rows AS (
			SELECT
				project.id AS project_id,
				project.code AS project_code,
				project.name AS project_name,
				COALESCE(metrics.unread_count, 0) AS unread_count,
				latest.latest_activity_id,
				metrics.latest_activity_at,
				latest.latest_comment,
				latest.latest_actor_name,
				COALESCE((
					NOT $%d::boolean
					OR $%d::boolean
					OR project.created_by=$%d::uuid
					OR EXISTS (
						SELECT 1 FROM project_members pm
						WHERE pm.project_id=project.id AND pm.staff_id=NULLIF($%d,'')::uuid AND pm.active
					)
					OR EXISTS (
						SELECT 1 FROM user_roles ur
						WHERE ur.project_id=project.id AND ur.user_id=$%d::uuid
					)
				), FALSE) AS can_post
			FROM scoped_projects project
			LEFT JOIN metrics ON metrics.project_id=project.id
			LEFT JOIN latest ON latest.project_id=project.id
		)
		SELECT
			project_rows.*,
			COUNT(*) OVER () AS filtered_total,
			COALESCE(SUM(unread_count) OVER (), 0) AS unread_total
		FROM project_rows
		ORDER BY (unread_count > 0) DESC, latest_activity_at DESC NULLS LAST, lower(project_name), project_code
		OFFSET $%d
		LIMIT $%d
	`, strings.Join(projectConditions, " AND "), discussionUnreadSQL, actorPlaceholder, projectDiscussionActivityTypesSQL, visibilitySQL, workItemScopeSQL("wi", workScopeStart), canReconcilePlaceholder, adminOrDirectorPlaceholder, actorPlaceholder, staffPlaceholder, actorPlaceholder, offsetPlaceholder, limitPlaceholder), args...)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(rows) == 0 {
		return rows, 0, 0, nil
	}
	return rows, int(number(rows[0]["filtered_total"])), int(number(rows[0]["unread_total"])), nil
}

func (r *Repository) MarkProjectDiscussionReadThrough(ctx context.Context, projectID, userID, activityID string) (Record, error) {
	return r.markProjectActivityReadThrough(ctx, projectID, userID, activityID, false)
}

func (r *Repository) markProjectActivityReadThrough(ctx context.Context, projectID, userID, activityID string, system bool) (Record, error) {
	table := "project_discussion_read_cursors"
	kind := "activity_type IN (" + projectDiscussionActivityTypesSQL + ")"
	if system {
		table = "project_system_activity_read_cursors"
		kind = "activity_type NOT IN (" + projectDiscussionActivityTypesSQL + ")"
	}
	row, err := r.QueryOne(ctx, fmt.Sprintf(`
		WITH candidate AS (
			SELECT id, project_id, created_at
			FROM work_item_activities
			WHERE id=$3::uuid
			  AND project_id=$1::uuid
			  AND %[1]s
			  AND withdrawn_at IS NULL
		), upserted AS (
			INSERT INTO %[2]s (
				project_id, user_id, last_read_activity_id, last_read_created_at, updated_at
			)
			SELECT project_id, $2::uuid, id, created_at, now()
			FROM candidate
			ON CONFLICT (project_id, user_id) DO UPDATE
			SET last_read_activity_id=EXCLUDED.last_read_activity_id,
				last_read_created_at=EXCLUDED.last_read_created_at,
				updated_at=now()
			WHERE (EXCLUDED.last_read_created_at, EXCLUDED.last_read_activity_id) >
				(%[2]s.last_read_created_at, COALESCE(%[2]s.last_read_activity_id, '00000000-0000-0000-0000-000000000000'::uuid))
			RETURNING *
		)
		SELECT * FROM upserted
		UNION ALL
		SELECT cursor.*
		FROM %[2]s cursor
		WHERE cursor.project_id=$1::uuid AND cursor.user_id=$2::uuid
		  AND NOT EXISTS (SELECT 1 FROM upserted)
		LIMIT 1
	`, kind, table), projectID, userID, activityID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, validationError("Hoạt động trao đổi không tồn tại hoặc đã được thu hồi", Record{"activity_id": activityID})
		}
		return nil, err
	}
	return row, nil
}
