package core

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type notificationCursor struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

func encodeNotificationCursor(createdAt any, id any) string {
	value := notificationCursor{CreatedAt: fmt.Sprint(createdAt), ID: cleanID(id)}
	raw, err := json.Marshal(value)
	if err != nil || value.CreatedAt == "" || value.ID == "" {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeNotificationCursor(value string) (notificationCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return notificationCursor{}, err
	}
	cursor := notificationCursor{}
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return notificationCursor{}, err
	}
	if cleanID(cursor.ID) == "" {
		return notificationCursor{}, errors.New("notification cursor id is required")
	}
	if _, err := time.Parse(time.RFC3339Nano, cursor.CreatedAt); err != nil {
		if _, fallbackErr := time.Parse(time.RFC3339, cursor.CreatedAt); fallbackErr != nil {
			return notificationCursor{}, err
		}
	}
	return cursor, nil
}

func (r *Repository) notificationVisibility(ctx context.Context, actorID string, start int, projectAlias string) (string, []any, error) {
	projectScope, scopeArgs, err := r.ProjectAccessFilter(ctx, actorID, "visible_project", start)
	if err != nil {
		return "", nil, err
	}
	permissionActorParam := start + len(scopeArgs)
	args := append(scopeArgs, actorID)
	return fmt.Sprintf(`(
		(notification.project_id IS NULL OR EXISTS (
			SELECT 1 FROM projects visible_project
			WHERE visible_project.id=notification.project_id
			  AND visible_project.status <> 'cancelled'
			  AND %s
		))
		AND (
			cardinality(notification.permissions_any_snapshot)=0
			OR EXISTS (
			SELECT 1
			FROM user_roles notification_assignment
			JOIN role_permissions notification_role_permission ON notification_role_permission.role_id=notification_assignment.role_id
			JOIN permissions notification_permission ON notification_permission.id=notification_role_permission.permission_id
			WHERE notification_assignment.user_id=$%d::uuid
			  AND (notification_assignment.project_id IS NULL OR notification_assignment.project_id=notification.project_id)
			  AND notification_permission.key=ANY(notification.permissions_any_snapshot)
			)
		)
	)`, projectScope, permissionActorParam), args, nil
}

func (r *Repository) UserNotifications(ctx context.Context, actorID string, filter NotificationFilter) (NotificationPage, error) {
	visibilitySQL, visibilityArgs, err := r.notificationVisibility(ctx, actorID, 2, "project")
	if err != nil {
		return NotificationPage{}, err
	}
	args := []any{actorID}
	args = append(args, visibilityArgs...)
	conditions := []string{
		"notification.recipient_user_id=$1::uuid",
		"(notification.expires_at IS NULL OR notification.expires_at > now())",
		visibilitySQL,
	}
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if filter.State == "unread" {
		conditions = append(conditions, "notification.archived_at IS NULL", "notification.read_at IS NULL")
	} else if filter.State == "action_required" {
		conditions = append(conditions, "notification.archived_at IS NULL", "notification.resolved_at IS NULL", "notification.intent='action_required'")
	} else if filter.State == "archived" {
		conditions = append(conditions, "notification.archived_at IS NOT NULL")
	} else {
		conditions = append(conditions, "notification.archived_at IS NULL")
	}
	if filter.EventKey != "" {
		conditions = append(conditions, "notification.event_key="+addArg(filter.EventKey))
	}
	if filter.Category != "" {
		conditions = append(conditions, "notification.category_key="+addArg(filter.Category))
	}
	if filter.ProjectID != "" {
		conditions = append(conditions, "notification.project_id="+addArg(filter.ProjectID)+"::uuid")
	}
	if filter.Search != "" {
		searchParam := addArg(filter.Search)
		conditions = append(conditions, fmt.Sprintf("(notification.title_snapshot ILIKE '%%'||%s||'%%' OR notification.message_snapshot ILIKE '%%'||%s||'%%')", searchParam, searchParam))
	}
	if filter.FromDate != "" {
		conditions = append(conditions, "notification.created_at >= "+addArg(filter.FromDate)+"::date")
	}
	if filter.ToDate != "" {
		conditions = append(conditions, "notification.created_at < "+addArg(filter.ToDate)+"::date + INTERVAL '1 day'")
	}
	if filter.Cursor != "" {
		cursor, cursorErr := decodeNotificationCursor(filter.Cursor)
		if cursorErr != nil {
			return NotificationPage{}, validationError("cursor thông báo không hợp lệ", Record{"cursor": filter.Cursor})
		}
		createdAtParam := addArg(cursor.CreatedAt)
		idParam := addArg(cursor.ID)
		conditions = append(conditions, fmt.Sprintf("(notification.created_at,notification.id) < (%s::timestamptz,%s::uuid)", createdAtParam, idParam))
	}
	limitParam := addArg(filter.Limit + 1)
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT notification.id::text AS id,
			COALESCE(notification.event_key,notification.notification_type) AS event_key,
			COALESCE(notification.event_key,notification.notification_type) AS notification_type,
			notification.category_key,
			notification.category_label,
			notification.intent,
			notification.severity,
			notification.icon_key,
			notification.tone_key,
			notification.source_kind,
			notification.source_id::text AS source_id,
			notification.source_event_id::text AS source_event_id,
			notification.project_id::text AS project_id,
			COALESCE(project.code,'') AS project_code,
			COALESCE(project.name,'') AS project_name,
			notification.actor_user_id::text AS actor_user_id,
			actor.full_name AS actor_name,
			COALESCE(actor.deleted_at IS NOT NULL,false) AS actor_account_deleted,
			notification.title_snapshot,
			notification.message_snapshot,
			notification.action_key,
			notification.action_payload_json,
			notification.action_href,
			notification.action_label,
			notification.read_at,
			notification.seen_at,
			notification.archived_at,
			notification.resolved_at,
			notification.created_at,
			to_char(notification.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"') AS notification_cursor_created_at
		FROM user_notifications notification
		LEFT JOIN projects project ON project.id=notification.project_id
		LEFT JOIN users actor ON actor.id=notification.actor_user_id
		WHERE %s
		ORDER BY notification.created_at DESC,notification.id DESC
		LIMIT %s
	`, strings.Join(conditions, " AND "), limitParam), args...)
	if err != nil {
		return NotificationPage{}, err
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	nextCursor := ""
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		nextCursor = encodeNotificationCursor(last["notification_cursor_created_at"], last["id"])
	}
	for _, row := range rows {
		delete(row, "notification_cursor_created_at")
	}
	return NotificationPage{Items: rows, HasMore: hasMore, NextCursor: nextCursor}, nil
}

func (r *Repository) UserNotificationSummary(ctx context.Context, actorID string) (NotificationSummary, error) {
	visibilitySQL, visibilityArgs, err := r.notificationVisibility(ctx, actorID, 2, "project")
	if err != nil {
		return NotificationSummary{}, err
	}
	args := []any{actorID}
	args = append(args, visibilityArgs...)
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT notification.category_key,
			COUNT(*) FILTER (WHERE notification.read_at IS NULL) AS unread_count,
			COUNT(*) FILTER (WHERE notification.intent='action_required' AND notification.resolved_at IS NULL) AS action_count
		FROM user_notifications notification
		LEFT JOIN projects project ON project.id=notification.project_id
		WHERE notification.recipient_user_id=$1::uuid
		  AND notification.archived_at IS NULL
		  AND (notification.expires_at IS NULL OR notification.expires_at > now())
		  AND %s
		GROUP BY notification.category_key
	`, visibilitySQL), args...)
	if err != nil {
		return NotificationSummary{}, err
	}
	summary := NotificationSummary{ByCategory: map[string]int{}}
	for _, row := range rows {
		unread := int(number(row["unread_count"]))
		summary.ByCategory[fmt.Sprint(row["category_key"])] = unread
		summary.UnreadTotal += unread
		summary.ActionRequired += int(number(row["action_count"]))
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COALESCE(revision,0) FROM notification_actor_revisions WHERE user_id=$1::uuid`, actorID).Scan(&summary.ActorRevision); errors.Is(err, sql.ErrNoRows) {
		summary.ActorRevision = 0
	} else if err != nil {
		return NotificationSummary{}, err
	}
	return summary, nil
}

func (r *Repository) MarkUserNotificationRead(ctx context.Context, notificationID, actorID string) (Record, error) {
	visibilitySQL, visibilityArgs, err := r.notificationVisibility(ctx, actorID, 3, "project")
	if err != nil {
		return nil, err
	}
	args := []any{notificationID, actorID}
	args = append(args, visibilityArgs...)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	row, err := queryOneRecord(ctx, tx, fmt.Sprintf(`
		UPDATE user_notifications notification
		SET read_at=COALESCE(read_at,now())
		WHERE notification.id=$1::uuid
		  AND notification.recipient_user_id=$2::uuid
		  AND %s
		RETURNING notification.id::text AS id,notification.read_at
	`, visibilitySQL), args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, forbiddenError("Thông báo không thuộc phạm vi hiện tại")
	}
	if err != nil {
		return nil, err
	}
	if err := bumpNotificationActorRevisionTx(ctx, tx, actorID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return row, nil
}

func (r *Repository) MarkAllUserNotificationsRead(ctx context.Context, actorID string) (Record, error) {
	visibilitySQL, visibilityArgs, err := r.notificationVisibility(ctx, actorID, 2, "project")
	if err != nil {
		return nil, err
	}
	args := []any{actorID}
	args = append(args, visibilityArgs...)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	row, err := queryOneRecord(ctx, tx, fmt.Sprintf(`
		WITH updated AS (
			UPDATE user_notifications notification
			SET read_at=now()
			WHERE notification.recipient_user_id=$1::uuid
			  AND notification.read_at IS NULL
			  AND notification.archived_at IS NULL
			  AND (notification.expires_at IS NULL OR notification.expires_at > now())
			  AND %s
			RETURNING notification.id
		)
		SELECT COUNT(*) AS updated_count FROM updated
	`, visibilitySQL), args...)
	if err != nil {
		return nil, err
	}
	if number(row["updated_count"]) > 0 {
		if err := bumpNotificationActorRevisionTx(ctx, tx, actorID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return row, nil
}

func (r *Repository) ArchiveUserNotification(ctx context.Context, notificationID, actorID string) (Record, error) {
	visibilitySQL, visibilityArgs, err := r.notificationVisibility(ctx, actorID, 3, "project")
	if err != nil {
		return nil, err
	}
	args := []any{notificationID, actorID}
	args = append(args, visibilityArgs...)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	row, err := queryOneRecord(ctx, tx, fmt.Sprintf(`
		UPDATE user_notifications notification
		SET archived_at=COALESCE(archived_at,now()),read_at=COALESCE(read_at,now())
		WHERE notification.id=$1::uuid
		  AND notification.recipient_user_id=$2::uuid
		  AND %s
		RETURNING notification.id::text AS id,notification.archived_at,notification.read_at
	`, visibilitySQL), args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, forbiddenError("Thông báo không thuộc phạm vi hiện tại")
	}
	if err != nil {
		return nil, err
	}
	if err := bumpNotificationActorRevisionTx(ctx, tx, actorID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return row, nil
}

func bumpNotificationActorRevisionTx(ctx context.Context, tx *sql.Tx, actorID string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO notification_actor_revisions (user_id,revision,updated_at)
		VALUES ($1::uuid,1,now())
		ON CONFLICT (user_id) DO UPDATE
		SET revision=notification_actor_revisions.revision+1,updated_at=now()
	`, actorID)
	return err
}

func (r *Repository) NotificationCatalog(ctx context.Context) ([]NotificationCatalogItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT event_key,domain_key,category_key,category_label,intent,severity,
			icon_key,tone_key,enabled,required_delivery,permissions_any
		FROM notification_event_types
		ORDER BY category_label,event_key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]NotificationCatalogItem, 0)
	for rows.Next() {
		item := NotificationCatalogItem{}
		if err := rows.Scan(&item.EventKey, &item.DomainKey, &item.CategoryKey, &item.CategoryLabel, &item.Intent, &item.Severity, &item.IconKey, &item.ToneKey, &item.Enabled, &item.RequiredDelivery, &item.Permissions); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) NotificationProjects(ctx context.Context, actorID string) ([]Record, error) {
	visibilitySQL, visibilityArgs, err := r.notificationVisibility(ctx, actorID, 2, "project")
	if err != nil {
		return nil, err
	}
	args := []any{actorID}
	args = append(args, visibilityArgs...)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT DISTINCT project.id::text AS id,project.code,project.name
		FROM user_notifications notification
		JOIN projects project ON project.id=notification.project_id
		WHERE notification.recipient_user_id=$1::uuid
		  AND notification.archived_at IS NULL
		  AND (notification.expires_at IS NULL OR notification.expires_at > now())
		  AND %s
		ORDER BY project.code,project.name
	`, visibilitySQL), args...)
}

func (r *Repository) NotificationPreferences(ctx context.Context, actorID string) ([]NotificationPreference, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT category_key,event_key,in_app_enabled,email_enabled,push_enabled,digest_mode,quiet_hours_json
		FROM notification_preferences
		WHERE user_id=$1::uuid
		ORDER BY category_key,event_key
	`, actorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]NotificationPreference, 0)
	for rows.Next() {
		item := NotificationPreference{}
		var quietRaw []byte
		if err := rows.Scan(&item.CategoryKey, &item.EventKey, &item.InApp, &item.Email, &item.Push, &item.DigestMode, &quietRaw); err != nil {
			return nil, err
		}
		item.QuietHours = map[string]any{}
		if len(quietRaw) > 0 {
			if err := json.Unmarshal(quietRaw, &item.QuietHours); err != nil {
				return nil, err
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) NotificationPreferenceTargetsRequiredEvent(ctx context.Context, categoryKey, eventKey string) (bool, error) {
	var required bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM notification_event_types
			WHERE required_delivery
			  AND ($1='*' OR category_key=$1)
			  AND ($2='*' OR event_key=$2)
		)
	`, categoryKey, eventKey).Scan(&required)
	return required, err
}

func (r *Repository) SaveNotificationPreference(ctx context.Context, actorID string, input NotificationPreference) (NotificationPreference, error) {
	quietRaw, err := json.Marshal(input.QuietHours)
	if err != nil {
		return input, err
	}
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO notification_preferences (
			user_id,category_key,event_key,in_app_enabled,email_enabled,push_enabled,digest_mode,quiet_hours_json,updated_at
		) VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8::jsonb,now())
		ON CONFLICT (user_id,category_key,event_key) DO UPDATE
		SET in_app_enabled=EXCLUDED.in_app_enabled,email_enabled=EXCLUDED.email_enabled,
			push_enabled=EXCLUDED.push_enabled,digest_mode=EXCLUDED.digest_mode,
			quiet_hours_json=EXCLUDED.quiet_hours_json,updated_at=now()
		RETURNING category_key,event_key,in_app_enabled,email_enabled,push_enabled,digest_mode,quiet_hours_json
	`, actorID, input.CategoryKey, input.EventKey, input.InApp, input.Email, input.Push, input.DigestMode, quietRaw).Scan(
		&input.CategoryKey, &input.EventKey, &input.InApp, &input.Email, &input.Push, &input.DigestMode, &quietRaw,
	)
	if err != nil {
		return input, err
	}
	input.QuietHours = map[string]any{}
	if len(quietRaw) > 0 {
		if err := json.Unmarshal(quietRaw, &input.QuietHours); err != nil {
			return input, err
		}
	}
	return input, nil
}
