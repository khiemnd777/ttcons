package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"consflow/backend/internal/shared/response"
	"github.com/gofiber/fiber/v2"
)

// Shared with the activity feed. Mention attention remains a separate read model.
func projectDiscussionUnreadSQL(actor string) string {
	return fmt.Sprintf(`(wia.withdrawn_at IS NULL AND wia.activity_type IN (%s)
		AND wia.actor_id IS DISTINCT FROM %s::uuid
		AND (discussion_cursor.last_read_created_at IS NULL OR
			(wia.created_at,wia.id) > (discussion_cursor.last_read_created_at,
			COALESCE(discussion_cursor.last_read_activity_id,'00000000-0000-0000-0000-000000000000'::uuid))))`, projectDiscussionActivityTypesSQL, actor)
}

func projectSystemUnreadSQL(actor string) string {
	return fmt.Sprintf(`(wia.withdrawn_at IS NULL
		AND wia.activity_type NOT IN (%s)
		AND wia.actor_id IS DISTINCT FROM %s::uuid
		AND wia.created_at > tracking.enabled_at
		AND (system_cursor.last_read_created_at IS NULL OR
			(wia.created_at,wia.id) > (system_cursor.last_read_created_at,
			COALESCE(system_cursor.last_read_activity_id,'00000000-0000-0000-0000-000000000000'::uuid))))`, projectDiscussionActivityTypesSQL, actor)
}

func normalizeProjectUpdateIDs(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	if strings.TrimSpace(raw) == "" || len(parts) > 100 {
		return nil, validationError("Chọn từ 1 đến 100 công trình để tải cập nhật")
	}
	ids := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		id := strings.ToLower(strings.TrimSpace(part))
		if !validActivityUUID(id) {
			return nil, validationError("Mã công trình không hợp lệ")
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids, nil
}

func (h *Handler) ProjectUpdatesSummary(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"project.view"}); err != nil {
		return err
	}
	ids, err := normalizeProjectUpdateIDs(c.Query("project_ids"))
	if err != nil {
		return err
	}
	items, err := h.service.ProjectUpdatesSummary(c.Context(), ids, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, Record{"items": items})
}

func (h *Handler) MarkProjectSystemActivityReadThrough(c *fiber.Ctx) error {
	req := ProjectDiscussionReadRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đánh dấu đã đọc không hợp lệ")
	}
	result, err := h.service.markProjectActivityReadThrough(c.Context(), c.Params("id"), req, actorID(c), true)
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (s *Service) ProjectUpdatesSummary(ctx context.Context, ids []string, actorID string) ([]Record, error) {
	ids, err := normalizeProjectUpdateIDs(strings.Join(ids, ","))
	if err != nil {
		return nil, err
	}
	// Reuse the discussion scope, including current project/work-item permissions.
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	projects, _, _, err := s.repo.MyProjectDiscussions(ctx, profile, ProjectDiscussionFilter{ProjectIDs: ids, Limit: len(ids)}, 0)
	if err != nil {
		return nil, err
	}
	items := make([]Record, 0, len(projects))
	visibleIDs := make([]string, 0, len(projects))
	byID := make(map[string]Record, len(projects))
	for _, project := range projects {
		id := cleanID(project["project_id"])
		item := Record{"project_id": id, "notification_unread": 0, "discussion_unread": project["unread_count"], "system_unread": 0, "latest": nil}
		items = append(items, item)
		visibleIDs = append(visibleIDs, id)
		byID[id] = item
	}
	if len(visibleIDs) == 0 {
		return items, nil
	}
	activity, err := s.repo.projectUpdateActivityMetrics(ctx, profile, visibleIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range activity {
		item := byID[cleanID(row["project_id"])]
		item["system_unread"] = row["system_unread"]
		item["latest"] = row["latest"]
	}
	notifications, err := s.repo.projectUpdateNotificationMetrics(ctx, actorID, visibleIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range notifications {
		item := byID[cleanID(row["project_id"])]
		item["notification_unread"] = row["unread_count"]
		// Both values use the same UTC, microsecond format for deterministic comparison.
		latest, _ := item["latest"].(map[string]any)
		next, _ := row["latest"].(map[string]any)
		if next != nil && (latest == nil || fmt.Sprint(next["created_at"]) > fmt.Sprint(latest["created_at"])) {
			item["latest"] = next
		}
	}
	return items, nil
}

func (r *Repository) projectUpdateActivityMetrics(ctx context.Context, profile workItemAccessProfile, ids []string) ([]Record, error) {
	args := append([]any{ids, profile.UserID}, workItemScopeArgs(profile, profileCanObserveAllProjectWork(profile), true)...)
	visibility := "TRUE"
	if profileIsClient(profile) && !profile.IsManager {
		visibility = "wia.visibility='client'"
	}
	return r.Query(ctx, fmt.Sprintf(`
		WITH visible AS (
			SELECT wia.*, wi.title AS work_item_title, u.full_name AS actor_name,
				%s AS system_unread
			FROM work_item_activities wia
			LEFT JOIN work_items wi ON wi.id=wia.work_item_id
			LEFT JOIN users u ON u.id=wia.actor_id
			LEFT JOIN project_system_activity_read_cursors system_cursor ON system_cursor.project_id=wia.project_id AND system_cursor.user_id=$2
			CROSS JOIN project_activity_tracking_start tracking
			WHERE wia.project_id=ANY($1::uuid[]) AND wia.withdrawn_at IS NULL
				AND (wi.id IS NULL OR wi.status <> 'cancelled') AND (%s)
				AND (wia.scope_type <> 'work_item' OR (wi.id IS NOT NULL AND (%s)))
		), metrics AS (
			SELECT project_id, COUNT(*) FILTER (WHERE system_unread) AS system_unread FROM visible GROUP BY project_id
		), latest AS (
			SELECT DISTINCT ON (project_id) * FROM visible ORDER BY project_id,created_at DESC,id DESC
		)
		SELECT metrics.project_id, metrics.system_unread,
			jsonb_build_object('kind', CASE WHEN latest.activity_type IN (%s) THEN 'discussion' ELSE 'system' END,
				'created_at',to_char(latest.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
				'comment',latest.comment,'actor_name',latest.actor_name,'activity_type',latest.activity_type,
				'from_status',latest.from_status,'to_status',latest.to_status,'target_label',latest.work_item_title) AS latest
		FROM metrics JOIN latest USING (project_id)
	`, projectSystemUnreadSQL("$2"), visibility, workItemScopeSQL("wi", 3), projectDiscussionActivityTypesSQL), args...)
}

func (r *Repository) projectUpdateNotificationMetrics(ctx context.Context, actorID string, ids []string) ([]Record, error) {
	visibility, scopeArgs, err := r.notificationVisibility(ctx, actorID, 3, "project")
	if err != nil {
		return nil, err
	}
	args := append([]any{actorID, ids}, scopeArgs...)
	return r.Query(ctx, fmt.Sprintf(`
		WITH visible AS (
			SELECT notification.* FROM user_notifications notification
			WHERE notification.recipient_user_id=$1::uuid AND notification.project_id=ANY($2::uuid[])
				AND notification.archived_at IS NULL AND (notification.expires_at IS NULL OR notification.expires_at > now())
				AND %s
		), metrics AS (
			SELECT project_id, COUNT(*) FILTER (WHERE read_at IS NULL) AS unread_count FROM visible GROUP BY project_id
		), latest AS (
			SELECT DISTINCT ON (project_id) * FROM visible ORDER BY project_id,created_at DESC,id DESC
		)
		SELECT metrics.project_id, metrics.unread_count,
			jsonb_build_object('kind','notification','comment',latest.title_snapshot,
				'created_at',to_char(latest.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')) AS latest
		FROM metrics JOIN latest USING (project_id)
	`, visibility), args...)
}
