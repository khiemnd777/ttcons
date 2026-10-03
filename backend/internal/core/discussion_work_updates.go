package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"consflow/backend/internal/shared/response"
	"github.com/gofiber/fiber/v2"
)

// Full snapshots stay out of the feed. Its metadata carries only stable source
// identity; opening the snapshot rechecks current project/work-item visibility.
func insertDiscussionWorkUpdateTx(ctx context.Context, tx *sql.Tx, projectID, workItemID, actorID, sourceType, sourceID, eventKey, comment string, snapshot Record) error {
	key := discussionWorkUpdateEventKey(sourceType, sourceID, eventKey)
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "discussion_update:"+key); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_work_updates WHERE source_event_key=$1)`, key).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	scope, scopeID := "project", projectID
	if sourceType == "procurement_request" {
		scope, scopeID = "procurement_request", sourceID
	}
	if workItemID != "" {
		scope, scopeID = "work_item", workItemID
	}
	metadata, err := json.Marshal(Record{"work_update": Record{
		"source_type": sourceType, "source_id": sourceID, "event_key": eventKey,
		"revision_label": snapshot["revision_label"], "status": snapshot["status"],
	}})
	if err != nil {
		return err
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	var activityID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO work_item_activities (project_id,work_item_id,scope_type,scope_id,actor_id,activity_type,comment,visibility,metadata_json)
		VALUES ($1,NULLIF($2,'')::uuid,$3,$4,NULLIF($5,'')::uuid,'work_update',$6,'internal',$7::jsonb) RETURNING id
	`, projectID, workItemID, scope, scopeID, actorID, comment, string(metadata)).Scan(&activityID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_work_updates (activity_id,source_event_key,source_type,source_id,source_snapshot_json) VALUES ($1,$2,$3,$4,$5::jsonb)`, activityID, key, sourceType, sourceID, string(raw))
	return err
}

func discussionWorkUpdateEventKey(sourceType, sourceID, eventKey string) string {
	return sourceType + ":" + sourceID + ":" + eventKey
}

func insertWorkflowDiscussionUpdateTx(ctx context.Context, tx *sql.Tx, instance Record, answers Record, revision int, event, actorID string) error {
	workItemID := cleanID(instance["work_item_id"])
	if workItemID == "" {
		return nil
	}
	fields, err := workflowFormFieldRowsTx(ctx, tx, cleanID(instance["form_version_id"]))
	if err != nil {
		return err
	}
	label := fmt.Sprintf("Lần chốt %d", revision)
	comment := "Đã chốt hồ sơ nghiệp vụ · " + label
	if event == "amended" {
		comment = "Đã bổ sung hồ sơ nghiệp vụ · " + label
	}
	return insertDiscussionWorkUpdateTx(ctx, tx, cleanID(instance["project_id"]), workItemID, actorID, "workflow_form", cleanID(instance["id"]), fmt.Sprintf("%s:%d", event, revision), comment, Record{
		"title": instance["title"], "revision_label": label, "revision_no": revision, "status": "submitted",
		"form_version_id": instance["form_version_id"], "form_code": instance["form_code"], "fields": fields, "answers": answers,
	})
}

func insertDesignDiscussionUpdateTx(ctx context.Context, tx *sql.Tx, release Record, workItemID, event, actorID string, document Record) error {
	releaseID, projectID := cleanID(release["id"]), cleanID(release["project_id"])
	if workItemID == "" {
		workItemID = cleanID(release["work_item_id"])
	}
	files, err := queryTx(ctx, tx, `
		SELECT evidence.id,evidence.id AS evidence_id,'evidence' AS reference_type,
			item.document_type,evidence.name,evidence.mime_type,evidence.file_size,
			evidence.evidence_type,evidence.status,evidence.revision_no,evidence.created_at
		FROM design_release_items item
		JOIN evidence_records evidence
		  ON evidence.id=item.evidence_id
		 AND evidence.project_id=item.project_id
		WHERE item.project_id=$1 AND item.design_release_id=$2
		ORDER BY item.position,item.created_at,item.id
	`, projectID, releaseID)
	if err != nil {
		return err
	}
	status, comment := fmt.Sprint(release["status"]), "Đã phát hành hồ sơ thiết kế"
	if document != nil {
		comment = "Đã thêm tệp thiết kế · " + fmt.Sprint(document["name"])
		files = designDiscussionFileSnapshots([]Record{document})
	} else if event == "released" {
		status = "released"
	} else {
		comment = "Quản lý dự án đã bàn giao hồ sơ thiết kế cho QS"
	}
	return insertDiscussionWorkUpdateTx(ctx, tx, projectID, workItemID, actorID, "design_release", releaseID, event, comment, Record{
		"title": "Hồ sơ thiết kế", "revision_label": release["revision_label"], "revision_no": release["revision_no"],
		"status": status, "change_reason": release["change_reason"], "notes": release["notes"], "files": files,
	})
}

func insertDesignUploadBatchDiscussionUpdateTx(ctx context.Context, tx *sql.Tx, release Record, batchID, actorID string, documents []Record) error {
	return insertDiscussionWorkUpdateTx(
		ctx,
		tx,
		cleanID(release["project_id"]),
		cleanID(release["work_item_id"]),
		actorID,
		"design_release",
		cleanID(release["id"]),
		"files:"+batchID,
		fmt.Sprintf("Đã thêm %d tệp thiết kế", len(documents)),
		Record{
			"title":          "Hồ sơ thiết kế",
			"revision_label": release["revision_label"],
			"revision_no":    release["revision_no"],
			"status":         release["status"],
			"change_reason":  release["change_reason"],
			"notes":          release["notes"],
			"files":          designDiscussionFileSnapshots(documents),
		},
	)
}

func designDiscussionFileSnapshots(documents []Record) []Record {
	files := make([]Record, 0, len(documents))
	for _, document := range documents {
		files = append(files, Record{
			"id":             document["id"],
			"evidence_id":    document["id"],
			"reference_type": "evidence",
			"name":           document["name"],
			"document_type":  document["document_type"],
			"mime_type":      document["mime_type"],
			"file_size":      document["file_size"],
			"evidence_type":  document["evidence_type"],
			"status":         document["status"],
			"created_at":     document["created_at"],
		})
	}
	return files
}

func (s *Service) DiscussionWorkUpdate(ctx context.Context, activityID, actorID string) (Record, error) {
	if !validActivityUUID(activityID) {
		return nil, validationError("Mã cập nhật không hợp lệ")
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	visible, err := s.repo.ActivityVisibleToUser(ctx, activityID, profile)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, forbiddenError("Bạn không có quyền xem cập nhật này")
	}
	row, err := s.repo.QueryOne(ctx, `SELECT u.activity_id,u.source_type,u.source_id,u.source_snapshot_json AS snapshot,a.project_id,a.work_item_id,a.created_at,a.comment FROM activity_work_updates u JOIN work_item_activities a ON a.id=u.activity_id WHERE u.activity_id=$1 AND a.activity_type='work_update' AND a.withdrawn_at IS NULL`, activityID)
	if err != nil {
		return nil, err
	}
	if row["source_type"] == "procurement_request" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, cleanID(row["project_id"]), []string{"procurement.view"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem yêu cầu mua này")
		}
		if _, err := s.repo.QueryOne(ctx, `SELECT id FROM procurement_requests WHERE id=$1 AND project_id=$2 AND status <> 'cancelled'`, row["source_id"], row["project_id"]); err != nil {
			return nil, err
		}
	}
	return row, nil
}

func (h *Handler) DiscussionWorkUpdate(c *fiber.Ctx) error {
	record, err := h.service.DiscussionWorkUpdate(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, record)
}

func (s *Service) publishDiscussionWorkUpdate(projectID, workItemID, actorID string) {
	s.publishActivityEvent(ActivityStreamEvent{Type: "activity_changed", ProjectID: projectID, WorkItemID: workItemID, ActorID: actorID})
}
