package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	notificationDispatchBatchSize = 50
	notificationDispatchAttempts  = 8
)

var notificationTemplateToken = regexp.MustCompile(`\{\{([a-z][a-z0-9_]*)\}\}`)

type notificationProjection struct {
	EventID             string
	EventKey            string
	AggregateType       string
	AggregateID         string
	SourceEventID       string
	ProjectID           string
	ActorUserID         string
	Payload             Record
	Audience            Record
	CorrelationKey      string
	OccurredAt          time.Time
	CategoryKey         string
	CategoryLabel       string
	Intent              string
	Severity            string
	IconKey             string
	ToneKey             string
	AudiencePolicyKey   string
	VisibilityPolicyKey string
	PermissionsAny      []string
	ActionRouteTemplate string
	ActionLabel         string
	RequiredDelivery    bool
	RetentionDays       int
	TitleTemplate       string
	BodyTemplate        string
	RequiredPayloadKeys []string
}

func emitNotificationEventTx(ctx context.Context, tx *sql.Tx, event NotificationDomainEvent) (string, error) {
	event.Key = strings.ToLower(strings.TrimSpace(event.Key))
	event.AggregateType = strings.ToLower(strings.TrimSpace(event.AggregateType))
	event.AggregateID = cleanID(event.AggregateID)
	event.SourceEventID = cleanID(event.SourceEventID)
	event.ProjectID = cleanID(event.ProjectID)
	event.ActorUserID = cleanID(event.ActorUserID)
	event.CorrelationKey = strings.TrimSpace(event.CorrelationKey)
	event.IdempotencyKey = strings.TrimSpace(event.IdempotencyKey)
	if !notificationKeyPattern.MatchString(event.Key) {
		return "", fmt.Errorf("notification event key is invalid: %q", event.Key)
	}
	if event.AggregateType == "" || !validActivityUUID(event.AggregateID) {
		return "", errors.New("notification aggregate is invalid")
	}
	if event.SourceEventID != "" && !validActivityUUID(event.SourceEventID) {
		return "", errors.New("notification source event is invalid")
	}
	if event.ProjectID != "" && !validActivityUUID(event.ProjectID) {
		return "", errors.New("notification project is invalid")
	}
	if event.ActorUserID != "" && !validActivityUUID(event.ActorUserID) {
		return "", errors.New("notification actor is invalid")
	}
	if event.IdempotencyKey == "" || len(event.IdempotencyKey) > 300 {
		return "", errors.New("notification idempotency key is invalid")
	}
	if event.Payload == nil {
		event.Payload = Record{}
	}
	if event.Audience == nil {
		event.Audience = Record{}
	}
	payloadRaw, err := json.Marshal(event.Payload)
	if err != nil {
		return "", err
	}
	audienceRaw, err := json.Marshal(event.Audience)
	if err != nil {
		return "", err
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	var eventID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO notification_events (
			event_key,aggregate_type,aggregate_id,source_event_id,project_id,actor_user_id,
			payload_json,audience_json,correlation_key,idempotency_key,occurred_at
		)
		SELECT catalog.event_key,$2,$3::uuid,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,
			$7::jsonb,$8::jsonb,NULLIF($9,''),$10,$11
		FROM notification_event_types catalog
		WHERE catalog.event_key=$1 AND catalog.enabled
		ON CONFLICT (idempotency_key) DO UPDATE SET id=notification_events.id
		RETURNING id::text
	`, event.Key, event.AggregateType, event.AggregateID, event.SourceEventID, event.ProjectID, event.ActorUserID,
		payloadRaw, audienceRaw, event.CorrelationKey, event.IdempotencyKey, event.OccurredAt).Scan(&eventID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("notification event is unknown or disabled: %s", event.Key)
	}
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO notification_outbox (event_id)
		VALUES ($1::uuid)
		ON CONFLICT (event_id) DO NOTHING
	`, eventID); err != nil {
		return "", err
	}
	return eventID, nil
}

func (s *Service) StartNotificationDispatcher(parent context.Context) func() {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		dispatchTicker := time.NewTicker(350 * time.Millisecond)
		defer dispatchTicker.Stop()
		cleanupTicker := time.NewTicker(30 * time.Minute)
		defer cleanupTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-dispatchTicker.C:
				if err := s.dispatchNotificationEvents(ctx); err != nil && ctx.Err() == nil {
					log.Printf("event=notification_dispatch_failed error=%q", sanitizeNotificationError(err))
				}
			case <-cleanupTicker.C:
				if err := s.cleanupNotificationOutbox(ctx); err != nil && ctx.Err() == nil {
					log.Printf("event=notification_cleanup_failed error=%q", sanitizeNotificationError(err))
				}
			}
		}
	}()
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
}

func (s *Service) dispatchNotificationEvents(ctx context.Context) error {
	if _, err := s.repo.db.ExecContext(ctx, `
		UPDATE notification_outbox
		SET status='retry',next_attempt_at=now(),locked_at=NULL,last_error='stale processing lease recovered'
		WHERE status='processing' AND locked_at < now()-INTERVAL '2 minutes'
	`); err != nil {
		return err
	}
	for processed := 0; processed < notificationDispatchBatchSize; processed++ {
		outboxID, eventID, err := s.claimNotificationOutbox(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		recipients, projectID, projectErr := s.projectNotificationEvent(ctx, outboxID, eventID)
		if projectErr != nil {
			if err := s.recordNotificationDispatchFailure(ctx, outboxID, eventID, projectErr); err != nil {
				return err
			}
			continue
		}
		for _, recipientID := range recipients {
			s.publishPersonalInvalidation("notification_changed", recipientID, projectID)
		}
	}
	return nil
}

func (s *Service) claimNotificationOutbox(ctx context.Context) (int64, string, error) {
	tx, err := s.repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback()
	var outboxID int64
	var eventID string
	err = tx.QueryRowContext(ctx, `
		WITH candidate AS (
			SELECT id FROM notification_outbox
			WHERE status IN ('pending','retry') AND next_attempt_at <= now()
			ORDER BY id
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE notification_outbox outbox
		SET status='processing',locked_at=now()
		FROM candidate
		WHERE outbox.id=candidate.id
		RETURNING outbox.id,outbox.event_id::text
	`).Scan(&outboxID, &eventID)
	if err != nil {
		return 0, "", err
	}
	if err := tx.Commit(); err != nil {
		return 0, "", err
	}
	return outboxID, eventID, nil
}

func (s *Service) projectNotificationEvent(ctx context.Context, outboxID int64, eventID string) ([]string, string, error) {
	tx, err := s.repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()
	projection, err := loadNotificationProjection(ctx, tx, eventID)
	if err != nil {
		return nil, "", err
	}
	title, err := renderNotificationText(projection.TitleTemplate, projection.Payload, projection.RequiredPayloadKeys)
	if err != nil {
		return nil, projection.ProjectID, fmt.Errorf("render title for %s: %w", projection.EventKey, err)
	}
	body, err := renderNotificationText(projection.BodyTemplate, projection.Payload, projection.RequiredPayloadKeys)
	if err != nil {
		return nil, projection.ProjectID, fmt.Errorf("render body for %s: %w", projection.EventKey, err)
	}
	if len([]rune(title)) > 200 || len([]rune(body)) > 2000 {
		return nil, projection.ProjectID, fmt.Errorf("rendered notification exceeds safe length for %s", projection.EventKey)
	}
	actionHref, err := renderNotificationRoute(projection.ActionRouteTemplate, projection.Payload)
	if err != nil {
		return nil, projection.ProjectID, fmt.Errorf("render action for %s: %w", projection.EventKey, err)
	}
	recipients, err := s.resolveNotificationRecipients(ctx, tx, projection)
	if err != nil {
		return nil, projection.ProjectID, err
	}
	payloadRaw, err := json.Marshal(projection.Payload)
	if err != nil {
		return nil, projection.ProjectID, err
	}
	invalidatedRecipients := map[string]bool{}
	if boolValue(projection.Audience["resolve_thread_for_all"]) && projection.CorrelationKey != "" {
		rows, err := tx.QueryContext(ctx, `
			UPDATE user_notifications
			SET resolved_at=COALESCE(resolved_at,$2)
			WHERE thread_key=$1 AND intent='action_required' AND resolved_at IS NULL
			RETURNING recipient_user_id::text
		`, projection.CorrelationKey, projection.OccurredAt)
		if err != nil {
			return nil, projection.ProjectID, err
		}
		for rows.Next() {
			var recipientID string
			if err := rows.Scan(&recipientID); err != nil {
				rows.Close()
				return nil, projection.ProjectID, err
			}
			invalidatedRecipients[recipientID] = true
		}
		if err := rows.Close(); err != nil {
			return nil, projection.ProjectID, err
		}
		if err := rows.Err(); err != nil {
			return nil, projection.ProjectID, err
		}
		for recipientID := range invalidatedRecipients {
			if err := bumpNotificationActorRevisionTx(ctx, tx, recipientID); err != nil {
				return nil, projection.ProjectID, err
			}
		}
	}
	for _, resolveID := range notificationAudienceUserIDs(projection.Audience, "resolve_user_ids") {
		result, err := tx.ExecContext(ctx, `
			UPDATE user_notifications
			SET resolved_at=COALESCE(resolved_at,$3)
			WHERE recipient_user_id=$1::uuid AND thread_key=$2
			  AND intent='action_required' AND resolved_at IS NULL
		`, resolveID, projection.CorrelationKey, projection.OccurredAt)
		if err != nil {
			return nil, projection.ProjectID, err
		}
		updated, err := result.RowsAffected()
		if err != nil {
			return nil, projection.ProjectID, err
		}
		if updated > 0 {
			if err := bumpNotificationActorRevisionTx(ctx, tx, resolveID); err != nil {
				return nil, projection.ProjectID, err
			}
			invalidatedRecipients[resolveID] = true
		}
	}
	for _, recipientID := range recipients {
		if projection.Intent == "outcome" && projection.CorrelationKey != "" {
			if _, err := tx.ExecContext(ctx, `
				UPDATE user_notifications
				SET resolved_at=COALESCE(resolved_at,$3)
				WHERE recipient_user_id=$1::uuid AND thread_key=$2
				  AND intent='action_required' AND resolved_at IS NULL
			`, recipientID, projection.CorrelationKey, projection.OccurredAt); err != nil {
				return nil, projection.ProjectID, err
			}
		}
		sourceEventID := projection.SourceEventID
		if sourceEventID == "" {
			sourceEventID = projection.EventID
		}
		var notificationID string
		err := tx.QueryRowContext(ctx, `
			INSERT INTO user_notifications (
				recipient_user_id,actor_user_id,project_id,notification_type,event_id,event_key,
				category_key,category_label,intent,severity,icon_key,tone_key,
				source_kind,source_id,source_event_id,title_snapshot,message_snapshot,
				action_key,action_payload_json,action_href,action_label,permissions_any_snapshot,
				visibility_policy_key_snapshot,idempotency_key,thread_key,expires_at,created_at
			) VALUES (
				$1::uuid,NULLIF($2,'')::uuid,NULLIF($3,'')::uuid,$4,$5::uuid,$4,
				$6,$7,$8,$9,$10,$11,
				$12,$13::uuid,$14::uuid,$15,$16,
				'open_notification_target',$17::jsonb,$18,$19,$20::text[],
				$21,$22,NULLIF($23,''),$24::timestamptz + make_interval(days => $25::int),$24::timestamptz
			)
			ON CONFLICT (recipient_user_id,idempotency_key) DO NOTHING
			RETURNING id::text
		`, recipientID, projection.ActorUserID, projection.ProjectID, projection.EventKey, projection.EventID,
			projection.CategoryKey, projection.CategoryLabel, projection.Intent, projection.Severity, projection.IconKey, projection.ToneKey,
			projection.AggregateType, projection.AggregateID, sourceEventID, title, body, payloadRaw, actionHref,
			projection.ActionLabel, projection.PermissionsAny, projection.VisibilityPolicyKey,
			"notification_event:"+projection.EventID+":"+recipientID, projection.CorrelationKey, projection.OccurredAt, projection.RetentionDays,
		).Scan(&notificationID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, projection.ProjectID, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO notification_delivery_attempts (notification_id,channel,status,attempt_count,delivered_at)
			VALUES ($1::uuid,'in_app','delivered',1,now())
			ON CONFLICT (notification_id,channel) DO NOTHING
		`, notificationID); err != nil {
			return nil, projection.ProjectID, err
		}
		if err := bumpNotificationActorRevisionTx(ctx, tx, recipientID); err != nil {
			return nil, projection.ProjectID, err
		}
		invalidatedRecipients[recipientID] = true
	}
	if _, err := tx.ExecContext(ctx, `UPDATE notification_events SET processed_at=now(),last_error=NULL WHERE id=$1::uuid`, eventID); err != nil {
		return nil, projection.ProjectID, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE notification_outbox
		SET status='completed',completed_at=now(),locked_at=NULL,last_error=NULL
		WHERE id=$1 AND event_id=$2::uuid
	`, outboxID, eventID); err != nil {
		return nil, projection.ProjectID, err
	}
	if err := tx.Commit(); err != nil {
		return nil, projection.ProjectID, err
	}
	return sortedNotificationRecipients(mapKeys(invalidatedRecipients)), projection.ProjectID, nil
}

func loadNotificationProjection(ctx context.Context, tx *sql.Tx, eventID string) (notificationProjection, error) {
	projection := notificationProjection{}
	var payloadRaw, audienceRaw, permissionsRaw, requiredPayloadKeysRaw []byte
	err := tx.QueryRowContext(ctx, `
		SELECT event.id::text,event.event_key,event.aggregate_type,event.aggregate_id::text,
			COALESCE(event.source_event_id::text,''),COALESCE(event.project_id::text,''),
			COALESCE(event.actor_user_id::text,''),event.payload_json,event.audience_json,
			COALESCE(event.correlation_key,''),event.occurred_at,
			catalog.category_key,catalog.category_label,catalog.intent,catalog.severity,
			catalog.icon_key,catalog.tone_key,catalog.audience_policy_key,catalog.visibility_policy_key,
			array_to_json(catalog.permissions_any)::text,catalog.action_route_template,catalog.action_label,
			catalog.required_delivery,catalog.retention_days,
			template.title_template,template.body_template,array_to_json(template.required_payload_keys)::text
		FROM notification_events event
		JOIN notification_event_types catalog ON catalog.event_key=event.event_key AND catalog.enabled
		JOIN LATERAL (
			SELECT title_template,body_template,required_payload_keys
			FROM notification_templates
			WHERE event_key=event.event_key AND locale=catalog.default_locale AND active
			ORDER BY version DESC LIMIT 1
		) template ON true
		WHERE event.id=$1::uuid
		FOR UPDATE OF event
	`, eventID).Scan(
		&projection.EventID, &projection.EventKey, &projection.AggregateType, &projection.AggregateID,
		&projection.SourceEventID, &projection.ProjectID, &projection.ActorUserID, &payloadRaw, &audienceRaw,
		&projection.CorrelationKey, &projection.OccurredAt, &projection.CategoryKey, &projection.CategoryLabel,
		&projection.Intent, &projection.Severity, &projection.IconKey, &projection.ToneKey,
		&projection.AudiencePolicyKey, &projection.VisibilityPolicyKey, &permissionsRaw,
		&projection.ActionRouteTemplate, &projection.ActionLabel, &projection.RequiredDelivery,
		&projection.RetentionDays, &projection.TitleTemplate, &projection.BodyTemplate, &requiredPayloadKeysRaw,
	)
	if err != nil {
		return projection, err
	}
	projection.Payload = Record{}
	projection.Audience = Record{}
	if err := json.Unmarshal(payloadRaw, &projection.Payload); err != nil {
		return projection, err
	}
	if err := json.Unmarshal(audienceRaw, &projection.Audience); err != nil {
		return projection, err
	}
	if err := json.Unmarshal(permissionsRaw, &projection.PermissionsAny); err != nil {
		return projection, err
	}
	if err := json.Unmarshal(requiredPayloadKeysRaw, &projection.RequiredPayloadKeys); err != nil {
		return projection, err
	}
	return projection, nil
}

func (s *Service) resolveNotificationRecipients(ctx context.Context, tx *sql.Tx, projection notificationProjection) ([]string, error) {
	switch projection.AudiencePolicyKey {
	case "explicit_users":
		if boolValue(projection.Audience["require_pending_approval"]) {
			approvalRequestID := cleanID(projection.Audience["approval_request_id"])
			if !validActivityUUID(approvalRequestID) {
				return nil, fmt.Errorf("notification event %s requires a valid pending approval request", projection.EventKey)
			}
			requests, err := queryTx(ctx, tx, `
				SELECT status,current_stage
				FROM approval_requests
				WHERE id=$1::uuid AND project_id=NULLIF($2,'')::uuid
			`, approvalRequestID, projection.ProjectID)
			if err != nil {
				return nil, err
			}
			if len(requests) == 0 || fmt.Sprint(requests[0]["status"]) != "pending" {
				return []string{}, nil
			}
			if expectedStage, ok := projection.Audience["expected_stage"]; ok && int(number(expectedStage)) != int(number(requests[0]["current_stage"])) {
				return []string{}, nil
			}
		}
		return filterNotificationRecipients(ctx, tx, projection, notificationAudienceUserIDs(projection.Audience, "user_ids"))
	case "approval_deciders":
		return s.resolveApprovalDeciderRecipients(ctx, tx, projection)
	default:
		return nil, fmt.Errorf("notification audience policy %q is not implemented for enabled event %q", projection.AudiencePolicyKey, projection.EventKey)
	}
}

func (s *Service) resolveApprovalDeciderRecipients(ctx context.Context, tx *sql.Tx, projection notificationProjection) ([]string, error) {
	approvalRequestID := cleanID(projection.Audience["approval_request_id"])
	workItemID := cleanID(projection.Audience["work_item_id"])
	if !validActivityUUID(approvalRequestID) && !validActivityUUID(workItemID) {
		return nil, fmt.Errorf("notification event %s requires a valid approval_request_id or work_item_id audience fact", projection.EventKey)
	}

	var canDecide func(string) (bool, error)
	if validActivityUUID(approvalRequestID) {
		requests, err := queryTx(ctx, tx, `
			SELECT * FROM approval_requests
			WHERE id=$1::uuid AND project_id=NULLIF($2,'')::uuid
		`, approvalRequestID, projection.ProjectID)
		if err != nil {
			return nil, err
		}
		if len(requests) == 0 {
			return nil, fmt.Errorf("notification approval request %s is not visible in project scope", approvalRequestID)
		}
		request := requests[0]
		if fmt.Sprint(request["status"]) != "pending" {
			return []string{}, nil
		}
		if expectedStage, ok := projection.Audience["expected_stage"]; ok && int(number(expectedStage)) != int(number(request["current_stage"])) {
			return []string{}, nil
		}
		canDecide = func(candidateID string) (bool, error) {
			return s.repo.CanDecideApprovalRequest(ctx, request, candidateID)
		}
	} else {
		items, err := queryTx(ctx, tx, `
			SELECT * FROM work_items
			WHERE id=$1::uuid AND project_id=NULLIF($2,'')::uuid
		`, workItemID, projection.ProjectID)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("notification work item %s is not visible in project scope", workItemID)
		}
		item := items[0]
		if !statusIn(fmt.Sprint(item["status"]), "waiting_review", "waiting_approval") {
			return []string{}, nil
		}
		canDecide = func(candidateID string) (bool, error) {
			return s.repo.CanDecideWorkItem(ctx, item, candidateID)
		}
	}

	candidates, err := notificationApprovalCandidates(ctx, tx, projection.ProjectID)
	if err != nil {
		return nil, err
	}
	deciders := make([]string, 0, len(candidates))
	for _, candidateID := range candidates {
		allowed, err := canDecide(candidateID)
		if err != nil {
			return nil, err
		}
		if allowed {
			deciders = append(deciders, candidateID)
		}
	}
	return filterNotificationRecipients(ctx, tx, projection, deciders)
}

func notificationApprovalCandidates(ctx context.Context, tx *sql.Tx, projectID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT account.id::text
		FROM users account
		WHERE account.status='active' AND account.deleted_at IS NULL
		  AND NOT EXISTS (
			SELECT 1 FROM user_roles external_assignment
			JOIN roles external_role ON external_role.id=external_assignment.role_id
			WHERE external_assignment.user_id=account.id AND external_role.external_only
		  )
		  AND (
			EXISTS (
				SELECT 1 FROM user_roles privileged_assignment
				JOIN roles privileged_role ON privileged_role.id=privileged_assignment.role_id
				WHERE privileged_assignment.user_id=account.id
				  AND privileged_assignment.project_id IS NULL
				  AND privileged_role.name IN ('admin','director')
			)
			OR EXISTS (
				SELECT 1 FROM user_roles project_assignment
				WHERE project_assignment.user_id=account.id
				  AND project_assignment.project_id=NULLIF($1,'')::uuid
			)
			OR EXISTS (
				SELECT 1 FROM staff person
				JOIN project_members member ON member.staff_id=person.id AND member.active
				WHERE person.user_id=account.id AND person.status <> 'deleted'
				  AND member.project_id=NULLIF($1,'')::uuid
			)
		  )
		ORDER BY account.id::text
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	candidates := make([]string, 0)
	for rows.Next() {
		var candidateID string
		if err := rows.Scan(&candidateID); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidateID)
		if len(candidates) > 500 {
			return nil, errors.New("notification approval audience exceeds 500 recipients")
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return candidates, nil
}

func filterNotificationRecipients(ctx context.Context, tx *sql.Tx, projection notificationProjection, rawCandidates []string) ([]string, error) {
	excludeActor := boolValue(projection.Audience["exclude_actor"])
	seen := map[string]bool{}
	candidates := make([]string, 0)
	for _, candidateID := range rawCandidates {
		if !validActivityUUID(candidateID) || seen[candidateID] || (excludeActor && candidateID == projection.ActorUserID) {
			continue
		}
		seen[candidateID] = true
		candidates = append(candidates, candidateID)
	}
	if len(candidates) == 0 {
		return []string{}, nil
	}
	if len(candidates) > 500 {
		return nil, errors.New("notification explicit audience exceeds 500 recipients")
	}
	args := []any{projection.ProjectID, projection.EventKey, projection.CategoryKey, projection.RequiredDelivery, projection.PermissionsAny}
	values := make([]string, 0, len(candidates))
	for _, candidateID := range candidates {
		args = append(args, candidateID)
		values = append(values, fmt.Sprintf("($%d::uuid)", len(args)))
	}
	rows, err := tx.QueryContext(ctx, fmt.Sprintf(`
		WITH candidates(user_id) AS (VALUES %s)
		SELECT account.id::text
		FROM candidates candidate
		JOIN users account ON account.id=candidate.user_id
		WHERE account.status='active' AND account.deleted_at IS NULL
		  AND NOT EXISTS (
			SELECT 1 FROM user_roles external_assignment
			JOIN roles external_role ON external_role.id=external_assignment.role_id
			WHERE external_assignment.user_id=account.id AND external_role.external_only
		  )
		  AND (
			NULLIF($1,'')::uuid IS NULL
			OR EXISTS (
				SELECT 1 FROM user_roles privileged_assignment
				JOIN roles privileged_role ON privileged_role.id=privileged_assignment.role_id
				WHERE privileged_assignment.user_id=account.id
				  AND privileged_assignment.project_id IS NULL
				  AND privileged_role.name IN ('admin','director')
			)
			OR EXISTS (
				SELECT 1 FROM user_roles project_assignment
				WHERE project_assignment.user_id=account.id AND project_assignment.project_id=NULLIF($1,'')::uuid
			)
			OR EXISTS (
				SELECT 1 FROM staff person
				JOIN project_members member ON member.staff_id=person.id AND member.active
				WHERE person.user_id=account.id AND person.status <> 'deleted'
				  AND member.project_id=NULLIF($1,'')::uuid
			)
		  )
		  AND (
			cardinality($5::text[])=0
			OR EXISTS (
				SELECT 1 FROM user_roles permission_assignment
				JOIN role_permissions granted ON granted.role_id=permission_assignment.role_id
				JOIN permissions permission ON permission.id=granted.permission_id
				WHERE permission_assignment.user_id=account.id
				  AND (permission_assignment.project_id IS NULL OR permission_assignment.project_id=NULLIF($1,'')::uuid)
				  AND permission.key=ANY($5::text[])
			)
		  )
		  AND (
			$4::boolean
			OR COALESCE((
				SELECT preference.in_app_enabled AND preference.digest_mode <> 'off'
				FROM notification_preferences preference
				WHERE preference.user_id=account.id
				  AND preference.category_key IN ('*',$3)
				  AND preference.event_key IN ('*',$2)
				ORDER BY (preference.event_key=$2) DESC,(preference.category_key=$3) DESC
				LIMIT 1
			),true)
		  )
		ORDER BY account.id
	`, strings.Join(values, ",")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	recipients := make([]string, 0, len(candidates))
	for rows.Next() {
		var recipientID string
		if err := rows.Scan(&recipientID); err != nil {
			return nil, err
		}
		recipients = append(recipients, recipientID)
	}
	return recipients, rows.Err()
}

func notificationAudienceUserIDs(audience Record, key string) []string {
	rawIDs, ok := audience[key].([]any)
	if !ok {
		if stringsList, stringsOK := audience[key].([]string); stringsOK {
			result := make([]string, 0, len(stringsList))
			for _, value := range stringsList {
				value = cleanID(value)
				if validActivityUUID(value) {
					result = append(result, value)
				}
			}
			return result
		}
	}
	result := make([]string, 0, len(rawIDs))
	for _, rawID := range rawIDs {
		value := cleanID(rawID)
		if validActivityUUID(value) {
			result = append(result, value)
		}
	}
	return result
}

func mapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func renderNotificationText(template string, payload Record, requiredKeys []string) (string, error) {
	for _, key := range requiredKeys {
		if strings.TrimSpace(notificationPayloadString(payload[key])) == "" {
			return "", fmt.Errorf("required payload key %q is missing", key)
		}
	}
	rendered := notificationTemplateToken.ReplaceAllStringFunc(template, func(token string) string {
		parts := notificationTemplateToken.FindStringSubmatch(token)
		if len(parts) != 2 {
			return token
		}
		return notificationPayloadString(payload[parts[1]])
	})
	if notificationTemplateToken.MatchString(rendered) {
		return "", errors.New("template contains unresolved placeholders")
	}
	rendered = strings.TrimSpace(rendered)
	if rendered == "" {
		return "", errors.New("template rendered empty text")
	}
	return rendered, nil
}

func renderNotificationRoute(template string, payload Record) (string, error) {
	missing := ""
	rendered := notificationTemplateToken.ReplaceAllStringFunc(template, func(token string) string {
		parts := notificationTemplateToken.FindStringSubmatch(token)
		if len(parts) != 2 {
			return token
		}
		value := strings.TrimSpace(notificationPayloadString(payload[parts[1]]))
		if value == "" {
			missing = parts[1]
			return ""
		}
		return url.PathEscape(value)
	})
	if missing != "" || notificationTemplateToken.MatchString(rendered) {
		return "", fmt.Errorf("action route payload key %q is missing", missing)
	}
	if !strings.HasPrefix(rendered, "/") || strings.HasPrefix(rendered, "//") || strings.Contains(rendered, "\\") || strings.ContainsAny(rendered, "\r\n\x00") {
		return "", errors.New("action route is not a safe internal path")
	}
	parsed, err := url.Parse(rendered)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return "", errors.New("action route is not a safe internal URL")
	}
	return rendered, nil
}

func notificationPayloadString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case float64:
		return fmt.Sprintf("%v", typed)
	case bool:
		return fmt.Sprintf("%t", typed)
	default:
		return fmt.Sprint(typed)
	}
}

func (s *Service) recordNotificationDispatchFailure(ctx context.Context, outboxID int64, eventID string, dispatchErr error) error {
	errorText := sanitizeNotificationError(dispatchErr)
	_, err := s.repo.db.ExecContext(ctx, `
		WITH failed AS (
			UPDATE notification_outbox
			SET attempt_count=attempt_count+1,
				status=CASE WHEN attempt_count+1 >= $3 THEN 'dead' ELSE 'retry' END,
				next_attempt_at=now()+LEAST(INTERVAL '5 minutes',INTERVAL '1 second'*power(2,LEAST(attempt_count,8))),
				dead_at=CASE WHEN attempt_count+1 >= $3 THEN now() ELSE NULL END,
				locked_at=NULL,last_error=$2
			WHERE id=$1 AND event_id=$4::uuid
			RETURNING status
		)
		UPDATE notification_events
		SET last_error=$2
		WHERE id=$4::uuid AND EXISTS (SELECT 1 FROM failed)
	`, outboxID, errorText, notificationDispatchAttempts, eventID)
	return err
}

func (s *Service) cleanupNotificationOutbox(ctx context.Context) error {
	_, err := s.repo.db.ExecContext(ctx, `
		DELETE FROM notification_outbox
		WHERE id IN (
			SELECT id FROM notification_outbox
			WHERE (completed_at < now()-INTERVAL '30 days' OR dead_at < now()-INTERVAL '180 days')
			ORDER BY id LIMIT 1000
		)
	`)
	if err != nil {
		return err
	}
	_, err = s.repo.db.ExecContext(ctx, `
		DELETE FROM user_notifications
		WHERE id IN (
			SELECT id FROM user_notifications
			WHERE expires_at IS NOT NULL AND expires_at < now()
			ORDER BY expires_at,id LIMIT 1000
		)
	`)
	return err
}

func sanitizeNotificationError(err error) string {
	if err == nil {
		return ""
	}
	value := strings.TrimSpace(err.Error())
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	if len(value) > 500 {
		value = value[:500]
	}
	return value
}

func sortedNotificationRecipients(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}
