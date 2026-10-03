package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

func (r *Repository) GetDocumentAttachment(ctx context.Context, documentID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT *
		FROM documents
		WHERE id=$1 AND status <> 'deleted'
	`, documentID)
}

func (r *Repository) RequestWorkItemApproval(ctx context.Context, item Record, req WorkItemActionRequest, actorID, actorName string) (Record, error) {
	workItemID := fmt.Sprint(item["id"])
	projectID := fmt.Sprint(item["project_id"])
	fromStatus := fmt.Sprint(item["status"])
	comment := workItemActionComment(req)
	if comment == "" {
		comment = "Yêu cầu duyệt đầu việc"
	}
	requiredRoles := normalizedRoleList(req.RequiredRoles)
	if len(requiredRoles) == 0 {
		requiredRoles = normalizedRoleList(stringSlice(item["role_ids"]))
	}
	if len(requiredRoles) == 0 {
		requiredRoles = []string{"PM"}
	}
	rawRequiredRoles, _ := json.Marshal(requiredRoles)
	rawMetadata, _ := json.Marshal(Record{"action": "approval_requested", "required_roles": requiredRoles})

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	approvalID := cleanID(item["approval_request_id"])
	var approval Record
	if approvalID == "" {
		rows, err := queryTx(ctx, tx, `
			INSERT INTO approval_requests (target_type, target_id, project_id, requested_by, status, reason, required_roles)
			VALUES ('work_item', $1, $2, NULLIF($3,'')::uuid, 'pending', $4, $5::jsonb)
			RETURNING *
		`, workItemID, projectID, actorID, comment, string(rawRequiredRoles))
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		approval = rows[0]
		approvalID = fmt.Sprint(approval["id"])
	} else {
		rows, err := queryTx(ctx, tx, `
			UPDATE approval_requests
			SET status='pending', reason=$2, required_roles=$3::jsonb, resolved_at=NULL
			WHERE id=$1 AND project_id=$4
			RETURNING *
		`, approvalID, comment, string(rawRequiredRoles), projectID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		approval = rows[0]
	}

	updated, err := queryTx(ctx, tx, `
		UPDATE work_items
		SET status='waiting_approval',
			approval_request_id=$2,
			updated_by=NULLIF($3,'')::uuid,
			updated_at=now()
		WHERE id=$1 AND status=$4
		RETURNING id
	`, workItemID, approvalID, actorID, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái đầu việc đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	if err := applyRunStepStatusTx(ctx, tx, item, "waiting_approval", actorID); err != nil {
		return nil, err
	}
	var activityID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, from_status, to_status, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'approval_requested', $4, $5, 'waiting_approval', $6::jsonb)
		RETURNING id::text
	`, workItemID, projectID, actorID, comment, fromStatus, string(rawMetadata)).Scan(&activityID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "work_item_approval_requested", "work_item", workItemID, workItemAuditDescription("approval_requested", item, comment), json.RawMessage(rawMetadata)); err != nil {
		return nil, err
	}
	if _, err := emitNotificationEventTx(ctx, tx, NotificationDomainEvent{
		Key:            "work_item.approval_requested",
		AggregateType:  "work_item",
		AggregateID:    workItemID,
		SourceEventID:  activityID,
		ProjectID:      projectID,
		ActorUserID:    actorID,
		CorrelationKey: "work_item:" + workItemID + ":approval",
		IdempotencyKey: "work_item_activity:" + activityID + ":notification",
		Payload: Record{
			"actor_name":          actorName,
			"work_item_title":     fmt.Sprint(item["title"]),
			"work_item_id":        workItemID,
			"project_id":          projectID,
			"approval_request_id": approvalID,
		},
		Audience: Record{"approval_request_id": approvalID, "exclude_actor": true},
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	out, err := r.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	out["approval_request"] = approval
	return out, nil
}

func (r *Repository) ApplyWorkItemTransition(ctx context.Context, item Record, toStatus, activityType string, req WorkItemActionRequest, actorID, actorName string) (Record, error) {
	workItemID := fmt.Sprint(item["id"])
	projectID := fmt.Sprint(item["project_id"])
	fromStatus := fmt.Sprint(item["status"])
	comment := workItemActionComment(req)
	payload := req.CompletionPayload
	if payload == nil {
		payload = Record{}
	}
	rawPayload, _ := json.Marshal(payload)
	metadata := Record{"action": activityType}
	expectedCurrentNodeID := ""
	if len(req.StartOverrideMetadata) > 0 {
		metadata["start_override"] = req.StartOverrideMetadata
		expectedCurrentNodeID = cleanID(req.StartOverrideMetadata["current_node_id"])
	}
	rawMetadata, _ := json.Marshal(metadata)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	updated, err := queryTx(ctx, tx, `
			UPDATE work_items
			SET status=$2,
				started_at=CASE WHEN $3 THEN COALESCE(started_at, now()) ELSE started_at END,
				completed_at=CASE WHEN $4 THEN now() ELSE completed_at END,
				blocked_at=CASE WHEN $5 THEN now() WHEN $6 THEN NULL ELSE blocked_at END,
			blocked_reason=CASE WHEN $5 THEN $7 WHEN $6 THEN NULL ELSE blocked_reason END,
				completion_payload_json=CASE WHEN $4 THEN $8::jsonb ELSE completion_payload_json END,
				updated_by=NULLIF($9,'')::uuid,
				updated_at=now()
			WHERE id=$1 AND status=$10
			  AND (
				$11=''
				OR EXISTS (
					SELECT 1 FROM pipeline_runs pr
					WHERE pr.id=work_items.pipeline_run_id
					  AND COALESCE(pr.current_node_id::text, '')=$11
					  AND pr.status NOT IN ('completed','cancelled')
				)
			  )
			RETURNING id
		`, workItemID, toStatus, activityType == "started", activityType == "completed", activityType == "blocked", activityType == "blocker_resolved", strings.TrimSpace(req.Reason), string(rawPayload), actorID, fromStatus, expectedCurrentNodeID)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		if expectedCurrentNodeID != "" {
			return nil, conflictError("Điều kiện bắt đầu đã thay đổi, vui lòng tải lại và xác nhận lại", Record{
				"expected_status":          fromStatus,
				"expected_current_node_id": expectedCurrentNodeID,
			})
		}
		return nil, validationError("Trạng thái đầu việc đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}

	if err := applyRunStepStatusTx(ctx, tx, item, toStatus, actorID); err != nil {
		return nil, err
	}

	var activityID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, from_status, to_status, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, $4, $5, $6, $7, $8::jsonb)
		RETURNING id::text
	`, workItemID, projectID, actorID, activityType, comment, fromStatus, toStatus, string(rawMetadata)).Scan(&activityID); err != nil {
		return nil, err
	}
	auditDescription := workItemAuditDescription(activityType, item, comment)
	if len(req.StartOverrideMetadata) > 0 {
		auditDescription = fmt.Sprintf("Bắt đầu sớm đầu việc %s: %s", item["title"], comment)
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "work_item_"+activityType, "work_item", workItemID, auditDescription, json.RawMessage(rawMetadata)); err != nil {
		return nil, err
	}
	if activityType == "review_requested" {
		if _, err := emitNotificationEventTx(ctx, tx, NotificationDomainEvent{
			Key:            "work_item.review_requested",
			AggregateType:  "work_item",
			AggregateID:    workItemID,
			SourceEventID:  activityID,
			ProjectID:      projectID,
			ActorUserID:    actorID,
			CorrelationKey: "work_item:" + workItemID + ":approval",
			IdempotencyKey: "work_item_activity:" + activityID + ":notification",
			Payload: Record{
				"actor_name":      actorName,
				"work_item_title": fmt.Sprint(item["title"]),
				"work_item_id":    workItemID,
				"project_id":      projectID,
			},
			Audience: Record{"work_item_id": workItemID, "exclude_actor": true},
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetWorkItemDetail(ctx, workItemID)
}

func (r *Repository) ApplyWorkItemDecision(ctx context.Context, item Record, action, toStatus, activityType string, req WorkItemActionRequest, actorID, actorName string) (Record, error) {
	workItemID := fmt.Sprint(item["id"])
	projectID := fmt.Sprint(item["project_id"])
	fromStatus := fmt.Sprint(item["status"])
	comment := workItemActionComment(req)
	payload := req.CompletionPayload
	if payload == nil {
		payload = Record{}
	}
	rawPayload, _ := json.Marshal(payload)
	rawMetadata, _ := json.Marshal(Record{"action": activityType})

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	approvalID := cleanID(item["approval_request_id"])
	requestedBy := ""
	if approvalID != "" {
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(requested_by::text,'')
			FROM approval_requests
			WHERE id=$1::uuid AND project_id=$2::uuid
			FOR UPDATE
		`, approvalID, projectID).Scan(&requestedBy); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO approval_actions (approval_request_id, user_id, action, comment)
			VALUES ($1, NULLIF($2,'')::uuid, $3, $4)
		`, approvalID, actorID, action, comment); err != nil {
			return nil, err
		}
		approvalStatus := "pending"
		switch action {
		case "approve":
			approvalStatus = "approved"
		case "reject":
			approvalStatus = "rejected"
		case "request-change":
			approvalStatus = "changes_requested"
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE approval_requests
			SET status=$2, resolved_at=now()
			WHERE id=$1 AND project_id=$3
		`, approvalID, approvalStatus, projectID); err != nil {
			return nil, err
		}
	}
	if requestedBy == "" {
		err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(actor_id::text,'')
			FROM work_item_activities
			WHERE work_item_id=$1::uuid AND activity_type IN ('approval_requested','review_requested')
			ORDER BY created_at DESC,id DESC
			LIMIT 1
		`, workItemID).Scan(&requestedBy)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	updated, err := queryTx(ctx, tx, `
		UPDATE work_items
		SET status=$2,
			completed_at=CASE WHEN $3 THEN now() ELSE completed_at END,
			completion_payload_json=CASE WHEN $3 THEN $4::jsonb ELSE completion_payload_json END,
			updated_by=NULLIF($5,'')::uuid,
			updated_at=now()
		WHERE id=$1 AND status=$6
		RETURNING id
	`, workItemID, toStatus, toStatus == "completed", string(rawPayload), actorID, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái đầu việc đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	if err := applyRunStepStatusTx(ctx, tx, item, toStatus, actorID); err != nil {
		return nil, err
	}
	var activityID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, from_status, to_status, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, $4, $5, $6, $7, $8::jsonb)
		RETURNING id::text
	`, workItemID, projectID, actorID, activityType, comment, fromStatus, toStatus, string(rawMetadata)).Scan(&activityID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "work_item_"+activityType, "work_item", workItemID, workItemAuditDescription(activityType, item, comment), json.RawMessage(rawMetadata)); err != nil {
		return nil, err
	}
	eventKey := ""
	switch action {
	case "approve":
		eventKey = "work_item.approved"
	case "reject":
		eventKey = "work_item.rejected"
	case "request-change":
		eventKey = "work_item.changes_requested"
	}
	if eventKey != "" && requestedBy != "" {
		if _, err := emitNotificationEventTx(ctx, tx, NotificationDomainEvent{
			Key:            eventKey,
			AggregateType:  "work_item",
			AggregateID:    workItemID,
			SourceEventID:  activityID,
			ProjectID:      projectID,
			ActorUserID:    actorID,
			CorrelationKey: "work_item:" + workItemID + ":approval",
			IdempotencyKey: "work_item_activity:" + activityID + ":notification",
			Payload: Record{
				"actor_name":      actorName,
				"work_item_title": fmt.Sprint(item["title"]),
				"work_item_id":    workItemID,
				"project_id":      projectID,
				"comment":         comment,
			},
			Audience: Record{
				"user_ids":               []string{requestedBy},
				"exclude_actor":          true,
				"resolve_thread_for_all": true,
			},
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetWorkItemDetail(ctx, workItemID)
}

func (r *Repository) AddWorkItemEvidence(ctx context.Context, item Record, req WorkItemEvidenceRequest, actorID, actorName string) (result Record, err error) {
	tracker := storedMutationTracker{}
	defer tracker.wrap(&err)
	workItemID := fmt.Sprint(item["id"])
	projectID := fmt.Sprint(item["project_id"])
	payload := req.Payload
	if payload == nil {
		payload = Record{}
	}
	rawPayload, _ := json.Marshal(payload)
	rawMetadata, _ := json.Marshal(Record{"evidence_type": req.EvidenceType})
	comment := req.Caption
	if comment == "" {
		comment = req.Description
	}
	if comment == "" {
		comment = req.Name
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO evidence_records
			(work_item_id, project_id, target_type, target_id, evidence_type, document_type, name, document_id, title, description, file_url, mime_type, file_size, storage_provider, storage_key, checksum_sha256, caption, revision_no, visibility, payload_json, status, uploaded_by, submitted_by)
		VALUES
			($1, $2, 'work_item', $1, $3, NULLIF($4,''), $5, NULLIF($6,'')::uuid, NULLIF($7,''), NULLIF($8,''), NULLIF($9,''), NULLIF($10,''), NULLIF($11,0), $12, NULLIF($13,''), NULLIF($14,''), NULLIF($15,''), NULLIF($16,''), $17, $18::jsonb, 'submitted', NULLIF($19,'')::uuid, NULLIF($19,'')::uuid)
		RETURNING *
	`, workItemID, projectID, req.EvidenceType, req.DocumentType, req.Name, req.DocumentID, req.Title, req.Description, req.FileURL, req.MIMEType, req.FileSize, req.StorageProvider, req.StorageKey, req.ChecksumSHA256, req.Caption, req.RevisionNo, req.Visibility, string(rawPayload), actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	evidence := rows[0]
	if err := insertDiscussionWorkUpdateTx(ctx, tx, projectID, workItemID, actorID, "evidence", cleanID(evidence["id"]), "uploaded", "Đã thêm minh chứng · "+req.Name, Record{
		"title": req.Name, "status": "submitted", "evidence_id": evidence["id"],
		"caption": comment, "revision_label": req.RevisionNo,
	}); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE work_items SET updated_by=NULLIF($2,'')::uuid, updated_at=now() WHERE id=$1`, workItemID, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'evidence_added', $4, $5::jsonb)
	`, workItemID, projectID, actorID, comment, string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "work_item_evidence_added", "work_item", workItemID, fmt.Sprintf("Thêm minh chứng %s cho đầu việc %s", req.Name, item["title"]), json.RawMessage(rawMetadata)); err != nil {
		return nil, err
	}
	return finalizeStoredMutation(&tracker, func() (Record, error) {
		return getEvidenceDetail(ctx, tx, fmt.Sprint(evidence["id"]))
	}, tx.Commit)
}

func (r *Repository) AddWorkItemActivity(ctx context.Context, item Record, req WorkItemActivityRequest, actorID, actorName string) (Record, error) {
	workItemID := fmt.Sprint(item["id"])
	projectID := fmt.Sprint(item["project_id"])
	rawMetadata, _ := json.Marshal(req.Metadata)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, $4, $5, $6, $7::jsonb)
		RETURNING *
	`, workItemID, projectID, actorID, req.ActivityType, req.Comment, req.Visibility, string(rawMetadata))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := r.insertActivityAttentionTx(ctx, tx, rows[0], req, actorID, actorName); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE work_items SET updated_by=NULLIF($2,'')::uuid, updated_at=now() WHERE id=$1`, workItemID, actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, workItemActivityEvent(req.ActivityType), "work_item", workItemID, workItemActivityDescription(req.ActivityType, item, req.Comment), json.RawMessage(rawMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) AddProjectActivity(ctx context.Context, projectID string, req WorkItemActivityRequest, actorID, actorName string) (Record, error) {
	rawMetadata, _ := json.Marshal(req.Metadata)
	auditMetadata := Record{}
	for key, value := range req.Metadata {
		auditMetadata[key] = value
	}
	if req.ParentActivityID != "" {
		auditMetadata["parent_activity_id"] = req.ParentActivityID
	}
	rawAuditMetadata, _ := json.Marshal(auditMetadata)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json, parent_activity_id)
		VALUES (NULLIF($2,'')::uuid, $1, $3, $4, NULLIF($5,'')::uuid, $6, $7, $8, $9::jsonb, NULLIF($10,'')::uuid)
		RETURNING *
	`, projectID, req.WorkItemID, req.ScopeType, req.ScopeID, actorID, req.ActivityType, req.Comment, req.Visibility, string(rawMetadata), req.ParentActivityID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := r.insertActivityAttentionTx(ctx, tx, rows[0], req, actorID, actorName); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "project_activity_"+req.ActivityType, req.ScopeType, req.ScopeID, projectActivityDescription(req.ActivityType, req.Comment), json.RawMessage(rawAuditMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) SetActivityReaction(ctx context.Context, activity Record, reaction string, active bool, userID string) (Record, error) {
	activityID := cleanID(activity["id"])
	projectID := cleanID(activity["project_id"])
	if err := r.Exec(ctx, `
		INSERT INTO activity_reactions (activity_id, project_id, user_id, reaction, active)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (activity_id, user_id, reaction)
		DO UPDATE SET active=EXCLUDED.active, updated_at=now()
	`, activityID, projectID, userID, reaction, active); err != nil {
		return nil, err
	}
	rows, err := r.Query(ctx, `
		SELECT reaction, COUNT(*) AS count,
			COALESCE(bool_or(user_id=$2), false) AS reacted_by_me
		FROM activity_reactions
		WHERE activity_id=$1 AND active
		GROUP BY reaction
		ORDER BY reaction
	`, activityID, userID)
	if err != nil {
		return nil, err
	}
	return Record{"activity_id": activityID, "project_id": projectID, "reactions": rows}, nil
}

func (r *Repository) UpdateActivityMessage(ctx context.Context, activity Record, comment, actorID, actorName string) (Record, error) {
	activityID := cleanID(activity["id"])
	projectID := cleanID(activity["project_id"])
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	currentRows, err := queryTx(ctx, tx, `
		SELECT *
		FROM work_item_activities
		WHERE id=$1 AND actor_id=$2 AND activity_type='comment'
		  AND withdrawn_at IS NULL
		  AND created_at >= now() - INTERVAL '15 minutes'
		FOR UPDATE
	`, activityID, actorID)
	if err != nil {
		return nil, err
	}
	if len(currentRows) == 0 {
		return nil, validationError("Tin nhắn không còn đủ điều kiện chỉnh sửa")
	}
	current := currentRows[0]
	revisionNo := int(number(current["message_revision_no"])) + 1
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO activity_message_revisions
			(activity_id, project_id, revision_no, action, previous_comment, new_comment, actor_id, actor_name)
		VALUES ($1, $2, $3, 'edited', $4, $5, $6, $7)
	`, activityID, projectID, revisionNo, current["comment"], comment, actorID, actorName); err != nil {
		return nil, err
	}
	updatedRows, err := queryTx(ctx, tx, `
		UPDATE work_item_activities
		SET comment=$2, edited_at=now(), edited_by=$3, message_revision_no=$4
		WHERE id=$1
		RETURNING *
	`, activityID, comment, actorID, revisionNo)
	if err != nil {
		return nil, err
	}
	if len(updatedRows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "project_activity_comment_edited", "activity", activityID, "Đã chỉnh sửa tin nhắn trao đổi", Record{"activity_id": activityID, "revision_no": revisionNo, "action": "edited"}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updatedRows[0], nil
}

func (r *Repository) WithdrawActivityMessage(ctx context.Context, activity Record, actorID, actorName string) (Record, error) {
	activityID := cleanID(activity["id"])
	projectID := cleanID(activity["project_id"])
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	currentRows, err := queryTx(ctx, tx, `
		SELECT *
		FROM work_item_activities
		WHERE id=$1 AND actor_id=$2 AND activity_type='comment'
		  AND withdrawn_at IS NULL
		  AND created_at >= now() - INTERVAL '15 minutes'
		FOR UPDATE
	`, activityID, actorID)
	if err != nil {
		return nil, err
	}
	if len(currentRows) == 0 {
		return nil, validationError("Tin nhắn không còn đủ điều kiện thu hồi")
	}
	current := currentRows[0]
	revisionNo := int(number(current["message_revision_no"])) + 1
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO activity_message_revisions
			(activity_id, project_id, revision_no, action, previous_comment, new_comment, actor_id, actor_name)
		VALUES ($1, $2, $3, 'withdrawn', $4, NULL, $5, $6)
	`, activityID, projectID, revisionNo, current["comment"], actorID, actorName); err != nil {
		return nil, err
	}
	updatedRows, err := queryTx(ctx, tx, `
		UPDATE work_item_activities
		SET comment=NULL, withdrawn_at=now(), withdrawn_by=$2, message_revision_no=$3
		WHERE id=$1
		RETURNING *
	`, activityID, actorID, revisionNo)
	if err != nil {
		return nil, err
	}
	if len(updatedRows) == 0 {
		return nil, sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `UPDATE activity_reactions SET active=false, updated_at=now() WHERE activity_id=$1 AND active`, activityID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE activity_read_states SET read_at=COALESCE(read_at, now()) WHERE activity_id=$1`, activityID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "project_activity_comment_withdrawn", "activity", activityID, "Đã thu hồi tin nhắn trao đổi", Record{"activity_id": activityID, "revision_no": revisionNo, "action": "withdrawn"}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updatedRows[0], nil
}

func (r *Repository) insertActivityAttentionTx(ctx context.Context, tx *sql.Tx, activity Record, req WorkItemActivityRequest, actorID, actorName string) error {
	activityID := cleanID(activity["id"])
	projectID := cleanID(activity["project_id"])
	if activityID == "" || projectID == "" || len(req.Mentions) == 0 {
		return nil
	}
	for _, mention := range req.Mentions {
		switch mention.Type {
		case "all":
			// Resolve the audience from live project membership in the same transaction as the activity.
			// Persist ordinary staff recipients; no new database mention_type or historical rewrite.
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO activity_mentions (activity_id, project_id, mention_type, mention_key, recipient_user_id, recipient_staff_id)
				SELECT DISTINCT $1::uuid, $2::uuid, 'staff', s.id::text, u.id, s.id
				FROM project_members pm
				JOIN staff s ON s.id=pm.staff_id AND s.status='active'
				JOIN users u ON u.id=s.user_id AND u.status='active' AND u.deleted_at IS NULL
				WHERE pm.project_id=$2::uuid AND pm.active
				  AND NOT EXISTS (
					SELECT 1 FROM user_roles ur JOIN roles role ON role.id=ur.role_id
					WHERE ur.user_id=u.id AND role.external_only
				  )
				ON CONFLICT DO NOTHING
			`, activityID, projectID); err != nil {
				return err
			}
		case "user":
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO activity_mentions (activity_id, project_id, mention_type, mention_key, recipient_user_id, recipient_staff_id)
				SELECT $1::uuid, $2::uuid, 'user', $3::text, u.id, s.id
				FROM users u
				LEFT JOIN staff s ON s.user_id=u.id AND s.status='active'
				WHERE u.id=$3::uuid
				  AND u.status='active'
				  AND EXISTS (
					SELECT 1
					FROM project_members pm
					WHERE pm.project_id=$2::uuid
					  AND pm.active
					  AND pm.staff_id=s.id
				  )
				ON CONFLICT DO NOTHING
			`, activityID, projectID, mention.ID); err != nil {
				return err
			}
		case "staff":
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO activity_mentions (activity_id, project_id, mention_type, mention_key, recipient_user_id, recipient_staff_id)
				SELECT $1::uuid, $2::uuid, 'staff', $3::text, s.user_id, s.id
				FROM staff s
				JOIN project_members pm ON pm.staff_id=s.id AND pm.project_id=$2::uuid AND pm.active
				WHERE s.id=$3::uuid
				  AND s.status='active'
				  AND s.user_id IS NOT NULL
				ON CONFLICT DO NOTHING
			`, activityID, projectID, mention.ID); err != nil {
				return err
			}
		case "role":
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO activity_mentions (activity_id, project_id, mention_type, mention_key, recipient_user_id, recipient_staff_id)
				SELECT DISTINCT $1::uuid, $2::uuid, 'role', $3::text, s.user_id, s.id
				FROM project_members pm
				JOIN staff s ON s.id=pm.staff_id
				WHERE pm.project_id=$2::uuid
				  AND pm.active
				  AND s.status='active'
				  AND s.user_id IS NOT NULL
				  AND (
					pm.role_ids ? $3::text
					OR (NULLIF($4::text, '') IS NOT NULL AND pm.role_ids ? $4::text)
					OR EXISTS (
						SELECT 1 FROM jsonb_array_elements_text(pm.role_ids) member_role(label)
						WHERE consflow_role_label_key(member_role.label)='pm'
						  AND (consflow_role_label_key($3::text)='pm' OR consflow_role_label_key($4::text)='pm')
					)
				  )
				ON CONFLICT DO NOTHING
			`, activityID, projectID, mention.ID, mention.Label); err != nil {
				return err
			}
		}
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO activity_read_states (activity_id, project_id, user_id, read_at)
		SELECT DISTINCT activity_id, project_id, recipient_user_id,
			CASE WHEN recipient_user_id=NULLIF($2,'')::uuid THEN now() ELSE NULL END
		FROM activity_mentions
		WHERE activity_id=$1
		ON CONFLICT DO NOTHING
	`, activityID, actorID)
	if err != nil {
		return err
	}
	recipientRows, err := queryTx(ctx, tx, `
		SELECT DISTINCT recipient_user_id::text AS user_id
		FROM activity_mentions
		WHERE activity_id=$1::uuid AND recipient_user_id IS NOT NULL
		ORDER BY recipient_user_id::text
	`, activityID)
	if err != nil {
		return err
	}
	recipientIDs := make([]string, 0, len(recipientRows))
	for _, row := range recipientRows {
		if recipientID := cleanID(row["user_id"]); recipientID != "" && recipientID != actorID {
			recipientIDs = append(recipientIDs, recipientID)
		}
	}
	if len(recipientIDs) == 0 {
		return nil
	}
	if len(recipientIDs) > 500 {
		return validationError("Một trao đổi chỉ được nhắc tối đa 500 người nhận. Vui lòng thu hẹp danh sách thành viên được nhắc.", Record{"field": "mentions", "recipient_count": len(recipientIDs)})
	}
	_, err = emitNotificationEventTx(ctx, tx, NotificationDomainEvent{
		Key:           "activity.mentioned",
		AggregateType: "activity",
		AggregateID:   activityID,
		SourceEventID: activityID,
		ProjectID:     projectID,
		ActorUserID:   actorID,
		Payload: Record{
			"activity_id": activityID,
			"project_id":  projectID,
			"actor_name":  actorName,
		},
		Audience:       Record{"user_ids": recipientIDs, "exclude_actor": true},
		CorrelationKey: "activity:" + activityID,
		IdempotencyKey: "activity_mentioned:" + activityID,
	})
	return err
}

func (r *Repository) ProjectActivityTargetExists(ctx context.Context, projectID, scopeType, scopeID string) (bool, error) {
	table, statusColumn, ok := activityTargetTable(scopeType)
	if !ok {
		return false, nil
	}
	statusSQL := ""
	if statusColumn != "" {
		statusSQL = fmt.Sprintf(" AND COALESCE(%s, '') <> 'cancelled'", statusColumn)
	}
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT COUNT(*) AS total
		FROM %s
		WHERE id=$1 AND project_id=$2%s
	`, table, statusSQL), scopeID, projectID)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && number(rows[0]["total"]) > 0, nil
}

func activityTargetTable(scopeType string) (string, string, bool) {
	switch scopeType {
	case "boq_item":
		return "boq_items", "status", true
	case "procurement_request":
		return "procurement_requests", "status", true
	case "stock_movement":
		return "stock_movements", "", true
	case "acceptance":
		return "acceptance_records", "status", true
	case "rework":
		return "rework_requests", "status", true
	case "handover":
		return "handover_records", "status", true
	case "payable":
		return "payables", "status", true
	case "receivable":
		return "receivables", "status", true
	case "payment":
		return "payments", "status", true
	default:
		return "", "", false
	}
}

func workItemActivityEvent(activityType string) string {
	if activityType == "comment" {
		return "work_item_comment"
	}
	return "work_item_activity_" + activityType
}

func workItemActivityDescription(activityType string, item Record, comment string) string {
	title := fmt.Sprint(item["title"])
	if activityType == "comment" {
		return fmt.Sprintf("Ghi chú đầu việc %s", title)
	}
	return fmt.Sprintf("Tạo hoạt động %s cho đầu việc %s: %s", activityType, title, comment)
}

func projectActivityDescription(activityType string, comment string) string {
	if activityType == "comment" {
		return "Ghi chú công trình"
	}
	return fmt.Sprintf("Tạo hoạt động %s cho công trình: %s", activityType, comment)
}

func evidenceReviewActionLabel(action string) string {
	switch action {
	case "accept":
		return "Chấp nhận"
	case "reject":
		return "Từ chối"
	default:
		return action
	}
}

func (r *Repository) EvidenceAction(ctx context.Context, evidence Record, item Record, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	evidenceID := fmt.Sprint(evidence["id"])
	workItemID := fmt.Sprint(item["id"])
	projectID := fmt.Sprint(item["project_id"])
	status := "accepted"
	activityType := "evidence_accepted"
	if action == "reject" {
		status = "rejected"
		activityType = "evidence_rejected"
	}
	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		comment = strings.TrimSpace(req.Reason)
	}
	rawMetadata, _ := json.Marshal(Record{"action": action, "evidence_id": evidenceID})
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	updated, err := queryTx(ctx, tx, `
		UPDATE evidence_records
		SET status=$2,
			reviewed_by=NULLIF($3,'')::uuid,
			reviewed_at=now(),
			updated_at=now()
		WHERE id=$1 AND status IN ('draft','submitted')
		RETURNING id
	`, evidenceID, status, actorID)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Minh chứng đã được rà soát hoặc không còn hợp lệ", Record{"status": evidence["status"]})
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, $4, $5, $6::jsonb)
	`, workItemID, projectID, actorID, activityType, comment, string(rawMetadata)); err != nil {
		return nil, err
	}
	actionLabel := evidenceReviewActionLabel(action)
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "work_item_"+activityType, "evidence_record", evidenceID, fmt.Sprintf("%s minh chứng %s", actionLabel, evidence["name"]), json.RawMessage(rawMetadata)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetEvidenceDetail(ctx, evidenceID)
}

func applyRunStepStatusTx(ctx context.Context, tx *sql.Tx, item Record, toStatus, actorID string) error {
	runStepID := cleanID(item["run_step_id"])
	if runStepID == "" {
		return nil
	}
	stepStatus := toStatus
	if toStatus == "ready" {
		stepStatus = "pending"
	}
	if toStatus == "completed" {
		rows, err := queryTx(ctx, tx, `
			UPDATE pipeline_run_steps
			SET status='completed', submitted_by=NULLIF($2,'')::uuid, submitted_at=now()
			WHERE id=$1
			RETURNING id
		`, runStepID, actorID)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return fmt.Errorf("linked pipeline run step %s was not found", runStepID)
		}
		return ensureRunStepWorkItemCompletionSyncTx(ctx, tx, runStepID)
	}
	rows, err := queryTx(ctx, tx, `UPDATE pipeline_run_steps SET status=$2 WHERE id=$1 RETURNING id`, runStepID, stepStatus)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("linked pipeline run step %s was not found", runStepID)
	}
	return nil
}

func ensureRunStepWorkItemCompletionSyncTx(ctx context.Context, tx *sql.Tx, runStepID string) error {
	runStepID = cleanID(runStepID)
	if runStepID == "" {
		return nil
	}
	rows, err := queryTx(ctx, tx, `
		SELECT
			rs.status AS run_step_status,
			wi.id AS work_item_id,
			wi.status AS work_item_status
		FROM pipeline_run_steps rs
		LEFT JOIN work_items wi ON wi.run_step_id=rs.id AND wi.status <> 'cancelled'
		WHERE rs.id=$1
		LIMIT 1
	`, runStepID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("linked pipeline run step %s was not found", runStepID)
	}
	pair := rows[0]
	if cleanID(pair["work_item_id"]) == "" {
		return nil
	}
	runStepCompleted := fmt.Sprint(pair["run_step_status"]) == "completed"
	workItemCompleted := fmt.Sprint(pair["work_item_status"]) == "completed"
	if runStepCompleted != workItemCompleted {
		return fmt.Errorf(
			"workflow completion state mismatch for run step %s: run_step=%s work_item=%s",
			runStepID,
			pair["run_step_status"],
			pair["work_item_status"],
		)
	}
	return nil
}

func ensureProjectRunStepWorkItemCompletionSyncTx(ctx context.Context, tx *sql.Tx, projectID string) error {
	rows, err := queryTx(ctx, tx, `
		SELECT
			rs.id AS run_step_id,
			rs.status AS run_step_status,
			wi.id AS work_item_id,
			wi.status AS work_item_status
		FROM pipeline_run_steps rs
		JOIN work_items wi ON wi.run_step_id=rs.id AND wi.project_id=rs.project_id
		WHERE rs.project_id=$1
		  AND ((rs.status='completed') <> (wi.status='completed'))
		ORDER BY rs.id
		LIMIT 1
	`, projectID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	pair := rows[0]
	return fmt.Errorf(
		"workflow completion state mismatch for project %s, run step %s, work item %s: run_step=%s work_item=%s",
		projectID,
		pair["run_step_id"],
		pair["work_item_id"],
		pair["run_step_status"],
		pair["work_item_status"],
	)
}

func workItemAuditDescription(activityType string, item Record, comment string) string {
	title := fmt.Sprint(item["title"])
	switch activityType {
	case "started":
		return fmt.Sprintf("Bắt đầu đầu việc %s", title)
	case "completed":
		return fmt.Sprintf("Hoàn tất đầu việc %s", title)
	case "blocked":
		return fmt.Sprintf("Chặn đầu việc %s: %s", title, comment)
	case "blocker_resolved":
		return fmt.Sprintf("Gỡ điểm chặn đầu việc %s: %s", title, comment)
	case "review_requested":
		return fmt.Sprintf("Yêu cầu rà soát đầu việc %s", title)
	case "approval_requested":
		return fmt.Sprintf("Yêu cầu duyệt đầu việc %s", title)
	case "approved":
		return fmt.Sprintf("Duyệt đầu việc %s", title)
	case "rejected":
		return fmt.Sprintf("Từ chối đầu việc %s: %s", title, comment)
	case "changes_requested":
		return fmt.Sprintf("Yêu cầu chỉnh sửa đầu việc %s: %s", title, comment)
	default:
		return fmt.Sprintf("Cập nhật đầu việc %s", title)
	}
}

func cleanID(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	return text
}

func (r *Repository) ProjectOperations(ctx context.Context, projectID, userID string, filter ProjectOperationsFilter) (Record, error) {
	if filter.View == projectOperationsViewWorkflowSidebar {
		return r.ProjectOperationsWorkflowSidebar(ctx, projectID, userID)
	}
	project, err := r.GetProjectSummary(ctx, projectID)
	if err != nil {
		return nil, err
	}
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	permissions, err := r.UserProjectPermissions(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}
	canViewDesign, err := r.CanAccessProjectPermission(ctx, userID, projectID, []string{"project.view"})
	if err != nil {
		return nil, err
	}
	workItems, err := r.ListProjectOperationsWorkItems(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	summary, blockers, activeWork := summarizeWorkItems(workItems)

	latestActivity, err := r.ProjectOperationsActivity(ctx, projectID, profile)
	if err != nil {
		return nil, err
	}
	projectActivitySummary, err := r.ProjectActivitySummary(ctx, projectID, profile)
	if err != nil {
		return nil, err
	}
	approvals, err := r.ProjectOperationsApprovals(ctx, projectID, profile)
	if err != nil {
		return nil, err
	}
	latestEvidence, err := r.ProjectOperationsEvidence(ctx, projectID, profile, canViewDesign)
	if err != nil {
		return nil, err
	}
	evidenceSummary, err := r.ProjectEvidenceSummary(ctx, projectID, profile, canViewDesign)
	if err != nil {
		return nil, err
	}
	auditRows, err := r.ProjectOperationsAudit(ctx, projectID, profile)
	if err != nil {
		return nil, err
	}
	domainCounts := Record{}
	if profileCanObserveAllProjectWork(profile) {
		domainCounts, err = r.ProjectOperationsDomainCounts(ctx, projectID)
		if err != nil {
			return nil, err
		}
	}
	run, err := r.GetPipelineRun(ctx, projectID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if run != nil && !profileCanObserveAllProjectWork(profile) {
		run = scopePipelineRunToWorkItems(run, workItems)
	}
	visibility := projectOperationsVisibility(profile, permissions)
	healthCounts := projectOperationsHealthCounts(summary, domainCounts)
	return Record{
		"project":             project,
		"project_summary":     projectOperationsProjectSummary(project),
		"visibility":          visibility,
		"health_counts":       healthCounts,
		"workflow_timeline":   workflowTimeline(run),
		"active_work":         activeWork,
		"blockers":            blockers,
		"approvals":           approvals,
		"latest_evidence":     latestEvidence,
		"evidence_summary":    evidenceSummary,
		"activity_summary":    projectActivitySummary,
		"latest_activity":     latestActivity,
		"audit_summary":       auditSummary(auditRows),
		"department_surfaces": departmentSurfaces(workItems),
		"pipeline_run":        run,
		"summary":             summary,
		"work_items":          workItems,
	}, nil
}

func (r *Repository) ProjectOperationsWorkflowSidebar(ctx context.Context, projectID, userID string) (Record, error) {
	project, err := r.GetProjectSummary(ctx, projectID)
	if err != nil {
		return nil, err
	}
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	permissions, err := r.UserProjectPermissions(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}
	workItems, err := r.ListProjectWorkflowSidebarWorkItems(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	summary, _, _ := summarizeWorkItems(workItems)
	run, err := r.GetPipelineRun(ctx, projectID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if run != nil && !profileCanObserveAllProjectWork(profile) {
		run = scopePipelineRunToWorkItems(run, workItems)
	}
	visibility := projectOperationsVisibility(profile, permissions)
	healthCounts := projectOperationsHealthCounts(summary, Record{})
	projectSummary := projectOperationsProjectSummary(project)
	return Record{
		"project":           projectSummary,
		"project_summary":   projectSummary,
		"visibility":        visibility,
		"health_counts":     healthCounts,
		"workflow_timeline": workflowTimeline(run),
		"pipeline_run":      run,
		"summary":           summary,
		"work_items":        workItems,
	}, nil
}

func (r *Repository) ProjectPipelineRun(ctx context.Context, projectID, userID string) (Record, error) {
	run, err := r.GetPipelineRun(ctx, projectID)
	if err != nil {
		return nil, err
	}
	profile, err := r.WorkItemAccessProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if profileCanObserveAllProjectWork(profile) {
		return run, nil
	}
	workItems, err := r.ListProjectWorkItems(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	return scopePipelineRunToWorkItems(run, workItems), nil
}

func (r *Repository) ProjectOperationsActivity(ctx context.Context, projectID string, profile workItemAccessProfile) ([]Record, error) {
	return r.ProjectActivityThread(ctx, projectID, profile, ProjectActivityFilter{Limit: 50})
}

func (r *Repository) ActivityVisibleToUser(ctx context.Context, activityID string, profile workItemAccessProfile) (bool, error) {
	row, err := r.QueryOne(ctx, `
		SELECT
			wia.project_id,
			wia.scope_type,
			wia.scope_id,
			wia.work_item_id,
			wia.visibility
		FROM work_item_activities wia
		JOIN projects p ON p.id=wia.project_id
		WHERE wia.id=$1
		  AND p.status <> 'cancelled'
	`, activityID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if profileIsClient(profile) && !profile.IsManager && fmt.Sprint(row["visibility"]) != "client" {
		return false, nil
	}
	projectID := cleanID(row["project_id"])
	allowed, err := r.CanViewProjectOperations(ctx, projectID, profile.UserID)
	if err != nil {
		return false, err
	}
	if !allowed {
		return false, nil
	}
	workItemID := cleanID(row["work_item_id"])
	if workItemID == "" && fmt.Sprint(row["scope_type"]) == "work_item" {
		workItemID = cleanID(row["scope_id"])
	}
	if workItemID == "" {
		return true, nil
	}
	return r.CanViewWorkItem(ctx, workItemID, profile.UserID)
}

func (r *Repository) ActivityRecord(ctx context.Context, activityID string) (Record, error) {
	return r.QueryOne(ctx, `SELECT * FROM work_item_activities WHERE id=$1`, activityID)
}

func (r *Repository) ActivityMentionedUser(ctx context.Context, activityID, userID string) (bool, error) {
	row, err := r.QueryOne(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM activity_mentions WHERE activity_id=$1 AND recipient_user_id=$2
		) AS allowed
	`, activityID, userID)
	if err != nil {
		return false, err
	}
	return boolValue(row["allowed"]), nil
}

func (r *Repository) ResolveActivity(ctx context.Context, activity Record, comment, actorID, actorName string) (Record, error) {
	activityID := cleanID(activity["id"])
	projectID := cleanID(activity["project_id"])
	activityType := strings.TrimSpace(fmt.Sprint(activity["activity_type"]))
	timestampKey := ""
	actorKey := ""
	actionLabel := ""
	switch activityType {
	case "question":
		timestampKey = "answered_at"
		actorKey = "answered_by"
		actionLabel = "Đã trả lời câu hỏi"
	case "blocker":
		timestampKey = "resolved_at"
		actorKey = "resolved_by"
		actionLabel = "Đã xử lý điểm chặn"
	case "reminder":
		timestampKey = "completed_at"
		actorKey = "completed_by"
		actionLabel = "Đã hoàn tất nhắc việc"
	default:
		return nil, validationError("Loại hoạt động này không có hành động hoàn tất", Record{"activity_type": activityType})
	}
	if comment == "" {
		comment = actionLabel
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		UPDATE work_item_activities
		SET metadata_json=COALESCE(metadata_json, '{}'::jsonb) || jsonb_build_object(
			$2::text, now(),
			$3::text, NULLIF($4,'')::uuid,
			'resolution_comment', $5::text
		)
		WHERE id=$1 AND COALESCE(metadata_json->>$2, metadata_json->>'closed_at', '')=''
		RETURNING *
	`, activityID, timestampKey, actorKey, actorID, comment)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, validationError("Hoạt động đã được xử lý trước đó", Record{"activity_id": activityID})
	}
	resolutionMetadata := Record{
		"source_activity_id":   activityID,
		"source_activity_type": activityType,
		"resolution_action":    timestampKey,
	}
	rawMetadata, _ := json.Marshal(resolutionMetadata)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities
			(work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES
			(NULLIF($1,'')::uuid, $2, $3, $4, NULLIF($5,'')::uuid, 'system_event', $6, $7, $8::jsonb)
	`, cleanID(activity["work_item_id"]), projectID, activity["scope_type"], activity["scope_id"], actorID, comment, activity["visibility"], string(rawMetadata)); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "activity_resolved", "work_item_activity", activityID, comment, resolutionMetadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) MarkActivityRead(ctx context.Context, activityID, userID string) (Record, error) {
	return r.QueryOne(ctx, `
		INSERT INTO activity_read_states (activity_id, project_id, user_id, read_at)
		SELECT id, project_id, $2::uuid, now()
		FROM work_item_activities
		WHERE id=$1
		ON CONFLICT (activity_id, user_id)
		DO UPDATE SET read_at=EXCLUDED.read_at
		RETURNING *
	`, activityID, userID)
}

func (r *Repository) MarkProjectActivityRead(ctx context.Context, projectID, userID string) (Record, error) {
	rows, err := r.Query(ctx, `
		UPDATE activity_read_states
		SET read_at=now()
		WHERE project_id=$1 AND user_id=$2 AND read_at IS NULL
		RETURNING activity_id
	`, projectID, userID)
	if err != nil {
		return nil, err
	}
	return Record{"project_id": projectID, "marked_count": len(rows)}, nil
}

func (r *Repository) ProjectActivitySummary(ctx context.Context, projectID string, profile workItemAccessProfile) (Record, error) {
	args := append([]any{projectID, profile.UserID}, workItemScopeArgs(profile, profileCanObserveAllProjectWork(profile), true)...)
	visibilitySQL := "TRUE"
	if profileIsClient(profile) && !profile.IsManager {
		visibilitySQL = "wia.visibility='client'"
	}
	unread := "(wia.withdrawn_at IS NULL AND ars.user_id IS NOT NULL AND ars.read_at IS NULL)"
	discussionUnread := fmt.Sprintf(`(
		wia.withdrawn_at IS NULL
		AND wia.activity_type IN (%s)
		AND wia.actor_id IS DISTINCT FROM $2::uuid
		AND (
			discussion_cursor.last_read_created_at IS NULL
			OR (wia.created_at, wia.id) > (discussion_cursor.last_read_created_at, COALESCE(discussion_cursor.last_read_activity_id, '00000000-0000-0000-0000-000000000000'::uuid))
		)
	)`, projectDiscussionActivityTypesSQL)
	mentioned := "(wia.withdrawn_at IS NULL AND COALESCE(mention_state.mentioned_me, false))"
	openQuestion := "(wia.activity_type='question' AND COALESCE(wia.metadata_json->>'answered_at', wia.metadata_json->>'resolved_at', wia.metadata_json->>'closed_at', '')='')"
	openBlocker := "(wia.activity_type='blocker' AND COALESCE(wia.metadata_json->>'resolved_at', wia.metadata_json->>'closed_at', '')='')"
	dueReminder := "(wia.activity_type='reminder' AND COALESCE(wia.metadata_json->>'completed_at', wia.metadata_json->>'done_at', wia.metadata_json->>'closed_at', '')='' AND COALESCE(wia.metadata_json->>'due_at', '') ~ '^\\d{4}-\\d{2}-\\d{2}' AND left(wia.metadata_json->>'due_at', 10)::date <= current_date + INTERVAL '7 days')"
	return r.QueryOne(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE wia.activity_type IN ('comment','status','blocker','question','decision','reminder','file_note','work_update')) AS discussion,
			COUNT(*) FILTER (WHERE wia.activity_type NOT IN ('comment','status','blocker','question','decision','reminder','file_note','work_update')) AS system,
			COUNT(*) FILTER (WHERE wia.activity_type='decision') AS decisions,
			COUNT(*) FILTER (WHERE %s) AS unread,
			COUNT(*) FILTER (WHERE %s) AS discussion_unread,
			COUNT(*) FILTER (WHERE %s) AS mentioned,
			COUNT(*) FILTER (WHERE %s) AS questions,
			COUNT(*) FILTER (WHERE %s) AS blockers,
			COUNT(*) FILTER (WHERE %s) AS reminders,
			COUNT(*) FILTER (WHERE %s OR %s OR %s OR %s) AS attention
		FROM work_item_activities wia
		LEFT JOIN work_items wi ON wi.id=wia.work_item_id
		LEFT JOIN activity_read_states ars ON ars.activity_id=wia.id AND ars.user_id=$2
		LEFT JOIN project_discussion_read_cursors discussion_cursor ON discussion_cursor.project_id=wia.project_id AND discussion_cursor.user_id=$2
		LEFT JOIN LATERAL (
			SELECT bool_or(am.recipient_user_id=$2) AS mentioned_me
			FROM activity_mentions am
			WHERE am.activity_id=wia.id
		) mention_state ON true
		WHERE wia.project_id=$1
		  AND (wi.id IS NULL OR wi.status <> 'cancelled')
		  AND (%s)
		  AND (wia.scope_type <> 'work_item' OR (wi.id IS NOT NULL AND (%s)))
	`, unread, discussionUnread, mentioned, openQuestion, openBlocker, dueReminder, unread, openQuestion, openBlocker, dueReminder, visibilitySQL, workItemScopeSQL("wi", 3)), args...)
}

func (r *Repository) ProjectActivityOptions(ctx context.Context, projectID string, profile workItemAccessProfile) (Record, error) {
	members, err := r.Query(ctx, `
		SELECT s.id, s.user_id, s.full_name AS label, s.department, s.position
		FROM project_members pm
		JOIN staff s ON s.id=pm.staff_id
		WHERE pm.project_id=$1 AND pm.active AND s.status='active' AND s.user_id IS NOT NULL
		ORDER BY lower(s.full_name)
	`, projectID)
	if err != nil {
		return nil, err
	}
	roles, err := r.Query(ctx, `
		SELECT DISTINCT ON (consflow_role_label_key(role_name)) role_name AS id,
			CASE WHEN consflow_role_label_key(role_name)='pm' THEN 'Quản lý dự án' ELSE role_name END AS label
		FROM (
			SELECT DISTINCT role_name
			FROM project_members pm
			CROSS JOIN LATERAL jsonb_array_elements_text(COALESCE(pm.role_ids, '[]'::jsonb)) AS roles(role_name)
			WHERE pm.project_id=$1 AND pm.active AND role_name <> ''
		) role_options
		ORDER BY consflow_role_label_key(role_name), role_name
	`, projectID)
	if err != nil {
		return nil, err
	}
	workArgs := append([]any{projectID}, workItemScopeArgs(profile, profileCanObserveAllProjectWork(profile), true)...)
	workTargets, err := r.Query(ctx, fmt.Sprintf(`
		SELECT 'work_item' AS scope_type, wi.id AS scope_id,
			COALESCE(NULLIF(wi.title, ''), wi.id::text) AS label,
			wi.id AS work_item_id, wi.status
		FROM work_items wi
		WHERE wi.project_id=$1 AND wi.status <> 'cancelled' AND (%s)
		ORDER BY wi.created_at DESC
		LIMIT 300
	`, workItemScopeSQL("wi", 2)), workArgs...)
	if err != nil {
		return nil, err
	}
	domainTargets, err := r.Query(ctx, `
		SELECT * FROM (
			SELECT 'boq_item' AS scope_type, id AS scope_id, work_item_name AS label, NULL::uuid AS work_item_id, status FROM boq_items WHERE project_id=$1 AND status NOT IN ('cancelled','deleted')
			UNION ALL
			SELECT 'procurement_request', id, request_code || ' · ' || title, NULL::uuid, status FROM procurement_requests WHERE project_id=$1 AND status NOT IN ('cancelled','deleted')
			UNION ALL
			SELECT 'stock_movement', id, movement_code, NULL::uuid, NULL::text FROM stock_movements WHERE project_id=$1
			UNION ALL
			SELECT 'acceptance', id, title, NULL::uuid, status FROM acceptance_records WHERE project_id=$1 AND status NOT IN ('cancelled','deleted')
			UNION ALL
			SELECT 'rework', id, title, NULL::uuid, status FROM rework_requests WHERE project_id=$1 AND status NOT IN ('cancelled','deleted')
			UNION ALL
			SELECT 'handover', id, 'Bàn giao · ' || status, NULL::uuid, status FROM handover_records WHERE project_id=$1 AND status NOT IN ('cancelled','deleted')
			UNION ALL
			SELECT 'payable', id, payable_code || ' · ' || vendor_name, NULL::uuid, status FROM payables WHERE project_id=$1 AND status NOT IN ('cancelled','deleted')
			UNION ALL
			SELECT 'receivable', id, receivable_code || ' · ' || client_name, NULL::uuid, status FROM receivables WHERE project_id=$1 AND status NOT IN ('cancelled','deleted')
			UNION ALL
			SELECT 'payment', id, payment_code || ' · ' || payment_type, NULL::uuid, status FROM payments WHERE project_id=$1 AND status NOT IN ('cancelled','deleted')
		) options
		ORDER BY scope_type, lower(label)
	`, projectID)
	if err != nil {
		return nil, err
	}
	targets := append(workTargets, domainTargets...)
	evidenceVisibility := "er.visibility IN ('internal','project','client')"
	if profileIsClient(profile) && !profile.IsManager {
		evidenceVisibility = "er.visibility='client'"
	}
	attachmentArgs := append([]any{projectID}, workItemScopeArgs(profile, profileCanObserveAllProjectWork(profile), true)...)
	attachments, err := r.Query(ctx, fmt.Sprintf(`
		SELECT 'evidence' AS type, er.id, COALESCE(NULLIF(er.name,''), NULLIF(er.title,''), er.id::text) AS label,
			er.status, er.visibility, er.work_item_id
		FROM evidence_records er
		JOIN work_items wi ON wi.id=er.work_item_id
		WHERE er.project_id=$1 AND er.status <> 'rejected' AND wi.status <> 'cancelled'
		  AND %s AND (%s)
		ORDER BY er.created_at DESC
		LIMIT 200
	`, evidenceVisibility, workItemScopeSQL("wi", 2)), attachmentArgs...)
	if err != nil {
		return nil, err
	}
	if !profileIsClient(profile) || profile.IsManager {
		documents, err := r.Query(ctx, `
			SELECT 'document' AS type, d.id, d.name AS label, d.status, 'project' AS visibility, NULL::uuid AS work_item_id
			FROM documents d
			WHERE d.project_id=$1 AND d.status NOT IN ('cancelled','deleted')
			ORDER BY d.created_at DESC
			LIMIT 200
		`, projectID)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, documents...)
	}
	return Record{
		"members":     members,
		"roles":       roles,
		"targets":     targets,
		"attachments": attachments,
	}, nil
}

func (r *Repository) ProjectActivityThread(ctx context.Context, projectID string, profile workItemAccessProfile, filter ProjectActivityFilter) ([]Record, error) {
	args := append([]any{projectID, profile.UserID}, workItemScopeArgs(profile, profileCanObserveAllProjectWork(profile), true)...)
	visibilitySQL := "TRUE"
	if profileIsClient(profile) && !profile.IsManager {
		visibilitySQL = "wia.visibility='client'"
	}
	conditions := []string{
		"wia.project_id=$1",
		"(wi.id IS NULL OR wi.status <> 'cancelled')",
		fmt.Sprintf("(%s)", visibilitySQL),
		fmt.Sprintf("(wia.scope_type <> 'work_item' OR (wi.id IS NOT NULL AND (%s)))", workItemScopeSQL("wi", 3)),
	}
	if filter.TransitionsOnly {
		conditions = append(conditions, "NULLIF(wia.from_status,'') IS NOT NULL AND NULLIF(wia.to_status,'') IS NOT NULL AND wia.from_status <> wia.to_status")
	}
	if filter.ScopeType != "" {
		args = append(args, filter.ScopeType)
		conditions = append(conditions, fmt.Sprintf("wia.scope_type=$%d", len(args)))
	}
	if filter.ScopeID != "" {
		args = append(args, filter.ScopeID)
		conditions = append(conditions, fmt.Sprintf("wia.scope_id=$%d::uuid", len(args)))
	}
	if filter.ActivityType != "" {
		args = append(args, filter.ActivityType)
		conditions = append(conditions, fmt.Sprintf("wia.activity_type=$%d", len(args)))
	}
	if filter.ActorID != "" {
		args = append(args, filter.ActorID)
		conditions = append(conditions, fmt.Sprintf("wia.actor_id=$%d::uuid", len(args)))
	}
	if filter.Visibility != "" {
		args = append(args, filter.Visibility)
		conditions = append(conditions, fmt.Sprintf("wia.visibility=$%d", len(args)))
	}
	switch filter.Kind {
	case "discussion":
		conditions = append(conditions, "wia.activity_type IN ("+projectDiscussionActivityTypesSQL+")")
	case "system":
		conditions = append(conditions, "wia.activity_type NOT IN ("+projectDiscussionActivityTypesSQL+")")
	case "decision":
		conditions = append(conditions, "wia.activity_type='decision'")
	}
	unread := "(wia.withdrawn_at IS NULL AND ars.user_id IS NOT NULL AND ars.read_at IS NULL)"
	mentioned := "(wia.withdrawn_at IS NULL AND COALESCE(mention_state.mentioned_me, false))"
	openQuestion := "(wia.activity_type='question' AND COALESCE(wia.metadata_json->>'answered_at', wia.metadata_json->>'resolved_at', wia.metadata_json->>'closed_at', '')='')"
	openBlocker := "(wia.activity_type='blocker' AND COALESCE(wia.metadata_json->>'resolved_at', wia.metadata_json->>'closed_at', '')='')"
	dueReminder := "(wia.activity_type='reminder' AND COALESCE(wia.metadata_json->>'completed_at', wia.metadata_json->>'done_at', wia.metadata_json->>'closed_at', '')='' AND COALESCE(wia.metadata_json->>'due_at', '') ~ '^\\d{4}-\\d{2}-\\d{2}' AND left(wia.metadata_json->>'due_at', 10)::date <= current_date + INTERVAL '7 days')"
	switch filter.Attention {
	case "discussion_unread":
		conditions = append(conditions, projectDiscussionUnreadSQL("$2"))
	case "system_unread":
		conditions = append(conditions, projectSystemUnreadSQL("$2"))
	case "mentions":
		conditions = append(conditions, mentioned)
	case "questions":
		conditions = append(conditions, openQuestion)
	case "blockers":
		conditions = append(conditions, openBlocker)
	case "reminders":
		conditions = append(conditions, dueReminder)
	case "unread":
		conditions = append(conditions, unread)
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		placeholder := len(args)
		conditions = append(conditions, fmt.Sprintf(`(
			COALESCE(wia.comment, '') ILIKE $%d
			OR COALESCE(u.full_name, '') ILIKE $%d
			OR COALESCE(wi.title, '') ILIKE $%d
			OR COALESCE(p.code, '') ILIKE $%d
			OR COALESCE(p.name, '') ILIKE $%d
			OR COALESCE(bi.work_item_name, '') ILIKE $%d
			OR COALESCE(pr.request_code, '') ILIKE $%d
			OR COALESCE(pr.title, '') ILIKE $%d
			OR COALESCE(sm.movement_code, '') ILIKE $%d
			OR COALESCE(ar.title, '') ILIKE $%d
			OR COALESCE(rw.title, '') ILIKE $%d
			OR COALESCE(pay.payable_code, '') ILIKE $%d
			OR COALESCE(rec.receivable_code, '') ILIKE $%d
			OR COALESCE(pm.payment_code, '') ILIKE $%d
		)`, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder))
	}
	if filter.CursorCreatedAt != "" && filter.CursorID != "" {
		args = append(args, filter.CursorCreatedAt, filter.CursorID)
		conditions = append(conditions, fmt.Sprintf("(wia.created_at, wia.id) < ($%d::timestamptz, $%d::uuid)", len(args)-1, len(args)))
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 80
	}
	args = append(args, filter.Limit)
	limitPlaceholder := len(args)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT
			wia.*,
			to_char(wia.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"') AS cursor_created_at,
			wi.title AS work_item_title,
			u.full_name AS actor_name,
			COALESCE(u.deleted_at IS NOT NULL, false) AS actor_account_deleted,
			COALESCE(mention_state.mentions, '[]'::jsonb) AS mentions,
				(wia.withdrawn_at IS NULL AND COALESCE(mention_state.mentioned_me, false)) AS mentioned_me,
			%s,
			%s,
			ars.read_at,
				(wia.withdrawn_at IS NULL AND ars.user_id IS NOT NULL AND ars.read_at IS NULL) AS is_unread,
			%s AS is_discussion_unread,
			%s AS is_system_unread,
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
			) AS target_label
		FROM work_item_activities wia
		JOIN projects p ON p.id=wia.project_id
		LEFT JOIN work_items wi ON wi.id=wia.work_item_id
		LEFT JOIN users u ON u.id=wia.actor_id
		%s
		%s
		LEFT JOIN activity_read_states ars ON ars.activity_id=wia.id AND ars.user_id=$2
		LEFT JOIN project_discussion_read_cursors discussion_cursor ON discussion_cursor.project_id=wia.project_id AND discussion_cursor.user_id=$2
		LEFT JOIN project_system_activity_read_cursors system_cursor ON system_cursor.project_id=wia.project_id AND system_cursor.user_id=$2
		CROSS JOIN project_activity_tracking_start tracking
		LEFT JOIN LATERAL (
			SELECT
				jsonb_agg(jsonb_build_object(
					'type', am.mention_type,
					'key', am.mention_key,
					'user_id', am.recipient_user_id,
					'staff_id', am.recipient_staff_id
				) ORDER BY am.created_at) AS mentions,
				bool_or(am.recipient_user_id=$2) AS mentioned_me
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
		WHERE %s
		ORDER BY wia.created_at DESC, wia.id DESC
		LIMIT $%d
		`, activityAttachmentSelectSQL(), activityConversationSelectSQL("$2"), projectDiscussionUnreadSQL("$2"), projectSystemUnreadSQL("$2"), activityAttachmentJoinSQL(), activityConversationJoinSQL("$2"), strings.Join(conditions, " AND "), limitPlaceholder), args...)
}

func profileIsClient(profile workItemAccessProfile) bool {
	for _, label := range profile.RoleLabels {
		normalized := strings.ToLower(strings.TrimSpace(label))
		if normalized == "client" || normalized == "chủ đầu tư" {
			return true
		}
	}
	return false
}

func (r *Repository) ProjectOperationsApprovals(ctx context.Context, projectID string, profile workItemAccessProfile) ([]Record, error) {
	if profileCanObserveAllProjectWork(profile) {
		return r.Query(ctx, `
			SELECT ar.*, u.full_name AS requested_by_name,
				COALESCE(u.deleted_at IS NOT NULL,false) AS requested_by_account_deleted
			FROM approval_requests ar
			LEFT JOIN users u ON u.id=ar.requested_by
			WHERE ar.project_id=$1
			ORDER BY
				CASE ar.status WHEN 'pending' THEN 1 WHEN 'changes_requested' THEN 2 ELSE 3 END,
				ar.created_at DESC
			LIMIT 50
		`, projectID)
	}
	args := append([]any{projectID, profile.UserID}, workItemScopeArgs(profile, false, true)...)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT ar.*, u.full_name AS requested_by_name,
			COALESCE(u.deleted_at IS NOT NULL,false) AS requested_by_account_deleted
		FROM approval_requests ar
		LEFT JOIN users u ON u.id=ar.requested_by
		LEFT JOIN work_items wi ON wi.approval_request_id=ar.id AND wi.status <> 'cancelled'
		WHERE ar.project_id=$1
		  AND (
			ar.requested_by=NULLIF($2,'')::uuid
			OR (wi.id IS NOT NULL AND (%s))
		  )
		ORDER BY
			CASE ar.status WHEN 'pending' THEN 1 WHEN 'changes_requested' THEN 2 ELSE 3 END,
			ar.created_at DESC
		LIMIT 50
	`, workItemScopeSQL("wi", 3)), args...)
}

func (r *Repository) ProjectOperationsEvidence(ctx context.Context, projectID string, profile workItemAccessProfile, canViewDesign bool) ([]Record, error) {
	args := append([]any{projectID, canViewDesign}, workItemScopeArgs(profile, profileCanObserveAllProjectWork(profile), true)...)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT er.*, wi.title AS work_item_title,u.full_name AS submitted_by_name,
			COALESCE(u.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,d.name AS document_name,
			item.design_release_id,release.revision_label AS design_revision_label,
			release.status AS design_release_status,release.change_reason AS design_change_reason
		FROM evidence_records er
		JOIN work_items wi ON wi.id=er.work_item_id
		LEFT JOIN users u ON u.id=er.submitted_by
		LEFT JOIN documents d ON d.id=er.document_id
		LEFT JOIN design_release_items item ON item.evidence_id=er.id AND item.project_id=er.project_id
		LEFT JOIN design_releases release ON release.id=item.design_release_id AND release.project_id=item.project_id
		WHERE er.project_id=$1
		  AND wi.status <> 'cancelled'
		  AND er.status <> 'rejected'
		  AND er.visibility IN ('internal','project')
		  AND ((item.evidence_id IS NOT NULL AND $2::boolean) OR (item.evidence_id IS NULL AND (%s)))
		ORDER BY er.created_at DESC
		LIMIT 50
	`, workItemScopeSQL("wi", 3)), args...)
}

func projectEvidenceWaitingSQL() string {
	return "((item.evidence_id IS NULL AND er.status IN ('draft','submitted')) OR (item.evidence_id IS NOT NULL AND release.status='released' AND er.status='submitted'))"
}

func projectEvidencePrioritySQL() string {
	return "CASE WHEN " + projectEvidenceWaitingSQL() + " THEN 0 ELSE 1 END"
}

func projectEvidenceWhere(projectID string, profile workItemAccessProfile, canViewDesign bool, filter ProjectEvidenceFilter, includeCursor bool) ([]string, []any) {
	args := append([]any{projectID, canViewDesign}, workItemScopeArgs(profile, profileCanObserveAllProjectWork(profile), true)...)
	conditions := []string{
		"er.project_id=$1",
		"wi.status <> 'cancelled'",
		fmt.Sprintf("((item.evidence_id IS NOT NULL AND $2::boolean) OR (item.evidence_id IS NULL AND (%s)))", workItemScopeSQL("wi", 3)),
	}
	if profileIsClient(profile) && !profile.IsManager {
		conditions = append(conditions, "er.visibility='client'")
	} else {
		conditions = append(conditions, "er.visibility IN ('internal','project','client')")
	}
	if filter.EvidenceType != "" {
		args = append(args, filter.EvidenceType)
		conditions = append(conditions, fmt.Sprintf("er.evidence_type=$%d", len(args)))
	}
	switch filter.Status {
	case "waiting":
		conditions = append(conditions, projectEvidenceWaitingSQL())
	case "accepted", "rejected":
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("er.status=$%d", len(args)))
	}
	if filter.WorkItemID != "" {
		args = append(args, filter.WorkItemID)
		conditions = append(conditions, fmt.Sprintf("er.work_item_id=$%d::uuid", len(args)))
	}
	if filter.SubmittedBy != "" {
		args = append(args, filter.SubmittedBy)
		conditions = append(conditions, fmt.Sprintf("er.submitted_by=$%d::uuid", len(args)))
	}
	if filter.Visibility != "" {
		args = append(args, filter.Visibility)
		conditions = append(conditions, fmt.Sprintf("er.visibility=$%d", len(args)))
	}
	if filter.DateFrom != "" {
		args = append(args, filter.DateFrom)
		conditions = append(conditions, fmt.Sprintf("er.created_at >= $%d::date", len(args)))
	}
	if filter.DateTo != "" {
		args = append(args, filter.DateTo)
		conditions = append(conditions, fmt.Sprintf("er.created_at < $%d::date + INTERVAL '1 day'", len(args)))
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		placeholder := len(args)
		conditions = append(conditions, fmt.Sprintf(`(
			er.name ILIKE $%d
			OR COALESCE(er.title, '') ILIKE $%d
			OR COALESCE(er.description, '') ILIKE $%d
			OR COALESCE(er.caption, '') ILIKE $%d
			OR COALESCE(er.file_url, '') ILIKE $%d
			OR COALESCE(er.checksum_sha256, '') ILIKE $%d
			OR COALESCE(er.payload_json->>'original_filename', '') ILIKE $%d
			OR wi.title ILIKE $%d
			OR COALESCE(d.name, '') ILIKE $%d
			OR COALESCE(u.full_name, '') ILIKE $%d
			OR COALESCE(release.revision_label, '') ILIKE $%d
			OR COALESCE(release.change_reason, '') ILIKE $%d
		)`, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder))
	}
	if includeCursor && filter.CursorCreatedAt != "" && filter.CursorID != "" {
		priorityExpression := projectEvidencePrioritySQL()
		args = append(args, filter.CursorPriority, filter.CursorCreatedAt, filter.CursorID)
		priorityPlaceholder := len(args) - 2
		createdPlaceholder := len(args) - 1
		idPlaceholder := len(args)
		conditions = append(conditions, fmt.Sprintf(`(
			%s > $%d
			OR (%s=$%d AND (er.created_at, er.id) < ($%d::timestamptz, $%d::uuid))
		)`, priorityExpression, priorityPlaceholder, priorityExpression, priorityPlaceholder, createdPlaceholder, idPlaceholder))
	}
	return conditions, args
}

func (r *Repository) ListProjectEvidence(ctx context.Context, projectID string, profile workItemAccessProfile, canViewDesign bool, filter ProjectEvidenceFilter) ([]Record, error) {
	conditions, args := projectEvidenceWhere(projectID, profile, canViewDesign, filter, true)
	args = append(args, filter.Limit)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT er.*,
			wi.title AS work_item_title,
			wi.status AS work_item_status,
			wi.role_ids AS work_item_role_ids,
			wi.approval_request_id AS work_item_approval_request_id,
			rn.name AS run_node_name,
			u.full_name AS submitted_by_name,
			COALESCE(u.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,
			reviewer.full_name AS reviewed_by_name,
			COALESCE(reviewer.deleted_at IS NOT NULL,false) AS reviewed_by_account_deleted,
			d.name AS document_name,
			item.design_release_id,
			CASE WHEN item.evidence_id IS NULL THEN NULL ELSE 'evidence' END AS reference_type,
			COALESCE(item.document_type,er.document_type) AS design_document_type,
			release.revision_label AS design_revision_label,
			release.status AS design_release_status,
			release.change_reason AS design_change_reason,
			%s AS review_priority
		FROM evidence_records er
		JOIN work_items wi ON wi.id=er.work_item_id AND wi.project_id=er.project_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN users u ON u.id=er.submitted_by
		LEFT JOIN users reviewer ON reviewer.id=er.reviewed_by
		LEFT JOIN documents d ON d.id=er.document_id
		LEFT JOIN design_release_items item ON item.evidence_id=er.id AND item.project_id=er.project_id
		LEFT JOIN design_releases release ON release.id=item.design_release_id AND release.project_id=item.project_id
		WHERE %s
		ORDER BY review_priority, er.created_at DESC, er.id DESC
		LIMIT $%d
	`, projectEvidencePrioritySQL(), strings.Join(conditions, " AND "), len(args)), args...)
}

func (r *Repository) ProjectEvidenceCount(ctx context.Context, projectID string, profile workItemAccessProfile, canViewDesign bool, filter ProjectEvidenceFilter) (int, error) {
	conditions, args := projectEvidenceWhere(projectID, profile, canViewDesign, filter, false)
	row, err := r.QueryOne(ctx, fmt.Sprintf(`
		SELECT COUNT(*) AS total
		FROM evidence_records er
		JOIN work_items wi ON wi.id=er.work_item_id AND wi.project_id=er.project_id
		LEFT JOIN users u ON u.id=er.submitted_by
		LEFT JOIN documents d ON d.id=er.document_id
		LEFT JOIN design_release_items item ON item.evidence_id=er.id AND item.project_id=er.project_id
		LEFT JOIN design_releases release ON release.id=item.design_release_id AND release.project_id=item.project_id
		WHERE %s
	`, strings.Join(conditions, " AND ")), args...)
	if err != nil {
		return 0, err
	}
	return int(number(row["total"])), nil
}

func (r *Repository) ProjectEvidenceSummary(ctx context.Context, projectID string, profile workItemAccessProfile, canViewDesign bool) (Record, error) {
	conditions, args := projectEvidenceWhere(projectID, profile, canViewDesign, ProjectEvidenceFilter{}, false)
	return r.QueryOne(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE %s) AS waiting,
			COUNT(*) FILTER (WHERE er.status='accepted') AS accepted,
			COUNT(*) FILTER (WHERE er.status='rejected') AS rejected,
			COUNT(*) FILTER (WHERE er.evidence_type='photo' OR COALESCE(er.mime_type, '') LIKE 'image/%%') AS photos,
			COUNT(*) FILTER (WHERE er.evidence_type='document' OR COALESCE(er.mime_type, '') ILIKE '%%pdf%%') AS documents
		FROM evidence_records er
		JOIN work_items wi ON wi.id=er.work_item_id AND wi.project_id=er.project_id
		LEFT JOIN users u ON u.id=er.submitted_by
		LEFT JOIN documents d ON d.id=er.document_id
		LEFT JOIN design_release_items item ON item.evidence_id=er.id AND item.project_id=er.project_id
		LEFT JOIN design_releases release ON release.id=item.design_release_id AND release.project_id=item.project_id
		WHERE %s
	`, projectEvidenceWaitingSQL(), strings.Join(conditions, " AND ")), args...)
}

func (r *Repository) ProjectEvidenceOptions(ctx context.Context, projectID string, profile workItemAccessProfile, canViewDesign bool) (Record, error) {
	conditions, args := projectEvidenceWhere(projectID, profile, canViewDesign, ProjectEvidenceFilter{}, false)
	where := strings.Join(conditions, " AND ")
	workItems, err := r.Query(ctx, fmt.Sprintf(`
		SELECT wi.id, wi.title AS label
		FROM evidence_records er
		JOIN work_items wi ON wi.id=er.work_item_id AND wi.project_id=er.project_id
		LEFT JOIN users u ON u.id=er.submitted_by
		LEFT JOIN documents d ON d.id=er.document_id
		LEFT JOIN design_release_items item ON item.evidence_id=er.id AND item.project_id=er.project_id
		LEFT JOIN design_releases release ON release.id=item.design_release_id AND release.project_id=item.project_id
		WHERE %s
		GROUP BY wi.id, wi.title
		ORDER BY lower(wi.title), wi.id
	`, where), args...)
	if err != nil {
		return nil, err
	}
	submitters, err := r.Query(ctx, fmt.Sprintf(`
		SELECT u.id, u.full_name AS label
		FROM evidence_records er
		JOIN work_items wi ON wi.id=er.work_item_id AND wi.project_id=er.project_id
		JOIN users u ON u.id=er.submitted_by
		LEFT JOIN documents d ON d.id=er.document_id
		LEFT JOIN design_release_items item ON item.evidence_id=er.id AND item.project_id=er.project_id
		LEFT JOIN design_releases release ON release.id=item.design_release_id AND release.project_id=item.project_id
		WHERE %s
		GROUP BY u.id, u.full_name
		ORDER BY lower(u.full_name), u.id
	`, where), args...)
	if err != nil {
		return nil, err
	}
	return Record{"work_items": workItems, "submitters": submitters}, nil
}

func (r *Repository) ProjectOperationsAudit(ctx context.Context, projectID string, profile workItemAccessProfile) ([]Record, error) {
	if profile.CanReconcileAllProjects && !profile.IsManager {
		return []Record{}, nil
	}
	if profile.IsManager {
		return r.Query(ctx, `
			SELECT al.*
			FROM audit_logs al
			WHERE al.project_id=$1
			ORDER BY al.created_at DESC
			LIMIT 50
		`, projectID)
	}
	args := append([]any{projectID}, workItemScopeArgs(profile, false, true)...)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT al.*
		FROM audit_logs al
		JOIN work_items wi ON wi.id=al.target_id AND al.target_type='work_item'
		WHERE al.project_id=$1
		  AND wi.status <> 'cancelled'
		  AND (%s)
		ORDER BY al.created_at DESC
		LIMIT 50
	`, workItemScopeSQL("wi", 2)), args...)
}

func (r *Repository) ProjectOperationsDomainCounts(ctx context.Context, projectID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			(SELECT COUNT(*) FROM issues WHERE project_id=$1 AND status <> 'closed') AS open_issues,
			(SELECT COUNT(*) FROM acceptance_records WHERE project_id=$1 AND result='fail' AND status <> 'cancelled') AS failed_acceptances,
			(SELECT COUNT(*) FROM procurement_requests WHERE project_id=$1 AND status IN ('draft','submitted')) AS pending_procurement,
			(SELECT COUNT(*) FROM site_material_balances WHERE project_id=$1 AND planned_quantity > issued_quantity) AS material_risks
	`, projectID)
}

func summarizeWorkItems(workItems []Record) (Record, []Record, []Record) {
	summary := Record{
		"total_work_items":      len(workItems),
		"active_work_items":     0,
		"blocked_work_items":    0,
		"completed_work_items":  0,
		"overdue_work_items":    0,
		"pending_approvals":     0,
		"waiting_evidence":      0,
		"visible_scope_applied": true,
	}
	statusCounts := Record{}
	departmentCounts := Record{}
	blockers := []Record{}
	activeWork := []Record{}
	for _, item := range workItems {
		status := fmt.Sprint(item["status"])
		statusCounts[status] = number(statusCounts[status]) + 1
		if status != "completed" && status != "cancelled" {
			summary["active_work_items"] = number(summary["active_work_items"]) + 1
			if len(activeWork) < 20 {
				activeWork = append(activeWork, item)
			}
		}
		if isOverdue(item) {
			summary["overdue_work_items"] = number(summary["overdue_work_items"]) + 1
		}
		if status == "blocked" {
			summary["blocked_work_items"] = number(summary["blocked_work_items"]) + 1
			blockers = append(blockers, item)
		}
		if status == "completed" {
			summary["completed_work_items"] = number(summary["completed_work_items"]) + 1
		}
		if status == "waiting_approval" {
			summary["pending_approvals"] = number(summary["pending_approvals"]) + 1
		}
		if status == "waiting_evidence" {
			summary["waiting_evidence"] = number(summary["waiting_evidence"]) + 1
		}
		department := cleanText(item["department"])
		if department != "" {
			departmentCounts[department] = number(departmentCounts[department]) + 1
		}
	}
	summary["status_counts"] = statusCounts
	summary["department_counts"] = departmentCounts
	return summary, blockers, activeWork
}

func isOverdue(item Record) bool {
	status := fmt.Sprint(item["status"])
	if status == "completed" || status == "cancelled" {
		return false
	}
	dueAt := cleanText(item["due_at"])
	if dueAt == "" {
		return false
	}
	parsed, err := time.Parse(time.RFC3339, dueAt)
	if err != nil {
		return false
	}
	return parsed.Before(time.Now())
}

func projectOperationsVisibility(profile workItemAccessProfile, permissions []string) Record {
	scope := "project_member"
	if profile.IsAdmin {
		scope = "admin"
	} else if profile.IsDirector {
		scope = "director"
	} else if profile.CanReconcileAllProjects {
		scope = "accounting_reconciliation"
	} else if profile.Department != "" {
		scope = "project_member"
	}
	hiddenSections := []string{}
	canObserveAll := profileCanObserveAllProjectWork(profile)
	if !canObserveAll {
		hiddenSections = append(hiddenSections, "domain_counts", "project_audit", "unscoped_pipeline_steps")
	} else if profile.CanReconcileAllProjects && !profile.IsManager {
		hiddenSections = append(hiddenSections, "project_audit")
	}
	return Record{
		"scope":                scope,
		"scoped":               !canObserveAll,
		"can_view_all_project": canObserveAll,
		"department":           profile.Department,
		"permissions":          permissions,
		"role_labels":          profile.RoleLabels,
		"hidden_sections":      hiddenSections,
	}
}

func projectOperationsHealthCounts(summary Record, domainCounts Record) Record {
	return Record{
		"active":              summary["active_work_items"],
		"blocked":             summary["blocked_work_items"],
		"completed":           summary["completed_work_items"],
		"overdue":             summary["overdue_work_items"],
		"waiting_approval":    summary["pending_approvals"],
		"waiting_evidence":    summary["waiting_evidence"],
		"open_issues":         number(domainCounts["open_issues"]),
		"failed_acceptances":  number(domainCounts["failed_acceptances"]),
		"pending_procurement": number(domainCounts["pending_procurement"]),
		"material_risks":      number(domainCounts["material_risks"]),
	}
}

func projectOperationsProjectSummary(project Record) Record {
	return Record{
		"id":                 project["id"],
		"code":               project["code"],
		"name":               project["name"],
		"client_name":        project["client_name"],
		"pm_name":            project["pm_name"],
		"status":             project["status"],
		"progress_percent":   project["progress_percent"],
		"estimated_budget":   project["estimated_budget"],
		"contract_amount":    project["contract_amount"],
		"planned_start_date": project["planned_start_date"],
		"planned_end_date":   project["planned_end_date"],
	}
}

func workflowTimeline(run Record) []Record {
	if run == nil {
		return []Record{}
	}
	nodes, _ := run["nodes"].([]Record)
	out := make([]Record, 0, len(nodes))
	for _, node := range nodes {
		steps, _ := node["steps"].([]Record)
		completedSteps := 0
		skippedSteps := 0
		for _, step := range steps {
			switch fmt.Sprint(step["status"]) {
			case "completed", "submitted":
				completedSteps++
			case "skipped", "cancelled":
				skippedSteps++
			}
		}
		progressSteps := completedSteps
		if fmt.Sprint(node["status"]) == "skipped" {
			progressSteps = 0
		}
		out = append(out, Record{
			"id":               node["id"],
			"name":             node["name"],
			"type":             node["type"],
			"order_index":      node["order_index"],
			"status":           node["status"],
			"step_count":       len(steps),
			"completed_steps":  completedSteps,
			"skipped_steps":    skippedSteps,
			"progress_percent": percent(progressSteps, len(steps)),
			"started_at":       node["started_at"],
			"completed_at":     node["completed_at"],
		})
	}
	return out
}

func activitySummary(rows []Record) Record {
	counts := Record{}
	for _, row := range rows {
		activityType := fmt.Sprint(row["activity_type"])
		counts[activityType] = number(counts[activityType]) + 1
	}
	return Record{"total_recent": len(rows), "type_counts": counts}
}

func auditSummary(rows []Record) Record {
	counts := Record{}
	for _, row := range rows {
		eventType := fmt.Sprint(row["event_type"])
		counts[eventType] = number(counts[eventType]) + 1
	}
	return Record{"total_recent": len(rows), "event_counts": counts, "recent": rows}
}

func departmentSurfaces(workItems []Record) []Record {
	index := map[string]Record{}
	order := []string{}
	for _, item := range workItems {
		department := cleanText(item["department"])
		if department == "" {
			department = "unassigned"
		}
		surface, ok := index[department]
		if !ok {
			surface = Record{
				"department":       department,
				"total":            0,
				"active":           0,
				"blocked":          0,
				"waiting_approval": 0,
				"work_items":       []Record{},
			}
			index[department] = surface
			order = append(order, department)
		}
		surface["total"] = number(surface["total"]) + 1
		status := fmt.Sprint(item["status"])
		if status != "completed" && status != "cancelled" {
			surface["active"] = number(surface["active"]) + 1
		}
		if status == "blocked" {
			surface["blocked"] = number(surface["blocked"]) + 1
		}
		if status == "waiting_approval" {
			surface["waiting_approval"] = number(surface["waiting_approval"]) + 1
		}
		items, _ := surface["work_items"].([]Record)
		if len(items) < 10 {
			surface["work_items"] = append(items, item)
		}
	}
	out := make([]Record, 0, len(order))
	for _, department := range order {
		out = append(out, index[department])
	}
	return out
}

func cleanText(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	return text
}

func percent(done int, total int) int {
	if total == 0 {
		return 0
	}
	return int(math.Round(float64(done) * 100 / float64(total)))
}

func scopePipelineRunToWorkItems(run Record, workItems []Record) Record {
	visibleNodes := map[string]bool{}
	visibleSteps := map[string]bool{}
	for _, item := range workItems {
		if id := cleanID(item["run_node_id"]); id != "" {
			visibleNodes[id] = true
		}
		if id := cleanID(item["run_step_id"]); id != "" {
			visibleSteps[id] = true
		}
	}
	nodes, _ := run["nodes"].([]Record)
	filteredNodes := []Record{}
	for _, node := range nodes {
		steps, _ := node["steps"].([]Record)
		filteredSteps := []Record{}
		for _, step := range steps {
			if visibleSteps[fmt.Sprint(step["id"])] {
				filteredSteps = append(filteredSteps, step)
			}
		}
		if visibleNodes[fmt.Sprint(node["id"])] || len(filteredSteps) > 0 {
			node["steps"] = filteredSteps
			filteredNodes = append(filteredNodes, node)
		}
	}
	run["nodes"] = filteredNodes
	run["scoped"] = true
	return run
}

func workItemScopeArgs(profile workItemAccessProfile, managerOverride bool, includeWatchers bool) []any {
	return []any{
		managerOverride,
		profile.UserID,
		profile.StaffID,
		profile.Department,
		includeWatchers,
	}
}

func workItemScopeSQL(alias string, start int) string {
	userID := fmt.Sprintf("NULLIF($%d,'')::uuid", start+1)
	manager := fmt.Sprintf("($%d::boolean AND (NOT %s OR %s))", start, deputyManagerUserSQL(userID), supervisedProjectSQL(alias+".project_id"))
	staffID := fmt.Sprintf("NULLIF($%d,'')::uuid", start+2)
	staffText := fmt.Sprintf("$%d", start+2)
	department := fmt.Sprintf("$%d", start+3)
	includeWatchers := fmt.Sprintf("$%d::boolean", start+4)
	return fmt.Sprintf(`
		%s OR (
			(%s IS NOT NULL OR %s IS NOT NULL) AND (
				%s.assignee_staff_id=%s
				OR (%s AND %s.watcher_staff_ids ? %s)
				OR EXISTS (
					SELECT 1
					FROM work_item_assignments wias
					WHERE wias.work_item_id=%s.id
					  AND (
						(wias.assignment_type <> 'watcher' AND wias.staff_id=%s)
						OR (%s AND wias.assignment_type='watcher' AND wias.staff_id=%s)
					  )
				)
				OR EXISTS (
					SELECT 1
					FROM project_members pm
					WHERE pm.project_id=%s.project_id
					  AND pm.staff_id=%s
					  AND pm.active
					  AND (
						pm.role_ids ? 'admin'
						OR pm.role_ids ? 'director'
						OR pm.role_ids ? 'Giám đốc'
						OR pm.role_ids ? 'pm'
						OR pm.role_ids ? 'PM'
						OR pm.role_ids ? 'Project Manager'
						OR pm.role_ids ? 'Quản lý dự án'
						OR (NULLIF(%s,'') IS NOT NULL AND %s.department=%s)
						OR EXISTS (
							SELECT 1
							FROM jsonb_array_elements_text(pm.role_ids) pm_role(role_label)
							JOIN jsonb_array_elements_text(%s.role_ids) wi_role(role_label) ON consflow_role_label_key(wi_role.role_label)=consflow_role_label_key(pm_role.role_label)
						)
						OR EXISTS (
							SELECT 1
							FROM work_item_assignments wias
							WHERE wias.work_item_id=%s.id
							  AND wias.assignment_type <> 'watcher'
							  AND (
								(NULLIF(%s,'') IS NOT NULL AND wias.department=%s)
								OR (
									NULLIF(wias.role_name, '') IS NOT NULL
									AND EXISTS (
										SELECT 1
										FROM jsonb_array_elements_text(pm.role_ids) pm_role(role_label)
										WHERE consflow_role_label_key(pm_role.role_label)=consflow_role_label_key(wias.role_name)
									)
								)
							  )
						)
					  )
				)
				OR EXISTS (
					SELECT 1
					FROM user_roles ur
					JOIN roles r ON r.id=ur.role_id
					WHERE ur.user_id=%s
					  AND ur.project_id=%s.project_id
					  AND (
						r.name IN ('admin','director','pm')
						OR r.display_name IN ('Admin','Giám đốc','PM','Project Manager','Quản lý dự án')
						OR EXISTS (
							SELECT 1
							FROM jsonb_array_elements_text(%s.role_ids) wi_role(role_label)
							WHERE consflow_role_label_key(wi_role.role_label) IN (consflow_role_label_key(r.name), consflow_role_label_key(r.display_name), r.id::text)
						)
						OR EXISTS (
							SELECT 1
							FROM work_item_assignments wias
							WHERE wias.work_item_id=%s.id
							  AND wias.assignment_type <> 'watcher'
							  AND NULLIF(wias.role_name, '') IS NOT NULL
							  AND consflow_role_label_key(wias.role_name) IN (consflow_role_label_key(r.name), consflow_role_label_key(r.display_name), r.id::text)
						)
					  )
				)
			)
		)
	`, manager, staffID, userID, alias, staffID, includeWatchers, alias, staffText, alias, staffID, includeWatchers, staffID, alias, staffID, department, alias, department, alias, alias, department, department, userID, alias, alias, alias)
}
